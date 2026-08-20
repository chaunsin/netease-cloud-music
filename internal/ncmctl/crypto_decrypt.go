// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	har "github.com/chaunsin/go-har"
	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

const (
	decryptTargetAuto     = "auto"
	decryptTargetRequest  = "request"
	decryptTargetResponse = "response"
	decryptTargetBoth     = "both"

	eapiEnvelopeDelimiter = "-36cd479b6b5-"
)

type decryptCmd struct {
	root *Crypto
	cmd  *cobra.Command
	l    *log.Logger

	url    string
	encode string
	target string

	dynamicKey       string
	dynamicKeyEncode string
}

func decrypt(root *Crypto, l *log.Logger) *cobra.Command {
	c := &decryptCmd{
		root: root,
		l:    l,
	}
	c.cmd = &cobra.Command{
		Use:   "decrypt <ciphertext-or-har>",
		Short: "Decrypt EAPI or XEAPI payloads and HAR files",
		Long: "Decrypt direct EAPI requests, XEAPI requests or responses, and matching HAR entries. " +
			"XEAPI request B needs its dynamic key; R is always decoded, while the S frame is validated " +
			"but cannot be decrypted without an X25519 private key. Raw direct bytes and HAR response " +
			"ciphertext are emitted as Base64 with their encoding recorded. Restrict mixed HAR captures " +
			"with --url. Inputs, dynamic " +
			"keys, and output may contain secrets.",
		Example: "  ncmctl crypto decrypt --kind eapi --encode hex 'CIPHERTEXT'\n" +
			"  ncmctl crypto decrypt --kind xeapi --encode hex 'CIPHERTEXT'\n" +
			"  ncmctl crypto decrypt --kind xeapi --target request --dynamic-key-encode hex --dynamic-key 00112233445566778899aabbccddeeff 'B=...&S=...&R=...'\n" +
			"  ncmctl crypto decrypt --url '/xeapi/*' capture.har --output decrypted.json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute(cmd.Context(), args)
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *decryptCmd) addFlags() {
	c.cmd.Flags().StringVarP(&c.encode, "encode", "e", "hex", "direct ciphertext encoding: string, hex, or base64")
	c.cmd.Flags().StringVarP(&c.url, "url", "u", "*", "path glob used to select HAR entries (for example /xeapi/*)")
	c.cmd.Flags().StringVar(&c.target, "target", decryptTargetAuto, "decrypt target: auto, request, response, or both (both is HAR-only)")
	c.cmd.Flags().StringVar(&c.dynamicKey, "dynamic-key", "", "XEAPI request dynamic/session key (sensitive; not read from HAR or state files)")
	c.cmd.Flags().StringVar(&c.dynamicKeyEncode, "dynamic-key-encode", "string", "dynamic key encoding: string, hex, or base64")
}

func (c *decryptCmd) execute(_ context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("nothing was entered")
	}

	if err := validateCryptoEncoding(c.encode); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	if err := validateCryptoEncoding(c.dynamicKeyEncode); err != nil {
		return fmt.Errorf("dynamic-key-encode: %w", err)
	}

	dynamicKey, err := decodeCryptoBytes(c.dynamicKey, c.dynamicKeyEncode)
	if err != nil {
		return fmt.Errorf("decode dynamic key: %w", err)
	}

	if keyErr := validateDynamicKey(dynamicKey); keyErr != nil {
		return keyErr
	}

	var (
		input       = args[0]
		fileData    []byte
		inputIsFile = utils.IsFile(input)
		isHAR       = inputIsFile && strings.EqualFold(filepath.Ext(input), ".har")
	)

	if inputIsFile {
		fileData, err = os.ReadFile(input)
		if err != nil {
			return fmt.Errorf("read input file: %w", err)
		}
	}

	target, err := c.resolveTarget(isHAR)
	if err != nil {
		return err
	}

	if len(dynamicKey) > 0 && (target == decryptTargetResponse || (!isHAR && c.root.Kind != "xeapi")) {
		return errors.New("dynamic-key is only valid when decrypting XEAPI requests")
	}

	if isHAR {
		return c.executeHAR(fileData, target, dynamicKey)
	}

	if inputIsFile {
		input = string(fileData)
	}
	return c.executeDirect(input, target, dynamicKey)
}

func (c *decryptCmd) resolveTarget(isHAR bool) (string, error) {
	switch c.target {
	case decryptTargetAuto:
		if isHAR {
			return decryptTargetBoth, nil
		}

		if c.root.Kind == "xeapi" {
			return decryptTargetResponse, nil
		}
		return decryptTargetRequest, nil
	case decryptTargetRequest, decryptTargetResponse:
	case decryptTargetBoth:
		if !isHAR {
			return "", errors.New("target both is only supported for HAR input")
		}
	default:
		return "", fmt.Errorf("unknown decrypt target %q", c.target)
	}

	return c.target, nil
}

func (c *decryptCmd) executeDirect(input, target string, dynamicKey []byte) error {
	payload := &Payload{Kind: c.root.Kind, Status: "ok"}

	switch target {
	case decryptTargetRequest:
		if payload.Kind == "xeapi" {
			params, parseErr := parseXeapiForm(input)
			payload.Request.Params = params

			if err := errors.Join(parseErr, c.decryptReq(payload, dynamicKey)); err != nil {
				markPartialRequest(payload, err)
				return c.writeResult(payload, errors.New("XEAPI request was only partially decrypted; inspect request.error"))
			}
		} else {
			ciphertext, encoding, err := normalizeCiphertext(input, c.encode)
			if err != nil {
				return fmt.Errorf("decode request ciphertext: %w", err)
			}

			payload.Request.Ciphertext = ciphertext
			payload.Request.CiphertextEncoding = encoding

			if err := c.decryptReq(payload, nil); err != nil {
				return fmt.Errorf("decrypt request: %w", err)
			}
		}
	case decryptTargetResponse:
		ciphertext, encoding, err := normalizeCiphertext(input, c.encode)
		if err != nil {
			return fmt.Errorf("decode response ciphertext: %w", err)
		}

		payload.Response.Ciphertext = ciphertext
		payload.Response.CiphertextEncoding = encoding

		if err := c.decryptRes(payload); err != nil {
			return fmt.Errorf("decrypt response: %w", err)
		}
	default:
		return fmt.Errorf("unsupported direct decrypt target %q", target)
	}

	return c.writeResult(payload, nil)
}

func (c *decryptCmd) executeHAR(data []byte, target string, dynamicKey []byte) error {
	list, err := c.parseHar(data, target)
	if err != nil {
		return fmt.Errorf("parse HAR: %w", err)
	}

	c.l.Debugf("parseHar entries=%d", len(list))

	partialCount := 0

	for i := range list {
		item := &list[i]
		item.Status = "ok"

		if target == decryptTargetRequest || target == decryptTargetBoth {
			requestErr := errors.Join(item.requestErr, c.decryptReq(item, dynamicKey))
			if requestErr != nil {
				if item.Kind != "xeapi" {
					return fmt.Errorf("decrypt request %s: %w", item.Api, requestErr)
				}

				markPartialRequest(item, requestErr)

				partialCount++
			}
		}

		if target == decryptTargetResponse || target == decryptTargetBoth {
			if err := c.decryptRes(item); err != nil {
				return fmt.Errorf("decrypt response %s: %w", item.Api, err)
			}
		}
	}

	var partialErr error
	if partialCount > 0 {
		partialErr = fmt.Errorf("%d XEAPI request(s) were only partially decrypted; inspect request.error", partialCount)
	}
	return c.writeResult(list, partialErr)
}

func (c *decryptCmd) writeResult(value any, resultErr error) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal decrypt result: %w", err)
	}

	if err := writeFile(c.cmd, c.root.Output, content); err != nil {
		return err
	}

	if resultErr != nil {
		root := c.cmd.Root()
		root.SilenceUsage = true
		root.SilenceErrors = true
	}
	return resultErr
}

func markPartialRequest(payload *Payload, err error) {
	payload.Status = "partial"
	payload.Request.Error = err.Error()
}

func parseEAPIEnvelope(raw string) (string, string, string) {
	if json.Valid([]byte(raw)) {
		return raw, "", ""
	}

	var (
		first = strings.Index(raw, eapiEnvelopeDelimiter)
		last  = strings.LastIndex(raw, eapiEnvelopeDelimiter)
	)

	if first < 0 || first == last {
		return raw, "", ""
	}

	return raw[first+len(eapiEnvelopeDelimiter) : last],
		raw[:first],
		raw[last+len(eapiEnvelopeDelimiter):]
}

func eapiResponseEncrypted(plaintext []byte) (bool, error) {
	data := bytes.TrimSpace(plaintext)
	if len(data) < 2 || data[0] != '{' || data[len(data)-1] != '}' {
		return true, nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return false, fmt.Errorf("json.Unmarshal EAPI request: %w", err)
	}

	raw, ok := fields["e_r"]
	if !ok {
		return true, nil
	}

	raw = bytes.TrimSpace(raw)
	if bytes.Equal(raw, []byte("null")) {
		return false, errors.New("e_r must be a boolean")
	}

	var encrypted bool
	if err := json.Unmarshal(raw, &encrypted); err != nil {
		return false, fmt.Errorf("decode e_r: %w", err)
	}
	return encrypted, nil
}

func (c *decryptCmd) decryptReq(p *Payload, dynamicKey []byte) error {
	if p == nil {
		return errors.New("request payload is nil")
	}

	switch p.Kind {
	case "eapi":
		if p.Request.Ciphertext == "" {
			return errors.New("request ciphertext is empty")
		}

		data, err := crypto.EApiDecrypt(p.Request.Ciphertext, p.Request.CiphertextEncoding)
		if err != nil {
			return fmt.Errorf("EApiDecrypt: %w", err)
		}

		raw := string(data)
		plaintext, requestURL, digest := parseEAPIEnvelope(raw)

		if !json.Valid([]byte(plaintext)) {
			return errors.New("EAPI request plaintext is not valid JSON")
		}

		responseEncrypted, err := eapiResponseEncrypted([]byte(plaintext))
		if err != nil {
			return fmt.Errorf("parse EAPI response mode: %w", err)
		}

		p.Request.RawPlaintext = raw
		p.Request.Url = requestURL
		p.Request.Digest = digest
		p.Request.Plaintext = json.RawMessage(plaintext)
		p.responseEncrypted = &responseEncrypted
	case "xeapi":
		decryptKey := dynamicKey
		if p.Request.Params["B"] == "" {
			// R remains independently recoverable from an incomplete capture.
			decryptKey = nil
		}

		result, decryptErr := crypto.XeapiDecryptRequest(crypto.XeapiEncryptedRequest{
			B: p.Request.Params["B"],
			S: p.Request.Params["S"],
			R: p.Request.Params["R"],
		}, decryptKey)
		if result.PublicKeyVersion != "" {
			p.Request.KeyVersion = result.PublicKeyVersion
			p.Request.SessionID = &result.SessionID
		}

		var requestErrs []error
		if decryptErr != nil {
			requestErrs = append(requestErrs, fmt.Errorf("XeapiDecryptRequest: %w", decryptErr))
		}

		if p.Request.Params["B"] == "" {
			requestErrs = append(requestErrs, errors.New("XEAPI request parameter B is missing"))
		}

		if p.Request.Params["S"] == "" {
			requestErrs = append(requestErrs, errors.New("XEAPI request parameter S is missing"))
		}

		if p.Request.Params["B"] != "" {
			switch {
			case len(dynamicKey) == 0:
				requestErrs = append(requestErrs, errors.New("dynamic key is required to decrypt XEAPI B"))
			case len(result.Plaintext) == 0 && decryptErr != nil:
				// The decrypt error above already identifies the malformed B field.
			case !json.Valid(result.Plaintext):
				requestErrs = append(requestErrs, errors.New("XEAPI request envelope is not valid JSON"))
			default:
				p.Request.RawPlaintext = string(result.Plaintext)
				p.Request.Plaintext = append(json.RawMessage(nil), result.Plaintext...)
			}
		}
		return errors.Join(requestErrs...)
	case "weapi":
		return fmt.Errorf("this [%s] method is not supported", p.Kind)
	case "api", "linux":
		return fmt.Errorf("%s to be realized", p.Kind)
	default:
		return fmt.Errorf("unknown crypto kind %q", p.Kind)
	}
	return nil
}

func (c *decryptCmd) decryptRes(p *Payload) error {
	if p == nil {
		return errors.New("response payload is nil")
	}

	if p.Response.Ciphertext == "" {
		return errors.New("response ciphertext is empty")
	}

	ciphertext, err := decodeCryptoBytes(p.Response.Ciphertext, p.Response.CiphertextEncoding)
	if err != nil {
		return fmt.Errorf("decode response ciphertext: %w", err)
	}

	c.l.Debugf("[decryptRes] ciphertext_bytes=%d", len(ciphertext))

	switch p.Kind {
	case "eapi":
		responseEncrypted := p.responseEncrypted == nil || *p.responseEncrypted
		plaintext := ciphertext

		if responseEncrypted {
			data, decryptErr := crypto.EApiDecrypt(string(ciphertext), "")
			if decryptErr != nil {
				return fmt.Errorf("EApiDecrypt: %w", decryptErr)
			}

			c.l.Debugf("[decryptRes] decrypted_bytes=%d", len(data))

			plaintext, err = utils.GzipReader(data)
			if err != nil {
				return fmt.Errorf("GzipReader: %w", err)
			}

			payload, _, _ := parseEAPIEnvelope(string(plaintext))
			plaintext = []byte(payload)
		}

		if !json.Valid(plaintext) {
			return errors.New("EAPI response plaintext is not valid JSON")
		}

		p.Response.Plaintext = append(json.RawMessage(nil), plaintext...)
	case "xeapi":
		plaintext, err := crypto.XeapiDecrypt(ciphertext)
		if err != nil {
			return fmt.Errorf("XeapiDecrypt: %w", err)
		}

		if !json.Valid(plaintext) {
			return errors.New("XEAPI response plaintext is not valid JSON")
		}

		c.l.Debugf("[decryptRes] decrypted_bytes=%d", len(plaintext))
		p.Response.Plaintext = append(json.RawMessage(nil), plaintext...)
	case "weapi":
		return fmt.Errorf("this [%s] method is not supported", p.Kind)
	case "api", "linux":
		return fmt.Errorf("%s to be realized", p.Kind)
	default:
		return fmt.Errorf("unknown crypto kind %q", p.Kind)
	}
	return nil
}

func (c *decryptCmd) parseHar(data []byte, target string) ([]Payload, error) {
	h, err := har.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("har.NewReader: %w", err)
	}

	if h.EntryTotal() == 0 {
		return nil, errors.New("request data is empty")
	}

	document := h.Export()

	var (
		requestSelected  = target == decryptTargetRequest || target == decryptTargetBoth
		responseSelected = target == decryptTargetResponse || target == decryptTargetBoth
		resp             = make([]Payload, 0, len(document.Log.Entries))
	)

	for i, entry := range document.Log.Entries {
		req := entry.Request
		if req == nil {
			return nil, fmt.Errorf("entry %d request is missing", i)
		}

		if strings.TrimSpace(req.URL) == "" {
			return nil, fmt.Errorf("entry %d request URL is missing", i)
		}

		parsedURL, err := url.Parse(req.URL)
		if err != nil {
			return nil, fmt.Errorf("parse entry %d request URL: %w", i, err)
		}

		matched, err := isMatch(c.url, parsedURL.Path)
		if err != nil {
			return nil, fmt.Errorf("match URL path: %w", err)
		}

		if !matched {
			continue
		}

		kind := harCryptoKind(parsedURL.Path, c.root.Kind)
		item := Payload{Api: req.URL, Method: req.Method, Kind: kind}

		if requestSelected {
			switch kind {
			case "eapi":
				encodedCiphertext, parseErr := harFormValue(req.PostData, "params")
				if parseErr != nil {
					return nil, fmt.Errorf("parse EAPI request %s: %w", req.URL, parseErr)
				}

				if encodedCiphertext != "" {
					ciphertext, encoding, normalizeErr := normalizeCiphertext(encodedCiphertext, "hex")
					if normalizeErr != nil {
						return nil, fmt.Errorf("decode EAPI request %s: %w", req.URL, normalizeErr)
					}

					item.Request.Ciphertext = ciphertext
					item.Request.CiphertextEncoding = encoding
				}
			case "xeapi":
				params, parseErr := harXeapiParams(req.PostData)

				item.Request.Params = params
				if parseErr != nil {
					item.requestErr = fmt.Errorf("parse XEAPI request form: %w", parseErr)
				}
			case "weapi":
				// A passive capture does not contain WEAPI's random request key.
			case "api":
				if req.PostData != nil && len(req.PostData.Params) > 0 && req.PostData.Params[0] != nil {
					item.Request.RawPlaintext = req.PostData.Params[0].Value
				}
			case "linux":
				return nil, fmt.Errorf("HAR parsing not supported for %s request %s", kind, req.URL)
			default:
				return nil, fmt.Errorf("unknown crypto kind %q for %s", kind, req.URL)
			}
		} else {
			switch kind {
			case "eapi", "xeapi", "weapi", "api", "linux":
			default:
				return nil, fmt.Errorf("unknown crypto kind %q for %s", kind, req.URL)
			}
		}

		if responseSelected {
			if entry.Response == nil {
				return nil, fmt.Errorf("entry %d response is missing", i)
			}

			if entry.Response.Content == nil {
				return nil, fmt.Errorf("entry %d response content is missing", i)
			}

			item.Response.Ciphertext = base64.StdEncoding.EncodeToString(entry.Response.Content.Text)
			item.Response.CiphertextEncoding = "base64"
		}

		resp = append(resp, item)
	}
	return resp, nil
}

func harCryptoKind(path, fallback string) string {
	kind := ""

	for part := range strings.SplitSeq(path, "/") {
		switch part {
		case "eapi", "xeapi", "weapi", "linux":
			return part
		case "api":
			kind = part
		}
	}

	if kind != "" {
		return kind
	}
	return fallback
}

func harFormValue(postData *har.PostData, name string) (string, error) {
	if postData == nil {
		return "", nil
	}

	for _, param := range postData.Params {
		if param != nil && param.Name == name {
			return param.Value, nil
		}
	}

	if !isFormURLEncoded(postData.MimeType) {
		return "", nil
	}

	values, err := url.ParseQuery(postData.Text)
	if err != nil {
		return "", fmt.Errorf("url.ParseQuery: %w", err)
	}
	return values.Get(name), nil
}

func harXeapiParams(postData *har.PostData) (map[string]string, error) {
	if postData == nil {
		return map[string]string{}, nil
	}

	values := make(url.Values)

	if len(postData.Params) > 0 {
		for _, param := range postData.Params {
			if param != nil {
				values.Set(param.Name, param.Value)
			}
		}
	} else if isFormURLEncoded(postData.MimeType) {
		parsed, err := url.ParseQuery(postData.Text)

		values = parsed
		if err != nil {
			return xeapiParams(values), fmt.Errorf("url.ParseQuery: %w", err)
		}
	}
	return xeapiParams(values), nil
}

func parseXeapiForm(input string) (map[string]string, error) {
	values, err := url.ParseQuery(input)
	if err != nil {
		return xeapiParams(values), fmt.Errorf("url.ParseQuery XEAPI form: %w", err)
	}
	return xeapiParams(values), nil
}

func isFormURLEncoded(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType, _, _ = strings.Cut(strings.TrimSpace(contentType), ";")
	}
	return strings.EqualFold(strings.TrimSpace(mediaType), "application/x-www-form-urlencoded")
}

func xeapiParams(values url.Values) map[string]string {
	params := make(map[string]string, 3)

	for _, name := range []string{"B", "S", "R"} {
		if value := values.Get(name); value != "" {
			params[name] = value
		}
	}
	return params
}

func validateCryptoEncoding(encoding string) error {
	switch encoding {
	case "string", "hex", "base64":
		return nil
	default:
		return fmt.Errorf("unknown encoding %q", encoding)
	}
}

func normalizeCiphertext(value, encoding string) (string, string, error) {
	data, err := decodeCryptoBytes(value, encoding)
	if err != nil {
		return "", "", err
	}

	switch encoding {
	case "", "string":
		return base64.StdEncoding.EncodeToString(data), "base64", nil
	case "hex", "base64":
		return strings.TrimSpace(value), encoding, nil
	default:
		return "", "", fmt.Errorf("unknown encoding %q", encoding)
	}
}

func decodeCryptoBytes(value, encoding string) ([]byte, error) {
	var (
		data []byte
		err  error
	)

	switch encoding {
	case "", "string":
		data = []byte(value)
	case "hex":
		data, err = hex.DecodeString(strings.TrimSpace(value))
	case "base64":
		data, err = base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	default:
		return nil, fmt.Errorf("unknown encoding %q", encoding)
	}

	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", encoding, err)
	}
	return data, nil
}

func validateDynamicKey(key []byte) error {
	if len(key) == 0 {
		return nil
	}

	switch len(key) {
	case 16, 24, 32:
		return nil
	default:
		return fmt.Errorf("dynamic key must decode to 16, 24, or 32 bytes; got %d", len(key))
	}
}

func isMatch(pattern, text string) (bool, error) {
	decodedPattern, err := url.PathUnescape(pattern)
	if err != nil {
		return false, fmt.Errorf("PathUnescape: %w", err)
	}

	pattern = decodedPattern
	pattern = strings.ReplaceAll(pattern, ".", `\.`)
	pattern = strings.ReplaceAll(pattern, "*", `.*`)
	pattern = "^" + pattern + "$"

	match, err := regexp.MatchString(pattern, text)
	if err != nil {
		return false, fmt.Errorf("MatchString: %w", err)
	}
	return match, nil
}

type Payload struct {
	Api      string   `json:"api,omitempty"`
	Method   string   `json:"method,omitempty"`
	Kind     string   `json:"kind,omitempty"`
	Status   string   `json:"status,omitempty"`
	Request  Request  `json:"request,omitzero"`
	Response Response `json:"response,omitzero"`

	requestErr        error
	responseEncrypted *bool
}

type Request struct {
	Ciphertext         string            `json:"ciphertext,omitempty"`
	CiphertextEncoding string            `json:"ciphertextEncoding,omitempty"`
	Params             map[string]string `json:"params,omitempty"`
	RawPlaintext       string            `json:"rawPlaintext,omitempty"`
	Url                string            `json:"url,omitempty"`
	Digest             string            `json:"digest,omitempty"`
	KeyVersion         string            `json:"keyVersion,omitempty"`
	SessionID          *string           `json:"sessionId,omitempty"`
	Error              string            `json:"error,omitempty"`
	Plaintext          json.RawMessage   `json:"plaintext,omitempty"`
}

type Response struct {
	Ciphertext         string          `json:"ciphertext,omitempty"`
	CiphertextEncoding string          `json:"ciphertextEncoding,omitempty"`
	Plaintext          json.RawMessage `json:"plaintext,omitempty"`
}

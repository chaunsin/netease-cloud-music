// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	ncmcrypto "github.com/chaunsin/netease-cloud-music/pkg/crypto"
)

type protocol string

const (
	protocolAPI     protocol = "api"
	protocolWEAPI   protocol = "weapi"
	protocolEAPI    protocol = "eapi"
	protocolLinux   protocol = "linux"
	protocolXEAPI   protocol = "xeapi"
	protocolGeneric protocol = "generic"
)

type decodeStatus string

const (
	decodeStatusPlaintext   decodeStatus = "plaintext"
	decodeStatusDecrypted   decodeStatus = "decrypted"
	decodeStatusUnsupported decodeStatus = "unsupported"
	decodeStatusFailed      decodeStatus = "failed"
	decodeStatusRaw         decodeStatus = "raw"
)

// decodeResult keeps protocol decoding separate from request forwarding. Its body
// and query fields are log copies and must never be written back to the HTTP flow.
type decodeResult struct {
	protocol          protocol
	status            decodeStatus
	body              []byte
	query             []byte
	apiPath           string
	detail            string
	responseEncrypted bool
}

const eapiSeparator = "-36cd479b6b5-"

func classifyProtocol(requestPath string) protocol {
	p := requestPath
	if u, err := url.Parse(requestPath); err == nil && u.Path != "" {
		p = u.Path
	}

	p = strings.ToLower(path.Clean("/" + strings.TrimPrefix(p, "/")))

	switch {
	case hasPathPrefix(p, "/api/linux/forward"):
		return protocolLinux
	case hasPathPrefix(p, "/weapi"):
		return protocolWEAPI
	case hasPathPrefix(p, "/eapi"):
		return protocolEAPI
	case hasPathPrefix(p, "/xeapi"):
		return protocolXEAPI
	case hasPathPrefix(p, "/api"):
		return protocolAPI
	default:
		return protocolGeneric
	}
}

func decodeRequestLimited(method string, u *url.URL, header http.Header, body []byte, showSensitive bool, maxBodyBytes int64) decodeResult {
	if u == nil {
		u = &url.URL{}
	}

	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultJSONDisplayLimit
	}

	query := u.Query()
	result := decodeResult{
		protocol:          classifyProtocol(u.Path),
		status:            decodeStatusPlaintext,
		apiPath:           u.Path,
		query:             formatQuery(query, showSensitive, maxBodyBytes),
		responseEncrypted: valuesRequestEncrypted(query),
	}

	switch result.protocol {
	case protocolEAPI:
		return decodeEAPIRequest(&result, query, header, body, showSensitive, maxBodyBytes)
	case protocolLinux:
		return decodeLinuxRequest(&result, query, header, body, showSensitive, maxBodyBytes)
	case protocolWEAPI:
		result.status = decodeStatusUnsupported
		result.detail = "weapi request decryption unsupported: the random AES key cannot be recovered"
	case protocolXEAPI:
		result.status = decodeStatusUnsupported
		result.detail = "xeapi request decryption unsupported: the session key is unavailable"
	case protocolAPI, protocolGeneric:
		// Plain API and generic requests do not require protocol-specific decoding.
	}

	display := formatBody(header, body, showSensitive, maxBodyBytes)

	result.body = display.body
	if display.structured {
		result.responseEncrypted = result.responseEncrypted || display.meta.requestEncrypted
	} else if result.status == decodeStatusPlaintext && len(body) > 0 {
		result.status = decodeStatusRaw
		result.detail = "request body is not structured JSON or form data"
	}

	result.detail = appendDetail(result.detail, display.detail)
	if len(body) == 0 && methodAllowsBody(method) {
		result.detail = appendDetail(result.detail, "empty request body")
	}
	return result
}

func decodeEAPIRequest(base *decodeResult, query url.Values, header http.Header, body []byte, showSensitive bool, maxBodyBytes int64) decodeResult {
	params, ok := requestParameter("params", query, header, body)
	if !ok || strings.TrimSpace(params) == "" {
		return failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, "eapi params field is missing")
	}

	plaintext, err := ncmcrypto.EApiDecrypt(strings.TrimSpace(params), "hex")
	if err != nil {
		return failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, "eapi decrypt: "+err.Error())
	}

	apiPath, payload, err := parseEAPIEnvelope(plaintext)
	if err != nil {
		return failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, err.Error())
	}

	safePath, err := sanitizeEAPIPath(apiPath, showSensitive)
	if err != nil {
		return failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, "eapi envelope path: "+err.Error())
	}

	formatted, meta, err := formatJSONLimited(payload, showSensitive, maxBodyBytes)
	if err != nil {
		return failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, "eapi payload JSON: "+err.Error())
	}

	base.status = decodeStatusDecrypted
	base.body = formatted
	base.apiPath = safePath
	base.detail = "eapi request decrypted; envelope digest verified"
	base.responseEncrypted = base.responseEncrypted || meta.requestEncrypted
	return *base
}

func parseEAPIEnvelope(plaintext []byte) (string, []byte, error) {
	first := bytes.Index(plaintext, []byte(eapiSeparator))

	last := bytes.LastIndex(plaintext, []byte(eapiSeparator))
	if first <= 0 || last <= first {
		return "", nil, errors.New("eapi envelope separators are missing")
	}

	apiPath := string(plaintext[:first])
	payload := plaintext[first+len(eapiSeparator) : last]

	digest := strings.TrimSpace(string(plaintext[last+len(eapiSeparator):]))
	if apiPath == "" || len(payload) == 0 || digest == "" {
		return "", nil, errors.New("eapi envelope is incomplete")
	}

	want := ncmcrypto.HexDigest("nobody" + apiPath + "use" + string(payload) + "md5forencrypt")

	got, err := hex.DecodeString(digest)
	if err != nil || len(got) != len(want)/2 {
		return "", nil, errors.New("eapi envelope digest is invalid")
	}

	if !strings.EqualFold(digest, want) {
		return "", nil, errors.New("eapi envelope digest mismatch")
	}
	return apiPath, payload, nil
}

func sanitizeEAPIPath(value string, showSensitive bool) (string, error) {
	if value == "" || !utf8.ValidString(value) {
		return "", errors.New("path is empty or not valid UTF-8")
	}

	if strings.Contains(value, "#") {
		return "", errors.New("path must not include a fragment")
	}

	for _, runeValue := range value {
		if unicode.IsControl(runeValue) {
			return "", errors.New("path contains a control character")
		}
	}

	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return "", fmt.Errorf("path is invalid: %w", err)
	}

	if parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || parsed.Path == "" || !strings.HasPrefix(parsed.Path, "/") {
		return "", errors.New("path must be an absolute API path without an authority")
	}

	if strings.HasPrefix(parsed.Path, "//") {
		return "", errors.New("path must not contain an authority-like prefix")
	}

	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("path must not include a query or fragment")
	}
	return redactURL(&url.URL{Path: parsed.Path, RawPath: parsed.RawPath}, showSensitive), nil
}

func decodeLinuxRequest(base *decodeResult, query url.Values, header http.Header, body []byte, showSensitive bool, maxBodyBytes int64) decodeResult {
	eparams, ok := requestParameter("eparams", query, header, body)
	if !ok || strings.TrimSpace(eparams) == "" {
		return failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, "linux eparams field is missing")
	}

	plaintext, err := ncmcrypto.LinuxApiDecrypt(strings.TrimSpace(eparams))
	if err != nil {
		return failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, "linux decrypt: "+err.Error())
	}

	formatted, meta, err := formatJSONLimited(plaintext, showSensitive, maxBodyBytes)
	if err != nil {
		return failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, "linux payload JSON: "+err.Error())
	}

	base.status = decodeStatusDecrypted
	base.body = formatted
	base.detail = "linux request decrypted"

	base.responseEncrypted = base.responseEncrypted || meta.requestEncrypted
	if requestURL := meta.rootURL; requestURL != "" {
		if parsed, parseErr := url.Parse(requestURL); parseErr == nil && parsed.Path != "" {
			if safePath, safeErr := sanitizeEAPIPath(parsed.EscapedPath(), showSensitive); safeErr == nil {
				base.apiPath = safePath
			}
		}
	}
	return *base
}

func failedRequestFallback(base *decodeResult, header http.Header, body []byte, showSensitive bool, maxBodyBytes int64, detail string) decodeResult {
	base.status = decodeStatusFailed
	display := formatBody(header, body, showSensitive, maxBodyBytes)
	base.responseEncrypted = base.responseEncrypted || display.meta.requestEncrypted

	base.detail = appendDetail(detail, display.detail)
	if display.structured {
		base.detail = appendDetail(base.detail, "showing safely formatted request")
	} else {
		base.detail = appendDetail(base.detail, "unstructured request body omitted by default")
	}

	base.body = display.body
	return *base
}

func decodeResponse(request *decodeResult, header http.Header, body []byte, maxBodyBytes int64, showSensitive bool) decodeResult {
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultJSONDisplayLimit
	}

	result := decodeResult{
		protocol:          request.protocol,
		status:            decodeStatusPlaintext,
		apiPath:           request.apiPath,
		responseEncrypted: request.responseEncrypted,
	}
	if len(body) == 0 {
		result.body = []byte{}
		result.detail = "empty response body"
		return result
	}

	if formatted, _, err := formatJSONLimited(body, showSensitive, maxBodyBytes); err == nil {
		result.body = formatted
		result.detail = "plaintext JSON response"
		return result
	}

	switch request.protocol {
	case protocolEAPI:
		if !request.responseEncrypted {
			display := formatBody(header, body, showSensitive, maxBodyBytes)
			result.status = decodeStatusRaw
			result.body = display.body
			result.detail = appendDetail("non-JSON EAPI response; response encryption was not declared", display.detail)
			return result
		}

		plaintext, gzipDecoded, encoding, err := decryptEAPIResponse(body, maxBodyBytes)
		if err == nil {
			result.status = decodeStatusDecrypted
			result.responseEncrypted = true
			display := formatBody(header, plaintext, showSensitive, maxBodyBytes)
			result.body = display.body

			result.detail = appendDetail("eapi encrypted response decrypted ("+encoding+")", display.detail)
			if gzipDecoded {
				result.detail += "; inner gzip decoded"
			}
			return result
		}

		result.status = decodeStatusFailed
		display := formatBody(header, body, showSensitive, maxBodyBytes)
		result.body = display.body
		result.detail = appendDetail(responseFailureDetail(request, "eapi response decrypt: "+err.Error()), display.detail)
		return result
	case protocolWEAPI, protocolXEAPI:
		// WEAPI/XEAPI use per-request or session secrets that a passive proxy
		// cannot recover. Treat non-JSON responses as opaque instead of trying
		// the unrelated static EAPI key and reporting a misleading failure.
		result.status = decodeStatusUnsupported
		display := formatBody(header, body, showSensitive, maxBodyBytes)
		result.body = display.body
		result.detail = appendDetail(string(request.protocol)+" response is not JSON; passive response decryption unsupported", display.detail)
		return result
	case protocolLinux:
		if !request.responseEncrypted {
			display := formatBody(header, body, showSensitive, maxBodyBytes)
			result.status = decodeStatusRaw
			result.body = display.body
			result.detail = appendDetail("non-JSON Linux response; response encryption was not declared", display.detail)
			return result
		}

		plaintext, err := ncmcrypto.LinuxApiDecrypt(strings.TrimSpace(string(body)))
		if err == nil {
			result.status = decodeStatusDecrypted
			result.responseEncrypted = true
			display := formatBody(header, plaintext, showSensitive, maxBodyBytes)
			result.body = display.body
			result.detail = appendDetail("linux encrypted response decrypted", display.detail)
			return result
		}

		result.status = decodeStatusFailed
		display := formatBody(header, body, showSensitive, maxBodyBytes)
		result.body = display.body
		result.detail = appendDetail(responseFailureDetail(request, "linux response decrypt: "+err.Error()), display.detail)
		return result
	default:
		display := formatBody(header, body, showSensitive, maxBodyBytes)
		result.body = display.body
		result.status = decodeStatusRaw
		result.detail = appendDetail("response body is not JSON", display.detail)
		return result
	}
}

func decryptEAPIResponse(body []byte, maxBodyBytes int64) ([]byte, bool, string, error) {
	var (
		plaintext []byte
		encoding  string
		err       error
	)
	// Hex-looking responses are unambiguous enough to prefer the request-style
	// representation. This also avoids a rare valid-padding false positive when
	// treating ASCII hex bytes as binary ciphertext first.
	if isHex(body) {
		plaintext, err = ncmcrypto.EApiDecrypt(strings.TrimSpace(string(body)), "hex")
		encoding = "hex"
	} else {
		plaintext, err = ncmcrypto.EApiDecrypt(string(body), "")
		encoding = "binary"
	}

	if err != nil {
		return nil, false, "", err
	}

	if len(plaintext) >= 2 && plaintext[0] == 0x1f && plaintext[1] == 0x8b {
		plaintext, err = gunzipLimited(plaintext, maxBodyBytes)
		if err != nil {
			return nil, false, "", fmt.Errorf("inner gzip: %w", err)
		}

		return plaintext, true, encoding, nil
	}
	return plaintext, false, encoding, nil
}

func gunzipLimited(data []byte, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, errors.New("decoded body limit must be greater than zero")
	}

	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	decoded, exceeded, readErr := readLimited(reader, limit)

	closeErr := reader.Close()
	if readErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close gzip reader: %w", closeErr)
	}

	if exceeded {
		return nil, fmt.Errorf("decoded body exceeds %d bytes", limit)
	}
	return decoded, nil
}

func responseFailureDetail(request *decodeResult, failure string) string {
	if request.responseEncrypted {
		return failure + "; request declared an encrypted response; showing raw response"
	}
	return failure + "; showing raw response"
}

type bodyDisplay struct {
	body       []byte
	structured bool
	meta       jsonDisplayMeta
	detail     string
}

func formatBody(header http.Header, body []byte, showSensitive bool, maxBodyBytes int64) bodyDisplay {
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultJSONDisplayLimit
	}

	if len(body) == 0 {
		return bodyDisplay{body: []byte{}}
	}

	if formatted, meta, err := formatJSONLimited(body, showSensitive, maxBodyBytes); err == nil {
		return bodyDisplay{body: formatted, structured: true, meta: meta}
	}

	if values, ok := parseForm(header, body); ok {
		formatted, meta, err := formatValuesForDisplay(values, showSensitive, maxBodyBytes)
		if err == nil {
			return bodyDisplay{body: formatted, structured: true, meta: meta}
		}
		return bodyDisplay{
			body:   safeBodyPlaceholder(maxBodyBytes, unsafeBodyPlaceholderReason),
			detail: "form body omitted because formatted output exceeds the display limit",
		}
	}

	if showSensitive {
		return bodyDisplay{body: append([]byte(nil), body...), detail: "unstructured body shown because --show-sensitive is enabled"}
	}
	return bodyDisplay{
		body:   safeBodyPlaceholder(maxBodyBytes, unsafeBodyPlaceholderReason),
		detail: "unstructured body omitted because it cannot be safely redacted",
	}
}

func formatQuery(values url.Values, showSensitive bool, maxBodyBytes int64) []byte {
	if len(values) == 0 {
		return []byte{}
	}

	formatted, _, err := formatValuesForDisplay(values, showSensitive, maxBodyBytes)
	if err != nil {
		return safeBodyPlaceholder(maxBodyBytes, "query omitted because it exceeds the display limit")
	}
	return formatted
}

func requestParameter(name string, query url.Values, header http.Header, body []byte) (string, bool) {
	if value, ok := firstValueFold(query, name); ok {
		return value, true
	}

	if values, ok := parseForm(header, body); ok {
		return firstValueFold(values, name)
	}

	if value, err := decodeJSON(body); err == nil {
		if object, ok := value.(map[string]any); ok {
			for key, raw := range object {
				if strings.EqualFold(key, name) {
					text, textOK := raw.(string)
					return text, textOK
				}
			}
		}
	}
	return "", false
}

func firstValueFold(values url.Values, name string) (string, bool) {
	for key, entries := range values {
		if strings.EqualFold(key, name) && len(entries) > 0 {
			return entries[0], true
		}
	}
	return "", false
}

func parseForm(header http.Header, body []byte) (url.Values, bool) {
	contentType := header.Get("Content-Type")

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil && contentType != "" {
		return nil, false
	}

	formContentType := strings.EqualFold(mediaType, "application/x-www-form-urlencoded")
	if !formContentType {
		// A declared non-form media type must not fall through to the loose
		// heuristic: malformed JSON containing '=' would otherwise be logged as
		// a form rather than failing closed.
		if mediaType != "" {
			return nil, false
		}

		if !bytes.Contains(body, []byte{'='}) || !isTextBody(body) {
			return nil, false
		}
	}

	values, err := url.ParseQuery(trimFormPadding(string(body)))
	if err != nil || len(values) == 0 {
		return nil, false
	}
	return values, true
}

func trimFormPadding(body string) string {
	percent := strings.LastIndexByte(body, '%')
	if percent == -1 || percent == len(body)-1 {
		return body
	}

	for i := percent + 1; i < len(body); i++ {
		if body[i] != '0' {
			return body
		}
	}
	return body[:percent]
}

func isTextBody(body []byte) bool {
	if !utf8.Valid(body) {
		return false
	}

	for _, r := range string(body) {
		if unicode.IsControl(r) && r != '\r' && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}

func methodAllowsBody(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func isHex(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed)%2 != 0 {
		return false
	}

	for _, c := range trimmed {
		if (c < '0' || c > '9') &&
			(c < 'a' || c > 'f') &&
			(c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func appendDetail(existing, detail string) string {
	if existing == "" {
		return detail
	}
	return existing + "; " + detail
}

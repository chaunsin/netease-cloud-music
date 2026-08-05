// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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
	decodeStatusPartial     decodeStatus = "partial"
	decodeStatusUnsupported decodeStatus = "unsupported"
	decodeStatusFailed      decodeStatus = "failed"
	decodeStatusRaw         decodeStatus = "raw"
)

// decodeResult keeps protocol decoding separate from request forwarding. Its body
// and query fields are log copies and must never be written back to the HTTP flow.
type decodeResult struct {
	protocol           protocol
	status             decodeStatus
	body               []byte
	query              []byte
	apiPath            string
	logicalMethod      string
	logicalContentType string
	keyVersion         string
	sessionID          *string
	keySource          string
	sFrameValid        bool
	detail             string
	responseEncrypted  bool
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

func decodeRequestLimitedWithXeapiSessions(method string, u *url.URL, header http.Header, body []byte, showSensitive bool, maxBodyBytes int64, sessions xeapiSessionLookup) decodeResult {
	if u == nil {
		u = &url.URL{}
	}

	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultJSONDisplayLimit
	}

	query := parseQueryForCapture(u.RawQuery)
	result := decodeResult{
		protocol:          classifyProtocol(u.Path),
		status:            decodeStatusPlaintext,
		apiPath:           u.Path,
		query:             formatQuery(query.display, showSensitive, maxBodyBytes),
		responseEncrypted: query.responseEncrypted,
	}

	switch result.protocol {
	case protocolEAPI:
		return decodeEAPIRequest(&result, query.parsed, header, body, showSensitive, maxBodyBytes)
	case protocolLinux:
		return decodeLinuxRequest(&result, query.parsed, header, body, showSensitive, maxBodyBytes)
	case protocolWEAPI:
		result.status = decodeStatusUnsupported
		result.detail = "weapi request decryption unsupported: the random AES key cannot be recovered"
	case protocolXEAPI:
		return decodeXEAPIRequest(&result, query, header, body, showSensitive, maxBodyBytes, sessions)
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

type xeapiRequestEnvelope struct {
	Body        *string `json:"body"`
	Method      string  `json:"method"`
	ContentType string  `json:"contentType"`
	QueryString string  `json:"queryString"`
}

type observedRequestParameter struct {
	value     string
	present   bool
	ambiguous bool
}

func (parameter *observedRequestParameter) add(value string, valid bool) {
	if !parameter.present {
		parameter.value = value
		parameter.present = true
		parameter.ambiguous = !valid
		return
	}

	if !valid || parameter.value != value {
		parameter.ambiguous = true
	}
}

type xeapiRequestParameters struct {
	b observedRequestParameter
	s observedRequestParameter
	r observedRequestParameter
}

func collectXEAPIRequestParameters(query url.Values, header http.Header, body []byte) xeapiRequestParameters {
	var parameters xeapiRequestParameters
	parameters.addValues(query)

	if values, ok := parseForm(header, body); ok {
		parameters.addValues(values)
	} else {
		parameters.addJSON(body)
	}
	return parameters
}

func (parameters *xeapiRequestParameters) addValues(values url.Values) {
	for name, entries := range values {
		parameter := parameters.named(name)
		if parameter == nil {
			continue
		}

		for _, entry := range entries {
			parameter.add(entry, true)
		}
	}
}

func (parameters *xeapiRequestParameters) addJSON(body []byte) {
	if !json.Valid(body) {
		return
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()

	object, ok := token.(json.Delim)
	if err != nil || !ok || object != '{' {
		return
	}

	for decoder.More() {
		nameToken, tokenErr := decoder.Token()

		name, nameOK := nameToken.(string)
		if tokenErr != nil || !nameOK {
			return
		}

		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return
		}

		parameter := parameters.named(name)
		if parameter == nil {
			continue
		}

		var value string

		valueErr := json.Unmarshal(raw, &value)
		parameter.add(value, valueErr == nil)
	}
}

func (parameters *xeapiRequestParameters) named(name string) *observedRequestParameter {
	switch {
	case strings.EqualFold(name, "B"):
		return &parameters.b
	case strings.EqualFold(name, "S"):
		return &parameters.s
	case strings.EqualFold(name, "R"):
		return &parameters.r
	default:
		return nil
	}
}

func decodeXEAPIRequest(
	base *decodeResult,
	query queryCapture,
	header http.Header,
	body []byte,
	showSensitive bool,
	maxBodyBytes int64,
	sessions xeapiSessionLookup,
) decodeResult {
	// XEAPI is complete only after every recoverable outer and logical field succeeds.
	base.status = decodeStatusPartial
	base.responseEncrypted = true
	base.query = formatXEAPIQuery(query.display, showSensitive, maxBodyBytes)

	outer := formatXEAPIOuterBody(header, body, showSensitive, maxBodyBytes)
	base.body = outer.body
	base.responseEncrypted = base.responseEncrypted || outer.meta.requestEncrypted
	base.detail = outer.detail

	parameters := collectXEAPIRequestParameters(query.parsed, header, body)
	if parameters.r.ambiguous {
		result := failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, "xeapi R field is ambiguous")
		result.status = decodeStatusPartial
		return result
	}

	encryptedR, hasR := parameters.r.value, parameters.r.present
	if !hasR || strings.TrimSpace(encryptedR) == "" {
		return failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, "xeapi R field is missing")
	}

	metadata, err := ncmcrypto.XeapiDecryptRequest(ncmcrypto.XeapiEncryptedRequest{R: encryptedR}, nil)
	if err != nil {
		return failedRequestFallback(base, header, body, showSensitive, maxBodyBytes, "xeapi R decrypt: "+err.Error())
	}

	base.keyVersion = redactDecodedMetadata(metadata.PublicKeyVersion, showSensitive)

	displaySessionID := metadata.SessionID
	if displaySessionID != "" && !showSensitive {
		displaySessionID = redactedValue
	}

	base.sessionID = &displaySessionID
	base.keySource = "unavailable"

	var (
		dynamicKey = []byte(nil)
		complete   = query.parseErr == nil
		details    = []string{"xeapi R decrypted"}
	)
	if query.parseErr != nil {
		details = append(details, "xeapi transport query is invalid: "+query.parseErr.Error())
	}

	switch {
	case metadata.SessionID == "":
		complete = false

		details = append(details, "session ID is empty; session key unavailable")
	case sessions != nil:
		key, source, ok := sessions.lookup(metadata.SessionID)
		if ok {
			dynamicKey = key
			base.keySource = source
			details = append(details, "session key matched source="+source)
		} else {
			complete = false

			details = append(details, "session key unavailable for recovered session ID")
		}
	default:
		complete = false

		details = append(details, "session key unavailable for recovered session ID")
	}

	encryptedB, hasB := parameters.b.value, parameters.b.present && !parameters.b.ambiguous
	switch {
	case parameters.b.ambiguous:
		encryptedB = ""
		complete = false

		details = append(details, "xeapi B field is ambiguous")
	case !hasB || strings.TrimSpace(encryptedB) == "":
		complete = false

		details = append(details, "xeapi B field is missing")
	}

	encryptedS, hasS := parameters.s.value, parameters.s.present && !parameters.s.ambiguous
	switch {
	case parameters.s.ambiguous:
		encryptedS = ""
		complete = false

		details = append(details, "xeapi S field is ambiguous")
	case !hasS || strings.TrimSpace(encryptedS) == "":
		complete = false

		details = append(details, "xeapi S field is missing")
	}

	decryptKey := dynamicKey
	if !hasB || strings.TrimSpace(encryptedB) == "" {
		decryptKey = nil
	}

	request, decryptErr := ncmcrypto.XeapiDecryptRequest(ncmcrypto.XeapiEncryptedRequest{
		B: encryptedB,
		S: encryptedS,
		R: encryptedR,
	}, decryptKey)
	base.sFrameValid = request.SFrameValid

	if decryptErr != nil {
		complete = false

		details = append(details, "xeapi recoverable field failed: "+decryptErr.Error())
	}

	if hasS && request.SFrameValid {
		details = append(details, "S frame validated but not decrypted because X25519 private material is unavailable")
	}

	if len(request.Plaintext) > 0 {
		if envelopeErr := applyXEAPIEnvelope(base, request.Plaintext, showSensitive, maxBodyBytes); envelopeErr != nil {
			complete = false

			details = append(details, "xeapi envelope: "+envelopeErr.Error())
		} else {
			details = append(details, "xeapi B decrypted and logical request restored")
		}
	} else if len(dynamicKey) > 0 && hasB {
		complete = false
	}

	base.detail = joinDetails(append([]string{base.detail}, details...)...)
	if complete {
		base.status = decodeStatusDecrypted
	}
	return *base
}

func applyXEAPIEnvelope(result *decodeResult, plaintext []byte, showSensitive bool, maxBodyBytes int64) error {
	trimmed := bytes.TrimSpace(plaintext)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return errors.New("plaintext is not a valid JSON object")
	}

	var envelope xeapiRequestEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	logicalPath, err := logicalXEAPIPath(result.apiPath, showSensitive)
	if err != nil {
		return err
	}

	result.apiPath = logicalPath

	method := strings.ToUpper(strings.TrimSpace(envelope.Method))
	if method == "" {
		method = http.MethodPost
	}

	if _, methodErr := http.NewRequest(method, "/", http.NoBody); methodErr != nil {
		return fmt.Errorf("method is invalid: %w", methodErr)
	}

	result.logicalMethod = redactDecodedMetadata(method, showSensitive)

	contentType := strings.TrimSpace(envelope.ContentType)
	if contentType == "" {
		contentType = "application/x-www-form-urlencoded;charset=utf-8"
	}

	if _, _, contentTypeErr := mime.ParseMediaType(contentType); contentTypeErr != nil {
		return fmt.Errorf("content type is invalid: %w", contentTypeErr)
	}

	result.logicalContentType = redactDecodedMetadata(contentType, showSensitive)

	logicalQuery := parseQueryForCapture(envelope.QueryString)
	result.query = formatXEAPIQuery(logicalQuery.display, showSensitive, maxBodyBytes)
	result.responseEncrypted = true

	var queryErr error
	if logicalQuery.parseErr != nil {
		queryErr = fmt.Errorf("query string is invalid: %w", logicalQuery.parseErr)
	}

	if envelope.Body == nil {
		result.body = []byte{}
		return queryErr
	}

	innerBody, err := base64.StdEncoding.Strict().DecodeString(*envelope.Body)
	if err != nil {
		return errors.Join(queryErr, fmt.Errorf("body base64 is invalid: %w", err))
	}

	display := formatBody(http.Header{"Content-Type": {contentType}}, innerBody, showSensitive, maxBodyBytes)
	result.body = display.body
	result.responseEncrypted = result.responseEncrypted || display.meta.requestEncrypted
	result.detail = appendDetail(result.detail, display.detail)
	return queryErr
}

func logicalXEAPIPath(transportPath string, showSensitive bool) (string, error) {
	lower := strings.ToLower(transportPath)

	var logical string

	switch {
	case lower == "/xeapi":
		logical = "/api"
	case strings.HasPrefix(lower, "/xeapi/"):
		logical = "/api/" + transportPath[len("/xeapi/"):]
	default:
		return "", errors.New("transport path does not start with /xeapi")
	}
	return sanitizeEAPIPath(logical, showSensitive)
}

func redactDecodedMetadata(value string, showSensitive bool) string {
	return string(redactDiagnostic([]byte(value), showSensitive))
}

func formatXEAPIQuery(values url.Values, showSensitive bool, maxBodyBytes int64) []byte {
	if showSensitive {
		return formatQuery(values, true, maxBodyBytes)
	}
	return formatQuery(redactXEAPIRValues(values), false, maxBodyBytes)
}

func formatXEAPIOuterBody(header http.Header, body []byte, showSensitive bool, maxBodyBytes int64) bodyDisplay {
	if showSensitive || len(body) == 0 {
		return formatBody(header, body, showSensitive, maxBodyBytes)
	}

	if redacted, ok := redactXEAPIJSONR(body); ok {
		return formatBody(header, redacted, false, maxBodyBytes)
	}

	if values, ok := parseForm(header, body); ok {
		return formatBody(header, []byte(redactXEAPIRValues(values).Encode()), false, maxBodyBytes)
	}
	return formatBody(header, body, false, maxBodyBytes)
}

func redactXEAPIRValues(values url.Values) url.Values {
	redacted := cloneURLValues(values)
	for name := range redacted {
		if strings.EqualFold(name, "R") {
			for i := range redacted[name] {
				redacted[name][i] = redactedValue
			}
		}
	}
	return redacted
}

func redactXEAPIJSONR(body []byte) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' || !utf8.Valid(trimmed) || !json.Valid(trimmed) {
		return nil, false
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, false
	}

	redacted := false

	for name := range object {
		if strings.EqualFold(name, "R") {
			object[name] = json.RawMessage(`"[REDACTED]"`)
			redacted = true
		}
	}

	if !redacted {
		return body, true
	}

	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, false
	}
	return encoded, true
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
				logicalQuery := parseQueryForCapture(parsed.RawQuery)

				base.apiPath = safePath
				base.query = formatQuery(logicalQuery.display, showSensitive, maxBodyBytes)
				base.responseEncrypted = base.responseEncrypted || logicalQuery.responseEncrypted
			}
		}
	}
	return *base
}

func failedRequestFallback(base *decodeResult, header http.Header, body []byte, showSensitive bool, maxBodyBytes int64, detail string) decodeResult {
	base.status = decodeStatusFailed

	display := formatBody(header, body, showSensitive, maxBodyBytes)
	if base.protocol == protocolXEAPI {
		display = formatXEAPIOuterBody(header, body, showSensitive, maxBodyBytes)
	}

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
		if request.protocol == protocolXEAPI {
			result.status = decodeStatusFailed
			result.detail = "empty XEAPI response body is neither plaintext JSON nor binary ciphertext"
		} else {
			result.detail = "empty response body"
		}
		return result
	}

	if request.protocol == protocolXEAPI && json.Valid(body) {
		display := formatBody(header, body, showSensitive, maxBodyBytes)
		result.body = display.body
		result.detail = appendDetail("plaintext JSON response", display.detail)
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
			result.setDecryptedResponse(header, plaintext, showSensitive, maxBodyBytes, "eapi encrypted response decrypted ("+encoding+")", gzipDecoded)
			return result
		}

		result.setFailedResponse(request, header, body, showSensitive, maxBodyBytes, "eapi response decrypt: "+err.Error())
		return result
	case protocolWEAPI:
		result.status = decodeStatusUnsupported
		display := formatBody(header, body, showSensitive, maxBodyBytes)
		result.body = display.body
		result.detail = appendDetail(string(request.protocol)+" response is not JSON; passive response decryption unsupported", display.detail)
		return result
	case protocolXEAPI:
		plaintext, gzipDecoded, err := decryptXEAPIResponse(body, maxBodyBytes)
		if err == nil {
			result.setDecryptedResponse(header, plaintext, showSensitive, maxBodyBytes, "xeapi binary response decrypted", gzipDecoded)
			return result
		}

		result.setFailedResponse(request, header, body, showSensitive, maxBodyBytes, "xeapi response decrypt: "+err.Error())
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
			result.setDecryptedResponse(header, plaintext, showSensitive, maxBodyBytes, "linux encrypted response decrypted", false)
			return result
		}

		result.setFailedResponse(request, header, body, showSensitive, maxBodyBytes, "linux response decrypt: "+err.Error())
		return result
	default:
		display := formatBody(header, body, showSensitive, maxBodyBytes)
		result.body = display.body
		result.status = decodeStatusRaw
		result.detail = appendDetail("response body is not JSON", display.detail)
		return result
	}
}

func (r *decodeResult) setDecryptedResponse(header http.Header, plaintext []byte, showSensitive bool, maxBodyBytes int64, detail string, gzipDecoded bool) {
	r.status = decodeStatusDecrypted
	r.responseEncrypted = true
	display := formatBody(header, plaintext, showSensitive, maxBodyBytes)
	r.body = display.body

	r.detail = appendDetail(detail, display.detail)
	if gzipDecoded {
		r.detail += "; inner gzip decoded"
	}
}

func (r *decodeResult) setFailedResponse(request *decodeResult, header http.Header, body []byte, showSensitive bool, maxBodyBytes int64, failure string) {
	r.status = decodeStatusFailed
	display := formatBody(header, body, showSensitive, maxBodyBytes)
	r.body = display.body
	r.detail = appendDetail(responseFailureDetail(request, failure), display.detail)
}

func decryptXEAPIResponse(body []byte, maxBodyBytes int64) ([]byte, bool, error) {
	if isHex(body) {
		return nil, false, errors.New("ASCII-hex XEAPI responses are unsupported")
	}

	plaintext, err := ncmcrypto.EApiDecrypt(string(body), "")
	if err != nil {
		return nil, false, err
	}

	plaintext, gzipDecoded, err := tryGunzip(plaintext, maxBodyBytes)
	if err != nil {
		return nil, false, err
	}

	if !json.Valid(plaintext) {
		return nil, false, errors.New("decrypted XEAPI response is not valid JSON")
	}
	return plaintext, gzipDecoded, nil
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

	plaintext, gzipDecoded, err := tryGunzip(plaintext, maxBodyBytes)
	if err != nil {
		return nil, false, "", err
	}
	return plaintext, gzipDecoded, encoding, nil
}

func tryGunzip(plaintext []byte, maxBodyBytes int64) ([]byte, bool, error) {
	if len(plaintext) >= 2 && plaintext[0] == 0x1f && plaintext[1] == 0x8b {
		decompressed, err := gunzipLimited(plaintext, maxBodyBytes)
		if err != nil {
			return nil, false, fmt.Errorf("inner gzip: %w", err)
		}
		return decompressed, true, nil
	}
	return plaintext, false, nil
}

func gunzipLimited(data []byte, limit int64) ([]byte, error) {
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
	if detail == "" {
		return existing
	}

	if existing == "" {
		return detail
	}
	return existing + "; " + detail
}

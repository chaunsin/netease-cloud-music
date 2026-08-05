// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	ncmcrypto "github.com/chaunsin/netease-cloud-music/pkg/crypto"
)

const eapiRequestGolden = "1BDAA66BB859333CCCE0A53AE6D1E6E61F5C1663DE05CFFB8C87BCE2FDC6F9ECAB1F5341B2FBCB5CBBACDA665D6F1A10B007189F44A13DB2463BB3EBF2639CF10A3E14D47E97975942FF626F17CE4A658E17F19C52EDACCB199F262EA09723E644C46E3880B4754AE1A2A1F4712268C52AEA6F5D0158780D82BDC30C930756181972480BE18A2ECD68A276C68E5214491F2323B3C87ECA2AF9532A4F483D55B8C5187D558AF5699D2C2437C1D98CB5AD7B90402CCDB12DF950521A86D854646BF8422708A649C1B8B752AF70AD5B3868F939FD0E9BEAA8BAE0D05BB0D4D88BE1A6BFAA8F5BBECD6F92368480E657D2200F8ACE7740ACAAA5634297D6661704EE7F74779E833DF2241939FC60C5D92569E31285E4F4A4F737CC8E89316DE7BBC8FB99E94B87DC05C190EA228637B2C0D182152BFAC603EF671A9A0B2F907D98F30E8A4614F236B3ED78392F039EDAD3C3CE5A856EE51BCDE2173F428CD1BB0239"

func TestClassifyProtocol(t *testing.T) {
	tests := []struct {
		path string
		want protocol
	}{
		{"/api/song/detail", protocolAPI},
		{"/weapi/login/cellphone", protocolWEAPI},
		{"/eapi/song/enhance/player/url", protocolEAPI},
		{"/api/linux/forward", protocolLinux},
		{"/api/linux/forward/v2", protocolLinux},
		{"/xeapi/song/detail", protocolXEAPI},
		{"https://music.163.com/EAPI/song?id=1", protocolEAPI},
		{"/eapievil/song", protocolGeneric},
		{"/assets/app.js", protocolGeneric},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := classifyProtocol(tt.path); got != tt.want {
				t.Fatalf("classifyProtocol(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDecodeEAPIRequestGolden(t *testing.T) {
	u := mustURL(t, "https://interface.music.163.com/eapi/music/partner/work/evaluate")
	body := []byte(url.Values{"params": {eapiRequestGolden}}.Encode())
	header := http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}

	result := decodeRequest(http.MethodPost, u, header, body)

	if result.protocol != protocolEAPI || result.status != decodeStatusDecrypted {
		t.Fatalf("unexpected result protocol/status: %+v", result)
	}

	if result.apiPath != "/api/music/partner/work/evaluate" {
		t.Fatalf("apiPath = %q", result.apiPath)
	}

	if !result.responseEncrypted {
		t.Fatal("e_r=true was not detected")
	}

	decoded, err := decodeJSON(result.body)
	if err != nil {
		t.Fatal(err)
	}

	object, ok := decoded.(map[string]any)
	if !ok {
		t.Fatalf("decoded EAPI body has type %T", decoded)
	}

	if got := object["taskId"]; got != "185640294" {
		t.Fatalf("taskId = %#v", got)
	}

	if !strings.Contains(result.detail, "digest verified") {
		t.Fatalf("detail = %q", result.detail)
	}
}

func TestMalformedQueryPlaceholderDoesNotOverrideEAPIBody(t *testing.T) {
	u := mustURL(t, "https://interface.music.163.com/eapi/music/partner/work/evaluate?params=%zz")
	body := []byte(url.Values{"params": {eapiRequestGolden}}.Encode())
	header := http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}

	result := decodeRequest(http.MethodPost, u, header, body)
	if result.status != decodeStatusDecrypted || !strings.Contains(result.detail, "digest verified") {
		t.Fatalf("malformed display query overrode the valid EAPI body: %+v", result)
	}

	if !bytes.Contains(result.query, []byte(redactedValue)) {
		t.Fatalf("malformed query field was not represented safely: %s", result.query)
	}
}

func TestParseEAPIEnvelopeUsesFirstAndLastSeparator(t *testing.T) {
	apiPath := "/api/test"
	payload := `{"value":"left-36cd479b6b5-right"}`
	digest := ncmcrypto.HexDigest("nobody" + apiPath + "use" + payload + "md5forencrypt")
	envelope := []byte(apiPath + eapiSeparator + payload + eapiSeparator + digest)

	gotPath, gotPayload, err := parseEAPIEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != apiPath || string(gotPayload) != payload {
		t.Fatalf("got path=%q payload=%q", gotPath, gotPayload)
	}

	badEnvelope := []byte(apiPath + eapiSeparator + payload + eapiSeparator + strings.Repeat("0", 32))
	if _, _, err = parseEAPIEnvelope(badEnvelope); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("expected digest mismatch, got %v", err)
	}
}

func TestDecodeEAPIRequestInvalidFallsBackToRawForm(t *testing.T) {
	u := mustURL(t, "https://interface.music.163.com/eapi/test")
	header := http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}
	body := []byte("params=not-hex&e_r=true&token=secret")

	result := decodeRequest(http.MethodPost, u, header, body)

	if result.status != decodeStatusFailed {
		t.Fatalf("status = %q, want failed", result.status)
	}

	if !result.responseEncrypted {
		t.Fatal("fallback lost the e_r response-encryption hint")
	}

	if !strings.Contains(string(result.body), `"params": "not-hex"`) {
		t.Fatalf("raw form was not retained: %s", result.body)
	}

	if strings.Contains(string(result.body), "secret") || !strings.Contains(string(result.body), redactedValue) {
		t.Fatalf("raw fallback was not redacted: %s", result.body)
	}
}

func TestParseFormTrimsNeteaseZeroPadding(t *testing.T) {
	header := http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}

	values, ok := parseForm(header, []byte("params=ABCDEF%000000"))
	if !ok {
		t.Fatal("padded form was not parsed")
	}

	if got := values.Get("params"); got != "ABCDEF" {
		t.Fatalf("params = %q, want ABCDEF", got)
	}

	values, ok = parseForm(header, []byte("value=100%25"))
	if !ok || values.Get("value") != "100%" {
		t.Fatalf("legitimate percent value changed: %#v", values)
	}
}

func TestParseFormRejectsMalformedDeclaredContentType(t *testing.T) {
	values, ok := parseForm(http.Header{"Content-Type": {"not a media type"}}, []byte("token=secret"))
	if ok || values != nil {
		t.Fatalf("malformed declared content type parsed as form: %#v", values)
	}
}

func TestDecodeRequestOnlyReportsEmptyBodyForBodyMethods(t *testing.T) {
	u := mustURL(t, "https://music.163.com/api/test")
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodDelete} {
		result := decodeRequest(method, u, nil, nil)
		if strings.Contains(result.detail, "empty request body") {
			t.Fatalf("%s request reported an unexpected empty body: %q", method, result.detail)
		}
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch} {
		result := decodeRequest(method, u, nil, nil)
		if !strings.Contains(result.detail, "empty request body") {
			t.Fatalf("%s request did not report an empty body: %q", method, result.detail)
		}
	}
}

func TestDecodeLinuxRequestAndResponse(t *testing.T) {
	payload := map[string]any{
		"method": "POST",
		"url":    "https://music.163.com/api/song/detail?id=12345&e_r=true&token=query-secret",
		"params": map[string]any{"ids": "[123]", "phone": "18800001111"},
	}

	encrypted, err := ncmcrypto.LinuxApiEncrypt(payload)
	if err != nil {
		t.Fatal(err)
	}

	body := []byte(url.Values{"eparams": {encrypted["eparams"]}}.Encode())
	header := http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}

	request := decodeRequest(http.MethodPost, mustURL(t, "https://music.163.com/api/linux/forward?eparams=%zz&transport=outer"), header, body)
	if request.status != decodeStatusDecrypted || request.apiPath != "/api/song/detail" {
		t.Fatalf("unexpected linux request: %+v", request)
	}

	logicalQuery, err := decodeJSON(request.query)
	if err != nil {
		t.Fatal(err)
	}

	query, ok := logicalQuery.(map[string]any)
	if !ok || query["id"] != "12345" || query["e_r"] != "true" || query["token"] != redactedValue {
		t.Fatalf("linux logical query was not restored safely: %s", request.query)
	}

	if _, ok := query["transport"]; ok {
		t.Fatalf("linux transport query leaked into logical query: %s", request.query)
	}

	if !request.responseEncrypted {
		t.Fatal("logical e_r=true was not detected")
	}

	if strings.Contains(string(request.body), "18800001111") || strings.Contains(string(request.body), "query-secret") {
		t.Fatalf("linux request leaked sensitive URL or body data: %s", request.body)
	}

	if !strings.Contains(string(request.body), url.QueryEscape(redactedValue)) {
		t.Fatalf("linux request URL did not retain a redacted value placeholder: %s", request.body)
	}

	responseCipher, err := ncmcrypto.LinuxApiEncrypt(map[string]any{"code": 200, "token": "secret"})
	if err != nil {
		t.Fatal(err)
	}

	response := decodeResponse(&request, nil, []byte(responseCipher["eparams"]), 1<<20, false)
	if response.status != decodeStatusDecrypted || !response.responseEncrypted {
		t.Fatalf("unexpected linux response: %+v", response)
	}

	if strings.Contains(string(response.body), "secret") || !strings.Contains(string(response.body), redactedValue) {
		t.Fatalf("linux response was not redacted: %s", response.body)
	}
}

func TestUnsupportedRequestsUseStructuredFallback(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		body   string
		header http.Header
		want   protocol
		status decodeStatus
	}{
		{
			name:   "weapi form",
			path:   "/weapi/login",
			body:   "params=ciphertext&encSecKey=rsa&csrf_token=csrf-secret",
			header: http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
			want:   protocolWEAPI,
			status: decodeStatusUnsupported,
		},
		{
			name:   "xeapi json",
			path:   "/xeapi/song/detail",
			body:   `{"B":"ciphertext","S":"signature","R":"nonce","header":"{\"MUSIC_U\":\"cookie-secret\"}"}`,
			header: http.Header{"Content-Type": {"application/json"}},
			want:   protocolXEAPI,
			status: decodeStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeRequest(http.MethodPost, mustURL(t, "https://music.163.com"+tt.path), tt.header, []byte(tt.body))
			if result.protocol != tt.want || result.status != tt.status {
				t.Fatalf("unexpected result: %+v", result)
			}

			if len(result.body) == 0 || !json.Valid(result.body) {
				t.Fatalf("fallback is not structured JSON: %s", result.body)
			}

			if strings.Contains(string(result.body), "secret") {
				t.Fatalf("sensitive value leaked: %s", result.body)
			}
		})
	}
}

func TestDecodeResponseJSONFirstAndEAPIFallback(t *testing.T) {
	request := decodeResult{
		protocol:          protocolEAPI,
		apiPath:           "/api/test",
		responseEncrypted: true,
	}

	plaintext := decodeResponse(&request, nil, []byte(`{"code":200,"token":"secret"}`), 1<<20, false)
	if plaintext.status != decodeStatusPlaintext || !plaintext.responseEncrypted {
		t.Fatalf("unexpected JSON-first result: %+v", plaintext)
	}

	if strings.Contains(string(plaintext.body), "secret") {
		t.Fatalf("plaintext response leaked token: %s", plaintext.body)
	}

	const responseGolden = "DCC52B3013E9B66C038F8E027E580ECEDF84E0F44CB93FC365BED7B646A9BC08"

	decrypted := decodeResponse(&request, nil, []byte(responseGolden), 1<<20, false)
	if decrypted.status != decodeStatusDecrypted || !decrypted.responseEncrypted {
		t.Fatalf("unexpected EAPI response: %+v", decrypted)
	}

	assertJSONNumber(t, decrypted.body, "code", "200")

	failed := decodeResponse(&request, nil, []byte("not encrypted"), 1<<20, false)
	if failed.status != decodeStatusFailed || strings.Contains(string(failed.body), "not encrypted") || !strings.Contains(string(failed.body), "unable to safely redact") {
		t.Fatalf("encrypted failure did not fail closed: %+v", failed)
	}
}

func TestDecodeEAPIResponseWithInnerGzip(t *testing.T) {
	var compressed bytes.Buffer

	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(`{"code":200,"password":"secret"}`)); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	ciphertext := encryptEAPIResponseForTest(t, compressed.Bytes())

	result := decodeResponse(&decodeResult{protocol: protocolEAPI, responseEncrypted: true}, nil, ciphertext, 1<<20, false)
	if result.status != decodeStatusDecrypted || !strings.Contains(result.detail, "inner gzip decoded") {
		t.Fatalf("unexpected gzip result: %+v", result)
	}

	if strings.Contains(string(result.body), "secret") || !json.Valid(result.body) {
		t.Fatalf("gzip response was not decoded/redacted: %s", result.body)
	}
}

func TestDecodeEAPIResponseInnerGzipHonorsBodyLimit(t *testing.T) {
	var compressed bytes.Buffer

	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(`{"value":"body larger than the configured limit"}`)); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	ciphertext := encryptEAPIResponseForTest(t, compressed.Bytes())

	result := decodeResponse(&decodeResult{protocol: protocolEAPI, responseEncrypted: true}, nil, ciphertext, 8, false)
	if result.status != decodeStatusFailed {
		t.Fatalf("status = %q, want failed", result.status)
	}

	if !strings.Contains(result.detail, "exceeds 8 bytes") {
		t.Fatalf("detail = %q", result.detail)
	}

	if bytes.Equal(result.body, ciphertext) || len(result.body) > 8 || len(result.body) == 0 {
		t.Fatalf("gzip limit failure did not fail closed: %q", result.body)
	}
}

func TestNonJSONWEAPIResponseRemainsUnsupported(t *testing.T) {
	result := decodeResponse(
		&decodeResult{protocol: protocolWEAPI, responseEncrypted: true},
		http.Header{"Content-Type": {"text/html"}},
		[]byte("<html>session-token-secret</html>"),
		1<<20,
		false,
	)
	if result.status != decodeStatusUnsupported {
		t.Fatalf("status = %q, want unsupported: %+v", result.status, result)
	}

	if strings.Contains(string(result.body), "session-token-secret") || strings.Contains(result.detail, "eapi response decrypt") {
		t.Fatalf("opaque response leaked or used static EAPI fallback: %+v", result)
	}
}

func TestSanitizeEAPIPathRejectsUnsafeOrNonEnvelopePaths(t *testing.T) {
	for _, value := range []string{
		"/api/test?token=path-secret",
		"/api/test#fragment",
		"/api/test\r\nforged",
		"https://music.163.com/api/test",
		"//music.163.com/api/test",
	} {
		if _, err := sanitizeEAPIPath(value, false); err == nil {
			t.Fatalf("unsafe EAPI path %q was accepted", value)
		}
	}

	path, err := sanitizeEAPIPath("/api/song/detail", false)
	if err != nil || path != "/api/song/detail" {
		t.Fatalf("safe EAPI path = %q, %v", path, err)
	}
}

func TestDecodeAPIFormatsQueryFormAndJSONNumbers(t *testing.T) {
	u := mustURL(t, "https://music.163.com/api/test?phone=18800001111&ids=1&ids=2")
	header := http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}
	body := []byte(`payload=%7B%22token%22%3A%22secret%22%2C%22id%22%3A9007199254740993%7D&MUSIC_R_U=renewal-cookie-secret&name=song`)

	result := decodeRequest(http.MethodPost, u, header, body)
	if result.protocol != protocolAPI || result.status != decodeStatusPlaintext {
		t.Fatalf("unexpected API result: %+v", result)
	}

	if !json.Valid(result.query) || !json.Valid(result.body) {
		t.Fatalf("query/body not formatted JSON: query=%s body=%s", result.query, result.body)
	}

	if strings.Contains(string(result.query), "18800001111") || strings.Contains(string(result.body), "secret") {
		t.Fatalf("formatted request leaked sensitive values: query=%s body=%s", result.query, result.body)
	}

	if !strings.Contains(string(result.body), "9007199254740993") {
		t.Fatalf("large JSON number was changed: %s", result.body)
	}
}

func TestDecodeRequestTracksNestedJSONStringEncryptionFlag(t *testing.T) {
	result := decodeRequest(
		http.MethodPost,
		mustURL(t, "https://music.163.com/api/test"),
		http.Header{"Content-Type": {"application/json"}},
		[]byte(`{"wrapper":"{\"e_r\":true}"}`),
	)
	if !result.responseEncrypted {
		t.Fatalf("nested JSON e_r flag was not retained: %+v", result)
	}
}

func TestDecodeXEAPIRequestIssue174GoldenMetadataAndFrames(t *testing.T) {
	const (
		goldenB = "J5+3SnVyE16Pm4720e7gA3mgIZ1L4axkB6jte8X079wgjs3SU+IK7AANKKdewVLtBIJw5y5LtyhCcJ3FZm4u2LOfXnKdOC0VKIfVgX/lWloAZX6hQGVaRHgnR3BdQi+t"
		goldenS = "B6N8vBQgk8i3VdwbEOhstCY3StFqqFPtC9/AsrhtHHwAAQIDBAUGBwgJCguNFV1OAc3Z5noM7bYwvLwNFBK0H8NY/JVdIRN2dRDdG1JrMTLDI/ArlqMSIXdq9rfulgMKqRO7imtYLn8PrI4cIbwOdSkz"
		goldenR = "3LCoCTuHo/mDfZ1x3PtHsQ=="
	)

	body := []byte(url.Values{"B": {goldenB}, "S": {goldenS}, "R": {goldenR}}.Encode())
	result := decodeRequestLimitedWithXeapiSessions(
		http.MethodPost,
		mustURL(t, "https://interface.music.163.com/xeapi/song/enhance/location/info"),
		http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
		body,
		false,
		1<<20,
		newXeapiSessionCache(nil),
	)

	if result.status != decodeStatusPartial || result.keyVersion != "1000000000000" {
		t.Fatalf("unexpected XEAPI golden result: %+v", result)
	}

	if result.sessionID == nil || *result.sessionID != "" || !result.sFrameValid {
		t.Fatalf("XEAPI R/S metadata was not recovered: %+v", result)
	}

	if !strings.Contains(result.detail, "X25519 private material is unavailable") {
		t.Fatalf("S boundary missing from detail: %q", result.detail)
	}
}

func TestDecodeXEAPIRequestReferenceVectorWithSyntheticSession(t *testing.T) {
	const (
		sessionID  = "proxy-reference-session"
		sessionKey = "reference-key-01"
		goldenB    = "4pQjeV8mg0tTIXezMVF7VJIqZBIhkb+rcjSmutdxZJ1su8eA4087uEnhgd+8+hZ9MrWsOB6eQhSArenm7vNbSFlnAZ5UzNnpsOU7gPlhFdzaJVRo0oeBR4E+8ZAlFErpmaI20sFIcwT1WArOuuHtdrD5wfPfHoCzQN/5QI1zp8mVLwoA3m1DaskKW02cCXhBke1AteDRcKe8usMRZwEVaYeU+UTr8Tp7F3cUfH4eG3nNiX6MJrD4TH88j5woRKnfRmBJFFBSH1yBkB7DjLPM8rLzcdgz6vmF6FBHQ1v7CU80Nmol7hoR1pMa6943NVU76qtbo8Gd1gqq6VNGs9S+IMQPJvbHfNJiM6hDbg6SQ6VToPXSVilr/XWLAx68uhfSalIGbAMDIeeVnPaiCCQOcywaBC8OArX5mgcQTnUYQjgEsEkGKKSm6E3YlUZjUxyAzKEQfz0dX9uJxM8I43jJyQ=="
		goldenS    = "B6N8vBQgk8i3VdwbEOhstCY3StFqqFPtC9/AsrhtHHwAAQIDBAUGBwgJCguvOk5KFqXB3EIPxpMRirlWCw+3HaF+/JVdIRN2dRDdG1JrMTLDI/ArlqMSIXdq9rfulgMKqRPzefUXdWGfB1rO3sPjOZbO"
		goldenR    = "1ayzaxtVlOipqMIqu///IHIds84B7RUefq+DxuHxV6E4VetqOOw4Iwej1EimjTAE"
	)

	// B/S/R are deterministic outputs from docs/xeapi.md's compatibility reference
	// using a synthetic session and fixed randomness. Hardcoding them avoids both
	// captured credentials and a Go encrypt/decrypt self-consistency test.
	body := []byte(url.Values{"B": {goldenB}, "S": {goldenS}, "R": {goldenR}}.Encode())
	cache := newXeapiSessionCache([]XeapiSessionSeed{{
		ID: sessionID, Key: sessionKey, Source: XeapiSessionSourceCommandLine,
	}})
	result := decodeRequestLimitedWithXeapiSessions(
		http.MethodPost,
		mustURL(t, "https://interface.music.163.com/xeapi/song/detail"),
		http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
		body,
		false,
		1<<20,
		cache,
	)

	if result.status != decodeStatusDecrypted || result.keyVersion != "1000000000000" {
		t.Fatalf("unexpected XEAPI reference result: %+v", result)
	}

	if result.sessionID == nil || *result.sessionID != redactedValue || result.keySource != XeapiSessionSourceCommandLine || !result.sFrameValid {
		t.Fatalf("session metadata was not safely reported: %+v", result)
	}

	if result.apiPath != "/api/song/detail" || result.logicalMethod != http.MethodGet || result.logicalContentType != "application/json" {
		t.Fatalf("logical request metadata was not restored: %+v", result)
	}

	if strings.Contains(string(result.query), "query-secret") || !strings.Contains(string(result.query), redactedValue) {
		t.Fatalf("logical query was not redacted: %s", result.query)
	}

	if strings.Contains(string(result.body), "inner-session") || strings.Contains(string(result.body), "inner-secret") || !strings.Contains(string(result.body), redactedValue) {
		t.Fatalf("logical body was not safely redacted: %s", result.body)
	}

	assertJSONNumber(t, result.body, "id", "9007199254740993")
}

func TestDecodeXEAPIRequestWithSessionRestoresLogicalRequest(t *testing.T) {
	const (
		sessionID  = "session-id"
		sessionKey = "0123456789abcdef"
	)

	params := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
		URI:         "/api/song/detail?id=1&token=query-secret",
		Method:      http.MethodGet,
		ContentType: "application/json",
		Body:        []byte(`{"sessionId":"inner-session","nested":{"token":"inner-secret"},"id":9007199254740993}`),
	}, ncmcrypto.XeapiSession{ID: sessionID, Key: sessionKey})

	cache := newXeapiSessionCache([]XeapiSessionSeed{{
		ID: sessionID, Key: sessionKey, Source: XeapiSessionSourceCommandLine,
	}})
	result := decodeRequestLimitedWithXeapiSessions(
		http.MethodPost,
		mustURL(t, "https://interface.music.163.com/xeapi/song/detail"),
		http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
		[]byte(params.Encode()),
		false,
		1<<20,
		cache,
	)

	if result.status != decodeStatusDecrypted {
		t.Fatalf("XEAPI request was not decrypted: %+v", result)
	}

	if result.apiPath != "/api/song/detail" || result.logicalMethod != http.MethodGet || result.logicalContentType != "application/json" {
		t.Fatalf("logical request metadata was not restored: %+v", result)
	}

	if result.sessionID == nil || *result.sessionID != redactedValue || result.keySource != XeapiSessionSourceCommandLine || !result.sFrameValid {
		t.Fatalf("session metadata was not safely reported: %+v", result)
	}

	if strings.Contains(string(result.query), "query-secret") || !strings.Contains(string(result.query), redactedValue) {
		t.Fatalf("logical query was not redacted: %s", result.query)
	}

	if strings.Contains(string(result.body), "inner-session") || strings.Contains(string(result.body), "inner-secret") {
		t.Fatalf("logical body leaked nested sensitive values: %s", result.body)
	}

	assertJSONNumber(t, result.body, "id", "9007199254740993")

	visible := decodeRequestLimitedWithXeapiSessions(
		http.MethodPost,
		mustURL(t, "https://interface.music.163.com/xeapi/song/detail"),
		http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
		[]byte(params.Encode()), true, 1<<20, cache,
	)
	if visible.sessionID == nil || *visible.sessionID != sessionID || !strings.Contains(string(visible.body), "inner-secret") {
		t.Fatalf("--show-sensitive boundary was not honored: %+v", visible)
	}
}

func TestDecodeXEAPIRequestPartialAndFailedBoundaries(t *testing.T) {
	const (
		sessionID  = "session-id"
		sessionKey = "0123456789abcdef"
	)

	valid := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
		URI:  "/api/test?id=1",
		Body: []byte{},
	}, ncmcrypto.XeapiSession{ID: sessionID, Key: sessionKey})

	tests := []struct {
		name       string
		mutate     func(url.Values)
		cache      *xeapiSessionCache
		want       decodeStatus
		wantDetail string
	}{
		{name: "unknown session", cache: newXeapiSessionCache(nil), want: decodeStatusPartial, wantDetail: "session key unavailable"},
		{name: "wrong key", cache: newXeapiSessionCache([]XeapiSessionSeed{{ID: sessionID, Key: "abcdef0123456789", Source: "wrong"}}), want: decodeStatusPartial, wantDetail: "decrypt B"},
		{name: "missing B", mutate: func(v url.Values) { v.Del("B") }, cache: xeapiTestCache(), want: decodeStatusPartial, wantDetail: "B field is missing"},
		{name: "missing S", mutate: func(v url.Values) { v.Del("S") }, cache: xeapiTestCache(), want: decodeStatusPartial, wantDetail: "S field is missing"},
		{name: "missing R", mutate: func(v url.Values) { v.Del("R") }, cache: xeapiTestCache(), want: decodeStatusFailed, wantDetail: "R field is missing"},
		{name: "malformed R", mutate: func(v url.Values) { v.Set("R", "%%") }, cache: xeapiTestCache(), want: decodeStatusFailed, wantDetail: "R decrypt"},
		{name: "malformed B", mutate: func(v url.Values) { v.Set("B", "AA==") }, cache: xeapiTestCache(), want: decodeStatusPartial, wantDetail: "decrypt B"},
		{name: "malformed S", mutate: func(v url.Values) { v.Set("S", "AA==") }, cache: xeapiTestCache(), want: decodeStatusPartial, wantDetail: "validate S"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := make(url.Values, len(valid))
			for key, entries := range valid {
				values[key] = append([]string(nil), entries...)
			}

			if tt.mutate != nil {
				tt.mutate(values)
			}

			result := decodeRequestLimitedWithXeapiSessions(
				http.MethodPost,
				mustURL(t, "https://interface.music.163.com/xeapi/test"),
				http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
				[]byte(values.Encode()), false, 1<<20, tt.cache,
			)
			if result.status != tt.want || !strings.Contains(result.detail, tt.wantDetail) {
				t.Fatalf("result = %+v, want status=%s detail containing %q", result, tt.want, tt.wantDetail)
			}
		})
	}
}

func TestDecodeXEAPIRequestExtractsQueryAndJSONFallback(t *testing.T) {
	const (
		sessionID  = "session-id"
		sessionKey = "0123456789abcdef"
	)

	params := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
		URI:  "/api/test?id=1",
		Body: []byte{},
	}, ncmcrypto.XeapiSession{ID: sessionID, Key: sessionKey})
	cache := xeapiTestCache()

	queryURL := mustURL(t, "https://interface.music.163.com/xeapi/test?"+params.Encode())

	fromQuery := decodeRequestLimitedWithXeapiSessions(http.MethodGet, queryURL, nil, nil, false, 1<<20, cache)
	if fromQuery.status != decodeStatusDecrypted {
		t.Fatalf("query XEAPI fields were not decrypted: %+v", fromQuery)
	}

	jsonBody, err := json.Marshal(map[string]string{
		"B": params.Get("B"), "S": params.Get("S"), "R": params.Get("R"),
	})
	if err != nil {
		t.Fatal(err)
	}

	fromJSON := decodeRequestLimitedWithXeapiSessions(
		http.MethodPost,
		mustURL(t, "https://interface.music.163.com/xeapi/test"),
		http.Header{"Content-Type": {"application/json"}},
		jsonBody, false, 1<<20, cache,
	)
	if fromJSON.status != decodeStatusDecrypted {
		t.Fatalf("JSON fallback XEAPI fields were not decrypted: %+v", fromJSON)
	}
}

func TestDecodeXEAPIRequestRejectsAmbiguousOuterFields(t *testing.T) {
	const (
		sessionID  = "session-id"
		sessionKey = "0123456789abcdef"
	)

	valid := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
		URI: "/api/test?id=1",
	}, ncmcrypto.XeapiSession{ID: sessionID, Key: sessionKey})

	cloneValid := func() url.Values {
		cloned := make(url.Values, len(valid))
		for key, entries := range valid {
			cloned[key] = append([]string(nil), entries...)
		}
		return cloned
	}
	decode := func(query url.Values, header http.Header, body []byte) decodeResult {
		requestURL := mustURL(t, "https://interface.music.163.com/xeapi/test")
		requestURL.RawQuery = query.Encode()
		return decodeRequestLimitedWithXeapiSessions(
			http.MethodPost, requestURL, header, body, false, 1<<20, xeapiTestCache(),
		)
	}
	assertPartialMetadata := func(t *testing.T, result decodeResult, field, original, conflicting string) {
		t.Helper()

		if result.status != decodeStatusPartial || !strings.Contains(result.detail, "xeapi "+field+" field is ambiguous") {
			t.Fatalf("ambiguous %s result = %+v", field, result)
		}

		if strings.Contains(result.detail, original) || strings.Contains(result.detail, conflicting) {
			t.Fatalf("ambiguous %s detail exposed a candidate value: %q", field, result.detail)
		}
	}

	t.Run("query and form R conflict", func(t *testing.T) {
		const conflicting = "conflicting-r-secret"

		form := cloneValid()
		form.Set("R", conflicting)
		result := decode(
			url.Values{"R": {valid.Get("R")}},
			http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
			[]byte(form.Encode()),
		)
		assertPartialMetadata(t, result, "R", valid.Get("R"), conflicting)
	})

	t.Run("duplicate B conflict", func(t *testing.T) {
		const conflicting = "conflicting-b-secret"

		form := cloneValid()
		form["B"] = append(form["B"], conflicting)
		result := decode(nil, http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}, []byte(form.Encode()))
		assertPartialMetadata(t, result, "B", valid.Get("B"), conflicting)

		if result.sessionID == nil || *result.sessionID != redactedValue || result.keySource != XeapiSessionSourceCommandLine {
			t.Fatalf("ambiguous B discarded recovered session metadata: %+v", result)
		}
	})

	t.Run("case-duplicate JSON S conflict", func(t *testing.T) {
		const conflicting = "conflicting-s-secret"

		body, err := json.Marshal(map[string]string{
			"B": valid.Get("B"),
			"R": valid.Get("R"),
			"S": valid.Get("S"),
			"s": conflicting,
		})
		if err != nil {
			t.Fatal(err)
		}

		result := decode(nil, http.Header{"Content-Type": {"application/json"}}, body)
		assertPartialMetadata(t, result, "S", valid.Get("S"), conflicting)

		if result.sessionID == nil || *result.sessionID != redactedValue || result.keySource != XeapiSessionSourceCommandLine {
			t.Fatalf("ambiguous S discarded recovered session metadata: %+v", result)
		}

		if result.sFrameValid {
			t.Fatalf("ambiguous S was incorrectly reported as validated: %+v", result)
		}
	})

	t.Run("identical duplicates are accepted", func(t *testing.T) {
		form := cloneValid()
		for _, field := range []string{"B", "S", "R"} {
			form[field] = append(form[field], form.Get(field))
		}

		result := decode(nil, http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}, []byte(form.Encode()))
		if result.status != decodeStatusDecrypted || strings.Contains(result.detail, "ambiguous") {
			t.Fatalf("identical duplicates were rejected: %+v", result)
		}
	})
}

func TestDecodeXEAPIRequestRedactsOuterRBySource(t *testing.T) {
	const (
		sessionID  = "session-id"
		sessionKey = "0123456789abcdef"
	)

	params := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
		URI: "/api/test?id=1",
	}, ncmcrypto.XeapiSession{ID: sessionID, Key: sessionKey})
	params.Del("B")
	rawR := params.Get("R")

	queryURL := mustURL(t, "https://interface.music.163.com/xeapi/test")
	queryURL.RawQuery = params.Encode()

	jsonBody, err := json.Marshal(map[string]string{"R": rawR, "S": params.Get("S")})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		u       *url.URL
		header  http.Header
		body    []byte
		display func(decodeResult) []byte
	}{
		{name: "query", u: queryURL, display: func(result decodeResult) []byte { return result.query }},
		{
			name:   "form",
			u:      mustURL(t, "https://interface.music.163.com/xeapi/test"),
			header: http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
			body:   []byte(params.Encode()),
			display: func(result decodeResult) []byte {
				return result.body
			},
		},
		{
			name:   "JSON",
			u:      mustURL(t, "https://interface.music.163.com/xeapi/test"),
			header: http.Header{"Content-Type": {"application/json"}},
			body:   jsonBody,
			display: func(result decodeResult) []byte {
				return result.body
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hidden := decodeRequestLimitedWithXeapiSessions(
				http.MethodPost, tt.u, tt.header, tt.body, false, 1<<20, xeapiTestCache(),
			)

			hiddenDisplay := tt.display(hidden)
			if hidden.status != decodeStatusPartial || hidden.sessionID == nil || *hidden.sessionID != redactedValue {
				t.Fatalf("default XEAPI result did not retain redacted metadata: %+v", hidden)
			}

			if bytes.Contains(hiddenDisplay, []byte(rawR)) || !bytes.Contains(hiddenDisplay, []byte("[REDACTED")) {
				t.Fatalf("default XEAPI %s exposed R: %s", tt.name, hiddenDisplay)
			}

			visible := decodeRequestLimitedWithXeapiSessions(
				http.MethodPost, tt.u, tt.header, tt.body, true, 1<<20, xeapiTestCache(),
			)
			if visible.status != decodeStatusPartial || visible.sessionID == nil || *visible.sessionID != sessionID {
				t.Fatalf("sensitive XEAPI result lost recovered metadata: %+v", visible)
			}

			if !bytes.Contains(tt.display(visible), []byte(rawR)) {
				t.Fatalf("--show-sensitive did not retain XEAPI %s R: %s", tt.name, tt.display(visible))
			}
		})
	}
}

func TestFormatXEAPIOuterBodyRejectsInvalidUTF8JSON(t *testing.T) {
	body := []byte(`{"R":"outer-secret","bad`)
	body = append(body, 0xff)
	body = append(body, []byte(`":"private-value"}`)...)
	header := http.Header{"Content-Type": {"application/json"}}

	hidden := formatXEAPIOuterBody(header, body, false, 1024)
	if hidden.structured || bytes.Contains(hidden.body, []byte("private-value")) || !bytes.Contains(hidden.body, []byte(unsafeBodyPlaceholderReason)) {
		t.Fatalf("invalid UTF-8 XEAPI JSON did not fail closed: %+v", hidden)
	}

	visible := formatXEAPIOuterBody(header, body, true, 1024)
	if !bytes.Equal(visible.body, body) {
		t.Fatalf("--show-sensitive changed invalid UTF-8 XEAPI JSON: %q", visible.body)
	}
}

func TestMalformedXEAPIQueryDoesNotBecomeSyntheticParameter(t *testing.T) {
	params := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
		URI: "/api/test?name=song",
	}, ncmcrypto.XeapiSession{ID: "session-id", Key: "0123456789abcdef"})

	result := decodeRequestLimitedWithXeapiSessions(
		http.MethodPost,
		mustURL(t, "https://interface.music.163.com/xeapi/test?R=%zz"),
		http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
		[]byte(params.Encode()),
		false,
		1<<20,
		xeapiTestCache(),
	)

	if result.status != decodeStatusPartial || result.apiPath != "/api/test" {
		t.Fatalf("malformed transport query blocked XEAPI body recovery: %+v", result)
	}

	if !strings.Contains(result.detail, "transport query is invalid") || strings.Contains(result.detail, "ambiguous") {
		t.Fatalf("malformed transport query produced the wrong diagnostic: %q", result.detail)
	}

	if !bytes.Contains(result.query, []byte(`"name": "song"`)) {
		t.Fatalf("logical query was not restored from the valid body: %s", result.query)
	}
}

func TestDecodeXEAPIRequestRedactsLogicalMetadata(t *testing.T) {
	const (
		sessionID         = "session-id"
		sessionKey        = "0123456789abcdef"
		contentTypeSecret = "content-type-secret"
		logicalRSecret    = "logical-r-secret"
	)

	params := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
		URI:         "/api/test?R=" + logicalRSecret + "&name=song",
		Method:      http.MethodGet,
		ContentType: "application/json; token=" + contentTypeSecret,
		Body:        []byte(`{"code":200}`),
	}, ncmcrypto.XeapiSession{ID: sessionID, Key: sessionKey})
	decode := func(showSensitive bool) decodeResult {
		return decodeRequestLimitedWithXeapiSessions(
			http.MethodPost,
			mustURL(t, "https://interface.music.163.com/xeapi/test"),
			http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
			[]byte(params.Encode()), showSensitive, 1<<20, xeapiTestCache(),
		)
	}

	hidden := decode(false)
	if hidden.status != decodeStatusDecrypted {
		t.Fatalf("XEAPI logical request was not decrypted: %+v", hidden)
	}

	if strings.Contains(hidden.logicalContentType, contentTypeSecret) || !strings.Contains(hidden.logicalContentType, redactedValue) {
		t.Fatalf("logical content type was not redacted: %q", hidden.logicalContentType)
	}

	if bytes.Contains(hidden.query, []byte(logicalRSecret)) || !bytes.Contains(hidden.query, []byte("[REDACTED")) {
		t.Fatalf("logical XEAPI R was not redacted: %s", hidden.query)
	}

	visible := decode(true)
	if visible.status != decodeStatusDecrypted || !strings.Contains(visible.logicalContentType, contentTypeSecret) || !bytes.Contains(visible.query, []byte(logicalRSecret)) {
		t.Fatalf("--show-sensitive did not retain logical metadata: %+v", visible)
	}
}

func TestDecodeXEAPIRequestRetainsMalformedLogicalQuery(t *testing.T) {
	params := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
		URI:         "/api/test?name=song&note=private-secret;token=hidden-secret",
		ContentType: "application/json",
		Body:        []byte(`{"name":"visible","token":"body-secret"}`),
	}, ncmcrypto.XeapiSession{ID: "session-id", Key: "0123456789abcdef"})

	result := decodeRequestLimitedWithXeapiSessions(
		http.MethodPost,
		mustURL(t, "https://interface.music.163.com/xeapi/test"),
		http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
		[]byte(params.Encode()),
		false,
		1<<20,
		xeapiTestCache(),
	)

	if result.status != decodeStatusPartial || !strings.Contains(result.detail, "query string is invalid") {
		t.Fatalf("malformed logical query did not remain partial: %+v", result)
	}

	decoded, err := decodeJSON(result.query)
	if err != nil {
		t.Fatal(err)
	}

	query, ok := decoded.(map[string]any)
	if !ok || query["name"] != "song" || query["note"] != redactedValue || query["e_r"] != "true" {
		t.Fatalf("malformed logical query lost safe fields: %s", result.query)
	}

	for _, secret := range []string{"private-secret", "hidden-secret"} {
		if bytes.Contains(result.query, []byte(secret)) {
			t.Fatalf("malformed logical query exposed %q: %s", secret, result.query)
		}
	}

	body, err := decodeJSON(result.body)
	if err != nil {
		t.Fatal(err)
	}

	object, ok := body.(map[string]any)
	if !ok || object["name"] != "visible" || object["token"] != redactedValue {
		t.Fatalf("malformed logical query blocked independent body recovery: %s", result.body)
	}
}

func TestDecodeXEAPIRequestInvalidEnvelopeAndUnstructuredInnerBody(t *testing.T) {
	const (
		sessionID  = "session-id"
		sessionKey = "0123456789abcdef"
	)

	cache := xeapiTestCache()

	t.Run("invalid content type envelope is partial", func(t *testing.T) {
		params := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
			URI:         "/api/test",
			ContentType: "not a media type",
			Body:        []byte{},
		}, ncmcrypto.XeapiSession{ID: sessionID, Key: sessionKey})

		result := decodeRequestLimitedWithXeapiSessions(
			http.MethodPost,
			mustURL(t, "https://interface.music.163.com/xeapi/test"),
			http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
			[]byte(params.Encode()),
			false,
			1<<20,
			cache,
		)
		if result.status != decodeStatusPartial || !strings.Contains(result.detail, "content type is invalid") {
			t.Fatalf("unexpected invalid envelope result: %+v", result)
		}
	})

	t.Run("invalid UTF-8 body fails closed after decrypt", func(t *testing.T) {
		params := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
			URI:         "/api/test",
			ContentType: "application/octet-stream",
			Body:        []byte{0xff, 0xfe, 0xfd},
		}, ncmcrypto.XeapiSession{ID: sessionID, Key: sessionKey})

		result := decodeRequestLimitedWithXeapiSessions(
			http.MethodPost,
			mustURL(t, "https://interface.music.163.com/xeapi/test"),
			http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
			[]byte(params.Encode()),
			false,
			1<<20,
			cache,
		)
		if result.status != decodeStatusDecrypted || !bytes.Contains(result.body, []byte("unable to safely redact")) {
			t.Fatalf("invalid UTF-8 body did not fail closed: %+v", result)
		}
	})
}

func TestDecodeXEAPIResponseEmptyFails(t *testing.T) {
	request := decodeResult{protocol: protocolXEAPI, responseEncrypted: true}
	result := decodeResponse(&request, nil, nil, 1<<20, false)

	if result.status != decodeStatusFailed || !strings.Contains(result.detail, "empty XEAPI response body") {
		t.Fatalf("empty XEAPI response was misclassified: %+v", result)
	}
}

func TestDecodeXEAPIResponseBinaryOnly(t *testing.T) {
	request := decodeResult{protocol: protocolXEAPI, responseEncrypted: true}

	plaintext := decodeResponse(&request, nil, []byte(`{"code":200,"token":"secret"}`), 1<<20, false)
	if plaintext.status != decodeStatusPlaintext || strings.Contains(string(plaintext.body), "secret") {
		t.Fatalf("plaintext XEAPI response was not handled safely: %+v", plaintext)
	}

	compact := []byte(`{"value":"12345"}`)

	limitedPlaintext := decodeResponse(&request, nil, compact, int64(len(compact)), false)
	if limitedPlaintext.status != decodeStatusPlaintext {
		t.Fatalf("valid plaintext JSON was misclassified at the display boundary: %+v", limitedPlaintext)
	}

	const responseHex = "BCC6C3A838364F78C6613EF403862326D0CB333FB97328516FB0C72CD7DB1B8E6AA3B102FBE7296AB0DB9EA5C46AD12B"

	binary, err := hex.DecodeString(responseHex)
	if err != nil {
		t.Fatal(err)
	}

	decrypted := decodeResponse(&request, nil, binary, 1<<20, false)
	if decrypted.status != decodeStatusDecrypted {
		t.Fatalf("binary XEAPI response was not decrypted: %+v", decrypted)
	}

	assertJSONNumber(t, decrypted.body, "code", "200")

	hexText := decodeResponse(&request, nil, []byte(responseHex), 1<<20, false)
	if hexText.status != decodeStatusFailed || !strings.Contains(hexText.detail, "ASCII-hex XEAPI responses are unsupported") {
		t.Fatalf("ASCII hex XEAPI response was guessed as compatible: %+v", hexText)
	}

	nonJSON := decodeResponse(&request, nil, encryptEAPIResponseForTest(t, []byte("not JSON")), 1<<20, false)
	if nonJSON.status != decodeStatusFailed || !strings.Contains(nonJSON.detail, "not valid JSON") {
		t.Fatalf("non-JSON plaintext was accepted after XEAPI decryption: %+v", nonJSON)
	}
}

func TestDecodeXEAPIResponseInnerGzipHonorsLimit(t *testing.T) {
	var compressed bytes.Buffer

	writer := gzip.NewWriter(&compressed)

	_, err := writer.Write([]byte(`{"value":"body larger than the configured limit"}`))
	if err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	ciphertext := encryptEAPIResponseForTest(t, compressed.Bytes())
	request := decodeResult{protocol: protocolXEAPI, responseEncrypted: true}

	decoded := decodeResponse(&request, nil, ciphertext, 1<<20, false)
	if decoded.status != decodeStatusDecrypted || !strings.Contains(decoded.detail, "inner gzip decoded") {
		t.Fatalf("XEAPI gzip response was not decoded: %+v", decoded)
	}

	overLimit := decodeResponse(&request, nil, ciphertext, 8, false)
	if overLimit.status != decodeStatusFailed || !strings.Contains(overLimit.detail, "exceeds 8 bytes") {
		t.Fatalf("XEAPI gzip response ignored limit: %+v", overLimit)
	}
}

func encryptXEAPIRequestForProxy(t *testing.T, request *ncmcrypto.XeapiEncryptRequest, session ncmcrypto.XeapiSession) url.Values {
	t.Helper()

	params, err := ncmcrypto.XeapiEncrypt(request, ncmcrypto.XeapiPublicKeyState{
		PublicKey: "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
		Version:   "1000000000000",
		SK:        "server-key",
	}, session)
	if err != nil {
		t.Fatal(err)
	}
	return url.Values{"B": {params["B"]}, "S": {params["S"]}, "R": {params["R"]}}
}

func xeapiTestCache() *xeapiSessionCache {
	return newXeapiSessionCache([]XeapiSessionSeed{{ID: "session-id", Key: "0123456789abcdef", Source: XeapiSessionSourceCommandLine}})
}

func assertJSONNumber(t *testing.T, data []byte, key, want string) {
	t.Helper()

	value, err := decodeJSON(data)
	if err != nil {
		t.Fatalf("decode JSON: %v: %s", err, data)
	}

	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("JSON is not an object: %T", value)
	}

	got, ok := object[key].(json.Number)
	if !ok || got.String() != want {
		t.Fatalf("%s = %#v, want json.Number(%s)", key, object[key], want)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func decodeRequest(method string, u *url.URL, header http.Header, body []byte) decodeResult {
	return decodeRequestLimitedWithXeapiSessions(method, u, header, body, false, defaultJSONDisplayLimit, nil)
}

func encryptEAPIResponseForTest(t *testing.T, plaintext []byte) []byte {
	t.Helper()

	block, err := aes.NewCipher([]byte("e82ckenh8dichen8"))
	if err != nil {
		t.Fatal(err)
	}

	padded, err := ncmcrypto.Pkcs7Padding(plaintext, block.BlockSize())
	if err != nil {
		t.Fatal(err)
	}
	return ncmcrypto.AesEncryptECB(block, padded)
}

func TestIsHex(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "00aAFF", want: true},
		{input: " 00aa\n", want: true},
		{input: "0"},
		{input: "x0"},
		{},
	}
	for _, test := range tests {
		input, want := test.input, test.want
		if got := isHex([]byte(input)); got != want {
			t.Errorf("isHex(%q) = %v, want %v", input, got, want)
		}
	}

	if _, err := hex.DecodeString("00aAFF"); err != nil {
		t.Fatal(err)
	}
}

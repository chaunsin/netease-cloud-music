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
		"url":    "https://music.163.com/api/song/detail",
		"params": map[string]any{"ids": "[123]", "e_r": true, "phone": "18800001111"},
	}

	encrypted, err := ncmcrypto.LinuxApiEncrypt(payload)
	if err != nil {
		t.Fatal(err)
	}

	body := []byte(url.Values{"eparams": {encrypted["eparams"]}}.Encode())
	header := http.Header{"Content-Type": {"application/x-www-form-urlencoded"}}

	request := decodeRequest(http.MethodPost, mustURL(t, "https://music.163.com/api/linux/forward"), header, body)
	if request.status != decodeStatusDecrypted || request.apiPath != "/api/song/detail" {
		t.Fatalf("unexpected linux request: %+v", request)
	}

	if !request.responseEncrypted {
		t.Fatal("nested e_r=true was not detected")
	}

	if strings.Contains(string(request.body), "18800001111") {
		t.Fatalf("phone leaked: %s", request.body)
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
	}{
		{
			name:   "weapi form",
			path:   "/weapi/login",
			body:   "params=ciphertext&encSecKey=rsa&csrf_token=csrf-secret",
			header: http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
			want:   protocolWEAPI,
		},
		{
			name:   "xeapi json",
			path:   "/xeapi/song/detail",
			body:   `{"B":"ciphertext","S":"signature","R":"nonce","header":"{\"MUSIC_U\":\"cookie-secret\"}"}`,
			header: http.Header{"Content-Type": {"application/json"}},
			want:   protocolXEAPI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeRequest(http.MethodPost, mustURL(t, "https://music.163.com"+tt.path), tt.header, []byte(tt.body))
			if result.protocol != tt.want || result.status != decodeStatusUnsupported {
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

func TestNonJSONWEAPIAndXEAPIResponsesRemainUnsupported(t *testing.T) {
	for _, currentProtocol := range []protocol{protocolWEAPI, protocolXEAPI} {
		t.Run(string(currentProtocol), func(t *testing.T) {
			result := decodeResponse(
				&decodeResult{protocol: currentProtocol, responseEncrypted: true},
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
		})
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
	return decodeRequestLimited(method, u, header, body, false, defaultJSONDisplayLimit)
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

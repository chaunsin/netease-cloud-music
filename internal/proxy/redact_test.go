// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestRedactHeadersDoesNotMutateInput(t *testing.T) {
	header := http.Header{
		"Authorization":       {"Bearer abc"},
		"Cookie":              {"MUSIC_U=cookie-secret"},
		"Set-Cookie":          {"MUSIC_A=another-secret"},
		"X-Csrf-Token":        {"csrf-secret"},
		"X-Custom-Token":      {"token-secret"},
		"Content-Type":        {"application/json"},
		"X-Unrelated-Headers": {"visible"},
		"Location":            {"https://music.163.com/redirect/token=location-path-secret?token=location-secret&name=song#access_token=location-fragment-secret"},
		"Content-Location":    {"https://music.163.com/download/token%3Dcontent-location-secret"},
		"Referer":             {"https://music.163.com/page?Signature=referer-secret"},
		"X-Meta":              {"old_password=header-secret safe=visible"},
	}

	redacted := redactHeaders(header, false)
	for _, key := range []string{"Authorization", "Cookie", "Set-Cookie", "X-Csrf-Token", "X-Custom-Token"} {
		if got := redacted.Get(key); got != redactedValue {
			t.Errorf("%s = %q", key, got)
		}
	}

	if redacted.Get("Content-Type") != "application/json" || redacted.Get("X-Unrelated-Headers") != "visible" {
		t.Fatalf("non-sensitive headers changed: %#v", redacted)
	}

	if meta := redacted.Get("X-Meta"); strings.Contains(meta, "header-secret") || !strings.Contains(meta, "safe=visible") {
		t.Fatalf("nested header metadata was not safely redacted: %q", meta)
	}

	for _, secret := range []string{"location-secret", "location-path-secret", "location-fragment-secret", "content-location-secret", "referer-secret"} {
		if strings.Contains(redacted.Get("Location")+redacted.Get("Content-Location")+redacted.Get("Referer"), secret) {
			t.Fatalf("URL-valued header leaked %q: %#v", secret, redacted)
		}
	}

	if header.Get("Authorization") != "Bearer abc" {
		t.Fatal("input header was mutated")
	}

	visible := redactHeaders(header, true)
	if visible.Get("Authorization") != "Bearer abc" || visible.Get("Cookie") != "MUSIC_U=cookie-secret" || !strings.Contains(visible.Get("Location"), "location-secret") {
		t.Fatalf("showSensitive did not bypass redaction: %#v", visible)
	}
}

func TestRedactURLAndNestedJSONQuery(t *testing.T) {
	u := mustURL(
		t,
		`https://username:password@music.163.com/api/token=path-secret?phone=18800001111&Signature=signed-secret&NOSAccessKeyId=access-secret&api_key=api-secret&payload=%7B%22access_token%22%3A%22secret%22%2C%22name%22%3A%22song%22%7D&name=song#access_token=fragment-secret`,
	)

	redactedRaw := redactURL(u, false)

	redactedURL, err := url.Parse(redactedRaw)
	if err != nil {
		t.Fatal(err)
	}

	if got := redactedURL.Query().Get("phone"); got != redactedValue {
		t.Fatalf("phone = %q", got)
	}

	if nested := redactedURL.Query().Get("payload"); strings.Contains(nested, "secret") || !strings.Contains(nested, redactedValue) {
		t.Fatalf("nested payload = %q", nested)
	}

	if redactedURL.Query().Get("name") != "song" {
		t.Fatalf("non-sensitive query changed: %q", redactedURL.Query().Get("name"))
	}

	for _, key := range []string{"Signature", "NOSAccessKeyId", "api_key"} {
		if got := redactedURL.Query().Get(key); got != redactedValue {
			t.Fatalf("%s = %q", key, got)
		}
	}

	if redactedURL.User == nil || redactedURL.User.Username() != redactedValue {
		t.Fatalf("URL credentials were not redacted: %s", redactedURL)
	}

	if _, hasPassword := redactedURL.User.Password(); hasPassword {
		t.Fatalf("redacted URL retained a password marker: %s", redactedURL)
	}

	for _, secret := range []string{"path-secret", "fragment-secret"} {
		if strings.Contains(redactedRaw, secret) {
			t.Fatalf("URL path or fragment leaked %q: %s", secret, redactedRaw)
		}
	}

	if u.Query().Get("phone") != "18800001111" {
		t.Fatal("input URL was mutated")
	}

	if visible := redactURL(u, true); !strings.Contains(visible, "18800001111") || !strings.Contains(visible, "username:password") ||
		!strings.Contains(visible, "path-secret") || !strings.Contains(visible, "fragment-secret") {
		t.Fatalf("showSensitive did not bypass redaction: %s", visible)
	}
}

func TestParseQueryForCaptureEmptyQueryIgnoresParameterLimit(t *testing.T) {
	t.Setenv("GODEBUG", "urlmaxqueryparams=1")

	capture := parseQueryForCapture("")
	if capture.parseErr != nil || len(capture.parsed) != 0 || len(capture.display) != 0 || capture.responseEncrypted {
		t.Fatalf("empty query produced synthetic capture state: %+v", capture)
	}
}

func TestParseQueryForCapturePreservesMalformedFieldPositions(t *testing.T) {
	capture := parseQueryForCapture("id=first&id=bad;secret&id=last")
	if capture.parseErr == nil {
		t.Fatal("malformed query did not report an error")
	}

	if got, want := capture.parsed["id"], []string{"first", "last"}; !slices.Equal(got, want) {
		t.Fatalf("parsed id values = %#v, want %#v", got, want)
	}

	if got, want := capture.display["id"], []string{"first", redactedValue, "last"}; !slices.Equal(got, want) {
		t.Fatalf("display id values = %#v, want %#v", got, want)
	}
}

func TestParseQueryForCaptureRedactsUnsafeKeys(t *testing.T) {
	for _, rawQuery := range []string{
		"name=song&bad%FF=private-secret&e%FF_r=true",
		"name=song&bad=%zz&e%FF_r=true",
	} {
		capture := parseQueryForCapture(rawQuery)
		if capture.display.Get("name") != "song" || capture.display.Get(redactedValue) != redactedValue {
			t.Fatalf("unsafe query key was not redacted safely: %#v", capture.display)
		}

		if strings.Contains(capture.display.Encode(), "private-secret") {
			t.Fatalf("unsafe query key exposed its value: %s", capture.display.Encode())
		}

		if capture.responseEncrypted {
			t.Fatalf("unsafe query key was trusted as e_r: %+v", capture)
		}
	}
}

func TestParseQueryForCaptureWholeQueryError(t *testing.T) {
	t.Setenv("GODEBUG", "urlmaxqueryparams=2")

	if _, err := url.ParseQuery("name=song&e_r=true&id=1"); err == nil {
		t.Skip("this Go toolchain does not enforce urlmaxqueryparams")
	}

	tests := []struct {
		name              string
		rawQuery          string
		responseEncrypted bool
	}{
		{name: "valid fields", rawQuery: "name=song&e_r=true&id=1", responseEncrypted: true},
		{name: "malformed fields", rawQuery: "name=%zz&note=%zz&id=%zz"},
		{name: "empty fields", rawQuery: "&&"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := parseQueryForCapture(tt.rawQuery)
			if capture.parseErr == nil || capture.display.Get(redactedValue) != redactedValue || len(capture.display) != 1 {
				t.Fatalf("whole-query failure was not represented safely: %+v", capture)
			}

			if capture.responseEncrypted != tt.responseEncrypted {
				t.Fatalf("responseEncrypted = %t, want %t", capture.responseEncrypted, tt.responseEncrypted)
			}
		})
	}
}

func TestMalformedURLQueryFailsClosedWithoutDroppingFields(t *testing.T) {
	u := &url.URL{
		Scheme:   "https",
		Host:     "music.163.com",
		Path:     "/api/test",
		RawQuery: "name=song&note=private-secret;token=hidden-secret&bad%zz=key-secret&phone=18800001111",
	}

	redactedRaw := redactURL(u, false)

	redactedURL, err := url.Parse(redactedRaw)
	if err != nil {
		t.Fatal(err)
	}

	query := redactedURL.Query()
	if query.Get("name") != "song" {
		t.Fatalf("safe query value changed: %q", query.Get("name"))
	}

	for _, key := range []string{"note", "phone", redactedValue} {
		if got := query.Get(key); got != redactedValue {
			t.Fatalf("%s = %q, want %q", key, got, redactedValue)
		}
	}

	for _, secret := range []string{"private-secret", "hidden-secret", "key-secret", "18800001111"} {
		if strings.Contains(redactedRaw, secret) {
			t.Fatalf("malformed URL query leaked %q: %s", secret, redactedRaw)
		}
	}

	result := decodeRequest(http.MethodGet, u, nil, nil)
	if !json.Valid(result.query) {
		t.Fatalf("formatted malformed query is invalid JSON: %s", result.query)
	}

	decoded, err := decodeJSON(result.query)
	if err != nil {
		t.Fatal(err)
	}

	formatted, ok := decoded.(map[string]any)
	if !ok || formatted["name"] != "song" {
		t.Fatalf("formatted malformed query lost safe fields: %s", result.query)
	}

	for _, key := range []string{"note", "phone", redactedValue} {
		if formatted[key] != redactedValue {
			t.Fatalf("formatted %s = %#v, want %q", key, formatted[key], redactedValue)
		}
	}

	for _, secret := range []string{"private-secret", "hidden-secret", "key-secret", "18800001111"} {
		if strings.Contains(string(result.query), secret) {
			t.Fatalf("formatted malformed query leaked %q: %s", secret, result.query)
		}
	}

	if visible := redactURL(u, true); visible != u.String() {
		t.Fatalf("showSensitive changed malformed raw query: %q", visible)
	}

	if u.RawQuery != "name=song&note=private-secret;token=hidden-secret&bad%zz=key-secret&phone=18800001111" {
		t.Fatal("input URL was mutated")
	}
}

func TestMalformedURLStringFailsClosed(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantPlaceholder string
	}{
		{
			name:            "malformed query field",
			input:           `{"redirect":"https://username:password@music.163.com/api?note=private-secret%zz"}`,
			wantPlaceholder: url.QueryEscape(redactedValue),
		},
		{
			name:            "URL parse failure",
			input:           `{"redirect":"https://username:password@music.163.com/%zz?note=private-secret"}`,
			wantPlaceholder: unsafeTextPlaceholder,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted, _, err := formatJSONLimited([]byte(tt.input), false, 1024)
			if err != nil {
				t.Fatal(err)
			}

			for _, secret := range []string{"username", "password", "private-secret"} {
				if strings.Contains(string(formatted), secret) {
					t.Fatalf("malformed URL string leaked %q: %s", secret, formatted)
				}
			}

			if !strings.Contains(string(formatted), tt.wantPlaceholder) {
				t.Fatalf("malformed URL string did not retain a safe placeholder: %s", formatted)
			}
		})
	}
}

func TestFormatJSONRecursiveRedactionAndUseNumber(t *testing.T) {
	input := []byte(`{
		"id": 9007199254740993,
		"password": "top-secret",
		"deviceId": "device-secret",
		"email": "listener@example.com",
		"items": [{"MUSIC_U": "cookie-secret"}],
		"header": "{\"csrf_token\":\"csrf-secret\",\"MUSIC_R_U\":\"renewal-cookie-secret\",\"safe\":\"visible\"}",
		"opaque": "MUSIC_R_U=opaque-cookie-secret",
		"download_url": "https://music.163.com/file?Signature=url-secret&name=visible",
		"redirect": "{\"url\":\"https://music.163.com/next?access_token=nested-url-secret\"}"
	}`)

	formatted, _, err := formatJSONLimited(input, false, defaultJSONDisplayLimit)
	if err != nil {
		t.Fatal(err)
	}

	if !json.Valid(formatted) {
		t.Fatalf("formatted JSON invalid: %s", formatted)
	}

	text := string(formatted)
	if strings.Contains(text, "top-secret") || strings.Contains(text, "cookie-secret") || strings.Contains(text, "csrf-secret") || strings.Contains(text, "renewal-cookie-secret") ||
		strings.Contains(text, "opaque-cookie-secret") ||
		strings.Contains(text, "url-secret") ||
		strings.Contains(text, "device-secret") ||
		strings.Contains(text, "listener@example.com") {
		t.Fatalf("sensitive value leaked: %s", text)
	}

	if !strings.Contains(text, "9007199254740993") || !strings.Contains(text, "visible") {
		t.Fatalf("safe data changed: %s", text)
	}

	_, encrypted, err := formatJSONLimited([]byte(`{"wrapper":"{\"e_r\":true}"}`), false, defaultJSONDisplayLimit)
	if err != nil || !encrypted.requestEncrypted {
		t.Fatal("nested JSON string e_r was not detected")
	}

	original, err := decodeJSON(input)
	if err != nil {
		t.Fatal(err)
	}

	object, ok := original.(map[string]any)
	if !ok {
		t.Fatalf("decoded JSON has type %T", original)
	}

	if number, ok := object["id"].(json.Number); !ok || number.String() != "9007199254740993" {
		t.Fatalf("number was not decoded with UseNumber: %#v", object["id"])
	}

	visible, _, err := formatJSONLimited(input, true, defaultJSONDisplayLimit)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(visible), "top-secret") || !strings.Contains(string(visible), "cookie-secret") {
		t.Fatalf("showSensitive did not bypass redaction: %s", visible)
	}
}

func TestFormatValuesForDisplayPreservesFormSemantics(t *testing.T) {
	html := strings.Repeat("<", 64)

	formatted, meta, err := formatValuesForDisplay(url.Values{
		"e_r":   {"true", "true"},
		"empty": {},
		"html":  {html},
		"ids":   {"1", "2"},
		"token": {"secret"},
	}, false, 256)
	if err != nil {
		t.Fatal(err)
	}

	if !meta.requestEncrypted || !json.Valid(formatted) {
		t.Fatalf("invalid formatted values: meta=%+v body=%s", meta, formatted)
	}

	decoded, err := decodeJSON(formatted)
	if err != nil {
		t.Fatal(err)
	}

	value, ok := decoded.(map[string]any)
	if !ok {
		t.Fatalf("decoded form has type %T", decoded)
	}

	if value["html"] != html || value["token"] != redactedValue {
		t.Fatalf("scalar values changed: %#v", value)
	}

	empty, emptyOK := value["empty"].([]any)
	encryptedEntries, encryptedOK := value["e_r"].([]any)

	ids, idsOK := value["ids"].([]any)
	if !emptyOK || !encryptedOK || !idsOK || len(empty) != 0 || len(encryptedEntries) != 2 || len(ids) != 2 {
		t.Fatalf("multi-value fields changed: %#v", value)
	}

	if _, _, err := formatValuesForDisplay(url.Values{"value": {strings.Repeat("x", 256)}}, false, 32); !errors.Is(err, errJSONDisplayLimit) {
		t.Fatalf("oversized form error = %v, want display limit", err)
	}
}

func TestFormatValuesForDisplayFailsClosedOnInvalidUTF8(t *testing.T) {
	invalid := string(append([]byte("private-value"), 0xff))
	values := url.Values{"note": {invalid}}

	formatted, _, err := formatValuesForDisplay(values, false, 1024)
	if err != nil {
		t.Fatal(err)
	}

	if !json.Valid(formatted) || strings.Contains(string(formatted), "private-value") || !strings.Contains(string(formatted), unsafeTextPlaceholder) {
		t.Fatalf("invalid form value did not fail closed: %q", formatted)
	}

	query := formatQuery(values, false, 1024)
	if !json.Valid(query) || strings.Contains(string(query), "private-value") || !strings.Contains(string(query), unsafeTextPlaceholder) {
		t.Fatalf("invalid query value did not fail closed: %q", query)
	}

	display := formatBody(
		http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
		[]byte("note=private-value%FF"),
		false,
		1024,
	)
	if !display.structured || !json.Valid(display.body) || strings.Contains(string(display.body), "private-value") || !strings.Contains(string(display.body), unsafeTextPlaceholder) {
		t.Fatalf("invalid form body did not fail closed: %#v", display)
	}

	invalidKey := string(append([]byte("bad"), 0xff))
	values = url.Values{invalidKey: {"private-key-value"}}

	formatted, _, err = formatValuesForDisplay(values, false, 1024)
	if err != nil {
		t.Fatal(err)
	}

	if !json.Valid(formatted) || strings.Contains(string(formatted), "private-key-value") || !strings.Contains(string(formatted), redactedValue) {
		t.Fatalf("invalid form key did not redact its value: %q", formatted)
	}

	display = formatBody(
		http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
		[]byte("bad%FF=private-key-value"),
		false,
		1024,
	)
	if !display.structured || !json.Valid(display.body) || strings.Contains(string(display.body), "private-key-value") || !strings.Contains(string(display.body), redactedValue) {
		t.Fatalf("invalid form body key did not fail closed: %#v", display)
	}

	visible, _, err := formatValuesForDisplay(url.Values{"note": {invalid}}, true, 1024)
	if err != nil || !json.Valid(visible) || !strings.Contains(string(visible), "private-value") {
		t.Fatalf("show-sensitive invalid value was not safely encoded: body=%q err=%v", visible, err)
	}
}

func TestSensitiveKeyVariants(t *testing.T) {
	sensitive := []string{
		"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "csrf_token",
		"password", "old_password", "userPassword", "password_hash", "accessToken", "refresh-token", "MUSIC_U", "MUSIC_A", "MUSIC_R_U", "MUSIC_R_A", "MUSIC_A_T", "cellphone",
		"phoneNumber", "mobile", "Signature", "NOSAccessKeyId", "api_key", "X-Encr-Sskey", "X-Encr-Ssid", "sessionId",
		"email", "deviceId", "device_identifier", "imei", "imsi", "captcha", "verification_code", "smsCode",
	}
	for _, key := range sensitive {
		if !sensitiveKey(key) {
			t.Errorf("%q should be sensitive", key)
		}
	}

	for _, key := range []string{"Content-Type", "requestId", "tokenTypeHintedByNameButStillToken"} {
		got := sensitiveKey(key)
		if key == "tokenTypeHintedByNameButStillToken" {
			if !got {
				t.Errorf("%q should be sensitive", key)
			}
		} else if got {
			t.Errorf("%q should not be sensitive", key)
		}
	}
}

func TestRedactTextBestEffort(t *testing.T) {
	input := []byte(strings.Join([]string{
		"Authorization: Bearer top-secret",
		"password=hunter2 old_password=old-secret userPassword=camel-secret password_hash=hash-secret safe=visible csrf_token: csrf-secret MUSIC_R_U=renewal-cookie-secret",
		"message: keep me",
	}, "\n"))

	redacted := string(redactText(input, false))
	for _, secret := range []string{"Bearer top-secret", "hunter2", "old-secret", "camel-secret", "hash-secret", "csrf-secret", "renewal-cookie-secret"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("text leaked %q: %s", secret, redacted)
		}
	}

	if !strings.Contains(redacted, "safe=visible") || !strings.Contains(redacted, "message: keep me") {
		t.Fatalf("safe text changed: %s", redacted)
	}

	if visible := string(redactText(input, true)); visible != string(input) {
		t.Fatalf("showSensitive changed text: %s", visible)
	}
}

func TestUnstructuredBodiesFailClosedUnlessSensitiveOutputIsEnabled(t *testing.T) {
	header := http.Header{"Content-Type": {"application/json"}}
	malformed := []byte(`{"MUSIC\u005fU":"escaped-cookie-secret",`)

	redacted := formatBody(header, malformed, false, 1024)
	if strings.Contains(string(redacted.body), "escaped-cookie-secret") {
		t.Fatalf("malformed JSON leaked a secret: %q", redacted.body)
	}

	if !strings.Contains(string(redacted.body), "unable to safely redact") || redacted.structured {
		t.Fatalf("malformed JSON did not fail closed: %#v", redacted)
	}

	visible := formatBody(header, malformed, true, 1024)
	if !bytes.Equal(visible.body, malformed) {
		t.Fatalf("show-sensitive did not retain malformed body: %q", visible.body)
	}

	nonUTF8 := append([]byte("MUSIC_U=invalid-byte-secret"), 0xff)

	redacted = formatBody(http.Header{"Content-Type": {"text/plain"}}, nonUTF8, false, 1024)
	if strings.Contains(string(redacted.body), "invalid-byte-secret") {
		t.Fatalf("non-UTF-8 body leaked a secret: %q", redacted.body)
	}

	visible = formatBody(http.Header{"Content-Type": {"text/plain"}}, nonUTF8, true, 1024)
	if !bytes.Equal(visible.body, nonUTF8) {
		t.Fatalf("show-sensitive changed non-UTF-8 body: %q", visible.body)
	}
}

func TestJSONDisplayHasDepthAndOutputBudgets(t *testing.T) {
	deep := []byte(strings.Repeat("[", maxJSONDisplayDepth+1) + strings.Repeat("]", maxJSONDisplayDepth+1))
	if _, _, err := formatJSONLimited(deep, false, int64(len(deep))); !errors.Is(err, errJSONDepth) {
		t.Fatalf("deep JSON error = %v, want depth limit", err)
	}

	compact := []byte(`{"a":1,"b":2}`)
	if _, _, err := formatJSONLimited(compact, false, int64(len(compact))); !errors.Is(err, errJSONDisplayLimit) {
		t.Fatalf("compact JSON error = %v, want display limit", err)
	}

	display := formatBody(http.Header{"Content-Type": {"application/json"}}, compact, false, int64(len(compact)))
	if strings.Contains(string(display.body), `"a":1`) || !strings.Contains(string(display.body), "body omitted") {
		t.Fatalf("over-budget JSON did not fail closed: %q", display.body)
	}
}

func TestEscapeLogFieldPreventsRecordInjection(t *testing.T) {
	input := "api-path\r\n[2026-01-01] #999999 REQUEST\x1b[2J"

	escaped := escapeLogField(input)
	if strings.Contains(escaped, "\r") || strings.Contains(escaped, "\n") || strings.Contains(escaped, "\x1b") {
		t.Fatalf("unsafe controls remained in log field: %q", escaped)
	}

	for _, want := range []string{`\r`, `\n`, `\x1b`} {
		if !strings.Contains(escaped, want) {
			t.Fatalf("escaped control %q missing from %q", want, escaped)
		}
	}
}

func TestRedactDiagnosticURLs(t *testing.T) {
	input := []byte("Got request GET HTTP://username:password@music.163.com/api?csrf_token=query-secret&name=song\n")

	redacted := string(redactDiagnostic(input, false))
	for _, secret := range []string{"username", "password", "query-secret"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("diagnostic leaked %q: %s", secret, redacted)
		}
	}

	if !strings.Contains(redacted, "name=song") || !strings.Contains(redacted, redactedValue) {
		t.Fatalf("diagnostic safe data changed or redaction marker missing: %s", redacted)
	}

	if visible := string(redactDiagnostic(input, true)); visible != string(input) {
		t.Fatalf("showSensitive changed diagnostic: %s", visible)
	}
}

func TestDiagnosticWriterRedactsXEAPIOuterR(t *testing.T) {
	const (
		xeapiRUpper = "xeapi-r-upper-secret"
		xeapiRLower = "xeapi-r-lower-secret"
		genericR    = "generic-r-visible"
	)

	input := []byte(
		"Got request GET https://music.163.com/xeapi/song/detail?B=outer-b&R=" + xeapiRUpper + "&r=" + xeapiRLower + "&S=outer-s\n" +
			"Got request GET https://music.163.com/api/song/detail?R=" + genericR + "&name=song\n",
	)

	var hidden bytes.Buffer

	written, err := (&diagnosticWriter{out: &hidden}).Write(input)
	if err != nil || written != len(input) {
		t.Fatalf("write diagnostic = %d, %v", written, err)
	}

	redacted := hidden.String()
	for _, secret := range []string{xeapiRUpper, xeapiRLower} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("default diagnostic leaked XEAPI R %q: %s", secret, redacted)
		}
	}

	for _, safe := range []string{"B=outer-b", "S=outer-s", "R=" + genericR, "name=song"} {
		if !strings.Contains(redacted, safe) {
			t.Fatalf("default diagnostic changed non-sensitive value %q: %s", safe, redacted)
		}
	}

	if strings.Count(redacted, url.QueryEscape(redactedValue)) != 2 {
		t.Fatalf("XEAPI R values were not precisely redacted: %s", redacted)
	}

	var visible bytes.Buffer

	written, err = (&diagnosticWriter{out: &visible, showSensitive: true}).Write(input)
	if err != nil || written != len(input) || !bytes.Equal(visible.Bytes(), input) {
		t.Fatalf("showSensitive diagnostic = %d, %v, %q", written, err, visible.String())
	}
}

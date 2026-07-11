// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
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
		"Location":            {"https://music.163.com/redirect?token=location-secret&name=song"},
		"Referer":             {"https://music.163.com/page?Signature=referer-secret"},
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
	for _, secret := range []string{"location-secret", "referer-secret"} {
		if strings.Contains(redacted.Get("Location")+redacted.Get("Referer"), secret) {
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
	u := mustURL(t, `https://username:password@music.163.com/api/test?phone=18800001111&Signature=signed-secret&NOSAccessKeyId=access-secret&api_key=api-secret&payload=%7B%22access_token%22%3A%22secret%22%2C%22name%22%3A%22song%22%7D&name=song`)

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
	if u.Query().Get("phone") != "18800001111" {
		t.Fatal("input URL was mutated")
	}
	if visible := redactURL(u, true); !strings.Contains(visible, "18800001111") || !strings.Contains(visible, "username:password") {
		t.Fatalf("showSensitive did not bypass redaction: %s", visible)
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

	formatted, original, err := formatJSON(input, false)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(formatted) {
		t.Fatalf("formatted JSON invalid: %s", formatted)
	}
	text := string(formatted)
	if strings.Contains(text, "top-secret") || strings.Contains(text, "cookie-secret") || strings.Contains(text, "csrf-secret") || strings.Contains(text, "renewal-cookie-secret") || strings.Contains(text, "opaque-cookie-secret") || strings.Contains(text, "url-secret") || strings.Contains(text, "device-secret") || strings.Contains(text, "listener@example.com") {
		t.Fatalf("sensitive value leaked: %s", text)
	}
	if !strings.Contains(text, "9007199254740993") || !strings.Contains(text, "visible") {
		t.Fatalf("safe data changed: %s", text)
	}
	if !valueRequestEncrypted(map[string]any{"wrapper": `{"e_r":true}`}) {
		t.Fatal("nested JSON string e_r was not detected")
	}
	object := original.(map[string]any)
	if number, ok := object["id"].(json.Number); !ok || number.String() != "9007199254740993" {
		t.Fatalf("number was not decoded with UseNumber: %#v", object["id"])
	}

	visible, _, err := formatJSON(input, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(visible), "top-secret") || !strings.Contains(string(visible), "cookie-secret") {
		t.Fatalf("showSensitive did not bypass redaction: %s", visible)
	}
}

func TestSensitiveKeyVariants(t *testing.T) {
	sensitive := []string{
		"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "csrf_token",
		"password", "accessToken", "refresh-token", "MUSIC_U", "MUSIC_A", "MUSIC_R_U", "MUSIC_R_A", "MUSIC_A_T", "cellphone",
		"phoneNumber", "mobile", "Signature", "NOSAccessKeyId", "api_key", "X-Encr-Sskey",
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
	input := []byte("Authorization: Bearer top-secret\npassword=hunter2 safe=visible csrf_token: csrf-secret MUSIC_R_U=renewal-cookie-secret\nmessage: keep me")
	redacted := string(redactText(input, false))
	for _, secret := range []string{"Bearer top-secret", "hunter2", "csrf-secret", "renewal-cookie-secret"} {
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

	redacted := formatRequestBody(header, malformed, false, 1024)
	if strings.Contains(string(redacted.body), "escaped-cookie-secret") {
		t.Fatalf("malformed JSON leaked a secret: %q", redacted.body)
	}
	if !strings.Contains(string(redacted.body), "unable to safely redact") || redacted.structured {
		t.Fatalf("malformed JSON did not fail closed: %#v", redacted)
	}

	visible := formatRequestBody(header, malformed, true, 1024)
	if string(visible.body) != string(malformed) {
		t.Fatalf("show-sensitive did not retain malformed body: %q", visible.body)
	}

	nonUTF8 := append([]byte("MUSIC_U=invalid-byte-secret"), 0xff)
	redacted = formatRequestBody(http.Header{"Content-Type": {"text/plain"}}, nonUTF8, false, 1024)
	if strings.Contains(string(redacted.body), "invalid-byte-secret") {
		t.Fatalf("non-UTF-8 body leaked a secret: %q", redacted.body)
	}
	visible = formatRequestBody(http.Header{"Content-Type": {"text/plain"}}, nonUTF8, true, 1024)
	if string(visible.body) != string(nonUTF8) {
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
	display := formatRequestBody(http.Header{"Content-Type": {"application/json"}}, compact, false, int64(len(compact)))
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

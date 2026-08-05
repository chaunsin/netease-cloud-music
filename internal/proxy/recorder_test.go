// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	ncmcrypto "github.com/chaunsin/netease-cloud-music/pkg/crypto"
)

type blockingWriter struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	mu      sync.Mutex
	buffer  bytes.Buffer
}

func (w *blockingWriter) Write(data []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	<-w.release
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buffer.Write(data)
}

func (w *blockingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buffer.String()
}

func (w *blockingWriter) unblock() {
	w.once.Do(func() {})

	select {
	case <-w.release:
	default:
		close(w.release)
	}
}

func recordTestRequest(recorder *recorder, state *captureState) {
	state.requestRecord, state.requestDecoded = newRequestRecord(state.requestURL)
	recorder.finishRequest(state.requestRecord, state)
}

func flushRecorder(recorder *recorder, timeout time.Duration) bool {
	done := make(chan struct{})

	if recorder == nil || timeout <= 0 || !recorder.submit(func() { close(done) }) {
		return false
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func TestRecorderQueueDoesNotBlockRequestCapture(t *testing.T) {
	writer := &blockingWriter{started: make(chan struct{}), release: make(chan struct{})}
	recorder := newRecorder(writer, 1024, false)

	t.Cleanup(func() {
		writer.unblock()
		recorder.CloseWithTimeout(time.Second)
	})

	requestURL, err := url.Parse("https://music.163.com/api/test")
	if err != nil {
		t.Fatal(err)
	}

	first := &captureState{
		requestMethod: http.MethodPost,
		requestURL:    requestURL,
		requestHeader: http.Header{"Content-Type": {"application/json"}},
		requestBody: bodySnapshot{
			raw:         []byte(`{"code":200}`),
			contentType: "application/json",
		},
	}
	recordTestRequest(recorder, first)

	select {
	case <-writer.started:
	case <-time.After(time.Second):
		t.Fatal("recorder worker did not reach the blocking writer")
	}

	started := time.Now()

	for range recorderQueueCapacity * 3 {
		state := &captureState{
			requestMethod: http.MethodPost,
			requestURL:    requestURL,
			requestHeader: http.Header{"Content-Type": {"application/json"}},
			requestBody: bodySnapshot{
				raw:         []byte(`{"token":"request-secret"}`),
				contentType: "application/json",
			},
		}
		recordTestRequest(recorder, state)
	}

	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("recording blocked request capture for %s", elapsed)
	}

	writer.unblock()

	deadline := time.Now().Add(time.Second)
	for !flushRecorder(recorder, 50*time.Millisecond) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if !strings.Contains(writer.String(), "CAPTURE_DROPPED") {
		t.Fatalf("full queue did not produce a dropped-capture marker: %q", writer.String())
	}
}

func TestRecorderEscapesMetadataAndRawControlBodies(t *testing.T) {
	var output bytes.Buffer

	recorder := newRecorder(&output, 1024, true)
	t.Cleanup(recorder.Close)

	state := &captureState{
		session:       1,
		requestMethod: "POST\r\nFORGED",
		requestURL:    &url.URL{Scheme: "https", Host: "music.163.com", Path: "/api/test"},
		requestHeader: http.Header{"X-Note": {"ok\r\n[FORGED]"}},
		requestBody: bodySnapshot{
			contentType: "text/plain",
		},
	}
	decoded := decodeResult{protocol: protocolEAPI, status: decodeStatusDecrypted, apiPath: "/api/test\r\napi-path: forged"}
	recorder.writeRequestBlock(state, &decoded, "detail\r\nforged")

	body, encoding, _ := terminalBody([]byte("body\r\n[FORGED]"), 1024)
	if encoding != "base64" || strings.Contains(body, "[FORGED]") {
		t.Fatalf("raw control body was not terminal-safe: body=%q encoding=%q", body, encoding)
	}

	text := output.String()
	for _, raw := range []string{"POST\r\nFORGED", "ok\r\n[FORGED]", "/api/test\r\napi-path: forged", "detail\r\nforged"} {
		if strings.Contains(text, raw) {
			t.Fatalf("raw control-bearing metadata leaked: %q in %q", raw, text)
		}
	}

	for _, escaped := range []string{`POST\r\nFORGED`, `ok\r\n[FORGED]`, `api-path: /api/test\r\napi-path: forged`} {
		if !strings.Contains(text, escaped) {
			t.Fatalf("escaped metadata %q missing from %q", escaped, text)
		}
	}
}

func TestWriteHeadersPreservesNonCanonicalValues(t *testing.T) {
	var output bytes.Buffer
	writeHeaders(&output, http.Header{
		"X-Trace": {"canonical"},
		"x-trace": {"lowercase"},
	}, false)

	if text := output.String(); !strings.Contains(text, "X-Trace: canonical") || !strings.Contains(text, "x-trace: lowercase") {
		t.Fatalf("header values were canonicalized onto the wrong keys: %q", text)
	}
}

func TestNewRequestRecordPreservesQueryEncryptionHint(t *testing.T) {
	requestURL, err := url.Parse("https://music.163.com/eapi/test?e_r=true")
	if err != nil {
		t.Fatal(err)
	}

	_, result := newRequestRecord(requestURL)
	if !result.responseEncrypted {
		t.Fatal("query e_r=true was not retained in provisional request metadata")
	}
}

func TestNewRequestRecordDefaultsXEAPIResponseToEncrypted(t *testing.T) {
	requestURL, err := url.Parse("https://music.163.com/xeapi/test")
	if err != nil {
		t.Fatal(err)
	}

	_, result := newRequestRecord(requestURL)
	if result.status != decodeStatusPartial || !result.responseEncrypted {
		t.Fatalf("XEAPI provisional result = %+v", result)
	}
}

func TestRecorderMarksIncompleteXEAPIObservationPartial(t *testing.T) {
	tests := []struct {
		name     string
		snapshot bodySnapshot
	}{
		{name: "truncated", snapshot: bodySnapshot{raw: []byte("B=value"), contentType: "application/x-www-form-urlencoded", truncated: true}},
		{
			name:     "unknown length omitted",
			snapshot: bodySnapshot{contentType: "application/x-www-form-urlencoded", omittedReason: "unknown-length request body omitted to avoid delaying streaming traffic"},
		},
		{name: "read failed", snapshot: bodySnapshot{raw: []byte("B=value"), contentType: "application/x-www-form-urlencoded", captureErr: io.ErrUnexpectedEOF}},
		{
			name: "content decoding failed",
			snapshot: bodySnapshot{
				raw: []byte("not-gzip"), contentType: "application/x-www-form-urlencoded", contentEncode: "gzip",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer

			recorder := newRecorder(&output, 1024, false)

			requestURL, err := url.Parse("https://music.163.com/xeapi/test")
			if err != nil {
				t.Fatal(err)
			}

			state := &captureState{
				session:       1,
				requestMethod: http.MethodPost,
				requestURL:    requestURL,
				requestHeader: http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
				requestBody:   tt.snapshot,
			}
			recordTestRequest(recorder, state)

			if !flushRecorder(recorder, time.Second) {
				t.Fatal("recorder did not flush")
			}

			recorder.CloseWithTimeout(time.Second)

			text := output.String()
			if !strings.Contains(text, "REQUEST protocol=xeapi decode=partial") || !strings.Contains(text, "XEAPI request observation is incomplete") {
				t.Fatalf("incomplete XEAPI request was misclassified: %s", text)
			}
		})
	}
}

func TestRecorderMarksIncompleteXEAPIResponsePartial(t *testing.T) {
	tests := []struct {
		name     string
		snapshot bodySnapshot
	}{
		{name: "truncated valid JSON", snapshot: bodySnapshot{raw: []byte(`{"code":200}`), contentType: "application/json", truncated: true}},
		{name: "omitted", snapshot: bodySnapshot{contentType: "application/octet-stream", omittedReason: "response body unavailable"}},
		{name: "read failed", snapshot: bodySnapshot{raw: []byte("ciphertext"), contentType: "application/octet-stream", captureErr: io.ErrUnexpectedEOF}},
		{
			name: "content decoding failed",
			snapshot: bodySnapshot{
				raw: []byte("not-gzip"), contentType: "application/octet-stream", contentEncode: "gzip",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer

			recorder := newRecorder(&output, 1024, false)
			t.Cleanup(recorder.Close)

			requestURL, err := url.Parse("https://music.163.com/xeapi/test")
			if err != nil {
				t.Fatal(err)
			}

			state := &captureState{
				session:        1,
				started:        time.Now(),
				requestMethod:  http.MethodGet,
				requestURL:     requestURL,
				requestDecoded: decodeResult{protocol: protocolXEAPI, status: decodeStatusDecrypted, responseEncrypted: true},
				responseBody:   tt.snapshot,
			}
			recorder.recordResponse(state, &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {tt.snapshot.contentType}},
			})

			if !flushRecorder(recorder, time.Second) {
				t.Fatal("recorder did not flush")
			}

			text := output.String()
			if !strings.Contains(text, "RESPONSE status=200") || !strings.Contains(text, "protocol=xeapi decode=partial") {
				t.Fatalf("incomplete XEAPI response was misclassified: %s", text)
			}

			if !strings.Contains(text, "XEAPI response observation is incomplete") {
				t.Fatalf("incomplete XEAPI response boundary was not reported: %s", text)
			}
		})
	}
}

func TestRecorderRedactsXEAPIRAndLogicalMetadata(t *testing.T) {
	const (
		sessionID         = "recorder-session-secret"
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
	outerR := params.Get("R")

	requestURL, err := url.Parse("https://music.163.com/xeapi/test?" + params.Encode())
	if err != nil {
		t.Fatal(err)
	}

	capture := func(t *testing.T, showSensitive bool) string {
		t.Helper()

		var output bytes.Buffer

		sessions := newXeapiSessionCache([]XeapiSessionSeed{{
			ID: sessionID, Key: sessionKey, Source: XeapiSessionSourceCommandLine,
		}})
		recorder := newRecorderWithXeapiSessions(&output, io.Discard, 1<<20, showSensitive, sessions)
		t.Cleanup(recorder.Close)

		state := &captureState{
			session:         1,
			started:         time.Now(),
			requestMethod:   http.MethodGet,
			requestURL:      requestURL,
			requestHeader:   make(http.Header),
			requestSessions: sessions.snapshot(),
			responseBody: bodySnapshot{
				raw: []byte(`{"code":200}`), contentType: "application/json",
			},
		}
		recordTestRequest(recorder, state)

		if !flushRecorder(recorder, time.Second) {
			t.Fatal("request record did not flush")
		}

		recorder.recordResponse(state, &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
		})
		recorder.recordResponseError(state, errors.New("upstream response failed"))

		if !flushRecorder(recorder, time.Second) {
			t.Fatal("response records did not flush")
		}

		return output.String()
	}

	hidden := capture(t, false)

	encodedOuterR := url.QueryEscape(outerR)
	if strings.Contains(hidden, outerR) || strings.Contains(hidden, encodedOuterR) {
		t.Fatalf("default capture exposed outer XEAPI R: %s", hidden)
	}

	for _, secret := range []string{sessionID, contentTypeSecret, logicalRSecret} {
		if strings.Contains(hidden, secret) {
			t.Fatalf("default capture exposed %q: %s", secret, hidden)
		}
	}

	if strings.Count(hidden, "&R=") < 3 || !strings.Contains(hidden, "[REDACTED") {
		t.Fatalf("REQUEST/RESPONSE/RESPONSE_ERROR URLs were not all redacted: %s", hidden)
	}

	visible := capture(t, true)
	if strings.Count(visible, "R="+encodedOuterR) < 3 {
		t.Fatalf("--show-sensitive did not retain all XEAPI URLs: %s", visible)
	}

	for _, original := range []string{sessionID, contentTypeSecret, logicalRSecret} {
		if !strings.Contains(visible, original) {
			t.Fatalf("--show-sensitive did not retain %q: %s", original, visible)
		}
	}
}

func TestXeapiSessionHeaderDiagnosticDoesNotBlockOrExposeKey(t *testing.T) {
	writer := &blockingWriter{started: make(chan struct{}), release: make(chan struct{})}
	recorder := newRecorderWithXeapiSessions(io.Discard, writer, 1024, true, nil)

	t.Cleanup(func() {
		writer.unblock()
		recorder.CloseWithTimeout(time.Second)
	})

	const key = "sensitive-key-12"

	started := time.Now()

	recorder.recordXeapiSessionHeaderError(42, errors.New("X-Encr-Sskey: "+key))

	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("diagnostic submission blocked for %s", elapsed)
	}

	select {
	case <-writer.started:
	case <-time.After(time.Second):
		t.Fatal("recorder worker did not reach the diagnostic writer")
	}

	writer.unblock()

	if !flushRecorder(recorder, time.Second) {
		t.Fatal("recorder did not flush")
	}

	text := writer.String()
	if !strings.Contains(text, "XEAPI_SESSION_HEADERS_IGNORED") || strings.Contains(text, key) {
		t.Fatalf("unsafe XEAPI session diagnostic: %s", text)
	}
}

func TestRecorderDoesNotApplyLaterXeapiSessionToQueuedRequest(t *testing.T) {
	const (
		sessionID  = "queued-session"
		sessionKey = "0123456789abcdef"
	)

	writer := &blockingWriter{started: make(chan struct{}), release: make(chan struct{})}
	sessions := newXeapiSessionCache(nil)
	recorder := newRecorderWithXeapiSessions(writer, io.Discard, 1<<20, false, sessions)

	t.Cleanup(func() {
		writer.unblock()
		recorder.CloseWithTimeout(time.Second)
	})

	requireSubmitted := recorder.submit(func() { _, _ = writer.Write([]byte("blocked\n")) })
	if !requireSubmitted {
		t.Fatal("failed to submit blocking recorder task")
	}

	select {
	case <-writer.started:
	case <-time.After(time.Second):
		t.Fatal("recorder worker did not reach the blocking writer")
	}

	params := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
		URI: "/api/queued?id=1",
	}, ncmcrypto.XeapiSession{ID: sessionID, Key: sessionKey})

	requestURL, err := url.Parse("https://music.163.com/xeapi/queued?" + params.Encode())
	if err != nil {
		t.Fatal(err)
	}

	request := &http.Request{Method: http.MethodGet, URL: requestURL, Header: make(http.Header), Body: http.NoBody}
	state := &captureState{
		session: 1, requestMethod: request.Method, requestURL: requestURL, requestHeader: request.Header.Clone(),
	}
	prepareRequestCapture(state, request, 1<<20, recorder)

	if err := sessions.learnResponseHeaders(http.Header{
		"X-Encr-Ssid": {sessionID}, "X-Encr-Sskey": {sessionKey},
	}); err != nil {
		t.Fatal(err)
	}

	writer.unblock()

	if !flushRecorder(recorder, time.Second) {
		t.Fatal("recorder did not flush")
	}

	output := writer.String()
	if !strings.Contains(output, "REQUEST protocol=xeapi decode=partial") || strings.Contains(output, "REQUEST protocol=xeapi decode=decrypted") {
		t.Fatalf("queued request used a session learned from its later response: %s", output)
	}
}

func TestRecorderRedactsXeapiSessionID(t *testing.T) {
	const (
		sessionID = "sensitive-session-12345"
	)

	t.Run("redacted when showSensitive is false", func(t *testing.T) {
		var buf bytes.Buffer

		r := newRecorder(&buf, 1024, false)
		defer r.Close()

		sid := sessionID
		decoded := decodeResult{
			protocol:  protocolXEAPI,
			status:    decodeStatusDecrypted,
			sessionID: &sid,
		}

		state := &captureState{
			session:       1,
			started:       time.Now(),
			requestMethod: "POST",
		}

		r.writeRequestBlock(state, &decoded, "")

		out := buf.String()
		if !strings.Contains(out, "xeapi-session-id: [REDACTED]") {
			t.Fatalf("expected xeapi-session-id to be redacted, got: %s", out)
		}

		if strings.Contains(out, sessionID) {
			t.Fatalf("sensitive sessionID leaked in output: %s", out)
		}
	})

	t.Run("exposed when showSensitive is true", func(t *testing.T) {
		var buf bytes.Buffer

		r := newRecorder(&buf, 1024, true)
		defer r.Close()

		sid := sessionID
		decoded := decodeResult{
			protocol:  protocolXEAPI,
			status:    decodeStatusDecrypted,
			sessionID: &sid,
		}

		state := &captureState{
			session:       1,
			started:       time.Now(),
			requestMethod: "POST",
		}

		r.writeRequestBlock(state, &decoded, "")

		out := buf.String()
		if !strings.Contains(out, "xeapi-session-id: "+sessionID) {
			t.Fatalf("expected raw sessionID when showSensitive is true, got: %s", out)
		}
	})
}

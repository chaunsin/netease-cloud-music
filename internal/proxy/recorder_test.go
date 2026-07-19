// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
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

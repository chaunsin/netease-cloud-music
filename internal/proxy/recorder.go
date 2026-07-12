// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	recorderQueueCapacity = 32
	recorderCloseTimeout  = 250 * time.Millisecond
)

// requestRecord coordinates request decoding with later response logging. It
// uses callbacks rather than having an output worker wait, so a full-duplex
// response can finish before its request body without deadlocking the queue.
type requestRecord struct {
	mu        sync.Mutex
	started   bool
	completed bool
	result    decodeResult
	callbacks []func(decodeResult)
}

func newRequestRecord(requestURL *url.URL) (*requestRecord, decodeResult) {
	path := ""
	if requestURL != nil {
		path = requestURL.Path
	}
	result := decodeResult{
		protocol: classifyProtocol(path),
		status:   decodeStatusPlaintext,
		apiPath:  path,
	}
	if requestURL != nil {
		// Preserve the response-encryption hint if overload drops full decoding.
		result.responseEncrypted = valuesRequestEncrypted(requestURL.Query())
	}
	switch result.protocol {
	case protocolWEAPI:
		result.status = decodeStatusUnsupported
	case protocolXEAPI:
		result.status = decodeStatusUnsupported
	}
	return &requestRecord{}, result
}

func (record *requestRecord) begin() bool {
	if record == nil {
		return false
	}
	record.mu.Lock()
	defer record.mu.Unlock()
	if record.started || record.completed {
		return false
	}
	record.started = true
	return true
}

func (record *requestRecord) complete(result decodeResult) {
	if record == nil {
		return
	}
	record.mu.Lock()
	if record.completed {
		record.mu.Unlock()
		return
	}
	record.completed = true
	record.result = result
	callbacks := record.callbacks
	record.callbacks = nil
	record.mu.Unlock()
	for _, callback := range callbacks {
		callback(result)
	}
}

func (record *requestRecord) onComplete(callback func(decodeResult)) {
	if record == nil || callback == nil {
		return
	}
	record.mu.Lock()
	if !record.completed {
		record.callbacks = append(record.callbacks, callback)
		record.mu.Unlock()
		return
	}
	result := record.result
	record.mu.Unlock()
	callback(result)
}

type recorder struct {
	out           io.Writer
	maxBodyBytes  int64
	showSensitive bool

	submitMu   sync.Mutex
	tasks      chan func()
	workerDone chan struct{}
	closeOnce  sync.Once
	closed     bool
	dropped    atomic.Uint64
}

func newRecorder(out io.Writer, maxBodyBytes int64, showSensitive bool) *recorder {
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultJSONDisplayLimit
	}
	r := &recorder{
		out:           out,
		maxBodyBytes:  maxBodyBytes,
		showSensitive: showSensitive,
		tasks:         make(chan func(), recorderQueueCapacity),
		workerDone:    make(chan struct{}),
	}
	go r.run()
	return r
}

// Close prevents future captures and waits only briefly for the worker. A
// blocked stdout/FIFO must never delay proxy shutdown indefinitely.
func (r *recorder) Close() {
	r.CloseWithTimeout(recorderCloseTimeout)
}

func (r *recorder) CloseWithTimeout(timeout time.Duration) {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		r.submitMu.Lock()
		r.closed = true
		close(r.tasks)
		r.submitMu.Unlock()
	})
	if timeout <= 0 {
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-r.workerDone:
	case <-timer.C:
	}
}

func (r *recorder) run() {
	defer close(r.workerDone)
	for task := range r.tasks {
		r.writeDroppedNotice()
		task()
	}
}

func (r *recorder) submit(task func()) bool {
	if r == nil || task == nil {
		return false
	}
	r.submitMu.Lock()
	defer r.submitMu.Unlock()
	if r.closed {
		return false
	}
	select {
	case r.tasks <- task:
		return true
	default:
		r.dropped.Add(1)
		return false
	}
}

func (r *recorder) writeDroppedNotice() {
	if count := r.dropped.Swap(0); count > 0 {
		r.writeBlock([]byte(fmt.Sprintf("[%s] CAPTURE_DROPPED count=%d reason=output_queue_full\n", time.Now().Format(time.RFC3339Nano), count)))
	}
}

func (r *recorder) finishRequest(record *requestRecord, state *captureState) {
	if record == nil || !record.begin() {
		return
	}
	provisional := state.requestDecoded
	if !r.submit(func() {
		body, captureDetail := r.bodyForDisplay(state.requestBody)
		decoded := decodeRequestLimited(state.requestMethod, state.requestURL, state.requestHeader, body, r.showSensitive, r.maxBodyBytes)
		detail := joinDetails(decoded.detail, captureDetail, snapshotDetail(state.requestBody))
		r.writeRequestBlock(state, decoded, detail)
		// Complete only after the request block is emitted so response records
		// stay ordered whenever stdout is able to make progress.
		record.complete(decoded)
	}) {
		record.complete(provisional)
	}
}

func (r *recorder) recordResponse(state *captureState, response *http.Response) {
	if state == nil || response == nil {
		return
	}
	metadata := cloneResponseMetadata(response)
	recordResponse := func(request decodeResult) {
		r.submit(func() {
			body, captureDetail := r.bodyForDisplay(state.responseBody)
			decoded := decodeResponse(request, metadata.Header, body, r.maxBodyBytes, r.showSensitive)
			detail := joinDetails(decoded.detail, captureDetail, snapshotDetail(state.responseBody))
			r.writeResponseBlock(state, metadata, decoded, detail)
		})
	}
	if state.requestRecord != nil {
		state.requestRecord.onComplete(recordResponse)
		return
	}
	recordResponse(state.requestDecoded)
}

func (r *recorder) recordResponseError(state *captureState, responseErr error) {
	if state == nil {
		return
	}
	recordError := func(_ decodeResult) {
		r.submit(func() {
			message := "response unavailable"
			if responseErr != nil {
				message = responseErr.Error()
			}
			var block bytes.Buffer
			fmt.Fprintf(&block, "[%s] #%06d RESPONSE_ERROR duration=%s\n",
				time.Now().Format(time.RFC3339Nano),
				state.session,
				time.Since(state.started).Round(time.Millisecond),
			)
			fmt.Fprintf(&block, "%s %s\n", escapeLogField(state.requestMethod), escapeLogField(redactURL(state.requestURL, r.showSensitive)))
			fmt.Fprintf(&block, "error: %s\n", escapeLogField(r.redactDiagnostic(message)))
			r.writeBlock(block.Bytes())
		})
	}
	if state.requestRecord != nil {
		state.requestRecord.onComplete(recordError)
		return
	}
	recordError(state.requestDecoded)
}

func (r *recorder) writeRequestBlock(state *captureState, decoded decodeResult, detail string) {
	var block bytes.Buffer
	fmt.Fprintf(&block, "[%s] #%06d REQUEST protocol=%s decode=%s\n",
		time.Now().Format(time.RFC3339Nano), state.session, decoded.protocol, decoded.status)
	fmt.Fprintf(&block, "%s %s\n", escapeLogField(state.requestMethod), escapeLogField(redactURL(state.requestURL, r.showSensitive)))
	if decoded.responseEncrypted {
		fmt.Fprintln(&block, "response-encrypted: true")
	}
	if decoded.apiPath != "" && (state.requestURL == nil || decoded.apiPath != state.requestURL.Path) {
		fmt.Fprintf(&block, "api-path: %s\n", escapeLogField(decoded.apiPath))
	}
	if len(decoded.query) > 0 {
		r.writeSection(&block, "query", decoded.query)
	}
	writeHeaders(&block, state.requestHeader, r.showSensitive)
	r.writeBody(&block, state.requestBody, decoded.body)
	if detail != "" {
		fmt.Fprintf(&block, "detail: %s\n", escapeLogField(r.redactDiagnostic(detail)))
	}
	r.writeBlock(block.Bytes())
}

func (r *recorder) writeResponseBlock(state *captureState, response *http.Response, decoded decodeResult, detail string) {
	var block bytes.Buffer
	fmt.Fprintf(&block, "[%s] #%06d RESPONSE status=%d duration=%s protocol=%s decode=%s\n",
		time.Now().Format(time.RFC3339Nano),
		state.session,
		response.StatusCode,
		time.Since(state.started).Round(time.Millisecond),
		decoded.protocol,
		decoded.status,
	)
	fmt.Fprintf(&block, "%s %s\n", escapeLogField(state.requestMethod), escapeLogField(redactURL(state.requestURL, r.showSensitive)))
	writeHeaders(&block, response.Header, r.showSensitive)
	r.writeBody(&block, state.responseBody, decoded.body)
	if detail != "" {
		fmt.Fprintf(&block, "detail: %s\n", escapeLogField(r.redactDiagnostic(detail)))
	}
	r.writeBlock(block.Bytes())
}

func (r *recorder) redactDiagnostic(value string) string {
	return string(redactDiagnostic([]byte(value), r.showSensitive))
}

func (r *recorder) bodyForDisplay(snapshot bodySnapshot) ([]byte, string) {
	if snapshot.omittedReason != "" || len(snapshot.raw) == 0 {
		return snapshot.raw, ""
	}
	if snapshot.contentEncode == "" {
		return snapshot.raw, ""
	}
	if snapshot.truncated {
		return snapshot.raw, "content-encoded body exceeded the capture limit and was not decoded"
	}
	decoded, truncated, err := decodeHTTPContent(snapshot.raw, snapshot.contentEncode, r.maxBodyBytes)
	if err != nil {
		return snapshot.raw, "HTTP content decoding failed: " + err.Error()
	}
	if truncated {
		return decoded, "decoded HTTP body exceeded the display limit"
	}
	return decoded, ""
}

func (r *recorder) writeBody(block *bytes.Buffer, snapshot bodySnapshot, body []byte) {
	contentType := snapshot.contentType
	if contentType == "" {
		contentType = "unknown"
	}
	fmt.Fprintf(block, "body: content-type=%q content-length=%d captured=%d",
		escapeLogField(contentType), snapshot.contentLength, len(snapshot.raw))
	if snapshot.contentEncode != "" {
		fmt.Fprintf(block, " content-encoding=%q", escapeLogField(snapshot.contentEncode))
	}
	if snapshot.truncated {
		fmt.Fprint(block, " truncated=true")
	}
	fmt.Fprintln(block)

	if snapshot.omittedReason != "" {
		fmt.Fprintf(block, "<%s>\n", escapeLogField(snapshot.omittedReason))
		return
	}
	if len(body) == 0 {
		fmt.Fprintln(block, "<empty>")
		return
	}

	printable, encoding, truncated := terminalBody(body, r.maxBodyBytes)
	if encoding != "" {
		fmt.Fprintf(block, "[%s]\n", encoding)
	}
	block.WriteString(printable)
	if !strings.HasSuffix(printable, "\n") {
		block.WriteByte('\n')
	}
	if truncated {
		fmt.Fprintln(block, "<formatted output truncated>")
	}
}

func (r *recorder) writeSection(block *bytes.Buffer, name string, body []byte) {
	printable, encoding, truncated := terminalBody(body, r.maxBodyBytes)
	fmt.Fprintf(block, "%s:\n", escapeLogField(name))
	if encoding != "" {
		fmt.Fprintf(block, "[%s]\n", encoding)
	}
	block.WriteString(printable)
	if !strings.HasSuffix(printable, "\n") {
		block.WriteByte('\n')
	}
	if truncated {
		fmt.Fprintf(block, "<%s output truncated>\n", escapeLogField(name))
	}
}

// A single recorder worker serializes all production calls to writeBlock.
func (r *recorder) writeBlock(block []byte) {
	if r.out == nil {
		return
	}
	if len(block) == 0 || block[len(block)-1] != '\n' {
		block = append(block, '\n')
	}
	_, _ = r.out.Write(append(block, '\n'))
}

func writeHeaders(block *bytes.Buffer, headers http.Header, showSensitive bool) {
	fmt.Fprintln(block, "headers:")
	redacted := redactHeaders(headers, showSensitive)
	keys := make([]string, 0, len(redacted))
	for key := range redacted {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		fmt.Fprintln(block, "  <none>")
		return
	}
	for _, key := range keys {
		for _, value := range redacted[key] {
			fmt.Fprintf(block, "  %s: %s\n", escapeLogField(key), escapeLogField(value))
		}
	}
}

// terminalBody only prints literal line breaks for JSON that our formatter has
// already serialized. Other text containing controls is base64-encoded so it
// cannot forge a new request/response block in a terminal or redirected log.
func terminalBody(body []byte, limit int64) (string, string, bool) {
	if limit <= 0 {
		return "", "", len(body) > 0
	}
	if utf8.Valid(body) && (json.Valid(body) || !containsUnsafeTerminalControl(body)) {
		printable, truncated := truncateUTF8Bytes(body, limit)
		return string(printable), "", truncated
	}

	maxInput := (limit / 4) * 3
	if maxInput <= 0 {
		return "", "base64", len(body) > 0
	}
	if int64(len(body)) > maxInput {
		body = body[:maxInput]
		return base64.StdEncoding.EncodeToString(body), "base64", true
	}
	return base64.StdEncoding.EncodeToString(body), "base64", false
}

func containsUnsafeTerminalControl(body []byte) bool {
	for _, runeValue := range string(body) {
		if unicode.IsControl(runeValue) {
			return true
		}
	}
	return false
}

func truncateUTF8Bytes(value []byte, limit int64) ([]byte, bool) {
	if int64(len(value)) <= limit {
		return value, false
	}
	value = value[:limit]
	for !utf8.Valid(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return value, true
}

func joinDetails(details ...string) string {
	seen := make(map[string]struct{}, len(details))
	result := make([]string, 0, len(details))
	for _, detail := range details {
		detail = strings.TrimSpace(detail)
		if detail == "" {
			continue
		}
		if _, ok := seen[detail]; ok {
			continue
		}
		seen[detail] = struct{}{}
		result = append(result, detail)
	}
	return strings.Join(result, "; ")
}

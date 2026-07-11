// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var errBodyClosedBeforeEOF = errors.New("body closed before EOF")

type bodySnapshot struct {
	raw           []byte
	contentType   string
	contentEncode string
	contentLength int64
	truncated     bool
	omittedReason string
	captureErr    error
}

type replayReadCloser struct {
	io.Reader
	closer io.Closer
}

func (r *replayReadCloser) Close() error {
	if r.closer == nil {
		return nil
	}
	return r.closer.Close()
}

func snapshotBody(body io.ReadCloser, header http.Header, contentLength, limit int64, path string) (bodySnapshot, io.ReadCloser) {
	snapshot := newBodySnapshot(header, contentLength)
	if body == nil || body == http.NoBody {
		return snapshot, body
	}
	if reason := bodyOmissionReason(snapshot.contentType, path); reason != "" {
		snapshot.omittedReason = reason
		return snapshot, body
	}

	consumed, err := io.ReadAll(io.LimitReader(body, captureReadLimit(limit)))
	restored := &replayReadCloser{
		Reader: io.MultiReader(bytes.NewReader(consumed), body),
		closer: body,
	}
	snapshot.captureErr = err
	if int64(len(consumed)) > limit {
		snapshot.raw = append([]byte(nil), consumed[:limit]...)
		snapshot.truncated = true
	} else {
		snapshot.raw = append([]byte(nil), consumed...)
	}
	return snapshot, restored
}

func captureReadLimit(limit int64) int64 {
	if limit >= math.MaxInt64 {
		return math.MaxInt64
	}
	return limit + 1
}

func newBodySnapshot(header http.Header, contentLength int64) bodySnapshot {
	return bodySnapshot{
		contentType:   header.Get("Content-Type"),
		contentEncode: header.Get("Content-Encoding"),
		contentLength: contentLength,
	}
}

type captureReadCloser struct {
	body     io.ReadCloser
	limit    int64
	snapshot bodySnapshot
	onDone   func(bodySnapshot)

	mu          sync.Mutex
	cond        *sync.Cond
	captured    []byte
	truncated   bool
	readBytes   int64
	terminalErr error
	completed   bool
	closing     bool
	activeReads int
	closeOnce   sync.Once
	closeErr    error
}

func newCaptureReadCloser(body io.ReadCloser, snapshot bodySnapshot, limit int64, onDone func(bodySnapshot)) *captureReadCloser {
	reader := &captureReadCloser{
		body:     body,
		limit:    limit,
		snapshot: snapshot,
		onDone:   onDone,
	}
	reader.cond = sync.NewCond(&reader.mu)
	return reader
}

func (r *captureReadCloser) Read(p []byte) (int, error) {
	r.mu.Lock()
	if r.closing {
		r.mu.Unlock()
		return 0, net.ErrClosed
	}
	r.activeReads++
	r.mu.Unlock()

	n, err := r.body.Read(p)

	r.mu.Lock()
	if n > 0 {
		r.readBytes += int64(n)
		remaining := r.limit - int64(len(r.captured))
		if remaining > 0 {
			captureLen := int64(n)
			if captureLen > remaining {
				captureLen = remaining
			}
			r.captured = append(r.captured, p[:captureLen]...)
		}
		if int64(n) > remaining {
			r.truncated = true
		}
	}
	if err != nil {
		r.completed = true
		if !errors.Is(err, io.EOF) && r.terminalErr == nil {
			r.terminalErr = err
		}
	}
	r.activeReads--
	if r.activeReads == 0 {
		r.cond.Broadcast()
	}
	r.mu.Unlock()
	return n, err
}

func (r *captureReadCloser) Close() error {
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closing = true
		r.mu.Unlock()

		r.closeErr = r.body.Close()

		r.mu.Lock()
		for r.activeReads > 0 {
			r.cond.Wait()
		}
		snapshot := r.snapshot
		snapshot.raw = append([]byte(nil), r.captured...)
		snapshot.truncated = r.truncated
		snapshot.captureErr = r.terminalErr
		bodyComplete := r.completed || (snapshot.contentLength >= 0 && r.readBytes >= snapshot.contentLength)
		if r.closeErr != nil {
			snapshot.captureErr = r.closeErr
		} else if !bodyComplete && snapshot.captureErr == nil {
			snapshot.captureErr = errBodyClosedBeforeEOF
		}
		r.mu.Unlock()
		if r.onDone != nil {
			r.onDone(snapshot)
		}
	})
	return r.closeErr
}

func bodyOmissionReason(contentType, path string) string {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch {
	case strings.HasPrefix(mediaType, "audio/"):
		return "audio body omitted"
	case strings.HasPrefix(mediaType, "video/"):
		return "video body omitted"
	case strings.HasPrefix(mediaType, "image/"):
		return "image body omitted"
	case strings.HasPrefix(mediaType, "multipart/"):
		return "multipart body omitted"
	case mediaType == "text/event-stream":
		return "streaming body omitted"
	case mediaType == "application/octet-stream" && !isAPIPath(path):
		return "binary body omitted"
	case mediaType == "application/zip" || mediaType == "application/x-protobuf":
		return "binary body omitted"
	default:
		return ""
	}
}

func isAPIPath(path string) bool {
	path = strings.ToLower(path)
	return strings.HasPrefix(path, "/api/") ||
		strings.HasPrefix(path, "/weapi/") ||
		strings.HasPrefix(path, "/eapi/") ||
		strings.HasPrefix(path, "/xeapi/") ||
		path == "/batch"
}

func snapshotDetail(snapshot bodySnapshot) string {
	switch {
	case snapshot.omittedReason != "":
		return snapshot.omittedReason
	case snapshot.captureErr != nil:
		return fmt.Sprintf("body capture failed: %v", snapshot.captureErr)
	case snapshot.truncated:
		return "display truncated"
	default:
		return ""
	}
}

func cloneURL(input *url.URL) *url.URL {
	if input == nil {
		return &url.URL{}
	}
	cloned := *input
	return &cloned
}

// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/require"
)

func TestSnapshotBodyRestoresCompleteStream(t *testing.T) {
	t.Parallel()

	original := bytes.Repeat([]byte("body-"), 100)
	snapshot, restored := snapshotBody(
		io.NopCloser(bytes.NewReader(original)),
		http.Header{"Content-Type": []string{"application/json"}},
		int64(len(original)),
		64,
		"/api/test",
	)

	require.True(t, snapshot.truncated)
	require.Len(t, snapshot.raw, 64)
	forwarded, err := io.ReadAll(restored)
	require.NoError(t, err)
	require.Equal(t, original, forwarded)
	require.NoError(t, restored.Close())
}

func TestSnapshotBodySkipsBinaryWithoutConsuming(t *testing.T) {
	t.Parallel()

	original := []byte{0x01, 0x02, 0x03}
	body := io.NopCloser(bytes.NewReader(original))
	snapshot, restored := snapshotBody(
		body,
		http.Header{"Content-Type": []string{"audio/mpeg"}},
		int64(len(original)),
		64,
		"/song.mp3",
	)

	require.Equal(t, "audio body omitted", snapshot.omittedReason)
	forwarded, err := io.ReadAll(restored)
	require.NoError(t, err)
	require.Equal(t, original, forwarded)
}

func TestCaptureReadLimitSaturatesAtMaxInt64(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(math.MaxInt64), captureReadLimit(math.MaxInt64))
	require.Equal(t, int64(1025), captureReadLimit(1024))
}

func TestBodyOmissionReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		contentType string
		path        string
		want        string
	}{
		{contentType: "audio/mpeg", path: "/song.mp3", want: "audio body omitted"},
		{contentType: "video/mp4", path: "/video.mp4", want: "video body omitted"},
		{contentType: "image/jpeg", path: "/cover.jpg", want: "image body omitted"},
		{contentType: "multipart/form-data; boundary=test", path: "/api/upload", want: "multipart body omitted"},
		{contentType: "text/event-stream", path: "/api/events", want: "streaming body omitted"},
		{contentType: "application/octet-stream", path: "/file", want: "binary body omitted"},
		{contentType: "application/octet-stream", path: "/api/blob", want: ""},
		{contentType: "application/zip", path: "/archive", want: "binary body omitted"},
		{contentType: "application/x-protobuf", path: "/api/data", want: "binary body omitted"},
	}
	for _, test := range tests {
		if got := bodyOmissionReason(test.contentType, test.path); got != test.want {
			t.Errorf("bodyOmissionReason(%q, %q) = %q, want %q", test.contentType, test.path, got, test.want)
		}
	}
}

func TestCaptureReadCloserFinalizesOnClose(t *testing.T) {
	t.Parallel()

	original := []byte("complete response")
	snapshots := make(chan bodySnapshot, 2)
	reader := newCaptureReadCloser(
		&eofWithDataReadCloser{data: original, terminalErr: io.EOF},
		newBodySnapshot(http.Header{"Content-Type": []string{"text/plain"}}, int64(len(original))),
		int64(len(original)),
		func(snapshot bodySnapshot) { snapshots <- snapshot },
	)

	buffer := make([]byte, len(original))
	n, err := reader.Read(buffer)
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, len(original), n)
	require.Equal(t, original, buffer)
	require.Empty(t, snapshots, "response logging must not run in the Read hot path")

	require.NoError(t, reader.Close())
	snapshot := <-snapshots
	require.Equal(t, original, snapshot.raw)
	require.NoError(t, snapshot.captureErr)
	require.NoError(t, reader.Close())
	require.Empty(t, snapshots, "Close must finalize exactly once")
}

func TestCaptureReadCloserReportsEarlyClose(t *testing.T) {
	t.Parallel()

	snapshots := make(chan bodySnapshot, 1)
	reader := newCaptureReadCloser(
		io.NopCloser(bytes.NewReader([]byte("unread"))),
		newBodySnapshot(http.Header{}, 6),
		64,
		func(snapshot bodySnapshot) { snapshots <- snapshot },
	)

	require.NoError(t, reader.Close())
	snapshot := <-snapshots
	require.ErrorIs(t, snapshot.captureErr, errBodyClosedBeforeEOF)
	require.Empty(t, snapshot.raw)
}

func TestCaptureReadCloserWaitsForInFlightRead(t *testing.T) {
	t.Parallel()

	original := []byte("bytes returned while closing")
	underlying := &closeUnblocksReadCloser{
		data:    original,
		started: make(chan struct{}),
		closed:  make(chan struct{}),
	}
	snapshots := make(chan bodySnapshot, 1)
	reader := newCaptureReadCloser(
		underlying,
		newBodySnapshot(http.Header{}, int64(len(original))),
		int64(len(original)),
		func(snapshot bodySnapshot) { snapshots <- snapshot },
	)
	readResult := make(chan struct {
		body []byte
		err  error
	}, 1)
	go func() {
		buffer := make([]byte, len(original))
		n, err := reader.Read(buffer)
		readResult <- struct {
			body []byte
			err  error
		}{body: buffer[:n], err: err}
	}()

	<-underlying.started
	closeResult := make(chan error, 1)
	go func() { closeResult <- reader.Close() }()
	select {
	case err := <-closeResult:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not wait for the in-flight read")
	}
	result := <-readResult
	require.ErrorIs(t, result.err, io.EOF)
	require.Equal(t, original, result.body)
	snapshot := <-snapshots
	require.Equal(t, original, snapshot.raw)
	require.NoError(t, snapshot.captureErr)
}

func TestDecodeHTTPContent(t *testing.T) {
	t.Parallel()

	original := bytes.Repeat([]byte("decoded body "), 20)
	tests := []struct {
		name     string
		encoding string
		encode   func(*testing.T, []byte) []byte
	}{
		{
			name:     "gzip",
			encoding: "gzip",
			encode: func(t *testing.T, input []byte) []byte {
				var output bytes.Buffer
				writer := gzip.NewWriter(&output)
				_, err := writer.Write(input)
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				return output.Bytes()
			},
		},
		{
			name:     "zlib deflate",
			encoding: "deflate",
			encode: func(t *testing.T, input []byte) []byte {
				var output bytes.Buffer
				writer := zlib.NewWriter(&output)
				_, err := writer.Write(input)
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				return output.Bytes()
			},
		},
		{
			name:     "raw deflate",
			encoding: "deflate",
			encode: func(t *testing.T, input []byte) []byte {
				var output bytes.Buffer
				writer, err := flate.NewWriter(&output, flate.DefaultCompression)
				require.NoError(t, err)
				_, err = writer.Write(input)
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				return output.Bytes()
			},
		},
		{
			name:     "brotli",
			encoding: "br",
			encode: func(t *testing.T, input []byte) []byte {
				var output bytes.Buffer
				writer := brotli.NewWriter(&output)
				_, err := writer.Write(input)
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				return output.Bytes()
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			decoded, truncated, err := decodeHTTPContent(test.encode(t, original), test.encoding, 1<<20)
			require.NoError(t, err)
			require.False(t, truncated)
			require.Equal(t, original, decoded)
		})
	}
}

func TestDecodeHTTPContentLimitsExpandedBody(t *testing.T) {
	t.Parallel()

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write(bytes.Repeat([]byte("x"), 4096))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	decoded, truncated, err := decodeHTTPContent(compressed.Bytes(), "gzip", 128)
	require.NoError(t, err)
	require.True(t, truncated)
	require.Len(t, decoded, 128)
}

type eofWithDataReadCloser struct {
	data        []byte
	terminalErr error
	read        bool
}

func (r *eofWithDataReadCloser) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	return copy(p, r.data), r.terminalErr
}

func (*eofWithDataReadCloser) Close() error { return nil }

type closeUnblocksReadCloser struct {
	data    []byte
	started chan struct{}
	closed  chan struct{}
}

func (r *closeUnblocksReadCloser) Read(p []byte) (int, error) {
	close(r.started)
	<-r.closed
	return copy(p, r.data), io.EOF
}

func (r *closeUnblocksReadCloser) Close() error {
	close(r.closed)
	return nil
}

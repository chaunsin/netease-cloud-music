// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type downloadRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f downloadRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type trackedDownloadBody struct {
	reader     io.Reader
	closeCalls int
}

func (b *trackedDownloadBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *trackedDownloadBody) Close() error {
	b.closeCalls++
	return nil
}

type failingDownloadWriter struct {
	err error
}

func (w failingDownloadWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestDownloadClosesResponseBodyBeforeReturning(t *testing.T) {
	t.Parallel()

	const content = "downloaded music"

	body := &trackedDownloadBody{reader: bytes.NewBufferString(content)}
	client := newDownloadTestClient(body, http.StatusOK, int64(len(content)))

	var output bytes.Buffer

	response, err := client.Download(context.Background(), "https://example.com/song", nil, nil, &output, nil) //nolint:bodyclose // Download closes the response body before returning.
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, content, output.String())
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, int64(len(content)), response.ContentLength)
	assert.Equal(t, 1, body.closeCalls)
}

func TestDownloadClosesResponseBodyOnError(t *testing.T) {
	t.Parallel()

	errDownloadWrite := errors.New("download writer failed")

	tests := []struct {
		name          string
		statusCode    int
		contentLength int64
		writer        io.Writer
		wantError     string
		wantErrorIs   error
	}{
		{
			name:          "non-success status",
			statusCode:    http.StatusBadGateway,
			contentLength: 4,
			writer:        io.Discard,
			wantError:     "http status code: 502",
		},
		{
			name:          "writer failure",
			statusCode:    http.StatusOK,
			contentLength: 4,
			writer:        failingDownloadWriter{err: errDownloadWrite},
			wantErrorIs:   errDownloadWrite,
		},
		{
			name:          "content length mismatch",
			statusCode:    http.StatusOK,
			contentLength: 5,
			writer:        io.Discard,
			wantError:     "file transfer interrupted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := &trackedDownloadBody{reader: bytes.NewBufferString("body")}
			client := newDownloadTestClient(body, tt.statusCode, tt.contentLength)

			response, err := client.Download(context.Background(), "https://example.com/song", nil, nil, tt.writer, nil) //nolint:bodyclose // Download closes the response body before returning.
			if tt.wantErrorIs != nil {
				require.ErrorIs(t, err, tt.wantErrorIs)
			} else {
				require.EqualError(t, err, tt.wantError)
			}

			assert.Nil(t, response)
			assert.Equal(t, 1, body.closeCalls)
		})
	}
}

func newDownloadTestClient(body io.ReadCloser, statusCode int, contentLength int64) *Client {
	restyClient := resty.New()
	restyClient.SetTransport(downloadRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    statusCode,
			Header:        make(http.Header),
			Body:          body,
			ContentLength: contentLength,
		}, nil
	}))

	return &Client{cli: restyClient}
}

// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mediaRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f mediaRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type mediaTrackingBody struct {
	io.Reader

	closed bool
}

func (body *mediaTrackingBody) Close() error {
	body.closed = true
	return nil
}

func TestSniffImageSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sample  []byte
		want    string
		wantErr bool
	}{
		{name: "JPEG", sample: []byte{0xff, 0xd8, 0xff, 0xe0}, want: ".jpg"},
		{name: "PNG", sample: []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, want: ".png"},
		{name: "GIF", sample: []byte("GIF89a"), want: ".gif"},
		{name: "WebP", sample: []byte("RIFF1234WEBPVP8 "), want: ".webp"},
		{name: "empty", wantErr: true},
		{name: "text", sample: []byte("plain text"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := sniffImageSuffix(tt.sample)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateMediaURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rawURL  string
		wantErr string
	}{
		{name: "NetEase 163", rawURL: "https://p1.music.163.com/cover.jpg"},
		{name: "NetEase 126", rawURL: "https://m10.music.126.net/audio.mp3"},
		{name: "explicit HTTPS port", rawURL: "https://music.163.com:443/cover.jpg"},
		{name: "HTTP", rawURL: "http://p1.music.163.com/cover.jpg", wantErr: "must use HTTPS"},
		{name: "lookalike host", rawURL: "https://music.163.com.example.org/cover.jpg", wantErr: "not allowed"},
		{name: "parent lookalike", rawURL: "https://evilmusic.163.com/cover.jpg", wantErr: "not allowed"},
		{name: "userinfo", rawURL: "https://user@music.163.com/cover.jpg", wantErr: "user information"},
		{name: "custom port", rawURL: "https://music.163.com:8443/cover.jpg", wantErr: "default HTTPS port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateMediaURL(tt.rawURL)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestNormalizeMediaURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rawURL  string
		want    string
		wantErr string
	}{
		{
			name:   "HTTPS remains unchanged",
			rawURL: "https://m701.music.126.net/audio.mp3?token=value",
			want:   "https://m701.music.126.net/audio.mp3?token=value",
		},
		{
			name:   "allowlisted HTTP is upgraded",
			rawURL: "http://m701.music.126.net/audio.mp3?token=value",
			want:   "https://m701.music.126.net/audio.mp3?token=value",
		},
		{name: "HTTP lookalike", rawURL: "http://music.126.net.example.org/audio.mp3", wantErr: "not allowed"},
		{name: "HTTP userinfo", rawURL: "http://user@music.126.net/audio.mp3", wantErr: "user information"},
		{name: "HTTP custom port", rawURL: "http://music.126.net:80/audio.mp3", wantErr: "default port"},
		{name: "unsupported scheme", rawURL: "ftp://music.126.net/audio.mp3", wantErr: "HTTP or HTTPS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeMediaURL(tt.rawURL)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateMediaIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		address string
		valid   bool
	}{
		{address: "8.8.8.8", valid: true},
		{address: "2606:4700:4700::1111", valid: true},
		{address: "0.0.0.0"},
		{address: "127.0.0.1"},
		{address: "10.0.0.1"},
		{address: "169.254.1.1"},
		{address: "224.0.0.1"},
		{address: "::1"},
		{address: "fc00::1"},
		{address: "fe80::1"},
		{address: "ff02::1"},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			t.Parallel()

			err := validateMediaIP(netip.MustParseAddr(tt.address))
			if tt.valid {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, "disallowed address")
		})
	}
}

func TestHardenedMediaClientRedirectPolicyAndTLS(t *testing.T) {
	client := newHardenedMediaClient(time.Second)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Nil(t, transport.Proxy)

	if transport.TLSClientConfig != nil {
		assert.False(t, transport.TLSClientConfig.InsecureSkipVerify)
	}

	request, err := http.NewRequest(http.MethodGet, "https://p1.music.163.com/next", http.NoBody)
	require.NoError(t, err)
	require.NoError(t, client.CheckRedirect(request, nil))

	downgrade, err := http.NewRequest(http.MethodGet, "http://p1.music.163.com/next", http.NoBody)
	require.NoError(t, err)
	require.ErrorContains(t, client.CheckRedirect(downgrade, nil), "must use HTTPS")

	require.NoError(t, client.CheckRedirect(request, make([]*http.Request, 5)))
	require.ErrorContains(t, client.CheckRedirect(request, make([]*http.Request, 6)), "exceeded 5 redirects")
}

func TestHardenedMediaDialerRejectsAnyUnsafeResolvedAddress(t *testing.T) {
	dial := hardenedMediaDialerWithLookup(time.Second, func(_ context.Context, network, host string) ([]netip.Addr, error) {
		assert.Equal(t, "ip", network)
		assert.Equal(t, "p1.music.163.com", host)
		return []netip.Addr{
			netip.MustParseAddr("8.8.8.8"),
			netip.MustParseAddr("127.0.0.1"),
		}, nil
	})

	connection, err := dial(context.Background(), "tcp", "p1.music.163.com:443")
	assert.Nil(t, connection)
	require.ErrorContains(t, err, "disallowed address 127.0.0.1")
}

func TestHardenedMediaDialerRejectsMissingAddressFamily(t *testing.T) {
	dial := hardenedMediaDialerWithLookup(time.Second, func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("2606:4700:4700::1111")}, nil
	})

	connection, err := dial(context.Background(), "tcp4", "p1.music.163.com:443")
	assert.Nil(t, connection)
	require.ErrorContains(t, err, "no address for network tcp4")
}

func TestMediaHelpersRejectBeforeNetwork(t *testing.T) {
	_, cleanup, err := downloadMediaToTemp(
		context.Background(),
		mediaStrict,
		time.Second,
		"http://127.0.0.1/cover.jpg",
		1024,
		"ncmctl-yunbei-test-*",
	)
	cleanup()
	require.ErrorContains(t, err, "not allowed")

	err = fetchMediaAudioSample(context.Background(), mediaStrict, time.Second, "https://127.0.0.1/audio.mp3", 1024)
	require.ErrorContains(t, err, "not allowed")
}

func TestDownloadMediaToTempDetectsSuffixAndLimit(t *testing.T) {
	t.Parallel()

	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	payload := make([]byte, 0, len(pngHeader)+8)
	payload = append(payload, pngHeader...)
	payload = append(payload, make([]byte, 8)...)
	clientFor := func(body []byte) *http.Client {
		return &http.Client{Transport: mediaRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
			assert.Equal(t, "https://p1.music.126.net/cover", request.URL.String())
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Request:    request,
			}, nil
		})}
	}

	path, cleanup, err := downloadMediaToTempWithClient(
		context.Background(),
		mediaStrict,
		clientFor(payload),
		"http://p1.music.126.net/cover",
		int64(len(payload)),
		"ncmctl-yunbei-download-",
	)
	require.NoError(t, err)
	assert.Equal(t, ".png", filepath.Ext(path))
	written, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, payload, written)
	cleanup()

	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)

	path, cleanup, err = downloadMediaToTempWithClient(
		context.Background(),
		mediaStrict,
		clientFor(payload),
		"http://p1.music.126.net/cover",
		int64(len(payload)-1),
		"ncmctl-yunbei-download-*",
	)
	cleanup()
	assert.Empty(t, path)
	require.ErrorContains(t, err, "exceeds")

	path, cleanup, err = downloadMediaToTempWithClient(
		context.Background(),
		mediaStrict,
		clientFor([]byte("not an image")),
		"http://p1.music.126.net/cover",
		1024,
		"ncmctl-yunbei-download-*",
	)
	cleanup()
	assert.Empty(t, path)
	require.ErrorContains(t, err, "supported image types")
}

func TestFetchMediaAudioSampleUsesRangeAndRejectsEmptyBody(t *testing.T) {
	t.Parallel()

	clientFor := func(body []byte) *http.Client {
		return &http.Client{Transport: mediaRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
			assert.Equal(t, "https://m1.music.126.net/audio", request.URL.String())
			assert.Equal(t, "bytes=0-3", request.Header.Get("Range"))
			return &http.Response{
				StatusCode: http.StatusPartialContent,
				Status:     "206 Partial Content",
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Request:    request,
			}, nil
		})}
	}

	require.NoError(t, fetchMediaAudioSampleWithClient(
		context.Background(),
		mediaStrict,
		clientFor([]byte("sample")),
		"http://m1.music.126.net/audio",
		4,
	))
	require.ErrorContains(t, fetchMediaAudioSampleWithClient(
		context.Background(),
		mediaStrict,
		clientFor(nil),
		"http://m1.music.126.net/audio",
		4,
	), "empty")
	require.ErrorContains(t, fetchMediaAudioSampleWithClient(
		context.Background(),
		mediaStrict,
		nil,
		"https://m1.music.126.net/audio",
		4,
	), "client is missing")
}

func TestMediaHelpersCloseResponseBodies(t *testing.T) {
	t.Parallel()

	clientFor := func(body io.ReadCloser) *http.Client {
		return &http.Client{Transport: mediaRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       body,
				Request:    request,
			}, nil
		})}
	}

	imageBody := &mediaTrackingBody{Reader: bytes.NewReader([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})}
	path, cleanup, err := downloadMediaToTempWithClient(
		context.Background(),
		mediaStrict,
		clientFor(imageBody),
		"https://p1.music.126.net/cover",
		1024,
		"ncmctl-yunbei-close-*",
	)
	require.NoError(t, err)
	cleanup()
	assert.NotEmpty(t, path)
	assert.True(t, imageBody.closed)

	audioBody := &mediaTrackingBody{Reader: bytes.NewReader([]byte("audio"))}
	require.NoError(t, fetchMediaAudioSampleWithClient(
		context.Background(),
		mediaStrict,
		clientFor(audioBody),
		"https://m1.music.126.net/audio",
		5,
	))
	assert.True(t, audioBody.closed)
}

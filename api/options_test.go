// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionsBoundarySetters(t *testing.T) {
	var opts Options

	require.NotPanics(t, func() {
		opts.SetHeader("Cookie", "MUSIC_U=token")
	})
	assert.Empty(t, opts.Headers.Get("Cookie"))

	cookie := &http.Cookie{Name: "token", Value: "value"}
	opts.SetCookies(nil, cookie)
	cookie.Value = "changed"

	require.Len(t, opts.cookies, 1)
	assert.Equal(t, "value", opts.GetCookie("token").Value)
	assert.Nil(t, opts.GetCookie("missing"))
}

func TestNewOptionsInitializesPublicHeaders(t *testing.T) {
	opts := NewOptions()

	require.NotPanics(t, func() {
		opts.Headers.Set("X-Test", "value")
	})
	assert.Equal(t, "value", opts.Headers.Get("X-Test"))
}

func TestRequestRejectsUnknownCryptoModeWithoutPanic(t *testing.T) {
	client := &Client{defHeader: defaultHeaders}

	for _, mode := range []CryptoMode{"", "unknown"} {
		t.Run(string(mode), func(t *testing.T) {
			var response map[string]any

			var err error

			require.NotPanics(t, func() {
				_, err = client.Request(
					context.Background(),
					"https://example.com/api/test",
					map[string]string{"id": "1"},
					&response,
					&Options{CryptoMode: mode},
				)
			})
			require.EqualError(t, err, string(mode)+" crypto mode unknown")
		})
	}
}

func TestOptionsSetCookiesNormalizesRequestLayer(t *testing.T) {
	var opts Options

	opts.SetCookies(
		&http.Cookie{Name: "first", Value: "first"},
		&http.Cookie{Name: "token", Value: "old"},
		&http.Cookie{Name: "Token", Value: "case-sensitive"},
		&http.Cookie{Name: "token", Value: "new"},
		&http.Cookie{Name: "empty", Value: ""},
	)
	opts.SetCookies(&http.Cookie{Name: "first", Value: "moved"})

	assert.Equal(t, []string{"Token", "token", "empty", "first"}, optionCookieNames(opts.cookies))
	assert.Equal(t, "new", opts.GetCookie("token").Value)
	assert.Equal(t, "case-sensitive", opts.GetCookie("Token").Value)
	assert.Empty(t, opts.GetCookie("empty").Value)
	assert.Equal(t, "moved", opts.GetCookie("first").Value)
}

func TestOptionsCookieCopiesAreIsolated(t *testing.T) {
	input := &http.Cookie{Name: "token", Value: "original", Unparsed: []string{"input-original"}}
	opts := NewOptions()
	opts.SetHeader("X-Test", "original")
	opts.SetCookies(input)

	input.Value = "input-mutated"
	input.Unparsed[0] = "input-mutated"
	got := opts.GetCookie("token")
	got.Value = "getter-mutated"
	got.Unparsed[0] = "getter-mutated"

	clone := cloneOptions(opts)
	clone.SetHeader("X-Test", "clone")
	clone.SetCookies(&http.Cookie{Name: "token", Value: "clone"})

	assert.Equal(t, "original", opts.GetCookie("token").Value)
	assert.Equal(t, []string{"input-original"}, opts.GetCookie("token").Unparsed)
	assert.Equal(t, "original", opts.Headers.Get("X-Test"))
	assert.Equal(t, "clone", clone.GetCookie("token").Value)
	assert.Equal(t, "clone", clone.Headers.Get("X-Test"))
}

func TestOptionsInvalidCookieBatchIsAtomicAndSticky(t *testing.T) {
	client := newCookieTransportTestClient(t)
	calls := 0

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))

	opts := NewOptions()
	opts.SetCookies(&http.Cookie{Name: "existing", Value: "kept"})
	opts.SetCookies(
		&http.Cookie{Name: "batch", Value: "must-not-stick"},
		&http.Cookie{Name: "bad name", Value: "sensitive-value"},
	)
	opts.SetCookies(&http.Cookie{Name: "later", Value: "ignored"})

	assert.Equal(t, "kept", opts.GetCookie("existing").Value)
	assert.Nil(t, opts.GetCookie("batch"))
	assert.Nil(t, opts.GetCookie("later"))

	var response map[string]any

	_, err := client.Request(
		context.Background(),
		"https://music.163.com/weapi/test",
		map[string]string{"id": "1"},
		&response,
		opts,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `option Cookie "bad name" at index 1 is invalid`)
	assert.NotContains(t, err.Error(), "sensitive-value")
	assert.Zero(t, calls)
}

func optionCookieNames(cookies []*http.Cookie) []string {
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}
	return names
}

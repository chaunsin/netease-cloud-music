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
	assert.Equal(t, "MUSIC_U=token", opts.Headers.Get("Cookie"))

	cookie := &http.Cookie{Name: "token", Value: "value"}
	opts.SetCookies(nil, cookie)
	opts.Cookies = append(opts.Cookies, nil)
	assert.Same(t, cookie, opts.GetCookie("token"))
	assert.Nil(t, opts.GetCookie("missing"))
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

func TestOptionsGetCookieUsesLastValue(t *testing.T) {
	first := &http.Cookie{Name: "token", Value: "first"}
	last := &http.Cookie{Name: "token", Value: "last"}
	opts := &Options{Cookies: []*http.Cookie{first, nil, last}}

	assert.Same(t, last, opts.GetCookie("token"))
	assert.Nil(t, opts.GetCookie("missing"))
}

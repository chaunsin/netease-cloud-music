// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

const defaultWeapiUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/3.0.12.2443"

func TestDefaultWeapiUserAgent(t *testing.T) {
	assert.Equal(t, defaultWeapiUserAgent, defaultHeaders.WEAPI.GetHeader("User-Agent"))
}

func TestHeadersValidateRejectsEmptyXeapiIdentity(t *testing.T) {
	for _, name := range []string{"appver", "buildver", "mobilename", "os", "osver"} {
		t.Run(name, func(t *testing.T) {
			headers := defaultHeaders.clone()
			headers.XEAPI.Cookie[name] = ""

			require.EqualError(t, headers.Validate(), fmt.Sprintf("xeapi: %s cookie value required. ", name))
		})
	}
}

func TestHeadersValidateRejectsInvalidCookieInEveryMode(t *testing.T) {
	tests := []struct {
		name    string
		cookies func(*Headers) map[string]string
	}{
		{name: "api", cookies: func(headers *Headers) map[string]string { return headers.API.Cookie }},
		{name: "eapi", cookies: func(headers *Headers) map[string]string { return headers.EAPI.Cookie }},
		{name: "weapi", cookies: func(headers *Headers) map[string]string { return headers.WEAPI.Cookie }},
		{name: "xeapi", cookies: func(headers *Headers) map[string]string { return headers.XEAPI.Cookie }},
		{name: "linuxapi", cookies: func(headers *Headers) map[string]string { return headers.LinuxAPI.Cookie }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := defaultHeaders.clone()
			secret := "configured-secret;private"
			tt.cookies(&headers)["unsafe"] = secret

			err := headers.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.name+`: Cookie "unsafe" is invalid`)
			assert.NotContains(t, err.Error(), secret)
		})
	}
}

func TestHeadersCloneAndTransactionalLoad(t *testing.T) {
	original := defaultHeaders.clone()
	configured := original.clone()

	configPath := filepath.Join(t.TempDir(), "header.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`weapi:
  header:
    User-Agent:
      - configured-agent
`), 0o600))
	require.NoError(t, configured.LoadConfig(configPath))

	assert.Equal(t, "configured-agent", configured.WEAPI.GetHeader("User-Agent"))
	assert.Equal(t, original.API, configured.API)
	assert.Equal(t, defaultWeapiUserAgent, original.WEAPI.GetHeader("User-Agent"))

	configured.WEAPI.Cookie["channel"] = "changed"
	configured.WEAPI.Header["User-Agent"][0] = "mutated"
	assert.Equal(t, "appstore", original.WEAPI.Cookie["channel"])
	assert.Equal(t, defaultWeapiUserAgent, original.WEAPI.Header.Get("User-Agent"))

	beforeInvalidLoad := configured.clone()

	require.NoError(t, os.WriteFile(configPath, []byte(`weapi:
  cookie:
    channel: ""
`), 0o600))
	require.Error(t, configured.LoadConfig(configPath))
	assert.Equal(t, beforeInvalidLoad, configured)
}

func TestNewClientHeaderIsolation(t *testing.T) {
	type result struct {
		client *Client
		logger *log.Logger
		wantUA string
		err    error
	}

	original := defaultHeaders.clone()
	start := make(chan struct{})
	results := make(chan result, 2)

	for i, userAgent := range []string{"client-one", "client-two"} {
		home := t.TempDir()
		stateDir := filepath.Join(home, ".ncmctl")
		require.NoError(t, os.MkdirAll(stateDir, 0o700))
		require.NoError(t, os.WriteFile(
			filepath.Join(stateDir, "header.yaml"),
			fmt.Appendf(nil, "weapi:\n  header:\n    User-Agent:\n      - %s\n", userAgent),
			0o600,
		))

		logger := log.New(&log.Config{Level: "error"})
		cfg := &Config{
			HomeDir: home,
			Cookie: cookie.Config{
				Filepath: filepath.Join(home, fmt.Sprintf("cookie-%d.json", i)),
				Interval: 0,
			},
		}

		go func(wantUA string) {
			<-start

			client, err := NewClient(cfg, logger)
			results <- result{client: client, logger: logger, wantUA: wantUA, err: err}
		}(userAgent)
	}

	close(start)

	for range 2 {
		got := <-results
		require.NoError(t, got.err)
		assert.Equal(t, got.wantUA, got.client.defHeader.WEAPI.GetHeader("User-Agent"))
		require.NoError(t, got.client.Close(context.Background()))
		require.NoError(t, got.logger.Close())
	}

	assert.Equal(t, original, defaultHeaders.clone())
}

func TestNewClientValidatesHeadersBeforeStartingCookiePersistence(t *testing.T) {
	home := t.TempDir()
	stateDir := filepath.Join(home, ".ncmctl")
	require.NoError(t, os.MkdirAll(stateDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "header.yaml"), []byte(`weapi:
  cookie:
    channel: ""
`), 0o600))

	logger := log.New(&log.Config{Level: "error"})

	t.Cleanup(func() { require.NoError(t, logger.Close()) })

	cookiePath := filepath.Join(home, "cookie-runtime", "cookie.json")
	client, err := NewClient(&Config{
		HomeDir: home,
		Cookie: cookie.Config{
			Filepath: cookiePath,
			Interval: time.Second,
		},
	}, logger)

	require.Error(t, err)
	assert.Nil(t, client)
	assert.NoDirExists(t, filepath.Dir(cookiePath))
}

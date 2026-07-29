// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	ncmcrypto "github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

func TestRewriteXeapiURL(t *testing.T) {
	tests := []struct {
		name         string
		rawURL       string
		wantEnvelope string
		wantRequest  string
		wantErr      bool
	}{
		{
			name:         "api path",
			rawURL:       "https://interface.music.163.com/api/song/detail?id=1#frag",
			wantEnvelope: "https://interface.music.163.com/api/song/detail?id=1#frag",
			wantRequest:  "https://interface.music.163.com/xeapi/song/detail",
		},
		{
			name:         "xeapi path",
			rawURL:       "https://interface.music.163.com/xeapi/song/detail?id=1",
			wantEnvelope: "https://interface.music.163.com/xeapi/song/detail?id=1",
			wantRequest:  "https://interface.music.163.com/xeapi/song/detail",
		},
		{
			name:         "eapi path",
			rawURL:       "https://interface.music.163.com/eapi/song/detail?id=1#frag",
			wantEnvelope: "https://interface.music.163.com/api/song/detail?id=1#frag",
			wantRequest:  "https://interface.music.163.com/xeapi/song/detail",
		},
		{
			name:    "unsupported path",
			rawURL:  "https://interface.music.163.com/weapi/song/detail",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelopeURL, requestURL, err := rewriteXeapiURL(tt.rawURL)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantEnvelope, envelopeURL)
			assert.Equal(t, tt.wantRequest, requestURL)
		})
	}
}

func TestRawTimestampString(t *testing.T) {
	got, err := rawTimestampString(json.RawMessage(`1779955010033`))
	require.NoError(t, err)
	assert.Equal(t, "1779955010033", got)

	got, err = rawTimestampString(json.RawMessage(`"1779955010033"`))
	require.NoError(t, err)
	assert.Equal(t, "1779955010033", got)

	_, err = rawTimestampString(json.RawMessage(`{}`))
	assert.Error(t, err)
}

func TestXeapiKeyNeedsRefresh(t *testing.T) {
	valid := ncmcrypto.XeapiPublicKeyState{
		PublicKey:      "public-key",
		Version:        "v1",
		NextUpdateTime: time.Now().Add(time.Hour).UnixMilli(),
		SK:             "server-key",
		DeviceID:       "device-a",
	}

	tests := []struct {
		name     string
		state    ncmcrypto.XeapiPublicKeyState
		deviceID string
		want     bool
	}{
		{name: "empty state", deviceID: "device-a", want: true},
		{name: "matching device", state: valid, deviceID: "device-a"},
		{name: "different device", state: valid, deviceID: "device-b", want: true},
		{name: "legacy cache without device", state: func() ncmcrypto.XeapiPublicKeyState {
			state := valid
			state.DeviceID = ""
			return state
		}(), deviceID: "device-b"},
		{name: "expired", state: func() ncmcrypto.XeapiPublicKeyState {
			state := valid
			state.NextUpdateTime = time.Now().Add(-time.Hour).UnixMilli()
			return state
		}(), deviceID: "device-a", want: true},
		{name: "no expiration", state: func() ncmcrypto.XeapiPublicKeyState {
			state := valid
			state.NextUpdateTime = 0
			return state
		}(), deviceID: "device-a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, xeapiKeyNeedsRefresh(tt.state, tt.deviceID))
		})
	}
}

func TestXeapiStateEntersRefreshPath(t *testing.T) {
	valid := ncmcrypto.XeapiPublicKeyState{
		PublicKey:      "public-key",
		Version:        "v1",
		NextUpdateTime: time.Now().Add(time.Hour).UnixMilli(),
		SK:             "server-key",
		DeviceID:       "device-a",
	}
	refreshErr := errors.New("refresh attempted")

	tests := []struct {
		name        string
		state       ncmcrypto.XeapiPublicKeyState
		deviceID    string
		wantVersion string
	}{
		{name: "cold start", deviceID: "device-a"},
		{name: "device changed", state: valid, deviceID: "device-b", wantVersion: "v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := resty.New()
			client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				calls++

				require.NoError(t, request.ParseForm())
				assert.Equal(t, "/api/gorilla/anti/crawler/security/key/get", request.URL.Path)
				assert.Equal(t, tt.deviceID, request.Form.Get("deviceId"))
				assert.Equal(t, tt.wantVersion, request.Form.Get("currentKeyVersion"))
				return nil, refreshErr
			}))

			manager := newXeapi(client, "")
			manager.PublicKeyState = tt.state

			_, err := manager.xeapiState(context.Background(), &ncmcrypto.XeapiEncryptRequest{
				DeviceID: tt.deviceID,
			})
			require.ErrorIs(t, err, refreshErr)
			assert.Equal(t, 1, calls)
		})
	}
}

func TestXeapiLoadConfig(t *testing.T) {
	validKey := ncmcrypto.XeapiPublicKeyState{
		PublicKey:      "public-key",
		Version:        "v1",
		NextUpdateTime: time.Now().Add(time.Hour).UnixMilli(),
		SK:             "server-key",
		DeviceID:       "device-a",
	}

	tests := []struct {
		name    string
		state   xeapiStateResult
		wantErr string
	}{
		{name: "without session", state: xeapiStateResult{PublicKeyState: validKey}},
		{name: "with session", state: xeapiStateResult{
			PublicKeyState: validKey,
			Session:        ncmcrypto.XeapiSession{ID: "session-id", Key: "0123456789abcdef"},
		}},
		{name: "expired key remains available for refresh", state: xeapiStateResult{
			PublicKeyState: func() ncmcrypto.XeapiPublicKeyState {
				state := validKey
				state.NextUpdateTime = time.Now().Add(-time.Hour).UnixMilli()
				return state
			}(),
		}},
		{name: "session id without key", state: xeapiStateResult{
			PublicKeyState: validKey,
			Session:        ncmcrypto.XeapiSession{ID: "session-id"},
		}, wantErr: "xeapi session is incomplete"},
		{name: "session key without id", state: xeapiStateResult{
			PublicKeyState: validKey,
			Session:        ncmcrypto.XeapiSession{Key: "0123456789abcdef"},
		}, wantErr: "xeapi session is incomplete"},
		{name: "incomplete public key", state: xeapiStateResult{
			PublicKeyState: ncmcrypto.XeapiPublicKeyState{PublicKey: "public-key", SK: "server-key"},
		}, wantErr: "public key state is incomplete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "xeapi.json")
			data, err := json.Marshal(tt.state)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, data, 0o600))

			manager := newXeapi(resty.New(), path)

			err = manager.LoadConfig()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.state, manager.xeapiStateResult)
		})
	}
}

func TestXeapiSyncSkipsIncompleteState(t *testing.T) {
	t.Run("does not create a file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "xeapi.yaml")
		manager := newXeapi(resty.New(), path)

		require.NoError(t, manager.Sync())

		_, err := os.Stat(path)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("does not overwrite a file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "xeapi.yaml")
		original := []byte("preserve existing state")
		require.NoError(t, os.WriteFile(path, original, 0o600))

		manager := newXeapi(resty.New(), path)
		require.NoError(t, manager.Sync())

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, original, got)
	})
}

func TestGetDeviceIdReadsXeapiDomain(t *testing.T) {
	jar, err := cookie.NewCookie(
		cookie.WithSyncInterval(0),
		cookie.WithFilePath(filepath.Join(t.TempDir(), "cookie.json")),
	)
	require.NoError(t, err)

	client := &Client{cookie: jar}
	uri, err := neturl.Parse("https://interface.music.163.com")
	require.NoError(t, err)
	client.SetCookies(uri, []*http.Cookie{{Name: "deviceId", Value: "xeapi-device-id"}})

	assert.Equal(t, "xeapi-device-id", client.GetDeviceId())
}

func TestUpdateXeapiSessionReadsIssue174Header(t *testing.T) {
	client := xeapi{}
	response := &resty.Response{
		RawResponse: &http.Response{Header: http.Header{}},
	}
	response.RawResponse.Header.Set("X-Encr-Ssid", "session-id")
	response.RawResponse.Header.Set("X-Encr-Sskey", "0123456789abcdef0123456789abcdef")

	client.updateSession(response)

	assert.Equal(t, "session-id", client.Session.ID)
	assert.Equal(t, "0123456789abcdef0123456789abcdef", client.Session.Key)
}

func TestNewClientUsesRuntimeHomeForStateFiles(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	stateDir := filepath.Join(home, ".ncmctl")
	require.NoError(t, os.MkdirAll(stateDir, 0o700))

	headers := *defaultHeaders
	headers.API.Header = defaultHeaders.API.Header.Clone()
	headers.API.Header.Set("User-Agent", "runtime-home-header")
	headerConfig, err := json.Marshal(headers)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "header.yaml"), headerConfig, 0o600))

	logger := log.New(&log.Config{Level: "error"})

	t.Cleanup(func() {
		require.NoError(t, logger.Close())
	})

	client, err := NewClient(&Config{
		HomeDir: home,
		Cookie: cookie.Config{
			Filepath: filepath.Join(stateDir, "cookie.json"),
			Interval: 0,
		},
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(context.Background()))
	})

	assert.Equal(t, filepath.Join(stateDir, "xeapi.yaml"), client.xeapi.storePath)
	assert.Equal(t, filepath.Join(stateDir, "anonymous_token"), client.anonymous.storePath)
	assert.Equal(t, "runtime-home-header", client.defHeader.API.Header.Get("User-Agent"))
}

func TestXeapiRequestSetsEncryptedAppHeaders(t *testing.T) {
	oldLogger := log.Default
	log.Default = log.New(nil)

	t.Cleanup(func() {
		log.Default = oldLogger
	})

	var (
		capturedHeader http.Header
		capturedHost   string
		homeDir        = t.TempDir()
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Clone()
		capturedHost = r.Host
		assert.Equal(t, "/xeapi/song/detail", r.URL.Path)
		assert.NotEmpty(t, r.FormValue("B"))
		assert.NotEmpty(t, r.FormValue("S"))
		assert.NotEmpty(t, r.FormValue("R"))
		_, _ = w.Write(encryptLegacyEapiResponse(t, []byte(`{"code":200}`)))
	}))
	defer server.Close()

	client, err := NewClient(&Config{
		Timeout: time.Second,
		HomeDir: homeDir,
		Cookie: cookie.Config{
			Filepath: filepath.Join(homeDir, "cookie.json"),
			Interval: 0,
		},
	}, log.Default)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, client.Close(context.Background()))
	})

	client.xeapi.PublicKeyState = ncmcrypto.XeapiPublicKeyState{
		PublicKey: "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
		Version:   "1000000000000",
		SK:        "8PZfbIFA1779944463972",
	}

	opts := NewOptions().SetCryptoModeXEAPI()

	var reply map[string]any

	_, err = client.Request(context.Background(), server.URL+"/eapi/song/detail?id=1", map[string]string{"id": "1"}, &reply, opts)
	require.NoError(t, err)

	assert.Equal(t, "ENCRYPTED", capturedHeader.Get("X-Client-Enc-State"))
	assert.Equal(t, defaultXeapiUserAgent, capturedHeader.Get("User-Agent"))
	assert.Equal(t, "application/x-www-form-urlencoded", capturedHeader.Get("Content-Type"))

	serverURL, err := neturl.Parse(server.URL)
	require.NoError(t, err)
	assert.Equal(t, serverURL.Host, capturedHost)
}

func encryptLegacyEapiResponse(t *testing.T, plaintext []byte) []byte {
	t.Helper()

	block, err := aes.NewCipher([]byte("e82ckenh8dichen8"))
	require.NoError(t, err)

	padding := block.BlockSize() - len(plaintext)%block.BlockSize()
	padded := append(append([]byte(nil), plaintext...), bytes.Repeat([]byte{byte(padding)}, padding)...)

	ciphertext := make([]byte, len(padded))
	for i := 0; i < len(padded); i += block.BlockSize() {
		block.Encrypt(ciphertext[i:i+block.BlockSize()], padded[i:i+block.BlockSize()])
	}
	return ciphertext
}

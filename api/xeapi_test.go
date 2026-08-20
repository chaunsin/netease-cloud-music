// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

				assert.Equal(t, "/eapi/gorilla/anti/crawler/security/key/get", request.URL.Path)
				payload := decodeEAPIRequestPayload(t, request)
				assert.Equal(t, tt.deviceID, payload["deviceId"])
				assert.Equal(t, tt.wantVersion, payload["currentKeyVersion"])
				assert.Equal(t, true, payload["e_r"])
				assert.Equal(t, "{}", payload["header"])
				return nil, refreshErr
			}))

			manager := newXeapi(client, "")
			manager.PublicKeyState = tt.state

			_, err := manager.xeapiState(context.Background(), &ncmcrypto.XeapiEncryptRequest{
				DeviceID: tt.deviceID,
				T1:       "token-t1",
				T2:       "token-t2",
				UID:      "user-id",
			})
			require.ErrorIs(t, err, refreshErr)
			assert.Equal(t, 1, calls)
		})
	}
}

func TestXeapiRefreshPublicKeyDecryptsEAPIResponseAndClearsSession(t *testing.T) {
	refreshed := ncmcrypto.XeapiPublicKeyState{
		PublicKey:      "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
		Version:        "v2",
		NextUpdateTime: 4102444800000,
		SK:             "server-key-v2",
	}

	client := resty.New()
	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "/eapi/gorilla/anti/crawler/security/key/get", request.URL.Path)
		assert.Equal(t, "true", request.Header.Get("X-Aeapi"))

		payload := decodeEAPIRequestPayload(t, request)
		assert.Equal(t, "/api/gorilla/anti/crawler/security/key/get", payload["_route"])
		assert.Equal(t, "v1", payload["currentKeyVersion"])
		assert.Equal(t, "device-b", payload["deviceId"])
		assert.NotContains(t, payload, "checkToken")

		const responseTimestamp = int64(1779955023124)

		nonce, ok := payload["nonce"].(string)
		require.True(t, ok)

		reply, err := json.Marshal(map[string]any{
			"code": http.StatusOK,
			"data": map[string]any{
				"encryptedData": encryptXeapiPublicKeyState(t, refreshed),
				"signature":     ncmcrypto.XeapiSign("1779955023124", nonce),
				"timestamp":     responseTimestamp,
			},
			"message": "",
		})
		require.NoError(t, err)

		body := encryptLegacyEapiResponse(t, gzipPayload(t, reply))
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    request,
		}, nil
	}))

	manager := newXeapi(client, "")
	manager.PublicKeyState = ncmcrypto.XeapiPublicKeyState{
		PublicKey:      refreshed.PublicKey,
		Version:        "v1",
		NextUpdateTime: 4102444800000,
		SK:             "server-key-v1",
		DeviceID:       "device-a",
	}
	manager.Session = ncmcrypto.XeapiSession{ID: "old-session", Key: "0123456789abcdef"}

	state, err := manager.xeapiState(
		context.Background(),
		&ncmcrypto.XeapiEncryptRequest{DeviceID: "device-b", OS: "android", AppVersion: "9.5.15"},
	)
	require.NoError(t, err)

	refreshed.DeviceID = "device-b"
	assert.Equal(t, refreshed, state.PublicKeyState)
	assert.Empty(t, state.Session)
	assert.Empty(t, manager.Session)
	assert.Equal(t, uint64(1), manager.keyRevision)
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
		{name: "invalid session key length", state: xeapiStateResult{
			PublicKeyState: validKey,
			Session:        ncmcrypto.XeapiSession{ID: "session-id", Key: "too-short"},
		}, wantErr: "xeapi session key length is invalid: got 9 bytes"},
		{name: "missing public key", state: xeapiStateResult{
			PublicKeyState: ncmcrypto.XeapiPublicKeyState{Version: "v1", SK: "server-key"},
		}, wantErr: "public key state is incomplete"},
		{name: "missing version", state: xeapiStateResult{
			PublicKeyState: ncmcrypto.XeapiPublicKeyState{PublicKey: "public-key", SK: "server-key"},
		}, wantErr: "public key state is incomplete"},
		{name: "missing server key", state: xeapiStateResult{
			PublicKeyState: ncmcrypto.XeapiPublicKeyState{PublicKey: "public-key", Version: "v1"},
		}, wantErr: "public key state is incomplete"},
		{name: "blank public key", state: xeapiStateResult{
			PublicKeyState: ncmcrypto.XeapiPublicKeyState{PublicKey: " \t", Version: "v1", SK: "server-key"},
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

func TestXeapiLoadConfigYAML(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "canonical field names", content: `publicKeyState:
    publicKey: public-key
    version: v1
    nextUpdateTime: 4102444800000
    sk: server-key
    deviceId: device-a
session:
    id: session-id
    key: 0123456789abcdef
`},
		{name: "legacy lowercase field names", content: `publicKeyState:
    publickey: public-key
    version: v1
    nextupdatetime: 0
    sk: server-key
    deviceid: ""
session:
    id: ""
    key: ""
`, wantErr: "public key state is incomplete"},
		{name: "empty file", wantErr: "public key state is incomplete"},
		{name: "malformed yaml", content: "publicKeyState: [", wantErr: "unmarshal err:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "xeapi.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))

			manager := newXeapi(resty.New(), path)

			err := manager.LoadConfig()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testXeapiState(), manager.xeapiStateResult)
		})
	}
}

func TestXeapiSyncRoundTripYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "xeapi.yaml")
	want := testXeapiState()

	writer := newXeapi(resty.New(), path)
	writer.xeapiStateResult = want
	require.NoError(t, writer.Sync())

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	output := string(data)
	for _, key := range []string{"publicKeyState:", "publicKey:", "nextUpdateTime:", "deviceId:"} {
		assert.Contains(t, output, key)
	}

	for _, legacyKey := range []string{"publickey:", "nextupdatetime:", "deviceid:"} {
		assert.NotContains(t, output, legacyKey)
	}

	if runtime.GOOS != "windows" {
		fileInfo, statErr := os.Stat(path)
		require.NoError(t, statErr)
		assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())

		dirInfo, statErr := os.Stat(filepath.Dir(path))
		require.NoError(t, statErr)
		assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
	}

	reader := newXeapi(resty.New(), path)
	require.NoError(t, reader.LoadConfig())
	assert.Equal(t, want, reader.xeapiStateResult)
}

func testXeapiState() xeapiStateResult {
	return xeapiStateResult{
		PublicKeyState: ncmcrypto.XeapiPublicKeyState{
			PublicKey:      "public-key",
			Version:        "v1",
			NextUpdateTime: 4102444800000,
			SK:             "server-key",
			DeviceID:       "device-a",
		},
		Session: ncmcrypto.XeapiSession{ID: "session-id", Key: "0123456789abcdef"},
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

	client := &Client{cookieJar: jar}
	uri, err := neturl.Parse("https://interface.music.163.com")
	require.NoError(t, err)
	client.SetCookies(uri, []*http.Cookie{{Name: "deviceId", Value: "xeapi-device-id"}})

	assert.Equal(t, "xeapi-device-id", client.GetDeviceId())
}

func TestUpdateXeapiSessionReadsIssue174Header(t *testing.T) {
	client := xeapi{keyRevision: 1}
	response := xeapiSessionResponse("session-id", "0123456789abcdef0123456789abcdef")

	require.NoError(t, client.updateSession(response, xeapiRequestRevision{keyRevision: 1, requestSequence: 1}))

	assert.Equal(t, "session-id", client.Session.ID)
	assert.Equal(t, "0123456789abcdef0123456789abcdef", client.Session.Key)
}

func TestUpdateXeapiSessionValidatesRevisionOrderingAndKey(t *testing.T) {
	manager := xeapi{keyRevision: 2}

	require.NoError(t, manager.updateSession(
		xeapiSessionResponse("stale-key-revision", "0123456789abcdef"),
		xeapiRequestRevision{keyRevision: 1, requestSequence: 100},
	))
	assert.Empty(t, manager.Session)

	err := manager.updateSession(
		xeapiSessionResponse("session-id", "too-short"),
		xeapiRequestRevision{keyRevision: 2, requestSequence: 1},
	)
	require.ErrorIs(t, err, ncmcrypto.ErrSessionKeyLength)
	assert.Empty(t, manager.Session)

	err = manager.updateSession(
		xeapiSessionResponse("session-id", ""),
		xeapiRequestRevision{keyRevision: 2, requestSequence: 1},
	)
	require.ErrorIs(t, err, ncmcrypto.ErrSessionIncomplete)
	assert.Empty(t, manager.Session)

	require.NoError(t, manager.updateSession(
		xeapiSessionResponse("newer-session", "0123456789abcdef"),
		xeapiRequestRevision{keyRevision: 2, requestSequence: 2},
	))
	require.NoError(t, manager.updateSession(
		xeapiSessionResponse("older-session", "abcdef0123456789"),
		xeapiRequestRevision{keyRevision: 2, requestSequence: 1},
	))
	assert.Equal(t, ncmcrypto.XeapiSession{ID: "newer-session", Key: "0123456789abcdef"}, manager.Session)
}

func TestUpdateXeapiSessionConcurrentResponsesKeepNewest(t *testing.T) {
	const requestCount = 64

	manager := xeapi{keyRevision: 1}
	start := make(chan struct{})
	results := make(chan error, requestCount)

	for sequence := uint64(1); sequence <= requestCount; sequence++ {
		go func(sequence uint64) {
			<-start

			results <- manager.updateSession(
				xeapiSessionResponse(fmt.Sprintf("session-%d", sequence), "0123456789abcdef"),
				xeapiRequestRevision{keyRevision: 1, requestSequence: sequence},
			)
		}(sequence)
	}

	close(start)

	for range requestCount {
		require.NoError(t, <-results)
	}

	assert.Equal(t, ncmcrypto.XeapiSession{ID: "session-64", Key: "0123456789abcdef"}, manager.Session)
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

func TestXeapiRequestUsesConfiguredMethodAndResolvedIdentity(t *testing.T) {
	var (
		capturedHeader  http.Header
		capturedCookies map[string]string
		capturedHost    string
		capturedMethods []string
		requestCount    int
	)

	client := newOfflineXeapiClient(t)
	client.SetTransport(testRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		requestCount++

		capturedMethods = append(capturedMethods, r.Method)
		capturedHeader = r.Header.Clone()
		capturedHost = r.URL.Hostname()

		capturedCookies = make(map[string]string)
		for _, ck := range r.Cookies() {
			capturedCookies[ck.Name] = ck.Value
		}

		assert.Equal(t, "/xeapi/song/detail", r.URL.Path)

		if r.Method == http.MethodPost {
			assert.NotEmpty(t, r.FormValue("B"))
			assert.NotEmpty(t, r.FormValue("S"))
			assert.NotEmpty(t, r.FormValue("R"))
		} else {
			assert.Empty(t, r.FormValue("B"))
			assert.Empty(t, r.FormValue("S"))
			assert.Empty(t, r.FormValue("R"))
		}

		body := encryptLegacyEapiResponse(t, []byte(`{"code":200}`))
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewReader(body)),
			ContentLength: int64(len(body)),
			Request:       r,
		}, nil
	}))

	for _, method := range []string{http.MethodPost, http.MethodGet} {
		opts := NewOptions().SetXEAPI().SetMethod(method)
		opts.SetCookies(
			&http.Cookie{Name: "deviceId", Value: "request-device-id"},
			&http.Cookie{Name: "MUSIC_U", Value: "request-music-u"},
		)

		var reply map[string]any

		_, err := client.Request(
			context.Background(),
			"https://interface.music.163.com/eapi/song/detail?id=1",
			map[string]string{"id": "1"},
			&reply,
			opts,
		)
		require.NoError(t, err)
	}

	assert.Equal(t, 2, requestCount)
	assert.Equal(t, []string{http.MethodPost, http.MethodGet}, capturedMethods)
	assert.Equal(t, "ENCRYPTED", capturedHeader.Get("X-Client-Enc-State"))
	assert.Equal(t, defaultXeapiUserAgent, capturedHeader.Get("User-Agent"))
	assert.Equal(t, "application/x-www-form-urlencoded", capturedHeader.Get("Content-Type"))
	assert.Equal(t, "request-device-id", capturedHeader.Get("X-Deviceid"))
	assert.Equal(t, "request-device-id", capturedHeader.Get("X-Sdeviceid"))
	assert.Equal(t, "request-music-u", capturedHeader.Get("X-Music-U"))
	assert.Equal(t, "request-device-id", capturedCookies["deviceId"])
	assert.Equal(t, "request-device-id", capturedCookies["sDeviceId"])
	assert.Equal(t, "request-music-u", capturedCookies["MUSIC_U"])

	assert.Equal(t, "interface.music.163.com", capturedHost)
}

func TestXeapiOutsideProtocolCookieDomainKeepsCookieIdentityOutOfBusinessPayload(t *testing.T) {
	const sessionKey = "0123456789abcdef"

	tests := []struct {
		name         string
		setup        func(*Client, *Options)
		wantCookies  map[string]string
		wantHeaders  map[string]string
		callerDevice bool
	}{
		{
			name:        "no identity does not use protocol defaults",
			setup:       func(*Client, *Options) {},
			wantCookies: map[string]string{},
			wantHeaders: map[string]string{},
		},
		{
			name: "URL scoped Jar identity remains available",
			setup: func(client *Client, _ *Options) {
				client.SetCookies(mustParseURL(t, "https://example.test/xeapi/song/detail"), []*http.Cookie{
					{Name: "appver", Value: "jar-appver"},
					{Name: "deviceId", Value: "jar-device"},
					{Name: "os", Value: "jar-os"},
				})
			},
			wantCookies: map[string]string{
				"appver":   "jar-appver",
				"deviceId": "jar-device",
				"os":       "jar-os",
			},
			wantHeaders: map[string]string{
				"X-Appver":    "jar-appver",
				"X-DeviceId":  "jar-device",
				"X-SDeviceId": "jar-device",
				"X-Os":        "jar-os",
			},
			callerDevice: true,
		},
		{
			name: "Options cookies override protocol defaults",
			setup: func(_ *Client, opts *Options) {
				opts.SetCookies(
					&http.Cookie{Name: "appver", Value: "option-appver"},
					&http.Cookie{Name: "deviceId", Value: "option-device"},
					&http.Cookie{Name: "os", Value: "option-os"},
				)
			},
			wantCookies: map[string]string{
				"appver":   "option-appver",
				"deviceId": "option-device",
				"os":       "option-os",
			},
			wantHeaders: map[string]string{
				"X-Appver":    "option-appver",
				"X-DeviceId":  "option-device",
				"X-SDeviceId": "option-device",
				"X-Os":        "option-os",
			},
			callerDevice: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newOfflineXeapiClient(t)
			client.xeapi.Session = ncmcrypto.XeapiSession{ID: "known-session", Key: sessionKey}

			opts := NewOptions().SetXEAPI()
			tt.setup(client, opts)

			var encrypted ncmcrypto.XeapiEncryptedRequest

			client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				assert.Equal(t, "example.test", request.URL.Hostname())
				assert.Equal(t, "/xeapi/song/detail", request.URL.Path)
				assert.Equal(t, tt.wantCookies, cookieValues(request.Cookies()))
				assertXEAPIIdentityHeaders(t, request.Header, tt.wantHeaders)

				require.NoError(t, request.ParseForm())
				encrypted = ncmcrypto.XeapiEncryptedRequest{
					B: request.Form.Get("B"),
					S: request.Form.Get("S"),
					R: request.Form.Get("R"),
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(encryptLegacyEapiResponse(t, []byte(`{"code":200}`)))),
					Request:    request,
				}, nil
			}))

			payload := map[string]string{"id": "1"}
			if tt.callerDevice {
				payload["deviceId"] = "caller-device"
			}

			var reply map[string]any

			_, err := client.Request(
				context.Background(),
				"https://example.test/api/song/detail",
				payload,
				&reply,
				opts,
			)
			require.NoError(t, err)

			decrypted, err := ncmcrypto.XeapiDecryptRequest(encrypted, []byte(sessionKey))
			require.NoError(t, err)

			var envelope struct {
				Body string `json:"body"`
			}
			require.NoError(t, json.Unmarshal(decrypted.Plaintext, &envelope))

			body, err := base64.StdEncoding.DecodeString(envelope.Body)
			require.NoError(t, err)
			form, err := neturl.ParseQuery(string(body))
			require.NoError(t, err)

			if !tt.callerDevice {
				assert.False(t, form.Has("deviceId"))
			} else {
				assert.Equal(t, "caller-device", form.Get("deviceId"))
			}
		})
	}
}

func TestXeapiRefreshOutsideProtocolCookieDomainKeepsAndroidOSFallback(t *testing.T) {
	client := newOfflineXeapiClient(t)
	client.xeapi.PublicKeyState = ncmcrypto.XeapiPublicKeyState{}

	refreshed := ncmcrypto.XeapiPublicKeyState{
		PublicKey:      "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
		Version:        "refreshed-key",
		NextUpdateTime: 4102444800000,
		SK:             "refreshed-server-key",
	}

	calls := 0

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		calls++

		switch request.URL.Path {
		case "/eapi/gorilla/anti/crawler/security/key/get":
			assert.Equal(t, "interface.music.163.com", request.URL.Hostname())
			// 协议身份 Cookie 已随 jar 持久化,刷新请求与主请求应携带同一组身份。
			assert.Equal(t,
				cookieValues(client.GetCookies(mustParseURL(t, xeapiPublicKeyURL))),
				cookieValues(request.Cookies()),
			)

			payload := decodeEAPIRequestPayload(t, request)
			assert.Empty(t, payload["appVersion"])
			assert.Empty(t, payload["deviceId"])
			assert.Equal(t, "android", payload["os"])

			nonce, ok := payload["nonce"].(string)
			require.True(t, ok)

			reply, err := json.Marshal(map[string]any{
				"code": http.StatusOK,
				"data": map[string]any{
					"encryptedData": encryptXeapiPublicKeyState(t, refreshed),
					"signature":     ncmcrypto.XeapiSign("1779955023124", nonce),
					"timestamp":     int64(1779955023124),
				},
			})
			require.NoError(t, err)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(encryptLegacyEapiResponse(t, gzipPayload(t, reply)))),
				Request:    request,
			}, nil
		case "/xeapi/song/detail":
			assert.Equal(t, "example.test", request.URL.Hostname())
			assert.Empty(t, request.Cookies())
			assertXEAPIIdentityHeaders(t, request.Header, map[string]string{})

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(encryptLegacyEapiResponse(t, []byte(`{"code":200}`)))),
				Request:    request,
			}, nil
		default:
			return nil, errors.New("unexpected XEAPI request path")
		}
	}))

	var reply map[string]any

	_, err := client.Request(
		context.Background(),
		"https://example.test/api/song/detail",
		map[string]string{"id": "1"},
		&reply,
		NewOptions().SetXEAPI(),
	)
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func assertXEAPIIdentityHeaders(t *testing.T, headers http.Header, want map[string]string) {
	t.Helper()

	for _, name := range []string{
		"X-Appver", "X-Buildver", "X-Channel", "X-DeviceId", "X-SDeviceId",
		"X-Mobilename", "X-Os", "X-Osver", "X-Music-U",
	} {
		wantValue, ok := want[name]
		if !ok {
			assert.Empty(t, headers.Values(name), "%s should not be sent", name)
			continue
		}

		assert.Equal(t, []string{wantValue}, headers.Values(name), name)
	}
}

func TestXeapiResponseSessionUpdate(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		body         func(*testing.T) []byte
		sessionKey   string
		wantAPIError bool
		wantSession  ncmcrypto.XeapiSession
	}{
		{
			name:         "valid session survives decrypt failure",
			statusCode:   http.StatusServiceUnavailable,
			body:         func(*testing.T) []byte { return []byte("not encrypted") },
			sessionKey:   "0123456789abcdef",
			wantAPIError: true,
			wantSession:  ncmcrypto.XeapiSession{ID: "new-session", Key: "0123456789abcdef"},
		},
		{
			name:       "valid session survives json failure",
			statusCode: http.StatusOK,
			body: func(t *testing.T) []byte {
				t.Helper()

				return encryptLegacyEapiResponse(t, []byte("not-json"))
			},
			sessionKey:   "0123456789abcdef",
			wantAPIError: true,
			wantSession:  ncmcrypto.XeapiSession{ID: "new-session", Key: "0123456789abcdef"},
		},
		{
			name:       "invalid session is ignored",
			statusCode: http.StatusOK,
			body: func(t *testing.T) []byte {
				t.Helper()

				return encryptLegacyEapiResponse(t, []byte(`{"code":200}`))
			},
			sessionKey: "too-short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Encr-Ssid", "new-session")
				w.Header().Set("X-Encr-Sskey", tt.sessionKey)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write(tt.body(t))
			}))
			defer server.Close()

			client := newOfflineXeapiClient(t)

			var reply map[string]any

			_, err := client.Request(
				context.Background(),
				server.URL+"/api/song/detail",
				map[string]string{"id": "1"},
				&reply,
				NewOptions().SetXEAPI(),
			)
			if tt.wantAPIError {
				require.Error(t, err)

				var apiErr *APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, tt.statusCode, apiErr.StatusCode)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantSession, client.xeapi.Session)
		})
	}
}

func encryptLegacyEapiResponse(t *testing.T, plaintext []byte) []byte {
	t.Helper()
	return encryptECBPayload(t, []byte("e82ckenh8dichen8"), plaintext)
}

func encryptXeapiPublicKeyState(t *testing.T, state ncmcrypto.XeapiPublicKeyState) string {
	t.Helper()

	key, err := base64.StdEncoding.DecodeString("qx1aQw9rsEo/Aegd3XK9kW1c5ZEkisEocUgG1/j7G4Q=")
	require.NoError(t, err)
	plaintext, err := json.Marshal(state)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(encryptECBPayload(t, key, plaintext))
}

func encryptECBPayload(t *testing.T, key, plaintext []byte) []byte {
	t.Helper()

	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	padding := block.BlockSize() - len(plaintext)%block.BlockSize()
	padded := append(append([]byte(nil), plaintext...), bytes.Repeat([]byte{byte(padding)}, padding)...)

	ciphertext := make([]byte, len(padded))
	for i := 0; i < len(padded); i += block.BlockSize() {
		block.Encrypt(ciphertext[i:i+block.BlockSize()], padded[i:i+block.BlockSize()])
	}
	return ciphertext
}

func gzipPayload(t *testing.T, plaintext []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(plaintext)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

func decodeEAPIRequestPayload(t *testing.T, request *http.Request) map[string]any {
	t.Helper()

	require.NoError(t, request.ParseForm())
	params := request.Form.Get("params")
	require.NotEmpty(t, params)

	plaintext, err := ncmcrypto.EApiDecrypt(params, "hex")
	require.NoError(t, err)

	parts := strings.Split(string(plaintext), "-36cd479b6b5-")
	require.Len(t, parts, 3)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(parts[1]), &payload))
	payload["_route"] = parts[0]
	return payload
}

func xeapiSessionResponse(id, key string) *resty.Response {
	header := make(http.Header)
	header.Set("X-Encr-Ssid", id)
	header.Set("X-Encr-Sskey", key)
	return &resty.Response{RawResponse: &http.Response{Header: header}}
}

func newOfflineXeapiClient(t *testing.T) *Client {
	t.Helper()

	homeDir := t.TempDir()
	logger := log.New(&log.Config{Level: "error"})

	t.Cleanup(func() {
		require.NoError(t, logger.Close())
	})

	client, err := NewClient(&Config{
		Timeout: time.Second,
		HomeDir: homeDir,
		Cookie: cookie.Config{
			Filepath: filepath.Join(homeDir, "cookie.json"),
			Interval: 0,
		},
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(context.Background()))
	})

	client.xeapi.PublicKeyState = ncmcrypto.XeapiPublicKeyState{
		PublicKey:      "3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=",
		Version:        "1000000000000",
		NextUpdateTime: 4102444800000,
		SK:             "8PZfbIFA1779944463972",
	}
	return client
}

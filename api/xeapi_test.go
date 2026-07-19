// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
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
	client := &Client{}
	response := &resty.Response{
		RawResponse: &http.Response{Header: http.Header{}},
	}
	response.RawResponse.Header.Set("X-Encr-Ssid", "session-id")
	response.RawResponse.Header.Set("X-Encr-Sskey", "0123456789abcdef0123456789abcdef")

	client.updateXeapiSession(response)

	assert.Equal(t, "session-id", client.xeapiSession.ID)
	assert.Equal(t, "0123456789abcdef0123456789abcdef", client.xeapiSession.Key)
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
		Cookie: cookie.Config{
			Filepath: filepath.Join(t.TempDir(), "cookie.json"),
			Interval: 0,
		},
	}, log.Default)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, client.Close(context.Background()))
	})

	client.xeapiKey = ncmcrypto.PublicKeyState{
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

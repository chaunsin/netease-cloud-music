// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"bytes"
	"context"
	"crypto/aes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type capturedXeapiRequest struct {
	method string
	path   string
	header http.Header
	form   url.Values
}

func TestDailySongShareAndVipMemberGiftXEAPIWire(t *testing.T) {
	client, transport, cleanup := newOfflineXeapiClient(t)
	t.Cleanup(cleanup)

	endpoint := New(client)
	tests := []struct {
		name        string
		path        string
		call        func(context.Context) error
		wantHeaders map[string]string
	}{
		{
			name: "daily registration",
			path: "/xeapi/note/common/activity/in/registration",
			call: func(ctx context.Context) error {
				_, err := endpoint.DailySongShareRegister(ctx, nil)
				return err
			},
		},
		{
			name: "daily attendance",
			path: "/xeapi/note/attendance/activity/register",
			call: func(ctx context.Context) error {
				_, err := endpoint.DailySongShareAttendanceRegister(ctx, &DailySongShareAttendanceRegisterReq{})
				return err
			},
		},
		{
			name: "daily guide",
			path: "/xeapi/note/attendance/activity/registration/v2/guide",
			call: func(ctx context.Context) error {
				_, err := endpoint.DailySongShareRegistrationGuide(ctx, nil)
				return err
			},
		},
		{
			name: "daily publish",
			path: "/xeapi/note/share/friends/resource",
			call: func(ctx context.Context) error {
				_, err := endpoint.DailySongSharePublish(ctx, &DailySongSharePublishReq{Msg: "test"})
				return err
			},
			wantHeaders: map[string]string{
				"Cmpageid":     "page_songlist",
				"Mconfig-Info": dailySongShareMConfigInfo,
			},
		},
		{
			name: "daily trigger",
			path: "/xeapi/music/song/share/trigger",
			call: func(ctx context.Context) error {
				_, err := endpoint.DailySongShareTrigger(ctx, &DailySongShareTriggerReq{SongId: "1"})
				return err
			},
		},
		{
			name: "daily lottery",
			path: "/xeapi/middle/play/do/lottery",
			call: func(ctx context.Context) error {
				_, err := endpoint.DailySongShareLottery(ctx, nil)
				return err
			},
		},
		{
			name: "member token",
			path: "/xeapi/vipactivity/app/vip/invitation/token/create",
			call: func(ctx context.Context) error {
				_, err := endpoint.VipMemberGiftTokenCreate(ctx, nil)
				return err
			},
		},
		{
			name: "member page",
			path: "/xeapi/vipactivity/app/vip/invitation/page/info",
			call: func(ctx context.Context) error {
				_, err := endpoint.VipMemberGiftPageInfo(ctx, nil)
				return err
			},
		},
		{
			name: "member detail",
			path: "/xeapi/vipactivity/app/vip/invitation/detail/info/get",
			call: func(ctx context.Context) error {
				_, err := endpoint.VipMemberGiftDetail(ctx, nil)
				return err
			},
		},
		{
			name: "member accept",
			path: "/xeapi/vipactivity/app/vip/invitation/accept",
			call: func(ctx context.Context) error {
				_, err := endpoint.VipMemberGiftAccept(ctx, &VipMemberGiftAcceptReq{Token: "token"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport.captured = capturedXeapiRequest{}

			require.NoError(t, tt.call(context.Background()))

			assert.Equal(t, http.MethodPost, transport.captured.method)
			assert.Equal(t, tt.path, transport.captured.path)
			assert.Equal(t, "application/x-www-form-urlencoded", transport.captured.header.Get("Content-Type"))
			assert.Equal(t, "ENCRYPTED", transport.captured.header.Get("X-Client-Enc-State"))
			assert.NotEmpty(t, transport.captured.form.Get("B"))
			assert.NotEmpty(t, transport.captured.form.Get("S"))
			assert.NotEmpty(t, transport.captured.form.Get("R"))

			for name, value := range tt.wantHeaders {
				assert.Equal(t, value, transport.captured.header.Get(name))
			}
		})
	}
}

type recordingXeapiTransport struct {
	captured capturedXeapiRequest
	body     []byte
}

func (t *recordingXeapiTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}

	t.captured = capturedXeapiRequest{
		method: request.Method,
		path:   request.URL.Path,
		header: request.Header.Clone(),
		form:   form,
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(t.body)),
		ContentLength: int64(len(t.body)),
		Request:       request,
	}, nil
}

func newOfflineXeapiClient(t *testing.T) (*api.Client, *recordingXeapiTransport, func()) {
	t.Helper()

	home := t.TempDir()
	stateDir := filepath.Join(home, ".ncmctl")
	require.NoError(t, os.MkdirAll(stateDir, 0o700))

	state := fmt.Sprintf(`publicKeyState:
  publicKey: 3m5wN9om11qRESjEV+5EoFf9qLEylO6gyThMbl1XxEk=
  version: "1000000000000"
  nextUpdateTime: %d
  sk: 8PZfbIFA1779944463972
`, time.Now().Add(time.Hour).UnixMilli())
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "xeapi.yaml"), []byte(state), 0o600))

	logger := log.New(&log.Config{Level: "error"})
	client, err := api.NewClient(&api.Config{
		Timeout: time.Second,
		HomeDir: home,
		Cookie: cookie.Config{
			Filepath: filepath.Join(home, "cookie.json"),
			Interval: 0,
		},
	}, logger)
	require.NoError(t, err)

	transport := &recordingXeapiTransport{body: encryptXeapiTestResponse(t, []byte(`{"code":200}`))}
	client.SetTransport(transport)

	cleanup := func() {
		require.NoError(t, client.Close(context.Background()))
		require.NoError(t, logger.Close())
	}
	return client, transport, cleanup
}

func encryptXeapiTestResponse(t *testing.T, plaintext []byte) []byte {
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

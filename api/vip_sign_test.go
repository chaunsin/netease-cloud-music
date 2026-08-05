// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package api_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	ncmcrypto "github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

const (
	vipHistoryResponse = `{
		"code": 200,
		"data": {
			"recordId": 1,
			"userId": 2,
			"time": 1785913200098,
			"songSrc": 3,
			"showTag": null,
			"songInfo": {
				"songId": 4,
				"songName": "test song",
				"artistName": "test artist",
				"album": "test album",
				"cover": "https://example.test/cover.jpg",
				"artistIds": [5],
				"seq": 0
			},
			"wishWords": "test wish",
			"wishWordType": 3,
			"wishUserNickname": null,
			"periodDto": {
				"periodType": 3,
				"startTime": "1722527999000",
				"endTime": "1893427199000"
			},
			"monthCheckInTotalDay": 3,
			"surprisePkgVo": null,
			"monthCheckInPrizList": [{
				"prizeId": 6,
				"vipType": 2,
				"prizeType": 0,
				"day": 28,
				"prizeShowName": "VIP",
				"showSubTitle": "3 days",
				"unitNum": 3,
				"userPrizeRecordId": 0,
				"time": 0
			}],
			"sceneId": 66,
			"jumpUrl": "https://example.test/detail"
		},
		"message": ""
	}`
	vipCardCompactResponse = `{
		"code": 200,
		"data": {
			"text": null,
			"subText": "compact card",
			"redPrize": false,
			"redDay": true,
			"btnText": "view",
			"bgTexture": "",
			"signInfoList": [{
				"dayText": "5",
				"sign": false,
				"songCoverUrl": null,
				"signTime": 0,
				"today": true
			}]
		},
		"message": ""
	}`
	vipCardDetailResponse = `{
		"code": 200,
		"data": {
			"text": "monthly music sign",
			"subText": "detail card",
			"redPrize": false,
			"redDay": true,
			"btnText": "view",
			"bgTexture": "https://example.test/background.png",
			"signInfoList": [{
				"dayText": "5",
				"sign": true,
				"songCoverUrl": "https://example.test/song.jpg",
				"signTime": 1785859200000,
				"today": true
			}]
		},
		"message": ""
	}`
)

type capturedEAPIRequest struct {
	host    string
	path    string
	query   string
	payload map[string]json.RawMessage
}

type recordingEAPIVipTransport struct {
	responses [][]byte
	requests  []capturedEAPIRequest
}

func (t *recordingEAPIVipTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}

	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}

	plaintext, err := ncmcrypto.EApiDecrypt(form.Get("params"), "hex")
	if err != nil {
		return nil, fmt.Errorf("decrypt EAPI request: %w", err)
	}

	parts := strings.SplitN(string(plaintext), "-36cd479b6b5-", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("decode EAPI envelope: got %d parts", len(parts))
	}

	payload := make(map[string]json.RawMessage)
	if err = json.Unmarshal([]byte(parts[1]), &payload); err != nil {
		return nil, fmt.Errorf("decode EAPI payload: %w", err)
	}

	t.requests = append(t.requests, capturedEAPIRequest{
		host:    request.URL.Host,
		path:    request.URL.Path,
		query:   request.URL.RawQuery,
		payload: payload,
	})
	responseBody := t.responses[len(t.requests)-1]

	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(responseBody)),
		ContentLength: int64(len(responseBody)),
		Request:       request,
	}, nil
}

type capturedWEAPIRequest struct {
	host  string
	path  string
	query string
}

type recordingWEAPIVipTransport struct {
	responses [][]byte
	requests  []capturedWEAPIRequest
}

func (t *recordingWEAPIVipTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if _, err := io.Copy(io.Discard, request.Body); err != nil {
		return nil, err
	}

	t.requests = append(t.requests, capturedWEAPIRequest{
		host:  request.URL.Host,
		path:  request.URL.Path,
		query: request.URL.RawQuery,
	})
	responseBody := t.responses[len(t.requests)-1]

	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(responseBody)),
		ContentLength: int64(len(responseBody)),
		Request:       request,
	}, nil
}

func TestEAPIVipMusicSignWireFlow(t *testing.T) {
	client := newVipTestClient(t)
	transport := &recordingEAPIVipTransport{responses: [][]byte{
		encryptEAPITestResponse(t, []byte(`{"code":200,"data":true,"message":""}`)),
		encryptEAPITestResponse(t, []byte(vipHistoryResponse)),
		encryptEAPITestResponse(t, []byte(vipCardCompactResponse)),
		encryptEAPITestResponse(t, []byte(vipCardDetailResponse)),
	}}
	client.GetClient().Transport = transport
	request := eapi.New(client)

	sign, err := request.VipTaskSign(context.Background(), &eapi.VipTaskSignReq{})
	require.NoError(t, err)
	assert.True(t, sign.Data)

	history, err := request.VipCheckinHistoryDetail(context.Background(), &eapi.VipCheckinHistoryDetailReq{
		SignDayTime: "1785913200098",
		Type:        "1",
	})
	require.NoError(t, err)
	require.NotNil(t, history.Data.SongInfo)
	assertVipHistoryResponse(t, history.Code, history.Data.SongInfo.SongName, len(history.Data.MonthCheckInPrizeList))

	compact, err := request.VipMinideskMusicSignPC(context.Background(), &eapi.VipMinideskMusicSignPCReq{Type: 0})
	require.NoError(t, err)
	assert.Nil(t, compact.Data.Text)
	require.Len(t, compact.Data.SignInfoList, 1)
	assert.Nil(t, compact.Data.SignInfoList[0].SongCoverUrl)

	detail, err := request.VipMinideskMusicSignPC(context.Background(), &eapi.VipMinideskMusicSignPCReq{Type: 1})
	require.NoError(t, err)
	require.NotNil(t, detail.Data.Text)
	assert.Equal(t, "monthly music sign", *detail.Data.Text)

	require.Len(t, transport.requests, 4)
	assert.Equal(t, []string{
		"/eapi/vip-center-bff/task/sign",
		"/eapi/vipnewcenter/app/level/user/checkin/history/detail",
		"/eapi/vipnewcenter/app/minidesk/music/sign/pc",
		"/eapi/vipnewcenter/app/minidesk/music/sign/pc",
	}, eapiRequestPaths(transport.requests))

	for _, captured := range transport.requests {
		assert.Equal(t, "interface.music.163.com", captured.host)
		assert.Empty(t, captured.query)
		assertJSONBool(t, captured.payload, "e_r", true)
	}

	assert.NotContains(t, transport.requests[0].payload, "isNew")
	assertJSONString(t, transport.requests[1].payload, "signDayTime", "1785913200098")
	assertJSONString(t, transport.requests[1].payload, "type", "1")
	assertJSONString(t, transport.requests[2].payload, "type", "0")
	assertJSONString(t, transport.requests[3].payload, "type", "1")
}

func TestWEAPIVipMusicSignEndpoints(t *testing.T) {
	client := newVipTestClient(t)
	transport := &recordingWEAPIVipTransport{responses: [][]byte{
		[]byte(vipHistoryResponse),
		[]byte(vipCardCompactResponse),
		[]byte(vipCardDetailResponse),
	}}
	client.GetClient().Transport = transport
	request := weapi.New(client)

	history, err := request.VipCheckinHistoryDetail(context.Background(), &weapi.VipCheckinHistoryDetailReq{
		SignDayTime: "10785913200098",
		Type:        "1",
	})
	require.NoError(t, err)
	require.NotNil(t, history.Data.SongInfo)
	assertVipHistoryResponse(t, int(history.Code), history.Data.SongInfo.SongName, len(history.Data.MonthCheckInPrizeList))

	compact, err := request.VipMinideskMusicSignPC(context.Background(), &weapi.VipMinideskMusicSignPCReq{Type: 0})
	require.NoError(t, err)
	assert.Nil(t, compact.Data.Text)

	detail, err := request.VipMinideskMusicSignPC(context.Background(), &weapi.VipMinideskMusicSignPCReq{Type: 1})
	require.NoError(t, err)
	require.NotNil(t, detail.Data.Text)
	assert.Equal(t, "monthly music sign", *detail.Data.Text)

	require.Len(t, transport.requests, 3)
	assert.Equal(t, []string{
		"/weapi/vipnewcenter/app/level/user/checkin/history/detail",
		"/weapi/vipnewcenter/app/minidesk/music/sign/pc",
		"/weapi/vipnewcenter/app/minidesk/music/sign/pc",
	}, weapiRequestPaths(transport.requests))

	for _, captured := range transport.requests {
		assert.Equal(t, "interface3.music.163.com", captured.host)
		assert.NotContains(t, captured.query, "signDayTime")
		assert.NotContains(t, captured.query, "type")
	}
}

func newVipTestClient(t *testing.T) *api.Client {
	t.Helper()

	home := t.TempDir()
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

	t.Cleanup(func() {
		require.NoError(t, client.Close(context.Background()))
		require.NoError(t, logger.Close())
	})
	return client
}

func encryptEAPITestResponse(t *testing.T, plaintext []byte) []byte {
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

func assertVipHistoryResponse(t *testing.T, code int, songName string, prizeCount int) {
	t.Helper()

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "test song", songName)
	assert.Equal(t, 1, prizeCount)
}

func eapiRequestPaths(requests []capturedEAPIRequest) []string {
	paths := make([]string, 0, len(requests))
	for _, request := range requests {
		paths = append(paths, request.path)
	}
	return paths
}

func weapiRequestPaths(requests []capturedWEAPIRequest) []string {
	paths := make([]string, 0, len(requests))
	for _, request := range requests {
		paths = append(paths, request.path)
	}
	return paths
}

func assertJSONBool(t *testing.T, payload map[string]json.RawMessage, name string, want bool) {
	t.Helper()

	var got bool
	require.NoError(t, json.Unmarshal(payload[name], &got))
	assert.Equal(t, want, got)
}

func assertJSONString(t *testing.T, payload map[string]json.RawMessage, name, want string) {
	t.Helper()

	var got string
	require.NoError(t, json.Unmarshal(payload[name], &got))
	assert.Equal(t, want, got)
}

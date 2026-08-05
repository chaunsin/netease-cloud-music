// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	ncmcrypto "github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

var (
	cli *Api
	ctx = context.TODO()
)

func TestMain(t *testing.M) {
	log.Default = log.New(&log.Config{
		Level:  "debug",
		Stdout: true,
	})
	cfg := &api.Config{
		Debug:   false,
		Timeout: 0,
		Retry:   0,
		Cookie: cookie.Config{
			Options:  nil,
			Filepath: "../../testdata/cookie.json",
			Interval: 0,
		},
	}

	client, err := api.NewClient(cfg, log.Default)
	if err != nil {
		panic(err)
	}

	cli = New(client)

	os.Exit(t.Run())
}

func TestEAPIRequestTypesEmbedCommon(t *testing.T) {
	commonType := reflect.TypeFor[types.EApiReqCommon]()
	commonJSONFields := map[string]struct{}{
		"e_r":      {},
		"header":   {},
		"deviceId": {},
		"os":       {},
		"verifyId": {},
	}
	requests := []any{
		ArtistHotReq{},
		DailySongShareRegisterReq{},
		DailySongShareAttendanceRegisterReq{},
		DailySongShareRegistrationGuideReq{},
		DailySongSharePublishReq{},
		DailySongShareTriggerReq{},
		DailySongShareLotteryReq{},
		EventPublishReq{},
		EventDeleteReq{},
		eventNosTokenReq{},
		eventUploadImgReq{},
		FansGroupDetailGetReq{},
		FansGroupMissionAllReq{},
		FansGroupFeedRecommendReq{},
		FansGroupMissionForwardProgressReq{},
		ResourceLikeReq{},
		FansGroupUserGroupDetailGetReq{},
		QrcodeCreateKeyReq{},
		QrcodeCheckReq{},
		GetUserInfoReq{},
		TokenRefreshReq{},
		MusicianVipTasksReq{},
		MusicianRoleGetReq{},
		MusicianSignReq{},
		MusicianMissionListReq{},
		MusicianRewardObtainReq{},
		PlaylistReq{},
		V3SongDetailReq{},
		v3SongDetailReq{},
		DiscoveryRecommendSongsReq{},
		SongLikeReq{},
		VipTaskListReq{},
		VipCommonReq{},
		VipTaskSignReq{},
		VipSignInfoReq{},
		VipGrowPointReq{},
		VipCheckinHistoryDetailReq{},
		VipMinideskMusicSignPCReq{},
		VipRewardGetAllReq{},
		VipWelfareListReq{},
		VipBenefitCategoryListReq{},
		VipBenefitGetReq{},
		TrialsongListenReq{},
		VipMemberGiftTokenCreateReq{},
		VipMemberGiftPageInfoReq{},
		VipMemberGiftDetailReq{},
		VipMemberGiftAcceptReq{},
		YunBeiSignInReq{},
		YunbeiClickTaskReq{},
		YunbeiDistributionRecommendSongReq{},
		YunbeiDistributionCreateReq{},
		YunbeiReserveInfoReq{},
		YunbeiReserveBookedReq{},
		YunbeiReserveRewardReceiveReq{},
		YunBeiTaskTodoReq{},
	}

	for _, request := range requests {
		requestType := reflect.TypeOf(request)
		t.Run(requestType.Name(), func(t *testing.T) {
			var hasCommon bool

			for i := range requestType.NumField() {
				field := requestType.Field(i)
				if field.Anonymous && field.Type == commonType {
					hasCommon = true
					continue
				}

				jsonName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				_, duplicate := commonJSONFields[jsonName]
				assert.False(t, duplicate, "%s redeclares common JSON field %q", requestType.Name(), jsonName)
			}

			require.True(t, hasCommon, "%s must directly embed types.EApiReqCommon", requestType.Name())
		})
	}
}

func TestEAPIRequestCommonJSONIsFlat(t *testing.T) {
	req := VipTaskListReq{
		EApiReqCommon: types.EApiReqCommon{
			Header:   `{"os":"ios"}`,
			DeviceId: "device-id",
			OS:       "iOS",
			VerifyId: 1,
		},
		IsNew: 1,
	}
	req.SetResponseEncrypted(false)

	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"e_r": false,
		"header": "{\"os\":\"ios\"}",
		"deviceId": "device-id",
		"os": "iOS",
		"verifyId": 1,
		"isNew": 1
	}`, string(data))
	assert.Equal(t, 1, strings.Count(string(data), `"e_r"`))
	assert.NotContains(t, string(data), "EApiReqCommon")
}

func TestMusicianVipTasksResponseMode(t *testing.T) {
	tests := []struct {
		name          string
		req           func() *MusicianVipTasksReq
		responseBody  func(*testing.T) []byte
		wantEncrypted bool
	}{
		{
			name: "zero value keeps plaintext response",
			req:  func() *MusicianVipTasksReq { return &MusicianVipTasksReq{} },
			responseBody: func(*testing.T) []byte {
				return []byte(`{"code":200,"data":{}}`)
			},
		},
		{
			name: "explicit encrypted response",
			req: func() *MusicianVipTasksReq {
				req := &MusicianVipTasksReq{}
				req.SetResponseEncrypted(true)
				return req
			},
			responseBody: func(t *testing.T) []byte {
				t.Helper()

				return encryptXeapiTestResponse(t, []byte(`{"code":200,"data":{}}`))
			},
			wantEncrypted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := newOfflineEAPIClient(t, tt.responseBody(t))
			req := tt.req()

			_, err := New(client).MusicianVipTasks(context.Background(), req)
			require.NoError(t, err)
			require.NotNil(t, req.ER)
			assert.Equal(t, tt.wantEncrypted, *req.ER)
			assertJSONBoolField(t, transport.payload, "e_r", tt.wantEncrypted)
		})
	}
}

func TestV3SongDetailForwardsEAPICommon(t *testing.T) {
	client, transport := newOfflineEAPIClient(t, []byte(`{"code":200}`))
	req := &V3SongDetailReq{
		EApiReqCommon: types.EApiReqCommon{
			Header:   `{"source":"test"}`,
			DeviceId: "device-id",
			OS:       "iOS",
			VerifyId: 7,
		},
		C: []V3SongDetailReqList{{Id: "1", V: 0}},
	}
	req.SetResponseEncrypted(false)

	_, err := New(client).V3SongDetail(context.Background(), req)
	require.NoError(t, err)
	assertJSONBoolField(t, transport.payload, "e_r", false)
	assertJSONStringField(t, transport.payload, "header", req.Header)
	assertJSONStringField(t, transport.payload, "deviceId", req.DeviceId)
	assertJSONStringField(t, transport.payload, "os", req.OS)
	assertJSONIntField(t, transport.payload, "verifyId", int64(req.VerifyId))
	assertJSONStringField(t, transport.payload, "c", `[{"id":"1","v":0}]`)
}

func TestFansGroupHonorsPlainResponseMode(t *testing.T) {
	client, transport := newOfflineEAPIClient(t, []byte(`{"code":200}`))
	req := &FansGroupDetailGetReq{GroupId: "group-id"}
	req.SetResponseEncrypted(false)

	_, err := New(client).FansGroupDetailGet(context.Background(), req)
	require.NoError(t, err)
	assertJSONBoolField(t, transport.payload, "e_r", false)
}

type recordingEAPITransport struct {
	responseBody []byte
	payload      map[string]json.RawMessage
}

func (t *recordingEAPITransport) RoundTrip(request *http.Request) (*http.Response, error) {
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

	if err = json.Unmarshal([]byte(parts[1]), &t.payload); err != nil {
		return nil, fmt.Errorf("decode EAPI request: %w", err)
	}

	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(t.responseBody)),
		ContentLength: int64(len(t.responseBody)),
		Request:       request,
	}, nil
}

func newOfflineEAPIClient(t *testing.T, responseBody []byte) (*api.Client, *recordingEAPITransport) {
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

	transport := &recordingEAPITransport{responseBody: responseBody}
	client.GetClient().Transport = transport
	return client, transport
}

func assertJSONBoolField(t *testing.T, payload map[string]json.RawMessage, name string, want bool) {
	t.Helper()

	var got bool
	require.NoError(t, json.Unmarshal(payload[name], &got))
	assert.Equal(t, want, got)
}

func assertJSONStringField(t *testing.T, payload map[string]json.RawMessage, name, want string) {
	t.Helper()

	var got string
	require.NoError(t, json.Unmarshal(payload[name], &got))
	assert.Equal(t, want, got)
}

func assertJSONIntField(t *testing.T, payload map[string]json.RawMessage, name string, want int64) {
	t.Helper()

	var got int64
	require.NoError(t, json.Unmarshal(payload[name], &got))
	assert.Equal(t, want, got)
}

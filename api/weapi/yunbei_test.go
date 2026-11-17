// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYunBeiTaskListV3ResponseDecoding(t *testing.T) {
	var got YunBeiTaskListV3Resp
	require.NoError(t, json.Unmarshal([]byte(`{
		"code": 200,
		"data": {
			"newbie": null,
			"normal": {
				"list": [{
					"completed": false,
					"completedPoint": 2,
					"extInfoMap": {"missionCode": "fixed-vector-code"},
					"period": 1,
					"status": 0,
					"subAction": "fixed-action",
					"targetPoint": 10,
					"targetUserTaskId": 456,
					"taskDescription": "fixed response",
					"taskId": 123,
					"taskName": "fixed task",
					"taskPoint": 5,
					"taskPointDetail": [{
						"progressRate": 2,
						"sumTarget": 10,
						"times": 1,
						"userMissionId": 789,
						"value": 2,
						"worth": 5
					}],
					"taskType": "NORMAL",
					"userTaskId": 456
				}]
			}
		}
	}`), &got))
	require.Equal(t, int64(200), got.Code)
	require.Len(t, got.Data.Normal.List, 1)

	item := got.Data.Normal.List[0]
	require.NotNil(t, item.ExtInfoMap)
	require.Equal(t, "fixed-vector-code", item.ExtInfoMap.MissionCode)
	require.Equal(t, int64(123), item.TaskId)
	require.Equal(t, int64(456), item.UserTaskId)
	require.Equal(t, int64(2), item.CompletedPoint)
	require.Equal(t, int64(10), item.TargetPoint)
	require.Equal(t, "fixed-action", item.SubAction)
	require.Len(t, item.TaskPointDetail, 1)
	require.Equal(t, int64(789), item.TaskPointDetail[0].UserMissionId)
}

func TestYunBeiTaskListV3TracksNonEmptyUntypedBuckets(t *testing.T) {
	var got YunBeiTaskListV3Resp
	require.NoError(t, json.Unmarshal([]byte(`{
		"code": 200,
		"data": {
			"newbie": {},
			"normal": {"list": []},
			"seasonal": {"list": [{"taskId": 1}]},
			"future": null
		}
	}`), &got))
	require.Len(t, got.Data.UnknownBucket, 1)
	require.Contains(t, got.Data.UnknownBucket, "seasonal")
}

type recordingYunBeiWEAPITransport struct {
	responses [][]byte
	paths     []string
	hosts     []string
}

func (t *recordingYunBeiWEAPITransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if _, err := io.Copy(io.Discard, request.Body); err != nil {
		return nil, err
	}

	t.paths = append(t.paths, request.URL.Path)
	t.hosts = append(t.hosts, request.URL.Host)
	body := t.responses[len(t.paths)-1]
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       request,
	}, nil
}

func TestYunBeiRequestContracts(t *testing.T) {
	request := newOfflineWEAPI(t)
	transport := &recordingYunBeiWEAPITransport{responses: [][]byte{
		[]byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[]}}}`),
		[]byte(`{"code":200,"data":true}`),
		[]byte(`{"code":200,"data":{"code":200,"data":true,"message":""},"message":""}`),
		[]byte(`{"code":200,"data":{"times":2,"amount":30,"singleAmount":15}}`),
		[]byte(`{"code":200,"data":[{"songId":9,"songName":"indie","completeStatus":false}]}`),
		[]byte(`{"code":200,"data":true}`),
		[]byte(`{"code":200,"data":[{"id":7,"name":"artist"}]}`),
		[]byte(`{"code":200,"ids":[9]}`),
		[]byte(`{"code":200,"playlistId":5}`),
	}}
	request.client.SetTransport(transport)

	_, err := request.YunBeiTaskListV3(context.Background(), &YunBeiTaskListV3Req{})
	require.NoError(t, err)
	_, err = request.YunBeiTaskFinish(context.Background(), &YunBeiTaskFinishReq{
		Period: "1", UserTaskId: "456", DepositCode: "1304",
	})
	require.NoError(t, err)
	vipCenter, err := request.YunBeiVIPCenterTaskExternal(context.Background(), &YunBeiVIPCenterTaskExternalReq{Type: 1})
	require.NoError(t, err)
	assert.True(t, vipCenter.Data.Data)

	distribution, err := request.YunBeiDistributionList(context.Background(), &YunBeiDistributionListReq{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), distribution.Data.Times)
	assert.Equal(t, int64(15), distribution.Data.SingleAmount)

	recommended, err := request.YunBeiDistributionRecommendSong(context.Background(), &YunBeiDistributionRecommendSongReq{
		Offset: 2,
		Limit:  1,
	})
	require.NoError(t, err)
	require.Len(t, recommended.Data, 1)
	assert.Equal(t, int64(9), recommended.Data[0].SongId)

	created, err := request.YunBeiDistributionCreate(context.Background(), &YunBeiDistributionCreateReq{YunbeiAmount: 15})
	require.NoError(t, err)
	assert.True(t, created.Data)

	artists, err := request.ArtistSublist(context.Background(), &ArtistSublistReq{Limit: 1000, Total: true})
	require.NoError(t, err)
	require.Len(t, artists.Data, 1)
	assert.Equal(t, int64(7), artists.Data[0].Id)

	liked, err := request.SongLikeList(context.Background(), &SongLikeListReq{Uid: 42})
	require.NoError(t, err)
	assert.Equal(t, []int64{9}, liked.Ids)

	heart, err := request.SongLike(context.Background(), &SongLikeReq{TrackId: 9, Like: true, Alg: "daily"})
	require.NoError(t, err)
	assert.Equal(t, int64(5), heart.PlaylistId)
	assert.Equal(t, []string{
		"/weapi/usertool/task/list/all/v3",
		"/weapi/usertool/task/point/receive",
		"/weapi/vipnewcenter/app/level/task/external",
		"/weapi/ad/power/yunbei/distribution/list",
		"/weapi/ad/power/yunbei/distribution/recommend/song",
		"/weapi/ad/power/yunbei/distribution/create",
		"/weapi/artist/sublist",
		"/weapi/song/like/get",
		"/weapi/song/like",
	}, transport.paths)
	assert.Equal(t, "interface.music.163.com", transport.hosts[2])

	payload, err := json.Marshal(YunBeiTaskFinishReq{Period: "1", UserTaskId: "456", DepositCode: "1304"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"period":"1","userTaskId":"456","depositCode":"1304"}`, string(payload))
	payload, err = json.Marshal(YunBeiDistributionCreateReq{YunbeiAmount: 15})
	require.NoError(t, err)
	assert.JSONEq(t, `{"yunbeiAmount":15}`, string(payload))
	payload, err = json.Marshal(SongLikeReq{TrackId: 9, Like: true, Alg: "daily"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"trackId":9,"like":true,"alg":"daily"}`, string(payload))
}

func TestYunBeiSign(t *testing.T) {
	cli := newLiveWEAPI(t)

	req := YunBeiSignInReq{}
	got, err := cli.YunBeiSignIn(t.Context(), &req)
	require.NoError(t, err)
	t.Logf("YunBeiSignIn: %+v\n", got)
}

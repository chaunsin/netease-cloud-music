// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYunBeiTaskTodoResponseDecoding(t *testing.T) {
	var got YunBeiTaskTodoResp
	require.NoError(t, json.Unmarshal([]byte(`{
		"code": 200,
		"data": [{
			"completed": true,
			"depositCode": 1304,
			"period": 1,
			"taskId": 123,
			"taskName": "fixed task",
			"taskPoint": 5,
			"userTaskId": 456
		}]
	}`), &got))
	require.Len(t, got.Data, 1)
	require.Equal(t, int64(123), got.Data[0].TaskId)
	require.Equal(t, int64(456), got.Data[0].UserTaskId)
	require.Equal(t, int64(1304), got.Data[0].DepositCode)
}

func TestYunBeiTaskTodoRequestContract(t *testing.T) {
	client, transport := newOfflineEAPIClient(t, []byte(`{
		"code": 200,
		"data": [{"taskId": 123, "userTaskId": 456}]
	}`))
	request := &YunBeiTaskTodoReq{}
	request.SetResponseEncrypted(false)

	got, err := New(client).YunBeiTaskTodo(t.Context(), request)
	require.NoError(t, err)
	require.Len(t, got.Data, 1)
	assert.Equal(t, "/eapi/usertool/task/todo/query", transport.path)
	assertEAPIResponseEncrypted(t, transport.payload, false)
	assertJSONStringField(t, transport.payload, "header", "{}")
	assert.Equal(t, int64(123), got.Data[0].TaskId)
	assert.Equal(t, int64(456), got.Data[0].UserTaskId)
}

func TestYunbeiReserveInfoResponseDecodesCheckToken(t *testing.T) {
	client, _ := newOfflineEAPIClient(t, []byte(`{
		"code": 200,
		"success": true,
		"data": {"type": "PREV_BOOKED_UNCLAIMED", "reqId": "request-id", "checkToken": "token"}
	}`))
	request := &YunbeiReserveInfoReq{}
	request.SetResponseEncrypted(false)

	got, err := New(client).YunbeiReserveInfo(t.Context(), request)
	require.NoError(t, err)
	assert.Equal(t, "token", got.Data.CheckToken)
}

func TestYunbeiReserveWriteRequestContracts(t *testing.T) {
	bookClient, bookTransport := newOfflineEAPIClient(t, []byte(`{"code":200,"success":true}`))
	bookRequest := &YunbeiReserveBookedReq{ReqId: "new-request"}
	bookRequest.SetResponseEncrypted(false)

	_, err := New(bookClient).YunbeiReserveBooked(t.Context(), bookRequest)
	require.NoError(t, err)
	assert.Equal(t, "/eapi/new/yunbei/activity/reserve/booked", bookTransport.path)
	assertJSONStringField(t, bookTransport.payload, "reqId", "new-request")

	claimClient, claimTransport := newOfflineEAPIClient(t, []byte(`{"code":200,"success":true}`))
	claimRequest := &YunbeiReserveRewardReceiveReq{ReqId: "old-request", CheckToken: "claim-token"}
	claimRequest.SetResponseEncrypted(false)

	_, err = New(claimClient).YunbeiReserveRewardReceive(t.Context(), claimRequest)
	require.NoError(t, err)
	assert.Equal(t, "/eapi/new/yunbei/activity/reserve/reward/receive", claimTransport.path)
	assertJSONStringField(t, claimTransport.payload, "reqId", "old-request")
	assertJSONStringField(t, claimTransport.payload, "checkToken", "claim-token")
}

func TestYunBeiInSign(t *testing.T) {
	cli := newLiveEAPI(t)

	req := YunBeiSignInReq{
		Type: 1,
	}
	got, err := cli.YunBeiSignIn(t.Context(), &req)
	require.NoError(t, err)
	t.Logf("YunBeiSignIn: %+v\n", got)
}

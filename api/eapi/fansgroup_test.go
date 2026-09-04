// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFansGroupMemberDetailIdentify 校验成员详情的身份码键名 (2026-09-04 实测载荷键为 identify,
// 原结构体误写 json:"identity" 导致字段永不解析).
func TestFansGroupMemberDetailIdentify(t *testing.T) {
	var resp FansGroupUserGroupDetailGetResp

	err := json.Unmarshal([]byte(`{
		"code": 200,
		"message": null,
		"data": {"fansGroupMemberDetail": {"userId": 1289504343, "identify": 99}}
	}`), &resp)
	require.NoError(t, err)
	assert.Equal(t, int64(99), resp.Data.FansGroupMemberDetail.Identify)
}

// TestFansGroupFeedRecommendPics 校验动态图片两层建模 (2026-09-04 实测: 扁平层仅含
// URL/Str-ID/尺寸, 数字形态 ID 全集在嵌套 picInfo 内).
func TestFansGroupFeedRecommendPics(t *testing.T) {
	var resp FansGroupFeedRecommendResp

	err := json.Unmarshal([]byte(`{
		"code": 200,
		"data": {"records": [{
			"pics": [{
				"originUrl": "http://p1.music.126.net/a/109951173872813731.jpg",
				"squareUrl": "http://p1.music.126.net/b/109951173881483462.jpg",
				"rectangleUrl": "http://p1.music.126.net/c/109951173881494057.jpg",
				"originId": 109951173872813731,
				"originIdStr": "109951173872813731",
				"squareIdStr": "109951173881483462",
				"format": "jpg",
				"width": 1920,
				"height": 1920,
				"videoNosKey": null,
				"videoDurationMs": 0,
				"picInfo": {
					"originId": 109951173872813731,
					"squareId": 109951173881483462,
					"rectangleId": 109951173881494057,
					"pcSquareId": 109951173881493596,
					"pcRectangleId": 109951173881493115,
					"originJpgId": 109951173872813735,
					"transcodeStatus": null,
					"videoId": null
				}
			}]
		}]}
	}`), &resp)
	require.NoError(t, err)

	require.Len(t, resp.Data.Records, 1)
	require.Len(t, resp.Data.Records[0].Pics, 1)
	pic := resp.Data.Records[0].Pics[0]

	// 扁平层
	assert.Equal(t, "http://p1.music.126.net/a/109951173872813731.jpg", pic.OriginUrl)
	assert.Equal(t, "http://p1.music.126.net/b/109951173881483462.jpg", pic.SquareUrl)
	assert.Equal(t, "http://p1.music.126.net/c/109951173881494057.jpg", pic.RectangleUrl)
	assert.Equal(t, int64(109951173872813731), pic.OriginId)
	assert.Equal(t, "jpg", pic.Format)

	// 嵌套层: 数字 ID 全集 (原单一 PicInfo 直接建模扁平层时这些值全部丢失)
	assert.Equal(t, int64(109951173881483462), pic.PicInfo.SquareId)
	assert.Equal(t, int64(109951173881494057), pic.PicInfo.RectangleId)
	assert.Equal(t, int64(109951173881493596), pic.PicInfo.PcSquareId)
	assert.Equal(t, int64(109951173881493115), pic.PicInfo.PcRectangleId)
	assert.Equal(t, int64(109951173872813735), pic.PicInfo.OriginJpgId)
}

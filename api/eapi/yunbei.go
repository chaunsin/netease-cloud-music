// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package eapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type YunBeiSignInReq struct {
	// Type 签到类型 0:安卓(默认)3点经验 1:web/PC2点经验
	Type int64 `json:"type"`
}

// YunBeiSignInResp 签到返回
type YunBeiSignInResp struct {
	// Code 错误码 -2:重复签到 200:成功(会有例外会出现“功能暂不支持”) 301:未登录
	types.RespCommon[any]
	// Point 签到获得积分奖励数量
	Point int64 `json:"point"`
}

// YunBeiSignIn 用户每日签到
// url:
// needLogin: 是
// todo:目前传0会出现功能暂不支持不知为何(可能请求头或cookie问题)待填坑
func (a *Api) YunBeiSignIn(ctx context.Context, req *YunBeiSignInReq) (*YunBeiSignInResp, error) {
	var (
		url   = "https://music.163.com/eapi/point/dailyTask"
		reply YunBeiSignInResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunbeiClickTaskReq struct {
	TaskId     int64  `json:"taskId"`
	SubAction  string `json:"subAction"`
	Type       string `json:"type"`
	CheckToken string `json:"checkToken"`
}

type YunbeiClickTaskResp struct {
	Code    int    `json:"code"`
	Data    bool   `json:"data"`
	Message string `json:"message"`
}

// YunbeiClickTask 宣告浏览会员中心等任务开始
func (a *Api) YunbeiClickTask(ctx context.Context, req *YunbeiClickTaskReq) (*YunbeiClickTaskResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/yunbei/click/task"
		reply YunbeiClickTaskResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunbeiDistributionRecommendSongReq struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type YunbeiDistributionRecommendSongResp struct {
	Code int `json:"code"`
	Data []struct {
		SongId   int64 `json:"songId"`
		AlbumId  int64 `json:"albumId"`
		ArtistId int64 `json:"artistId"`
	} `json:"data"`
}

// YunbeiDistributionRecommendSong 获取探索小众歌曲推荐列表
func (a *Api) YunbeiDistributionRecommendSong(ctx context.Context, req *YunbeiDistributionRecommendSongReq) (*YunbeiDistributionRecommendSongResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/ad/power/yunbei/distribution/recommend/song"
		reply YunbeiDistributionRecommendSongResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunbeiDistributionCreateReq struct {
	YunbeiAmount int64 `json:"yunbeiAmount"`
}

type YunbeiDistributionCreateResp struct {
	Code    int    `json:"code"`
	Data    bool   `json:"data"`
	Message string `json:"message"`
}

// YunbeiDistributionCreate 完成小众歌曲听歌任务并申请云贝分配
func (a *Api) YunbeiDistributionCreate(ctx context.Context, req *YunbeiDistributionCreateReq) (*YunbeiDistributionCreateResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/ad/power/yunbei/distribution/create"
		reply YunbeiDistributionCreateResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunbeiReserveInfoReq struct {
}

type YunbeiReserveInfoResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    struct {
		Type          string `json:"type"`
		ReqId         string `json:"reqId"`
		CurrentAmount int64  `json:"currentAmount"`
	} `json:"data"`
}

// YunbeiReserveInfo 获取预约活动领云贝信息
func (a *Api) YunbeiReserveInfo(ctx context.Context, req *YunbeiReserveInfoReq) (*YunbeiReserveInfoResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/new/yunbei/activity/reserve/info"
		reply YunbeiReserveInfoResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunbeiReserveBookedReq struct {
	ReqId string `json:"reqId"`
}

type YunbeiReserveBookedResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

// YunbeiReserveBooked 预约领云贝
func (a *Api) YunbeiReserveBooked(ctx context.Context, req *YunbeiReserveBookedReq) (*YunbeiReserveBookedResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/new/yunbei/activity/reserve/booked"
		reply YunbeiReserveBookedResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunbeiReserveRewardReceiveReq struct {
	ReqId      string `json:"reqId"`
	CheckToken string `json:"checkToken"`
}

type YunbeiReserveRewardReceiveResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    struct {
		CurrentAmount int64 `json:"currentAmount"`
	} `json:"data"`
}

// YunbeiReserveRewardReceive 领取预约奖励云贝
func (a *Api) YunbeiReserveRewardReceive(ctx context.Context, req *YunbeiReserveRewardReceiveReq) (*YunbeiReserveRewardReceiveResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/new/yunbei/activity/reserve/reward/receive"
		reply YunbeiReserveRewardReceiveResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

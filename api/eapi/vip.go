// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type VipTaskListReq struct {
	DeviceId string `json:"deviceId,omitempty"`
	OS       string `json:"os,omitempty"`
	VerifyId int    `json:"verifyId,omitempty"`
	Header   any    `json:"header"`
	IsNew    int    `json:"isNew,omitempty"`
	ER       bool   `json:"e_r,omitempty"`
}

type VipTaskListResp struct {
	Code int               `json:"code"`
	Data []VipTaskListData `json:"data"`
}

type VipTaskListData struct {
	Point           int64  `json:"point"`
	MissionId       int64  `json:"missionId"`
	MissionType     int64  `json:"missionType"`
	MissionEntityId int64  `json:"missionEntityId"`
	MissionCode     string `json:"missionCode"`
	Status          int64  `json:"status"` // 100: 已打卡/已完成 10: 未完成
	Worth           int64  `json:"worth"`
	MainTitle       string `json:"mainTitle"`
	SubTitle        string `json:"subTitle"`
	JumpUrl         string `json:"jumpUrl"`
	ButtonText      string `json:"buttonText"`
}

// VipTaskList 获取黑胶 VIP 任务列表
func (a *Api) VipTaskList(ctx context.Context, req *VipTaskListReq) (*VipTaskListResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vip-center-bff/task/list"
		reply VipTaskListResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipCommonReq struct {
	DeviceId string `json:"deviceId,omitempty"`
	OS       string `json:"os,omitempty"`
	VerifyId int    `json:"verifyId,omitempty"`
	Header   any    `json:"header"`
	ER       bool   `json:"e_r,omitempty"`
}

type VipCommonResp struct {
	Code    int    `json:"code"`
	Data    any    `json:"data"`
	Message string `json:"message"`
}

type VipTaskSignReq struct {
	Header any    `json:"header"`
	IsNew  string `json:"isNew"`
	ER     bool   `json:"e_r,omitempty"`
}

type VipTaskSignResp struct {
	Code    int    `json:"code"`
	Data    bool   `json:"data"`
	Message string `json:"message"`
}

// VipTaskSign 执行尊享 VIP 签到 (EAPI)
func (a *Api) VipTaskSign(ctx context.Context, req *VipTaskSignReq) (*VipTaskSignResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vip-center-bff/task/sign"
		reply VipTaskSignResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipSignInfoReq struct {
	VipCommonReq
}

type VipSignInfoResp struct {
	Code    int               `json:"code"`
	Data    []VipSignInfoData `json:"data"`
	Message string            `json:"message"`
}

type VipSignInfoData struct {
	RecordId  int64  `json:"recordId"`
	UserId    int64  `json:"userId"`
	Time      int64  `json:"time"`
	TimeStr   string `json:"timeStr"`
	SongId    int64  `json:"songId"`
	SongCover any    `json:"songCover"`
	Score     int64  `json:"score"`
	Today     bool   `json:"today"`
}

// VipSignInfo 获取黑胶乐签最近签到记录 (EAPI)。
func (a *Api) VipSignInfo(ctx context.Context, req *VipSignInfoReq) (*VipSignInfoResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/user/sign/info"
		reply VipSignInfoResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipGrowPointReq struct {
	VipCommonReq
}

type VipGrowPointResp struct {
	Code    int              `json:"code"`
	Data    VipGrowPointData `json:"data"`
	Message string           `json:"message"`
}

type VipGrowPointData struct {
	UserLevel VipGrowPointUserLevel `json:"userLevel"`
}

type VipGrowPointUserLevel struct {
	UserId          int64  `json:"userId"`
	Level           int64  `json:"level"`
	GrowthPoint     int64  `json:"growthPoint"`
	LevelName       string `json:"levelName"`
	ExtJson         string `json:"extJson"`
	LatestVipStatus int64  `json:"latestVipStatus"`
}

// VipGrowPoint 获取黑胶成长值状态 (EAPI)。
func (a *Api) VipGrowPoint(ctx context.Context, req *VipGrowPointReq) (*VipGrowPointResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/level/growhpoint/basic"
		reply VipGrowPointResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// VipOldSignPrizeList 获取旧版连续乐签奖品列表，用于模拟 App 打卡后的刷新链路。
func (a *Api) VipOldSignPrizeList(ctx context.Context, req *VipCommonReq) (*VipCommonResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/level/user/checkin/old/sign-prize/list"
		reply VipCommonResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipMonthPrizeListResp struct {
	Code int `json:"code"`
	Data struct {
		MonthCheckInTotalDay int64 `json:"monthCheckInTotalDay"`
		NextPrzieRemaingDay  int64 `json:"nextPrzieRemaingDay"`
		TodayDailyGrowth     int64 `json:"todayDailyGrowth"`
		PrizeList            []struct {
			Day               int64  `json:"day"`
			PrizeId           int64  `json:"prizeId"`
			PrizeShowName     string `json:"prizeShowName"`
			PrizeType         int64  `json:"prizeType"`
			ShowSubTitle      string `json:"showSubTitle"`
			Time              int64  `json:"time"`
			UnitNum           int64  `json:"unitNum"`
			UserPrizeRecordId int64  `json:"userPrizeRecordId"`
			VipType           int64  `json:"vipType"`
		} `json:"przeList"`
	} `json:"data"`
	Message string `json:"message"`
}

// VipMonthPrizeList 获取本月乐签奖品列表，用于模拟 App 打卡后的刷新链路。
func (a *Api) VipMonthPrizeList(ctx context.Context, req *VipCommonReq) (*VipMonthPrizeListResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/level/user/checkin/month-prize/list"
		reply VipMonthPrizeListResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// VipFrontInfo 获取会员前台信息，用于模拟 App 打卡后的刷新链路。
func (a *Api) VipFrontInfo(ctx context.Context, req *VipCommonReq) (*VipCommonResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/music-vip-membership/front/vip/info"
		reply VipCommonResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipCheckinHistoryDetailReq struct {
	VipCommonReq
	SignDayTime int64 `json:"-"`
	Type        int   `json:"-"`
}

// VipCheckinHistoryDetail 获取指定日期乐签详情，用于模拟 App 打卡后的刷新链路。
func (a *Api) VipCheckinHistoryDetail(ctx context.Context, req *VipCheckinHistoryDetailReq) (*VipCommonResp, error) {
	if req.Type == 0 {
		req.Type = 1
	}
	var (
		url = fmt.Sprintf(
			"https://interface3.music.163.com/eapi/vipnewcenter/app/level/user/checkin/history/detail?signDayTime=%d&type=%d",
			req.SignDayTime,
			req.Type,
		)
		reply VipCommonResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipRewardGetAllReq struct {
	DeviceId string `json:"deviceId,omitempty"`
	OS       string `json:"os,omitempty"`
	VerifyId int    `json:"verifyId,omitempty"`
	Header   any    `json:"header,omitempty"`
	ER       bool   `json:"e_r,omitempty"`
}

type VipRewardGetAllResp struct {
	Code int `json:"code"`
	Data struct {
		Result bool `json:"result"`
	} `json:"data"`
	Message string `json:"message"`
}

// VipRewardGetAll 一键领取所有黑胶 VIP 成长值 (EAPI)
func (a *Api) VipRewardGetAll(ctx context.Context, req *VipRewardGetAllReq) (*VipRewardGetAllResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/level/task/reward/getall"
		reply VipRewardGetAllResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipWelfareListReq struct {
	DeviceId string `json:"deviceId,omitempty"`
	OS       string `json:"os,omitempty"`
	VerifyId int    `json:"verifyId,omitempty"`
	Header   any    `json:"header"`
}

type VipWelfareListResp struct {
	Code int `json:"code"`
	Data any `json:"data"`
}

// VipWelfareList 获取会员等级福利列表 (EAPI)
func (a *Api) VipWelfareList(ctx context.Context, req *VipWelfareListReq) (*VipWelfareListResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/level/welfare/new/list"
		reply VipWelfareListResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipBenefitCategoryListReq struct {
	Category string `json:"category"`
	Header   any    `json:"header"`
	ER       bool   `json:"e_r,omitempty"`
}

type VipBenefitCategoryListResp struct {
	Code int                      `json:"code"`
	Data []VipBenefitCategoryData `json:"data"`
}

type VipBenefitCategoryData struct {
	Id         int64  `json:"id"`
	Name       string `json:"name"`
	BenefitGet bool   `json:"benefitGet"`
}

// VipBenefitCategoryList 获取分类下免费福利券列表
func (a *Api) VipBenefitCategoryList(ctx context.Context, req *VipBenefitCategoryListReq) (*VipBenefitCategoryListResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/benefitcenter/benefits/category/list"
		reply VipBenefitCategoryListResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipBenefitGetReq struct {
	Id     string `json:"id"`
	Header any    `json:"header"`
	ER     bool   `json:"e_r,omitempty"`
}

type VipBenefitGetResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		BenefitGet bool `json:"benefitGet"`
	} `json:"result"`
}

// VipBenefitGet 领取免费商家福利券
func (a *Api) VipBenefitGet(ctx context.Context, req *VipBenefitGetReq) (*VipBenefitGetResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipcenter/benefits/get"
		reply VipBenefitGetResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type TrialsongListenReq struct {
	types.EApiReqCommon
	SongId  string `json:"songId"`
	AlbumId string `json:"albumId"`
	Scene   int    `json:"scene"`
}

type TrialsongListenResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    bool   `json:"data"`
}

// TrialsongListen 上报听歌状态（黑胶/小众歌曲打卡）
func (a *Api) TrialsongListen(ctx context.Context, req *TrialsongListenReq) (*TrialsongListenResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipmall/interest/trialsong/listen"
		reply TrialsongListenResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

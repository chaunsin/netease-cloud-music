// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

// Musician VIP Tasks API
// Ported from https://github.com/neteasecloudmusicapienhanced/api-enhanced

package eapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

// MusicianVipTasksReq 获取音乐人黑胶会员任务请求
type MusicianVipTasksReq struct {
	ER bool `json:"e_r"` // false=明文响应, true=加密响应
}

// MusicianVipTasksData 获取音乐人黑胶会员任务响应 (data 字段内容)
type MusicianVipTasksData struct {
	HasOpen              bool                    `json:"hasOpen"`
	IsMusician           bool                    `json:"isMusician"`
	CanOpen              bool                    `json:"canOpen"`
	HasFurtherTask       bool                    `json:"hasFurtherTask"`
	TaskStatus           bool                    `json:"taskStatus"`
	MusicianType         int                     `json:"musicianType"`
	Status               int                     `json:"status"`
	MaintainDays         int                     `json:"maintainDays"`
	RecentPlayCount30    int                     `json:"recentPlayCount30"`
	IsTodayStart         bool                    `json:"isTodayStart"`
	IsGrowthSupportUser  bool                    `json:"isGrowthSupportUser"`
	UnlockVipRight       bool                    `json:"unlockVipRight"`
	FurtherVipGetTime    int64                   `json:"furtherVipGetTime"`
	FurtherTaskStartTime int64                   `json:"furtherTaskStartTime"`
	FurtherTask          *MusicianVipFurtherTask `json:"furtherTask"`
}

// MusicianVipTasksResp 获取音乐人黑胶会员任务响应
type MusicianVipTasksResp struct {
	types.RespCommon[MusicianVipTasksData]
}

// MusicianEAPIReq 音乐人 EAPI 通用请求字段。
type MusicianEAPIReq struct {
	DeviceId string      `json:"deviceId,omitempty"`
	OS       string      `json:"os,omitempty"`
	VerifyId int         `json:"verifyId,omitempty"`
	Header   interface{} `json:"header"`
	ER       bool        `json:"e_r"`
}

// MusicianVipFurtherTask 进阶任务
type MusicianVipFurtherTask struct {
	Name             string               `json:"name"`
	TotalCompleteNum int                  `json:"totalCompleteNum"`
	ProgressRate     int                  `json:"progressRate"`
	MissionStatus    int                  `json:"missionStatus"`
	MissionCode      string               `json:"missionCode"`
	SortValue        int                  `json:"sortValue"`
	Desc             string               `json:"desc"`
	TaskProgressText string               `json:"taskProgressText"`
	Button           string               `json:"button"`
	IconUrl          string               `json:"iconUrl"`
	IosUrl           string               `json:"iosUrl"`
	AndroidUrl       string               `json:"androidUrl"`
	PcUrl            string               `json:"pcUrl"`
	Children         []MusicianVipSubTask `json:"children"`
}

// MusicianVipSubTask 子任务
type MusicianVipSubTask struct {
	Name             string               `json:"name"`
	TotalCompleteNum int                  `json:"totalCompleteNum"`
	ProgressRate     int                  `json:"progressRate"`
	MissionStatus    int                  `json:"missionStatus"`
	MissionCode      string               `json:"missionCode"`
	SortValue        int                  `json:"sortValue"`
	Desc             string               `json:"desc"`
	TaskProgressText string               `json:"taskProgressText"`
	Button           string               `json:"button"`
	IconUrl          string               `json:"iconUrl"`
	IosUrl           string               `json:"iosUrl"`
	AndroidUrl       string               `json:"androidUrl"`
	PcUrl            string               `json:"pcUrl"`
	Children         []MusicianVipSubTask `json:"children"`
}

// MusicianVipTasks 获取音乐人黑胶会员任务
// needLogin: 是
func (a *Api) MusicianVipTasks(ctx context.Context, req *MusicianVipTasksReq) (*MusicianVipTasksResp, error) {
	var (
		url   = "https://music.163.com/eapi/nmusician/workbench/special/right/vip/info"
		reply MusicianVipTasksResp
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

// MusicianRoleGetReq 获取音乐人身份请求。
type MusicianRoleGetReq struct {
	MusicianEAPIReq
}

// MusicianRoleGetResp 获取音乐人身份响应。
type MusicianRoleGetResp struct {
	types.RespCommon[MusicianRoleGetData]
}

// MusicianRoleGetData 音乐人身份数据。
type MusicianRoleGetData struct {
	UserId           int64   `json:"userId"`
	ArtistId         int64   `json:"artistId"`
	Identity         []int64 `json:"identity"`
	IsMusician       bool    `json:"isMusician"`
	AuthType         int64   `json:"authType"`
	IdentityCategory string  `json:"identityCategory"`
}

// MusicianRoleGet 获取音乐人身份。
func (a *Api) MusicianRoleGet(ctx context.Context, req *MusicianRoleGetReq) (*MusicianRoleGetResp, error) {
	if req == nil {
		req = &MusicianRoleGetReq{}
	}
	a.fillMusicianEAPIReq(&req.MusicianEAPIReq)

	var (
		url   = "https://interface3.music.163.com/eapi/nmusician/workbench/musician/role/get"
		reply MusicianRoleGetResp
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

// MusicianMissionListReq 获取音乐人任务列表请求。
type MusicianMissionListReq struct {
	MusicianEAPIReq
	Platform   int `json:"platform,omitempty"`
	Tag        int `json:"tag,omitempty"`
	ActionType int `json:"actionType,omitempty"`
}

// MusicianMissionListResp 获取音乐人任务列表响应。
type MusicianMissionListResp struct {
	types.RespCommon[MusicianMissionListData]
}

// MusicianMissionListData 音乐人任务列表数据。
type MusicianMissionListData struct {
	List []MusicianMissionTask `json:"list"`
}

// MusicianMissionTask 音乐人任务项。
type MusicianMissionTask struct {
	Business                     string `json:"business"`
	UserMissionId                int64  `json:"userMissionId"`
	MissionId                    int64  `json:"missionId"`
	UserId                       int64  `json:"userId"`
	MissionEntityId              int64  `json:"missionEntityId"`
	RewardId                     int64  `json:"rewardId"`
	ProgressRate                 int64  `json:"progressRate"`
	Name                         string `json:"name"`
	Description                  string `json:"description"`
	Type                         int64  `json:"type"`
	Tag                          int64  `json:"tag"`
	ActionType                   int64  `json:"actionType"`
	Platform                     int64  `json:"platform"`
	Status                       int64  `json:"status"`
	Button                       string `json:"button"`
	SortValue                    int64  `json:"sortValue"`
	StartTime                    int64  `json:"startTime"`
	EndTime                      int64  `json:"endTime"`
	ExtendInfo                   string `json:"extendInfo"`
	CreateTime                   int64  `json:"createTime"`
	UpdateTime                   int64  `json:"updateTime"`
	Period                       int64  `json:"period"`
	UserUnObtainRewardExpireTime int64  `json:"userUnObtainRewardExpireTime"`
	TargetCount                  int64  `json:"targetCount"`
	RewardWorth                  string `json:"rewardWorth"`
	RewardType                   int64  `json:"rewardType"`
	NeedToReceive                int64  `json:"needToReceive"`
	Title                        string `json:"title"`
}

// MusicianMissionCycleList 获取音乐人周期任务列表。
func (a *Api) MusicianMissionCycleList(ctx context.Context, req *MusicianMissionListReq) (*MusicianMissionListResp, error) {
	if req == nil {
		req = &MusicianMissionListReq{}
	}
	a.fillMusicianEAPIReq(&req.MusicianEAPIReq)

	var (
		url   = "https://interface3.music.163.com/eapi/nmusician/workbench/mission/cycle/list"
		reply MusicianMissionListResp
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

// MusicianMissionStageList 获取音乐人阶段任务列表。
func (a *Api) MusicianMissionStageList(ctx context.Context, req *MusicianMissionListReq) (*MusicianMissionListResp, error) {
	if req == nil {
		req = &MusicianMissionListReq{}
	}
	a.fillMusicianEAPIReq(&req.MusicianEAPIReq)

	var (
		url   = "https://interface3.music.163.com/eapi/nmusician/workbench/mission/stage/list"
		reply MusicianMissionListResp
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

// MusicianRewardObtainReq 音乐人云豆领奖请求。
type MusicianRewardObtainReq struct {
	MusicianEAPIReq
	UserMissionId int64 `json:"userMissionId"`
	Period        int64 `json:"period"`
}

// MusicianRewardObtainResp 音乐人云豆领奖响应。
type MusicianRewardObtainResp struct {
	types.RespCommon[any]
}

// MusicianRewardObtain 领取音乐人云豆奖励。
func (a *Api) MusicianRewardObtain(ctx context.Context, req *MusicianRewardObtainReq) (*MusicianRewardObtainResp, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if req.UserMissionId <= 0 {
		return nil, fmt.Errorf("userMissionId is required")
	}
	a.fillMusicianEAPIReq(&req.MusicianEAPIReq)

	var (
		url   = "https://interface3.music.163.com/eapi/nmusician/workbench/mission/reward/obtain/new"
		reply MusicianRewardObtainResp
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

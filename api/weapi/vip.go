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

package weapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type VipRewardGetReq struct {
	TaskIds []string
}

type vipRewardGetReq struct {
	TaskIds string `json:"taskIds"`
}

type VipRewardGetResp struct {
	types.RespCommon[any]
}

// VipRewardGet 领取vip成长值
// url:
// needLogin: 未知
func (a *Api) VipRewardGet(ctx context.Context, req *VipRewardGetReq) (*VipRewardGetResp, error) {
	if len(req.TaskIds) <= 0 {
		return nil, fmt.Errorf("taskIds is empty")
	}

	var (
		opts    = api.NewOptions()
		url     = "https://music.163.com/weapi/vipnewcenter/app/level/task/reward/get"
		reply   VipRewardGetResp
		request = vipRewardGetReq{
			TaskIds: strings.Join(req.TaskIds, ","),
		}
	)

	resp, err := a.client.Request(ctx, url, &request, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipRewardGetAllReq struct{}

type VipRewardGetAllResp struct {
	types.RespCommon[VipRewardGetAllRespData]
}

type VipRewardGetAllRespData struct {
	Result bool `json:"result"`
}

// VipRewardGetAll 领取vip所有成长值
// url:
// needLogin: 未知
func (a *Api) VipRewardGetAll(ctx context.Context, req *VipRewardGetAllReq) (*VipRewardGetAllResp, error) {
	var (
		url   = "https://music.163.com/weapi/vipnewcenter/app/level/task/reward/getall"
		reply VipRewardGetAllResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipTaskReq struct{}

type VipTaskResp struct {
	types.RespCommon[VipTaskRespData]
}

type VipTaskRespData struct {
	TaskList []struct {
		Seq       int64  `json:"seq"`
		SeqName   string `json:"seqName"`
		TaskItems []struct {
			Action          string      `json:"action"`
			ActionType      int         `json:"actionType"`
			BasicTaskId     int         `json:"basicTaskId"`
			BusinessIdent   interface{} `json:"businessIdent"`
			CurrentProgress int         `json:"currentProgress"`
			Description     string      `json:"description"`
			GrowthPoint     int         `json:"growthPoint"`
			IconUrl         string      `json:"iconUrl"`
			MissionId       int         `json:"missionId"`
			Name            string      `json:"name"`
			NeedReceive     bool        `json:"needReceive"`
			Period          int         `json:"period"`
			ProgressType    int         `json:"progressType"`
			RuleId          int         `json:"ruleId"`
			SeqTag          int         `json:"seqTag"`
			ShowProgress    bool        `json:"showProgress"`
			SortValue       int         `json:"sortValue"`
			Status          int         `json:"status"`
			TagId           int         `json:"tagId"`
			TargetWorth     int         `json:"targetWorth"`
			Targets         interface{} `json:"targets"`
			TaskId          string      `json:"taskId"`
			TaskTag         string      `json:"taskTag"`
			TotalUngetScore int         `json:"totalUngetScore"`
			Type            int         `json:"type"`
			TypeCode        interface{} `json:"typeCode"`
			UnGetIds        []string    `json:"unGetIds"`
			UpdateTime      int         `json:"updateTime"`
			Url             string      `json:"url"`
			UserMissionId   interface{} `json:"userMissionId"`
		} `json:"taskItems"`
		TaskType int `json:"taskType"`
	} `json:"taskList"`
	TaskScore int `json:"taskScore"`
}

// VipTask vip任务列表 todo:该任务列表应该是旧接口貌似
// url:
// needLogin: 未知
func (a *Api) VipTask(ctx context.Context, req *VipTaskReq) (*VipTaskResp, error) {
	var (
		url   = "https://music.163.com/weapi/vipnewcenter/app/level/task/list"
		reply VipTaskResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipTaskV2Req struct{}

type VipTaskV2Resp struct {
	types.RespCommon[VipTaskV2RespData]
}

type VipTaskV2RespData struct {
	MaxScore      int `json:"maxScore"`
	ScoreDuration int `json:"scoreDuration"`
	TaskScore     int `json:"taskScore"`
	UnGetAllScore int `json:"unGetAllScore"`
	TaskList      []struct {
		Seq       int    `json:"seq"`
		SeqName   string `json:"seqName"`
		TaskItems []struct {
			CurrentInfo struct {
				Action          string      `json:"action"`
				ActionType      int         `json:"actionType"`
				BasicTaskId     int         `json:"basicTaskId"`
				BusinessIdent   interface{} `json:"businessIdent"`
				CurrentProgress int         `json:"currentProgress"`
				Description     string      `json:"description"`
				GrowthPoint     int         `json:"growthPoint"`
				IconUrl         string      `json:"iconUrl"`
				MissionId       int         `json:"missionId"`
				Name            string      `json:"name"`
				NeedReceive     bool        `json:"needReceive"`
				Period          int         `json:"period"`
				ProgressType    int         `json:"progressType"`
				RuleId          int         `json:"ruleId"`
				SeqTag          int         `json:"seqTag"`
				ShowProgress    bool        `json:"showProgress"`
				SortValue       int         `json:"sortValue"`
				Status          int         `json:"status"`
				TagId           int         `json:"tagId"`
				TargetWorth     int         `json:"targetWorth"`
				TaskId          string      `json:"taskId"`
				TaskTag         string      `json:"taskTag"`
				TotalUngetScore int         `json:"totalUngetScore"`
				Type            int         `json:"type"`
				TypeCode        interface{} `json:"typeCode"`
				UnGetIds        interface{} `json:"unGetIds"`
				UpdateTime      int64       `json:"updateTime"`
				Url             string      `json:"url"`
				UserMissionId   *int64      `json:"userMissionId"`
			} `json:"currentInfo"`
			SubList interface{} `json:"subList"`
		} `json:"taskItems"`
		TaskType int `json:"taskType"`
	} `json:"taskList"`
}

// VipTaskV2 vip任务列表V2
// url:
// needLogin: 未知
func (a *Api) VipTaskV2(ctx context.Context, req *VipTaskV2Req) (*VipTaskV2Resp, error) {
	var (
		url   = "https://music.163.com/weapi/vipnewcenter/app/level/task/newlist"
		reply VipTaskV2Resp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipInfoReq struct {
	UserId string `json:"userId"` // 为空默认为当前登录用户
}

type VipInfoResp struct {
	types.RespCommon[VipInfoRespData]
}

type VipInfoRespData struct {
	Associator struct {
		DynamicIconUrl  string `json:"dynamicIconUrl"`
		ExpireTime      int64  `json:"expireTime"`
		IconUrl         string `json:"iconUrl"`
		IsSign          bool   `json:"isSign"`
		IsSignDeduct    bool   `json:"isSignDeduct"`
		IsSignIap       bool   `json:"isSignIap"`
		IsSignIapDeduct bool   `json:"isSignIapDeduct"`
		VipCode         int    `json:"vipCode"`
		VipLevel        int    `json:"vipLevel"`
	} `json:"associator"`
	MusicPackage struct {
		DynamicIconUrl  string `json:"dynamicIconUrl"`
		ExpireTime      int64  `json:"expireTime"`
		IconUrl         string `json:"iconUrl"`
		IsSign          bool   `json:"isSign"`
		IsSignDeduct    bool   `json:"isSignDeduct"`
		IsSignIap       bool   `json:"isSignIap"`
		IsSignIapDeduct bool   `json:"isSignIapDeduct"`
		VipCode         int    `json:"vipCode"`
		VipLevel        int    `json:"vipLevel"`
	} `json:"musicPackage"`
	RedVipAnnualCount     int         `json:"redVipAnnualCount"`
	RedVipDynamicIconUrl  interface{} `json:"redVipDynamicIconUrl"`
	RedVipDynamicIconUrl2 interface{} `json:"redVipDynamicIconUrl2"`
	RedVipLevel           int         `json:"redVipLevel"`
	RedVipLevelIcon       string      `json:"redVipLevelIcon"`
	Redplus               struct {
		DynamicIconUrl  string `json:"dynamicIconUrl"`
		ExpireTime      int64  `json:"expireTime"`
		IconUrl         string `json:"iconUrl"`
		IsSign          bool   `json:"isSign"`
		IsSignDeduct    bool   `json:"isSignDeduct"`
		IsSignIap       bool   `json:"isSignIap"`
		IsSignIapDeduct bool   `json:"isSignIapDeduct"`
		VipCode         int    `json:"vipCode"`
		VipLevel        int    `json:"vipLevel"`
	} `json:"redplus"`
}

// VipInfo vip信息
// url:
// needLogin: 未知
func (a *Api) VipInfo(ctx context.Context, req *VipInfoReq) (*VipInfoResp, error) {
	var (
		url   = "https://music.163.com/weapi/music-vip-membership/front/vip/info"
		reply VipInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type VipClientInfoReq struct {
	UserId string `json:"userId"` // 为空默认为当前登录用户
}

type VipClientInfoResp struct {
	types.RespCommon[VipClientInfoRespData]
}

type VipClientInfoRespData struct {
	AlbumVip struct {
		DynamicIconUrl  interface{} `json:"dynamicIconUrl"`
		ExpireTime      int64       `json:"expireTime"`
		IconUrl         interface{} `json:"iconUrl"`
		IsSign          bool        `json:"isSign"`
		IsSignDeduct    bool        `json:"isSignDeduct"`
		IsSignIap       bool        `json:"isSignIap"`
		IsSignIapDeduct bool        `json:"isSignIapDeduct"`
		VipCode         int         `json:"vipCode"`
		VipLevel        int         `json:"vipLevel"`
	} `json:"albumVip"`
	Associator struct {
		DynamicIconUrl  interface{} `json:"dynamicIconUrl"`
		ExpireTime      int64       `json:"expireTime"`
		IconUrl         interface{} `json:"iconUrl"`
		IsSign          bool        `json:"isSign"`
		IsSignDeduct    bool        `json:"isSignDeduct"`
		IsSignIap       bool        `json:"isSignIap"`
		IsSignIapDeduct bool        `json:"isSignIapDeduct"`
		VipCode         int         `json:"vipCode"`
		VipLevel        int         `json:"vipLevel"`
	} `json:"associator"`
	MusicPackage struct {
		DynamicIconUrl  interface{} `json:"dynamicIconUrl"`
		ExpireTime      int64       `json:"expireTime"`
		IconUrl         interface{} `json:"iconUrl"`
		IsSign          bool        `json:"isSign"`
		IsSignDeduct    bool        `json:"isSignDeduct"`
		IsSignIap       bool        `json:"isSignIap"`
		IsSignIapDeduct bool        `json:"isSignIapDeduct"`
		VipCode         int         `json:"vipCode"`
		VipLevel        int         `json:"vipLevel"`
	} `json:"musicPackage"`
	Now               int64 `json:"now"`
	OldCacheProtocol  bool  `json:"oldCacheProtocol"`
	RedVipAnnualCount int   `json:"redVipAnnualCount"`
	RedVipLevel       int   `json:"redVipLevel"`
	Redplus           struct {
		DynamicIconUrl  interface{} `json:"dynamicIconUrl"`
		ExpireTime      int64       `json:"expireTime"`
		IconUrl         interface{} `json:"iconUrl"`
		IsSign          bool        `json:"isSign"`
		IsSignDeduct    bool        `json:"isSignDeduct"`
		IsSignIap       bool        `json:"isSignIap"`
		IsSignIapDeduct bool        `json:"isSignIapDeduct"`
		VipCode         int         `json:"vipCode"`
		VipLevel        int         `json:"vipLevel"`
	} `json:"redplus"`
	RelationOtherUserId               int `json:"relationOtherUserId"`
	RelationOtherUserRedVipExpireTime int `json:"relationOtherUserRedVipExpireTime"`
	RelationType                      int `json:"relationType"`
	UserId                            int `json:"userId"`
	VoiceBookVip                      struct {
		DynamicIconUrl  interface{} `json:"dynamicIconUrl"`
		ExpireTime      int64       `json:"expireTime"`
		IconUrl         interface{} `json:"iconUrl"`
		IsSign          bool        `json:"isSign"`
		IsSignDeduct    bool        `json:"isSignDeduct"`
		IsSignIap       bool        `json:"isSignIap"`
		IsSignIapDeduct bool        `json:"isSignIapDeduct"`
		VipCode         int         `json:"vipCode"`
		VipLevel        int         `json:"vipLevel"`
	} `json:"voiceBookVip"`
}

// VipClientInfo vip信息
// url:
// needLogin: 未知
func (a *Api) VipClientInfo(ctx context.Context, req *VipClientInfoReq) (*VipClientInfoResp, error) {
	var (
		url   = "https://music.163.com/api/music-vip-membership/client/vip/info"
		reply VipClientInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// MIT License
//
// Copyright (c) 2026 chaunsin
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

// FansGroup (乐迷团) API — 乐迷团任务相关接口

package eapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
)

// FansGroupDetailGetReq 获取乐迷团详情请求
type FansGroupDetailGetReq struct {
	GroupId string `json:"groupId"` // 乐迷团ID
	Scene   string `json:"scene"`   // 场景, 可留空
	Header  string `json:"header"`  // 固定 "{}"
	ER      bool   `json:"e_r"`     // 固定 true
}

// FansGroupDetailGetResp 获取乐迷团详情响应
type FansGroupDetailGetResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		FansGroupInfo struct {
			FansGroupId       string `json:"fansGroupId"`
			FansGroupName     string `json:"fansGroupName"`
			FansGroupPureName string `json:"fansGroupPureName"`
			HeadId            int64  `json:"headId"`        // 歌手/头像ID
			ArtistName        string `json:"artistName"`    // 歌手名
			BoardId           string `json:"boardId"`       // 看板ID = activityInfoList 中的 id
			TopicId           string `json:"topicId"`       // 话题ID
			HeadAvatarUrl     string `json:"headAvatarUrl"` // 头像URL
			Musician          bool   `json:"musician"`      // 是否音乐人
		} `json:"fansGroupInfo"`
	} `json:"data"`
}

// FansGroupDetailGet 获取乐迷团详情 (含 boardId 等关键信息)
func (a *Api) FansGroupDetailGet(ctx context.Context, req *FansGroupDetailGetReq) (*FansGroupDetailGetResp, error) {
	if req.Header == "" {
		req.Header = "{}"
	}
	req.ER = true

	queryParams := fmt.Sprintf("groupId=%s", req.GroupId)
	if req.Scene != "" {
		queryParams += fmt.Sprintf("&scene=%s", req.Scene)
	}

	var (
		url   = fmt.Sprintf("https://interface3.music.163.com/eapi/social/fansgroup/bff/detail/get?%s", queryParams)
		reply FansGroupDetailGetResp
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

// FansGroupMissionAllReq 获取乐迷团全部任务列表请求
type FansGroupMissionAllReq struct {
	FansGroupId string `json:"fansGroupId"` // 乐迷团ID
	Header      string `json:"header"`      // 固定 "{}"
	ER          bool   `json:"e_r"`         // 固定 true
}

// FansGroupMissionItem 单个乐迷团任务
type FansGroupMissionItem struct {
	MissionId       int64  `json:"missionId"`
	MissionType     string `json:"missionType"`     // "normal" / "userSurprise"
	Title           string `json:"title"`           // 任务标题: 播放歌曲/发布图文笔记/分享歌曲/点赞乐迷笔记
	Status          string `json:"status"`          // "INIT"=未开始 "PROCESSING"=进行中 "COMPLETED"=已完成
	CurrentProgress int    `json:"currentProgress"` // 当前进度
	AllProgress     int    `json:"allProgress"`     // 总进度
	Integral        string `json:"integral"`        // 奖励积分
	Order           int    `json:"order"`           // 排序
	LogInfo         string `json:"logInfo"`         // 日志信息JSON
	IconUi          struct {
		IconUrl   string `json:"iconUrl"`
		TargetUrl string `json:"targetUrl"` // 包含任务参数的JSON
	} `json:"iconUi"`
	Button struct {
		Copywriter string `json:"copywriter"` // 按钮文案
		Url        string `json:"url"`        // 包含任务参数的JSON (与 TargetUrl 结构相同)
	} `json:"button"`
}

// FansGroupMissionOriginality 今日加速任务 (随机任务)
type FansGroupMissionOriginality struct {
	MissionId       int64       `json:"missionId"`
	MissionType     string      `json:"missionType"` // "userSurprise"
	Title           string      `json:"title"`       // "今日加速任务"
	Status          string      `json:"status"`
	CurrentProgress int         `json:"currentProgress"`
	AllProgress     int         `json:"allProgress"`
	Integral        string      `json:"integral"`
	Subtitle        string      `json:"subtitle"`
	LogInfo         string      `json:"logInfo"`
	MissionDetail   interface{} `json:"missionDetail"`
	Button          struct {
		Copywriter string `json:"copywriter"`
		Url        string `json:"url"`
	} `json:"button"`
}

// FansGroupMissionAllResp 获取乐迷团全部任务列表响应
type FansGroupMissionAllResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Normal struct {
			Success bool                   `json:"success"`
			Data    []FansGroupMissionItem `json:"data"`
		} `json:"normal"`
		Originality struct {
			Success bool                        `json:"success"`
			Data    FansGroupMissionOriginality `json:"data"`
		} `json:"originality"`
		RemainingIntegral int `json:"remainingIntegral"`
		DailyMaxIntimacy  int `json:"dailyMaxIntimacy"`
	} `json:"data"`
}

// FansGroupMissionAll 获取乐迷团全部任务列表
func (a *Api) FansGroupMissionAll(ctx context.Context, req *FansGroupMissionAllReq) (*FansGroupMissionAllResp, error) {
	if req.Header == "" {
		req.Header = "{}"
	}
	req.ER = true

	var (
		url   = fmt.Sprintf("https://interface3.music.163.com/eapi/fans/group/mission/all?fansGroupId=%s", req.FansGroupId)
		reply FansGroupMissionAllResp
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

// FansGroupFeedRecommendReq 获取乐迷团推荐Feed请求
type FansGroupFeedRecommendReq struct {
	ArtistSelf  string `json:"artistSelf"`  // 固定 "0"
	Cursor      string `json:"cursor"`      // 游标, 首次 "0"
	FansGroupId string `json:"fansGroupId"` // 乐迷团ID
	Size        string `json:"size"`        // 数量, 默认 "10"
	Header      string `json:"header"`      // 固定 "{}"
	ER          bool   `json:"e_r"`         // 固定 true
}

// FansGroupFeedRecommendResp 获取乐迷团推荐Feed响应
type FansGroupFeedRecommendResp struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"` // 复杂结构, 按需解析
}

// FansGroupFeedRecommend 获取乐迷团推荐Feed
func (a *Api) FansGroupFeedRecommend(ctx context.Context, req *FansGroupFeedRecommendReq) (*FansGroupFeedRecommendResp, error) {
	if req.ArtistSelf == "" {
		req.ArtistSelf = "0"
	}
	if req.Cursor == "" {
		req.Cursor = "0"
	}
	if req.Size == "" {
		req.Size = "10"
	}
	if req.Header == "" {
		req.Header = "{}"
	}
	req.ER = true

	var (
		url   = fmt.Sprintf("https://interface3.music.163.com/eapi/fans/group/feed/recommend/get?artistSelf=%s&cursor=%s&fansGroupId=%s&size=%s", req.ArtistSelf, req.Cursor, req.FansGroupId, req.Size)
		reply FansGroupFeedRecommendResp
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

// FansGroupMissionForwardProgressReq 分享进度上报请求
type FansGroupMissionForwardProgressReq struct {
	ResourceId   string `json:"resourceId"`   // 歌曲ID (从任务列表的 button.url 中解析)
	Action       string `json:"action"`       // 固定 "share"
	FansGroupId  string `json:"fansGroupId"`  // 固定 "null" (HAR中观察到的值)
	ResourceType string `json:"resourceType"` // 固定 "4" (歌曲类型)
	Header       string `json:"header"`       // 固定 "{}"
	ER           bool   `json:"e_r"`          // 固定 true
}

// FansGroupMissionForwardProgressResp 分享进度上报响应
type FansGroupMissionForwardProgressResp struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// FansGroupMissionForwardProgress 分享进度上报
func (a *Api) FansGroupMissionForwardProgress(ctx context.Context, req *FansGroupMissionForwardProgressReq) (*FansGroupMissionForwardProgressResp, error) {
	if req.Action == "" {
		req.Action = "share"
	}
	if req.FansGroupId == "" {
		req.FansGroupId = "null"
	}
	if req.ResourceType == "" {
		req.ResourceType = "4"
	}
	if req.Header == "" {
		req.Header = "{}"
	}
	req.ER = true

	var (
		url   = fmt.Sprintf("https://interface3.music.163.com/eapi/fans/group/mission/forward/progress?resourceId=%s&action=%s&fansGroupId=%s&resourceType=%s", req.ResourceId, req.Action, req.FansGroupId, req.ResourceType)
		reply FansGroupMissionForwardProgressResp
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

// ResourceLikeReq 点赞资源请求
type ResourceLikeReq struct {
	ThreadId  string `json:"threadId"`  // 动态的ThreadId, 格式如: A_EV_2_{eventId}_{userId}
	AppLogExt string `json:"appLogExt"` // 日志扩展字段, 包含乐迷团归属信息
	Header    string `json:"header"`    // 固定 "{}"
	ER        bool   `json:"e_r"`       // 固定 true
}

// ResourceLikeResp 点赞资源响应
type ResourceLikeResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ResourceLike 点赞资源 (用于点赞乐迷团笔记)
func (a *Api) ResourceLike(ctx context.Context, req *ResourceLikeReq) (*ResourceLikeResp, error) {
	if req.Header == "" {
		req.Header = "{}"
	}
	req.ER = true

	var (
		url   = "https://interface3.music.163.com/eapi/resource/like"
		reply ResourceLikeResp
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

// FansGroupUserGroupDetailGetReq 获取用户在乐迷团的详情请求
type FansGroupUserGroupDetailGetReq struct {
	GroupId string `json:"groupId"` // 乐迷团ID
	Header  string `json:"header"`  // 固定 "{}"
	ER      bool   `json:"e_r"`     // 固定 true
}

// FansGroupUserGroupDetailGetResp 获取用户在乐迷团的详情响应
type FansGroupUserGroupDetailGetResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		FansGroupMemberDetail struct {
			UserId      int64  `json:"userId"`
			Nickname    string `json:"nickname"`
			Joined      bool   `json:"joined"`
			FansGroupId string `json:"fansGroupId"`
			Level       struct {
				Level       string `json:"level"`
				FanTitle    string `json:"fanTitle"`
				Segment     string `json:"segment"`
				SegmentCode string `json:"segmentCode"`
			} `json:"level"`
		} `json:"fansGroupMemberDetail"`
	} `json:"data"`
}

// FansGroupUserGroupDetailGet 获取用户在乐迷团的详情
func (a *Api) FansGroupUserGroupDetailGet(ctx context.Context, req *FansGroupUserGroupDetailGetReq) (*FansGroupUserGroupDetailGetResp, error) {
	if req.Header == "" {
		req.Header = "{}"
	}
	req.ER = true

	var (
		url   = fmt.Sprintf("https://interface3.music.163.com/eapi/social/fansgroup/bff/user/group/detail/get?groupId=%s", req.GroupId)
		reply FansGroupUserGroupDetailGetResp
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

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

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type PartnerWeekReq struct {
	types.ReqCommon
	Period string `json:"period"` // 格式:MMD-1617552000000-37-1
}

type PartnerWeekResp struct {
	types.RespCommon[PartnerWeekData]
}

type PartnerWeekData struct {
	Period        int64       `json:"period"`
	Week          int64       `json:"week"`
	Periods       interface{} `json:"periods"`
	SectionPeriod string      `json:"sectionPeriod"`
	User          struct {
		UserId    int64  `json:"userId"`
		NickName  string `json:"nickName"`
		AvatarUrl string `json:"avatarUrl"`
	} `json:"user"`
	PickRight struct {
		Status    interface{} `json:"status"`
		ValidTime int64       `json:"validTime"`
		ValidDay  int64       `json:"validDay"`
	} `json:"pickRight"`
	Title      interface{} `json:"title"`
	Integral   int64       `json:"integral"`
	Evaluation struct {
		EvaluateCount    int64  `json:"evaluateCount"`
		BasicIntegral    int64  `json:"basicIntegral"`
		AccuracyIntegral int64  `json:"accuracyIntegral"`
		AccurateCount    int64  `json:"accurateCount"`
		AccurateRate     int64  `json:"accurateRate"`
		AccuracyLevel    string `json:"accuracyLevel"`
	} `json:"evaluation"`
	Top3 []struct {
		Work struct {
			Id                  int64       `json:"id"`
			ResourceType        string      `json:"resourceType"`
			ResourceId          int64       `json:"resourceId"`
			Name                string      `json:"name"`
			CoverUrl            string      `json:"coverUrl"`
			AuthorName          string      `json:"authorName"`
			Duration            int64       `json:"duration"`
			Source              string      `json:"source"`
			Status              string      `json:"status"`
			BackendForceOffline bool        `json:"backendForceOffline"`
			WorkResourceInfo    interface{} `json:"workResourceInfo"`
		} `json:"work"`
		Score            float64 `json:"score"`
		AvgScore         float64 `json:"avgScore"`
		BasicIntegral    int64   `json:"basicIntegral"`
		AccuracyIntegral int64   `json:"accuracyIntegral"`
		EvaluateCount    int64   `json:"evaluateCount"`
		Tags             []struct {
			Tag   string `json:"tag"`
			Count int64  `json:"count"`
		} `json:"tags"`
		ScoreStats struct {
			Field1 int64 `json:"4.0"`
			Field2 int64 `json:"1.0,omitempty"`
			Field3 int64 `json:"2.0"`
			Field4 int64 `json:"5.0"`
			Field5 int64 `json:"3.0"`
		} `json:"scoreStats"`
		ScorePercentMap struct {
			Field1 float64 `json:"1.0,omitempty"`
			Field2 float64 `json:"4.0"`
			Field3 float64 `json:"2.0"`
			Field4 float64 `json:"5.0"`
			Field5 float64 `json:"3.0"`
		} `json:"scorePercentMap"`
		Accuracy float64 `json:"accuracy"`
	} `json:"top3"`
	AccurateWorks []struct {
		Work struct {
			Id                  int64       `json:"id"`
			ResourceType        string      `json:"resourceType"`
			ResourceId          int64       `json:"resourceId"`
			Name                string      `json:"name"`
			CoverUrl            string      `json:"coverUrl"`
			AuthorName          string      `json:"authorName"`
			Duration            int64       `json:"duration"`
			Source              string      `json:"source"`
			Status              string      `json:"status"`
			BackendForceOffline bool        `json:"backendForceOffline"`
			WorkResourceInfo    interface{} `json:"workResourceInfo"`
		} `json:"work"`
		Score            float64     `json:"score"`
		AvgScore         float64     `json:"avgScore"`
		BasicIntegral    int64       `json:"basicIntegral"`
		AccuracyIntegral int64       `json:"accuracyIntegral"`
		EvaluateCount    int64       `json:"evaluateCount"`
		Tags             interface{} `json:"tags"`
		ScoreStats       interface{} `json:"scoreStats"`
		ScorePercentMap  interface{} `json:"scorePercentMap"`
		Accuracy         float64     `json:"accuracy"`
	} `json:"accurateWorks"`
	ExcellentWorks     []interface{} `json:"excellentWorks"`
	RecoverStatus      bool          `json:"recoverStatus"`
	RecoverExpiredTime int64         `json:"recoverExpiredTime"`
	ExcellentPlaylists []struct {
		Id    int64  `json:"id"`
		Name  string `json:"name"`
		Cover string `json:"cover"`
	} `json:"excellentPlaylists"`
	Status            string      `json:"status"`
	ResultConfigTitle interface{} `json:"resultConfigTitle"`
	ConfigedAct       bool        `json:"configedAct"`
	Eliminated        bool        `json:"eliminated"`
}

// PartnerWeek 查询当前周期周一数据报告情况
func (a *Api) PartnerWeek(ctx context.Context, req *PartnerWeekReq) (*PartnerWeekResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/week/result/get"
		reply PartnerWeekResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerPeriodReq struct {
	types.ReqCommon
}

type PartnerPeriodResp struct {
	types.RespCommon[PartnerPeriodRespData]
}

type PartnerPeriodRespData struct {
	Period        int64       `json:"period"`
	Week          int64       `json:"week"`
	Periods       string      `json:"periods"`
	SectionPeriod interface{} `json:"sectionPeriod"`
	User          struct {
		UserId    int64  `json:"userId"`
		NickName  string `json:"nickName"`
		AvatarUrl string `json:"avatarUrl"`
	} `json:"user"`
	PickRight  interface{} `json:"pickRight"`
	Title      string      `json:"title"`
	Integral   int64       `json:"integral"`
	Evaluation struct {
		EvaluateCount    int64  `json:"evaluateCount"`
		BasicIntegral    int64  `json:"basicIntegral"`
		AccuracyIntegral int64  `json:"accuracyIntegral"`
		AccurateCount    int64  `json:"accurateCount"`
		AccurateRate     int64  `json:"accurateRate"`
		AccuracyLevel    string `json:"accuracyLevel"`
	} `json:"evaluation"`
	Top3 []struct {
		Work struct {
			Id                  int64       `json:"id"`
			ResourceType        string      `json:"resourceType"`
			ResourceId          int64       `json:"resourceId"`
			Name                string      `json:"name"`
			CoverUrl            string      `json:"coverUrl"`
			AuthorName          string      `json:"authorName"`
			Duration            int64       `json:"duration"`
			Source              string      `json:"source"`
			Status              string      `json:"status"`
			BackendForceOffline bool        `json:"backendForceOffline"`
			WorkResourceInfo    interface{} `json:"workResourceInfo"`
		} `json:"work"`
		Score            float64 `json:"score"`
		AvgScore         float64 `json:"avgScore"`
		BasicIntegral    int64   `json:"basicIntegral"`
		AccuracyIntegral int64   `json:"accuracyIntegral"`
		EvaluateCount    int64   `json:"evaluateCount"`
		Tags             []struct {
			Tag   string `json:"tag"`
			Count int64  `json:"count"`
		} `json:"tags"`
		ScoreStats struct {
			Field1 int64 `json:"2.0"`
			Field2 int64 `json:"4.0"`
			Field3 int64 `json:"5.0"`
			Field4 int64 `json:"3.0"`
			Field5 int64 `json:"1.0,omitempty"`
		} `json:"scoreStats"`
		ScorePercentMap struct {
			Field1 float64 `json:"4.0"`
			Field2 float64 `json:"2.0"`
			Field3 float64 `json:"5.0"`
			Field4 float64 `json:"3.0"`
			Field5 float64 `json:"1.0,omitempty"`
		} `json:"scorePercentMap"`
		Accuracy float64 `json:"accuracy"`
	} `json:"top3"`
	AccurateWorks []struct {
		Work struct {
			Id                  int64       `json:"id"`
			ResourceType        string      `json:"resourceType"`
			ResourceId          int64       `json:"resourceId"`
			Name                string      `json:"name"`
			CoverUrl            string      `json:"coverUrl"`
			AuthorName          string      `json:"authorName"`
			Duration            int64       `json:"duration"`
			Source              string      `json:"source"`
			Status              string      `json:"status"`
			BackendForceOffline bool        `json:"backendForceOffline"`
			WorkResourceInfo    interface{} `json:"workResourceInfo"`
		} `json:"work"`
		Score            float64     `json:"score"`
		AvgScore         float64     `json:"avgScore"`
		BasicIntegral    int64       `json:"basicIntegral"`
		AccuracyIntegral int64       `json:"accuracyIntegral"`
		EvaluateCount    int64       `json:"evaluateCount"`
		Tags             interface{} `json:"tags"`
		ScoreStats       interface{} `json:"scoreStats"`
		ScorePercentMap  interface{} `json:"scorePercentMap"`
		Accuracy         float64     `json:"accuracy"`
	} `json:"accurateWorks"`
	ExcellentWorks     []interface{} `json:"excellentWorks"`
	RecoverStatus      bool          `json:"recoverStatus"`
	RecoverExpiredTime int64         `json:"recoverExpiredTime"`
	ExcellentPlaylists []struct {
		Id    int64  `json:"id"`
		Name  string `json:"name"`
		Cover string `json:"cover"`
	} `json:"excellentPlaylists"`
	// Status 状态 SETTLED: 可能是代表本期已经结算或者未满足320分失去测评资格了
	Status            string      `json:"status"`
	ResultConfigTitle interface{} `json:"resultConfigTitle"`
	ConfigedAct       interface{} `json:"configedAct"`
	// Eliminated 状态: true 可能是代表未满足320分失去测评资格了,很大概率是，它和Status状态二者必占其一
	Eliminated bool `json:"eliminated"`
}

// PartnerPeriod 查询当前周期数据报告情况
func (a *Api) PartnerPeriod(ctx context.Context, req *PartnerPeriodReq) (*PartnerPeriodResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/period/result/get"
		reply PartnerPeriodResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerUserinfoReq struct {
	types.ReqCommon
}

// PartnerUserinfoResp
// code:703 非音乐合伙人
type PartnerUserinfoResp struct {
	types.RespCommon[PartnerUserinfoRespData]
}

type PartnerUserinfoRespData struct {
	UserId        int64  `json:"userId"`
	NickName      string `json:"nickName"`
	AvatarUrl     string `json:"avatarUrl"`
	Number        int64  `json:"number"`
	Title         string `json:"title"`
	Days          int64  `json:"days"`
	Integral      int64  `json:"integral"`
	EvaluateCount int64  `json:"evaluateCount"`
	PickCount     int64  `json:"pickCount"`
	// Status 状态: ELIMINATED: 可能是代表本期已经结算或者未满足320分失去测评资格了
	Status     string        `json:"status"`
	PickRights []interface{} `json:"pickRights"`
	TitleStats []struct {
		Title string `json:"title"`
		Count int64  `json:"count"`
	} `json:"titleStats"`
	CurrentPeriodRank  interface{} `json:"currentPeriodRank"`
	RecoverExpiredTime int64       `json:"recoverExpiredTime"`
	RightType          int64       `json:"rightType"`
}

// PartnerUserinfo 查询当前用户数据
func (a *Api) PartnerUserinfo(ctx context.Context, req *PartnerUserinfoReq) (*PartnerUserinfoResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/user/info/get"
		reply PartnerUserinfoResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerLatestReq struct {
	types.ReqCommon
}

type PartnerLatestResp struct {
	types.RespCommon[PartnerLatestRespData]
}

type PartnerLatestRespData struct {
	SectionPeriod       string `json:"sectionPeriod"`
	Periods             string `json:"periods"`
	NextPeriodStartTime int64  `json:"nextPeriodStartTime"`
}

// PartnerLatest 查询下个周期开始时间
func (a *Api) PartnerLatest(ctx context.Context, req *PartnerLatestReq) (*PartnerLatestResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/latest/settle/period/get"
		reply PartnerLatestResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerHomeReq struct {
	types.ReqCommon
}

type PartnerHomeResp struct {
	types.RespCommon[PartnerHomeRespData]
}

type PartnerHomeRespData struct {
	Period    int64 `json:"period"`
	Week      int64 `json:"week"`
	StartDate int64 `json:"startDate"`
	EndDate   int64 `json:"endDate"`
	User      struct {
		UserId    int64  `json:"userId"`
		NickName  string `json:"nickName"`
		AvatarUrl string `json:"avatarUrl"`
		Title     string `json:"title"`
		Days      int64  `json:"days"`
		Number    int64  `json:"number"`
	} `json:"user"`
	Integral struct {
		Integral            int64 `json:"integral"`
		CurrentWeekIntegral int64 `json:"currentWeekIntegral"`
	} `json:"integral"`
	Title   interface{} `json:"title"`
	Banner  interface{} `json:"banner"`
	BtnDesc interface{} `json:"btnDesc"`
}

// PartnerHome 查询本周完成任务情况
func (a *Api) PartnerHome(ctx context.Context, req *PartnerHomeReq) (*PartnerHomeResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/home/get"
		reply PartnerHomeResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerTaskReq struct {
	types.ReqCommon
}

type PartnerTaskResp struct {
	types.RespCommon[PartnerTaskRespData]
}

type PartnerWork struct {
	Id                int64  `json:"id"`
	ResourceType      string `json:"resourceType"` // 资源类型 SONG
	ResourceId        int64  `json:"resourceId"`   // 歌曲id
	Name              string `json:"name"`
	CoverUrl          string `json:"coverUrl"`
	AuthorName        string `json:"authorName"`
	LyricType         int64  `json:"lyricType"`
	LyricContent      string `json:"lyricContent"`
	Duration          int64  `json:"duration"`
	SongStartPosition int64  `json:"songStartPosition"`
	SongEndPosition   int64  `json:"songEndPosition"`
	Status            string `json:"status"` // NORMAL
	PlayUrl           string `json:"playUrl"`
	Source            string `json:"source"` // RANK_INSERT,MUSE,等
	GoodRate          int64  `json:"goodRate"`
	Style             string `json:"style"` // 华语、华语嘻哈说唱、等
}

type PartnerTaskRespData struct {
	Id int64 `json:"id"`
	// 任务数量
	Count int64 `json:"count"`
	// 完成数量
	CompletedCount int64 `json:"completedCount"`
	// 完成任务获得的积分老版为10现在3.0版本为8分
	Integral  int64       `json:"integral"`
	TaskTitle interface{} `json:"taskTitle"`
	// Works 如果没有测评资格则该任务列表为空
	Works []struct {
		Work            PartnerWork `json:"work"`
		Completed       bool        `json:"completed"`
		Score           float64     `json:"score"`
		UserScore       float64     `json:"userScore"`
		Tags            interface{} `json:"tags"`
		CustomTags      interface{} `json:"customTags"`
		Comment         interface{} `json:"comment"`
		TaskTitleDesc   interface{} `json:"taskTitleDesc"`
		SongCommentInfo struct {
			CommentId int64  `json:"commentId"`
			ThreadId  string `json:"threadId"`
		} `json:"songCommentInfo"`
	} `json:"works"`
	// 推荐歌曲列表该列表为新得音乐合伙人3.0功能中增加
	RecResources []struct {
		Work           PartnerWork `json:"work"`
		SpecialTag     interface{} `json:"specialTag"`
		SongCommonTags interface{} `json:"songCommonTags"`
		ReceivedScore  int         `json:"receivedScore"`
		QualityScore   int         `json:"qualityScore"`
		RedHeartSong   bool        `json:"redHeartSong"`
		Listened       bool        `json:"listened"`
		CanInteract    bool        `json:"canInteract"`
		PublishComment bool        `json:"publishComment"`
		PublishEvent   bool        `json:"publishEvent"`
		CollectList    bool        `json:"collectList"`
		TotalTaskNum   int         `json:"totalTaskNum"`
		FinishTaskNum  int         `json:"finishTaskNum"`
	} `json:"recResources"`
	PageTaskType int64 `json:"pageTaskType"`
	CurRcmdScore int64 `json:"curRcmdScore"`
	CanInteract  bool  `json:"canInteract"`
	Completed    bool  `json:"completed"`
}

// PartnerDailyTask 查询当日任务情况
func (a *Api) PartnerDailyTask(ctx context.Context, req *PartnerTaskReq) (*PartnerTaskResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/daily/task/get"
		reply PartnerTaskResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerPickRightReq struct {
	types.ReqCommon
}

type PartnerPickRightResp struct {
	types.RespCommon[[]PartnerPickRightRespData]
}

// PartnerPickRightRespData TODO:待补充参数
type PartnerPickRightRespData struct {
}

// PartnerPickRight todo:正确数量？
func (a *Api) PartnerPickRight(ctx context.Context, req *PartnerPickRightReq) (*PartnerPickRightResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/song/pick/right/get"
		reply PartnerPickRightResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerNoticeReq struct {
	types.ReqCommon
}

type PartnerNoticeResp struct {
	types.RespCommon[bool]
}

// PartnerNotice 是否开启通知？
func (a *Api) PartnerNotice(ctx context.Context, req *PartnerNoticeReq) (*PartnerNoticeResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/daily/notice/switch/get"
		reply PartnerNoticeResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// PartnerTags 音乐合伙人测评默认标签
type PartnerTags string

const (
	OneAOnePartnerTags   PartnerTags = "1-A-1" // 歌词立意差
	OneBOnePartnerTags   PartnerTags = "1-B-1" // 旋律无记忆
	OneCOnePartnerTags   PartnerTags = "1-C-1" // 唱功不好
	OneDOnePartnerTags   PartnerTags = "1-D-1" // 音色普通
	OneDTwoPartnerTags   PartnerTags = "1-D-2" // 情感不饱满
	TwoAOnePartnerTags   PartnerTags = "2-A-1" // 歌词立意一般
	TwoBOnePartnerTags   PartnerTags = "2-B-1" // 旋律平平
	TwoCOnePartnerTags   PartnerTags = "2-C-1" // 唱功一般
	TwoDOnePartnerTags   PartnerTags = "2-D-1" // 音色普通
	TwoDTwoPartnerTags   PartnerTags = "2-D-2" // 情感不饱满
	ThreeAOnePartnerTags PartnerTags = "3-A-1" // 歌词有共鸣
	ThreeATwoPartnerTags PartnerTags = "3-A-2" // 歌词立意不错
	ThreeBOnePartnerTags PartnerTags = "3-B-1" // 旋律耐听
	ThreeCOnePartnerTags PartnerTags = "3-C-1" // 唱功不错
	ThreeDOnePartnerTags PartnerTags = "3-D-1" // 音色独特
	ThreeDTwoPartnerTags PartnerTags = "3-D-2" // 情感到位
	ThreeEOnePartnerTags PartnerTags = "3-E-1" // 有节奏感
	ThreeETwoPartnerTags PartnerTags = "3-E-2" // 洗脑
	FourAOnePartnerTags  PartnerTags = "4-A-1" // 歌词有共鸣
	FourATwoPartnerTags  PartnerTags = "4-A-2" // 歌词立意好
	FourBOnePartnerTags  PartnerTags = "4-B-1" // 旋律耐听
	FourCOnePartnerTags  PartnerTags = "4-C-1" // 唱功不错
	FourDOnePartnerTags  PartnerTags = "4-D-1" // 音色独特
	FourDTwoPartnerTags  PartnerTags = "4-D-2" // 情感到位
	FourEOnePartnerTags  PartnerTags = "4-E-1" // 有节奏感
	FourETwoPartnerTags  PartnerTags = "4-E-2" // 洗脑
	FiveAOnePartnerTags  PartnerTags = "5-A-1" // 歌词强共鸣
	FiveATwoPartnerTags  PartnerTags = "5-A-2" // 歌词立意极高
	FiveBOnePartnerTags  PartnerTags = "5-B-1" // 旋律有记忆点
	FiveCOnePartnerTags  PartnerTags = "5-C-1" // 唱功惊艳
	FiveDOnePartnerTags  PartnerTags = "5-D-1" // 音色独特
	FiveDTwoPartnerTags  PartnerTags = "5-D-2" // 情感到位
	FiveEOnePartnerTags  PartnerTags = "5-E-1" // 有节奏感
	FiveETwoPartnerTags  PartnerTags = "5-E-2" // 洗脑
)

func (p PartnerTags) String() string {
	switch p {
	case OneAOnePartnerTags:
		return "歌词立意差"
	case OneBOnePartnerTags:
		return "旋律无记忆"
	case OneCOnePartnerTags:
		return "唱功不好"
	case OneDOnePartnerTags:
		return "音色普通"
	case OneDTwoPartnerTags:
		return "情感不饱满"
	case TwoAOnePartnerTags:
		return "歌词立意一般"
	case TwoBOnePartnerTags:
		return "旋律平平"
	case TwoCOnePartnerTags:
		return "唱功一般"
	case TwoDOnePartnerTags:
		return "音色普通"
	case TwoDTwoPartnerTags:
		return "情感不饱满"
	case ThreeAOnePartnerTags:
		return "歌词有共鸣"
	case ThreeATwoPartnerTags:
		return "歌词立意不错"
	case ThreeBOnePartnerTags:
		return "旋律耐听"
	case ThreeCOnePartnerTags:
		return "唱功不错"
	case ThreeDOnePartnerTags:
		return "音色独特"
	case ThreeDTwoPartnerTags:
		return "情感到位"
	case ThreeEOnePartnerTags:
		return "有节奏感"
	case ThreeETwoPartnerTags:
		return "洗脑"
	case FourAOnePartnerTags:
		return "歌词有共鸣"
	case FourATwoPartnerTags:
		return "歌词立意好"
	case FourBOnePartnerTags:
		return "旋律耐听"
	case FourCOnePartnerTags:
		return "唱功不错"
	case FourDOnePartnerTags:
		return "音色独特"
	case FourDTwoPartnerTags:
		return "情感到位"
	case FourEOnePartnerTags:
		return "有节奏感"
	case FourETwoPartnerTags:
		return "洗脑"
	case FiveAOnePartnerTags:
		return "歌词强共鸣"
	case FiveATwoPartnerTags:
		return "歌词立意极高"
	case FiveBOnePartnerTags:
		return "旋律有记忆点"
	case FiveCOnePartnerTags:
		return "唱功经验"
	case FiveDOnePartnerTags:
		return "音色独特"
	case FiveDTwoPartnerTags:
		return "情感到位"
	case FiveEOnePartnerTags:
		return "有节奏感"
	case FiveETwoPartnerTags:
		return "洗脑"
	}
	return ""
}

var PartnerTagsGroup = map[int64][]PartnerTags{
	1: {OneAOnePartnerTags, OneBOnePartnerTags, OneCOnePartnerTags, OneDOnePartnerTags, OneDTwoPartnerTags},
	2: {TwoAOnePartnerTags, TwoBOnePartnerTags, TwoCOnePartnerTags, TwoDOnePartnerTags, TwoDTwoPartnerTags},
	3: {ThreeAOnePartnerTags, ThreeATwoPartnerTags, ThreeBOnePartnerTags, ThreeCOnePartnerTags, ThreeDOnePartnerTags, ThreeDTwoPartnerTags, ThreeEOnePartnerTags, ThreeETwoPartnerTags},
	4: {FourAOnePartnerTags, FourATwoPartnerTags, FourBOnePartnerTags, FourCOnePartnerTags, FourDOnePartnerTags, FourDTwoPartnerTags, FourEOnePartnerTags, FourETwoPartnerTags},
	5: {FiveAOnePartnerTags, FiveATwoPartnerTags, FiveBOnePartnerTags, FiveCOnePartnerTags, FiveDOnePartnerTags, FiveDTwoPartnerTags, FiveEOnePartnerTags, FiveETwoPartnerTags},
}

// PartnerEvaluateReq "{"taskId":118761451,"workId":787080,"score":4,"tags":"4-A-1,4-A-2,4-B-1,4-C-1,4-D-1,4-D-2,4-E-1,4-E-2","customTags":"[\"特别\"]","comment":"","syncYunCircle":false,"syncComment":true,"source":"mp-music-partner","csrf_token":"77bf3a5074699038504234d63d68d917"}"
type PartnerEvaluateReq struct {
	types.ReqCommon
	TaskId        int64       `json:"taskId"`        // 任务id 参数值对应https://interface.music.163.com/weapi/music/partner/daily/task/get 接口
	WorkId        int64       `json:"workId"`        // 哪首歌曲id 参数值对应https://interface.music.163.com/weapi/music/partner/daily/task/get 接口
	Score         int64       `json:"score"`         // 分值1~5
	Tags          PartnerTags `json:"tags"`          // 音乐标签 多个以逗号分隔
	CustomTags    string      `json:"customTags"`    // 实际为数组
	Comment       string      `json:"comment"`       // 评论内容
	SyncYunCircle bool        `json:"syncYunCircle"` // 同步到音乐圈中
	SyncComment   bool        `json:"syncComment"`   // ?
	Source        string      `json:"source"`        // 应该表示平台 例如:mp-music-partner
	ExtraResource bool        `json:"extraResource"` // 当测评更多歌曲时使用
}

type PartnerEvaluateResp struct {
	types.RespCommon[any]
}

type PartnerEvaluateRespData struct {
	SongCommentInfo struct {
		CommentId int64  `json:"commentId"`
		ThreadId  string `json:"threadId"`
	} `json:"songCommentInfo"`
	EvaluateRes       bool  `json:"evaluateRes"`
	TodayExtendEvaNum int64 `json:"todayExtendEvaNum"` // 应该是今天测评了多少扩展歌曲
	CurScore          int64 `json:"curScore"`          // 当前歌曲测评得分
}

// PartnerEvaluate 音乐评审提交
func (a *Api) PartnerEvaluate(ctx context.Context, req *PartnerEvaluateReq) (*PartnerEvaluateResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/work/evaluate"
		reply PartnerEvaluateResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerExtraTaskReq struct {
	types.ReqCommon
}

type PartnerExtraTaskResp struct {
	types.RespCommon[[]PartnerExtraTaskRespData]
}

type PartnerExtraTaskRespData struct {
	Comment         string        `json:"comment"`
	Completed       bool          `json:"completed"`
	CustomTags      []interface{} `json:"customTags"`
	Score           float64       `json:"score"`
	SongCommentInfo interface{}   `json:"songCommentInfo"`
	Tags            []interface{} `json:"tags"`
	TaskTitleDesc   string        `json:"taskTitleDesc"`
	UserScore       float64       `json:"userScore"`
	Work            PartnerWork   `json:"work"`
}

// PartnerExtraTask 扩展听歌任务列表,返回得数据有100首歌(2024年10月21日推出的新功能测评)。
func (a *Api) PartnerExtraTask(ctx context.Context, req *PartnerExtraTaskReq) (*PartnerExtraTaskResp, error) {
	var (
		url   = "https://interface.music.163.com/api/music/partner/extra/wait/evaluate/work/list"
		reply PartnerExtraTaskResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerExtraReportReq struct {
	types.ReqCommon
	WorkId        int64  `json:"workId"`
	ResourceId    int64  `json:"resourceId"`
	BizResourceId string `json:"bizResourceId"`
	InteractType  string `json:"interactType"` // PLAY_END(目前只知道这个一个)
}

type PartnerExtraReportResp struct {
	types.RespCommon[PartnerExtraReportRespData]
}

type PartnerExtraReportRespData struct {
	FailedReason   interface{} `json:"failedReason"`
	InteractResult bool        `json:"interactResult"`
}

// PartnerExtraReport 报告扩展听歌任务(2024年10月21日出的新功能测评)
func (a *Api) PartnerExtraReport(ctx context.Context, req *PartnerExtraReportReq) (*PartnerExtraReportResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/partner/resource/interact/report"
		reply PartnerExtraReportResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

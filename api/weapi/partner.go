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
	"net/http"

	"github.com/chaunsin/netease-cloud-music/api"
)

type PartnerPeriodReq struct {
	CsrfToken string `json:"csrf_token"`
}

type PartnerPeriodResp struct {
	api.RespCommon[PartnerPeriodRespData]
}

type PartnerPeriodRespData struct {
	Period        int         `json:"period"`
	Week          int         `json:"week"`
	Periods       string      `json:"periods"`
	SectionPeriod interface{} `json:"sectionPeriod"`
	User          struct {
		UserId    int    `json:"userId"`
		NickName  string `json:"nickName"`
		AvatarUrl string `json:"avatarUrl"`
	} `json:"user"`
	PickRight  interface{} `json:"pickRight"`
	Title      string      `json:"title"`
	Integral   int         `json:"integral"`
	Evaluation struct {
		EvaluateCount    int    `json:"evaluateCount"`
		BasicIntegral    int    `json:"basicIntegral"`
		AccuracyIntegral int    `json:"accuracyIntegral"`
		AccurateCount    int    `json:"accurateCount"`
		AccurateRate     int    `json:"accurateRate"`
		AccuracyLevel    string `json:"accuracyLevel"`
	} `json:"evaluation"`
	Top3 []struct {
		Work struct {
			Id                  int         `json:"id"`
			ResourceType        string      `json:"resourceType"`
			ResourceId          int         `json:"resourceId"`
			Name                string      `json:"name"`
			CoverUrl            string      `json:"coverUrl"`
			AuthorName          string      `json:"authorName"`
			Duration            int         `json:"duration"`
			Source              string      `json:"source"`
			Status              string      `json:"status"`
			BackendForceOffline bool        `json:"backendForceOffline"`
			WorkResourceInfo    interface{} `json:"workResourceInfo"`
		} `json:"work"`
		Score            float64 `json:"score"`
		AvgScore         float64 `json:"avgScore"`
		BasicIntegral    int     `json:"basicIntegral"`
		AccuracyIntegral int     `json:"accuracyIntegral"`
		EvaluateCount    int     `json:"evaluateCount"`
		Tags             []struct {
			Tag   string `json:"tag"`
			Count int    `json:"count"`
		} `json:"tags"`
		ScoreStats struct {
			Field1 int `json:"2.0"`
			Field2 int `json:"4.0"`
			Field3 int `json:"5.0"`
			Field4 int `json:"3.0"`
			Field5 int `json:"1.0,omitempty"`
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
			Id                  int         `json:"id"`
			ResourceType        string      `json:"resourceType"`
			ResourceId          int         `json:"resourceId"`
			Name                string      `json:"name"`
			CoverUrl            string      `json:"coverUrl"`
			AuthorName          string      `json:"authorName"`
			Duration            int         `json:"duration"`
			Source              string      `json:"source"`
			Status              string      `json:"status"`
			BackendForceOffline bool        `json:"backendForceOffline"`
			WorkResourceInfo    interface{} `json:"workResourceInfo"`
		} `json:"work"`
		Score            float64     `json:"score"`
		AvgScore         float64     `json:"avgScore"`
		BasicIntegral    int         `json:"basicIntegral"`
		AccuracyIntegral int         `json:"accuracyIntegral"`
		EvaluateCount    int         `json:"evaluateCount"`
		Tags             interface{} `json:"tags"`
		ScoreStats       interface{} `json:"scoreStats"`
		ScorePercentMap  interface{} `json:"scorePercentMap"`
		Accuracy         float64     `json:"accuracy"`
	} `json:"accurateWorks"`
	ExcellentWorks     []interface{} `json:"excellentWorks"`
	RecoverStatus      bool          `json:"recoverStatus"`
	RecoverExpiredTime int           `json:"recoverExpiredTime"`
	ExcellentPlaylists []struct {
		Id    int64  `json:"id"`
		Name  string `json:"name"`
		Cover string `json:"cover"`
	} `json:"excellentPlaylists"`
	Status            string      `json:"status"`
	ResultConfigTitle interface{} `json:"resultConfigTitle"`
	ConfigedAct       interface{} `json:"configedAct"`
	Eliminated        bool        `json:"eliminated"`
}

// PartnerPeriod 查询当前周期数据报告情况
func (a *Api) PartnerPeriod(ctx context.Context, req *PartnerPeriodReq) (*PartnerPeriodResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/period/result/get"
		reply PartnerPeriodResp
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerPeriodUserinfoReq struct {
	CsrfToken string `json:"csrf_token"`
}

type PartnerPeriodUserinfoResp struct {
	api.RespCommon[PartnerPeriodUserinfoRespData]
}

type PartnerPeriodUserinfoRespData struct {
	UserId        int           `json:"userId"`
	NickName      string        `json:"nickName"`
	AvatarUrl     string        `json:"avatarUrl"`
	Number        int           `json:"number"`
	Title         string        `json:"title"`
	Days          int           `json:"days"`
	Integral      int           `json:"integral"`
	EvaluateCount int           `json:"evaluateCount"`
	PickCount     int           `json:"pickCount"`
	Status        string        `json:"status"`
	PickRights    []interface{} `json:"pickRights"`
	TitleStats    []struct {
		Title string `json:"title"`
		Count int    `json:"count"`
	} `json:"titleStats"`
	CurrentPeriodRank  interface{} `json:"currentPeriodRank"`
	RecoverExpiredTime int         `json:"recoverExpiredTime"`
	RightType          int         `json:"rightType"`
}

// PartnerPeriodUserinfo 查询当前用户数据
func (a *Api) PartnerPeriodUserinfo(ctx context.Context, req *PartnerPeriodUserinfoReq) (*PartnerPeriodUserinfoResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/user/info/get"
		reply PartnerPeriodUserinfoResp
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerLatestReq struct {
	CsrfToken string `json:"csrf_token"`
}

type PartnerLatestResp struct {
	api.RespCommon[PartnerLatestRespData]
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
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerHomeReq struct {
	CsrfToken string `json:"csrf_token"`
}

type PartnerHomeResp struct {
	api.RespCommon[PartnerHomeRespData]
}

type PartnerHomeRespData struct {
	Period    int   `json:"period"`
	Week      int   `json:"week"`
	StartDate int64 `json:"startDate"`
	EndDate   int64 `json:"endDate"`
	User      struct {
		UserId    int    `json:"userId"`
		NickName  string `json:"nickName"`
		AvatarUrl string `json:"avatarUrl"`
		Title     string `json:"title"`
		Days      int    `json:"days"`
		Number    int    `json:"number"`
	} `json:"user"`
	Integral struct {
		Integral            int `json:"integral"`
		CurrentWeekIntegral int `json:"currentWeekIntegral"`
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
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerTaskReq struct {
	CsrfToken string `json:"csrf_token"`
}

type PartnerTaskResp struct {
	api.RespCommon[PartnerTaskRespData]
}

type PartnerTaskRespData struct {
	Id             int         `json:"id"`
	Count          int         `json:"count"`
	CompletedCount int         `json:"completedCount"`
	Integral       int         `json:"integral"`
	TaskTitle      interface{} `json:"taskTitle"`
	Works          []struct {
		Work struct {
			Id                int    `json:"id"`
			ResourceType      string `json:"resourceType"`
			ResourceId        int    `json:"resourceId"`
			Name              string `json:"name"`
			CoverUrl          string `json:"coverUrl"`
			AuthorName        string `json:"authorName"`
			LyricType         int    `json:"lyricType"`
			LyricContent      string `json:"lyricContent"`
			Duration          int    `json:"duration"`
			SongStartPosition int    `json:"songStartPosition"`
			SongEndPosition   int    `json:"songEndPosition"`
			Status            string `json:"status"`
			PlayUrl           string `json:"playUrl"`
			Source            string `json:"source"`
		} `json:"work"`
		Completed     bool        `json:"completed"`
		Score         float64     `json:"score"`
		UserScore     float64     `json:"userScore"`
		Tags          interface{} `json:"tags"`
		CustomTags    interface{} `json:"customTags"`
		Comment       interface{} `json:"comment"`
		TaskTitleDesc interface{} `json:"taskTitleDesc"`
	} `json:"works"`
	PageTaskType int  `json:"pageTaskType"`
	Completed    bool `json:"completed"`
}

// PartnerTask 查询当日任务情况
func (a *Api) PartnerTask(ctx context.Context, req *PartnerTaskReq) (*PartnerTaskResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/daily/task/get"
		reply PartnerTaskResp
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerPickRightReq struct {
	CsrfToken string `json:"csrf_token"`
}

type PartnerPickRightResp struct {
	api.RespCommon[[]PartnerPickRightRespData]
}

// TODO:待补充参数
type PartnerPickRightRespData struct {
}

// PartnerPickRight todo:正确数量？
func (a *Api) PartnerPickRight(ctx context.Context, req *PartnerPickRightReq) (*PartnerPickRightResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/song/pick/right/get"
		reply PartnerPickRightResp
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerNoticeReq struct {
	CsrfToken string `json:"csrf_token"`
}

type PartnerNoticeResp struct {
	api.RespCommon[bool] // todo: 参数待确定
}

// PartnerNotice todo：通知？
func (a *Api) PartnerNotice(ctx context.Context, req *PartnerNoticeReq) (*PartnerNoticeResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/daily/notice/switch/get"
		reply PartnerNoticeResp
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PartnerEvaluateReq struct {
	CsrfToken     string `json:"csrf_token"`
	TaskId        int    `json:"taskId"`        // 任务id 参数值对应https://interface.music.163.com/weapi/music/partner/daily/task/get 接口
	WorkId        int    `json:"workId"`        // 哪首歌曲id 参数值对应https://interface.music.163.com/weapi/music/partner/daily/task/get 接口
	Score         int    `json:"score"`         // 分值1~5
	Tags          string `json:"tags"`          // 音乐标签
	CustomTags    string `json:"customTags"`    // 实际为数组
	Comment       string `json:"comment"`       // 评论内容
	SyncYunCircle bool   `json:"syncYunCircle"` // 同步到音乐圈中
	Source        string `json:"source"`        // 应该表示平台 例如:mp-music-partner
}

type PartnerEvaluateResp struct {
	api.RespCommon[any]
}

// PartnerEvaluate 音乐评审提交
func (a *Api) PartnerEvaluate(ctx context.Context, req *PartnerEvaluateReq) (*PartnerEvaluateResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/music/partner/work/evaluate"
		reply PartnerEvaluateResp
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

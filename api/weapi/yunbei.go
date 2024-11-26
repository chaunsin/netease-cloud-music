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

type SignInReq struct {
	// Type 签到类型 0:安卓(默认) 1:web/PC
	Type int64 `json:"type"`
}

// SignInResp 签到返回
type SignInResp struct {
	// Code 错误码 -2:重复签到 200:成功(会有例外会出现“功能暂不支持”) 301:未登录
	types.RespCommon[any]
	// Point 签到获得积分奖励数量,目前签到规则已经更改变成连续几天签到才能拿获取奖励
	Point int64 `json:"point"`
}

// SignIn 乐签每日签到
// url:
// needLogin: 是
// todo:
//
//	1.目前传0会出现功能暂不支持不知为何(可能请求头或cookie问题)待填坑
//	2.该接口签到成功后在手机app云贝中心看不到对应得奖励数据以及记录,猜测该接口可能要废弃了。
func (a *Api) SignIn(ctx context.Context, req *SignInReq) (*SignInResp, error) {
	var (
		url   = "https://music.163.com/weapi/point/dailyTask"
		reply SignInResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type SignInProgressReq struct {
	ModuleId string `json:"moduleId"` // 默认: 1207signin-1207signin
}

type SignInProgressResp struct {
	types.RespCommon[SignInProgressRespData]
}

type SignInProgressRespData struct {
	// StartTime 时间戳 eg:1638806400000
	StartTime int64 `json:"startTime"`
	// EndTime 时间戳 eg:4102415999000
	EndTime int64
	// Records 记录 YunBeiSignIn(https://music.163.com/weapi/point/dailyTask) 签到信息情况
	Records []struct {
		// Day 签到日期 eg:2024-06-21
		Day string `json:"day"`
		// Signed true:已签到
		Signed bool `json:"signed"`
	} `json:"records"`
	Stats []SignInProgressRespDataStats `json:"stats"`
	// Today 今天签到情况
	Today struct {
		TodaySignedIn bool `json:"todaySignedIn"`
		// TodayStats 里面包含不同类型的签到，连续签到，今日签到等情况，也就是ACCUMULATE、CURRENT_INDEX、CONTINUOUS。
		TodayStats []SignInProgressRespDataStats `json:"todayStats"`
	} `json:"today"`
}

type SignInProgressRespDataStats struct {
	// CalcType 计算方式 ACCUMULATE:累计签到 CURRENT_INDEX:本周/本月签到情况?待确定 CONTINUOUS:连续签到
	CalcType            string                              `json:"calcType"`
	CurrentProgress     int64                               `json:"currentProgress"`
	CurrentSignDesc     interface{}                         `json:"currentSignDesc"`
	Description         string                              `json:"description"`
	EndTime             int64                               `json:"endTime"`
	Id                  int64                               `json:"id"`
	MaxProgressReachDay string                              `json:"maxProgressReachDay"`
	MaxProgressReached  int64                               `json:"maxProgressReached"`
	Prizes              []SignInProgressRespDataStatsPrizes `json:"prizes"`
	RepeatType          string                              `json:"repeatType"` // RepeatType 重复类型 eg:FOUR_WEEKS、NEVER
	StartDay            string                              `json:"startDay"`
	StartTime           int64                               `json:"startTime"`
}

type SignInProgressRespDataStatsPrizes struct {
	Amount           int64  `json:"amount"`
	Description      string `json:"description"`
	Name             string `json:"name"`
	Obtained         bool   `json:"obtained"`
	ObtainedImageUrl string `json:"obtainedImageUrl"`
	PrizeImageUrl    string `json:"prizeImageUrl"`
	Progress         int64  `json:"progress"`
	Type             string `json:"type"`
	Url              string `json:"url"`
}

// SignInProgress 获取签到进度
// url:
// needLogin: 是
func (a *Api) SignInProgress(ctx context.Context, req *SignInProgressReq) (*SignInProgressResp, error) {
	var (
		url   = "https://music.163.com/weapi/act/modules/signin/v2/progress"
		reply SignInProgressResp
		opts  = api.NewOptions()
	)
	if req.ModuleId == "" {
		req.ModuleId = "1207signin-1207signin"
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type SignHappyInfoReq struct{}

type SignHappyInfoResp struct {
	types.RespCommon[any]
}

type SignHappyInfoRespData struct {
	Info struct {
		Author          string      `json:"author"`
		BackColor       string      `json:"backColor"`
		BtnPicUrl       interface{} `json:"btnPicUrl"`
		CurrentUserName string      `json:"currentUserName"`
		EndTime         int64       `json:"endTime"`
		HotComments     []struct {
			AuthorName string `json:"authorName"`
			Content    string `json:"content"`
		} `json:"hotComments"`
		Id                int64       `json:"id"`
		JumpText          interface{} `json:"jumpText"`
		JumpUrl           string      `json:"jumpUrl"`
		MainText          string      `json:"mainText"`
		NewPicUrl         string      `json:"newPicUrl"`
		NewSharePicUrl    string      `json:"newSharePicUrl"`
		Operator          interface{} `json:"operator"`
		PicUrl            string      `json:"picUrl"`
		QrCodeUrl         string      `json:"qrCodeUrl"`
		QrCodeWithLogoUrl string      `json:"qrCodeWithLogoUrl"`
		ResourceAuthor    string      `json:"resourceAuthor"`
		ResourceCover     string      `json:"resourceCover"`
		ResourceId        int64       `json:"resourceId"`
		ResourceName      string      `json:"resourceName"`
		ResourceType      int64       `json:"resourceType"`
		ResourceUrl       string      `json:"resourceUrl"`
		SharePicUrl       string      `json:"sharePicUrl"`
		SpecialJumpUrl    interface{} `json:"specialJumpUrl"`
		StartTime         int64       `json:"startTime"`
		Status            int64       `json:"status"`
		Type              int64       `json:"type"`
		VideoHeight       int64       `json:"videoHeight"`
		VideoStrId        interface{} `json:"videoStrId"`
		VideoWidth        int64       `json:"videoWidth"`
	} `json:"info"`
}

// SignInHappyInfo 乐签签到成功后返回的每日一言信息
// url:
// needLogin: 是
// todo: 该接口应该是旧得签到信息,现在云贝中心里面看不到此信息了
func (a *Api) SignInHappyInfo(ctx context.Context, req *SignHappyInfoReq) (*SignHappyInfoResp, error) {
	var (
		url   = "https://music.163.com/weapi/sign/happy/info"
		reply SignHappyInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSignInfoReq struct{}

// YunBeiSignInfoResp 签到返回
type YunBeiSignInfoResp struct {
	// Code 错误码 200:成功
	types.RespCommon[YunBeiSignInfoRespData]
	// Point 签到获得积分奖励数量,目前签到规则已经更改变成连续几天签到才能拿获取奖励
	Point int64 `json:"point"`
}

type YunBeiSignInfoRespData struct {
	Days   int64 `json:"days"`
	Shells int64 `json:"shells"`
}

// YunBeiSignInfo 获取用户每日签到任务信息？
// url:
// needLogin: 是
func (a *Api) YunBeiSignInfo(ctx context.Context, req *YunBeiSignInfoReq) (*YunBeiSignInfoResp, error) {
	var (
		url   = "https://music.163.com/weapi/point/signed/get"
		reply YunBeiSignInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiUserInfoReq struct{}

type YunBeiUserInfoResp struct {
	types.RespCommon[any]
	// Level 账号等级L1~L10
	Level     int64 `json:"level"`
	UserPoint struct {
		// Balance 云贝可用数量
		Balance int64 `json:"balance"`
		// BlockBalance 云贝冻结数量
		BlockBalance int64 `json:"blockBalance"`
		// Status 状态 0:正常 其他待补充
		Status     int64 `json:"status"`
		UpdateTime int64 `json:"updateTime"`
		UserId     int64 `json:"userId"`
		Version    int64 `json:"version"`
	} `json:"userPoint"`
	MobileSign       bool        `json:"mobileSign"`
	PcSign           bool        `json:"pcSign"`
	Viptype          int64       `json:"viptype"`
	Expiretime       int64       `json:"expiretime"`
	BackupExpireTime int64       `json:"backupExpireTime"`
	StoreTitle       string      `json:"storeTitle"`
	Pubwords         string      `json:"pubwords"`
	GameConfig       interface{} `json:"gameConfig"`
	RingConfig       interface{} `json:"ringConfig"`
	FmConfig         interface{} `json:"fmConfig"`
	TicketConfig     struct {
		PicId  string `json:"picId"`
		PicUrl string `json:"picUrl"`
	} `json:"ticketConfig"`
}

// YunBeiUserInfo 获取用户云贝用户信息
// url:
// needLogin: 是
func (a *Api) YunBeiUserInfo(ctx context.Context, req *YunBeiUserInfoReq) (*YunBeiUserInfoResp, error) {
	var (
		url   = "https://music.163.com/weapi/v1/user/info"
		reply YunBeiUserInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSignInReq struct{}

type YunBeiSignInResp struct {
	types.RespCommon[YunBeiSignInRespData]
}

type YunBeiSignInRespData struct {
	// Sign 签到成功返回true重复签到则返回false
	Sign bool `json:"sign"`
}

// YunBeiSignIn 云贝中心每日签到 该接口应该新版本接口,该接口签到成功后可在云贝中心看到奖励,而 SignIn() 签到成功后看不到奖励
// url:
// needLogin: 是
func (a *Api) YunBeiSignIn(ctx context.Context, req *YunBeiSignInReq) (*YunBeiSignInResp, error) {
	var (
		url   = "https://music.163.com/weapi/pointmall/user/sign"
		reply YunBeiSignInResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiTodaySignInInfoReq struct{}

type YunBeiTodaySignInInfoResp struct {
	types.RespCommon[YunBeiTodaySignInInfoRespData]
}

type YunBeiTodaySignInInfoRespData struct {
	Shells int64 `json:"shells"`
}

// YunBeiTodaySignInInfo 获取今天签到获取的云贝数量
// url:
// needLogin: 是
func (a *Api) YunBeiTodaySignInInfo(ctx context.Context, req *YunBeiTodaySignInInfoReq) (*YunBeiTodaySignInInfoResp, error) {
	var (
		url   = "https://music.163.com/weapi/point/today/get"
		reply YunBeiTodaySignInInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiExpenseReq struct {
	// Limit 每页数量default 10
	Limit int64 `json:"limit"`
	// Offset 第几页
	Offset int64 `json:"offset"`
}

// YunBeiExpenseResp .
type YunBeiExpenseResp struct {
	// Code 错误码 200:成功
	types.RespCommon[[]YunBeiReceiptAndExpenseRespData]
	// HasMore 分页迭代使用
	HasMore bool `json:"hasmore"`
}

// YunBeiExpense 获取用户云贝支出记录列表
// url:
// needLogin: 是
// todo: 迁移到合适的包中
func (a *Api) YunBeiExpense(ctx context.Context, req *YunBeiExpenseReq) (*YunBeiExpenseResp, error) {
	var (
		url   = "https://music.163.com/store/api/point/expense"
		reply YunBeiExpenseResp
		opts  = api.NewOptions()
	)
	if req.Limit == 0 {
		req.Limit = 10
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiReceiptReq struct {
	// Limit 每页数量default 10
	Limit int64 `json:"limit"`
	// Offset 第几页
	Offset int64 `json:"offset"`
}

// YunBeiReceiptResp .
type YunBeiReceiptResp struct {
	// Code 错误码 200:成功
	types.RespCommon[[]YunBeiReceiptAndExpenseRespData]
	// HasMore 分页迭代使用
	HasMore bool `json:"hasmore"`
}

type YunBeiReceiptAndExpenseRespData struct {
	Date string `json:"date"`
	// Fixed 描述
	Fixed string `json:"fixed"`
	Id    int64  `json:"id"`
	// OrderId 订单id
	OrderId interface{} `json:"orderId"`
	// PointCost 云贝数量
	PointCost int64 `json:"pointCost"`
	// Type 0:云贝过期、购买商品、签到奖励、听歌任务奖励、xxx活动等都是0 2:求歌词 其他待补充
	Type int64 `json:"type"`
	// Variable Fixed描述中使用得变量,展示时进行拼接比如type=2时 fixed="求翻译:" variable="爱如潮水" 则前端展示`求翻译:爱如潮水`
	Variable string `json:"variable"`
}

// YunBeiReceipt 获取用户云贝收入记录列表
// url:
// needLogin: 是
// todo: 迁移到合适的包中
func (a *Api) YunBeiReceipt(ctx context.Context, req *YunBeiReceiptReq) (*YunBeiReceiptResp, error) {
	var (
		url   = "https://music.163.com/store/api/point/receipt"
		reply YunBeiReceiptResp
		opts  = api.NewOptions()
	)
	if req.Limit == 0 {
		req.Limit = 10
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiTaskListReq struct{}

type YunBeiTaskListResp struct {
	types.RespCommon[[]YunBeiTaskListRespData]
}

type YunBeiTaskListRespData struct {
	ActionType       int64       `json:"actionType"`
	BackgroundPicUrl interface{} `json:"backgroundPicUrl"`
	// Completed 任务数是否处理
	Completed        bool        `json:"completed"`
	CompletedIconUrl interface{} `json:"completedIconUrl"`
	CompletedPoint   int64       `json:"completedPoint"`
	ExtInfoMap       interface{} `json:"extInfoMap"`
	// Link 任务跳转链接 例如: orpheus://songrcmd
	Link             string      `json:"link"`
	LinkText         string      `json:"linkText"`
	Period           int64       `json:"period"`
	Position         int64       `json:"position"`
	Status           int         `json:"status"`
	TargetPoint      int         `json:"targetPoint"`
	TargetStatus     interface{} `json:"targetStatus"`
	TargetUserTaskId int         `json:"targetUserTaskId"`
	// TaskDescription 任务描述
	TaskDescription string `json:"taskDescription"`
	// TaskId 任务id
	TaskId int `json:"taskId"`
	// TaskName 任务名称
	TaskName string `json:"taskName"`
	// TaskPoint 任务云贝奖励数量
	TaskPoint       int `json:"taskPoint"`
	TaskPointDetail []struct {
		ProgressRate     int    `json:"progressRate"`
		RewardExtendInfo string `json:"rewardExtendInfo"`
		RewardId         int    `json:"rewardId"`
		RewardType       int    `json:"rewardType"`
		SortValue        int    `json:"sortValue"`
		StageType        int    `json:"stageType"`
		Status           int    `json:"status"`
		SumTarget        int    `json:"sumTarget"`
		Times            int    `json:"times"`
		UserMissionId    int    `json:"userMissionId"`
		Value            int    `json:"value"`
		Worth            int    `json:"worth"`
	} `json:"taskPointDetail"`
	TaskType    string      `json:"taskType"`
	UserTaskId  int         `json:"userTaskId"`
	WebPicUrl   string      `json:"webPicUrl"`
	WordsPicUrl interface{} `json:"wordsPicUrl"`
}

// YunBeiTaskList 获取用户云贝任务列表
// url:
// needLogin: 是
func (a *Api) YunBeiTaskList(ctx context.Context, req *YunBeiTaskListReq) (*YunBeiTaskListResp, error) {
	var (
		url   = "https://music.163.com/weapi/usertool/task/list/all"
		reply YunBeiTaskListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiTaskListV3Req struct{}

type YunBeiTaskListV3Resp struct {
	types.RespCommon[YunBeiTaskListRespV3Data]
}

type YunBeiTaskListRespV3Data struct {
	Newbie interface{} `json:"newbie"`
	Normal struct {
		List []struct {
			ActionType       int         `json:"actionType"`
			BackgroundPicUrl interface{} `json:"backgroundPicUrl"`
			Completed        bool        `json:"completed"`
			CompletedIconUrl interface{} `json:"completedIconUrl"`
			CompletedPoint   int         `json:"completedPoint"`
			ExtInfoMap       *struct {
				MissionCode string `json:"missionCode"`
			} `json:"extInfoMap"`
			Link             string      `json:"link"`
			LinkText         string      `json:"linkText"`
			Period           int         `json:"period"`
			Position         int         `json:"position"`
			Status           int         `json:"status"`
			TargetPoint      int         `json:"targetPoint"`
			TargetStatus     interface{} `json:"targetStatus"`
			TargetUserTaskId int         `json:"targetUserTaskId"`
			TaskDescription  string      `json:"taskDescription"`
			TaskId           int         `json:"taskId"`
			TaskName         string      `json:"taskName"`
			TaskPoint        int         `json:"taskPoint"`
			TaskPointDetail  []struct {
				ProgressRate     int    `json:"progressRate"`
				RewardExtendInfo string `json:"rewardExtendInfo"`
				RewardId         int    `json:"rewardId"`
				RewardType       int    `json:"rewardType"`
				SortValue        int    `json:"sortValue"`
				StageType        int    `json:"stageType"`
				Status           int    `json:"status"`
				SumTarget        int    `json:"sumTarget"`
				Times            int    `json:"times"`
				UserMissionId    int64  `json:"userMissionId"`
				Value            int    `json:"value"`
				Worth            int    `json:"worth"`
			} `json:"taskPointDetail"`
			TaskType    string      `json:"taskType"`
			UserTaskId  int64       `json:"userTaskId"`
			WebPicUrl   string      `json:"webPicUrl"`
			WordsPicUrl interface{} `json:"wordsPicUrl"`
		} `json:"list"`
		TypeList []struct {
			Name string `json:"name"`
		} `json:"typeList"`
	} `json:"normal"`
}

// YunBeiTaskListV3 获取用户云贝任务列表V3(任务中心)
// url:
// needLogin: 是
func (a *Api) YunBeiTaskListV3(ctx context.Context, req *YunBeiTaskListV3Req) (*YunBeiTaskListV3Resp, error) {
	var (
		url   = "https://music.163.com/weapi/usertool/task/list/all/v3"
		reply YunBeiTaskListV3Resp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiTaskTodoReq struct{}

type YunBeiTaskTodoResp struct {
	types.RespCommon[[]YunBeiTaskTodoRespData]
}

type YunBeiTaskTodoRespData struct {
	// Completed 任务数是否处理
	Completed   bool `json:"completed"`
	DepositCode int  `json:"depositCode"`
	ExpireTime  int  `json:"expireTime"`
	// Link 任务跳转链接 例如: orpheus://songrcmd
	Link   string `json:"link"`
	Period int    `json:"period"`
	// TaskName 任务名称
	TaskName string `json:"taskName"`
	// TaskPoint 任务云贝奖励数量
	TaskPoint  int   `json:"taskPoint"`
	UserTaskId int64 `json:"userTaskId"`
}

// YunBeiTaskTodo 获取用户云贝todo任务列表,返回得列表中只包含未完成的任务。
// url:
// needLogin: 是
func (a *Api) YunBeiTaskTodo(ctx context.Context, req *YunBeiTaskTodoReq) (*YunBeiTaskTodoResp, error) {
	var (
		url   = "https://music.163.com/weapi/usertool/task/todo/query"
		reply YunBeiTaskTodoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiTaskFinishReq struct {
	Period      string `json:"period"`
	UserTaskId  string `json:"userTaskId"`
	DepositCode string `json:"depositCode"`
}

type YunBeiTaskFinishResp struct {
	Code    string // 此接口code返回类型为string
	Success bool
	Message string
	Data    any
	Ignore  bool
	Present bool
	Empty   bool
}

// YunBeiTaskFinish 获取完成云贝任务奖励,一次只能领取一个,网易一键领取是调用了多次该接口实现。
// url:
// needLogin: 是
func (a *Api) YunBeiTaskFinish(ctx context.Context, req *YunBeiTaskFinishReq) (*YunBeiTaskFinishResp, error) {
	var (
		url   = "https://music.163.com/weapi/usertool/task/point/receive"
		reply YunBeiTaskFinishResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSignInCalendarReq struct{}

type YunBeiSignInCalendarResp struct {
	types.RespCommon[YunBeiSignInCalendarRespData]
}

type YunBeiSignInCalendarRespData struct {
	// SignStr 例如:000000000000111101100000000000 其中1代表对应天数数是否签到
	SignStr string `json:"signStr"`
	// CurTimeStamp 例如:1718792819079
	CurTimeStamp int64 `json:"curTimeStamp"`
}

// YunBeiSignInCalendar 获取签到日历情况
// url:
// needLogin: 是
func (a *Api) YunBeiSignInCalendar(ctx context.Context, req *YunBeiSignInCalendarReq) (*YunBeiSignInCalendarResp, error) {
	var (
		url   = "https://music.163.com/weapi/pointmall/sign/calendar"
		reply YunBeiSignInCalendarResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSignInJudgeReq struct{}

type YunBeiSignInJudgeResp struct {
	// data true 为已签到
	types.RespCommon[bool]
}

// YunBeiSignInJudge todo: 貌似判断当日是否签到状态待确认经测试发现未签到时也是返回true状态，还需要确定排查
// url:
// needLogin: 是
func (a *Api) YunBeiSignInJudge(ctx context.Context, req *YunBeiSignInJudgeReq) (*YunBeiSignInJudgeResp, error) {
	var (
		url   = "https://music.163.com/weapi/pointmall/extra/sign/judge"
		reply YunBeiSignInJudgeResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSignInProgressReq struct{}

type YunBeiSignInProgressResp struct {
	types.RespCommon[YunBeiSignInProgressRespData]
}

type YunBeiSignInProgressRespData struct {
	ReSignJumpUrl string `json:"reSignJumpUrl,omitempty"`
	// ExtraCount 再签几天到可以获得奖励
	ExtraCount    int64                                       `json:"extraCount,omitempty"`
	ExtInfo       string                                      `json:"extInfo,omitempty"`
	LotteryConfig []YunBeiSignInProgressRespDataLotteryConfig `json:"lotteryConfig,omitempty"`
}

type YunBeiSignInProgressRespDataLotteryConfig struct {
	// SignDay 签到天数
	SignDay int `json:"signDay"`
	// BaseGrant 签到奖励相关描述
	BaseGrant struct {
		Id int `json:"id"`
		// Name 签到奖励描述 例如: 3云贝
		Name    string `json:"name"`
		IconUrl string `json:"iconUrl"`
		Type    int    `json:"type"`
		// Note 提示描述 例如: 云贝直接充值到账，详情可至账单查看
		Note string `json:"note"`
	} `json:"baseGrant"`
	ExtraseLotteryId int `json:"extraseLotteryId"`
	// BaseLotteryStatus 签到奖励状态 0:未领取 1:已领取
	BaseLotteryStatus  int `json:"baseLotteryStatus"`
	ExtraLotteryId     int `json:"extraLotteryId"`
	ExtraLotteryStatus int `json:"extraLotteryStatus"`
}

// YunBeiSignInProgress 获取签到阶段奖励列表
// url:
// needLogin: 是
func (a *Api) YunBeiSignInProgress(ctx context.Context, req *YunBeiSignInProgressReq) (*YunBeiSignInProgressResp, error) {
	var (
		url   = "https://music.163.com/weapi/pointmall/user/sign/config"
		reply YunBeiSignInProgressResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiNewJudgeReq struct{}

type YunBeiNewJudgeResp struct {
	types.RespCommon[YunBeiNewJudgeRespData]
}

type YunBeiNewJudgeRespData struct {
	Count       int `json:"count"`
	DetailId    int `json:"detailId"`
	DepositCode int `json:"depositCode"`
}

// YunBeiNewJudge TODO: 未知
// url:
// needLogin: 是
func (a *Api) YunBeiNewJudge(ctx context.Context, req *YunBeiNewJudgeReq) (*YunBeiNewJudgeResp, error) {
	var (
		url   = "https://music.163.com/weapi/usertool/user/new/judge"
		reply YunBeiNewJudgeResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiExpireReq struct{}

type YunBeiExpireResp struct {
	types.RespCommon[YunBeiExpireRespData]
}

type YunBeiExpireRespData struct {
	ExpireAmount int `json:"expireAmount"`
	Day          int `json:"day"`
}

// YunBeiExpire TODO: 应该是获取云贝过期数量
// url:
// needLogin: 是
func (a *Api) YunBeiExpire(ctx context.Context, req *YunBeiExpireReq) (*YunBeiExpireResp, error) {
	var (
		url   = "https://music.163.com/weapi/yunbei/expire/get"
		reply YunBeiExpireResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiRecommendConfigReq struct{}

type YunBeiRecommendConfigResp struct {
	types.RespCommon[YunBeiRecommendConfigRespData]
}

type YunBeiRecommendConfigRespData struct {
	RedeemCount      int    `json:"redeemCount"`
	RedeemFlag       int    `json:"redeemFlag"`
	RedeemProductIds string `json:"redeemProductIds"`
	RefreshTime      int    `json:"refreshTime"`
}

// YunBeiRecommendConfig 推荐配置
// url:
// needLogin: 是
func (a *Api) YunBeiRecommendConfig(ctx context.Context, req *YunBeiRecommendConfigReq) (*YunBeiRecommendConfigResp, error) {
	var (
		url   = "https://music.163.com/weapi/pointmall/recommend/config"
		reply YunBeiRecommendConfigResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

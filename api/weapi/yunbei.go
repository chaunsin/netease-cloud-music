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
	"encoding/json"
	"fmt"
	neturl "net/url"

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

// YunBeiSignIn 云贝中心每日签到.该接口签到成功后可在云贝中心看到奖励,而 SignIn() 签到成功后看不到奖励
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
// har:
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
	Status           int64       `json:"status"`
	TargetPoint      int64       `json:"targetPoint"`
	TargetStatus     interface{} `json:"targetStatus"`
	TargetUserTaskId int64       `json:"targetUserTaskId"`
	// TaskDescription 任务描述
	TaskDescription string `json:"taskDescription"`
	// TaskId 任务id
	TaskId int64 `json:"taskId"`
	// TaskName 任务名称
	TaskName string `json:"taskName"`
	// TaskPoint 任务云贝奖励数量
	TaskPoint       int64 `json:"taskPoint"`
	TaskPointDetail []struct {
		ProgressRate     int64  `json:"progressRate"`
		RewardExtendInfo string `json:"rewardExtendInfo"`
		RewardId         int64  `json:"rewardId"`
		RewardType       int64  `json:"rewardType"`
		SortValue        int64  `json:"sortValue"`
		StageType        int64  `json:"stageType"`
		Status           int64  `json:"status"`
		SumTarget        int64  `json:"sumTarget"`
		Times            int64  `json:"times"`
		UserMissionId    int64  `json:"userMissionId"`
		Value            int64  `json:"value"`
		Worth            int64  `json:"worth"`
	} `json:"taskPointDetail"`
	TaskType    string      `json:"taskType"`
	UserTaskId  int64       `json:"userTaskId"`
	WebPicUrl   string      `json:"webPicUrl"`
	WordsPicUrl interface{} `json:"wordsPicUrl"`
}

// YunBeiTaskList 获取用户云贝任务列表,常规任务
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
			ActionType       int64       `json:"actionType"`
			BackgroundPicUrl interface{} `json:"backgroundPicUrl"`
			Completed        bool        `json:"completed"`
			CompletedIconUrl interface{} `json:"completedIconUrl"`
			CompletedPoint   int64       `json:"completedPoint"`
			ExtInfoMap       *struct {
				MissionCode string `json:"missionCode"`
			} `json:"extInfoMap"`
			Link             string      `json:"link"`
			LinkText         string      `json:"linkText"`
			Period           int64       `json:"period"`
			Position         int64       `json:"position"`
			Status           int64       `json:"status"`
			TargetPoint      int64       `json:"targetPoint"`
			TargetStatus     interface{} `json:"targetStatus"`
			TargetUserTaskId int64       `json:"targetUserTaskId"`
			TaskDescription  string      `json:"taskDescription"`
			TaskId           int64       `json:"taskId"`
			TaskName         string      `json:"taskName"`
			TaskPoint        int64       `json:"taskPoint"`
			TaskPointDetail  []struct {
				ProgressRate     int64  `json:"progressRate"`
				RewardExtendInfo string `json:"rewardExtendInfo"`
				RewardId         int64  `json:"rewardId"`
				RewardType       int64  `json:"rewardType"`
				SortValue        int64  `json:"sortValue"`
				StageType        int64  `json:"stageType"`
				Status           int64  `json:"status"`
				SumTarget        int64  `json:"sumTarget"`
				Times            int64  `json:"times"`
				UserMissionId    int64  `json:"userMissionId"`
				Value            int64  `json:"value"`
				Worth            int64  `json:"worth"`
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
	Completed   bool  `json:"completed"`
	DepositCode int64 `json:"depositCode"`
	ExpireTime  int64 `json:"expireTime"`
	// Link 任务跳转链接 例如: orpheus://songrcmd
	Link   string `json:"link"`
	Period int64  `json:"period"`
	// TaskName 任务名称
	TaskName string `json:"taskName"`
	// TaskPoint 任务云贝奖励数量
	TaskPoint  int64 `json:"taskPoint"`
	UserTaskId int64 `json:"userTaskId"`
}

// YunBeiTaskTodo 返回未完成的任务列表。
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
	Period      string `json:"period"`      // eg: 1
	UserTaskId  string `json:"userTaskId"`  // eg: 293239602686
	DepositCode string `json:"depositCode"` // eg: 1304
}

type YunBeiTaskFinishResp struct {
	types.RespCommon[bool]
}

// YunBeiTaskFinish 完成云贝任务奖励,一次只能领取一个,网易一键领取是调用了多次该接口实现。
// har: 66.har
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
// url: 41.har
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
	// ExtraCount 待分析: 再签几天到可以获得奖励(此理论不对)
	ExtraCount    int64                                       `json:"extraCount,omitempty"`
	ExtInfo       string                                      `json:"extInfo,omitempty"`
	LotteryConfig []YunBeiSignInProgressRespDataLotteryConfig `json:"lotteryConfig,omitempty"`
}

type YunBeiSignInProgressRespDataLotteryConfig struct {
	// SignDay 签到多少天可以获得奖励
	SignDay int64 `json:"signDay"`
	// BaseGrant 签到奖励相关描述
	BaseGrant struct {
		Id int64 `json:"id"`
		// Name 签到奖励描述 例如: 3云贝
		Name    string `json:"name"`
		IconUrl string `json:"iconUrl"`
		Type    int64  `json:"type"`
		// Note 提示描述 例如: 云贝直接充值到账，详情可至账单查看
		Note string `json:"note"`
	} `json:"baseGrant"`
	ExtraGrant *ExtraGrant `json:"extraGrant"`
	// BaseLotteryId 签到奖励id,当可以领取时则有值,反之id为0
	BaseLotteryId int64 `json:"baseLotteryId"`
	// BaseLotteryStatus 签到奖励状态 0:可领取或未达成？ 1:已领取
	BaseLotteryStatus  int64 `json:"baseLotteryStatus"`
	ExtraLotteryId     int64 `json:"extraLotteryId"`
	ExtraLotteryStatus int64 `json:"extraLotteryStatus"`
}

type ExtraGrant struct {
	Id      int64       `json:"id"`
	Name    string      `json:"name"` // eg: 连续签到抽奖机会
	IconUrl interface{} `json:"iconUrl"`
	Type    int64       `json:"type"`
	Note    interface{} `json:"note"`
}

// YunBeiSignInProgress 获取签到阶段奖励列表
// url: 40.har
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
	Count       int64 `json:"count"`
	DetailId    int64 `json:"detailId"`
	DepositCode int64 `json:"depositCode"`
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
	ExpireAmount int64 `json:"expireAmount"`
	Day          int64 `json:"day"`
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
	RedeemCount      int64  `json:"redeemCount"`
	RedeemFlag       int64  `json:"redeemFlag"`
	RedeemProductIds string `json:"redeemProductIds"`
	RefreshTime      int64  `json:"refreshTime"`
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

type YunBeiBalanceReq struct {
	types.ReqCommon
}

type YunBeiBalanceResp struct {
	types.RespCommon[YunBeiBalanceRespData]
}

type YunBeiBalanceRespData struct {
	UserId       int64 `json:"userId"`       // 用户id
	Balance      int64 `json:"balance"`      // 可用数量
	BlockBalance int64 `json:"blockBalance"` // 冻结数量
}

// YunBeiBalance 云贝余额
// har: 39.har
func (a *Api) YunBeiBalance(ctx context.Context, req *YunBeiBalanceReq) (*YunBeiBalanceResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/middle/mall/balance"
		reply YunBeiBalanceResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSignLotteryReq struct {
	types.ReqCommon
	UserLotteryId string `json:"userLotteryId"` // 对应 YunBeiSignInProgressRespDataLotteryConfig 中得BaseLotteryId字段
}

type YunBeiSignLotteryResp struct {
	types.RespCommon[bool] // true: 领取成功,如果领取过则为false
}

// YunBeiSignLottery 每日连续签到云贝领取
// har: 42.har
func (a *Api) YunBeiSignLottery(ctx context.Context, req *YunBeiSignLotteryReq) (*YunBeiSignLotteryResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/pointmall/user/sign/lottery/get"
		reply YunBeiSignLotteryResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSquareBlockCategoryReq struct {
	types.ReqCommon
}

type YunBeiSquareBlockCategoryResp struct {
	types.RespCommon[YunBeiSquareBlockCategoryRespData]
}

type YunBeiSquareBlockCategoryRespData struct {
	BlockCategoryList []YunBeiSquareBlockCategoryRespDataBlockCategoryList `json:"blockCategoryList"`
}

type YunBeiSquareBlockCategoryRespDataBlockCategoryList struct {
	Id                   int64                                                                    `json:"id"`
	Name                 string                                                                   `json:"name"`
	ImageUrl             string                                                                   `json:"imageUrl"`
	SecondCategoryVOList []YunBeiSquareBlockCategoryRespDataBlockCategoryListSecondCategoryVOList `json:"secondCategoryVOList"`
}

type YunBeiSquareBlockCategoryRespDataBlockCategoryListSecondCategoryVOList struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
}

// YunBeiSquareBlockCategory 兑换好礼集合列表 eg: 推荐、云村专区、个性定制、专享权益...
// har: 60.har
// needLogin: 未知
func (a *Api) YunBeiSquareBlockCategory(ctx context.Context, req *YunBeiSquareBlockCategoryReq) (*YunBeiSquareBlockCategoryResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/yunbei-center/square/block/list/category"
		reply YunBeiSquareBlockCategoryResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiRecommendReq struct {
	types.ReqCommon
}

type YunBeiRecommendResp struct {
	types.RespCommon[[]YunBeiRecommendRespData]
}

type YunBeiRecommendRespData struct {
	Id             int64       `json:"id"`
	Name           string      `json:"name"`
	CoverIdStr     string      `json:"coverIdStr"`
	CoverUrl       string      `json:"coverUrl"`
	SpecialType    int64       `json:"specialType"`
	AllowDupBuy    bool        `json:"allowDupBuy"`
	Price          int64       `json:"price"`
	Status         int64       `json:"status"`
	ListPicUrl     string      `json:"listPicUrl"`
	Sales          int64       `json:"sales"`
	RmbOriginPrice string      `json:"rmbOriginPrice"`
	SkuId          int64       `json:"skuId"`
	ExtItemType    interface{} `json:"extItemType"`
	ExtItemId      interface{} `json:"extItemId"`
	CnySkuId       int64       `json:"cnySkuId"`
	CnyProductId   int64       `json:"cnyProductId"`
	ShowTagName    string      `json:"showTagName"`
	ListWebPicUrl  interface{} `json:"listWebPicUrl"`
	SupportShare   int64       `json:"supportShare"`
	ShowType       int64       `json:"showType"`
	InnerLabel     string      `json:"innerLabel"`
	DayLimit       int64       `json:"dayLimit"`
}

// YunBeiRecommend 推荐列表。貌似废弃了
// har: 61.har
// needLogin: 未知
func (a *Api) YunBeiRecommend(ctx context.Context, req *YunBeiRecommendReq) (*YunBeiRecommendResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/pointmall/point/recommend"
		reply YunBeiRecommendResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiTaskRecommendV2Req struct {
	types.ReqCommon
	AdExtJson YunBeiTaskRecommendV2ReqAdExtJson `json:"adExtJson"`
}

// YunBeiTaskRecommendV2ReqAdExtJson
// {"resolution":{"width":450,"height":800},"idfa":"","openudid":"","imei":"","aaid":"","mobilename":"","android_id":"","terminal":"","mac":"","network":0,"op":"","manufacturer":"","oaid":"","teenMode":false,"adReqId":"1289504343_1746441620734_49400","sceneInfo":{"songId":"","gameId":""}}
type YunBeiTaskRecommendV2ReqAdExtJson struct {
	Resolution struct {
		Width  int64 `json:"width"`
		Height int64 `json:"height"`
	} `json:"resolution"`
	Idfa         string `json:"idfa"`
	Openudid     string `json:"openudid"`
	Imei         string `json:"imei"`
	Aaid         string `json:"aaid"`
	Mobilename   string `json:"mobilename"`
	AndroidId    string `json:"android_id"`
	Terminal     string `json:"terminal"`
	Mac          string `json:"mac"`
	Network      int64  `json:"network"`
	Op           string `json:"op"`
	Manufacturer string `json:"manufacturer"`
	Oaid         string `json:"oaid"`
	TeenMode     bool   `json:"teenMode"`
	AdReqId      string `json:"adReqId"`
	SceneInfo    struct {
		SongId string `json:"songId"`
		GameId string `json:"gameId"`
	} `json:"sceneInfo"`
}

type YunBeiTaskRecommendV2Resp struct {
	types.RespCommon[[]YunBeiTaskRecommendV2RespData]
}

type YunBeiTaskRecommendV2RespData struct {
	TaskId           int64       `json:"taskId"`
	UserTaskId       int64       `json:"userTaskId"`
	TaskName         string      `json:"taskName"`
	TaskPoint        int64       `json:"taskPoint"`
	WebPicUrl        string      `json:"webPicUrl"`
	CompletedIconUrl interface{} `json:"completedIconUrl"`
	BackgroundPicUrl interface{} `json:"backgroundPicUrl"`
	WordsPicUrl      interface{} `json:"wordsPicUrl"`
	Link             string      `json:"link"`
	LinkText         string      `json:"linkText"`
	Completed        bool        `json:"completed"`
	CompletedPoint   int64       `json:"completedPoint"`
	Status           int64       `json:"status"`
	TargetStatus     interface{} `json:"targetStatus"`
	TargetPoint      int64       `json:"targetPoint"`
	TargetUserTaskId int64       `json:"targetUserTaskId"`
	TaskDescription  string      `json:"taskDescription"`
	Position         int64       `json:"position"`
	ActionType       int64       `json:"actionType"`
	TaskType         string      `json:"taskType"`
	ExtInfoMap       struct {
		MissionCode string `json:"missionCode"`
	} `json:"extInfoMap"`
	TaskPointDetail []struct {
		UserMissionId    int64  `json:"userMissionId"`
		SortValue        int64  `json:"sortValue"`
		StageType        int64  `json:"stageType"`
		Times            int64  `json:"times"`
		Value            int64  `json:"value"`
		ProgressRate     int64  `json:"progressRate"`
		SumTarget        int64  `json:"sumTarget"`
		RewardId         int64  `json:"rewardId"`
		RewardType       int64  `json:"rewardType"`
		Worth            int64  `json:"worth"`
		RewardExtendInfo string `json:"rewardExtendInfo"`
		Status           int64  `json:"status"`
	} `json:"taskPointDetail"`
	Period    int64  `json:"period"`
	SubAction string `json:"subAction"`
}

// YunBeiTaskRecommendV2 「做任务得云贝」列表. 另外此接口同样的参数每次调用的结果也相同。
// har: 75.har
// needLogin: 未知
func (a *Api) YunBeiTaskRecommendV2(ctx context.Context, req *YunBeiTaskRecommendV2Req) (*YunBeiTaskRecommendV2Resp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/usertool/task/recommend/v2?adExtJson="
		reply YunBeiTaskRecommendV2Resp
		opts  = api.NewOptions()
	)
	data, err := json.Marshal(req.AdExtJson)
	if err != nil {
		return nil, err
	}
	fmt.Printf("data: %+v\n", string(data))
	url += neturl.QueryEscape(string(data))

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiCoinRecordInsertReq struct {
	types.ReqCommon
	ReqId string `json:"reqId"` // eg: 6c63b960-d8fe-446a-b640-b8be30ff99c2
}

type YunBeiCoinRecordInsertResp struct {
	types.RespCommon[any]
}

// YunBeiCoinRecordInsert todo: 广告相关后续分析
// har: 62.har
// needLogin: 未知
func (a *Api) YunBeiCoinRecordInsert(ctx context.Context, req *YunBeiCoinRecordInsertReq) (*YunBeiCoinRecordInsertResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/ad/listening/new/yunbei/coin/record/insert"
		reply YunBeiCoinRecordInsertResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiProductListReq struct {
	types.ReqCommon
	Limit  string `json:"limit"`
	Offset string `json:"offset"` // TODO: 需要明确是否有此字段
}

type YunBeiProductListResp struct {
	types.RespCommon[YunBeiProductListRespData]
}

type YunBeiProductListRespData struct {
	RedirectCategoryId int64                                `json:"redirectCategoryId"`
	OrderList          []YunBeiProductListRespDataOrderList `json:"orderList"`
}

type YunBeiProductListRespDataOrderList struct {
	ProductId       int64  `json:"productId"`
	UserId          int64  `json:"userId"`
	NickName        string `json:"nickName"`
	AvatarUrl       string `json:"avatarUrl"`
	ProductShowName string `json:"productShowName"`
	CategoryId      int64  `json:"categoryId"`
}

// YunBeiProductList 貌似是【兑好礼】中的推荐列表。待确认
// har: 63.har
// needLogin: 未知
func (a *Api) YunBeiProductList(ctx context.Context, req *YunBeiProductListReq) (*YunBeiProductListResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/pointmall/special/product/list"
		reply YunBeiProductListResp
		opts  = api.NewOptions()
	)
	if req.Limit == "" {
		req.Limit = "20"
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSignHolidayReq struct {
	types.ReqCommon
}

type YunBeiSignHolidayResp struct {
	types.RespCommon[string] // 电力满格 快乐无限🥳
}

// YunBeiSignHoliday 提示内容
// har: 64.har
// needLogin: 未知
func (a *Api) YunBeiSignHoliday(ctx context.Context, req *YunBeiSignHolidayReq) (*YunBeiSignHolidayResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/pointmall/user/sign/holiday"
		reply YunBeiSignHolidayResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiTodayRecommendCardReq struct {
	types.ReqCommon
	Scene string `json:"scene"` // eg: 0
}

type YunBeiTodayRecommendCardResp struct {
	types.RespCommon[[]YunBeiTodayRecommendCardRespData]
}

type YunBeiTodayRecommendCardRespData struct {
	Background string `json:"background"`
	Overlay    string `json:"overlay"`
	Theme      string `json:"theme"`
	DateDesc   string `json:"dateDesc"`
}

// YunBeiTodayRecommendCard 获取今日推荐背景相关属性
// har: 65.har
// needLogin: 未知
func (a *Api) YunBeiTodayRecommendCard(ctx context.Context, req *YunBeiTodayRecommendCardReq) (*YunBeiTodayRecommendCardResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/pointmall/today/recommend/card"
		reply YunBeiTodayRecommendCardResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiActivityReserveReq struct {
	types.ReqCommon
}

type YunBeiActivityReserveResp struct {
	types.RespCommon[YunBeiActivityReserveRespData]
	Success bool `json:"success"`
}

type YunBeiActivityReserveRespData struct {
	Type          string `json:"type"`          // eg: NO_PREV_BOOKED
	CurrentAmount int64  `json:"currentAmount"` // 当前可领取的数量
	ImgUrl        string `json:"imgUrl"`
	Title         string `json:"title"`
	SubTitle      string `json:"subTitle"`
	ButtonTitle   string `json:"buttonTitle"`
	Countdown     int64  `json:"countdown"` // 倒计时
}

// YunBeiActivityReserve 预约领取云贝任务查询
// har: 67.har
// needLogin: 未知
func (a *Api) YunBeiActivityReserve(ctx context.Context, req *YunBeiActivityReserveReq) (*YunBeiActivityReserveResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/new/yunbei/activity/reserve/info/simple"
		reply YunBeiActivityReserveResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiMergeConvertReq struct {
	types.ReqCommon
}

type YunBeiMergeConvertResp struct {
	types.RespCommon[int64]
}

// YunBeiMergeConvert todo: 未知
// har: 68.har
// needLogin: 未知
func (a *Api) YunBeiMergeConvert(ctx context.Context, req *YunBeiMergeConvertReq) (*YunBeiMergeConvertResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/pointmall/merge/convert"
		reply YunBeiMergeConvertResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiDragonJudgePopupReq struct {
	types.ReqCommon
}

type YunBeiDragonJudgePopupResp struct {
	types.RespCommon[YunBeiDragonJudgePopupRespData]
}

type YunBeiDragonJudgePopupRespData struct {
	Code    int64       `json:"code"`
	Message interface{} `json:"message"`
	Data    bool        `json:"data"`
}

// YunBeiDragonJudgePopup todo: 未知
// har: 69.har
// needLogin: 未知
func (a *Api) YunBeiDragonJudgePopup(ctx context.Context, req *YunBeiDragonJudgePopupReq) (*YunBeiDragonJudgePopupResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/yunbei/user/dragon/judge/popup"
		reply YunBeiDragonJudgePopupResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSignCalenderDayReq struct {
	types.ReqCommon
	Month string `json:"month"` // eg: 5
	Day   string `json:"day"`   // eg: 5
}

type YunBeiSignCalenderDayResp struct {
	types.RespCommon[YunBeiSignCalenderDayRespData]
}

type YunBeiSignCalenderDayRespData struct{}

// YunBeiSignCalenderDay todo: 未知
// har: 70.har
// needLogin: 未知
func (a *Api) YunBeiSignCalenderDay(ctx context.Context, req *YunBeiSignCalenderDayReq) (*YunBeiSignCalenderDayResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/pointmall/sign/calendar/day"
		reply YunBeiSignCalenderDayResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSignRemindReq struct {
	types.ReqCommon
}

type YunBeiSignRemindResp struct {
	types.RespCommon[int64] // 0:关闭 1:开启
}

// YunBeiSignRemind 是否开启签到提醒
// har: 71.har
// needLogin: 未知
func (a *Api) YunBeiSignRemind(ctx context.Context, req *YunBeiSignRemindReq) (*YunBeiSignRemindResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/pointmall/extra/sign/remind"
		reply YunBeiSignRemindResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiSceneResourceReq struct {
	types.ReqCommon
}

type YunBeiSceneResourceResp struct {
	types.RespCommon[YunBeiSceneResourceRespData]
}

type YunBeiSceneResourceRespData struct {
	ExclusivePositionCodes []interface{} `json:"exclusivePositionCodes"`
	Hints                  []struct {
		Template struct {
			TemplateType int64 `json:"templateType"`
		} `json:"template"`
		Data struct {
			Extra struct {
				Duration int64 `json:"duration"`
				Log      struct {
				} `json:"log"`
				ConstructLogId    string `json:"constructLogId"`
				IconType          int64  `json:"iconType"`
				ShowType          string `json:"showType"`
				StartTime         int64  `json:"startTime"`
				Position          int64  `json:"position"`
				EndTime           int64  `json:"endTime"`
				GeneralizedObject []struct {
					CreativeReachId              string `json:"creativeReachId"`
					Summary                      string `json:"summary"`
					SubIndex                     int64  `json:"subIndex"`
					ResourceId                   string `json:"resourceId"`
					Code                         string `json:"code"`
					ResourceFrequencyControlCode struct {
						PositionCode string `json:"positionCode"`
					} `json:"resourceFrequencyControlCode"`
					TrpId string `json:"trp_id"`
					Log   struct {
						SCtrp string `json:"s_ctrp"`
					} `json:"log"`
					PositionCode string `json:"positionCode"`
					TemplateId   int64  `json:"templateId"`
					CreativeId   int64  `json:"creativeId"`
					Scene        string `json:"scene"`
					TrpType      string `json:"trp_type"`
					PlanId       string `json:"planId"`
					SCtrp        string `json:"s_ctrp"`
					ResourceType string `json:"resourceType"`
					ChannelCode  string `json:"channelCode"`
				} `json:"generalizedObject"`
				LogMap struct {
					Fgid string `json:"fgid"`
				} `json:"logMap"`
			} `json:"extra"`
		} `json:"data"`
		Position struct {
			Code string `json:"code"`
		} `json:"position"`
	} `json:"hints"`
	FixedActions []string `json:"fixedActions"`
	Message      string   `json:"message"`
	Trp          struct {
		Rules []string `json:"rules"`
	} `json:"trp"`
}

// YunBeiSceneResource todo: 未知应该是展示资源样式使用,需要补充request参数。另外需要迁移到合适的文件中。
// har: 72.har
// needLogin: 未知
func (a *Api) YunBeiSceneResource(ctx context.Context, req *YunBeiSceneResourceReq) (*YunBeiSceneResourceResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/link/scene/show/resource"
		reply YunBeiSceneResourceResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiPositionResourceReq struct {
	types.ReqCommon
	PositionCode string `json:"positionCode"` // eg: yunbei_banner
}

type YunBeiPositionResourceResp struct {
	types.RespCommon[YunBeiPositionResourceRespData]
	Trp struct {
		Rules []string `json:"rules"`
	} `json:"trp"`
}

type YunBeiPositionResourceRespData struct {
	LibraLogList    []interface{} `json:"libraLogList"`
	ExposureRecords string        `json:"exposureRecords"`
}

// YunBeiPositionResource todo: 未知应该是展示资源样式使用。另外需要迁移到合适的文件中。
// har: 73.har
// needLogin: 未知
func (a *Api) YunBeiPositionResource(ctx context.Context, req *YunBeiPositionResourceReq) (*YunBeiPositionResourceResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/link/position/show/resource"
		reply YunBeiPositionResourceResp
		opts  = api.NewOptions()
	)
	if req.PositionCode != "" {
		url = url + "?positionCode=" + req.PositionCode
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiMultiTerminalWidgetCalenderReq struct {
	types.ReqCommon
	Suggest string `json:"suggest"`
}

type YunBeiMultiTerminalWidgetCalenderResp struct {
	types.RespCommon[YunBeiMultiTerminalWidgetCalenderRespData]
}

type YunBeiMultiTerminalWidgetCalenderRespData struct {
	Texts           []string `json:"texts"`
	Origin          string   `json:"origin"`
	SongId          int64    `json:"songId"`
	CommentId       int64    `json:"commentId"`
	SongName        string   `json:"songName"`
	CoverUrl        string   `json:"coverUrl"`
	SingerName      string   `json:"singerName"`
	CommentCalendar struct {
		Festival                  interface{} `json:"festival"`
		DateImg                   interface{} `json:"dateImg"`
		BigBackground             string      `json:"bigBackground"`
		Background                string      `json:"background"`
		FontColor                 interface{} `json:"fontColor"`
		AndroidRoundedCornerImg   string      `json:"androidRoundedCornerImg"`
		AndroidSmallWidgetMainImg interface{} `json:"androidSmallWidgetMainImg"`
		MonthImg                  string      `json:"monthImg"`
		Month                     int64       `json:"month"`
		Day                       int64       `json:"day"`
		DayOfWeek                 int64       `json:"dayOfWeek"`
		DayImg                    string      `json:"dayImg"`
		DateColor                 interface{} `json:"dateColor"`
		LogoColor                 interface{} `json:"logoColor"`
		ContentColor              interface{} `json:"contentColor"`
		DescColor                 interface{} `json:"descColor"`
		MusicNameColor            interface{} `json:"musicNameColor"`
		MusicArtistColor          interface{} `json:"musicArtistColor"`
		PlayBtnColor              interface{} `json:"playBtnColor"`
	} `json:"commentCalendar"`
}

// YunBeiMultiTerminalWidgetCalender todo: 貌似好像是签到成功之后返回的日历信息，需要确认。另外需要迁移到合适的文件中。
// har: 74.har
// needLogin: 未知
func (a *Api) YunBeiMultiTerminalWidgetCalender(ctx context.Context, req *YunBeiMultiTerminalWidgetCalenderReq) (*YunBeiMultiTerminalWidgetCalenderResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/music/multi/terminal/widget/24/comment/calendar" // 24是动态参数？
		reply YunBeiMultiTerminalWidgetCalenderResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiDayVipInfoReq struct {
	types.ReqCommon
}

type YunBeiDayVipInfoResp struct {
	types.RespCommon[YunBeiDayVipInfoRespData]
}

type YunBeiDayVipInfoRespData struct {
	ReqId                        string      `json:"reqId"`
	SkuCode                      int64       `json:"skuCode"`
	SkuImgUrl                    string      `json:"skuImgUrl"`
	CurrentStageOriginCoinAmount int64       `json:"currentStageOriginCoinAmount"` // 兑换需要的原价云贝数量
	CurrentStageActualCoinAmount int64       `json:"currentStageActualCoinAmount"` // 当前兑换需要的实际云贝数量
	CurrentUserCoinAmount        int64       `json:"currentUserCoinAmount"`        // 当前用户的当前阶段可用的云贝数量
	CurrentStage                 int64       `json:"currentStage"`
	CurrentStageCompleted        bool        `json:"currentStageCompleted"`
	TodayHasNext                 bool        `json:"todayHasNext"`
	TodayUnlockNext              interface{} `json:"todayUnlockNext"`
	ButtonTitle                  string      `json:"buttonTitle"` // eg: 去兑换
	CurrentButtonStatus          int64       `json:"currentButtonStatus"`
	UnlockCoinAmount             interface{} `json:"unlockCoinAmount"`
	ActionUrl                    interface{} `json:"actionUrl"`
	BubbleDisplayed              interface{} `json:"bubbleDisplayed"`
	BubbleCoinAmount             interface{} `json:"bubbleCoinAmount"`
	SubButtonTitle               interface{} `json:"subButtonTitle"`
	SubActionUrl                 interface{} `json:"subActionUrl"`
	SubTitle                     string      `json:"subTitle"` // eg: 金币已集齐，快去兑换VIP吧~
}

// YunBeiDayVipInfo 「显示福利」黑胶vip天卡兑换信息查询
// har: 74.har
// needLogin: 未知
func (a *Api) YunBeiDayVipInfo(ctx context.Context, req *YunBeiDayVipInfoReq) (*YunBeiDayVipInfoResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/ad/listening/new/yunbei/center/day/vip/info"
		reply YunBeiDayVipInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

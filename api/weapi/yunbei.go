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

	"github.com/chaunsin/netease-cloud-music/api/types"
)

type YunBeiSignInReq struct {
	// Type 签到类型 0:安卓(默认) 1:web/PC
	Type int64 `json:"type"`
}

// YunBeiSignInResp 签到返回
type YunBeiSignInResp struct {
	// Code 错误码 -2:重复签到 200:成功(会有例外会出现“功能暂不支持”) 301:未登录
	types.RespCommon[any]
	// Point 签到获得积分奖励数量,目前签到规则已经更改变成连续几天签到才能拿获取奖励
	Point int64 `json:"point"`
}

// YunBeiSignIn 用户每日签到
// url:
// needLogin: 是
// todo:目前传0会出现功能暂不支持不知为何(可能请求头或cookie问题)待填坑
func (a *Api) YunBeiSignIn(ctx context.Context, req *YunBeiSignInReq) (*YunBeiSignInResp, error) {
	var (
		url   = "https://music.163.com/weapi/point/dailyTask"
		reply YunBeiSignInResp
	)
	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
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
// todo: 迁移到合适的包中
func (a *Api) YunBeiSignInfo(ctx context.Context, req *YunBeiSignInfoReq) (*YunBeiSignInfoResp, error) {
	var (
		url   = "https://music.163.com/api/point/signed/get"
		reply YunBeiSignInfoResp
	)
	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
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

// YunBeiExpense 获取用户云贝支出记录
// url:
// needLogin: 是
// todo: 迁移到合适的包中
func (a *Api) YunBeiExpense(ctx context.Context, req *YunBeiExpenseReq) (*YunBeiExpenseResp, error) {
	var (
		url   = "https://music.163.com/store/api/point/expense"
		reply YunBeiExpenseResp
	)
	if req.Limit == 0 {
		req.Limit = 10
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
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
	PointCost int `json:"pointCost"`
	// Type 0:云贝过期、购买商品、签到奖励、听歌任务奖励、xxx活动等都是0 2:求歌词 其他待补充
	Type int `json:"type"`
	// Variable Fixed描述中使用得变量,展示时进行拼接比如type=2时 fixed="求翻译:" variable="爱如潮水" 则前端展示`求翻译:爱如潮水`
	Variable string `json:"variable"`
}

// YunBeiReceipt 获取用户云贝收入记录
// url:
// needLogin: 是
// todo: 迁移到合适的包中
func (a *Api) YunBeiReceipt(ctx context.Context, req *YunBeiReceiptReq) (*YunBeiReceiptResp, error) {
	var (
		url   = "https://music.163.com/store/api/point/receipt"
		reply YunBeiReceiptResp
	)
	if req.Limit == 0 {
		req.Limit = 10
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
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
	Level     int `json:"level"`
	UserPoint struct {
		// Balance 云贝可用数量
		Balance int `json:"balance"`
		// BlockBalance 云贝冻结数量
		BlockBalance int `json:"blockBalance"`
		// Status 状态 0:正常 其他待补充
		Status     int   `json:"status"`
		UpdateTime int64 `json:"updateTime"`
		UserId     int   `json:"userId"`
		Version    int   `json:"version"`
	} `json:"userPoint"`
	MobileSign       bool        `json:"mobileSign"`
	PcSign           bool        `json:"pcSign"`
	Viptype          int         `json:"viptype"`
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
// todo: 迁移到合适的包中
func (a *Api) YunBeiUserInfo(ctx context.Context, req *YunBeiUserInfoReq) (*YunBeiUserInfoResp, error) {
	var (
		url   = "https://music.163.com/api/v1/user/info"
		reply YunBeiUserInfoResp
	)

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiPointMallSignInReq struct{}

type YunBeiPointMallSignInResp struct {
	types.RespCommon[YunBeiPointMallSignInRespData]
}

type YunBeiPointMallSignInRespData struct {
	Sign bool `json:"sign"`
}

// YunBeiPointMallSignIn todo: 功能未知待分析
// url:
// needLogin: 是
// todo: 迁移到合适的包中
func (a *Api) YunBeiPointMallSignIn(ctx context.Context, req *YunBeiPointMallSignInReq) (*YunBeiPointMallSignInResp, error) {
	var (
		url   = "https://music.163.com/api/pointmall/user/sign"
		reply YunBeiPointMallSignInResp
	)

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type YunBeiTodaySignInReq struct{}

type YunBeiTodaySignInResp struct {
	types.RespCommon[any]
}

type YunBeiTodaySignInRespData struct {
	Shells int64 `json:"shells"`
}

// YunBeiTodaySignIn 每日签到 todo: YunBeiSignIn() 方法有啥差异？
// url:
// needLogin: 是
// todo: 迁移到合适的包中
func (a *Api) YunBeiTodaySignIn(ctx context.Context, req *YunBeiTodaySignInReq) (*YunBeiTodaySignInResp, error) {
	var (
		url   = "https://music.163.com/api/point/today/get"
		reply YunBeiTodaySignInResp
	)

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

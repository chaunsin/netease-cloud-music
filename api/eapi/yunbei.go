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
	"net/http"

	"github.com/chaunsin/netease-cloud-music/api/types"
)

type YunBeiSignReq struct {
	// Type 签到类型 0:安卓(默认) 1:web/PC
	Type int64 `json:"type"`
}

type YunBeiSignResp struct {
	// 错误码 -2:重复签到 200:成功
	types.RespCommon[any]
	Point int64 `json:"point"`
}

// YunBeiSign 用户每日签到
// url:
// needLogin: 未知
// todo:目前传0会出现功能暂不支持不知为何(可能请求头或cookie问题)待填坑
func (a *Api) YunBeiSign(ctx context.Context, req *YunBeiSignReq) (*YunBeiSignResp, error) {
	var (
		url   = "https://music.163.com/eapi/point/dailyTask"
		reply YunBeiSignResp
	)
	resp, err := a.client.Request(ctx, http.MethodPost, url, "eapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

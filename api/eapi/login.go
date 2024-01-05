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

	"github.com/chaunsin/netease-cloud-music/api"
)

type LoginPhoneReq struct {
	CounterCode   string
	Phone         string
	Password      string
	Captcha       string
	RememberLogin bool
}

type LoginPhoneResp struct {
	api.RespCommon
}

// LoginPhone 手机号登录
func (a *Api) LoginPhone(ctx context.Context, req *LoginPhoneReq) (*LoginPhoneResp, error) {
	var reply LoginPhoneResp
	// resp, err := a.cli.R().
	// 	SetContext(ctx).
	// 	SetHeader("Content-Type", "").
	// 	SetResult(&reply).
	// 	Post("https://music.163.com/eapi/w/login/cellphone")
	// if err != nil {
	// 	return nil, err
	// }
	// if resp.StatusCode() != http.StatusOK {
	// 	return nil, fmt.Errorf("http status code: %d", resp.StatusCode())
	// }
	return &reply, nil
}

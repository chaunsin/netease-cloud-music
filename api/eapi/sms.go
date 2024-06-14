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

	"github.com/chaunsin/netease-cloud-music/api/types"
)

type CaptchaSendReq struct {
	Phone  string
	CTCode string
}

type CaptchaSendResp struct {
	types.RespCommon[any]
}

// CaptchaSend 发送验证码 PC客户端
func (a *Api) CaptchaSend(ctx context.Context, req *CaptchaSendReq) (*CaptchaSendResp, error) {
	// TODO
	return nil, nil
}

type CaptchaVerifyReq struct {
	Phone   string `json:"phone"`
	CTCode  string `json:"ctcode"`
	Captcha string `json:"captcha"`
}

type CaptchaVerifyResp struct {
	types.RespCommon[any]
}

// CaptchaVerify 验证验证码
func (a *Api) CaptchaVerify(ctx context.Context, req *CaptchaVerifyReq) (*CaptchaVerifyResp, error) {
	// TODO
	return nil, nil
}

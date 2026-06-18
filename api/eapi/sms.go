// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

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

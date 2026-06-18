// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	phone = "your mobile phone number"
	ct    = "86"
)

func TestCaptchaSend(t *testing.T) {
	// 发送验证码
	var req = CaptchaSendReq{
		Phone:  phone,
		CTCode: ct,
	}
	got, err := cli.CaptchaSend(ctx, &req)
	assert.NoError(t, err)
	t.Logf("CaptchaSend: %+v\n", got)
}

func TestCaptchaVerify(t *testing.T) {
	// 发送验证码
	var req = CaptchaVerifyReq{
		Phone:   phone,
		CTCode:  ct,
		Captcha: "2129",
	}
	got, err := cli.CaptchaVerify(ctx, &req)
	assert.NoError(t, err)
	t.Logf("CaptchaVerify: %+v\n", got)
}

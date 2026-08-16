// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	phone = "your mobile phone number"
	ct    = "86"
)

func TestCaptchaSend(t *testing.T) {
	// 发送验证码
	cli := new(Api)
	req := CaptchaSendReq{
		Phone:  phone,
		CTCode: ct,
	}
	got, err := cli.CaptchaSend(t.Context(), &req)
	require.Nil(t, got)
	require.EqualError(t, err, "CaptchaSend is not implemented")
}

func TestCaptchaVerify(t *testing.T) {
	// 发送验证码
	cli := new(Api)
	req := CaptchaVerifyReq{
		Phone:   phone,
		CTCode:  ct,
		Captcha: "2129",
	}
	got, err := cli.CaptchaVerify(t.Context(), &req)
	require.Nil(t, got)
	require.EqualError(t, err, "CaptchaVerify is not implemented")
}

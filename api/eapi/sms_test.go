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
	got, err := a.CaptchaSend(ctx, &req)
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
	got, err := a.CaptchaVerify(ctx, &req)
	assert.NoError(t, err)
	t.Logf("CaptchaVerify: %+v\n", got)
}

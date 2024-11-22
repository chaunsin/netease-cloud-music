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

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"

	"github.com/skip2/go-qrcode"
)

type LoginPhoneReq struct {
	CounterCode   string
	Phone         string
	Password      string
	Captcha       string
	RememberLogin bool
}

type LoginPhoneResp struct {
	types.RespCommon[any]
}

// LoginPhone 手机号登录
func (a *Api) LoginPhone(ctx context.Context, req *LoginPhoneReq) (*LoginPhoneResp, error) {
	var reply LoginPhoneResp
	// todo: 由于云盾关系绕不过暂时搁置
	return &reply, nil
}

type QrcodeCreateKeyReq struct {
	types.ReqCommon
	Type int64 `json:"type"`
}

type QrcodeCreateKeyResp struct {
	types.RespCommon[any]
	UniKey string `json:"unikey"`
}

// QrcodeCreateKey 生成二维码需要得key
// 常见问题
// 1. 请求成功了,但是body为空值什么也没有,原因还是参数加密出现了问题。
// 2. crsftoken 可传可不传个人猜测前端写得通用框架传了
func (a *Api) QrcodeCreateKey(ctx context.Context, req *QrcodeCreateKeyReq) (*QrcodeCreateKeyResp, error) {
	var (
		url   = "https://music.163.com/eapi/login/qrcode/unikey"
		reply QrcodeCreateKeyResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type QrcodeGenerateReq struct {
	CodeKey string
}

type QrcodeGenerateResp struct {
	types.RespCommon[any]
	Qrcode      []byte //
	QrcodePrint string
}

// QrcodeGenerate 根据 QrcodeCreateKey 接口生成得key生成生成二维码,注意此处不是调用服务接口。
func (a *Api) QrcodeGenerate(ctx context.Context, req *QrcodeGenerateReq) (*QrcodeGenerateResp, error) {
	var (
		content = fmt.Sprintf("https://music.163.com/login?codekey=%s", req.CodeKey)
		reply   QrcodeGenerateResp
	)

	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	reply.Qrcode, err = qr.PNG(256)
	if err != nil {
		return nil, fmt.Errorf("PNG: %w", err)
	}
	reply.QrcodePrint = qr.ToSmallString(false)
	// if err := qr.WriteFile(256, "./qrcode.png"); err != nil {
	// 	return nil, fmt.Errorf("WriteFile: %w", err)
	// }

	// if err := qrcode.WriteFile(content, qrcode.Medium, 256, "./qrcode.png"); err != nil {
	// 	return nil, fmt.Errorf("WriteFile: %w", err)
	// }
	return &reply, nil
}

type QrcodeCheckReq struct {
	Key  string `json:"key"`  // QrcodeCreateKey()返回值codekey
	Type int64  `json:"type"` // 目前传3 weapi中好像传1
}

type QrcodeCheckResp struct {
	types.RespCommon[any]
}

// QrcodeCheck 查询扫码状态
// 返回值:
// 800-二维码不存在或已过期
// 801-等待扫码
// 802-正在扫码授权中
// 803-授权登录成功
func (a *Api) QrcodeCheck(ctx context.Context, req *QrcodeCheckReq) (*QrcodeCheckResp, error) {
	var (
		url   = "https://music.163.com/eapi/login/qrcode/client/login"
		reply QrcodeCheckResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type GetUserInfoReq struct {
	types.ReqCommon
}

type GetUserInfoResp struct {
	types.RespCommon[any]
	Account GetUserInfoRespAccount `json:"account"`
	Profile GetUserInfoRespProfile `json:"profile"`
}

type GetUserInfoRespAccount struct {
	Id                 int64  `json:"id"`
	UserName           string `json:"userName"`
	Type               int64  `json:"type"`
	Status             int64  `json:"status"`
	WhitelistAuthority int64  `json:"whitelistAuthority"`
	CreateTime         int64  `json:"createTime"`
	TokenVersion       int64  `json:"tokenVersion"`
	Ban                int64  `json:"ban"`
	BaoyueVersion      int64  `json:"baoyueVersion"`
	DonateVersion      int64  `json:"donateVersion"`
	VipType            int64  `json:"vipType"`
	AnonimousUser      bool   `json:"anonimousUser"`
	PaidFee            bool   `json:"paidFee"`
}

type GetUserInfoRespProfile struct {
	UserId              int64       `json:"userId"`
	UserType            int64       `json:"userType"`
	Nickname            string      `json:"nickname"`
	AvatarImgId         int64       `json:"avatarImgId"`
	AvatarUrl           string      `json:"avatarUrl"`
	BackgroundImgId     int64       `json:"backgroundImgId"`
	BackgroundUrl       string      `json:"backgroundUrl"`
	Signature           string      `json:"signature"`
	CreateTime          int64       `json:"createTime"`
	UserName            string      `json:"userName"`
	AccountType         int64       `json:"accountType"`
	ShortUserName       string      `json:"shortUserName"`
	Birthday            int64       `json:"birthday"`
	Authority           int64       `json:"authority"`
	Gender              int64       `json:"gender"`
	AccountStatus       int64       `json:"accountStatus"`
	Province            int64       `json:"province"`
	City                int64       `json:"city"`
	AuthStatus          int64       `json:"authStatus"`
	Description         interface{} `json:"description"`
	DetailDescription   interface{} `json:"detailDescription"`
	DefaultAvatar       bool        `json:"defaultAvatar"`
	ExpertTags          interface{} `json:"expertTags"`
	Experts             interface{} `json:"experts"`
	DjStatus            int64       `json:"djStatus"`
	LocationStatus      int64       `json:"locationStatus"`
	VipType             int64       `json:"vipType"`
	Followed            bool        `json:"followed"`
	Mutual              bool        `json:"mutual"`
	Authenticated       bool        `json:"authenticated"`
	LastLoginTime       int64       `json:"lastLoginTime"`
	LastLoginIP         string      `json:"lastLoginIP"`
	RemarkName          interface{} `json:"remarkName"`
	ViptypeVersion      int64       `json:"viptypeVersion"`
	AuthenticationTypes int64       `json:"authenticationTypes"`
	AvatarDetail        interface{} `json:"avatarDetail"`
	Anchor              bool        `json:"anchor"`
}

// GetUserInfo 获取用户信息
func (a *Api) GetUserInfo(ctx context.Context, req *GetUserInfoReq) (*GetUserInfoResp, error) {
	var (
		url   = "https://music.163.com/eapi/w/nuser/account/get"
		reply GetUserInfoResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type RefreshTokenReq struct {
}

type RefreshTokenResp struct {
	types.RespCommon[any]
}

// RefreshToken token刷新
// url: https://app.apifox.com/project/3870894
func (a *Api) RefreshToken(ctx context.Context, req *RefreshTokenReq) (*RefreshTokenResp, error) {
	var (
		url   = "https://music.163.com/eapi/login/token/refresh"
		reply RefreshTokenResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

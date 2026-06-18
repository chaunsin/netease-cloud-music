// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"context"
	"net/url"
	"time"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type Api struct {
	client *api.Client
}

func New(client *api.Client) *Api {
	a := Api{client: client}
	return &a
}

func (a *Api) NeedLogin(ctx context.Context) bool {
	u, _ := url.Parse("https://music.163.com")
	for _, ck := range a.client.GetClient().Jar.Cookies(u) {
		// 判断用户是否有登录信息,如果有登录信息,还需要调用接口进行判断,单纯的判断cookie过期时间是不行的
		if ck.Name == "MUSIC_U" && ck.Expires.Before(time.Now()) {
			reply, err := a.GetUserInfo(ctx, &GetUserInfoReq{})
			if err != nil {
				return true
			}
			log.Debug("NeedLogin: %+v", reply)
			if reply.Code != 200 || reply.Account == nil || reply.Profile == nil {
				return true
			}
			return false
		}
	}
	return true
}

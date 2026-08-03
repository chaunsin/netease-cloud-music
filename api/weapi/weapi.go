// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"context"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

const userInfoURL = "https://music.163.com/weapi/w/nuser/account/get"

type Api struct {
	client *api.Client
}

func New(client *api.Client) *Api {
	a := Api{client: client}
	return &a
}

func (a *Api) NeedLogin(ctx context.Context) bool {
	musicU, ok := a.client.Cookie(userInfoURL, "MUSIC_U")
	if !ok || musicU.Value == "" {
		musicU, ok = a.client.Cookie(userInfoURL, "MUSIC_R_U")
	}

	if !ok || musicU.Value == "" {
		return true
	}

	// Cookie presence is only a hint; the account endpoint remains authoritative.
	reply, err := a.GetUserInfo(ctx, &GetUserInfoReq{})
	if err != nil {
		return true
	}

	log.Debugf("NeedLogin: code=%d account=%t profile=%t", reply.Code, reply.Account != nil, reply.Profile != nil)
	return reply.Code != 200 || reply.Account == nil || reply.Profile == nil
}

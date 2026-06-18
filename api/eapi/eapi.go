// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import "github.com/chaunsin/netease-cloud-music/api"

type Api struct {
	client *api.Client
}

func New(client *api.Client) *Api {
	a := Api{client: client}
	return &a
}

func (a *Api) fillMusicianEAPIReq(req *MusicianEAPIReq) {
	if req.DeviceId == "" {
		req.DeviceId = a.client.GetDeviceId()
	}
	if req.OS == "" {
		req.OS = "iOS"
	}
	if req.VerifyId == 0 {
		req.VerifyId = 1
	}
	if req.Header == nil {
		req.Header = struct{}{}
	}
	req.ER = true
}

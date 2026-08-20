// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package linux

import "github.com/chaunsin/netease-cloud-music/api"

type Api struct {
	client *api.Client
}

func New(client *api.Client) *Api {
	a := Api{client: client}
	return &a
}

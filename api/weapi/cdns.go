// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type CDNListReq struct{}

type CDNListResp struct {
	types.RespCommon[[][]string]
}

// CDNList 获取CDN列表
// url: testdata/har/5.har
// needLogin: 未知.
func (a *Api) CDNList(ctx context.Context, req *CDNListReq) (*CDNListResp, error) {
	var (
		url   = "https://music.163.com/weapi/cdns"
		reply CDNListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

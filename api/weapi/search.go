// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type SearchDefaultReq struct{}

type SearchDefaultResp struct {
	types.RespCommon[SearchDefaultRespData]
}

type SearchDefaultRespData struct {
	RefreshTime int `json:"refreshTime"`
	Keywords    []struct {
		Action       int    `json:"action"`
		Alg          string `json:"alg"`
		BizQueryInfo string `json:"bizQueryInfo"`
		Gap          int    `json:"gap"`
		ImageUrl     any    `json:"imageUrl"`
		LogInfo      any    `json:"logInfo"`
		Realkeyword  string `json:"realkeyword"`
		SearchType   int    `json:"searchType"`
		ShowKeyword  string `json:"showKeyword"`
		Source       any    `json:"source"`
		StyleKeyword struct {
			DescWord *string `json:"descWord"`
			KeyWord  string  `json:"keyWord"`
		} `json:"styleKeyword"`
		TrpId   any `json:"trp_id"`
		TrpType any `json:"trp_type"`
	} `json:"keywords"`
}

// SearchDefault 首页搜索输入框默认搜索关键词
// url: testdata/har/11.har
// needLogin: 未知
func (a *Api) SearchDefault(ctx context.Context, req *SearchDefaultReq) (*SearchDefaultResp, error) {
	var (
		url   = "https://interface.music.163.com/eapi/search/default/keyword/get"
		reply SearchDefaultResp
		opts  = api.NewOptions()
	)

	opts.CryptoMode = api.CryptoModeEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

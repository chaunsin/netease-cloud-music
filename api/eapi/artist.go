// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaunsin/netease-cloud-music/api"
)

type ArtistHotReq struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type ArtistHotResp struct {
	Code int                 `json:"code"`
	More bool                `json:"more"`
	Data []ArtistHotRespData `json:"data"`
}

type ArtistHotRespData struct {
	Artists []ArtistHotRespArtist `json:"artists"`
	Title   string                `json:"title"`
}

type ArtistHotRespArtist struct {
	Id       int64  `json:"id"`
	Name     string `json:"name"`
	Followed bool   `json:"followed"`
}

// ArtistHot 获取热门歌手列表
func (a *Api) ArtistHot(ctx context.Context, req *ArtistHotReq) (*ArtistHotResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/artist/hot"
		reply ArtistHotResp
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

type ArtistSubReq struct {
	ArtistId string `json:"artistId"`
}

type ArtistSubResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ArtistSub 关注歌手
func (a *Api) ArtistSub(ctx context.Context, req *ArtistSubReq) (*ArtistSubResp, error) {
	var (
		url   = "https://music.163.com/weapi/artist/sub"
		reply ArtistSubResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeWEAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type ArtistUnsubReq struct {
	ArtistIds string `json:"artistIds"`
}

type ArtistUnsubResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type weapiArtistUnsubReq struct {
	ArtistIds []string `json:"artistIds"`
}

// ArtistUnsub 取消关注歌手
func (a *Api) ArtistUnsub(ctx context.Context, req *ArtistUnsubReq) (*ArtistUnsubResp, error) {
	// Parse req.ArtistIds string (e.g. "[3684]" or "3684") into []string
	idsStr := req.ArtistIds
	idsStr = strings.ReplaceAll(idsStr, "[", "")
	idsStr = strings.ReplaceAll(idsStr, "]", "")
	idsStr = strings.ReplaceAll(idsStr, "\"", "")
	var parsedIds []string
	for _, id := range strings.Split(idsStr, ",") {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			parsedIds = append(parsedIds, trimmed)
		}
	}

	var (
		url   = "https://music.163.com/weapi/artist/unsub"
		reply ArtistUnsubResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeWEAPI
	weapiReq := &weapiArtistUnsubReq{ArtistIds: parsedIds}
	resp, err := a.client.Request(ctx, url, weapiReq, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

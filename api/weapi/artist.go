// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type ArtistSongsReq struct {
	Id           int64  `json:"id"`            // 歌手id
	PrivateCloud string `json:"private_cloud"` // boolean
	WorkType     int64  `json:"work_type"`     // 通常为1
	Order        string `json:"order"`         // hot,time
	Offset       int64  `json:"offset"`        // 第几页
	Limit        int64  `json:"limit"`         // 每页条数
}

type ArtistSongsResp struct {
	types.RespCommon[any]
	More  bool                   `json:"more"`
	Total int64                  `json:"total"`
	Songs []ArtistSongsRespSongs `json:"songs"`
}

type ArtistSongsRespSongs struct {
	Id              int64          `json:"id"`
	A               any            `json:"a"`
	Al              types.Album    `json:"al"`
	Alia            []string       `json:"alia"`
	Ar              []types.Artist `json:"ar"`
	Cd              string         `json:"cd"`
	Cf              string         `json:"cf"`
	Cp              int64          `json:"cp"`
	Crbt            any            `json:"crbt"`
	DjId            int64          `json:"djId"`
	Dt              int64          `json:"dt"`
	Fee             int64          `json:"fee"`
	Ftype           int64          `json:"ftype"`
	H               *types.Quality `json:"h"`
	Hr              *types.Quality `json:"hr"`
	L               *types.Quality `json:"l"`
	M               *types.Quality `json:"m"`
	Sq              *types.Quality `json:"sq"`
	Mst             int64          `json:"mst"`
	Mv              int64          `json:"mv"`
	Name            string         `json:"name"`
	No              int64          `json:"no"`
	NoCopyrightRcmd any            `json:"noCopyrightRcmd"`
	Pop             float64        `json:"pop"`
	Pst             int64          `json:"pst"`
	Rt              string         `json:"rt"`
	RtUrl           any            `json:"rtUrl"`
	RtUrls          []any          `json:"rtUrls"`
	Rtype           int64          `json:"rtype"`
	Rurl            any            `json:"rurl"`
	SongJumpInfo    any            `json:"songJumpInfo"`
	St              int64          `json:"st"`
	T               int64          `json:"t"`
	V               int64          `json:"v"`
	Tns             []string       `json:"tns,omitempty"`
	Privilege       struct {
		types.Privileges
		Code    int64 `json:"code"`
		Message any   `json:"message"`
	} `json:"privilege"`
}

// ArtistSongs 歌手所有歌曲
// url:
// needLogin:
func (a *Api) ArtistSongs(ctx context.Context, req *ArtistSongsReq) (*ArtistSongsResp, error) {
	var (
		url   = "https://music.163.com/weapi/v1/artist/songs"
		reply ArtistSongsResp
		opts  = api.NewOptions()
	)
	if req.Order == "" {
		req.Order = "hot"
	}
	if req.Limit == 0 {
		req.Limit = 100
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

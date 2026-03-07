package weapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type SongDynamicCoverReq struct {
	SongId string `json:"songId"`
}

type SongDynamicCoverResp struct {
	types.RespCommon[any]
}

func (a *Api) SongDynamicCover(ctx context.Context, req *SongDynamicCoverReq) (*SongDynamicCoverResp, error) {
	var (
		url   = "https://music.163.com/weapi/songplay/dynamic-cover"
		reply SongDynamicCoverResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type SongLyricsMarkReq struct {
	SongId  string `json:"songId"`
	Type    string `json:"type"` // 0: 歌词 1: 翻译
	Version string `json:"version"`
}

type SongLyricsMarkResp struct {
	types.RespCommon[any]
}

func (a *Api) SongLyricsMark(ctx context.Context, req *SongLyricsMarkReq) (*SongLyricsMarkResp, error) {
	var (
		url   = "https://music.163.com/weapi/song/lyrics/mark"
		reply SongLyricsMarkResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

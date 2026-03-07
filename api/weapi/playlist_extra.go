package weapi

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type PlaylistUpdatePlayCountReq struct {
	Id string `json:"id"`
}

type PlaylistUpdatePlayCountResp struct {
	types.RespCommon[any]
}

func (a *Api) PlaylistUpdatePlayCount(ctx context.Context, req *PlaylistUpdatePlayCountReq) (*PlaylistUpdatePlayCountResp, error) {
	var (
		url   = "https://music.163.com/weapi/playlist/update/playcount"
		reply PlaylistUpdatePlayCountResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

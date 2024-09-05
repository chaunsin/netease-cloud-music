package weapi

import (
	"context"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

// LayoutReq .
type LayoutReq struct {
	CsrfToken string `json:"csrf_token"`
}

type LayoutResp struct {
	types.RespCommon[any]
}

// Layout 退出
func (a *Api) Layout(ctx context.Context, req *LayoutReq) (*LayoutResp, error) {
	var (
		url  = "https://music.163.com/weapi/logout"
		resp LayoutResp
		opts = api.NewOptions()
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}
	reply, err := a.client.Request(ctx, url, req, &resp, opts)
	if err != nil {
		return nil, err
	}
	_ = reply
	return &resp, nil
}

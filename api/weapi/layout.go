package weapi

import (
	"context"
	"net/http"

	"github.com/chaunsin/netease-cloud-music/api/types"
)

// LayoutReq .
type LayoutReq struct {
	CsrfToken string `json:"csrf_token"`
}

type LayoutResp struct {
	types.RespCommon[any]
}

// Layout 退出 TODO:未完成
func (a *Api) Layout(ctx context.Context, req *LayoutReq) (*LayoutResp, error) {
	var (
		url  = "https://music.163.com/weapi/feedback/weblog?csrf_token=9f6b902c3c811cd4d9f32ec9544c6747"
		resp LayoutResp
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}
	reply, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &resp)
	if err != nil {
		return nil, err
	}
	_ = reply
	return &resp, nil
}

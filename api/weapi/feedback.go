package weapi

import (
	"context"
	"net/http"

	"github.com/chaunsin/netease-cloud-music/api"
)

type WebLogReqJson struct {
	Plist []struct {
		Oid string `json:"_oid"`
	} `json:"_plist"`
	Elist     []interface{} `json:"_elist"`
	Spm       string        `json:"_spm"`
	Scm       string        `json:"_scm"`
	Duration  string        `json:"duration"` //
	Eventtime int64         `json:"_eventtime"`
	Sessid    string        `json:"_sessid"`
	GDprefer  string        `json:"g_dprefer"`
	IsWebview int           `json:"is_webview"`
}

// WebLogReq .
// [{"action":"_pv","useForRefer":true,"json":{"_plist":[{"_oid":"page_web_register_login"},{"_oid":"page_h5_biz"}],"_elist":[],"_spm":"page_web_register_login|page_h5_biz","_scm":":::|::","_eventtime":1704464373629,"_sessid":"1704464373588#479","g_dprefer":"[F:1][1704464373588#479]","is_webview":1}}]
type WebLogReq struct {
	Action      string        `json:"action"`
	UseForRefer bool          `json:"useForRefer"`
	Json        WebLogReqJson `json:"json"`
}

type WebLogResp struct {
	api.RespCommon
}

// WebLog 登录使用行为
func (a *Api) WebLog(ctx context.Context, req *WebLogReq) (*WebLogResp, error) {
	var (
		url  = "https://music.163.com/weapi/feedback/weblog?csrf_token=9f6b902c3c811cd4d9f32ec9544c6747"
		resp WebLogResp
	)
	reply, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &resp)
	if err != nil {
		return nil, err
	}
	_ = reply
	return &resp, nil
}

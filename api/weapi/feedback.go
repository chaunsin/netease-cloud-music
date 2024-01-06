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
		url  = "https://interface.music.163.com/api/feedback/weblog"
		resp WebLogResp
	)
	reply, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &resp)
	if err != nil {
		return nil, err
	}
	_ = reply
	return &resp, nil
}

// WeApiWebLogReq
//
//	{
//		"logs": "[{\"action\":\"mobile_monitor\",\"json\":{\"meta._ver\":2,\"meta._dataName\":\"pip_lyric_monitor\",\"action\":\"impress\",\"userAgent\":\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36\",\"chromeVersion\":120,\"mainsite\":\"1\"}}]",
//		"csrf_token": "9f6b902c3c811cd4d9f32ec9544c6747"
//	}
type WeApiWebLogReq struct {
	Logs      string `json:"logs"`
	CsrfToken string `json:"csrf_token"` // 可不用传递
}

type WeApiWebLogReqLog struct {
	Action string `json:"action"`
	Json   struct {
		MetaVer       int    `json:"meta._ver"`
		MetaDataName  string `json:"meta._dataName"`
		Action        string `json:"action"`
		UserAgent     string `json:"userAgent"`
		ChromeVersion int    `json:"chromeVersion"`
		MainSite      string `json:"mainsite"`
	} `json:"json"` // 此值为动态值考虑使用map
}

type WeApiWebLogResp struct {
	api.RespCommon
}

// WeApiWebLog 登录使用行为
func (a *Api) WeApiWebLog(ctx context.Context, req *WeApiWebLogReq) (*WeApiWebLogResp, error) {
	var (
		url  = "https://music.163.com/weapi/feedback/weblog?csrf_token=9f6b902c3c811cd4d9f32ec9544c6747"
		resp WeApiWebLogResp
	)
	reply, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &resp)
	if err != nil {
		return nil, err
	}
	_ = reply
	return &resp, nil
}

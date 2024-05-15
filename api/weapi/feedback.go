package weapi

import (
	"context"
	"net/http"

	"github.com/chaunsin/netease-cloud-music/api/types"
)

type ApiWebLogReqJson struct {
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
type ApiWebLogReq struct {
	CsrfToken   string           `json:"csrf_token"`
	Action      string           `json:"action"`
	UseForRefer bool             `json:"useForRefer"`
	Json        ApiWebLogReqJson `json:"json"`
}

type ApiWebLogResp struct {
	types.RespCommon[any]
}

// ApiWebLog 日志上报
// 目前已知使用场景
// 1. 登录使用行为
func (a *Api) ApiWebLog(ctx context.Context, req *ApiWebLogReq) (*ApiWebLogResp, error) {
	var (
		url  = "https://interface.music.163.com/api/feedback/weblog"
		resp ApiWebLogResp
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

// WeApiWebLogReq
//
//	{
//		"logs": "[{\"action\":\"mobile_monitor\",\"json\":{\"meta._ver\":2,\"meta._dataName\":\"pip_lyric_monitor\",\"action\":\"impress\",\"userAgent\":\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36\",\"chromeVersion\":120,\"mainsite\":\"1\"}}]",
//		"csrf_token": "9f6b902c3c811cd4d9f32ec9544c6747"
//	}
type WeApiWebLogReq struct {
	CsrfToken string `json:"csrf_token"` // 可不用传递
	Logs      string `json:"logs"`       // WeApiWebLogReqLog
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
	types.RespCommon[any]
}

// WeApiWebLog 日志上报
// 目前已经使用场景
// 1. 登录使用行为
func (a *Api) WeApiWebLog(ctx context.Context, req *WeApiWebLogReq) (*WeApiWebLogResp, error) {
	var (
		url  = "https://music.163.com/weapi/feedback/weblog?csrf_token=9f6b902c3c811cd4d9f32ec9544c6747"
		resp WeApiWebLogResp
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

type InterfaceWeApiWebLogReq struct {
	CsrfToken string `json:"csrf_token"` // 可不用传递
	Logs      string `json:"logs"`       // InterfaceWeApiWebLogReqLog
}

type InterfaceWeApiWebLogReqLog struct {
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

type InterfaceWeApiWebLogResp struct {
	types.RespCommon[any]
}

// InterfaceWeApiWebLog 日志上报
// 目前已知使用场景
// 1. mac通过消息中心跳转到音乐合伙人功能时，音乐合伙人相关功能日志会上报
func (a *Api) InterfaceWeApiWebLog(ctx context.Context, req *InterfaceWeApiWebLogReq) (*InterfaceWeApiWebLogResp, error) {
	var (
		url  = "https://interface.music.163.com/weapi/feedback/weblog?csrf_token=21c5adf8ad859322cbfa3180d829f8ec"
		resp InterfaceWeApiWebLogResp
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

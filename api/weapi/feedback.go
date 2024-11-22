package weapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type ApiWebLogReqJson struct {
	Plist []struct {
		Oid string `json:"_oid"`
	} `json:"_plist"`
	Elist     []interface{} `json:"_elist"`
	Spm       string        `json:"_spm"`
	Scm       string        `json:"_scm"`
	Duration  string        `json:"duration"`
	Eventtime int64         `json:"_eventtime"`
	Sessid    string        `json:"_sessid"`
	GDprefer  string        `json:"g_dprefer"`
	IsWebview int64         `json:"is_webview"`
}

// ApiWebLogReq .
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

// WebLogReq
// *** 以下日志是针对mac pc网页端日志分析,由于日志类型太多不能穷举各种日志,而且很多日志无需关心 分析时间:2024-06-24 ***
// 1.应该是准备开始播放事件
// "[{"action":"startplay","json":{"id":1984580503,"type":"song","content":"id=1981392816","mainsite":"1"}}]"
// 2.开始播放事件
// "[{"action":"play","json":{"id":"1984580503","type":"song","source":"list","sourceid":"1981392816","mainsite":"1","content":"id=1981392816"}}]"
// 3.播放完成事件
// "[{"action":"play","json":{"type":"song","wifi":0,"download":0,"id":1984580503,"time":199,"end":"ui","source":"list","sourceId":"1981392816","mainsite":"1","content":"id=1981392816"}}]"
// 其中歌曲播放一半切歌之后的日志时间为
// "[{"action":"play","json":{"type":"song","wifi":0,"download":0,"id":2600804126,"time":22,"end":"interrupt","source":"toplist","sourceId":"19723756","mainsite":"1","content":"id=1981392816"}}]"
// 4.未知 在播放音乐时间隔一定时间会上传此日志
// "[{"action":"impress","json":{"mspm":"619df35ce51b6b383f5fafdb","page":"mainpage","module":"nav_bar","target":"friends","reddot":"1","mainsite":"1"}}]"
// 5.未知 当一首歌曲播放完之后会产生此日志
// "[{"action":"sysaction","json":{"dataType":"cdnCompare","cdnType":"NetEase","loadeddataTime":41716,"resourceType":"audiom4a","resourceId":1984580503,"resourceUrl":"https://m804.music.126.net/20240624165706/16fab8b9a63e70c89f75de59635a784b/jdyyaac/obj/w5rDlsOJwrLDjj7CmsOj/19576100567/d33e/c04b/ac0d/829119824fad1696351f7e0898dd266a.m4a?authSecret=00000190495fb0bc11120a3b1e596978","xySupport":true,"error":false,"errorType":"","ua":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36","mainsite":"1"}}]"
// 6.未知
// "[{\"action\":\"mobile_monitor\",\"json\":{\"meta._ver\":2,\"meta._dataName\":\"pip_lyric_monitor\",\"action\":\"impress\",\"userAgent\":\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36\",\"chromeVersion\":120,\"mainsite\":\"1\"}}]",
type WebLogReq struct {
	CsrfToken string                   `json:"csrf_token"` // 可不用传递
	Logs      []map[string]interface{} `json:"logs"`       // 具体事件内容
}

type webLogReq struct {
	CsrfToken string `json:"csrf_token"` // 可不用传递
	Logs      string `json:"logs"`       // WebLogReqLog
}

type WebLogResp struct {
	types.RespCommon[string]
}

// WebLog 日志上报
func (a *Api) WebLog(ctx context.Context, req *WebLogReq) (*WebLogResp, error) {
	var (
		url  = "https://music.163.com/weapi/feedback/weblog"
		resp WebLogResp
		opts = api.NewOptions()
	)
	if req.CsrfToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CsrfToken = csrf
	}

	data, err := json.Marshal(req.Logs)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal(req.Logs) error: %v", err)
	}
	var request = &webLogReq{
		CsrfToken: req.CsrfToken,
		Logs:      string(data),
	}

	reply, err := a.client.Request(ctx, url, &request, &resp, opts)
	if err != nil {
		return nil, err
	}
	_ = reply
	return &resp, nil
}

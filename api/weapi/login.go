// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package weapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/chaunsin/netease-cloud-music/api"

	"github.com/skip2/go-qrcode"
)

type QrcodeCreateKeyReq struct {
	Type      int64  `json:"type"`
	CsrfToken string `json:"csrf_token"`
}

type QrcodeCreateKeyResp struct {
	api.RespCommon
	UniKey string `json:"unikey"`
}

// QrcodeCreateKey 生成二维码需要得key
// 常见问题
// 1. 请求成功了,但是body为空值什么也没有,原因还是参数加密出现了问题。
// 2. crsftoken 可传可不传个人猜测前端写得通用框架传了
func (a *Api) QrcodeCreateKey(ctx context.Context, req *QrcodeCreateKeyReq) (*QrcodeCreateKeyResp, error) {
	var (
		url   = "https://music.163.com/weapi/login/qrcode/unikey"
		reply QrcodeCreateKeyResp
	)

	// data, err := WeApiEncrypt(req)
	// if err != nil {
	// 	return nil, fmt.Errorf("EApiEncrypt: %w", err)
	// }
	// // data := map[string]string{
	// // 	"params":    "4Y5vIViLy9xfHuTtGvZpklB5xPbCdAFNx6Ua+xbOqCmDSKpqaToPWyHj2pvZ+Mnfg3MPOSbuuxkneY0zSo6AfsVPKVOpNbhwJCNZaiFrcKaY0yQ04fzgj3Ia5fd5G0Oljh49edwI6gNoKY38S+8Ytg==",
	// // 	"encSecKey": "33b63ddf73a7b7f15da04b2f5f1dbfcaac34ab0dec3ecbba2071e56dd631606ba57f91c2b390c7fa89ae32629b546b9e6adacfa58811640a3a7f134ef7c7be0375f0408e5643b789e84f03caf45779d8da34f721b7f1cfe52a5a9f7827affacccecc3b2b8346fe0ee82763310fef149fb5564ee0336421a4e72d8f13fe3f3484",
	// // }
	// fmt.Printf("data: %+v\nencriypt: %+v\n", req, data)
	//
	// resp, err := c.cli.R().
	// 	SetContext(ctx).
	// 	SetHeader("Host", "music.163.com").
	// 	SetHeader("Connection", "keep-alive").
	// 	SetHeader("Accept", "*/*").
	// 	SetHeader("Accept-Encoding", "gzip, deflate, br").
	// 	SetHeader("Content-Type", "application/x-www-form-urlencoded").
	// 	SetHeader("Accept-language", "zh-CN,zh-Hans;q=0.9").
	// 	SetHeader("Referer", "https://music.163.com").
	// 	SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034").
	// 	// SetHeader("sec-ch-ua-mobile", "?0").
	// 	// SetHeader("sec-ch-ua-platform", "\"macOS\"").
	// 	// SetHeader("sec-fetch-site", "same-origin").
	// 	// SetHeader("sec-fetch-mode", "cors").
	// 	// SetHeader("sec-fetch-dest", "empty").
	// 	// SetCookie(&http.Cookie{Name: "NMTID", Value: "00OoHY0iWG6k2yHgkeDtChanlm4m80AAAGMJOAR6g", Path: "/", Domain: "music.163.com", MaxAge: 315360000}).
	// 	// SetCookie(&http.Cookie{Name: "__remember_me", Value: "true"}).
	// 	// SetCookie(&http.Cookie{Name: "__csrf", Value: "9f6b902c3c811cd4d9f32ec9544c6747"}).
	// 	// SetCookie(&http.Cookie{Name: "_ntes_nnid", Value: "4aa19aa783c710f8cea394f9b40ecfa0,1703339184278"}).
	// 	// SetCookie(&http.Cookie{Name: "_ntes_nnid", Value: "4aa19aa783c710f8cea394f9b40ecfa0"}).
	// 	// SetCookie(&http.Cookie{Name: "MUSIC_U", Value: "005F7E15781DFD33490CAC7976D67289BBB5038FFAD001E1E7E20BD626A7EFB2CCE28772906C1BA06726DC7F61A0AF8A928203854164105329183B0680A8A3C6C1A2A883B977F4CD4DA149226C247C59AF1DD96A2F7F66CD731A92942BF38669CAB9AD9D2054FD9B9D5BD2839E4D2F66D9C41C58697FD6E54FE07791EEEB88B9CDCF9CBF20A947B901958C758C581CBD08C8EDC44EFFD923E13E743F733B7C870BCB912CFDDFEA2072CBB888992F255835A256025BAC9B99EE2E3942922246F24673DA6C1392F9571AFCB5AC7DFAC7BA875D306B83C3A9ED1F4BFE9DD26569EF3369F4266F85137C3F1C3024EC02F959227AE1212D445C58A8448EA0D8B0CE01D50BBA1051947FFDD0555AC2C270CE03297347FE54A83DFB1E41F617E1CCB0ED989CA8A9BFEAF78CE5990447FD20370E220E5BD0D908F82386C0691E0167E3E14E"}).
	// 	// SetCookie(&http.Cookie{Name: "__csrf", Value: "7e5b7084a7572375487ce2bc3bff9bad"}).
	// 	// SetCookie(&http.Cookie{Name: "WEVNSM", Value: "1.0.0"}).
	// 	// SetCookie(&http.Cookie{Name: "WNMCID", Value: "molntu.1703339249350.01.0"}).
	// 	// SetCookie(&http.Cookie{Name: "WM_TID", Value: "KJDGXolnU%2FJFEVUFFFeUW6yKLNnTaOk%2F"}).
	// 	// SetCookie(&http.Cookie{Name: "ntes_utid", Value: "tid._.pvLEa8mD3YxABkBQBRfUxD4Vep5PCyma._.0"}).
	// 	// SetCookie(&http.Cookie{Name: "sDeviceId", Value: "YD-XOLQbJSfPgJBAwQAQULEKCPB8TZpQO%2FR"}).
	// 	// SetCookie(&http.Cookie{Name: "ntes_kaola_ad", Value: "1"}).
	// 	// SetCookie(&http.Cookie{Name: "mp_versions_hubble_jsSDK", Value: "DATracker.globals.1.6.14"}).
	// 	// SetCookie(&http.Cookie{Name: "__root_domain_v", Value: ".163.com"}).
	// 	// SetCookie(&http.Cookie{Name: "_qddaz", Value: "QD.826104043793660"}).
	// 	// SetCookie(&http.Cookie{Name: "wyy_uid", Value: "a70c327c-43d6-4429-b58c-908ba153399b"}).
	// 	// SetCookie(&http.Cookie{Name: "hb_MA-91DF-2127272A00D5_source", Value: "dun.163.com"}).
	// 	// SetCookie(&http.Cookie{Name: "_gcl_au", Value: "1.1.1997216281.1704043848"}).
	// 	// SetCookie(&http.Cookie{Name: "urs_u", Value: "-vwgTRum9Tpq9JwNGzpODQ7kYwQUnzlzeg0ibPb6Z8u30-Ys4c1TMZQPbuCnSXT1rMi2smUGOPnedH8cLtQEpAauw1lhkpSrRJwsQKR60-kAJMHhf57VmCKqa0BNFHTWbhN/DNznoSwHtgiisXkOPNVc3T066hUWEtpvXPLUHUKCnhdjV0o/kuQnTSMB5lz33z1SU2Hy56sfVeaxskazS0GXTpSFp12SBQuJOuC9gNHE/wsCVF5CNg/SqWLRPPGh2B051aOSt99fXt5QpoP0F3LZi6NzxKbbJCl1hSnDV15SrZl3rA-fjJ4a0Opy61jP"}).
	// 	// SetCookie(&http.Cookie{Name: "hb_MA-93D5-9AD06EA4329A_source", Value: "dun.163.com"}).
	// 	// SetCookie(&http.Cookie{Name: "JSESSIONID-WYYY", Value: "Ey2oO%2Fu1Z9KpdHsfpTVt%5C34PkF7khpRgcUuEVH6xCVpkEHgNcmSFPMNq28D2vUA7%2F9lvGfm13nfo%2FP0qjkQTrTgHCp%2FZtSTYEJynHPB7d8UIwKjv%2B%5Cmhgn1xoa4hPc4GVv9jA1ZlhxtfFX%2B2PfcHjiKvKpHUpkp8P6VGpp4bJe%2BXTixR%3A1704093215301"}).
	// 	// SetCookie(&http.Cookie{Name: "_iuqxldmzr_", Value: "32"}).
	// 	// SetCookie(&http.Cookie{Name: "WM_NI", Value: "Y0cwY8B6sLkOv6r3FO4BAZXjqowvFbWmtlNrjtmL4%2Fv2yI49ANHNsbr1o%2B6noPW%2FSg%2BKoG3OzJ5%2FGjkGiRWieX8jbMlIo2%2BTO%2BuNoXKgpCCONazihLS33NWvSnNa9PMJS20%3D"}).
	// 	// SetCookie(&http.Cookie{Name: "WM_NIKE", Value: "9ca17ae2e6ffcda170e2e6eeaffb609a9d96afe979939a8ba6d84b979a8b82c1408d9aa0d6d579b7effa99bb2af0fea7c3b92a9bf0ad98dc3482eabdadea45b6eabed4c545f89e8fbbdb62b3b1b997c56a95b3bc8dcd7aaaaca3adb725b6eeb7aab733839288aff26ab2efa1ccb566ad9099d0d26de995a1a8b245ba96fbd0c662a999a7b3d663a788f88be147b097a1d3aa608df5bbafee5489b9c0a4c2458ebfb68aeb4891edfdafd67e91b28282bc5c8f969dd3b337e2a3"}).
	// 	// SetCookie(&http.Cookie{Name: "channel", Value: "\"h=yd&t=yd&i18nEnable=true&locale=zh_CN&referrer=https%3A%2F%2Fdun.163.com%2Fdashboard&fromyd=baiduP_PP_PP664\""}).
	// 	SetFormData(data).
	// 	Post("https://music.163.com" + url)
	// if err != nil {
	// 	return nil, fmt.Errorf("post: %w", err)
	// }
	//
	// fmt.Printf("response: %+v\n", string(resp.Body()))
	// if err := json.Unmarshal(resp.Body(), &reply); err != nil {
	// 	return nil, err
	// }
	// if resp.StatusCode() != http.StatusOK {
	// 	return nil, fmt.Errorf("http status code: %d", resp.StatusCode())
	// }

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type QrcodeGenerateReq struct {
	CodeKey string
}

type QrcodeGenerateResp struct {
	api.RespCommon
}

// QrcodeGenerate 根据生成得key 请求生成二维码
func (a *Api) QrcodeGenerate(ctx context.Context, req *QrcodeGenerateReq) (*QrcodeGenerateResp, error) {
	var (
		content = fmt.Sprintf("https://music.163.com/login?codekey=%s", req.CodeKey)
		reply   QrcodeGenerateResp
	)
	if err := qrcode.WriteFile(content, qrcode.Medium, 256, "./qrcode.png"); err != nil {
		return nil, fmt.Errorf("WriteFile: %w", err)
	}
	return &reply, nil
}

type QrcodeCheckReq struct {
	Key  string `json:"key"`  // QrcodeCreateKey()返回值codekey
	Type int64  `json:"type"` // 目前传1
}

type QrcodeCheckResp struct {
	api.RespCommon
}

// QrcodeCheck 查询扫码状态
// 返回值:
// 800-二维码不存在或已过期
// 801-等待扫码
// 802-正在扫码授权中
// 803-授权登录成功
func (a *Api) QrcodeCheck(ctx context.Context, req *QrcodeCheckReq) (*QrcodeCheckResp, error) {
	var (
		url   = "https://music.163.com/weapi/login/qrcode/client/login"
		reply QrcodeCheckResp
	)

	// data, err := WeApiEncrypt(req)
	// if err != nil {
	// 	return nil, fmt.Errorf("EApiEncrypt: %w", err)
	// }
	// // data := map[string]string{
	// // 	"params":    "4Y5vIViLy9xfHuTtGvZpklB5xPbCdAFNx6Ua+xbOqCmDSKpqaToPWyHj2pvZ+Mnfg3MPOSbuuxkneY0zSo6AfsVPKVOpNbhwJCNZaiFrcKaY0yQ04fzgj3Ia5fd5G0Oljh49edwI6gNoKY38S+8Ytg==",
	// // 	"encSecKey": "33b63ddf73a7b7f15da04b2f5f1dbfcaac34ab0dec3ecbba2071e56dd631606ba57f91c2b390c7fa89ae32629b546b9e6adacfa58811640a3a7f134ef7c7be0375f0408e5643b789e84f03caf45779d8da34f721b7f1cfe52a5a9f7827affacccecc3b2b8346fe0ee82763310fef149fb5564ee0336421a4e72d8f13fe3f3484",
	// // }
	// fmt.Printf("data: %+v\nencriypt: %+v\n", req, data)
	//
	// resp, err := c.cli.R().
	// 	SetContext(ctx).
	// 	SetHeader("Host", "music.163.com").
	// 	SetHeader("Connection", "keep-alive").
	// 	SetHeader("Accept", "*/*").
	// 	SetHeader("Accept-Encoding", "gzip, deflate, br").
	// 	SetHeader("Content-Type", "application/x-www-form-urlencoded").
	// 	SetHeader("Accept-language", "zh-CN,zh-Hans;q=0.9").
	// 	SetHeader("Referer", "https://music.163.com").
	// 	SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034").
	// 	// SetHeader("sec-ch-ua-mobile", "?0").
	// 	// SetHeader("sec-ch-ua-platform", "\"macOS\"").
	// 	// SetHeader("sec-fetch-site", "same-origin").
	// 	// SetHeader("sec-fetch-mode", "cors").
	// 	// SetHeader("sec-fetch-dest", "empty").
	// 	// SetCookie(&http.Cookie{Name: "NMTID", Value: "00OoHY0iWG6k2yHgkeDtChanlm4m80AAAGMJOAR6g", Path: "/", Domain: "music.163.com", MaxAge: 315360000}).
	// 	// SetCookie(&http.Cookie{Name: "__remember_me", Value: "true"}).
	// 	// SetCookie(&http.Cookie{Name: "__csrf", Value: "9f6b902c3c811cd4d9f32ec9544c6747"}).
	// 	// SetCookie(&http.Cookie{Name: "_ntes_nnid", Value: "4aa19aa783c710f8cea394f9b40ecfa0,1703339184278"}).
	// 	// SetCookie(&http.Cookie{Name: "_ntes_nnid", Value: "4aa19aa783c710f8cea394f9b40ecfa0"}).
	// 	// SetCookie(&http.Cookie{Name: "MUSIC_U", Value: "005F7E15781DFD33490CAC7976D67289BBB5038FFAD001E1E7E20BD626A7EFB2CCE28772906C1BA06726DC7F61A0AF8A928203854164105329183B0680A8A3C6C1A2A883B977F4CD4DA149226C247C59AF1DD96A2F7F66CD731A92942BF38669CAB9AD9D2054FD9B9D5BD2839E4D2F66D9C41C58697FD6E54FE07791EEEB88B9CDCF9CBF20A947B901958C758C581CBD08C8EDC44EFFD923E13E743F733B7C870BCB912CFDDFEA2072CBB888992F255835A256025BAC9B99EE2E3942922246F24673DA6C1392F9571AFCB5AC7DFAC7BA875D306B83C3A9ED1F4BFE9DD26569EF3369F4266F85137C3F1C3024EC02F959227AE1212D445C58A8448EA0D8B0CE01D50BBA1051947FFDD0555AC2C270CE03297347FE54A83DFB1E41F617E1CCB0ED989CA8A9BFEAF78CE5990447FD20370E220E5BD0D908F82386C0691E0167E3E14E"}).
	// 	// SetCookie(&http.Cookie{Name: "__csrf", Value: "7e5b7084a7572375487ce2bc3bff9bad"}).
	// 	// SetCookie(&http.Cookie{Name: "WEVNSM", Value: "1.0.0"}).
	// 	// SetCookie(&http.Cookie{Name: "WNMCID", Value: "molntu.1703339249350.01.0"}).
	// 	// SetCookie(&http.Cookie{Name: "WM_TID", Value: "KJDGXolnU%2FJFEVUFFFeUW6yKLNnTaOk%2F"}).
	// 	// SetCookie(&http.Cookie{Name: "ntes_utid", Value: "tid._.pvLEa8mD3YxABkBQBRfUxD4Vep5PCyma._.0"}).
	// 	// SetCookie(&http.Cookie{Name: "sDeviceId", Value: "YD-XOLQbJSfPgJBAwQAQULEKCPB8TZpQO%2FR"}).
	// 	// SetCookie(&http.Cookie{Name: "ntes_kaola_ad", Value: "1"}).
	// 	// SetCookie(&http.Cookie{Name: "mp_versions_hubble_jsSDK", Value: "DATracker.globals.1.6.14"}).
	// 	// SetCookie(&http.Cookie{Name: "__root_domain_v", Value: ".163.com"}).
	// 	// SetCookie(&http.Cookie{Name: "_qddaz", Value: "QD.826104043793660"}).
	// 	// SetCookie(&http.Cookie{Name: "wyy_uid", Value: "a70c327c-43d6-4429-b58c-908ba153399b"}).
	// 	// SetCookie(&http.Cookie{Name: "hb_MA-91DF-2127272A00D5_source", Value: "dun.163.com"}).
	// 	// SetCookie(&http.Cookie{Name: "_gcl_au", Value: "1.1.1997216281.1704043848"}).
	// 	// SetCookie(&http.Cookie{Name: "urs_u", Value: "-vwgTRum9Tpq9JwNGzpODQ7kYwQUnzlzeg0ibPb6Z8u30-Ys4c1TMZQPbuCnSXT1rMi2smUGOPnedH8cLtQEpAauw1lhkpSrRJwsQKR60-kAJMHhf57VmCKqa0BNFHTWbhN/DNznoSwHtgiisXkOPNVc3T066hUWEtpvXPLUHUKCnhdjV0o/kuQnTSMB5lz33z1SU2Hy56sfVeaxskazS0GXTpSFp12SBQuJOuC9gNHE/wsCVF5CNg/SqWLRPPGh2B051aOSt99fXt5QpoP0F3LZi6NzxKbbJCl1hSnDV15SrZl3rA-fjJ4a0Opy61jP"}).
	// 	// SetCookie(&http.Cookie{Name: "hb_MA-93D5-9AD06EA4329A_source", Value: "dun.163.com"}).
	// 	// SetCookie(&http.Cookie{Name: "JSESSIONID-WYYY", Value: "Ey2oO%2Fu1Z9KpdHsfpTVt%5C34PkF7khpRgcUuEVH6xCVpkEHgNcmSFPMNq28D2vUA7%2F9lvGfm13nfo%2FP0qjkQTrTgHCp%2FZtSTYEJynHPB7d8UIwKjv%2B%5Cmhgn1xoa4hPc4GVv9jA1ZlhxtfFX%2B2PfcHjiKvKpHUpkp8P6VGpp4bJe%2BXTixR%3A1704093215301"}).
	// 	// SetCookie(&http.Cookie{Name: "_iuqxldmzr_", Value: "32"}).
	// 	// SetCookie(&http.Cookie{Name: "WM_NI", Value: "Y0cwY8B6sLkOv6r3FO4BAZXjqowvFbWmtlNrjtmL4%2Fv2yI49ANHNsbr1o%2B6noPW%2FSg%2BKoG3OzJ5%2FGjkGiRWieX8jbMlIo2%2BTO%2BuNoXKgpCCONazihLS33NWvSnNa9PMJS20%3D"}).
	// 	// SetCookie(&http.Cookie{Name: "WM_NIKE", Value: "9ca17ae2e6ffcda170e2e6eeaffb609a9d96afe979939a8ba6d84b979a8b82c1408d9aa0d6d579b7effa99bb2af0fea7c3b92a9bf0ad98dc3482eabdadea45b6eabed4c545f89e8fbbdb62b3b1b997c56a95b3bc8dcd7aaaaca3adb725b6eeb7aab733839288aff26ab2efa1ccb566ad9099d0d26de995a1a8b245ba96fbd0c662a999a7b3d663a788f88be147b097a1d3aa608df5bbafee5489b9c0a4c2458ebfb68aeb4891edfdafd67e91b28282bc5c8f969dd3b337e2a3"}).
	// 	// SetCookie(&http.Cookie{Name: "channel", Value: "\"h=yd&t=yd&i18nEnable=true&locale=zh_CN&referrer=https%3A%2F%2Fdun.163.com%2Fdashboard&fromyd=baiduP_PP_PP664\""}).
	// 	SetFormData(data).
	// 	Post("https://music.163.com" + url)
	// if err != nil {
	// 	return nil, fmt.Errorf("post: %w", err)
	// }
	//
	// fmt.Printf("response: %+v\n", string(resp.Body()))
	// if err := json.Unmarshal(resp.Body(), &reply); err != nil {
	// 	return nil, err
	// }
	// if resp.StatusCode() != http.StatusOK {
	// 	return nil, fmt.Errorf("http status code: %d", resp.StatusCode())
	// }

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type GetUserInfoReq struct {
	CSRFToken string `json:"csrf_token"`
}

type GetUserInfoResp struct {
	api.RespCommon
}

func (a *Api) GetUserInfo(ctx context.Context, req *GetUserInfoReq) (*GetUserInfoResp, error) {
	var (
		url   = "https://music.163.com/weapi/w/nuser/account/get"
		reply GetUserInfoResp
	)
	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

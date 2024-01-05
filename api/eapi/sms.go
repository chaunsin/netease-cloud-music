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

package eapi

import (
	"context"

	"github.com/chaunsin/netease-cloud-music/api"
)

type CaptchaSendReq struct {
	Phone  string
	CTCode string
}

type CaptchaSendResp struct {
	api.RespCommon
}

// CaptchaSend 发送验证码 PC客户端
func (a *Api) CaptchaSend(ctx context.Context, req *CaptchaSendReq) (*CaptchaSendResp, error) {
	// var (
	// 	reply      CaptchaSendResp
	// 	url        = "/eapi/sms/captcha/sent"
	// 	sendSMSReq = SendSMSReq{
	// 		CtCode:    req.CTCode,
	// 		Cellphone: req.Phone,
	// 		DeviceId:  "4cdb39bf34a848781b89663e1e546b8b",
	// 		Os:        "OSX",
	// 		VerifyId:  1,
	// 		ER:        true,
	// 	}
	// 	header = SendSMSReqHeader{
	// 		Os:       "osx",
	// 		AppVer:   "2.3.17",
	// 		DeviceId: "7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62%7C5FD718A3-0602-4389-B612-EBEFAA7F108B",
	// 		// RequestId:     "93487028",
	// 		ClientSign:    "",
	// 		OsVer:         "%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89",
	// 		NmGCoreStatus: "1",
	// 		MConfigInfo:   `{"IuRPVVmc3WWul9fT":{"version":143360,"appver":"2.3.17"}}`,
	// 		MGProductName: "music",
	// 	}
	// )
	// headerByte, err := json.Marshal(header)
	// if err != nil {
	// 	return nil, err
	// }
	// sendSMSReq.Header = string(headerByte)
	// // data, err := json.Marshal(sendSMSReq)
	// // if err != nil {
	// // 	return nil, err
	// // }
	// data, err := EApiEncrypt(url, sendSMSReq)
	// if err != nil {
	// 	return nil, fmt.Errorf("EApiEncrypt: %w", err)
	// }
	// fmt.Printf("data: %+v\nencriypt: %+v\n", sendSMSReq, data)
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
	// 	SetHeader("mg-product-name", "music").
	// 	SetHeader("nm-gcore-status", "1").
	// 	// SetHeader("X-Real-IP", "120.245.4.63").
	// 	// SetHeader("X-Forwarded-For", "120.245.4.63").
	// 	SetFormData(data).
	// 	// SetBody(data).
	// 	// SetFormDataFromValues(v).
	// 	// SetQueryParam("csrf_token", "64573858135942e6d9310d7bfb2f0b21").
	// 	// SetResult(&reply).
	// 	SetCookie(&http.Cookie{Name: "os", Value: "osx"}).
	// 	SetCookie(&http.Cookie{Name: "channel", Value: "netease"}).
	// 	SetCookie(&http.Cookie{Name: "osver", Value: "%E7%89%88%E6%9C%AC12.6%EF%BC%88%E7%89%88%E5%8F%B721G115%EF%BC%89"}).
	// 	Post("https://music.163.com" + url)
	// if err != nil {
	// 	return nil, fmt.Errorf("post: %w", err)
	// }
	//
	// raw, err := EApiDecrypt(string(resp.Body()))
	// if err != nil {
	// 	fmt.Println("EApiDecrypt:", err)
	// }
	// fmt.Printf("raw: %+v\n", string(raw))
	// if err := json.Unmarshal(raw, &reply); err != nil {
	// 	return nil, err
	// }
	// fmt.Printf("response: %+v\n", reply)
	// if resp.StatusCode() != http.StatusOK {
	// 	return nil, fmt.Errorf("http status code: %d", resp.StatusCode())
	// }
	// return &reply, nil
	return nil, nil
}

type CaptchaVerifyReq struct {
	Phone   string `json:"phone"`
	CTCode  string `json:"ctcode"`
	Captcha string `json:"captcha"`
}

type CaptchaVerifyResp struct {
	api.RespCommon
}

// CaptchaVerify 验证验证码
func (a *Api) CaptchaVerify(ctx context.Context, req *CaptchaVerifyReq) (*CaptchaVerifyResp, error) {
	// var (
	// 	reply CaptchaVerifyResp
	// 	url   = "/weapi/captcha/verify"
	// )
	//
	// data, err := WeApiEncrypt(req)
	// if err != nil {
	// 	return nil, fmt.Errorf("EApiEncrypt: %w", err)
	// }
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
	// 	SetCookie(&http.Cookie{
	// 		Name:   "NMTID",
	// 		Value:  "00ONsg1HuozhdMziktUgfiiQ44NwAwAAAGMxDrXt",
	// 		Path:   "/",
	// 		Domain: "music.163.com",
	// 		MaxAge: 315360000,
	// 	}).
	// 	SetFormData(data).
	// 	Post("https://music.163.com" + url)
	// if err != nil {
	// 	return nil, fmt.Errorf("post: %w", err)
	// }
	//
	// fmt.Printf("response: %+v\n", string(resp.Body()))
	// if resp.StatusCode() != http.StatusOK {
	// 	return nil, fmt.Errorf("http status code: %d", resp.StatusCode())
	// }
	// return &reply, nil
	return nil, nil
}

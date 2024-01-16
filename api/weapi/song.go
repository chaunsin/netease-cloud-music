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
)

type SongPlayerReq struct {
	api.ReqCommon
}

type SongPlayerResp struct {
	api.RespCommon[SongPlayerReqData]
}

type SongPlayerReqData struct {
	Id                 int         `json:"id"`
	Url                string      `json:"url"`
	Br                 int         `json:"br"`
	Size               int         `json:"size"`
	Md5                string      `json:"md5"`
	Code               int         `json:"code"`
	Expi               int         `json:"expi"`
	Type               string      `json:"type"`
	Gain               float64     `json:"gain"`
	Peak               float64     `json:"peak"`
	Fee                int         `json:"fee"`
	Uf                 interface{} `json:"uf"`
	Payed              int         `json:"payed"`
	Flag               int         `json:"flag"`
	CanExtend          bool        `json:"canExtend"`
	FreeTrialInfo      interface{} `json:"freeTrialInfo"`
	Level              string      `json:"level"`
	EncodeType         string      `json:"encodeType"`
	ChannelLayout      interface{} `json:"channelLayout"`
	FreeTrialPrivilege struct {
		ResConsumable      bool        `json:"resConsumable"`
		UserConsumable     bool        `json:"userConsumable"`
		ListenType         interface{} `json:"listenType"`
		CannotListenReason interface{} `json:"cannotListenReason"`
		PlayReason         interface{} `json:"playReason"`
	} `json:"freeTrialPrivilege"`
	FreeTimeTrialPrivilege struct {
		ResConsumable  bool `json:"resConsumable"`
		UserConsumable bool `json:"userConsumable"`
		Type           int  `json:"type"`
		RemainTime     int  `json:"remainTime"`
	} `json:"freeTimeTrialPrivilege"`
	UrlSource   int         `json:"urlSource"`
	RightSource int         `json:"rightSource"`
	PodcastCtrp interface{} `json:"podcastCtrp"`
	EffectTypes interface{} `json:"effectTypes"`
	Time        int         `json:"time"`
}

// SongPlayer 音乐播放详情
func (a *Api) SongPlayer(ctx context.Context, req *SongPlayerReq) (*SongPlayerResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/song/enhance/player/url"
		reply SongPlayerResp
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

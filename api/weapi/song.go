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

	"github.com/chaunsin/netease-cloud-music/api/types"
)

// SongPlayerReq
//
//	{
//	   "ids": "[1955097630]",
//	   "br": "128000",
//	   "csrf_token": "77bf3a5074699038504234d63d68d917"
//	}
type SongPlayerReq struct {
	types.ReqCommon
	Ids types.IntsString `json:"ids"` // 歌曲id
	Br  string           `json:"br"`  // 音乐bit率 例如:128000 320000
}

// SongPlayerResp
//
//	{
//	   "data": [
//	       {
//	           "id": 1955097630,
//	           "url": "http://m804.music.126.net/20240517003128/e80a8269f8e418f11fd349420dcf42e6/jdymusic/obj/wo3DlMOGwrbDjj7DisKw/14968401923/7c8d/4357/dc0e/50023048ed42819c67acaec403d832fe.mp3?authSecret=0000018f8227a7c702350aaba39a935b",
//	           "br": 128000,
//	           "size": 4265133,
//	           "md5": "50023048ed42819c67acaec403d832fe",
//	           "code": 200,
//	           "expi": 1200,
//	           "type": "mp3",
//	           "gain": 0,
//	           "peak": 0,
//	           "fee": 0,
//	           "uf": null,
//	           "payed": 0,
//	           "flag": 1,
//	           "canExtend": false,
//	           "freeTrialInfo": null,
//	           "level": "standard",
//	           "encodeType": "mp3",
//	           "channelLayout": null,
//	           "freeTrialPrivilege": {
//	               "resConsumable": false,
//	               "userConsumable": false,
//	               "listenType": null,
//	               "cannotListenReason": null,
//	               "playReason": null
//	           },
//	           "freeTimeTrialPrivilege": {
//	               "resConsumable": false,
//	               "userConsumable": false,
//	               "type": 0,
//	               "remainTime": 0
//	           },
//	           "urlSource": 0,
//	           "rightSource": 0,
//	           "podcastCtrp": null,
//	           "effectTypes": null,
//	           "time": 266516,
//	           "message": null
//	       }
//	   ],
//	   "code": 200
//	}
type SongPlayerResp struct {
	types.RespCommon[[]SongPlayerReqData]
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
	Message     interface{} `json:"message"`
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

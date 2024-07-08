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

type ArtistSongsReq struct {
	Id           int64  `json:"id"`            // 歌手id
	PrivateCloud string `json:"private_cloud"` // boolean
	WorkType     int64  `json:"work_type"`     // 通常为1
	Order        string `json:"order"`         // hot,time
	Offset       int64  `json:"offset"`
	Limit        int64  `json:"limit"`
}

type ArtistSongsResp struct {
	types.RespCommon[any]
	More  bool                   `json:"more"`
	Total int64                  `json:"total"`
	Songs []ArtistSongsRespSongs `json:"songs"`
}

type ArtistSongsRespSongs struct {
	A  interface{} `json:"a"`
	Al struct {
		Id     int    `json:"id"`
		Name   string `json:"name"`
		Pic    int64  `json:"pic"`
		PicUrl string `json:"picUrl"`
		PicStr string `json:"pic_str"`
	} `json:"al"`
	Alia []string `json:"alia"`
	Ar   []struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	} `json:"ar"`
	Cd              string         `json:"cd"`
	Cf              string         `json:"cf"`
	Cp              int            `json:"cp"`
	Crbt            interface{}    `json:"crbt"`
	DjId            int            `json:"djId"`
	Dt              int            `json:"dt"`
	Fee             int            `json:"fee"`
	Ftype           int            `json:"ftype"`
	H               *types.Quality `json:"h"`
	Hr              *types.Quality `json:"hr"`
	Id              int64          `json:"id"`
	L               *types.Quality `json:"l"`
	M               *types.Quality `json:"m"`
	Mst             int            `json:"mst"`
	Mv              int            `json:"mv"`
	Name            string         `json:"name"`
	No              int            `json:"no"`
	NoCopyrightRcmd interface{}    `json:"noCopyrightRcmd"`
	Pop             int            `json:"pop"`
	Privilege       struct {
		ChargeInfoList []struct {
			ChargeMessage interface{} `json:"chargeMessage"`
			ChargeType    int         `json:"chargeType"`
			ChargeUrl     interface{} `json:"chargeUrl"`
			Rate          int         `json:"rate"`
		} `json:"chargeInfoList"`
		Code               int    `json:"code"`
		Cp                 int    `json:"cp"`
		Cs                 bool   `json:"cs"`
		Dl                 int    `json:"dl"`
		DlLevel            string `json:"dlLevel"`
		DownloadMaxBrLevel string `json:"downloadMaxBrLevel"`
		DownloadMaxbr      int    `json:"downloadMaxbr"`
		Fee                int    `json:"fee"`
		Fl                 int    `json:"fl"`
		FlLevel            string `json:"flLevel"`
		Flag               int    `json:"flag"`
		FreeTrialPrivilege struct {
			CannotListenReason interface{} `json:"cannotListenReason"`
			ListenType         interface{} `json:"listenType"`
			PlayReason         interface{} `json:"playReason"`
			ResConsumable      bool        `json:"resConsumable"`
			UserConsumable     bool        `json:"userConsumable"`
		} `json:"freeTrialPrivilege"`
		Id             int64       `json:"id"`
		MaxBrLevel     string      `json:"maxBrLevel"`
		Maxbr          int         `json:"maxbr"`
		Message        interface{} `json:"message"`
		Payed          int         `json:"payed"`
		Pl             int         `json:"pl"`
		PlLevel        string      `json:"plLevel"`
		PlayMaxBrLevel string      `json:"playMaxBrLevel"`
		PlayMaxbr      int         `json:"playMaxbr"`
		PreSell        bool        `json:"preSell"`
		RightSource    int         `json:"rightSource"`
		Rscl           interface{} `json:"rscl"`
		Sp             int         `json:"sp"`
		St             int         `json:"st"`
		Subp           int         `json:"subp"`
		Toast          bool        `json:"toast"`
	} `json:"privilege"`
	Pst          int            `json:"pst"`
	Rt           string         `json:"rt"`
	RtUrl        interface{}    `json:"rtUrl"`
	RtUrls       []interface{}  `json:"rtUrls"`
	Rtype        int            `json:"rtype"`
	Rurl         interface{}    `json:"rurl"`
	SongJumpInfo interface{}    `json:"songJumpInfo"`
	Sq           *types.Quality `json:"sq"`
	St           int            `json:"st"`
	T            int            `json:"t"`
	V            int            `json:"v"`
	Tns          []string       `json:"tns,omitempty"`
}

// ArtistSongs 歌手所有歌曲
// url:
// needLogin:
func (a *Api) ArtistSongs(ctx context.Context, req *ArtistSongsReq) (*ArtistSongsResp, error) {
	var (
		url   = "https://music.163.com/weapi/v1/artist/songs"
		reply ArtistSongsResp
	)
	if req.Order == "" {
		req.Order = "hot"
	}
	if req.Limit == 0 {
		req.Limit = 100
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

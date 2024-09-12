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

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type LyricReq struct {
	Id     int64 `json:"id"`
	TV     int64 `json:"tv"`      // 翻译版本
	LV     int64 `json:"lv"`      // 歌词版本.
	RV     int64 `json:"rv"`      // 音译版本
	KV     int64 `json:"kv"`      // ?
	NMCLFL int64 `json:"_nmclfl"` // ?
}

type LyricResp struct {
	types.RespCommon[any]
	Sgc       bool      `json:"sgc"`
	Sfy       bool      `json:"sfy"`
	Qfy       bool      `json:"qfy"`
	TransUser TransUser `json:"transUser,omitempty"` // 翻译贡献者
	LyricUser TransUser `json:"lyricUser,omitempty"` // 歌词贡献者
	Lrc       Lyric     `json:"lrc"`                 // 歌词
	KLyric    Lyric     `json:"klyric"`              // ?
	TLyric    Lyric     `json:"tlyric"`              // 翻译歌词版本
	RomaLrc   Lyric     `json:"romalrc"`             // 音译歌词 例如: 今天我寒夜里看雪飘过 -> gam tin o hon yei lei hon sv piu guo
}

type TransUser struct {
	Id       int64  `json:"id"`
	Status   int64  `json:"status"`
	Demand   int64  `json:"demand"`
	UserId   int64  `json:"userid"`
	Nickname string `json:"nickname"`
	Uptime   int64  `json:"uptime"` // 1571647920128
}

type Lyric struct {
	Version int64  `json:"version"`
	Lyric   string `json:"lyric"`
}

// Lyric 根据歌曲id获取歌曲歌词
// url:
// needLogin: 否
func (a *Api) Lyric(ctx context.Context, req *LyricReq) (*LyricResp, error) {
	var (
		url   = "https://music.163.com/weapi/song/lyric"
		reply LyricResp
		opts  = api.NewOptions()
	)
	if req.TV == 0 {
		req.TV = -1
	}
	if req.LV == 0 {
		req.LV = -1
	}
	if req.RV == 0 {
		req.RV = -1
	}
	if req.KV == 0 {
		req.KV = -1
	}
	if req.NMCLFL == 0 {
		req.NMCLFL = 1
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type LyricV1Req struct {
	Id  int64 `json:"id"`
	CP  bool  `json:"cp"`  // ?
	TV  int64 `json:"tv"`  // 翻译版本
	LV  int64 `json:"lv"`  // 歌词版本.
	RV  int64 `json:"rv"`  // 音译版本
	KV  int64 `json:"kv"`  // ?
	YV  int64 `json:"yv"`  // ?
	YTK int64 `json:"ytk"` //
	YRV int64 `json:"yrv"` //
}

type LyricV1Resp struct {
	types.RespCommon[any]
	Sgc       bool        `json:"sgc"`
	Sfy       bool        `json:"sfy"`
	Qfy       bool        `json:"qfy"`
	NeedDesc  bool        `json:"needDesc"`
	PureMusic bool        `json:"pureMusic"`
	BriefDesc interface{} `json:"briefDesc,omitempty"`
	TransUser TransUser   `json:"transUser,omitempty"` // 翻译贡献者
	LyricUser TransUser   `json:"lyricUser,omitempty"` // 歌词贡献者
	Lrc       Lyric       `json:"lrc"`                 // 歌词
	KLyric    Lyric       `json:"klyric"`              //
	TLyric    Lyric       `json:"tlyric"`              // 翻译版本
	RomaLrc   Lyric       `json:"romalrc"`             // 音译歌词
	Yrc       Lyric       `json:"yrc,omitempty"`       // 逐字歌词
	YRomaLrc  Lyric       `json:"yromalrc,omitempty"`  //
}

// LyricV1 获取歌曲歌词,支持逐字歌词。
// url:
// needLogin: 未知
// see:
// https://github.com/Binaryify/NeteaseCloudMusicApi/issues/1667
// https://docs-neteasecloudmusicapi.vercel.app/docs/#/?id=%e8%8e%b7%e5%8f%96%e9%80%90%e5%ad%97%e6%ad%8c%e8%af%8d
func (a *Api) LyricV1(ctx context.Context, req *LyricV1Req) (*LyricV1Resp, error) {
	var (
		url   = "https://music.163.com/weapi/song/lyric/v1"
		reply LyricV1Resp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

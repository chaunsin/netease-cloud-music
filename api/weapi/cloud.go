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

type CloudListReq struct {
	types.ReqCommon
	Limit  int64 `json:"limit,omitempty"`
	Offset int64 `json:"offset,omitempty"`
}

type CloudListResp struct {
	types.RespCommon[[]CloudListRespData]
	HasMore     bool
	UpgradeSign int64
	MaxSize     string
	Size        string
	Count       int64
}

type CloudListRespData struct {
	SimpleSong CloudListRespDataSimpleSong `json:"simpleSong"`
	SongId     int                         `json:"songId"`   // 歌曲ID
	AddTime    int64                       `json:"addTime"`  // 上传到网盘时间
	Bitrate    int                         `json:"bitrate"`  //
	SongName   string                      `json:"songName"` // 歌曲名称
	Album      string                      `json:"album"`    // 专辑名称
	Artist     string                      `json:"artist"`   // 歌手
	Cover      int                         `json:"cover"`
	CoverId    string                      `json:"coverId"`
	LyricId    string                      `json:"lyricId"`
	Version    int                         `json:"version"`
	FileSize   int                         `json:"fileSize"` // 文件大小单位B
	FileName   string                      `json:"fileName"` // 音乐文件名称例如: 陈琳 - 十二种颜色.flac
}

type Quality struct {
	Br   int     `json:"br"`
	Fid  int     `json:"fid"`
	Size int     `json:"size"`
	Vd   float64 `json:"vd"`
}

type CloudListRespDataSimpleSong struct {
	Name string `json:"name"`
	Id   int    `json:"id"`
	Pst  int    `json:"pst"`
	T    int    `json:"t"`
	Ar   []struct {
		Id    int           `json:"id"`
		Name  string        `json:"name"`
		Tns   []interface{} `json:"tns"`
		Alias []interface{} `json:"alias"`
	} `json:"ar"`
	Alia []interface{} `json:"alia"`
	Pop  float64       `json:"pop"`
	St   int           `json:"st"`
	Rt   string        `json:"rt"`
	Fee  int           `json:"fee"`
	V    int           `json:"v"`
	Crbt interface{}   `json:"crbt"`
	Cf   string        `json:"cf"`
	Al   struct {
		Id     int           `json:"id"`
		Name   string        `json:"name"`
		PicUrl string        `json:"picUrl"`
		Tns    []interface{} `json:"tns"`
		PicStr string        `json:"pic_str,omitempty"`
		Pic    int64         `json:"pic"`
	} `json:"al"`
	Dt                   int           `json:"dt"`
	H                    Quality       `json:"h"`
	M                    Quality       `json:"m"`
	L                    Quality       `json:"l"`
	A                    interface{}   `json:"a"`
	Cd                   string        `json:"cd"`
	No                   int           `json:"no"`
	RtUrl                interface{}   `json:"rtUrl"`
	Ftype                int           `json:"ftype"`
	RtUrls               []interface{} `json:"rtUrls"`
	DjId                 int           `json:"djId"`
	Copyright            int           `json:"copyright"`
	SId                  int           `json:"s_id"`
	Mark                 int64         `json:"mark"`
	OriginCoverType      int           `json:"originCoverType"`
	OriginSongSimpleData interface{}   `json:"originSongSimpleData"`
	Single               int           `json:"single"`
	NoCopyrightRcmd      struct {
		Type     int         `json:"type"`
		TypeDesc string      `json:"typeDesc"`
		SongId   interface{} `json:"songId"`
	} `json:"noCopyrightRcmd"`
	Cp          int         `json:"cp"`
	Mv          int         `json:"mv"`
	Mst         int         `json:"mst"`
	Rurl        interface{} `json:"rurl"`
	Rtype       int         `json:"rtype"`
	PublishTime int64       `json:"publishTime"`
	Privilege   struct {
		Id                 int         `json:"id"`
		Fee                int         `json:"fee"`
		Payed              int         `json:"payed"`
		St                 int         `json:"st"`
		Pl                 int         `json:"pl"`
		Dl                 int         `json:"dl"`
		Sp                 int         `json:"sp"`
		Cp                 int         `json:"cp"`
		Subp               int         `json:"subp"`
		Cs                 bool        `json:"cs"`
		Maxbr              int         `json:"maxbr"`
		Fl                 int         `json:"fl"`
		Toast              bool        `json:"toast"`
		Flag               int         `json:"flag"`
		PreSell            bool        `json:"preSell"`
		PlayMaxbr          int         `json:"playMaxbr"`
		DownloadMaxbr      int         `json:"downloadMaxbr"`
		MaxBrLevel         string      `json:"maxBrLevel"`
		PlayMaxBrLevel     string      `json:"playMaxBrLevel"`
		DownloadMaxBrLevel string      `json:"downloadMaxBrLevel"`
		PlLevel            string      `json:"plLevel"`
		DlLevel            string      `json:"dlLevel"`
		FlLevel            string      `json:"flLevel"`
		Rscl               interface{} `json:"rscl"`
		FreeTrialPrivilege struct {
			ResConsumable  bool        `json:"resConsumable"`
			UserConsumable bool        `json:"userConsumable"`
			ListenType     interface{} `json:"listenType"`
		} `json:"freeTrialPrivilege"`
		ChargeInfoList []struct {
			Rate          int         `json:"rate"`
			ChargeUrl     interface{} `json:"chargeUrl"`
			ChargeMessage interface{} `json:"chargeMessage"`
			ChargeType    int         `json:"chargeType"`
		} `json:"chargeInfoList"`
	} `json:"privilege"`
}

// CloudList 查询云盘列表
func (a *Api) CloudList(ctx context.Context, req *CloudListReq) (*CloudListResp, error) {
	var (
		url   = "https://music.163.com/weapi/v1/cloud/get"
		reply CloudListResp
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

type CloudTokenAllocReq struct {
	types.ReqCommon
	Bucket string `json:"bucket,omitempty"`
	// 文件扩展名 例如mp3
	Ext      string `json:"ext,omitempty"`
	Filename string `json:"filename,omitempty"`
	Local    string `json:"local,omitempty"`
	// 3
	NosProduct string `json:"nos_product,omitempty"`
	// 文件类型 例如 audio
	Type string `json:"type,omitempty"`
	Md5  string `json:"md5,omitempty"`
}

type CloudTokenAllocResp struct {
	types.RespCommon[any]
	CloudTokenAllocRespResult `json:"result,omitempty"`
}

type CloudTokenAllocRespResult struct {
	Bucket     string `json:"bucket"`
	DocID      string `json:"docId"`
	ObjectKey  string `json:"objectKey"`
	OuterURL   string `json:"outerUrl"`
	ResourceID int64  `json:"resourceId"`
	Token      string `json:"token"`
}

// CloudTokenAlloc 获取上传云盘token
// url:
// needLogin: 未知
// todo:待验证
func (a *Api) CloudTokenAlloc(ctx context.Context, req *CloudTokenAllocReq) (*CloudTokenAllocResp, error) {
	var (
		url   = "https://music.163.com/weapi/nos/token/alloc"
		reply CloudTokenAllocResp
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

type CloudUploadCheckReq struct {
	types.ReqCommon
	// 音乐比特率 例如: 128000、192000、320000、999000
	Bitrate string `json:"bitrate,omitempty"`
	Ext     string `json:"ext,omitempty"`
	Length  string `json:"length,omitempty"`
	Md5     string `json:"md5,omitempty"`
	SongId  string `json:"songId,omitempty"`
	Version string `json:"version,omitempty"`
}

type CloudUploadCheckResp struct {
	types.RespCommon[any]
	SongId     string `json:"songId,omitempty"`
	NeedUpload bool   `json:"needUpload" json:"needUpload,omitempty"`
}

// CloudUploadCheck 获取上传云盘token
// url:
// needLogin: 未知
// todo: 需要迁移到api包中
// todo: 待验证
func (a *Api) CloudUploadCheck(ctx context.Context, req *CloudUploadCheckReq) (*CloudUploadCheckResp, error) {
	var (
		url = "https://interface.music.163.com/weapi/cloud/upload/check"
		// url   = "https://interface.music.163.com/api/cloud/upload/check" // TODO:原本url 考虑Request替换url
		reply CloudUploadCheckResp
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

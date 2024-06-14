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
	"net/url"

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

// CloudTokenAllocRespResult 数据示例
// {
// "bucket": "jd-musicrep-privatecloud-audio-public",
// "token": "UPLOAD 037a197cb50b42468694de59c0bdd9b1:zWmW6BmWPo5mWEMdTCEjtu9SaSRpgYmSQpXtb20fVd0=:eyJSZWdpb24iOiJKRCIsIk9iamVjdCI6Im9iai93b0REbU1PQnc2UENsV3pDbk1LLS8zNjY1NjcyOTU5OC8xN2JjL2Y0MjQvYjMyNi84MDJiMmVmZTJiMGY1ZjU0MzAyNGFlYWFmNzQ3NGEwNi5tNGEiLCJFeHBpcmVzIjoxNzE4MzUyNjI4LCJCdWNrZXQiOiJqZC1tdXNpY3JlcC1wcml2YXRlY2xvdWQtYXVkaW8tcHVibGljIn0=",
// "outerUrl": "https://jd-musicrep-privatecloud-audio-public.nos-jd.163yun.com/obj%2FwoDDmMOBw6PClWzCnMK-%2F36656729598%2F17bc%2Ff424%2Fb326%2F802b2efe2b0f5f543024aeaaf7474a06.m4a?Signature=NuGYr715XbqmSdr7xWoVoYR0GiwDc6zJ0luYLY0WSaE%3D&Expires=1718350828&NOSAccessKeyId=037a197cb50b42468694de59c0bdd9b1",
// "docId": "-1",
// "objectKey": "obj/woDDmMOBw6PClWzCnMK-/36656729598/17bc/f424/b326/802b2efe2b0f5f543024aeaaf7474a06.m4a",
// "resourceId": 36656729598
// }
type CloudTokenAllocRespResult struct {
	Bucket     string `json:"bucket"`
	Token      string `json:"token"`
	OuterURL   string `json:"outerUrl"`
	DocID      string `json:"docId"`
	ObjectKey  string `json:"objectKey"`
	ResourceID int64  `json:"resourceId"`
}

// CloudTokenAlloc 获取上传云盘token
// url:
// needLogin: 未知
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
	SongId string `json:"songId,omitempty"`
	// NeedUpload 是否需要上传 true:需要上传说明网易云网盘没有此音乐文件
	NeedUpload bool `json:"needUpload" json:"needUpload,omitempty"`
}

// CloudUploadCheck 获取上传云盘token
// url:
// needLogin: 未知
// todo: 需要迁移到api包中
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

type CloudUploadReq struct {
	types.ReqCommon
	Bucket    string `json:"bucket"`
	ObjectKey string `json:"objectKey"`
	Token     string `json:"token"`
	Filepath  string `json:"filepath"`
}

type CloudUploadResp struct {
	// types.RespCommon[any]
	ErrCode        string `json:"errCode,omitempty"`
	ErrMsg         string `json:"errMsg,omitempty"`
	RequestId      string `json:"requestId,omitempty"`
	Offset         int64  `json:"offset,omitempty"`
	Context        string `json:"context,omitempty"`
	CallbackRetMsg string `json:"callbackRetMsg,omitempty"`
	DownloadUrl    string `json:"downloadUrl,omitempty"` // 为啥没有？
}

// CloudUpload 上传到云盘
// url:
// needLogin: 未知
// todo: 需要迁移到合适的包中
func (a *Api) CloudUpload(ctx context.Context, req *CloudUploadReq) (*CloudUploadResp, error) {
	// 获取上传地址，查找服务上传点
	// https://wanproxy.127.net/lbs?version=1.0&bucketname=${bucket}
	// TODO: https://gitlab.com/Binaryify/neteasecloudmusicapi/-/blob/main/plugins/songUpload.js?ref_type=heads#L42

	objectKey, err := url.PathUnescape(req.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("PathUnescape: %v", err)
	}

	var (
		// url   = fmt.Sprintf("http://45.127.129.8/%s/%s?offset=0&complete=true&version=1.0", req.Bucket, objectKey) // 写死的地址方式目前也能上传
		url   = fmt.Sprintf("http://59.111.242.121/%s/%s?offset=0&complete=true&version=1.0", req.Bucket, objectKey)
		reply CloudUploadResp
	)

	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	var headers = map[string]string{
		// "Content-Type":   "audio/mpeg",
		// "Content-Length": "1.0",
		// "Content-Md5":    "",
		"X-Nos-Token": req.Token,
	}

	resp, err := a.client.Upload(ctx, url, headers, req.Filepath, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type CloudInfoReq struct {
	types.ReqCommon
	Md5      string `json:"md5,omitempty"`
	SongId   string `json:"songid,omitempty"`
	Filename string `json:"filename,omitempty"`
	// Song 歌曲名称
	Song string `json:"song,omitempty"`
	// Album 专辑名称
	Album string `json:"album,omitempty"`
	// Artist 艺术家
	Artist     string `json:"artist,omitempty"`
	Bitrate    string `json:"bitrate,omitempty"`
	ResourceId int64  `json:"resourceId,omitempty"`
}

type CloudInfoResp struct {
	Code           int64        `json:"code,omitempty"`
	SongId         string       `json:"songId,omitempty"`
	WaitTime       int          `json:"waitTime"`
	Exists         bool         `json:"exists"`
	NextUploadTime int          `json:"nextUploadTime"`
	SongIdLong     int          `json:"songIdLong"`
	PrivateCloud   PrivateCloud `json:"privateCloud"`
}

type PrivateCloud struct {
	SimpleSong struct {
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
		Mark                 int           `json:"mark"`
		OriginCoverType      int           `json:"originCoverType"`
		OriginSongSimpleData interface{}   `json:"originSongSimpleData"`
		Single               int           `json:"single"`
		NoCopyrightRcmd      interface{}   `json:"noCopyrightRcmd"`
		Mst                  int           `json:"mst"`
		Cp                   int           `json:"cp"`
		Mv                   int           `json:"mv"`
		Rtype                int           `json:"rtype"`
		Rurl                 interface{}   `json:"rurl"`
		PublishTime          int64         `json:"publishTime"`
		Privilege            struct {
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
			ChargeInfoList interface{} `json:"chargeInfoList"`
		} `json:"privilege"`
	} `json:"simpleSong"`
	Cover    int    `json:"cover"`
	AddTime  int64  `json:"addTime"`
	SongName string `json:"songName"`
	Album    string `json:"album"`
	Artist   string `json:"artist"`
	Bitrate  int    `json:"bitrate"`
	SongId   int    `json:"songId"`
	CoverId  string `json:"coverId"`
	LyricId  string `json:"lyricId"`
	Version  int    `json:"version"`
	FileSize int    `json:"fileSize"`
	FileName string `json:"fileName"`
}

// CloudInfo 上传信息歌曲信息
// url:
// needLogin: 未知
// todo: 需要迁移到合适的包中
func (a *Api) CloudInfo(ctx context.Context, req *CloudInfoReq) (*CloudInfoResp, error) {
	var (
		url   = "https://music.163.com/api/upload/cloud/info/v2"
		reply CloudInfoResp
	)
	if req.Album == "" {
		req.Album = "未知专辑"
	}
	if req.Artist == "" {
		req.Artist = "未知艺术家"
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type CloudPublishReq struct {
	types.ReqCommon
	SongId string `json:"songid"`
}

type CloudPublishResp struct {
	// 200:成功 201:貌似重复上传
	Code         int64        `json:"code"`
	PrivateCloud PrivateCloud `json:"privateCloud"`
}

// CloudPublish 上传信息发布
// url:
// needLogin: 未知
// todo: 需要迁移到合适的包中
func (a *Api) CloudPublish(ctx context.Context, req *CloudPublishReq) (*CloudPublishResp, error) {
	var (
		url   = "https://interface.music.163.com/api/cloud/pub/v2"
		reply CloudPublishResp
	)

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

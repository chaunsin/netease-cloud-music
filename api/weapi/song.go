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
	"encoding/json"
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
	Type               string      `json:"type"` // 类型eg: mp3、FLAC
	Gain               float64     `json:"gain"`
	Peak               float64     `json:"peak"`
	Fee                int         `json:"fee"`
	Uf                 interface{} `json:"uf"`
	Payed              int         `json:"payed"`
	Flag               int         `json:"flag"`
	CanExtend          bool        `json:"canExtend"`
	FreeTrialInfo      interface{} `json:"freeTrialInfo"`
	Level              string      `json:"level"` // 通常所说的音质水平 eg: standard、exhigh、higher、lossless、hires
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
	Time        int         `json:"time"` // 音乐时长,单位毫秒
	Message     interface{} `json:"message"`
}

// SongPlayer 音乐播放详情
// url:
// needLogin: 未知
// 提示: 获取的歌曲url有时效性,失效时间目前测试为20分钟,过期访问则会出现403错误
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

// Level 音乐品质
type Level string

const (
	// LevelStandard 标准品质
	LevelStandard Level = "standard"
	// LevelHigher 较高品质
	LevelHigher Level = "higher"
	// LevelExhigh 极高品质
	LevelExhigh Level = "exhigh"
	// LevelLossless 无损品质
	LevelLossless Level = "lossless"
	// LevelHires Hi-Res品质
	LevelHires Level = "hires"
	// LevelJyeffect 高清环绕声品质
	LevelJyeffect Level = "jyeffect"
	// LevelSky 沉浸环绕声品质
	LevelSky Level = "sky"
	// LevelJymaster 超清母带品质
	LevelJymaster Level = "jymaster"
)

type SongPlayerReqV1 struct {
	types.ReqCommon
	Ids         types.IntsString `json:"ids"`         // 歌曲id eg: 2016588459_1289504343 下滑线前位歌曲id, todo: 后位目前未知,不过不传下划线后面的内容也是可以正方返回得
	Level       Level            `json:"level"`       // 音乐质量 Level
	EncodeType  string           `json:"encodeType"`  // 音乐格式 eg: mp3
	ImmerseType string           `json:"immerseType"` // 只有Level为sky时生效
}

type SongPlayerRespV1 struct {
	types.RespCommon[[]SongPlayerRespV1Data]
}

type SongPlayerRespV1Data struct {
	Id                 int         `json:"id"`   // 歌曲id
	Url                string      `json:"url"`  // 歌曲资源url有时效性
	Br                 int         `json:"br"`   // 码率
	Size               int         `json:"size"` // 文件大小单位字节
	Md5                string      `json:"md5"`  // 文件MD5值
	Code               int         `json:"code"` // 状态码
	Expi               int         `json:"expi"` // 可访问url的过期时间,目前为1200秒
	Type               string      `json:"type"` // 类型eg: mp3、FLAC
	Gain               float64     `json:"gain"`
	Peak               float64     `json:"peak"`
	Fee                int         `json:"fee"`
	Uf                 interface{} `json:"uf"`
	Payed              int         `json:"payed"`
	Flag               int         `json:"flag"`
	CanExtend          bool        `json:"canExtend"`
	FreeTrialInfo      interface{} `json:"freeTrialInfo"`
	Level              string      `json:"level"`      // 音质水平 eg: standard、exhigh、higher、lossless、hires
	EncodeType         string      `json:"encodeType"` // eg: mp3
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
	UrlSource    int         `json:"urlSource"`
	RightSource  int         `json:"rightSource"`
	PodcastCtrp  interface{} `json:"podcastCtrp"`
	EffectTypes  interface{} `json:"effectTypes"`
	Time         int         `json:"time"` // 音乐时长,单位毫秒
	Message      interface{} `json:"message"`
	LevelConfuse interface{} `json:"levelConfuse"`
}

// SongPlayerV1 音乐播放详情
// url: testdata/har/6.har
// needLogin: 未知
// 提示: 获取的歌曲url有时效性,失效时间目前测试为20分钟,过期访问则会出现403错误
func (a *Api) SongPlayerV1(ctx context.Context, req *SongPlayerReqV1) (*SongPlayerRespV1, error) {
	var (
		url   = "https://music.163.com/weapi/song/enhance/player/url/v1"
		reply SongPlayerRespV1
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}
	if req.Level == LevelSky {
		req.ImmerseType = "c51"
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type SongDetailReq struct {
	C []SongDetailReqList `json:"c"`
}

type SongDetailReqList struct {
	Id string `json:"id"`
	V  int64  `json:"v"`
}

type songDetailReq struct {
	C string `json:"c"`
}

// SongDetailResp .
type SongDetailResp struct {
	types.RespCommon[any]
	Songs      []SongDetailRespSongs      `json:"songs"`
	Privileges []SongDetailRespPrivileges `json:"privileges"`
}

// SongDetailRespSongs
// see: https://github.com/Binaryify/NeteaseCloudMusicApi/issues/1121#issuecomment-774438040
type SongDetailRespSongs struct {
	// Name 歌曲标题
	Name string `json:"name"`
	// Id 歌曲ID
	Id int `json:"id"`
	// Pst 功能未知
	Pst int `json:"pst"`
	// T
	// 0: 一般类型
	// 1: 通过云盘上传的音乐，网易云不存在公开对应
	//  如果没有权限将不可用，除了歌曲长度以外大部分信息都为null。
	//  可以通过 `/api/v1/playlist/manipulate/tracks` 接口添加到播放列表。
	//  如果添加到“我喜欢的音乐”，则仅自己可见，除了长度意外各种信息均为未知，且无法播放。
	//  如果添加到一般播放列表，虽然返回code 200，但是并没有效果。
	//  网页端打开会看到404画面。
	//  属于这种歌曲的例子: https://music.163.com/song/1345937107
	// 2: 通过云盘上传的音乐，网易云存在公开对应
	//  如果没有权限则只能看到信息，但无法直接获取到文件。
	//	可以通过 `/api/v1/playlist/manipulate/tracks` 接口添加到播放列表。
	//	如果添加到“我喜欢的音乐”，则仅自己可见，且无法播放。
	//	如果添加到一般播放列表，则自己会看到显示“云盘文件”，且云盘会多出其对应的网易云公开歌曲。其他人看到的是其对应的网易云公开歌曲。
	//	网页端打开会看到404画面。
	//	属于这种歌曲的例子: https://music.163.com/song/435005015
	T int `json:"t"`
	// Ar 歌手列表
	Ar []struct {
		Id    int           `json:"id"`
		Name  string        `json:"name"`
		Tns   []interface{} `json:"tns"`
		Alias []interface{} `json:"alias"`
	} `json:"ar"`
	// Alia 别名列表,第一个别名会被显示作副标题 例子: https://music.163.com/song/536623501
	Alia []interface{} `json:"alia"`
	// Pop 小数，常取[0.0, 100.0]中离散的几个数值, 表示歌曲热度
	Pop float64 `json:"pop"`
	// St 未知
	St int `json:"st"`
	// Rt None、空白字串、或者类似`600902000007902089`的字符串，功能未知
	Rt string `json:"rt"`
	// Fee 费用情况 0:免费 1:2元购买单曲 4:购买专辑 8:低音质免费
	Fee int `json:"fee"`
	// V 常为[1, ?]任意数字, 功能未知
	V int `json:"v"`
	// Crbt None或字符串表示的十六进制，功能未知
	Crbt interface{} `json:"crbt"`
	// Cf 空白字串或者None，功能未知
	Cf string `json:"cf"`
	// Al Album, 专辑，如果是DJ节目(dj_type != 0)或者无专辑信息(single == 1)，则专辑id为0
	Al struct {
		Id     int           `json:"id"`
		Name   string        `json:"name"`
		PicUrl string        `json:"picUrl"`
		Tns    []interface{} `json:"tns"`
		PicStr string        `json:"pic_str"`
		Pic    int64         `json:"pic"`
	} `json:"al"`
	// Dt 歌曲时长
	Dt int `json:"dt"`
	// H 级高质量文件信息
	H types.Quality `json:"h"`
	// M 中质量文件信息
	M types.Quality `json:"m"`
	// L 标准质量文件信息
	L types.Quality `json:"l"`
	// Sq 无损质量文件信息
	Sq types.Quality `json:"sq"`
	// Hr Hi-Res质量文件信息
	Hr interface{} `json:"hr"`
	// A 常为None，功能未知
	A interface{} `json:"a"`
	// Cd None或如"04", "1/2", "3", "null"的字符串，表示歌曲属于专辑中第几张CD，对应音频文件的Tag
	Cd string `json:"cd"`
	// No 表示歌曲属于CD中第几曲, 0表示没有这个字段, 对应音频文件的Tag
	No int `json:"no"`
	// RtUrl 常为None, 功能未知
	RtUrl interface{} `json:"rtUrl"`
	// Ftype 未知
	Ftype int `json:"ftype"`
	// RtUrls 常为空列表，功能未知
	RtUrls []interface{} `json:"rtUrls"`
	// DjId 0:不是DJ节目 其他:是DJ节目，表示DJ ID
	DjId int `json:"djId"`
	// Copyright 0, 1, 2: 功能未知
	Copyright int `json:"copyright"`
	// SId 对于t == 2的歌曲，表示匹配到的公开版本歌曲ID
	SId int `json:"s_id"`
	// Mark 功能未知
	Mark int `json:"mark"`
	// OriginCoverType 0:未知 1:原曲 2:翻唱
	OriginCoverType int `json:"originCoverType"`
	// OriginSongSimpleData 对于翻唱曲，可选提供原曲简单格式的信息
	OriginSongSimpleData interface{} `json:"originSongSimpleData"`
	// SongMeiZuData 功能未知
	TagPicList interface{} `json:"tagPicList"`
	// ResourceState 未知
	ResourceState bool `json:"resourceState"`
	// Version 什么版本？
	Version int `json:"version"`
	// SongJumpInfo 功能未知
	SongJumpInfo interface{} `json:"songJumpInfo"`
	// EntranceCrash 功能未知
	EntertainmentTags interface{} `json:"entertainmentTags"`
	// AwardTags 功能未知
	AwardTags interface{} `json:"awardTags"`
	// Single 0: 有专辑信息或者是DJ节目 1:未知专辑
	Single int `json:"single"`
	// NoCopyrightRcmd None表示可以播，非空表示无版权
	NoCopyrightRcmd interface{} `json:"noCopyrightRcmd"`
	// Mv 非零表示有MV ID
	Mv int `json:"mv"`
	// Rurl 常为None，功能未知
	Rurl interface{} `json:"rurl"`
	// Mst 偶尔为0, 常为9，功能未知
	Mst int `json:"mst"`
	// Cp 未知
	Cp int `json:"cp"`
	// Rtype 常为0，功能未知
	Rtype int `json:"rtype"`
	// PublishTime 毫秒为单位的Unix时间戳
	PublishTime int `json:"publishTime"`
}

type SongDetailRespPrivileges struct {
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
		ResConsumable      bool        `json:"resConsumable"`
		UserConsumable     bool        `json:"userConsumable"`
		ListenType         interface{} `json:"listenType"`
		CannotListenReason interface{} `json:"cannotListenReason"`
		PlayReason         interface{} `json:"playReason"`
	} `json:"freeTrialPrivilege"`
	RightSource    int `json:"rightSource"`
	ChargeInfoList []struct {
		Rate          int         `json:"rate"`
		ChargeUrl     interface{} `json:"chargeUrl"`
		ChargeMessage interface{} `json:"chargeMessage"`
		ChargeType    int         `json:"chargeType"`
	} `json:"chargeInfoList"`
}

// SongDetail 根据歌曲id获取歌曲详情
// url: https://app.apifox.com/project/3870894 testdata/har/1.har
// needLogin: 未知
func (a *Api) SongDetail(ctx context.Context, req *SongDetailReq) (*SongDetailResp, error) {
	var (
		url   = "https://music.163.com/weapi/v3/song/detail"
		reply SongDetailResp
	)

	// "[{\"id\":\"1974334953\",\"v\":0}]
	data, err := json.Marshal(req.C)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", &songDetailReq{C: string(data)}, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type SongMusicQualityReq struct {
	SongId string `json:"songId"`
}

type SongMusicQualityResp struct {
	types.RespCommon[SongMusicQualityRespData]
}

type SongMusicQualityRespData struct {
	// Db 未知通常为null
	Db any `json:"db"`
	// SongId 歌曲id
	SongId int64 `json:"songId"`
	// L 标准品质
	L *types.Quality `json:"l"`
	// M 貌似是高音质,通常客户端好像看不到这个音质了目前
	M *types.Quality `json:"m"`
	// H 极高品质
	H *types.Quality `json:"h"`
	// Sq 无损品质
	Sq *types.Quality `json:"sq"`
	// Hr Hi-Res品质
	Hr *types.Quality `json:"hr"`
	// Je 高清环绕声品质
	Je *types.Quality `json:"je"`
	// Sk 沉浸环绕声品质
	Sk *types.Quality `json:"sk"`
	// Jm 超清母带品质
	Jm *types.Quality `json:"jm"`
}

// SongMusicQuality 根据歌曲id获取音质详情
// url:
// needLogin: 未知
func (a *Api) SongMusicQuality(ctx context.Context, req *SongMusicQualityReq) (*SongMusicQualityResp, error) {
	var (
		url   = "https://music.163.com/weapi/song/music/detail/get"
		reply SongMusicQualityResp
	)

	resp, err := a.client.Request(ctx, http.MethodPost, url, "weapi", &req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/cheggaaa/pb/v3"
)

type CloudListReq struct {
	types.ReqCommon
	Limit  int64 `json:"limit,omitempty"`
	Offset int64 `json:"offset,omitempty"`
}

type CloudListResp struct {
	types.RespCommon[[]CloudListRespData]
	HasMore     bool   // 用于分页
	UpgradeSign int64  // 目前未知
	MaxSize     string // 网盘总共空间
	Size        string // 当前已经使用得空间
	Count       int64  // 歌曲总数量
}

type CloudListRespData struct {
	SimpleSong CloudListRespDataSimpleSong `json:"simpleSong"`
	SongId     int64                       `json:"songId"`   // 歌曲ID
	AddTime    int64                       `json:"addTime"`  // 上传到网盘时间
	Bitrate    int64                       `json:"bitrate"`  //
	SongName   string                      `json:"songName"` // 歌曲名称
	Album      string                      `json:"album"`    // 专辑名称
	Artist     string                      `json:"artist"`   // 歌手
	Cover      int64                       `json:"cover"`
	CoverId    string                      `json:"coverId"`
	LyricId    string                      `json:"lyricId"`
	Version    int64                       `json:"version"`
	FileSize   int64                       `json:"fileSize"` // 文件大小单位B
	FileName   string                      `json:"fileName"` // 音乐文件名称例如: 陈琳 - 十二种颜色.flac
}

type CloudListRespDataSimpleSong struct {
	Name                 string         `json:"name"`
	Id                   int64          `json:"id"`
	Pst                  int64          `json:"pst"`
	T                    int64          `json:"t"`
	Ar                   []types.Artist `json:"ar"`
	Alia                 []interface{}  `json:"alia"`
	Pop                  float64        `json:"pop"`
	St                   int64          `json:"st"`
	Rt                   string         `json:"rt"`
	Fee                  int64          `json:"fee"`
	V                    int64          `json:"v"`
	Crbt                 interface{}    `json:"crbt"`
	Cf                   string         `json:"cf"`
	Al                   types.Album    `json:"al"`
	Dt                   int64          `json:"dt"`
	H                    *types.Quality `json:"h"`
	M                    *types.Quality `json:"m"`
	L                    *types.Quality `json:"l"`
	A                    interface{}    `json:"a"`
	Cd                   string         `json:"cd"`
	No                   int64          `json:"no"`
	RtUrl                interface{}    `json:"rtUrl"`
	Ftype                int64          `json:"ftype"`
	RtUrls               []interface{}  `json:"rtUrls"`
	DjId                 int64          `json:"djId"`
	Copyright            int64          `json:"copyright"`
	SId                  int64          `json:"s_id"`
	Mark                 int64          `json:"mark"`
	OriginCoverType      int64          `json:"originCoverType"`
	OriginSongSimpleData interface{}    `json:"originSongSimpleData"`
	Single               int64          `json:"single"`
	NoCopyrightRcmd      struct {
		Type     int64       `json:"type"`
		TypeDesc string      `json:"typeDesc"`
		SongId   interface{} `json:"songId"`
	} `json:"noCopyrightRcmd"`
	Cp          int64            `json:"cp"`
	Mv          int64            `json:"mv"`
	Mst         int64            `json:"mst"`
	Rurl        interface{}      `json:"rurl"`
	Rtype       int64            `json:"rtype"`
	PublishTime int64            `json:"publishTime"`
	Privilege   types.Privileges `json:"privilege"`
}

// CloudList 查询云盘列表,包含云盘空间大小、已用空间数
func (a *Api) CloudList(ctx context.Context, req *CloudListReq) (*CloudListResp, error) {
	var (
		url   = "https://music.163.com/weapi/v1/cloud/get"
		reply CloudListResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
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
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
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
	// code 501:貌似上传得文件过大
	types.RespCommon[any]
	SongId string `json:"songId,omitempty"`
	// NeedUpload 是否需要上传 true:需要上传说明网易云网盘没有此音乐文件
	NeedUpload bool `json:"needUpload" json:"needUpload,omitempty"`
}

// CloudUploadCheck 获取上传云盘token
// url:
// needLogin: 未知
func (a *Api) CloudUploadCheck(ctx context.Context, req *CloudUploadCheckReq) (*CloudUploadCheckResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/cloud/upload/check"
		reply CloudUploadCheckResp
		opts  = api.NewOptions()
	)
	if req.CSRFToken == "" {
		csrf, _ := a.client.GetCSRF(url)
		req.CSRFToken = csrf
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type CloudUploadReq struct {
	// types.ReqCommon
	Bucket      string          `json:"bucket"`
	ObjectKey   string          `json:"objectKey"`
	Token       string          `json:"token"`
	Filepath    string          `json:"filepath"`
	ProgressBar *pb.ProgressBar `json:"-"` // 仅用于上传显示进度条使用跟网易云api无关.通常设置成nil
}

type CloudUploadResp struct {
	// types.RespCommon[any]
	// ErrCode 为空则说明成功
	ErrCode   string `json:"errCode,omitempty"`
	ErrMsg    string `json:"errMsg,omitempty"`
	RequestId string `json:"requestId,omitempty"`
	// Offset 用于分片上传时使用
	Offset int64 `json:"offset,omitempty"`
	// Context 用于分片上传时使用,用于下一个请求携带
	Context        string `json:"context,omitempty"`
	CallbackRetMsg string `json:"callbackRetMsg,omitempty"`
	DownloadUrl    string `json:"downloadUrl,omitempty"` // 为啥没有？
}

type CloudUploadLbsResp struct {
	Lbs    string   `json:"lbs"`
	Upload []string `json:"upload"`
}

// CloudUpload 上传到云盘
// url:
// needLogin: 未知
// todo: 需要迁移到合适的包中
func (a *Api) CloudUpload(ctx context.Context, req *CloudUploadReq) (*CloudUploadResp, error) {
	objectKey, err := url.PathUnescape(req.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("PathUnescape: %v", err)
	}

	var (
		addr      = fmt.Sprintf("https://wanproxy.127.net/lbs?version=1.0&bucketname=%s", req.Bucket)
		urlFormat = "%s/%s/%s"
		ip        = "http://59.111.242.121"
		uploadUrl = fmt.Sprintf(urlFormat, ip, req.Bucket, objectKey)
		reply     CloudUploadResp
	)

	// 获取上传地址，查找服务上传点
	resp, err := a.client.
		NewRequest().
		SetContext(ctx).
		SetHeader("Referer", "https://music.163.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034"). // todo: hard code
		Get(addr)
	if err != nil || resp.StatusCode() != http.StatusOK {
		log.Error("user default upload lbs node. get %s error: %v", addr, err)
		return nil, fmt.Errorf("Get: %w", err)
	} else {
		var lbs CloudUploadLbsResp
		if err := json.Unmarshal(resp.Body(), &lbs); err != nil {
			log.Error("user default upload lbs node. Unmarshal %s error: %v", addr, err)
		} else {
			if len(lbs.Upload) > 0 {
				ip = lbs.Upload[rand.Intn(len(lbs.Upload))]
				uploadUrl = fmt.Sprintf(urlFormat, ip, req.Bucket, objectKey)
			}
		}
	}

	// return uploadFile(ip, req.Filepath, objectKey, req.Token)

	file, err := os.Open(req.Filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	md5, err := utils.MD5Hex(data)
	if err != nil {
		return nil, fmt.Errorf("MD5Hex: %v", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("SeekStart: %v", err)
	}

	var (
		ext         = filepath.Ext(req.Filepath)
		totalSize   = stat.Size()
		chunkSize   = utils.MB * 80
		chunks      = int((totalSize + chunkSize - 1) / chunkSize)
		nextContext = ""
	)
	uploadUrl = uploadUrl + "?offset=%d&complete=%v&version=1.0"

	var headers = map[string]string{
		"X-Nos-Token":    req.Token,
		"Content-Length": fmt.Sprintf("%d", totalSize),
		"Content-Md5":    md5,
		"Content-Type":   utils.DetectContentType(data, ext),
		// "x-nos-meta-origin-md5": md5,
		// "x-nos-meta-origin-source": "A-cloudmusic-9.1.10", // 手机app版本
		// "X-MAM-CustomMark": "okhttp",
		// "Transfer-Encoding": "chunked", // 与Content-Length是互斥得
		// "CMPageId": "UploadMusicActivity",
	}
	// resp, err = a.client.Upload(ctx, fmt.Sprintf(uploadUrl, 0, true), headers, file, &reply, req.ProgressBar)
	// if err != nil {
	// 	return nil, fmt.Errorf("Upload: %w", err)
	// }
	// return &reply, nil

	for i := 0; i < chunks; i++ {
		var (
			complete = i == chunks-1
			start    = int64(i) * chunkSize
			end      = start + chunkSize
		)
		if end > totalSize {
			end = totalSize
		}

		_addr := fmt.Sprintf(uploadUrl, start, complete)
		if nextContext != "" {
			_addr += "&context=" + nextContext
		}

		partData, err := splitFile(file, start, end)
		if err != nil {
			return nil, fmt.Errorf("splitFile: %w", err)
		}

		resp, err = a.client.Upload(ctx, _addr, headers, bytes.NewReader(partData), &reply, req.ProgressBar)
		log.Debug("upload addr: %s chunk %d/%d, offset: %d, complete: %v, resp: %+v",
			addr, i+1, chunks, start, complete, reply.ErrCode)
		if err != nil {
			return nil, fmt.Errorf("Upload: %w", err)
		}

		nextContext = reply.Context
		_ = resp
	}
	return &reply, nil
}

func splitFile(file *os.File, start, end int64) ([]byte, error) {
	var buf = make([]byte, end-start)
	_, err := file.ReadAt(buf, start)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}

type CloudInfoReq struct {
	types.ReqCommon
	// Md5 文件md5
	Md5 string `json:"md5,omitempty"`
	// SongId 歌曲id 从 CloudUploadCheck() api/cloud/upload/check接口返回值中获取
	SongId string `json:"songid,omitempty"`
	// Filename 文件名
	Filename string `json:"filename,omitempty"`
	// Song 歌曲名称
	Song string `json:"song,omitempty"`
	// Album 专辑名称
	Album string `json:"album,omitempty"`
	// Artist 艺术家
	Artist string `json:"artist,omitempty"`
	// Bitrate 比特率
	Bitrate    string `json:"bitrate,omitempty"`
	ResourceId int64  `json:"resourceId,omitempty"`
	// CoverId 封面id
	CoverId string `json:"coverid,omitempty"`
	// ObjectKey 在windows抓包发现需要上传此内容。更奇怪的是上传没有发现调用上传接口,
	// 而是有点像非秒传场景直接忽略了上传这个步骤，有一点可以确定的是,我上传的文件"检测文件接口"返回得是true。
	// 如果我按照windows上传方式传入此ObjectKey,此值时则会报以下错误:{"msg":"rep create failed","code":404}
	// 解决方案目前暂时不传值此值
	ObjectKey string `json:"objectKey,omitempty"`
}

type CloudInfoResp struct {
	// Code 404: 错误未知,目前在上传文件时文件大于200MB时出现此错误，经后来测试多试了几次重传发现又好了貌似是临时性错误，待确认排查。
	Code           int64        `json:"code,omitempty"`
	SongId         string       `json:"songId,omitempty"`
	WaitTime       int64        `json:"waitTime"`
	Exists         bool         `json:"exists"`
	NextUploadTime int64        `json:"nextUploadTime"`
	SongIdLong     int          `json:"songIdLong"`
	PrivateCloud   PrivateCloud `json:"privateCloud"`
}

type PrivateCloud struct {
	SimpleSong struct {
		Name                 string           `json:"name"`
		Id                   int              `json:"id"`
		Pst                  int              `json:"pst"`
		T                    int              `json:"t"`
		Ar                   []types.Artist   `json:"ar"`
		Alia                 []interface{}    `json:"alia"`
		Pop                  float64          `json:"pop"`
		St                   int              `json:"st"`
		Rt                   string           `json:"rt"`
		Fee                  int              `json:"fee"`
		V                    int              `json:"v"`
		Crbt                 interface{}      `json:"crbt"`
		Cf                   string           `json:"cf"`
		Al                   types.Album      `json:"al"`
		Dt                   int              `json:"dt"`
		H                    types.Quality    `json:"h"`
		M                    types.Quality    `json:"m"`
		L                    types.Quality    `json:"l"`
		A                    interface{}      `json:"a"`
		Cd                   string           `json:"cd"`
		No                   int              `json:"no"`
		RtUrl                interface{}      `json:"rtUrl"`
		Ftype                int              `json:"ftype"`
		RtUrls               []interface{}    `json:"rtUrls"`
		DjId                 int              `json:"djId"`
		Copyright            int              `json:"copyright"`
		SId                  int              `json:"s_id"`
		Mark                 int              `json:"mark"`
		OriginCoverType      int              `json:"originCoverType"`
		OriginSongSimpleData interface{}      `json:"originSongSimpleData"`
		Single               int              `json:"single"`
		NoCopyrightRcmd      interface{}      `json:"noCopyrightRcmd"`
		Mst                  int              `json:"mst"`
		Cp                   int              `json:"cp"`
		Mv                   int              `json:"mv"`
		Rtype                int              `json:"rtype"`
		Rurl                 interface{}      `json:"rurl"`
		PublishTime          int64            `json:"publishTime"`
		Privilege            types.Privileges `json:"privilege"`
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
// url: /testdata/9.har
// needLogin: 未知
func (a *Api) CloudInfo(ctx context.Context, req *CloudInfoReq) (*CloudInfoResp, error) {
	var (
		url   = "https://music.163.com/weapi/upload/cloud/info/v2" // 是api还是weapi？
		reply CloudInfoResp
		opts  = api.NewOptions()
	)
	if req.Album == "" {
		req.Album = "未知专辑"
	}
	if req.Artist == "" {
		req.Artist = "未知艺术家"
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type CloudMusicStatusReq struct {
	SongIds types.IntsString `json:"songIds"`
}

type CloudMusicStatusResp struct {
	types.RespCommon[any]
	// Key为歌曲的id
	Statuses map[string]CloudMusicStatusRespData `json:"statuses"`
}

type CloudMusicStatusRespData struct {
	// 0:成功 9:待转码貌似
	Status   int64 `json:"status"`
	WaitTime int64 `json:"waitTime"`
}

// CloudMusicStatus 查询上传文件状态信息,此接口貌似是上传文件后查询文件转码状态
// url: /testdata/10.har
// needLogin: 未知
func (a *Api) CloudMusicStatus(ctx context.Context, req *CloudMusicStatusReq) (*CloudMusicStatusResp, error) {
	var (
		url   = "https://music.163.com/weapi/v1/cloud/music/status"
		reply CloudMusicStatusResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
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
// url: testdata/har/13.har
// needLogin: 未知
func (a *Api) CloudPublish(ctx context.Context, req *CloudPublishReq) (*CloudPublishResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/cloud/pub/v2"
		reply CloudPublishResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type CloudDownloadReq struct {
	SongId string `json:"songId"`
}

type CloudDownloadResp struct {
	types.RespCommon[any]
	Name string `json:"name"`
	Url  string `json:"url"`
	// Size 单位字节(B)
	Size int64 `json:"size"`
}

// CloudDownload 云盘歌曲下载歌曲
// url: testdata/har/2.har
// needLogin: 未知
func (a *Api) CloudDownload(ctx context.Context, req *CloudDownloadReq) (*CloudDownloadResp, error) {
	var (
		url   = "https://music.163.com/weapi/cloud/dowonload"
		reply CloudDownloadResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type CloudLyricReq struct {
	UserId string `json:"userId,omitempty"`
	SongId string `json:"songId,omitempty"`
	Lv     string `json:"lv,omitempty"`
	Kv     string `json:"kv,omitempty"`
}

type CloudLyricResp struct {
	types.RespCommon[any]
	Lyc string `json:"lrc"`
	Krc string `json:"krc"`
}

// CloudLyric 云盘歌曲歌词获取
// url: testdata/har/3.har
// needLogin: 未知
func (a *Api) CloudLyric(ctx context.Context, req *CloudLyricReq) (*CloudLyricResp, error) {
	var (
		url   = "https://music.163.com/weapi/cloud/lyric/get"
		reply CloudLyricResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type CloudDelReq struct {
	SongIds types.IntsString `json:"songIds"`
}

type CloudDelResp struct {
	// Code 200:成功 404:删除失败(当重复删除同一个id时会出现)
	types.RespCommon[any]
	// FailIds 删除失败的歌曲id
	FailIds []int64 `json:"failIds"`
	// SuccIds 删除成功的歌曲id
	SuccIds []int64 `json:"succIds"`
}

// CloudDel 云盘歌曲删除
// url:
// needLogin: 未知
func (a *Api) CloudDel(ctx context.Context, req *CloudDelReq) (*CloudDelResp, error) {
	var (
		url   = "https://music.163.com/weapi/cloud/del"
		reply CloudDelResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

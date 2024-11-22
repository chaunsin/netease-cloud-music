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

type AreaId int64

const (
	AreaIdAll           AreaId = 0  // 全部
	AreaIdChinese       AreaId = 7  // 华语
	AreaIdJapan         AreaId = 8  // 日本
	AreaIdKorea         AreaId = 16 // 韩国
	AreaIdEuropeAmerica AreaId = 96 // 欧美
)

type TopNewSongsReq struct {
	AreaId AreaId `json:"areaId"`
	Total  bool   `json:"total"`
}

type TopNewSongsResp struct {
	types.RespCommon[[]TopNewSongsRespData]
}

// MusicQuality 和 type.Quality 类似
type MusicQuality struct {
	Bitrate     int64       `json:"bitrate"`
	DfsId       int         `json:"dfsId"`
	Extension   string      `json:"extension"`
	Id          int64       `json:"id"`
	Name        interface{} `json:"name"`
	PlayTime    int         `json:"playTime"`
	Size        int         `json:"size"`
	Sr          int         `json:"sr"`
	VolumeDelta float64     `json:"volumeDelta"`
}

type TopNewSongsRespData struct {
	Album struct {
		Alias  []string `json:"alias"`
		Artist struct {
			AlbumSize   int           `json:"albumSize"`
			Alias       []interface{} `json:"alias"`
			BriefDesc   string        `json:"briefDesc"`
			Followed    bool          `json:"followed"`
			Id          int           `json:"id"`
			Img1V1Id    int64         `json:"img1v1Id"`
			Img1V1IdStr string        `json:"img1v1Id_str"`
			Img1V1Url   string        `json:"img1v1Url"`
			MusicSize   int           `json:"musicSize"`
			Name        string        `json:"name"`
			PicId       int           `json:"picId"`
			PicUrl      string        `json:"picUrl"`
			TopicPerson int           `json:"topicPerson"`
			Trans       string        `json:"trans"`
		} `json:"artist"`
		Artists []struct {
			AlbumSize   int           `json:"albumSize"`
			Alias       []interface{} `json:"alias"`
			BriefDesc   string        `json:"briefDesc"`
			Followed    bool          `json:"followed"`
			Id          int           `json:"id"`
			Img1V1Id    int64         `json:"img1v1Id"`
			Img1V1IdStr string        `json:"img1v1Id_str"`
			Img1V1Url   string        `json:"img1v1Url"`
			MusicSize   int           `json:"musicSize"`
			Name        string        `json:"name"`
			PicId       int           `json:"picId"`
			PicUrl      string        `json:"picUrl"`
			TopicPerson int           `json:"topicPerson"`
			Trans       string        `json:"trans"`
		} `json:"artists"`
		BlurPicUrl      string      `json:"blurPicUrl"`
		BriefDesc       string      `json:"briefDesc"`
		CommentThreadId string      `json:"commentThreadId"`
		Company         string      `json:"company"`
		CompanyId       int         `json:"companyId"`
		CopyrightId     int         `json:"copyrightId"`
		Description     string      `json:"description"`
		Id              int         `json:"id"`
		Name            string      `json:"name"`
		OnSale          bool        `json:"onSale"`
		Paid            bool        `json:"paid"`
		Pic             int64       `json:"pic"`
		PicId           int64       `json:"picId"`
		PicIdStr        string      `json:"picId_str"`
		PicUrl          string      `json:"picUrl"`
		PublishTime     int64       `json:"publishTime"`
		Size            int         `json:"size"`
		Songs           interface{} `json:"songs"`
		Status          int         `json:"status"`
		SubType         string      `json:"subType"`
		Tags            string      `json:"tags"`
		Type            string      `json:"type"`
		TransNames      []string    `json:"transNames,omitempty"`
	} `json:"album"`
	AlbumData interface{} `json:"albumData"`
	Alias     []string    `json:"alias"`
	Artists   []struct {
		AlbumSize   int           `json:"albumSize"`
		Alias       []interface{} `json:"alias"`
		BriefDesc   string        `json:"briefDesc"`
		Followed    bool          `json:"followed"`
		Id          int           `json:"id"`
		Img1V1Id    int64         `json:"img1v1Id"`
		Img1V1IdStr string        `json:"img1v1Id_str"`
		Img1V1Url   string        `json:"img1v1Url"`
		MusicSize   int           `json:"musicSize"`
		Name        string        `json:"name"`
		PicId       int           `json:"picId"`
		PicUrl      string        `json:"picUrl"`
		TopicPerson int           `json:"topicPerson"`
		Trans       string        `json:"trans"`
	} `json:"artists"`
	Audition        interface{}  `json:"audition"`
	BMusic          MusicQuality `json:"bMusic"`
	CommentThreadId string       `json:"commentThreadId"`
	CopyFrom        string       `json:"copyFrom"`
	CopyrightId     int          `json:"copyrightId"`
	Crbt            interface{}  `json:"crbt"`
	DayPlays        int          `json:"dayPlays"`
	Disc            string       `json:"disc"`
	Duration        int          `json:"duration"`
	Exclusive       bool         `json:"exclusive"`
	Fee             int          `json:"fee"`
	Ftype           int          `json:"ftype"`
	HMusic          MusicQuality `json:"hMusic"`
	HearTime        int          `json:"hearTime"`
	Id              int64        `json:"id"`
	LMusic          MusicQuality `json:"lMusic"`
	MMusic          MusicQuality `json:"mMusic"`
	Mp3Url          string       `json:"mp3Url"`
	Mvid            int          `json:"mvid"`
	Name            string       `json:"name"`
	No              int          `json:"no"`
	PlayedNum       int          `json:"playedNum"`
	Popularity      float64      `json:"popularity"`
	Position        int          `json:"position"`
	Privilege       struct {
		ChargeInfoList []struct {
			ChargeMessage interface{} `json:"chargeMessage"`
			ChargeType    int         `json:"chargeType"`
			ChargeUrl     interface{} `json:"chargeUrl"`
			Rate          int         `json:"rate"`
		} `json:"chargeInfoList"`
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
	RelatedVideo interface{} `json:"relatedVideo"`
	Ringtone     string      `json:"ringtone"`
	RtUrl        interface{} `json:"rtUrl"`
	RtUrls       interface{} `json:"rtUrls"`
	Rtype        int         `json:"rtype"`
	Rurl         interface{} `json:"rurl"`
	Score        int         `json:"score"`
	St           int         `json:"st"`
	Starred      bool        `json:"starred"`
	StarredNum   int         `json:"starredNum"`
	Status       int         `json:"status"`
	VideoInfo    interface{} `json:"videoInfo"`
	TransNames   []string    `json:"transNames,omitempty"`
}

// TopNewSongs 新歌榜(新歌速递)
// url:
// needLogin: 未知
func (a *Api) TopNewSongs(ctx context.Context, req *TopNewSongsReq) (*TopNewSongsResp, error) {
	var (
		url   = "https://music.163.com/weapi/v1/discovery/new/songs"
		reply TopNewSongsResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type TopListReq struct{}

type TopListResp struct {
	types.RespCommon[any]
	List []TopListRespList `json:"list"`
}

type TopListRespList struct {
	ToplistType           string        `json:"ToplistType,omitempty"`
	AdType                int           `json:"adType"`
	AlgType               interface{}   `json:"algType"`
	Anonimous             bool          `json:"anonimous"`
	Artists               interface{}   `json:"artists"`
	BackgroundCoverId     int           `json:"backgroundCoverId"`
	BackgroundCoverUrl    interface{}   `json:"backgroundCoverUrl"`
	CloudTrackCount       int           `json:"cloudTrackCount"`
	CommentThreadId       string        `json:"commentThreadId"`
	CoverImageUrl         interface{}   `json:"coverImageUrl"`
	CoverImgId            int64         `json:"coverImgId"`
	CoverImgIdStr         string        `json:"coverImgId_str"`
	CoverImgUrl           string        `json:"coverImgUrl"`
	CoverText             interface{}   `json:"coverText"`
	CreateTime            int64         `json:"createTime"`
	Creator               interface{}   `json:"creator"`
	Description           *string       `json:"description"`
	EnglishTitle          interface{}   `json:"englishTitle"`
	HighQuality           bool          `json:"highQuality"`
	IconImageUrl          interface{}   `json:"iconImageUrl"`
	Id                    int64         `json:"id"`
	Name                  string        `json:"name"`
	NewImported           bool          `json:"newImported"`
	OpRecommend           bool          `json:"opRecommend"`
	Ordered               bool          `json:"ordered"`
	PlayCount             int64         `json:"playCount"`
	Privacy               int           `json:"privacy"`
	RecommendInfo         interface{}   `json:"recommendInfo"`
	SocialPlaylistCover   interface{}   `json:"socialPlaylistCover"`
	SpecialType           int           `json:"specialType"`
	Status                int           `json:"status"`
	Subscribed            interface{}   `json:"subscribed"`
	SubscribedCount       int           `json:"subscribedCount"`
	Subscribers           []interface{} `json:"subscribers"`
	Tags                  []string      `json:"tags"`
	TitleImage            int           `json:"titleImage"`
	TitleImageUrl         interface{}   `json:"titleImageUrl"`
	TotalDuration         int           `json:"totalDuration"`
	TrackCount            int           `json:"trackCount"`
	TrackNumberUpdateTime int64         `json:"trackNumberUpdateTime"`
	TrackUpdateTime       int64         `json:"trackUpdateTime"`
	Tracks                interface{}   `json:"tracks"`
	TsSongCount           int           `json:"tsSongCount"`
	UpdateFrequency       string        `json:"updateFrequency"`
	UpdateTime            int64         `json:"updateTime"`
	UserId                int64         `json:"userId"`
}

// TopList 排行榜列表,里面包含 飙升榜、热歌榜、新歌榜、原创榜.等等
// url: https://music.163.com/#/discover/toplist
// needLogin: 未知
func (a *Api) TopList(ctx context.Context, req *TopListReq) (*TopListResp, error) {
	var (
		url   = "https://music.163.com/api/toplist"
		reply TopListResp
		opts  = api.NewOptions()
	)

	opts.CryptoMode = api.CryptoModeAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

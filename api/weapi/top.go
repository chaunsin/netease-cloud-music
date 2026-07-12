// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

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
	Bitrate     int64   `json:"bitrate"`
	DfsId       int     `json:"dfsId"`
	Extension   string  `json:"extension"`
	Id          int64   `json:"id"`
	Name        any     `json:"name"`
	PlayTime    int     `json:"playTime"`
	Size        int     `json:"size"`
	Sr          int     `json:"sr"`
	VolumeDelta float64 `json:"volumeDelta"`
}

type TopNewSongsRespData struct {
	Album struct {
		Alias  []string `json:"alias"`
		Artist struct {
			AlbumSize   int    `json:"albumSize"`
			Alias       []any  `json:"alias"`
			BriefDesc   string `json:"briefDesc"`
			Followed    bool   `json:"followed"`
			Id          int    `json:"id"`
			Img1V1Id    int64  `json:"img1v1Id"`
			Img1V1IdStr string `json:"img1v1Id_str"`
			Img1V1Url   string `json:"img1v1Url"`
			MusicSize   int    `json:"musicSize"`
			Name        string `json:"name"`
			PicId       int    `json:"picId"`
			PicUrl      string `json:"picUrl"`
			TopicPerson int    `json:"topicPerson"`
			Trans       string `json:"trans"`
		} `json:"artist"`
		Artists []struct {
			AlbumSize   int    `json:"albumSize"`
			Alias       []any  `json:"alias"`
			BriefDesc   string `json:"briefDesc"`
			Followed    bool   `json:"followed"`
			Id          int    `json:"id"`
			Img1V1Id    int64  `json:"img1v1Id"`
			Img1V1IdStr string `json:"img1v1Id_str"`
			Img1V1Url   string `json:"img1v1Url"`
			MusicSize   int    `json:"musicSize"`
			Name        string `json:"name"`
			PicId       int    `json:"picId"`
			PicUrl      string `json:"picUrl"`
			TopicPerson int    `json:"topicPerson"`
			Trans       string `json:"trans"`
		} `json:"artists"`
		BlurPicUrl      string   `json:"blurPicUrl"`
		BriefDesc       string   `json:"briefDesc"`
		CommentThreadId string   `json:"commentThreadId"`
		Company         string   `json:"company"`
		CompanyId       int      `json:"companyId"`
		CopyrightId     int      `json:"copyrightId"`
		Description     string   `json:"description"`
		Id              int      `json:"id"`
		Name            string   `json:"name"`
		OnSale          bool     `json:"onSale"`
		Paid            bool     `json:"paid"`
		Pic             int64    `json:"pic"`
		PicId           int64    `json:"picId"`
		PicIdStr        string   `json:"picId_str"`
		PicUrl          string   `json:"picUrl"`
		PublishTime     int64    `json:"publishTime"`
		Size            int      `json:"size"`
		Songs           any      `json:"songs"`
		Status          int      `json:"status"`
		SubType         string   `json:"subType"`
		Tags            string   `json:"tags"`
		Type            string   `json:"type"`
		TransNames      []string `json:"transNames,omitempty"`
	} `json:"album"`
	AlbumData any      `json:"albumData"`
	Alias     []string `json:"alias"`
	Artists   []struct {
		AlbumSize   int    `json:"albumSize"`
		Alias       []any  `json:"alias"`
		BriefDesc   string `json:"briefDesc"`
		Followed    bool   `json:"followed"`
		Id          int    `json:"id"`
		Img1V1Id    int64  `json:"img1v1Id"`
		Img1V1IdStr string `json:"img1v1Id_str"`
		Img1V1Url   string `json:"img1v1Url"`
		MusicSize   int    `json:"musicSize"`
		Name        string `json:"name"`
		PicId       int    `json:"picId"`
		PicUrl      string `json:"picUrl"`
		TopicPerson int    `json:"topicPerson"`
		Trans       string `json:"trans"`
	} `json:"artists"`
	Audition        any          `json:"audition"`
	BMusic          MusicQuality `json:"bMusic"`
	CommentThreadId string       `json:"commentThreadId"`
	CopyFrom        string       `json:"copyFrom"`
	CopyrightId     int          `json:"copyrightId"`
	Crbt            any          `json:"crbt"`
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
			ChargeMessage any `json:"chargeMessage"`
			ChargeType    int `json:"chargeType"`
			ChargeUrl     any `json:"chargeUrl"`
			Rate          int `json:"rate"`
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
			CannotListenReason any  `json:"cannotListenReason"`
			ListenType         any  `json:"listenType"`
			PlayReason         any  `json:"playReason"`
			ResConsumable      bool `json:"resConsumable"`
			UserConsumable     bool `json:"userConsumable"`
		} `json:"freeTrialPrivilege"`
		Id             int64  `json:"id"`
		MaxBrLevel     string `json:"maxBrLevel"`
		Maxbr          int    `json:"maxbr"`
		Payed          int    `json:"payed"`
		Pl             int    `json:"pl"`
		PlLevel        string `json:"plLevel"`
		PlayMaxBrLevel string `json:"playMaxBrLevel"`
		PlayMaxbr      int    `json:"playMaxbr"`
		PreSell        bool   `json:"preSell"`
		RightSource    int    `json:"rightSource"`
		Rscl           any    `json:"rscl"`
		Sp             int    `json:"sp"`
		St             int    `json:"st"`
		Subp           int    `json:"subp"`
		Toast          bool   `json:"toast"`
	} `json:"privilege"`
	RelatedVideo any      `json:"relatedVideo"`
	Ringtone     string   `json:"ringtone"`
	RtUrl        any      `json:"rtUrl"`
	RtUrls       any      `json:"rtUrls"`
	Rtype        int      `json:"rtype"`
	Rurl         any      `json:"rurl"`
	Score        int      `json:"score"`
	St           int      `json:"st"`
	Starred      bool     `json:"starred"`
	StarredNum   int      `json:"starredNum"`
	Status       int      `json:"status"`
	VideoInfo    any      `json:"videoInfo"`
	TransNames   []string `json:"transNames,omitempty"`
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
		return nil, fmt.Errorf("request: %w", err)
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
	ToplistType           string   `json:"ToplistType,omitempty"`
	AdType                int      `json:"adType"`
	AlgType               any      `json:"algType"`
	Anonimous             bool     `json:"anonimous"`
	Artists               any      `json:"artists"`
	BackgroundCoverId     int      `json:"backgroundCoverId"`
	BackgroundCoverUrl    any      `json:"backgroundCoverUrl"`
	CloudTrackCount       int      `json:"cloudTrackCount"`
	CommentThreadId       string   `json:"commentThreadId"`
	CoverImageUrl         any      `json:"coverImageUrl"`
	CoverImgId            int64    `json:"coverImgId"`
	CoverImgIdStr         string   `json:"coverImgId_str"`
	CoverImgUrl           string   `json:"coverImgUrl"`
	CoverText             any      `json:"coverText"`
	CreateTime            int64    `json:"createTime"`
	Creator               any      `json:"creator"`
	Description           *string  `json:"description"`
	EnglishTitle          any      `json:"englishTitle"`
	HighQuality           bool     `json:"highQuality"`
	IconImageUrl          any      `json:"iconImageUrl"`
	Id                    int64    `json:"id"`
	Name                  string   `json:"name"`
	NewImported           bool     `json:"newImported"`
	OpRecommend           bool     `json:"opRecommend"`
	Ordered               bool     `json:"ordered"`
	PlayCount             int64    `json:"playCount"`
	Privacy               int      `json:"privacy"`
	RecommendInfo         any      `json:"recommendInfo"`
	SocialPlaylistCover   any      `json:"socialPlaylistCover"`
	SpecialType           int      `json:"specialType"`
	Status                int      `json:"status"`
	Subscribed            any      `json:"subscribed"`
	SubscribedCount       int      `json:"subscribedCount"`
	Subscribers           []any    `json:"subscribers"`
	Tags                  []string `json:"tags"`
	TitleImage            int      `json:"titleImage"`
	TitleImageUrl         any      `json:"titleImageUrl"`
	TotalDuration         int      `json:"totalDuration"`
	TrackCount            int      `json:"trackCount"`
	TrackNumberUpdateTime int64    `json:"trackNumberUpdateTime"`
	TrackUpdateTime       int64    `json:"trackUpdateTime"`
	Tracks                any      `json:"tracks"`
	TsSongCount           int      `json:"tsSongCount"`
	UpdateFrequency       string   `json:"updateFrequency"`
	UpdateTime            int64    `json:"updateTime"`
	UserId                int64    `json:"userId"`
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
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

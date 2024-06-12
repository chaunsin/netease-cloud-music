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

package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/chaunsin/netease-cloud-music/api/types"
)

type PlaylistDetailReq struct {
	Id string
}

type PlaylistDetailResp struct {
	types.ApiRespCommon[any]
	RelatedVideos interface{} `json:"relatedVideos"`
	Playlist      struct {
		Id                    int64         `json:"id"`
		Name                  string        `json:"name"`
		CoverImgId            int64         `json:"coverImgId"`
		CoverImgUrl           string        `json:"coverImgUrl"`
		CoverImgIdStr         string        `json:"coverImgId_str"`
		AdType                int           `json:"adType"`
		UserId                int           `json:"userId"`
		CreateTime            int64         `json:"createTime"`
		Status                int           `json:"status"`
		OpRecommend           bool          `json:"opRecommend"`
		HighQuality           bool          `json:"highQuality"`
		NewImported           bool          `json:"newImported"`
		UpdateTime            int64         `json:"updateTime"`
		TrackCount            int           `json:"trackCount"`
		SpecialType           int           `json:"specialType"`
		Privacy               int           `json:"privacy"`
		TrackUpdateTime       int64         `json:"trackUpdateTime"`
		CommentThreadId       string        `json:"commentThreadId"`
		PlayCount             int           `json:"playCount"`
		TrackNumberUpdateTime int64         `json:"trackNumberUpdateTime"`
		SubscribedCount       int           `json:"subscribedCount"`
		CloudTrackCount       int           `json:"cloudTrackCount"`
		Ordered               bool          `json:"ordered"`
		Description           string        `json:"description"`
		Tags                  []interface{} `json:"tags"`
		UpdateFrequency       interface{}   `json:"updateFrequency"`
		BackgroundCoverId     int           `json:"backgroundCoverId"`
		BackgroundCoverUrl    interface{}   `json:"backgroundCoverUrl"`
		TitleImage            int           `json:"titleImage"`
		TitleImageUrl         interface{}   `json:"titleImageUrl"`
		DetailPageTitle       interface{}   `json:"detailPageTitle"`
		EnglishTitle          interface{}   `json:"englishTitle"`
		OfficialPlaylistType  interface{}   `json:"officialPlaylistType"`
		Copied                bool          `json:"copied"`
		RelateResType         interface{}   `json:"relateResType"`
		CoverStatus           int           `json:"coverStatus"`
		Subscribers           []interface{} `json:"subscribers"`
		Subscribed            interface{}   `json:"subscribed"`
		Creator               struct {
			DefaultAvatar       bool        `json:"defaultAvatar"`
			Province            int         `json:"province"`
			AuthStatus          int         `json:"authStatus"`
			Followed            bool        `json:"followed"`
			AvatarUrl           string      `json:"avatarUrl"`
			AccountStatus       int         `json:"accountStatus"`
			Gender              int         `json:"gender"`
			City                int         `json:"city"`
			Birthday            int         `json:"birthday"`
			UserId              int         `json:"userId"`
			UserType            int         `json:"userType"`
			Nickname            string      `json:"nickname"`
			Signature           string      `json:"signature"`
			Description         string      `json:"description"`
			DetailDescription   string      `json:"detailDescription"`
			AvatarImgId         int64       `json:"avatarImgId"`
			BackgroundImgId     int64       `json:"backgroundImgId"`
			BackgroundUrl       string      `json:"backgroundUrl"`
			Authority           int         `json:"authority"`
			Mutual              bool        `json:"mutual"`
			ExpertTags          interface{} `json:"expertTags"`
			Experts             interface{} `json:"experts"`
			DjStatus            int         `json:"djStatus"`
			VipType             int         `json:"vipType"`
			RemarkName          interface{} `json:"remarkName"`
			AuthenticationTypes int         `json:"authenticationTypes"`
			AvatarDetail        struct {
				UserType        int    `json:"userType"`
				IdentityLevel   int    `json:"identityLevel"`
				IdentityIconUrl string `json:"identityIconUrl"`
			} `json:"avatarDetail"`
			AvatarImgIdStr     string `json:"avatarImgIdStr"`
			BackgroundImgIdStr string `json:"backgroundImgIdStr"`
			Anchor             bool   `json:"anchor"`
			AvatarImgIdStr1    string `json:"avatarImgId_str"`
		} `json:"creator"`
		Tracks []struct {
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
			Rt   *string       `json:"rt"`
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
			Dt int `json:"dt"`
			H  struct {
				Br   int     `json:"br"`
				Fid  int     `json:"fid"`
				Size int     `json:"size"`
				Vd   float64 `json:"vd"`
				Sr   int     `json:"sr"`
			} `json:"h"`
			M struct {
				Br   int     `json:"br"`
				Fid  int     `json:"fid"`
				Size int     `json:"size"`
				Vd   float64 `json:"vd"`
				Sr   int     `json:"sr"`
			} `json:"m"`
			L struct {
				Br   int     `json:"br"`
				Fid  int     `json:"fid"`
				Size int     `json:"size"`
				Vd   float64 `json:"vd"`
				Sr   int     `json:"sr"`
			} `json:"l"`
			Sq struct {
				Br   int     `json:"br"`
				Fid  int     `json:"fid"`
				Size int     `json:"size"`
				Vd   float64 `json:"vd"`
				Sr   int     `json:"sr"`
			} `json:"sq"`
			Hr *struct {
				Br   int     `json:"br"`
				Fid  int     `json:"fid"`
				Size int     `json:"size"`
				Vd   float64 `json:"vd"`
				Sr   int     `json:"sr"`
			} `json:"hr"`
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
			TagPicList           interface{}   `json:"tagPicList"`
			ResourceState        bool          `json:"resourceState"`
			Version              int           `json:"version"`
			SongJumpInfo         interface{}   `json:"songJumpInfo"`
			EntertainmentTags    interface{}   `json:"entertainmentTags"`
			AwardTags            interface{}   `json:"awardTags"`
			Single               int           `json:"single"`
			NoCopyrightRcmd      interface{}   `json:"noCopyrightRcmd"`
			Alg                  interface{}   `json:"alg"`
			DisplayReason        interface{}   `json:"displayReason"`
			Rtype                int           `json:"rtype"`
			Rurl                 interface{}   `json:"rurl"`
			Mst                  int           `json:"mst"`
			Cp                   int           `json:"cp"`
			Mv                   int           `json:"mv"`
			PublishTime          int64         `json:"publishTime"`
		} `json:"tracks"`
		VideoIds interface{} `json:"videoIds"`
		Videos   interface{} `json:"videos"`
		TrackIds []struct {
			Id         int         `json:"id"`
			V          int         `json:"v"`
			T          int         `json:"t"`
			At         int64       `json:"at"`
			Alg        interface{} `json:"alg"`
			Uid        int         `json:"uid"`
			RcmdReason string      `json:"rcmdReason"`
			Sc         interface{} `json:"sc"`
			F          interface{} `json:"f"`
			Sr         interface{} `json:"sr"`
			Dpr        interface{} `json:"dpr"`
		} `json:"trackIds"`
		BannedTrackIds          interface{}   `json:"bannedTrackIds"`
		MvResourceInfos         interface{}   `json:"mvResourceInfos"`
		ShareCount              int           `json:"shareCount"`
		CommentCount            int           `json:"commentCount"`
		RemixVideo              interface{}   `json:"remixVideo"`
		NewDetailPageRemixVideo interface{}   `json:"newDetailPageRemixVideo"`
		SharedUsers             interface{}   `json:"sharedUsers"`
		HistorySharedUsers      interface{}   `json:"historySharedUsers"`
		GradeStatus             string        `json:"gradeStatus"`
		Score                   interface{}   `json:"score"`
		AlgTags                 interface{}   `json:"algTags"`
		DistributeTags          []interface{} `json:"distributeTags"`
		TrialMode               int           `json:"trialMode"`
		DisplayTags             interface{}   `json:"displayTags"`
		PlaylistType            string        `json:"playlistType"`
	} `json:"playlist"`
	Urls       interface{} `json:"urls"`
	Privileges []struct {
		Id                 int         `json:"id"`
		Fee                int         `json:"fee"`
		Payed              int         `json:"payed"`
		RealPayed          int         `json:"realPayed"`
		St                 int         `json:"st"`
		Pl                 int         `json:"pl"`
		Dl                 int         `json:"dl"`
		Sp                 int         `json:"sp"`
		Cp                 int         `json:"cp"`
		Subp               int         `json:"subp"`
		Cs                 bool        `json:"cs"`
		Maxbr              int         `json:"maxbr"`
		Fl                 int         `json:"fl"`
		Pc                 interface{} `json:"pc"`
		Toast              bool        `json:"toast"`
		Flag               int         `json:"flag"`
		PaidBigBang        bool        `json:"paidBigBang"`
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
			CannotListenReason *int        `json:"cannotListenReason"`
			PlayReason         interface{} `json:"playReason"`
		} `json:"freeTrialPrivilege"`
		RightSource    int `json:"rightSource"`
		ChargeInfoList []struct {
			Rate          int         `json:"rate"`
			ChargeUrl     interface{} `json:"chargeUrl"`
			ChargeMessage interface{} `json:"chargeMessage"`
			ChargeType    int         `json:"chargeType"`
		} `json:"chargeInfoList"`
	} `json:"privileges"`
	SharedPrivilege interface{} `json:"sharedPrivilege"`
	ResEntrance     interface{} `json:"resEntrance"`
	FromUsers       interface{} `json:"fromUsers"`
	FromUserCount   int         `json:"fromUserCount"`
	SongFromUsers   interface{} `json:"songFromUsers"`
}

// PlaylistDetail 歌单列表 不需要认证
// https://music.163.com/api/v6/playlist/detail?id=9011496609
func (a *Api) PlaylistDetail(ctx context.Context, req *PlaylistDetailReq) (*PlaylistDetailResp, error) {
	var (
		url   = "https://music.163.com/api/v6/playlist/detail"
		reply PlaylistDetailResp
	)
	resp, err := a.client.Request(ctx, http.MethodGet, url, "api", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

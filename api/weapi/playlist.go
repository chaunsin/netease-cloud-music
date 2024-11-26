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

type PlaylistReq struct {
	Uid    string `json:"uid"`
	Offset string `json:"offset"`
	// Limit default 1000
	Limit string `json:"limit"`
}

type PlaylistResp struct {
	types.RespCommon[any]
	Version  string             `json:"version"` // 时间戳1703557080686
	More     bool               `json:"more"`
	Playlist []PlaylistRespList `json:"playlist"`
}

type PlaylistRespList struct {
	Subscribers []interface{} `json:"subscribers"`
	Subscribed  bool          `json:"subscribed"`
	Creator     struct {
		DefaultAvatar     bool     `json:"defaultAvatar"`
		Province          int64    `json:"province"`
		AuthStatus        int64    `json:"authStatus"`
		Followed          bool     `json:"followed"`
		AvatarUrl         string   `json:"avatarUrl"`
		AccountStatus     int64    `json:"accountStatus"`
		Gender            int64    `json:"gender"`
		City              int64    `json:"city"`
		Birthday          int64    `json:"birthday"`
		UserId            int64    `json:"userId"`
		UserType          int64    `json:"userType"`
		Nickname          string   `json:"nickname"`
		Signature         string   `json:"signature"`
		Description       string   `json:"description"`
		DetailDescription string   `json:"detailDescription"`
		AvatarImgId       int64    `json:"avatarImgId"`
		BackgroundImgId   int64    `json:"backgroundImgId"`
		BackgroundUrl     string   `json:"backgroundUrl"`
		Authority         int64    `json:"authority"`
		Mutual            bool     `json:"mutual"`
		ExpertTags        []string `json:"expertTags"`
		Experts           *struct {
			Field1 string `json:"2"`
		} `json:"experts"`
		DjStatus            int64       `json:"djStatus"`
		VipType             int64       `json:"vipType"`
		RemarkName          interface{} `json:"remarkName"`
		AuthenticationTypes int64       `json:"authenticationTypes"`
		AvatarDetail        interface{} `json:"avatarDetail"`
		BackgroundImgIdStr  string      `json:"backgroundImgIdStr"`
		AvatarImgIdStr      string      `json:"avatarImgIdStr"`
		Anchor              bool        `json:"anchor"`
		AvatarImgIdStr1     string      `json:"avatarImgId_str,omitempty"`
	} `json:"creator"`
	Artists            interface{} `json:"artists"`
	Tracks             interface{} `json:"tracks"`
	Top                bool        `json:"top"`
	UpdateFrequency    *string     `json:"updateFrequency"`
	BackgroundCoverId  int64       `json:"backgroundCoverId"`
	BackgroundCoverUrl *string     `json:"backgroundCoverUrl"`
	TitleImage         int64       `json:"titleImage"`
	TitleImageUrl      *string     `json:"titleImageUrl"`
	EnglishTitle       *string     `json:"englishTitle"`
	OpRecommend        bool        `json:"opRecommend"`
	RecommendInfo      *struct {
		Alg     string `json:"alg"`
		LogInfo string `json:"logInfo"`
	} `json:"recommendInfo"`
	SubscribedCount       int64       `json:"subscribedCount"`
	CloudTrackCount       int64       `json:"cloudTrackCount"`
	UserId                int64       `json:"userId"`
	TotalDuration         int64       `json:"totalDuration"`
	CoverImgId            int64       `json:"coverImgId"`
	Privacy               int64       `json:"privacy"`
	TrackUpdateTime       int64       `json:"trackUpdateTime"`
	TrackCount            int64       `json:"trackCount"`
	UpdateTime            int64       `json:"updateTime"`
	CommentThreadId       string      `json:"commentThreadId"`
	CoverImgUrl           string      `json:"coverImgUrl"`
	SpecialType           int64       `json:"specialType"`
	Anonimous             bool        `json:"anonimous"`
	CreateTime            int64       `json:"createTime"`
	HighQuality           bool        `json:"highQuality"`
	NewImported           bool        `json:"newImported"`
	TrackNumberUpdateTime int64       `json:"trackNumberUpdateTime"`
	PlayCount             int64       `json:"playCount"`
	AdType                int64       `json:"adType"`
	Description           *string     `json:"description"`
	Tags                  []string    `json:"tags"`
	Ordered               bool        `json:"ordered"`
	Status                int64       `json:"status"`
	Name                  string      `json:"name"`
	Id                    int64       `json:"id"`
	CoverImgIdStr         *string     `json:"coverImgId_str"`
	SharedUsers           interface{} `json:"sharedUsers"`
	ShareStatus           interface{} `json:"shareStatus"`
	Copied                bool        `json:"copied"`
}

// Playlist 歌单列表.其中包含用户创建得歌单+我喜欢得歌单
// url: https://app.apifox.com/project/3870894 testdata/har/4.har
// NeedLogin: 未知
func (a *Api) Playlist(ctx context.Context, req *PlaylistReq) (*PlaylistResp, error) {
	var (
		url   = "https://music.163.com/weapi/user/playlist/"
		reply PlaylistResp
		opts  = api.NewOptions()
	)
	if req.Limit == "" {
		req.Limit = "1000"
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PlaylistDetailReq struct {
	Id string `json:"id"` // 歌单id 从接口 Playlist() 中获取
	N  string `json:"n"`  // 数值类型，未知通常可以为0
	S  string `json:"s"`  // 数值类型，歌单最近得S个收藏者 see: https://docs-neteasecloudmusicapi.vercel.app/docs/#/?id=%e8%8e%b7%e5%8f%96%e6%ad%8c%e5%8d%95%e8%af%a6%e6%83%85
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
		AdType                int64         `json:"adType"`
		UserId                int64         `json:"userId"`
		CreateTime            int64         `json:"createTime"`
		Status                int64         `json:"status"`
		OpRecommend           bool          `json:"opRecommend"`
		HighQuality           bool          `json:"highQuality"`
		NewImported           bool          `json:"newImported"`
		UpdateTime            int64         `json:"updateTime"`
		TrackCount            int64         `json:"trackCount"`
		SpecialType           int64         `json:"specialType"`
		Privacy               int64         `json:"privacy"`
		TrackUpdateTime       int64         `json:"trackUpdateTime"`
		CommentThreadId       string        `json:"commentThreadId"`
		PlayCount             int64         `json:"playCount"`
		TrackNumberUpdateTime int64         `json:"trackNumberUpdateTime"`
		SubscribedCount       int64         `json:"subscribedCount"`
		CloudTrackCount       int64         `json:"cloudTrackCount"`
		Ordered               bool          `json:"ordered"`
		Description           string        `json:"description"`
		Tags                  []interface{} `json:"tags"`
		UpdateFrequency       interface{}   `json:"updateFrequency"`
		BackgroundCoverId     int64         `json:"backgroundCoverId"`
		BackgroundCoverUrl    interface{}   `json:"backgroundCoverUrl"`
		TitleImage            int64         `json:"titleImage"`
		TitleImageUrl         interface{}   `json:"titleImageUrl"`
		DetailPageTitle       interface{}   `json:"detailPageTitle"`
		EnglishTitle          interface{}   `json:"englishTitle"`
		OfficialPlaylistType  interface{}   `json:"officialPlaylistType"`
		Copied                bool          `json:"copied"`
		RelateResType         interface{}   `json:"relateResType"`
		CoverStatus           int64         `json:"coverStatus"`
		Subscribers           []interface{} `json:"subscribers"`
		Subscribed            interface{}   `json:"subscribed"`
		Creator               struct {
			DefaultAvatar       bool        `json:"defaultAvatar"`
			Province            int64       `json:"province"`
			AuthStatus          int64       `json:"authStatus"`
			Followed            bool        `json:"followed"`
			AvatarUrl           string      `json:"avatarUrl"`
			AccountStatus       int64       `json:"accountStatus"`
			Gender              int64       `json:"gender"`
			City                int64       `json:"city"`
			Birthday            int64       `json:"birthday"`
			UserId              int64       `json:"userId"`
			UserType            int64       `json:"userType"`
			Nickname            string      `json:"nickname"`
			Signature           string      `json:"signature"`
			Description         string      `json:"description"`
			DetailDescription   string      `json:"detailDescription"`
			AvatarImgId         int64       `json:"avatarImgId"`
			BackgroundImgId     int64       `json:"backgroundImgId"`
			BackgroundUrl       string      `json:"backgroundUrl"`
			Authority           int64       `json:"authority"`
			Mutual              bool        `json:"mutual"`
			ExpertTags          interface{} `json:"expertTags"`
			Experts             interface{} `json:"experts"`
			DjStatus            int64       `json:"djStatus"`
			VipType             int64       `json:"vipType"`
			RemarkName          interface{} `json:"remarkName"`
			AuthenticationTypes int64       `json:"authenticationTypes"`
			AvatarDetail        struct {
				UserType        int64  `json:"userType"`
				IdentityLevel   int    `json:"identityLevel"`
				IdentityIconUrl string `json:"identityIconUrl"`
			} `json:"avatarDetail"`
			AvatarImgIdStr     string `json:"avatarImgIdStr"`
			BackgroundImgIdStr string `json:"backgroundImgIdStr"`
			Anchor             bool   `json:"anchor"`
			AvatarImgIdStr1    string `json:"avatarImgId_str"`
		} `json:"creator"`
		// Tracks 只包含了10首歌曲详情信息,而 TrackIds 包含歌曲所有的信息id不包含歌曲详情因此需要配合详情接口查询
		Tracks []struct {
			Name                 string         `json:"name"`
			Id                   int64          `json:"id"`
			Pst                  int64          `json:"pst"`
			T                    int64          `json:"t"`
			Ar                   []types.Artist `json:"ar"`
			Alia                 []interface{}  `json:"alia"`
			Pop                  float64        `json:"pop"`
			St                   int64          `json:"st"`
			Rt                   *string        `json:"rt"`
			Fee                  int64          `json:"fee"`
			V                    int64          `json:"v"`
			Crbt                 interface{}    `json:"crbt"`
			Cf                   string         `json:"cf"`
			Al                   types.Album    `json:"al"`
			Dt                   int64          `json:"dt"`
			H                    types.Quality  `json:"h"`
			M                    types.Quality  `json:"m"`
			L                    types.Quality  `json:"l"`
			Sq                   types.Quality  `json:"sq"`
			Hr                   types.Quality  `json:"hr"`
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
			TagPicList           interface{}    `json:"tagPicList"`
			ResourceState        bool           `json:"resourceState"`
			Version              int64          `json:"version"`
			SongJumpInfo         interface{}    `json:"songJumpInfo"`
			EntertainmentTags    interface{}    `json:"entertainmentTags"`
			AwardTags            interface{}    `json:"awardTags"`
			Single               int64          `json:"single"`
			NoCopyrightRcmd      interface{}    `json:"noCopyrightRcmd"`
			Alg                  interface{}    `json:"alg"`
			DisplayReason        interface{}    `json:"displayReason"`
			Rtype                int64          `json:"rtype"`
			Rurl                 interface{}    `json:"rurl"`
			Mst                  int64          `json:"mst"`
			Cp                   int64          `json:"cp"`
			Mv                   int64          `json:"mv"`
			PublishTime          int64          `json:"publishTime"`
		} `json:"tracks"`
		VideoIds interface{} `json:"videoIds"`
		Videos   interface{} `json:"videos"`
		TrackIds []struct {
			Id         int64       `json:"id"`
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
		ShareCount              int64         `json:"shareCount"`
		CommentCount            int64         `json:"commentCount"`
		RemixVideo              interface{}   `json:"remixVideo"`
		NewDetailPageRemixVideo interface{}   `json:"newDetailPageRemixVideo"`
		SharedUsers             interface{}   `json:"sharedUsers"`
		HistorySharedUsers      interface{}   `json:"historySharedUsers"`
		GradeStatus             string        `json:"gradeStatus"`
		Score                   interface{}   `json:"score"`
		AlgTags                 interface{}   `json:"algTags"`
		DistributeTags          []interface{} `json:"distributeTags"`
		TrialMode               int64         `json:"trialMode"`
		DisplayTags             interface{}   `json:"displayTags"`
		PlaylistType            string        `json:"playlistType"`
	} `json:"playlist"`
	Urls            interface{}        `json:"urls"`
	Privileges      []types.Privileges `json:"privileges"`
	SharedPrivilege interface{}        `json:"sharedPrivilege"`
	ResEntrance     interface{}        `json:"resEntrance"`
	FromUsers       interface{}        `json:"fromUsers"`
	FromUserCount   int64              `json:"fromUserCount"`
	SongFromUsers   interface{}        `json:"songFromUsers"`
}

// PlaylistDetail 歌单列表
// url: testdata/har/7.har
// needLogin: 不需要认证
// https://music.163.com/api/v6/playlist/detail?id=1981392816
func (a *Api) PlaylistDetail(ctx context.Context, req *PlaylistDetailReq) (*PlaylistDetailResp, error) {
	var (
		url   = "https://music.163.com/api/v6/playlist/detail?id=" + req.Id
		reply PlaylistDetailResp
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

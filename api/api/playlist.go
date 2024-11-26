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

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type PlaylistDetailReq struct {
	Id string `json:"id"` // 歌单id 从接口中获取 eapi/user/playlist/
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
			Name                 string         `json:"name"`
			Id                   int            `json:"id"`
			Pst                  int            `json:"pst"`
			T                    int            `json:"t"`
			Ar                   []types.Artist `json:"ar"`
			Alia                 []interface{}  `json:"alia"`
			Pop                  float64        `json:"pop"`
			St                   int            `json:"st"`
			Rt                   *string        `json:"rt"`
			Fee                  int            `json:"fee"`
			V                    int            `json:"v"`
			Crbt                 interface{}    `json:"crbt"`
			Cf                   string         `json:"cf"`
			Al                   types.Album    `json:"al"`
			Dt                   int            `json:"dt"`
			H                    *types.Quality `json:"h"`
			M                    *types.Quality `json:"m"`
			L                    *types.Quality `json:"l"`
			Sq                   *types.Quality `json:"sq"`
			Hr                   *types.Quality `json:"hr"`
			A                    interface{}    `json:"a"`
			Cd                   string         `json:"cd"`
			No                   int            `json:"no"`
			RtUrl                interface{}    `json:"rtUrl"`
			Ftype                int            `json:"ftype"`
			RtUrls               []interface{}  `json:"rtUrls"`
			DjId                 int            `json:"djId"`
			Copyright            int            `json:"copyright"`
			SId                  int            `json:"s_id"`
			Mark                 int64          `json:"mark"`
			OriginCoverType      int            `json:"originCoverType"`
			OriginSongSimpleData interface{}    `json:"originSongSimpleData"`
			TagPicList           interface{}    `json:"tagPicList"`
			ResourceState        bool           `json:"resourceState"`
			Version              int            `json:"version"`
			SongJumpInfo         interface{}    `json:"songJumpInfo"`
			EntertainmentTags    interface{}    `json:"entertainmentTags"`
			AwardTags            interface{}    `json:"awardTags"`
			Single               int            `json:"single"`
			NoCopyrightRcmd      interface{}    `json:"noCopyrightRcmd"`
			Alg                  interface{}    `json:"alg"`
			DisplayReason        interface{}    `json:"displayReason"`
			Rtype                int            `json:"rtype"`
			Rurl                 interface{}    `json:"rurl"`
			Mst                  int            `json:"mst"`
			Cp                   int            `json:"cp"`
			Mv                   int            `json:"mv"`
			PublishTime          int64          `json:"publishTime"`
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
	Urls            interface{}        `json:"urls"`
	Privileges      []types.Privileges `json:"privileges"`
	SharedPrivilege interface{}        `json:"sharedPrivilege"`
	ResEntrance     interface{}        `json:"resEntrance"`
	FromUsers       interface{}        `json:"fromUsers"`
	FromUserCount   int                `json:"fromUserCount"`
	SongFromUsers   interface{}        `json:"songFromUsers"`
}

// PlaylistDetail 歌单列表
// url: testdata/har/7.har
// needLogin: 不需要认证
// https://music.163.com/api/v6/playlist/detail?id=9011496609
func (a *Api) PlaylistDetail(ctx context.Context, req *PlaylistDetailReq) (*PlaylistDetailResp, error) {
	var (
		url   = "https://music.163.com/api/v6/playlist/detail"
		reply PlaylistDetailResp
		opts  = api.NewOptions()
	)
	opts.Method = http.MethodGet
	opts.CryptoMode = api.CryptoModeAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

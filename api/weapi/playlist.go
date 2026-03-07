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
func (a *Api) PlaylistDetail(ctx context.Context, req *PlaylistDetailReq) (*PlaylistDetailResp, error) {
	var (
		url   = "https://music.163.com/api/v6/playlist/detail"
		reply PlaylistDetailResp
		opts  = api.NewOptions()
	)

	if req.N == "" {
		req.N = "100000"
	}
	if req.S == "" {
		req.S = "8"
	}

	opts.CryptoMode = api.CryptoModeAPI
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type RadioReq struct {
	types.ReqCommon
	ImageFm string `json:"imageFm"` // 0: 1: 待分析
}

type RadioResp struct {
	types.RespCommon[[]RadioRespData]
	PopAdjust bool        `json:"popAdjust"`
	Tag       interface{} `json:"tag"`
}

type RadioRespDataMusic struct {
	Name        interface{} `json:"name"`
	Id          int64       `json:"id"`
	Size        int64       `json:"size"`
	Extension   string      `json:"extension"`
	Sr          int64       `json:"sr"`
	DfsId       int64       `json:"dfsId"`
	Bitrate     int64       `json:"bitrate"`
	PlayTime    int64       `json:"playTime"`
	VolumeDelta float64     `json:"volumeDelta"`
}

type RadioRespDataArtist struct {
	Name      string        `json:"name"`
	Id        int64         `json:"id"`
	PicId     int64         `json:"picId"`
	Img1V1Id  int64         `json:"img1v1Id"`
	BriefDesc string        `json:"briefDesc"`
	PicUrl    string        `json:"picUrl"`
	Img1V1Url string        `json:"img1v1Url"`
	AlbumSize int64         `json:"albumSize"`
	Alias     []interface{} `json:"alias"`
	Trans     string        `json:"trans"`
	MusicSize int64         `json:"musicSize"`
}

type RadioRespData struct {
	Name        string                `json:"name"`
	Id          int64                 `json:"id"`
	Position    int64                 `json:"position"`
	Alias       []interface{}         `json:"alias"`
	Status      int64                 `json:"status"`
	Fee         int64                 `json:"fee"`
	CopyrightId int64                 `json:"copyrightId"`
	Disc        string                `json:"disc"`
	No          int64                 `json:"no"`
	Artists     []RadioRespDataArtist `json:"artists"`
	Album       struct {
		Name            string                `json:"name"`
		Id              int64                 `json:"id"`
		Type            string                `json:"type"`
		Size            int64                 `json:"size"`
		PicId           int64                 `json:"picId"`
		BlurPicUrl      string                `json:"blurPicUrl"`
		CompanyId       int64                 `json:"companyId"`
		Pic             int64                 `json:"pic"`
		PicUrl          string                `json:"picUrl"`
		PublishTime     int64                 `json:"publishTime"`
		Description     string                `json:"description"`
		Tags            string                `json:"tags"`
		Company         string                `json:"company"`
		BriefDesc       string                `json:"briefDesc"`
		Artist          RadioRespDataArtist   `json:"artist"`
		Songs           []interface{}         `json:"songs"`
		Alias           []interface{}         `json:"alias"`
		Status          int64                 `json:"status"`
		CopyrightId     int64                 `json:"copyrightId"`
		CommentThreadId string                `json:"commentThreadId"`
		Artists         []RadioRespDataArtist `json:"artists"`
		SubType         string                `json:"subType"`
		TransName       interface{}           `json:"transName"`
		PicIdStr        string                `json:"picId_str,omitempty"`
	} `json:"album"`
	Starred         bool               `json:"starred"`
	Popularity      float64            `json:"popularity"`
	Score           int64              `json:"score"`
	StarredNum      int64              `json:"starredNum"`
	Duration        int64              `json:"duration"`
	PlayedNum       int64              `json:"playedNum"`
	DayPlays        int64              `json:"dayPlays"`
	HearTime        int64              `json:"hearTime"`
	Ringtone        *string            `json:"ringtone"`
	Crbt            interface{}        `json:"crbt"`
	Audition        interface{}        `json:"audition"`
	CopyFrom        string             `json:"copyFrom"`
	CommentThreadId string             `json:"commentThreadId"`
	RtUrl           interface{}        `json:"rtUrl"`
	Ftype           int64              `json:"ftype"`
	RtUrls          []interface{}      `json:"rtUrls"`
	Copyright       int64              `json:"copyright"`
	TransName       interface{}        `json:"transName"`
	Sign            interface{}        `json:"sign"`
	HMusic          RadioRespDataMusic `json:"hMusic"`
	MMusic          RadioRespDataMusic `json:"mMusic"`
	LMusic          RadioRespDataMusic `json:"lMusic"`
	BMusic          RadioRespDataMusic `json:"bMusic"`
	Rtype           int64              `json:"rtype"`
	Rurl            interface{}        `json:"rurl"`
	Mvid            int64              `json:"mvid"`
	Mp3Url          interface{}        `json:"mp3Url"`
	Privilege       struct {
		Id                 int64                    `json:"id"`
		Fee                int64                    `json:"fee"`
		Payed              int64                    `json:"payed"`
		RealPayed          int64                    `json:"realPayed"`
		St                 int64                    `json:"st"`
		Pl                 int64                    `json:"pl"`
		Dl                 int64                    `json:"dl"`
		Sp                 int64                    `json:"sp"`
		Cp                 int64                    `json:"cp"`
		Subp               int64                    `json:"subp"`
		Cs                 bool                     `json:"cs"`
		Maxbr              int64                    `json:"maxbr"`
		Fl                 int64                    `json:"fl"`
		Pc                 interface{}              `json:"pc"`
		Toast              bool                     `json:"toast"`
		Flag               int64                    `json:"flag"`
		PaidBigBang        bool                     `json:"paidBigBang"`
		PreSell            bool                     `json:"preSell"`
		PlayMaxbr          int64                    `json:"playMaxbr"`
		DownloadMaxbr      int64                    `json:"downloadMaxbr"`
		MaxBrLevel         string                   `json:"maxBrLevel"`
		PlayMaxBrLevel     string                   `json:"playMaxBrLevel"`
		DownloadMaxBrLevel string                   `json:"downloadMaxBrLevel"`
		PlLevel            string                   `json:"plLevel"`
		DlLevel            string                   `json:"dlLevel"`
		FlLevel            string                   `json:"flLevel"`
		Rscl               interface{}              `json:"rscl"`
		FreeTrialPrivilege types.FreeTrialPrivilege `json:"freeTrialPrivilege"`
		RightSource        int64                    `json:"rightSource"`
		ChargeInfoList     []types.ChargeInfo       `json:"chargeInfoList"`
		Code               int64                    `json:"code"`
		Message            interface{}              `json:"message"`
		PlLevels           interface{}              `json:"plLevels"`
		DlLevels           interface{}              `json:"dlLevels"`
	} `json:"privilege"`
	Alg string `json:"alg"`
}

// Radio 私人漫游歌单
// har: 32.har
// todo: 目前貌似今日首次进入为1,然后之后都为0，另外接口没有发现分页参数，另外貌似每次调用返回结果都不一样
func (a *Api) Radio(ctx context.Context, req *RadioReq) (*RadioResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/v1/radio/get"
		reply RadioResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PCRecentListenListReq struct {
	types.ReqCommon
}

type PCRecentListenListResp struct {
	types.RespCommon[PCRecentListenListRespData]
}

type PCRecentListenListRespData struct {
	Title     string `json:"title"` // eg: 最近常听
	Resources []struct {
		ResourceId       int64       `json:"resourceId"` // 资源id,可以是歌单、或者其它类型,根据ResourceType来决定
		ResourceCode     interface{} `json:"resourceCode"`
		ResourceType     string      `json:"resourceType"` // list:歌单 userfm:私人漫游
		SubResourceId    string      `json:"subResourceId"`
		Title            string      `json:"title"` // eg:摇滚、私人漫游、等等
		Tag              string      `json:"tag"`   // eg:歌单、漫游、等等
		CoverUrlList     []string    `json:"coverUrlList"`
		LandingUrl       string      `json:"landingUrl"`
		Update           bool        `json:"update"`
		PlayOrUpdateTime int64       `json:"playOrUpdateTime"`
		SimilarFmType    interface{} `json:"similarFmType"`
		DateNum          interface{} `json:"dateNum"`
		Star             bool        `json:"star"`
		CoverCenter      bool        `json:"coverCenter"`
		CoverExt         *struct {
			CoverId   string      `json:"coverId"`
			CoverType string      `json:"coverType"`
			CoverAlg  interface{} `json:"coverAlg"`
		} `json:"coverExt"`
		SongIdList interface{} `json:"songIdList"`
		PlayIndex  interface{} `json:"playIndex"`
	} `json:"resources"`
}

// PCRecentListenList PC端查看最近播放歌单
// har: 38.har
func (a *Api) PCRecentListenList(ctx context.Context, req *PCRecentListenListReq) (*PCRecentListenListResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/pc/recent/listen/list"
		reply PCRecentListenListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type PlaylistAddOrDelReq struct {
	Op       string           `json:"op"`       // 增加歌曲为add,删除为del,更新顺序为update
	Pid      int64            `json:"pid"`      // 歌单id
	TrackIds types.IntsString `json:"trackIds"` // 歌曲id (传入格式如types.IntsString{349823, 423521})
	Imme     bool             `json:"imme"`     // 是否立刻上传(默认为true),实际检测不会产生太大影响,猜测为了防止阻塞残留
}

type PlaylistAddOrDelResp struct {
	types.RespCommon[any]
	TrackIds   string `json:"trackIds"`   // 成功添加的歌曲id(返回为string类型数组如"[349823,423521]")
	Count      int64  `json:"count"`      // 该歌单歌曲数量(添加后)
	CloudCount int64  `json:"cloudCount"` // 该歌单内云盘歌曲数量(添加后)
}

// PlaylistAddOrDel 对歌单添加或删除歌曲
// code message:502 歌单歌曲重复, 404 歌单不存在(包含没有权限添加的歌单), 400 当前歌曲已下架，无法收藏哦(包含不存在的歌曲id)
func (a *Api) PlaylistAddOrDel(ctx context.Context, req *PlaylistAddOrDelReq) (*PlaylistAddOrDelResp, error) {
	var (
		url   = "https://music.163.com/weapi/playlist/manipulate/tracks"
		reply PlaylistAddOrDelResp
		opts  = api.NewOptions()
	)
	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

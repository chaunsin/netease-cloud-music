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

package eapi

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

type PlaylistResp struct {
	types.RespCommon[any]
	Version  string             `json:"version"` // 时间戳1703557080686
	More     bool               `json:"more"`
	Playlist []PlaylistRespList `json:"playlist"`
}

// Playlist 歌单列表.其中包含用户创建得歌单+我喜欢得歌单
// url: https://app.apifox.com/project/3870894 testdata/har/4.har
// NeedLogin: 未知
func (a *Api) Playlist(ctx context.Context, req *PlaylistReq) (*PlaylistResp, error) {
	var (
		url   = "https://music.163.com/eapi/user/playlist/"
		reply PlaylistResp
		opts  = api.NewOptions()
	)
	opts.CryptoMode = api.CryptoModeEAPI
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

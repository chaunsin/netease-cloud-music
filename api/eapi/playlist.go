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
	"net/http"

	"github.com/chaunsin/netease-cloud-music/api"
)

type PlaylistReq struct {
}

type PlaylistRespList struct {
	AdType             int         `json:"adType"`
	Anonimous          bool        `json:"anonimous"`
	Artists            interface{} `json:"artists"`
	BackgroundCoverId  int64       `json:"backgroundCoverId"`
	BackgroundCoverUrl *string     `json:"backgroundCoverUrl"`
	CloudTrackCount    int         `json:"cloudTrackCount"`
	CommentThreadId    string      `json:"commentThreadId"`
	Copied             bool        `json:"copied"`
	CoverImgId         int64       `json:"coverImgId"`
	CoverImgIdStr      *string     `json:"coverImgId_str"`
	CoverImgUrl        string      `json:"coverImgUrl"`
	CreateTime         int64       `json:"createTime"`
	Creator            struct {
		AccountStatus       int         `json:"accountStatus"`
		Anchor              bool        `json:"anchor"`
		AuthStatus          int         `json:"authStatus"`
		AuthenticationTypes int         `json:"authenticationTypes"`
		Authority           int         `json:"authority"`
		AvatarDetail        interface{} `json:"avatarDetail"`
		AvatarImgId         int64       `json:"avatarImgId"`
		AvatarImgIdStr      string      `json:"avatarImgIdStr"`
		AvatarImgIdStr1     string      `json:"avatarImgId_str,omitempty"`
		AvatarUrl           string      `json:"avatarUrl"`
		BackgroundImgId     int64       `json:"backgroundImgId"`
		BackgroundImgIdStr  string      `json:"backgroundImgIdStr"`
		BackgroundUrl       string      `json:"backgroundUrl"`
		Birthday            int         `json:"birthday"`
		City                int         `json:"city"`
		DefaultAvatar       bool        `json:"defaultAvatar"`
		Description         string      `json:"description"`
		DetailDescription   string      `json:"detailDescription"`
		DjStatus            int         `json:"djStatus"`
		ExpertTags          []string    `json:"expertTags"`
		Experts             *struct {
			Field1 string `json:"2"`
		} `json:"experts"`
		Followed   bool        `json:"followed"`
		Gender     int         `json:"gender"`
		Mutual     bool        `json:"mutual"`
		Nickname   string      `json:"nickname"`
		Province   int         `json:"province"`
		RemarkName interface{} `json:"remarkName"`
		Signature  string      `json:"signature"`
		UserId     int         `json:"userId"`
		UserType   int         `json:"userType"`
		VipType    int         `json:"vipType"`
	} `json:"creator"`
	Description   *string `json:"description"`
	EnglishTitle  *string `json:"englishTitle"`
	HighQuality   bool    `json:"highQuality"`
	Id            int64   `json:"id"`
	Name          string  `json:"name"`
	NewImported   bool    `json:"newImported"`
	OpRecommend   bool    `json:"opRecommend"`
	Ordered       bool    `json:"ordered"`
	PlayCount     int64   `json:"playCount"`
	Privacy       int     `json:"privacy"`
	RecommendInfo *struct {
		Alg     string `json:"alg"`
		LogInfo string `json:"logInfo"`
	} `json:"recommendInfo"`
	ShareStatus           interface{}   `json:"shareStatus"`
	SharedUsers           interface{}   `json:"sharedUsers"`
	SpecialType           int           `json:"specialType"`
	Status                int           `json:"status"`
	Subscribed            bool          `json:"subscribed"`
	SubscribedCount       int           `json:"subscribedCount"`
	Subscribers           []interface{} `json:"subscribers"`
	Tags                  []string      `json:"tags"`
	TitleImage            int64         `json:"titleImage"`
	TitleImageUrl         *string       `json:"titleImageUrl"`
	Top                   bool          `json:"top"`
	TotalDuration         int           `json:"totalDuration"`
	TrackCount            int           `json:"trackCount"`
	TrackNumberUpdateTime int64         `json:"trackNumberUpdateTime"`
	TrackUpdateTime       int64         `json:"trackUpdateTime"`
	Tracks                interface{}   `json:"tracks"`
	UpdateFrequency       *string       `json:"updateFrequency"`
	UpdateTime            int64         `json:"updateTime"`
	UserId                int           `json:"userId"`
}

type PlaylistResp struct {
	api.RespCommon[any]
	Playlist PlaylistRespList `json:"playlist"`
	Version  string           `json:"version"` // 时间戳1703557080686
}

// Playlist 自创建的歌单列表
// url: https://app.apifox.com/project/3870894
func (a *Api) Playlist(ctx context.Context, req *RefreshTokenReq) (*PlaylistResp, error) {
	var (
		url   = "https://music.163.com/eapi/user/playlist/"
		reply PlaylistResp
	)
	resp, err := a.client.Request(ctx, http.MethodPost, url, "eapi", req, &reply)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// MIT License
//
// Copyright (c) 2025 chaunsin
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

type GetUserInfoDetailReq struct {
	types.ReqCommon
	UserId int64 `json:"userId"`
}

type GetUserInfoDetailResp struct {
	Code        int64 `json:"code"`  // 200:成功 404:未找到用户
	Level       int64 `json:"level"` // 账号等级
	ListenSongs int64 `json:"listenSongs"`
	// UserPoint 云贝信息
	UserPoint struct {
		UserId       int64 `json:"userId"`
		Balance      int64 `json:"balance"` // 云贝数量
		UpdateTime   int64 `json:"updateTime"`
		Version      int64 `json:"version"` // 版本(貌似表示此工能版本)
		Status       int64 `json:"status"`
		BlockBalance int64 `json:"blockBalance"` // 冻结数量
	} `json:"userPoint"`
	MobileSign bool `json:"mobileSign"`
	PcSign     bool `json:"pcSign"`
	Profile    struct {
		PrivacyItemUnlimit struct {
			Area       bool `json:"area"`
			College    bool `json:"college"`
			Gender     bool `json:"gender"`
			Age        bool `json:"age"`
			VillageAge bool `json:"villageAge"`
		} `json:"privacyItemUnlimit"`
		AvatarDetail              interface{}   `json:"avatarDetail"`
		CreateTime                int64         `json:"createTime"`
		AvatarImgId               int64         `json:"avatarImgId"`
		Birthday                  int64         `json:"birthday"` // eg: 851875200000
		Gender                    int64         `json:"gender"`   // 性别 0:未知
		Nickname                  string        `json:"nickname"`
		VipType                   int64         `json:"vipType"` // 0:无vip
		Mutual                    bool          `json:"mutual"`
		RemarkName                interface{}   `json:"remarkName"`
		AccountStatus             int64         `json:"accountStatus"`
		AuthStatus                int64         `json:"authStatus"`
		AvatarUrl                 string        `json:"avatarUrl"`
		BackgroundImgId           int64         `json:"backgroundImgId"`
		BackgroundUrl             string        `json:"backgroundUrl"`
		City                      int64         `json:"city"`
		DetailDescription         string        `json:"detailDescription"`
		DjStatus                  int64         `json:"djStatus"`
		ExpertTags                interface{}   `json:"expertTags"`
		Followed                  bool          `json:"followed"`
		Province                  int64         `json:"province"`
		UserType                  int64         `json:"userType"`
		DefaultAvatar             bool          `json:"defaultAvatar"`
		Experts                   struct{}      `json:"experts"`
		AvatarImgIdStr            string        `json:"avatarImgIdStr"`
		BackgroundImgIdStr        string        `json:"backgroundImgIdStr"`
		Description               string        `json:"description"`
		UserId                    int64         `json:"userId"`
		Signature                 string        `json:"signature"` // 简介
		Y                         int64         `json:"y"`
		Authority                 int64         `json:"authority"`
		Followeds                 int64         `json:"followeds"` // 粉丝数量 和下面的 NewFollows 粉丝数量不值有何区别
		Follows                   int64         `json:"follows"`
		Blacklist                 bool          `json:"blacklist"`
		EventCount                int64         `json:"eventCount"`
		AllSubscribedCount        int64         `json:"allSubscribedCount"`
		PlaylistBeSubscribedCount int64         `json:"playlistBeSubscribedCount"`
		AvatarImgIdStr1           string        `json:"avatarImgId_str"`
		FollowTime                interface{}   `json:"followTime"`
		FollowMe                  bool          `json:"followMe"`
		ArtistIdentity            []interface{} `json:"artistIdentity"`
		CCount                    int64         `json:"cCount"`
		InBlacklist               bool          `json:"inBlacklist"`
		SDJPCount                 int64         `json:"sDJPCount"`
		PlaylistCount             int64         `json:"playlistCount"` // 创建的歌单数量
		SCount                    int64         `json:"sCount"`        // 收藏的歌单数量
		NewFollows                int64         `json:"newFollows"`    // 粉丝数量 和上面的Followeds不知有何区别
	} `json:"profile"`
	PeopleCanSeeMyPlayRecord bool `json:"peopleCanSeeMyPlayRecord"`
	// Bindings 绑定账号信息，比如是否有手机号绑定 see: Api.GetUserBindings()
	Bindings []struct {
		ExpiresIn    int64       `json:"expiresIn"`
		RefreshTime  int64       `json:"refreshTime"`
		BindingTime  int64       `json:"bindingTime"`
		TokenJsonStr interface{} `json:"tokenJsonStr"`
		Url          string      `json:"url"`
		Expired      bool        `json:"expired"`
		UserId       int64       `json:"userId"`
		Id           int64       `json:"id"`
		Type         int64       `json:"type"` // 1:手机号 5:qq 其他暂时未知
	} `json:"bindings"`
	AdValid    bool  `json:"adValid"`
	NewUser    bool  `json:"newUser"`
	RecallUser bool  `json:"recallUser"`
	CreateTime int64 `json:"createTime"`
	CreateDays int64 `json:"createDays"` // 创建账号天数
	// 村民证
	ProfileVillageInfo struct {
		Title     string `json:"title"`
		ImageUrl  string `json:"imageUrl"`
		TargetUrl string `json:"targetUrl"`
	} `json:"profileVillageInfo"`
}

// GetUserInfoDetail 获取用户信息
// har: 29.har
func (a *Api) GetUserInfoDetail(ctx context.Context, req *GetUserInfoDetailReq) (*GetUserInfoDetailResp, error) {
	var (
		url   = fmt.Sprintf("https://interface.music.163.com/weapi/w/v1/user/detail/%v", req.UserId)
		reply GetUserInfoDetailResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type GetUserBindingsReq struct {
	UserId int64 `json:"userId"`
}

type GetUserBindingsResp struct {
	Code     int64 `json:"code"`
	Bindings []struct {
		TokenJsonStr string `json:"tokenJsonStr"`
		ExpiresIn    int64  `json:"expiresIn"`
		BindingTime  int64  `json:"bindingTime"`
		RefreshTime  int64  `json:"refreshTime"`
		Url          string `json:"url"`
		Expired      bool   `json:"expired"`
		UserId       int64  `json:"userId"`
		Id           int64  `json:"id"`
		Type         int64  `json:"type"` // 1:手机号 5:qq 其他暂时未知
	} `json:"bindings"`
}

// GetUserBindings 获取用户绑定账号信息
// har:
func (a *Api) GetUserBindings(ctx context.Context, req *GetUserBindingsReq) (*GetUserBindingsResp, error) {
	var (
		url   = fmt.Sprintf("https://interface.music.163.com/weapi/w/v1/user/bindings/%v", req.UserId)
		reply GetUserBindingsResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

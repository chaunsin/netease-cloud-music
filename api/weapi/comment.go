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

type CommentInfoListReq struct {
	types.ReqCommon
}

type CommentInfoListResp struct {
	types.RespCommon[[]CommentInfoListRespData]
}

type CommentInfoListRespData struct {
	LatestLikedUsers  interface{} `json:"latestLikedUsers"`
	Liked             bool        `json:"liked"`
	Comments          interface{} `json:"comments"`
	ResourceType      int64       `json:"resourceType"`
	ResourceId        int64       `json:"resourceId"` // 应改是对应的歌曲id
	CommentUpgraded   bool        `json:"commentUpgraded"`
	MusicianSaidCount int64       `json:"musicianSaidCount"`
	CommentCountDesc  string      `json:"commentCountDesc"` // 评论数描述基本和commentCount一样
	LikedCount        int64       `json:"likedCount"`
	CommentCount      int64       `json:"commentCount"` // 评论数
	ShareCount        int64       `json:"shareCount"`
	ThreadId          string      `json:"threadId"` // 线程id，用于获取评论列表使用
}

// CommentInfoList 获取歌曲评论梗概信息
// har: 36.har
// needLogin: 未知
func (a *Api) CommentInfoList(ctx context.Context, req *CommentInfoListReq) (*CommentInfoListResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/resource/commentInfo/list"
		reply CommentInfoListResp
		opts  = api.NewOptions()
	)

	// 目前不传值也没发现什么问题
	// if req.Level == types.LevelSky {
	// 	req.ImmerseType = "c51"
	// }

	resp, err := a.client.Request(ctx, url, &req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type CommentsReq struct {
	types.ReqCommon
	ComposeConcert      string `json:"composeConcert"`      // eg: bool
	CommentId           string `json:"commentId"`           // eg: 0
	MarkReplied         bool   `json:"markReplied"`         // eg: bool
	Offset              string `json:"offset"`              // 第几页 eg: "0"
	Limit               string `json:"limit"`               // 每页数量 eg: "60"
	CompareUserLocation bool   `json:"compareUserLocation"` // eg: bool
	ForceFlatComment    bool   `json:"forceFlatComment"`    // eg: bool
	BeforeTime          string `json:"beforeTime"`          // eg: "0"
	ShowInner           bool   `json:"showInner"`           // eg: bool
	ThreadId            string `json:"threadId"`            // eg: R_SO_4_2128846655 see CommentInfoListRespData.ThreadId
}

type CommentsResp struct {
	IsMusician  bool          `json:"isMusician"`
	Cnum        int64         `json:"cnum"`
	UserId      int64         `json:"userId"`
	TopComments []interface{} `json:"topComments"`
	Code        int64         `json:"code"`
	Comments    []struct {
		User struct {
			LocationInfo interface{} `json:"locationInfo"`
			LiveInfo     interface{} `json:"liveInfo"`
			Anonym       int64       `json:"anonym"`
			Highlight    bool        `json:"highlight"`
			AvatarUrl    string      `json:"avatarUrl"`
			AvatarDetail *struct {
				UserType        int64  `json:"userType"`
				IdentityLevel   int64  `json:"identityLevel"`
				IdentityIconUrl string `json:"identityIconUrl"`
			} `json:"avatarDetail"`
			UserType     int64       `json:"userType"`
			Followed     bool        `json:"followed"`
			Mutual       bool        `json:"mutual"`
			RemarkName   interface{} `json:"remarkName"`
			SocialUserId interface{} `json:"socialUserId"`
			VipRights    *struct {
				Associator *struct {
					VipCode int64  `json:"vipCode"`
					Rights  bool   `json:"rights"`
					IconUrl string `json:"iconUrl"`
				} `json:"associator"`
				MusicPackage *struct {
					VipCode int64  `json:"vipCode"`
					Rights  bool   `json:"rights"`
					IconUrl string `json:"iconUrl"`
				} `json:"musicPackage"`
				Redplus *struct {
					VipCode int64  `json:"vipCode"`
					Rights  bool   `json:"rights"`
					IconUrl string `json:"iconUrl"`
				} `json:"redplus"`
				RedVipAnnualCount int64       `json:"redVipAnnualCount"`
				RedVipLevel       int64       `json:"redVipLevel"`
				RelationType      int64       `json:"relationType"`
				MemberLogo        interface{} `json:"memberLogo"`
			} `json:"vipRights"`
			Nickname       string      `json:"nickname"`
			AuthStatus     int64       `json:"authStatus"`
			ExpertTags     interface{} `json:"expertTags"`
			Experts        interface{} `json:"experts"`
			VipType        int64       `json:"vipType"`
			CommonIdentity interface{} `json:"commonIdentity"`
			UserId         int64       `json:"userId"`
			Target         interface{} `json:"target"`
		} `json:"user"`
		BeReplied []struct {
			User struct {
				LocationInfo interface{} `json:"locationInfo"`
				LiveInfo     interface{} `json:"liveInfo"`
				Anonym       int64       `json:"anonym"`
				Highlight    bool        `json:"highlight"`
				AvatarUrl    string      `json:"avatarUrl"`
				AvatarDetail *struct {
					UserType        int64  `json:"userType"`
					IdentityLevel   int64  `json:"identityLevel"`
					IdentityIconUrl string `json:"identityIconUrl"`
				} `json:"avatarDetail"`
				UserType       int64       `json:"userType"`
				Followed       bool        `json:"followed"`
				Mutual         bool        `json:"mutual"`
				RemarkName     interface{} `json:"remarkName"`
				SocialUserId   interface{} `json:"socialUserId"`
				VipRights      interface{} `json:"vipRights"`
				Nickname       string      `json:"nickname"`
				AuthStatus     int64       `json:"authStatus"`
				ExpertTags     interface{} `json:"expertTags"`
				Experts        interface{} `json:"experts"`
				VipType        int64       `json:"vipType"`
				CommonIdentity interface{} `json:"commonIdentity"`
				UserId         int64       `json:"userId"`
				Target         interface{} `json:"target"`
			} `json:"user"`
			BeRepliedCommentId int64       `json:"beRepliedCommentId"`
			Content            *string     `json:"content"`
			RichContent        *string     `json:"richContent"`
			Status             int64       `json:"status"`
			ExpressionUrl      interface{} `json:"expressionUrl"`
			IpLocation         struct {
				Ip       interface{} `json:"ip"`
				Location string      `json:"location"`
				UserId   int64       `json:"userId"`
			} `json:"ipLocation"`
		} `json:"beReplied"`
		PendantData *struct {
			Id       int64  `json:"id"`
			ImageUrl string `json:"imageUrl"`
		} `json:"pendantData"`
		ShowFloorComment    interface{} `json:"showFloorComment"`
		Status              int64       `json:"status"`
		CommentId           int64       `json:"commentId"`
		Content             string      `json:"content"` // 评论内容
		RichContent         *string     `json:"richContent"`
		ContentResource     interface{} `json:"contentResource"`
		Time                int64       `json:"time"`
		TimeStr             string      `json:"timeStr"`
		NeedDisplayTime     bool        `json:"needDisplayTime"`
		LikedCount          int64       `json:"likedCount"`
		ExpressionUrl       interface{} `json:"expressionUrl"`
		CommentLocationType int64       `json:"commentLocationType"`
		ParentCommentId     int64       `json:"parentCommentId"`
		Decoration          struct {
		} `json:"decoration"`
		RepliedMark   interface{} `json:"repliedMark"`
		Grade         interface{} `json:"grade"`
		UserBizLevels interface{} `json:"userBizLevels"`
		IpLocation    struct {
			Ip       interface{} `json:"ip"`
			Location string      `json:"location"`
			UserId   int64       `json:"userId"`
		} `json:"ipLocation"`
		Owner            bool        `json:"owner"`
		Medal            interface{} `json:"medal"`
		LikeAnimationMap struct {
		} `json:"likeAnimationMap"`
		Liked bool `json:"liked"`
	} `json:"comments"`
	Total int64 `json:"total"`
	More  bool  `json:"more"`
}

// Comments 获取歌曲评论列表
// har: 37.har
// needLogin: 未知
func (a *Api) Comments(ctx context.Context, req *CommentsReq) (*CommentsResp, error) {
	var (
		url   = fmt.Sprintf("https://interface.music.163.com/weapi/v1/resource/comments/" + req.ThreadId)
		reply CommentsResp
		opts  = api.NewOptions()
	)

	// 目前不传值也没发现什么问题
	// if req.Level == types.LevelSky {
	// 	req.ImmerseType = "c51"
	// }

	resp, err := a.client.Request(ctx, url, &req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("Request: %w", err)
	}
	_ = resp
	return &reply, nil
}

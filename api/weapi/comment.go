// Copyright (c) 2025-2026 chaunsin
// SPDX-License-Identifier: MIT

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
	LatestLikedUsers  any    `json:"latestLikedUsers"`
	Liked             bool   `json:"liked"`
	Comments          any    `json:"comments"`
	ResourceType      int64  `json:"resourceType"`
	ResourceId        int64  `json:"resourceId"` // 应改是对应的歌曲id
	CommentUpgraded   bool   `json:"commentUpgraded"`
	MusicianSaidCount int64  `json:"musicianSaidCount"`
	CommentCountDesc  string `json:"commentCountDesc"` // 评论数描述基本和commentCount一样
	LikedCount        int64  `json:"likedCount"`
	CommentCount      int64  `json:"commentCount"` // 评论数
	ShareCount        int64  `json:"shareCount"`
	ThreadId          string `json:"threadId"` // 线程id，用于获取评论列表使用
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
		return nil, fmt.Errorf("request: %w", err)
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
	IsMusician  bool  `json:"isMusician"`
	Cnum        int64 `json:"cnum"`
	UserId      int64 `json:"userId"`
	TopComments []any `json:"topComments"`
	Code        int64 `json:"code"`
	Comments    []struct {
		User struct {
			LocationInfo any    `json:"locationInfo"`
			LiveInfo     any    `json:"liveInfo"`
			Anonym       int64  `json:"anonym"`
			Highlight    bool   `json:"highlight"`
			AvatarUrl    string `json:"avatarUrl"`
			AvatarDetail *struct {
				UserType        int64  `json:"userType"`
				IdentityLevel   int64  `json:"identityLevel"`
				IdentityIconUrl string `json:"identityIconUrl"`
			} `json:"avatarDetail"`
			UserType     int64 `json:"userType"`
			Followed     bool  `json:"followed"`
			Mutual       bool  `json:"mutual"`
			RemarkName   any   `json:"remarkName"`
			SocialUserId any   `json:"socialUserId"`
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
				RedVipAnnualCount int64 `json:"redVipAnnualCount"`
				RedVipLevel       int64 `json:"redVipLevel"`
				RelationType      int64 `json:"relationType"`
				MemberLogo        any   `json:"memberLogo"`
			} `json:"vipRights"`
			Nickname       string `json:"nickname"`
			AuthStatus     int64  `json:"authStatus"`
			ExpertTags     any    `json:"expertTags"`
			Experts        any    `json:"experts"`
			VipType        int64  `json:"vipType"`
			CommonIdentity any    `json:"commonIdentity"`
			UserId         int64  `json:"userId"`
			Target         any    `json:"target"`
		} `json:"user"`
		BeReplied []struct {
			User struct {
				LocationInfo any    `json:"locationInfo"`
				LiveInfo     any    `json:"liveInfo"`
				Anonym       int64  `json:"anonym"`
				Highlight    bool   `json:"highlight"`
				AvatarUrl    string `json:"avatarUrl"`
				AvatarDetail *struct {
					UserType        int64  `json:"userType"`
					IdentityLevel   int64  `json:"identityLevel"`
					IdentityIconUrl string `json:"identityIconUrl"`
				} `json:"avatarDetail"`
				UserType       int64  `json:"userType"`
				Followed       bool   `json:"followed"`
				Mutual         bool   `json:"mutual"`
				RemarkName     any    `json:"remarkName"`
				SocialUserId   any    `json:"socialUserId"`
				VipRights      any    `json:"vipRights"`
				Nickname       string `json:"nickname"`
				AuthStatus     int64  `json:"authStatus"`
				ExpertTags     any    `json:"expertTags"`
				Experts        any    `json:"experts"`
				VipType        int64  `json:"vipType"`
				CommonIdentity any    `json:"commonIdentity"`
				UserId         int64  `json:"userId"`
				Target         any    `json:"target"`
			} `json:"user"`
			BeRepliedCommentId int64   `json:"beRepliedCommentId"`
			Content            *string `json:"content"`
			RichContent        *string `json:"richContent"`
			Status             int64   `json:"status"`
			ExpressionUrl      any     `json:"expressionUrl"`
			IpLocation         struct {
				Ip       any    `json:"ip"`
				Location string `json:"location"`
				UserId   int64  `json:"userId"`
			} `json:"ipLocation"`
		} `json:"beReplied"`
		PendantData *struct {
			Id       int64  `json:"id"`
			ImageUrl string `json:"imageUrl"`
		} `json:"pendantData"`
		ShowFloorComment    any      `json:"showFloorComment"`
		Status              int64    `json:"status"`
		CommentId           int64    `json:"commentId"`
		Content             string   `json:"content"` // 评论内容
		RichContent         *string  `json:"richContent"`
		ContentResource     any      `json:"contentResource"`
		Time                int64    `json:"time"`
		TimeStr             string   `json:"timeStr"`
		NeedDisplayTime     bool     `json:"needDisplayTime"`
		LikedCount          int64    `json:"likedCount"`
		ExpressionUrl       any      `json:"expressionUrl"`
		CommentLocationType int64    `json:"commentLocationType"`
		ParentCommentId     int64    `json:"parentCommentId"`
		Decoration          struct{} `json:"decoration"`
		RepliedMark         any      `json:"repliedMark"`
		Grade               any      `json:"grade"`
		UserBizLevels       any      `json:"userBizLevels"`
		IpLocation          struct {
			Ip       any    `json:"ip"`
			Location string `json:"location"`
			UserId   int64  `json:"userId"`
		} `json:"ipLocation"`
		Owner            bool     `json:"owner"`
		Medal            any      `json:"medal"`
		LikeAnimationMap struct{} `json:"likeAnimationMap"`
		Liked            bool     `json:"liked"`
	} `json:"comments"`
	Total int64 `json:"total"`
	More  bool  `json:"more"`
}

// Comments 获取歌曲评论列表
// har: 37.har
// needLogin: 未知
func (a *Api) Comments(ctx context.Context, req *CommentsReq) (*CommentsResp, error) {
	var (
		url   = fmt.Sprintf("https://interface.music.163.com/weapi/v1/resource/comments/%s", req.ThreadId)
		reply CommentsResp
		opts  = api.NewOptions()
	)

	// 目前不传值也没发现什么问题
	// if req.Level == types.LevelSky {
	// 	req.ImmerseType = "c51"
	// }

	resp, err := a.client.Request(ctx, url, &req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

type CommentLikeReq struct {
	types.ReqCommon
	ThreadId  string `json:"threadId"`  // 线程id，eg: R_SO_4_2128846655 see CommentInfoListRespData.ThreadId
	CommentId string `json:"commentId"` // 评论id
}

type CommentLikeResp struct {
	types.RespCommon[any]
}

// CommentLike 点赞歌曲/动态的评论
// needLogin: 是
func (a *Api) CommentLike(ctx context.Context, req *CommentLikeReq) (*CommentLikeResp, error) {
	var (
		url   = "https://music.163.com/weapi/v1/comment/like"
		reply CommentLikeResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, &req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

// CommentUnlike 取消点赞歌曲/动态的评论
// needLogin: 是
func (a *Api) CommentUnlike(ctx context.Context, req *CommentLikeReq) (*CommentLikeResp, error) {
	var (
		url   = "https://music.163.com/weapi/v1/comment/unlike"
		reply CommentLikeResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, &req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	_ = resp
	return &reply, nil
}

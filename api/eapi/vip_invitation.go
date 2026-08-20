// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

// VipMemberGiftTokenCreateReq creates an invitation token.
type VipMemberGiftTokenCreateReq struct {
	types.EApiReqCommon
}

// VipMemberGiftTokenCreateResp is the invitation token response.
type VipMemberGiftTokenCreateResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
	} `json:"data"`
}

// VipMemberGiftTokenCreate creates a VIP member gift invitation token.
func (a *Api) VipMemberGiftTokenCreate(ctx context.Context, req *VipMemberGiftTokenCreateReq) (*VipMemberGiftTokenCreateResp, error) {
	if req == nil {
		req = &VipMemberGiftTokenCreateReq{}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/vipactivity/app/vip/invitation/token/create"
		reply    VipMemberGiftTokenCreateResp
		opts     = api.NewOptions().SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request VIP member gift token creation: %w", err)
	}
	return &reply, nil
}

// VipMemberGiftPageInfoReq requests the current member gift page state.
type VipMemberGiftPageInfoReq struct {
	types.EApiReqCommon
}

// VipMemberGiftPageInfoResp describes whether and how the current user can send VIP days.
type VipMemberGiftPageInfoResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		CanSendVip            int    `json:"canSendVip"`
		SendVipDayTotal       int    `json:"sendVipDayTotal"`
		SendVipDayLeft        int    `json:"sendVipDayLeft"`
		ButtonText            string `json:"buttonText"`
		Desc                  string `json:"desc"`
		VipLevelDesc          string `json:"vipLevelDesc"`
		Img                   string `json:"img"`
		VipLevel              int    `json:"vipLevel"`
		HasSendVipInThisMonth bool   `json:"hasSendVipInThisMonth"`
	} `json:"data"`
}

// VipMemberGiftPageInfo gets the current member gift page state.
func (a *Api) VipMemberGiftPageInfo(ctx context.Context, req *VipMemberGiftPageInfoReq) (*VipMemberGiftPageInfoResp, error) {
	if req == nil {
		req = &VipMemberGiftPageInfoReq{}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/vipactivity/app/vip/invitation/page/info"
		reply    VipMemberGiftPageInfoResp
		opts     = api.NewOptions().SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request VIP member gift page info: %w", err)
	}
	return &reply, nil
}

// VipMemberGiftDetailReq identifies a member gift by token or record ID.
type VipMemberGiftDetailReq struct {
	types.EApiReqCommon

	Token    string `json:"token,omitempty"`
	RecordId int64  `json:"recordId,omitempty"`
}

// VipMemberGiftUser is a user summary in a member gift response.
type VipMemberGiftUser struct {
	UserId    int64  `json:"userId"`
	Nickname  string `json:"nickname"`
	AvatarUrl string `json:"avatarUrl"`
}

// VipMemberGiftDetailResp describes a member gift invitation and its participants.
type VipMemberGiftDetailResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RecordId             int64              `json:"recordId"`
		Inviter              *VipMemberGiftUser `json:"inviter"`
		Invitee              *VipMemberGiftUser `json:"invitee"`
		TokenExpireTime      int64              `json:"tokenExpireTime"`
		AcceptedRewardImgUrl string             `json:"accpetedRewardImgUrl"`
		DetailRewardImgUrl   string             `json:"detailRewardImgUrl"`
		RewardName           string             `json:"rewardName"`
		ShareSwitch          bool               `json:"shareSwitch"`
		CurrentUserRecordId  *int64             `json:"currentUserRecordId"`
		InviterTotalDays     *int64             `json:"inviterTotalDays"`
		VipType              int                `json:"vipType"`
		Duration             int                `json:"duration"`
		AcceptTime           int64              `json:"acceptTime"`
	} `json:"data"`
}

// VipMemberGiftDetail gets the details of a member gift invitation.
func (a *Api) VipMemberGiftDetail(ctx context.Context, req *VipMemberGiftDetailReq) (*VipMemberGiftDetailResp, error) {
	if req == nil {
		req = &VipMemberGiftDetailReq{}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/vipactivity/app/vip/invitation/detail/info/get"
		reply    VipMemberGiftDetailResp
		opts     = api.NewOptions().SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request VIP member gift detail: %w", err)
	}
	return &reply, nil
}

// VipMemberGiftAcceptReq accepts a member gift invitation.
type VipMemberGiftAcceptReq struct {
	types.EApiReqCommon

	Token      string `json:"token"`
	Refer      string `json:"refer,omitempty"`
	CheckToken string `json:"checkToken,omitempty"`
}

// VipMemberGiftAcceptResp is the accepted member gift response.
type VipMemberGiftAcceptResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RecordId             int64              `json:"recordId"`
		AcceptedRewardImgUrl string             `json:"accpetedRewardImgUrl"`
		ActivityRewardImgUrl string             `json:"activityRewardImgUrl"`
		VipType              int                `json:"vipType"`
		RewardName           string             `json:"rewardName"`
		DetailRewardImgUrl   string             `json:"detailRewardImgUrl"`
		CanReceiveCoupon     bool               `json:"canReceiveCoupon"`
		Inviter              *VipMemberGiftUser `json:"inviter"`
		Duration             int                `json:"duration"`
		CouponImageUrl       string             `json:"couponImageUrl"`
		VipLevel             int                `json:"vipLevel"`
	} `json:"data"`
}

// VipMemberGiftAccept accepts a VIP member gift invitation.
func (a *Api) VipMemberGiftAccept(ctx context.Context, req *VipMemberGiftAcceptReq) (*VipMemberGiftAcceptResp, error) {
	if req == nil {
		return nil, errors.New("VIP member gift accept request is nil")
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/vipactivity/app/vip/invitation/accept"
		reply    VipMemberGiftAcceptResp
		opts     = api.NewOptions().SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request VIP member gift acceptance: %w", err)
	}
	return &reply, nil
}

// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

const dailySongShareMConfigInfo = `{"IuRPVVmc3WWul9fT":{"version":"115240960","appver":"9.5.37"},"tPJJnts2H31BZXmp":{"version":"5230592","appver":"4.74.0"},"c0Ve6C0uNl2Am0Rl":{"version":"276480","appver":"1.4.30"},"zr4bw6pKFDIZScpo":{"version":"3758080","appver":"2.40.0"}}`

// DailySongShareRegisterReq registers the current user for the sharing activity.
type DailySongShareRegisterReq struct {
	types.EApiReqCommon
}

// DailySongShareRegisterResp is the sharing activity registration response.
type DailySongShareRegisterResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		NoteAttendance bool `json:"noteAttendance"`
	} `json:"data"`
}

// DailySongShareRegister registers the current user for the daily sharing activity.
func (a *Api) DailySongShareRegister(ctx context.Context, req *DailySongShareRegisterReq) (*DailySongShareRegisterResp, error) {
	if req == nil {
		req = &DailySongShareRegisterReq{}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/note/common/activity/in/registration"
		reply    DailySongShareRegisterResp
		opts     = api.NewOptions().SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share registration: %w", err)
	}
	return &reply, nil
}

// DailySongShareAttendanceRegisterReq registers an activity attendance cycle.
type DailySongShareAttendanceRegisterReq struct {
	types.EApiReqCommon

	ActivityId      int64 `json:"activityId"`
	ActivityCycleId int64 `json:"activityCycleId"`
	AutoRegister    bool  `json:"autoRegister"`
}

// DailySongShareAttendanceRegisterResp is the attendance registration response.
type DailySongShareAttendanceRegisterResp struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

// DailySongShareAttendanceRegister enrolls the current activity cycle.
func (a *Api) DailySongShareAttendanceRegister(ctx context.Context, req *DailySongShareAttendanceRegisterReq) (*DailySongShareAttendanceRegisterResp, error) {
	if req == nil {
		return nil, errors.New("daily song share attendance register request is nil")
	}

	req.AutoRegister = true

	var (
		endpoint = "https://interface3.music.163.com/xeapi/note/attendance/activity/register"
		reply    DailySongShareAttendanceRegisterResp
		opts     = api.NewOptions().SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share attendance registration: %w", err)
	}
	return &reply, nil
}

// DailySongShareRegistrationGuideReq requests the current activity guide.
type DailySongShareRegistrationGuideReq struct {
	types.EApiReqCommon
}

// DailySongShareRegistrationGuideResp describes the current activity and progress.
type DailySongShareRegistrationGuideResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		RegisterStatus     string `json:"registerStatus"`     // NOREGISTER:没有参与活动 REGISTER:参与
		ActivityId         int64  `json:"activityId"`         // eg: 1
		ActivityCycleId    int64  `json:"activityCycleId"`    // eg: 501572
		ActivityInterestId int64  `json:"activityInterestId"` // eg: 11066304
		RewardJumpUrl      string `json:"rewardJumpUrl"`      // eg: https://st.music.163.com/g/platform/lottery?activityIds=11066304\u0026newVersion=1
		Duration           string `json:"duration"`           // eg: 第0817-0823期
		RegisteredGuide    struct {
			Title           string `json:"title"`           // eg: 每日推歌挑战赛
			SignUp          string `json:"signUp"`          // eg: 暂无抽奖机会
			SignTip         string `json:"signTip"`         // eg: 剩余0次机会
			RewardCount     int    `json:"rewardCount"`     // 今日奖励次数？
			HaveRewardCount int    `json:"haveRewardCount"` // 可以抽奖次数
			AlreadyPubEvent bool   `json:"alreadyPubEvent"` // 貌似是今日是否有发布动态，待确认？
			PubEventCount   int    `json:"pubEventCount"`   // 已发布的动态次数
		} `json:"noteAttendanceRegisteredGuideVo"`
	} `json:"data"`
}

// DailySongShareRegistrationGuide gets the current activity guide and identifiers.
func (a *Api) DailySongShareRegistrationGuide(ctx context.Context, req *DailySongShareRegistrationGuideReq) (*DailySongShareRegistrationGuideResp, error) {
	if req == nil {
		req = &DailySongShareRegistrationGuideReq{}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/note/attendance/activity/registration/v2/guide"
		reply    DailySongShareRegistrationGuideResp
		opts     = api.NewOptions().SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share registration guide: %w", err)
	}
	return &reply, nil
}

// DailySongSharePublishReq publishes a note used by the sharing activity.
type DailySongSharePublishReq struct {
	types.EApiReqCommon

	AddComment         bool   `json:"addComment"`
	AutoSaveDraft      bool   `json:"autoSaveDraft"`
	Id                 string `json:"id,omitempty"`
	ThreadId           string `json:"threadId,omitempty"`
	ResourceId         string `json:"resourceId,omitempty"`
	Msg                string `json:"msg"`
	SessionId          string `json:"sessionId"` // 格式貌似为 a1b2c3d4-e5f
	TargetPublishTime  any    `json:"targetPublishTime"`
	ServerUuid         string `json:"serverUuid"`
	UseNewUpload       bool   `json:"useNewUpload"`
	FromRn             bool   `json:"fromRN"`
	ActivityInfoList   string `json:"activityInfoList,omitempty"`
	PubSource          string `json:"pubSource"`
	PubTraceId         string `json:"pubTraceId"`
	PublishTime        any    `json:"publishTime"`
	Title              string `json:"title,omitempty"`
	SocialSpaceVisible int    `json:"socialSpaceVisible"`
	PrivacySetting     string `json:"privacySetting"`
	Uid                string `json:"uid"`
	ContainAiContent   bool   `json:"containAiContent"`
	Uuid               string `json:"uuid"`
	Type               string `json:"type"`
	Pics               string `json:"pics,omitempty"`
	NeedsGuardianToken bool   `json:"needsGuardianToken"`
	ContentDeclaration string `json:"contentDeclaration,omitempty"`
	ProduceInfo        string `json:"produceInfo,omitempty"`
	RepostSource       string `json:"repostSource,omitempty"`
}

// DailySongSharePublish publishes a note or song share for the activity.
func (a *Api) DailySongSharePublish(ctx context.Context, req *DailySongSharePublishReq) (*EventPublishResp, error) {
	if req == nil {
		return nil, errors.New("daily song share publish request is nil")
	}

	req.AutoSaveDraft = true
	req.UseNewUpload = true
	req.FromRn = true
	req.NeedsGuardianToken = true

	if req.Uuid == "" {
		req.Uuid = uuid.NewString()
	}

	if req.OS == "" {
		req.OS = "android"
	}

	if req.TargetPublishTime == nil {
		req.TargetPublishTime = "-1"
	}

	if req.PublishTime == nil {
		req.PublishTime = "0"
	}

	if req.PrivacySetting == "" {
		req.PrivacySetting = "0"
	}

	if req.SocialSpaceVisible == 0 {
		req.SocialSpaceVisible = 1
	}

	if req.Type == "" {
		req.Type = "noresource"
	}

	if req.Type == "song" && req.Id != "" {
		if req.ThreadId == "" {
			req.ThreadId = "R_SO_4_" + req.Id
		}

		if req.ResourceId == "" {
			req.ResourceId = req.Id
		}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/note/share/friends/resource"
		reply    EventPublishResp
		opts     = api.NewOptions().
				SetXEAPI().
				SetHeader("CMPageId", "page_songlist").
				SetHeader("MConfig-Info", dailySongShareMConfigInfo)
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share publish: %w", err)
	}
	return &reply, nil
}

// DailySongShareTriggerReq reports a song-sharing trigger.
type DailySongShareTriggerReq struct {
	types.EApiReqCommon

	SongId  string `json:"songId"`
	Channel string `json:"channel"`
}

// DailySongShareTriggerResp is the song-sharing trigger response.
type DailySongShareTriggerResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    bool   `json:"data"`
}

// DailySongShareTrigger reports that a song was shared through a channel.
func (a *Api) DailySongShareTrigger(ctx context.Context, req *DailySongShareTriggerReq) (*DailySongShareTriggerResp, error) {
	if req == nil {
		return nil, errors.New("daily song share trigger request is nil")
	}

	if strings.TrimSpace(req.Channel) == "" {
		req.Channel = "cloudmusic"
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/music/song/share/trigger"
		reply    DailySongShareTriggerResp
		opts     = api.NewOptions().SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share trigger: %w", err)
	}
	return &reply, nil
}

// DailySongShareLotteryReq draws a prize from the sharing activity.
type DailySongShareLotteryReq struct {
	types.EApiReqCommon

	ActivityId int64 `json:"activityId"` // 对应 DailySongShareRegistrationGuideResp.Data.ActivityInterestId
}

// DailySongShareLotteryPrizeDetail describes one possible lottery prize.
type DailySongShareLotteryPrizeDetail struct {
	PrizeName    string   `json:"prizeName"`
	WinPrizeDesc string   `json:"winPrizeDesc"`
	PrizeImgList []string `json:"prizeImgList"`
	ExchangeUrl  string   `json:"exchangeUrl"`
	PrizeType    int      `json:"prizeType"`
	SubType      int      `json:"subType"`
	ContentId    string   `json:"contentId"`
	DefaultPrize int      `json:"defaultPrize"`
	PrizeLevel   int      `json:"prizeLevel"`
}

// DailySongShareLotteryResp is the sharing activity lottery response.
type DailySongShareLotteryResp struct {
	Code    int    `json:"code"` // 456:很遗憾，您本次权益不足，谢谢您的参与
	Message string `json:"message"`
	Data    struct {
		UserId             int64                                       `json:"userId"`
		IdempotentId       string                                      `json:"idempotentId"`
		ActivityId         int64                                       `json:"activityId"`
		PrizeSchemeId      int64                                       `json:"prizeSchemeId"`
		DrawPrizeTime      int64                                       `json:"drawPrizeTime"`
		PrizeDetailInfoMap map[string]DailySongShareLotteryPrizeDetail `json:"prizeDetailInfoMap"`
		NoLotteryContent   any                                         `json:"noLotteryContent"`
		RestChance         int                                         `json:"restChance"`
	} `json:"data"`
}

// DailySongShareLottery draws a prize from the current sharing activity.
func (a *Api) DailySongShareLottery(ctx context.Context, req *DailySongShareLotteryReq) (*DailySongShareLotteryResp, error) {
	if req == nil {
		req = &DailySongShareLotteryReq{}
	}

	var (
		endpoint = "https://interface3.music.163.com/xeapi/middle/play/do/lottery"
		reply    DailySongShareLotteryResp
		opts     = api.NewOptions().SetXEAPI()
	)

	if _, err := a.client.Request(ctx, endpoint, req, &reply, opts); err != nil {
		return nil, fmt.Errorf("request daily song share lottery: %w", err)
	}
	return &reply, nil
}

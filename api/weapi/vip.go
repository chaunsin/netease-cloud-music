// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type VipRewardGetReq struct {
	TaskIds []string
}

type vipRewardGetReq struct {
	TaskIds string `json:"taskIds"`
}

type VipRewardGetResp struct {
	types.RespCommon[any]
}

// VipRewardGet 领取vip成长值
// url:
// needLogin: 未知.
func (a *Api) VipRewardGet(ctx context.Context, req *VipRewardGetReq) (*VipRewardGetResp, error) {
	if len(req.TaskIds) == 0 {
		return nil, errors.New("taskIds is empty")
	}

	var (
		opts    = api.NewOptions()
		url     = "https://music.163.com/weapi/vipnewcenter/app/level/task/reward/get"
		reply   VipRewardGetResp
		request = vipRewardGetReq{
			TaskIds: strings.Join(req.TaskIds, ","),
		}
	)

	resp, err := a.client.Request(ctx, url, &request, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipRewardGetAllReq struct{}

type VipRewardGetAllResp struct {
	types.RespCommon[VipRewardGetAllRespData]
}

type VipRewardGetAllRespData struct {
	Result bool `json:"result"`
}

// VipRewardGetAll 领取vip所有成长值
// url:
// needLogin: 未知.
func (a *Api) VipRewardGetAll(ctx context.Context, req *VipRewardGetAllReq) (*VipRewardGetAllResp, error) {
	var (
		url   = "https://music.163.com/weapi/vipnewcenter/app/level/task/reward/getall"
		reply VipRewardGetAllResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipTaskReq struct{}

type VipTaskResp struct {
	types.RespCommon[VipTaskRespData]
}

type VipTaskRespData struct {
	TaskList []struct {
		Seq       int64  `json:"seq"`
		SeqName   string `json:"seqName"`
		TaskItems []struct {
			Action          string   `json:"action"`
			ActionType      int64    `json:"actionType"`
			BasicTaskId     int64    `json:"basicTaskId"`
			BusinessIdent   any      `json:"businessIdent"`
			CurrentProgress int64    `json:"currentProgress"`
			Description     string   `json:"description"`
			GrowthPoint     int64    `json:"growthPoint"`
			IconUrl         string   `json:"iconUrl"`
			MissionId       int64    `json:"missionId"`
			Name            string   `json:"name"`
			NeedReceive     bool     `json:"needReceive"`
			Period          int64    `json:"period"`
			ProgressType    int64    `json:"progressType"`
			RuleId          int64    `json:"ruleId"`
			SeqTag          int64    `json:"seqTag"`
			ShowProgress    bool     `json:"showProgress"`
			SortValue       int64    `json:"sortValue"`
			Status          int64    `json:"status"`
			TagId           int64    `json:"tagId"`
			TargetWorth     int64    `json:"targetWorth"`
			Targets         any      `json:"targets"`
			TaskId          string   `json:"taskId"`
			TaskTag         string   `json:"taskTag"`
			TotalUngetScore int64    `json:"totalUngetScore"`
			Type            int64    `json:"type"`
			TypeCode        any      `json:"typeCode"`
			UnGetIds        []string `json:"unGetIds"`
			UpdateTime      int64    `json:"updateTime"`
			Url             string   `json:"url"`
			UserMissionId   any      `json:"userMissionId"`
		} `json:"taskItems"`
		TaskType int64 `json:"taskType"`
	} `json:"taskList"`
	TaskScore int64 `json:"taskScore"`
}

// VipTask vip任务列表 todo:该任务列表应该是旧接口貌似
// url:
// needLogin: 未知.
func (a *Api) VipTask(ctx context.Context, req *VipTaskReq) (*VipTaskResp, error) {
	var (
		url   = "https://music.163.com/weapi/vipnewcenter/app/level/task/list"
		reply VipTaskResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipTaskV2Req struct{}

type VipTaskV2Resp struct {
	types.RespCommon[VipTaskV2RespData]
}

type VipTaskV2RespData struct {
	MaxScore      int64 `json:"maxScore"`
	ScoreDuration int64 `json:"scoreDuration"`
	TaskScore     int64 `json:"taskScore"`
	UnGetAllScore int64 `json:"unGetAllScore"`
	TaskList      []struct {
		Seq       int64  `json:"seq"`
		SeqName   string `json:"seqName"`
		TaskItems []struct {
			CurrentInfo struct {
				Action          string `json:"action"`
				ActionType      int64  `json:"actionType"`
				BasicTaskId     int64  `json:"basicTaskId"`
				BusinessIdent   any    `json:"businessIdent"`
				CurrentProgress int64  `json:"currentProgress"`
				Description     string `json:"description"`
				GrowthPoint     int64  `json:"growthPoint"`
				IconUrl         string `json:"iconUrl"`
				MissionId       int64  `json:"missionId"`
				Name            string `json:"name"`
				NeedReceive     bool   `json:"needReceive"`
				Period          int64  `json:"period"`
				ProgressType    int64  `json:"progressType"`
				RuleId          int64  `json:"ruleId"`
				SeqTag          int64  `json:"seqTag"`
				ShowProgress    bool   `json:"showProgress"`
				SortValue       int64  `json:"sortValue"`
				Status          int64  `json:"status"`
				TagId           int64  `json:"tagId"`
				TargetWorth     int64  `json:"targetWorth"`
				TaskId          string `json:"taskId"`
				TaskTag         string `json:"taskTag"`
				TotalUngetScore int64  `json:"totalUngetScore"`
				Type            int64  `json:"type"`
				TypeCode        any    `json:"typeCode"`
				UnGetIds        any    `json:"unGetIds"`
				UpdateTime      int64  `json:"updateTime"`
				Url             string `json:"url"`
				UserMissionId   *int64 `json:"userMissionId"`
			} `json:"currentInfo"`
			SubList any `json:"subList"`
		} `json:"taskItems"`
		TaskType int64 `json:"taskType"`
	} `json:"taskList"`
}

// VipTaskV2 vip任务列表V2 貌似也是旧接口
// url:
// needLogin: 未知.
func (a *Api) VipTaskV2(ctx context.Context, req *VipTaskV2Req) (*VipTaskV2Resp, error) {
	var (
		url   = "https://music.163.com/weapi/vipnewcenter/app/level/task/newlist"
		reply VipTaskV2Resp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipInfoReq struct {
	UserId string `json:"userId"` // 为空默认为当前登录用户
}

type VipInfoResp struct {
	types.RespCommon[VipInfoRespData]
}

type VipInfoRespData struct {
	Associator struct {
		DynamicIconUrl  string `json:"dynamicIconUrl"`
		ExpireTime      int64  `json:"expireTime"`
		IconUrl         string `json:"iconUrl"`
		IsSign          bool   `json:"isSign"`
		IsSignDeduct    bool   `json:"isSignDeduct"`
		IsSignIap       bool   `json:"isSignIap"`
		IsSignIapDeduct bool   `json:"isSignIapDeduct"`
		VipCode         int64  `json:"vipCode"`
		VipLevel        int64  `json:"vipLevel"`
	} `json:"associator"`
	MusicPackage struct {
		DynamicIconUrl  string `json:"dynamicIconUrl"`
		ExpireTime      int64  `json:"expireTime"`
		IconUrl         string `json:"iconUrl"`
		IsSign          bool   `json:"isSign"`
		IsSignDeduct    bool   `json:"isSignDeduct"`
		IsSignIap       bool   `json:"isSignIap"`
		IsSignIapDeduct bool   `json:"isSignIapDeduct"`
		VipCode         int64  `json:"vipCode"`
		VipLevel        int64  `json:"vipLevel"`
	} `json:"musicPackage"`
	RedVipAnnualCount     int64  `json:"redVipAnnualCount"`
	RedVipDynamicIconUrl  any    `json:"redVipDynamicIconUrl"`
	RedVipDynamicIconUrl2 any    `json:"redVipDynamicIconUrl2"`
	RedVipLevel           int64  `json:"redVipLevel"`
	RedVipLevelIcon       string `json:"redVipLevelIcon"`
	Redplus               struct {
		DynamicIconUrl  string `json:"dynamicIconUrl"`
		ExpireTime      int64  `json:"expireTime"`
		IconUrl         string `json:"iconUrl"`
		IsSign          bool   `json:"isSign"`
		IsSignDeduct    bool   `json:"isSignDeduct"`
		IsSignIap       bool   `json:"isSignIap"`
		IsSignIapDeduct bool   `json:"isSignIapDeduct"`
		VipCode         int64  `json:"vipCode"`
		VipLevel        int64  `json:"vipLevel"`
	} `json:"redplus"`
}

// VipInfo vip信息
// har:
// needLogin: 未知.
func (a *Api) VipInfo(ctx context.Context, req *VipInfoReq) (*VipInfoResp, error) {
	var (
		url   = "https://music.163.com/weapi/music-vip-membership/front/vip/info"
		reply VipInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipClientInfoReq struct {
	UserId string `json:"userId"` // 为空默认为当前登录用户
}

type VipClientInfoResp struct {
	types.RespCommon[VipClientInfoRespData]
}

type VipClientInfoRespData struct {
	AlbumVip struct {
		DynamicIconUrl  any   `json:"dynamicIconUrl"`
		ExpireTime      int64 `json:"expireTime"`
		IconUrl         any   `json:"iconUrl"`
		IsSign          bool  `json:"isSign"`
		IsSignDeduct    bool  `json:"isSignDeduct"`
		IsSignIap       bool  `json:"isSignIap"`
		IsSignIapDeduct bool  `json:"isSignIapDeduct"`
		VipCode         int64 `json:"vipCode"`
		VipLevel        int64 `json:"vipLevel"`
	} `json:"albumVip"`
	Associator struct {
		DynamicIconUrl  any   `json:"dynamicIconUrl"`
		ExpireTime      int64 `json:"expireTime"`
		IconUrl         any   `json:"iconUrl"`
		IsSign          bool  `json:"isSign"`
		IsSignDeduct    bool  `json:"isSignDeduct"`
		IsSignIap       bool  `json:"isSignIap"`
		IsSignIapDeduct bool  `json:"isSignIapDeduct"`
		VipCode         int64 `json:"vipCode"`
		VipLevel        int64 `json:"vipLevel"`
	} `json:"associator"`
	MusicPackage struct {
		DynamicIconUrl  any   `json:"dynamicIconUrl"`
		ExpireTime      int64 `json:"expireTime"`
		IconUrl         any   `json:"iconUrl"`
		IsSign          bool  `json:"isSign"`
		IsSignDeduct    bool  `json:"isSignDeduct"`
		IsSignIap       bool  `json:"isSignIap"`
		IsSignIapDeduct bool  `json:"isSignIapDeduct"`
		VipCode         int64 `json:"vipCode"`
		VipLevel        int64 `json:"vipLevel"`
	} `json:"musicPackage"`
	Now               int64 `json:"now"` // eg: 1746370409099
	OldCacheProtocol  bool  `json:"oldCacheProtocol"`
	RedVipAnnualCount int64 `json:"redVipAnnualCount"`
	RedVipLevel       int64 `json:"redVipLevel"`
	Redplus           struct {
		DynamicIconUrl  any   `json:"dynamicIconUrl"`
		ExpireTime      int64 `json:"expireTime"`
		IconUrl         any   `json:"iconUrl"`
		IsSign          bool  `json:"isSign"`
		IsSignDeduct    bool  `json:"isSignDeduct"`
		IsSignIap       bool  `json:"isSignIap"`
		IsSignIapDeduct bool  `json:"isSignIapDeduct"`
		VipCode         int64 `json:"vipCode"`
		VipLevel        int64 `json:"vipLevel"`
	} `json:"redplus"`
	RelationOtherUserId               int64 `json:"relationOtherUserId"`
	RelationOtherUserRedVipExpireTime int64 `json:"relationOtherUserRedVipExpireTime"`
	RelationType                      int64 `json:"relationType"`
	UserId                            int64 `json:"userId"`
	VoiceBookVip                      struct {
		DynamicIconUrl  any   `json:"dynamicIconUrl"`
		ExpireTime      int64 `json:"expireTime"`
		IconUrl         any   `json:"iconUrl"`
		IsSign          bool  `json:"isSign"`
		IsSignDeduct    bool  `json:"isSignDeduct"`
		IsSignIap       bool  `json:"isSignIap"`
		IsSignIapDeduct bool  `json:"isSignIapDeduct"`
		VipCode         int64 `json:"vipCode"`
		VipLevel        int64 `json:"vipLevel"`
	} `json:"voiceBookVip"`
}

// VipClientInfo vip信息
// har:
// needLogin: 未知.
func (a *Api) VipClientInfo(ctx context.Context, req *VipClientInfoReq) (*VipClientInfoResp, error) {
	var (
		url   = "https://music.163.com/api/music-vip-membership/client/vip/info"
		reply VipClientInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipGrowPointReq struct {
	types.ReqCommon
}

type VipGrowPointResp struct {
	types.RespCommon[VipGrowPointRespData]
}

type VipGrowPointRespData struct {
	UserLevel struct {
		UserId          int64  `json:"userId"`
		Level           int64  `json:"level"`
		GrowthPoint     int64  `json:"growthPoint"` // 当前成长值
		LevelName       string `json:"levelName"`   // 黑胶·肆
		YesterdayPoint  int64  `json:"yesterdayPoint"`
		VipType         int64  `json:"vipType"`    // -2:过期？
		ExtJson         string `json:"extJson"`    // eg: "{\"yearMonth\":\"20255\",\"lastDay\":\"202554\",\"lastDayScore\":-6,\"todayScore\":6,\"currentDay\":\"202555\",\"totalScore\":0,\"monthTaskTotalScore\":6}"
		ExpireTime      int64  `json:"expireTime"` // 1746892799000,
		AvatarUrl       any    `json:"avatarUrl"`
		LatestVipType   int64  `json:"latestVipType"`   // 100:领取的赠送？
		LatestVipStatus int64  `json:"latestVipStatus"` // 0:失效或关闭 1: 貌似正常
		Normal          bool   `json:"normal"`
		MaxLevel        bool   `json:"maxLevel"` // true: 最高等级
	} `json:"userLevel"`
	LevelCard struct {
		RightId                           int64  `json:"rightId"`
		Level                             int64  `json:"level"`             // vip等级
		PrivilegeName                     string `json:"privilegeName"`     // V4等级标识
		PrivilegeSubTitle                 string `json:"privilegeSubTitle"` // V4尊享
		PrivilegeIconUrl                  string `json:"privilegeIconUrl"`
		PrivilegePlusIconUrl              any    `json:"privilegePlusIconUrl"`
		ResourceId                        int64  `json:"resourceId"`
		ObtainLimitType                   int64  `json:"obtainLimitType"`
		LevelBackgroundCardImageUrl       string `json:"levelBackgroundCardImageUrl"`
		LevelBackgroundCardExpireImageUrl string `json:"levelBackgroundCardExpireImageUrl"`
		LevelName                         string `json:"levelName"` // eg: 黑胶·肆
		LevelMarkImageUrl                 string `json:"levelMarkImageUrl"`
		LevelMarkExpireImageUrl           string `json:"levelMarkExpireImageUrl"`
		BackgroundImageUrl                string `json:"backgroundImageUrl"`
		UpgradeFireworksImageUrl          string `json:"upgradeFireworksImageUrl"`
		NewUpgradeFireworksImageUrl       string `json:"newUpgradeFireworksImageUrl"`
		BlurryBackgroundImageUrl          string `json:"blurryBackgroundImageUrl"`
		RedVipImageUrl                    string `json:"redVipImageUrl"`
		RedVipExpireImageUrl              string `json:"redVipExpireImageUrl"`
		RedVipWholeImageUrl               string `json:"redVipWholeImageUrl"`
		RedVipExpireWholeImageUrl         string `json:"redVipExpireWholeImageUrl"`
		RedVipBuckleImageUrl              string `json:"redVipBuckleImageUrl"`
		RedVipExpireBuckleImageUrl        string `json:"redVipExpireBuckleImageUrl"`
		VipGiftRightBarImageUrl           string `json:"vipGiftRightBarImageUrl"`
		VipGiftExpireRightBarImageUrl     any    `json:"vipGiftExpireRightBarImageUrl"`
		VipLevelPageCardImgUrl            string `json:"vipLevelPageCardImgUrl"`
		VipLevelPageExpireCardImgUrl      string `json:"vipLevelPageExpireCardImgUrl"`
		AccountPageIconImgUrl             string `json:"accountPageIconImgUrl"`
		FlashIconImgUrl                   string `json:"flashIconImgUrl"`
	} `json:"levelCard"`
}

// VipGrowPoint 获取当前账号 VIP 成长值信息
// har: 46.har
// needLogin: 未知.
func (a *Api) VipGrowPoint(ctx context.Context, req *VipGrowPointReq) (*VipGrowPointResp, error) {
	var (
		url   = "https://music.163.com/weapi/vipnewcenter/app/level/growhpoint/basic"
		reply VipGrowPointResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipProgressListReq struct {
	types.ReqCommon
}

type VipProgressListResp struct {
	types.RespCommon[[]VipProgressListRespData]
}

type VipProgressListRespData struct {
	UserId                int64  `json:"userId"`
	UserType              string `json:"userType"`
	ProgressRate          int64  `json:"progressRate"`
	TotalCompleteNum      int64  `json:"totalCompleteNum"`
	MissionStatus         int64  `json:"missionStatus"`
	UserMissionProgressId any    `json:"userMissionProgressId"`
	LatestRecordTime      any    `json:"latestRecordTime"`
	StartTime             string `json:"startTime"`
	EndTime               string `json:"endTime"`
	UpdateTime            any    `json:"updateTime"`
	BasicMissionDTO       struct {
		MissionId              int64  `json:"missionId"`
		MissionCode            string `json:"missionCode"`
		Name                   string `json:"name"`
		MissionType            int64  `json:"missionType"`
		Alue                   int64  `json:"alue"`
		NeedToReceive          bool   `json:"needToReceive"`
		MissionEntityId        int64  `json:"missionEntityId"`
		StartTime              string `json:"startTime"`
		CurrentPeriodStartTime string `json:"currentPeriodStartTime"`
		EndTime                string `json:"endTime"`
		CurrentPeriodEndTime   string `json:"currentPeriodEndTime"`
		Tag                    int64  `json:"tag"`
		SchemaContent          string `json:"schemaContent"` // use VipProgressListRespDataSchemaContent
		MissionTimeSettingDto  any    `json:"missionTimeSettingDto"`
	} `json:"basicMissionDTO"`
	StageProgressDTOS []struct {
		CompleteNum       int64  `json:"completeNum"`
		CompleteNumPerDay any    `json:"completeNumPerDay"`
		ProgressRate      int64  `json:"progressRate"`
		StageStatus       int64  `json:"stageStatus"`
		IsCurrentStage    bool   `json:"isCurrentStage"`
		RewardType        int64  `json:"rewardType"`
		RewardId          int64  `json:"rewardId"`
		RewardName        string `json:"rewardName"`
		ProvideMethod     int64  `json:"provideMethod"`
		Worth             int64  `json:"worth"`
		RewardCount       int64  `json:"rewardCount"`
		UserRewardId      any    `json:"userRewardId"`
		UserProgressId    any    `json:"userProgressId"`
		RewardExpireTime  any    `json:"rewardExpireTime"`
		RewardExtendInfo  string `json:"rewardExtendInfo"`
		StageIx           int64  `json:"stageIx"`
		ComposeInterestId any    `json:"composeInterestId"`
		RewardInfoDTOS    any    `json:"rewardInfoDTOS"`
		StageDescription  string `json:"stageDescription"`
	} `json:"stageProgressDTOS"`
	HistoryUnObtainRewardWorth int64 `json:"historyUnObtainRewardWorth"`
	Children                   []any `json:"children"`
}

type VipProgressListRespDataSchemaContent struct {
	TaskTitle string `json:"taskTitle"`
	Icon      []struct {
		Url         string `json:"url"`
		CompleteUrl string `json:"completeUrl"`
		NosKey      string `json:"nosKey"`
	} `json:"icon"`
	GoldenIcon []struct {
		Url         string `json:"url"`
		CompleteUrl string `json:"completeUrl"`
		NosKey      string `json:"nosKey"`
	} `json:"goldenIcon"`
	BottomVipIcon []struct {
		Url         string `json:"url"`
		CompleteUrl string `json:"completeUrl"`
		NosKey      string `json:"nosKey"`
	} `json:"bottomVipIcon"`
	BottomSvipIcon []struct {
		Url         string `json:"url"`
		CompleteUrl string `json:"completeUrl"`
		NosKey      string `json:"nosKey"`
	} `json:"bottomSvipIcon"`
	FinishedTaskSubTitle           string `json:"finishedTaskSubTitle"`
	UnfinishTaskSubTitle           string `json:"unfinishTaskSubTitle"`
	TaskInitButton                 string `json:"taskInitButton "`
	TaskRewardWaitingReceiveButton string `json:"taskRewardWaitingReceiveButton "`
	JumpUrl                        string `json:"jumpUrl "`
	SeqName                        string `json:"seqName"`
	FinishedReceiveRewardJumpUrl   any    `json:"finishedReceiveRewardJumpUrl"`
}

// VipProgressList vip成长任务列表(日常任务)
// har: 43.har
// needLogin: 未知.
func (a *Api) VipProgressList(ctx context.Context, req *VipProgressListReq) (*VipProgressListResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/middle/vip/mission/user/progress/list"
		reply VipProgressListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipSignInfoReq struct {
	types.ReqCommon
}

type VipSignInfoResp struct {
	types.RespCommon[[]VipSignInfoRespData] // 貌似一次只返回7条也就是七天
}

// VipSignInfoRespData 当Today为true时则RecordId、Time、SongId有值返回则为0.
type VipSignInfoRespData struct {
	RecordId  int64  `json:"recordId"`
	UserId    int64  `json:"userId"`
	Time      int64  `json:"time"`    // eg: 1746421099000,
	TimeStr   string `json:"timeStr"` // eg: 2025-05-03
	SongId    int64  `json:"songId"`
	SongCover any    `json:"songCover"`
	Score     int64  `json:"score"` // 成长值
	Today     bool   `json:"today"` // 是否今天签到 true:是
}

// VipSignInfo 签到信息
// har: 44.har
// needLogin: 未知.
func (a *Api) VipSignInfo(ctx context.Context, req *VipSignInfoReq) (*VipSignInfoResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipnewcenter/app/user/sign/info"
		reply VipSignInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipMAXScoreReq struct {
	types.ReqCommon
}

type VipMAXScoreResp struct {
	types.RespCommon[VipMAXScoreRespData]
}

type VipMAXScoreRespData struct {
	ReachMonthMaxScore bool  `json:"reachMonthMaxScore"` // 是否达到本月最大成长值 true:是
	UnGetAllScore      int64 `json:"unGetAllScore"`      // ?
	Gap                int64 `json:"gap"`                // 剩余可获得分值数
	MaxTaskScore       int64 `json:"maxTaskScore"`       // 黑胶每月可获得300 svip为400
}

// VipMAXScore 本月可获取的最大成长值，有待确定
// har: 45.har
// needLogin: 未知.
func (a *Api) VipMAXScore(ctx context.Context, req *VipMAXScoreReq) (*VipMAXScoreResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipnewcenter/app/user/max/score"
		reply VipMAXScoreResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipNewListReq struct {
	types.ReqCommon
}

type VipNewListResp struct {
	types.RespCommon[VipNewListData]
}
type VipNewListData struct {
	RightsTypeImage map[string]string `json:"rightsTypeImage"` // VipNewListDataRightsTypeImage
	LevelAuthList   map[string]string `json:"levelAuthList"`   // key对应的等级,value: VipNewListDataLevelAuthList
}

type VipNewListDataRightsTypeImage struct {
	IconSvg any `json:"iconSvg"`
}

type VipNewListDataLevelAuthList struct {
	Id                   int64                         `json:"id"`
	RightsType           int64                         `json:"rightsType"`
	ShowName             string                        `json:"showName"`
	SubTitle             string                        `json:"subTitle"`
	RightsDetailUrl      string                        `json:"rightsDetailUrl"`
	TabornerType         int64                         `json:"tabornerType"`
	RightsIcon           VipNewListDataRightsTypeImage `json:"rightsIcon"`
	ReceiveType          int64                         `json:"receiveType"`
	ReceiveJumpUrl       any                           `json:"receiveJumpUrl"`
	AleadyReveiveJumpUrl string                        `json:"aleadyReveiveJumpUrl"`
	Status               int64                         `json:"status"`
	PrivilegeDetail      string                        `json:"privilegeDetail"`
	Seq                  int64                         `json:"seq"`
}

// VipNewList 尊享权益列表
// har: 47.har
// needLogin: 未知.
func (a *Api) VipNewList(ctx context.Context, req *VipNewListReq) (*VipNewListResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipnewcenter/app/level/auth/new/list"
		reply VipNewListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipCashierInfoReq struct {
	types.ReqCommon
}

type VipCashierInfoResp struct {
	types.RespCommon[VipCashierInfoData]
}

type VipCashierInfoData struct {
	Vip  VipCashierInfoDataVip  `json:"vip"`
	User VipCashierInfoDataUser `json:"user"`
}

type VipCashierInfoDataVip struct {
	RedVipLevelIcon            string  `json:"redVipLevelIcon"`
	RedVipLeve                 int64   `json:"redVipLeve"`
	UserVipStatus              []int64 `json:"userVipStatus"`
	RedVipAnnualCount          int64   `json:"redVipAnnualCount"`
	RedVipAnnualRequiredMonths int64   `json:"redVipAnnualRequiredMonths"`
	Associator                 struct {
		VipCode         int64  `json:"vipCode"`
		ExpireTime      int64  `json:"expireTime"`
		IconUrl         string `json:"iconUrl"`
		DynamicIconUrl  string `json:"dynamicIconUrl"`
		VipLevel        int64  `json:"vipLevel"`
		IsSignDeduct    bool   `json:"isSignDeduct"`
		IsSignIap       bool   `json:"isSignIap"`
		IsSignIapDeduct bool   `json:"isSignIapDeduct"`
		IsSign          bool   `json:"isSign"`
	} `json:"associator"`
	MusicPackage struct {
		VipCode         int64  `json:"vipCode"`
		ExpireTime      int64  `json:"expireTime"`
		IconUrl         string `json:"iconUrl"`
		DynamicIconUrl  string `json:"dynamicIconUrl"`
		VipLevel        int64  `json:"vipLevel"`
		IsSignDeduct    bool   `json:"isSignDeduct"`
		IsSignIap       bool   `json:"isSignIap"`
		IsSignIapDeduct bool   `json:"isSignIapDeduct"`
		IsSign          bool   `json:"isSign"`
	} `json:"musicPackage"`
	RedVipDynamicIconUrl  any `json:"redVipDynamicIconUrl"`
	RedVipDynamicIconUrl2 any `json:"redVipDynamicIconUrl2"`
	Redplus               struct {
		VipCode         int64  `json:"vipCode"`
		ExpireTime      int64  `json:"expireTime"`
		IconUrl         string `json:"iconUrl"`
		DynamicIconUrl  any    `json:"dynamicIconUrl"`
		VipLevel        int64  `json:"vipLevel"`
		IsSignDeduct    bool   `json:"isSignDeduct"`
		IsSignIap       bool   `json:"isSignIap"`
		IsSignIapDeduct bool   `json:"isSignIapDeduct"`
		IsSign          bool   `json:"isSign"`
	} `json:"redplus"`
}

type VipCashierInfoDataUser struct {
	Account struct {
		Id                 int64  `json:"id"`
		UserName           string `json:"userName"`
		Type               int64  `json:"type"`
		Status             int64  `json:"status"`
		WhitelistAuthority int64  `json:"whitelistAuthority"`
		CreateTime         int64  `json:"createTime"`
		TokenVersion       int64  `json:"tokenVersion"`
		Ban                int64  `json:"ban"`
		BaoyueVersion      int64  `json:"baoyueVersion"`
		DonateVersion      int64  `json:"donateVersion"`
		VipType            int64  `json:"vipType"`
		IsAnonimousUser    bool   `json:"isAnonimousUser"`
		AdjustSongVersion  int64  `json:"adjustSongVersion"`
		ViptypeVersion     int64  `json:"viptypeVersion"`
		IsPaidFee          bool   `json:"isPaidFee"`
	} `json:"account"`
	Profile struct {
		UserId            int64  `json:"userId"`
		UserType          int64  `json:"userType"`
		Nickname          string `json:"nickname"`
		AvatarImgId       int64  `json:"avatarImgId"`
		AvatarUrl         string `json:"avatarUrl"`
		BackgroundImgId   int64  `json:"backgroundImgId"`
		BackgroundUrl     string `json:"backgroundUrl"`
		Signature         string `json:"signature"`
		CreateTime        int64  `json:"createTime"`
		UserName          string `json:"userName"`
		AccountType       int64  `json:"accountType"`
		ShortUserName     string `json:"shortUserName"`
		Birthday          int64  `json:"birthday"`
		Authority         int64  `json:"authority"`
		Gender            int64  `json:"gender"`
		AccountStatus     int64  `json:"accountStatus"`
		Province          int64  `json:"province"`
		City              int64  `json:"city"`
		AuthStatus        int64  `json:"authStatus"`
		Description       any    `json:"description"`
		DetailDescription any    `json:"detailDescription"`
		IsDefaultAvatar   bool   `json:"isDefaultAvatar"`
		ExpertTags        any    `json:"expertTags"`
		Experts           any    `json:"experts"`
		DjStatus          int64  `json:"djStatus"`
		LocationStatus    int64  `json:"locationStatus"`
		VipType           int64  `json:"vipType"`
		IsFollowed        bool   `json:"isFollowed"`
		IsMutual          bool   `json:"isMutual"`
		IsAuthenticated   bool   `json:"isAuthenticated"`
		LastLoginTime     int64  `json:"lastLoginTime"`
		LastLoginIP       string `json:"lastLoginIP"`
		RemarkName        any    `json:"remarkName"`
	} `json:"profile"`
}

// VipCashierInfo todo:具体作用待分析
// har: 48.har
// needLogin: 未知.
func (a *Api) VipCashierInfo(ctx context.Context, req *VipCashierInfoReq) (*VipCashierInfoResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/music-vip-membership/cashier/info"
		reply VipCashierInfoResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipLevelListReq struct {
	types.ReqCommon
}

type VipLevelListResp struct {
	types.RespCommon[[]VipLevelListData]
}

type VipLevelListData struct {
	Level                          int64  `json:"level"`          // 2
	Title                          string `json:"title"`          // eg: 黑胶·贰
	MaxGrowthPoint                 int64  `json:"maxGrowthPoint"` // 1500
	MinGrowthPoint                 int64  `json:"minGrowthPoint"` // 480
	LevelBackgroundCardImage       string `json:"levelBackgroundCardImage"`
	LevelBackgroundCardExpireImage string `json:"levelBackgroundCardExpireImage,omitempty"`
	VipLevelPageCardImg            string `json:"vipLevelPageCardImg"`
	VipLevelPageExpireCardImg      string `json:"vipLevelPageExpireCardImg"`
}

// VipLevelList 获取黑胶每升一级成长值
// har: 49.har
// needLogin: 未知.
func (a *Api) VipLevelList(ctx context.Context, req *VipLevelListReq) (*VipLevelListResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipnewcenter/app/level/list"
		reply VipLevelListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipWelfareListReq struct {
	types.ReqCommon
}

type VipWelfareListResp struct {
	types.RespCommon[map[string][]VipWelfareListData] // key为等级
}

type VipWelfareListData struct {
	Id          int64  `json:"id"`
	Type        int64  `json:"type"`
	Level       int64  `json:"level"` // 对应黑胶等级
	ShowName    string `json:"showName"`
	WelfareIcon struct {
		IconNosKey            string `json:"iconNosKey"`
		IconImgUrl            string `json:"iconImgUrl"`
		FrontColor            string `json:"frontColor"`
		BackgroundColor       string `json:"backgroundColor"`
		SolidBackgroundColor  string `json:"solidBackgroundColor"`
		ButtonBackgroundColor string `json:"buttonBackgroundColor"`
		ButtonTextColor       string `json:"buttonTextColor"`
	} `json:"welfareIcon,omitzero"`
	JumpUrl           string  `json:"jumpUrl"`
	EffectStartTime   int64   `json:"effectStartTime"`
	EffectEndTime     int64   `json:"effectEndTime"`
	ShowMinLevel      int64   `json:"showMinLevel"`
	ShowMaxLevel      int64   `json:"showMaxLevel"`
	SpecialPrice      float64 `json:"specialPrice"`
	OrginalPrice      float64 `json:"orginalPrice"`
	Status            int64   `json:"status"`
	UserReceiveStatus int64   `json:"userReceiveStatus"`
	Seq               int64   `json:"seq"`
	WelfareItem       struct {
		Level        int64  `json:"level"`
		SubtitleName string `json:"subtitleName"`
	} `json:"welfareItem"`
}

// VipWelfareList 尊享福利列表
// har: 50.har
// needLogin: 未知.
func (a *Api) VipWelfareList(ctx context.Context, req *VipWelfareListReq) (*VipWelfareListResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipnewcenter/app/level/welfare/new/list"
		reply VipWelfareListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipDetailListReq struct {
	types.ReqCommon

	IsSupportHistoryGift bool `json:"isSupportHistoryGift"`
}

type VipDetailListResp struct {
	types.RespCommon[[]VipDetailListData] // key为等级
}

type VipDetailListData struct {
	Id              int64  `json:"id"`
	RightsType      int64  `json:"rightsType"`
	SubRightsType   int64  `json:"subRightsType"`
	ShowName        string `json:"showName"`
	SubTitle        string `json:"subTitle"`
	RightsDetailUrl string `json:"rightsDetailUrl"`
	CornerType      int64  `json:"cornerType"`
	RightsIcon      struct {
		IconSvg string `json:"iconSvg"`
	} `json:"rightsIcon"`
	ReceiveType          int64  `json:"receiveType"`
	LimitType            int64  `json:"limitType"`
	ReceiveJumpUrl       string `json:"receiveJumpUrl"`
	AleadyReveiveJumpUrl string `json:"aleadyReveiveJumpUrl"`
	Level                int64  `json:"level"`
	ReceiveStatus        int64  `json:"receiveStatus"`
	PrivilegeDetail      string `json:"privilegeDetail"`
	DeliveryId           int64  `json:"deliveryId"`
	Seq                  int64  `json:"seq"`
}

// VipDetailList 等级特权详情列表
// har: 51.har
// needLogin: 未知.
func (a *Api) VipDetailList(ctx context.Context, req *VipDetailListReq) (*VipDetailListResp, error) {
	var (
		url = fmt.Sprintf("https://interface3.music.163.com/weapi/vipnewcenter/app/level/auth/new/detail/list?isSupportHistoryGift=%v",
			req.IsSupportHistoryGift)
		reply VipDetailListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipConfigReq struct {
	types.ReqCommon

	ConfigName string `json:"configName"` // viplevel.paytask.tasklist
}

type VipConfigResp struct {
	types.RespCommon[string] // VipConfigData
}
type VipConfigData struct {
	Title string              `json:"title"`
	List  []VipConfigDataList `json:"list"`
}

type VipConfigDataList struct {
	Action      string `json:"action"`
	Description string `json:"description"`
	IconUrl     string `json:"iconUrl"`
	TaskTag     string `json:"taskTag"`
	BtnText     string `json:"btnText"`
	JumpUrl     string `json:"jumpUrl"`
}

// VipConfig 快速成长通道
// har: 52.har
// needLogin: 未知.
func (a *Api) VipConfig(ctx context.Context, req *VipConfigReq) (*VipConfigResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/music-vip-configuration/config/query"
		reply VipConfigResp
		opts  = api.NewOptions()
	)

	if req.ConfigName == "" {
		req.ConfigName = "viplevel.paytask.tasklist"
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipDowngradeCompensateReq struct {
	types.ReqCommon
}

type VipDowngradeCompensateResp struct {
	types.RespCommon[VipDowngradeCompensateData]
}
type VipDowngradeCompensateData struct {
	UserId                   int64 `json:"userId"`
	NickName                 any   `json:"nickName"`
	UserAvatarUrl            any   `json:"userAvatarUrl"`
	Downgrade                bool  `json:"downgrade"`
	Receive                  bool  `json:"receive"`
	ReceiveScore             int64 `json:"receiveScore"`
	Level                    int64 `json:"level"`
	IconImg                  any   `json:"iconImg"`
	ReceiveScoreAfterLevel   int64 `json:"receiveScoreAfterLevel"`
	ReceiveScoreAfterIconImg any   `json:"receiveScoreAfterIconImg"`
	ActivityValid            bool  `json:"activityValid"`
	SpecialV6                bool  `json:"specialV6"`
}

// VipDowngradeCompensate 降级补偿,貌似实在黑胶乐签领取失败场景触发
// har: 53.har
// needLogin: 未知.
func (a *Api) VipDowngradeCompensate(ctx context.Context, req *VipDowngradeCompensateReq) (*VipDowngradeCompensateResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipnewcenter/app/level/downgrade/compensate"
		reply VipDowngradeCompensateResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipInterestsReq struct {
	types.ReqCommon

	InterestsType string `json:"interestsType"`
}

type VipInterestsResp struct {
	types.RespCommon[any]
}

// VipInterests 未知待分析
// har: 54.har
// needLogin: 未知.
func (a *Api) VipInterests(ctx context.Context, req *VipInterestsReq) (*VipInterestsResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipauth/app/interests/userrecord/get"
		reply VipInterestsResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipFloatDataReq struct {
	types.ReqCommon
}

type VipFloatDataResp struct {
	types.RespCommon[VipFloatDataRespData]
}

type VipFloatDataRespData struct {
	BirthdayData struct {
		UserId     int64  `json:"userId"`
		NickName   string `json:"nickName"`
		Birthday   string `json:"birthday"`
		NeedPopUp  bool   `json:"needPopUp"`
		CurrentDay bool   `json:"currentDay"`
	} `json:"birthdayData"`
	FloatTip     any `json:"floatTip"`
	GiftCardTip  any `json:"giftCardTip"`
	LevelPopData struct {
		Code    int64  `json:"code"`
		Data    any    `json:"data"`
		Message string `json:"message"`
	} `json:"levelPopData"`
	PopupData     any `json:"popupData"`
	TopRedDotData struct {
		NewWelfareIdLevelMap struct {
			Field1 int64 `json:"0"`
			Field2 int64 `json:"1"`
			Field3 int64 `json:"2"`
			Field4 int64 `json:"3"`
			Field5 int64 `json:"4"`
			Field6 int64 `json:"5"`
			Field7 int64 `json:"6"`
			Field8 int64 `json:"7"`
		} `json:"newWelfareIdLevelMap"`
		NewGiftCard string `json:"newGiftCard"`
	} `json:"topRedDotData"`
	ViewTaskData any `json:"viewTaskData"`
	VipInfo      struct {
		RedVipLevelIcon   string `json:"redVipLevelIcon"`
		RedVipLevel       int64  `json:"redVipLevel"`
		RedVipAnnualCount int64  `json:"redVipAnnualCount"`
		MusicPackage      struct {
			VipCode         int64  `json:"vipCode"`
			ExpireTime      int64  `json:"expireTime"`
			IconUrl         string `json:"iconUrl"`
			DynamicIconUrl  string `json:"dynamicIconUrl"`
			VipLevel        int64  `json:"vipLevel"`
			IsSignDeduct    bool   `json:"isSignDeduct"`
			IsSignIap       bool   `json:"isSignIap"`
			IsSignIapDeduct bool   `json:"isSignIapDeduct"`
			IsSign          bool   `json:"isSign"`
		} `json:"musicPackage"`
		Associator struct {
			VipCode         int64  `json:"vipCode"`
			ExpireTime      int64  `json:"expireTime"`
			IconUrl         string `json:"iconUrl"`
			DynamicIconUrl  string `json:"dynamicIconUrl"`
			VipLevel        int64  `json:"vipLevel"`
			IsSignDeduct    bool   `json:"isSignDeduct"`
			IsSignIap       bool   `json:"isSignIap"`
			IsSignIapDeduct bool   `json:"isSignIapDeduct"`
			IsSign          bool   `json:"isSign"`
		} `json:"associator"`
		RedVipDynamicIconUrl  any `json:"redVipDynamicIconUrl"`
		RedVipDynamicIconUrl2 any `json:"redVipDynamicIconUrl2"`
		Redplus               struct {
			VipCode         int64  `json:"vipCode"`
			ExpireTime      int64  `json:"expireTime"`
			IconUrl         string `json:"iconUrl"`
			DynamicIconUrl  string `json:"dynamicIconUrl"`
			VipLevel        int64  `json:"vipLevel"`
			IsSignDeduct    bool   `json:"isSignDeduct"`
			IsSignIap       bool   `json:"isSignIap"`
			IsSignIapDeduct bool   `json:"isSignIapDeduct"`
			IsSign          bool   `json:"isSign"`
		} `json:"redplus"`
	} `json:"vipInfo"`
	ValidUser   bool `json:"validUser"`
	CashierData struct {
		IsNotify   bool   `json:"isNotify"`
		Link       string `json:"link"`
		CashierTab any    `json:"cashierTab"`
	} `json:"cashierData"`
	BirthdayEggData struct {
		IsNotify  bool   `json:"isNotify"`
		Link      string `json:"link"`
		LottieUrl string `json:"lottieUrl"`
		SourceUrl string `json:"sourceUrl"`
	} `json:"birthdayEggData"`
}

// VipFloatData todo: 相关用户信息数据待明确
// har: 55.har
// needLogin: 未知.
func (a *Api) VipFloatData(ctx context.Context, req *VipFloatDataReq) (*VipFloatDataResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vip-center-bff/float/data"
		reply VipFloatDataResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipCommonListReq struct {
	types.ReqCommon

	PositionType string `json:"positionType"` // eg:25
}

type VipCommonListResp struct {
	types.RespCommon[VipCommonListRespData]
}
type VipCommonListRespData struct {
	ContentList []string `json:"contentList"` // 里面是jsonVipCommonListRespDataContentList
}

type VipCommonListRespDataContentList struct {
	Type     string                                     `json:"type"`
	Title    string                                     `json:"title"`
	MoreUrl  string                                     `json:"moreUrl"`
	MoreText string                                     `json:"moreText"`
	Children []VipCommonListRespDataContentListChildren `json:"children"`
}

type VipCommonListRespDataContentListChildren struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	MoreUrl  string `json:"moreUrl"`
	MoreText string `json:"moreText"`
	Collapse any    `json:"collapse"`
}

// VipCommonList 首页布局列表例如：vip成长任务、热门活动、我的vip特权
// har: 56.har
// needLogin: 未知.
func (a *Api) VipCommonList(ctx context.Context, req *VipCommonListReq) (*VipCommonListResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipnewcenter/app/resource/common/list"
		reply VipCommonListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipNewAccountPageReq struct {
	types.ReqCommon

	GroupName string `json:"groupName"` // eg: t2
}

type VipNewAccountPageResp struct {
	types.RespCommon[VipNewAccountPageRespData]
}

type VipNewAccountPageRespData struct {
	MainTitle struct {
		VipCurrLevel  int64  `json:"vipCurrLevel"`  // 当前vip等级
		SubPercent    int64  `json:"subPercent"`    // 进度百分比 eg: 86
		NextLevel     int64  `json:"nextLevel"`     // 下一级vip等级
		ImgUrl        string `json:"imgUrl"`        // img
		ReachMaxLevel bool   `json:"reachMaxLevel"` // 是否满级
		JumpUrl       string `json:"jumpUrl"`       // 站内跳转地址
		CurrScore     int64  `json:"currScore"`     // 当前分数
	} `json:"mainTitle"`
	SubTitle struct {
		CarouselKey string   `json:"carouselKey"` // eg: 1107_1745808226000
		Carousels   []string `json:"carousels"`   // "黑胶有效期仅6天","会员任务｜打卡升级提醒⏰","会员权益｜联名装扮上新","会员福利｜上新福利集合","黑胶时光机｜音乐报告周日更新"
		HasRedot    bool     `json:"hasRedot"`
	} `json:"subTitle"`
	ButtonTitle struct {
		Title   string `json:"title"` // 会员中心
		JumpUrl string `json:"jumpUrl"`
	} `json:"buttonTitle"`
}

// VipNewAccountPage 会员中心(手机点击抽屉顶部所展示的信息)
// har: 57.har
// needLogin: 未知.
func (a *Api) VipNewAccountPage(ctx context.Context, req *VipNewAccountPageReq) (*VipNewAccountPageResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipnewcenter/app/resource/newaccountpage"
		reply VipNewAccountPageResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

// VipTaskSignReq 黑胶乐签签到请求。
type VipTaskSignReq struct {
	types.ReqCommon

	IsNew string `json:"isNew,omitempty"` // 可选字段；本次桌面端 EAPI 抓包未携带，具体取值语义待确认。
}

// VipTaskSignResp 黑胶乐签签到响应。
type VipTaskSignResp struct {
	types.RespCommon[bool]
}

// VipTaskSign 黑胶乐签
// har: 58.har、59.har
// needLogin: 未知.
func (a *Api) VipTaskSign(ctx context.Context, req *VipTaskSignReq) (*VipTaskSignResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vip-center-bff/task/sign"
		reply VipTaskSignResp
		opts  = api.NewOptions()
	)
	if req.IsNew != "" {
		url = url + "?isNew=" + req.IsNew
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipCheckinHistoryListReq struct {
	types.ReqCommon
}

type VipCheckinHistoryListResp struct {
	Code    int                             `json:"code"`
	Message string                          `json:"message"`
	Data    []VipCheckinHistoryListRespData `json:"data"`
}

type VipCheckinHistoryListRespData struct {
	RecordID   int    `json:"recordId"`
	UserID     int    `json:"userId"`
	DayTime    int64  `json:"dayTime"`
	DayTimeStr string `json:"dayTimeStr"`
	Time       int64  `json:"time"`
	SongID     int    `json:"songId"`
	SongCover  string `json:"songCover"`
}

// VipCheckinHistoryList 获取已经签到的记录列表.
func (a *Api) VipCheckinHistoryList(ctx context.Context, req *VipCheckinHistoryListReq) (*VipCheckinHistoryListResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipnewcenter/app/level/user/checkin/history/list"
		reply VipCheckinHistoryListResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

// VipCheckinHistoryDetailReq 指定乐签日期的详情请求。
//
// 请求示例：VipCheckinHistoryDetailReq{SignDayTime: 1785913200098, Type: 1}。
type VipCheckinHistoryDetailReq struct {
	types.ReqCommon

	SignDayTime string `json:"signDayTime,omitempty"` // 乐签时间，Unix 毫秒时间戳；发送时编码为字符串。 当type为1时必传。
	Type        string `json:"type"`                  // 详情类型；详情类型；1,2。
	RecordId    string `json:"recordId,omitempty"`    // 当type为2时必传。为/api/vipnewcenter/app/level/user/checkin/history/list列表中得recordId
}

// VipCheckinHistoryDetailResp 指定日期的乐签详情响应。
type VipCheckinHistoryDetailResp struct {
	types.RespCommon[VipCheckinHistoryDetailData]
}

// VipCheckinHistoryDetailData 乐签详情数据。
type VipCheckinHistoryDetailData struct {
	RecordId              int64                               `json:"recordId"`             // 乐签记录 ID；抓包值为 0。
	UserId                int64                               `json:"userId"`               // 乐签用户 ID。
	Time                  int64                               `json:"time"`                 // 乐签时间，Unix 毫秒时间戳。
	SongSrc               int                                 `json:"songSrc"`              // 歌曲来源类型；抓包值为 3，枚举含义待确认。
	ShowTag               any                                 `json:"showTag"`              // 展示标签；抓包返回 null，具体结构待确认。
	SongInfo              *VipCheckinHistoryDetailSongInfo    `json:"songInfo"`             // 本次乐签关联的歌曲信息。
	WishWords             string                              `json:"wishWords"`            // 乐签寄语文案。
	WishWordType          int                                 `json:"wishWordType"`         // 寄语类型；抓包值为 3，枚举含义待确认。
	WishUserNickname      *string                             `json:"wishUserNickname"`     // 寄语来源用户昵称；抓包返回 null。
	PeriodDto             *VipCheckinHistoryDetailPeriod      `json:"periodDto"`            // 当前乐签活动周期。
	MonthCheckInTotalDay  int                                 `json:"monthCheckInTotalDay"` // 当月累计乐签天数；抓包示例为 3。
	SurprisePkgVo         any                                 `json:"surprisePkgVo"`        // 惊喜礼包信息；抓包返回 null，具体结构待确认。
	MonthCheckInPrizeList []VipCheckinHistoryDetailMonthPrize `json:"monthCheckInPrizList"` // 当月阶段奖励；服务端字段拼写为 PrizList。
	SceneId               int64                               `json:"sceneId"`              // 场景 ID；抓包值为 66。
	JumpUrl               string                              `json:"jumpUrl"`              // 乐签详情关联的跳转地址。
}

// VipCheckinHistoryDetailSongInfo 乐签歌曲信息。
type VipCheckinHistoryDetailSongInfo struct {
	SongId     int64   `json:"songId"`     // 歌曲 ID。
	SongName   string  `json:"songName"`   // 歌曲名称。
	ArtistName string  `json:"artistName"` // 歌手展示名称。
	Album      string  `json:"album"`      // 专辑名称。
	Cover      string  `json:"cover"`      // 歌曲或专辑封面 URL。
	ArtistIds  []int64 `json:"artistIds"`  // 歌手 ID 列表。
	Seq        int     `json:"seq"`        // 展示序号；抓包值为 0。
}

// VipCheckinHistoryDetailPeriod 乐签活动周期。
type VipCheckinHistoryDetailPeriod struct {
	PeriodType int    `json:"periodType"` // 周期类型；抓包值为 3，枚举含义待确认。
	StartTime  string `json:"startTime"`  // 周期开始时间，字符串形式的 Unix 毫秒时间戳。
	EndTime    string `json:"endTime"`    // 周期结束时间，字符串形式的 Unix 毫秒时间戳。
}

// VipCheckinHistoryDetailMonthPrize 当月累计乐签奖励节点。
type VipCheckinHistoryDetailMonthPrize struct {
	PrizeId           int64  `json:"prizeId"`           // 奖励 ID。
	VipType           int    `json:"vipType"`           // VIP 权益类型；抓包值为 2，枚举含义待确认。
	PrizeType         int    `json:"prizeType"`         // 奖励类型；抓包值为 0，枚举含义待确认。
	Day               int    `json:"day"`               // 获得奖励所需的当月累计乐签天数，如 7、14、28。
	PrizeShowName     string `json:"prizeShowName"`     // 奖励展示名称，如 VIP、高清臻音。
	ShowSubTitle      string `json:"showSubTitle"`      // 奖励副标题，如 3天。
	UnitNum           int    `json:"unitNum"`           // 奖励数量；抓包中的 3 天 VIP 对应 3。
	UserPrizeRecordId int64  `json:"userPrizeRecordId"` // 用户奖励记录 ID；抓包值为 0。
	Time              int64  `json:"time"`              // 奖励时间字段；抓包值为 0，具体语义待确认。
}

// VipCheckinHistoryDetail 获取指定日期乐签详情。
func (a *Api) VipCheckinHistoryDetail(ctx context.Context, req *VipCheckinHistoryDetailReq) (*VipCheckinHistoryDetailResp, error) {
	var (
		url   = "https://interface3.music.163.com/weapi/vipnewcenter/app/level/user/checkin/history/detail"
		reply VipCheckinHistoryDetailResp
		opts  = api.NewOptions()
	)

	if req.Type == "" {
		return nil, errors.New("type is empty")
	}

	if req.Type == "1" && req.SignDayTime == "" {
		return nil, errors.New("SignDayTime is empty")
	}

	if req.Type == "2" && req.RecordId == "" {
		return nil, errors.New("RecordId is empty")
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

// VipMinideskMusicSignPCReq 桌面端乐签卡片请求。
//
// 完整桌面端流程会依次请求 Type 0 和 1。
type VipMinideskMusicSignPCReq struct {
	types.ReqCommon

	Type int `json:"-"` // 卡片类型；对应 EAPI 抓包中 0 返回简版提示，1 返回月度乐签详情。
}

type vipMinideskMusicSignPCReq struct {
	types.ReqCommon

	Type string `json:"type"`
}

// VipMinideskMusicSignPCResp 桌面端乐签卡片响应。
type VipMinideskMusicSignPCResp struct {
	types.RespCommon[VipMinideskMusicSignPCData]
}

// VipMinideskMusicSignPCData 桌面端乐签卡片数据。
type VipMinideskMusicSignPCData struct {
	Text         *string                          `json:"text"`         // 卡片标题；Type 0 时可为 null，Type 1 示例为“8月黑胶乐签”。
	SubText      string                           `json:"subText"`      // 连续乐签进度或下一奖励提示。
	RedPrize     bool                             `json:"redPrize"`     // 奖励红点标识。
	RedDay       bool                             `json:"redDay"`       // 乐签日期红点标识。
	BtnText      string                           `json:"btnText"`      // 操作按钮文案，如“查看乐签”。
	BgTexture    string                           `json:"bgTexture"`    // 背景纹理 URL；Type 0 抓包中为空字符串。
	SignInfoList []VipMinideskMusicSignPCSignInfo `json:"signInfoList"` // 卡片展示的近期乐签记录。
}

// VipMinideskMusicSignPCSignInfo 卡片中的单日乐签状态。
type VipMinideskMusicSignPCSignInfo struct {
	DayText      string  `json:"dayText"`      // 日期展示文案，如“5日”。
	Sign         bool    `json:"sign"`         // 当日是否已完成乐签。
	SongCoverUrl *string `json:"songCoverUrl"` // 乐签歌曲封面 URL；未乐签记录可为 null。
	SignTime     int64   `json:"signTime"`     // 乐签时间，Unix 毫秒时间戳；未乐签时为 0。
	Today        bool    `json:"today"`        // 是否为当天记录。
}

// VipMinideskMusicSignPC 获取桌面端黑胶乐签卡片信息。
func (a *Api) VipMinideskMusicSignPC(ctx context.Context, req *VipMinideskMusicSignPCReq) (*VipMinideskMusicSignPCResp, error) {
	var (
		url     = "https://interface3.music.163.com/weapi/vipnewcenter/app/minidesk/music/sign/pc"
		reply   VipMinideskMusicSignPCResp
		opts    = api.NewOptions()
		request = vipMinideskMusicSignPCReq{
			ReqCommon: req.ReqCommon,
			Type:      strconv.Itoa(req.Type),
		}
	)

	resp, err := a.client.Request(ctx, url, &request, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipWelfareClaimReq struct {
	types.ReqCommon

	WelfareId int64 `json:"welfareId"` // 福利id
}

type VipWelfareClaimResp struct {
	types.RespCommon[any]
}

// VipWelfareClaim 领取黑胶会员尊享福利
// needLogin: 是.
func (a *Api) VipWelfareClaim(ctx context.Context, req *VipWelfareClaimReq) (*VipWelfareClaimResp, error) {
	var (
		url   = "https://music.163.com/weapi/vipnewcenter/app/level/welfare/claim"
		reply VipWelfareClaimResp
		opts  = api.NewOptions()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipMiddlePageViewReportReq struct {
	types.ReqCommon

	WebkitContext string `json:"-"` // netease_webkit_context, 透传到请求头
	Data          string `json:"data"`
}

type VipMiddlePageViewReportData struct {
	ActionType         string `json:"actionType"`
	Time               int64  `json:"time"`
	ActivityPlatformId string `json:"activityPlatformId,omitempty"`
	TaskId             string `json:"taskId"`
	TaskType           int    `json:"taskType"`
	ViewTime           int64  `json:"viewTime"`
	JumpUrl            string `json:"jumpUrl"`
	TaskBusiness       string `json:"taskBusiness"`
	ResourceType       string `json:"resourceType"`
	PageCode           string `json:"pageCode"`
}

type VipMiddlePageViewReportResp struct {
	Code    int    `json:"code"`
	Data    bool   `json:"data"`
	Message string `json:"message"`
}

// VipMiddlePageViewReport 上报中台页面浏览时长
// needLogin: 是.
func (a *Api) VipMiddlePageViewReport(ctx context.Context, req *VipMiddlePageViewReportReq) (*VipMiddlePageViewReportResp, error) {
	var (
		url   = "https://interface.music.163.com/weapi/middle/page/view/report"
		reply VipMiddlePageViewReportResp
		opts  = api.NewOptions()
	)

	// 模拟 webview 环境设置自定义 User-Agent、Host、Origin、Referer 及 netease_webkit_context
	opts.SetHeader("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 CloudMusic/0.1.1 NeteaseMusic/9.4.95")
	opts.SetHeader("Host", "interface.music.163.com")
	opts.SetHeader("Origin", "https://music.163.com")
	opts.SetHeader("Referer", "https://music.163.com/")

	if req.WebkitContext != "" {
		opts.SetHeader("netease_webkit_context", req.WebkitContext)
	}

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

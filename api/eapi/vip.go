// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
)

type VipTaskListReq struct {
	types.EApiReqCommon

	IsNew int `json:"isNew,omitempty"`
}

type VipTaskListResp struct {
	Code int               `json:"code"`
	Data []VipTaskListData `json:"data"`
}

type VipTaskListData struct {
	Point           int64  `json:"point"`
	MissionId       int64  `json:"missionId"`
	MissionType     int64  `json:"missionType"`
	MissionEntityId int64  `json:"missionEntityId"`
	MissionCode     string `json:"missionCode"`
	Status          int64  `json:"status"` // 100: 已打卡/已完成 10: 未完成
	Worth           int64  `json:"worth"`
	MainTitle       string `json:"mainTitle"`
	SubTitle        string `json:"subTitle"`
	JumpUrl         string `json:"jumpUrl"`
	ButtonText      string `json:"buttonText"`
}

// VipTaskList 获取黑胶 VIP 任务列表.
func (a *Api) VipTaskList(ctx context.Context, req *VipTaskListReq) (*VipTaskListResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vip-center-bff/task/list"
		reply VipTaskListResp
		opts  = api.NewOptions("eapi.VipTaskList").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipCommonReq struct {
	types.EApiReqCommon
}

type VipCommonResp struct {
	Code    int    `json:"code"`
	Data    any    `json:"data"`
	Message string `json:"message"`
}

// VipTaskSignReq 黑胶乐签签到请求。
type VipTaskSignReq struct {
	types.EApiReqCommon

	IsNew string `json:"isNew,omitempty"` // 可选字段；本次桌面端抓包未携带，具体取值语义待确认。
}

// VipTaskSignResp 黑胶乐签签到响应。
type VipTaskSignResp struct {
	Code    int    `json:"code"`    // 业务状态码；抓包成功响应为 200。
	Data    bool   `json:"data"`    // 是否签到成功；抓包成功响应为 true。
	Message string `json:"message"` // 服务端消息；抓包成功响应为空字符串。
}

// VipTaskSign 执行尊享 VIP 签到 (EAPI).
func (a *Api) VipTaskSign(ctx context.Context, req *VipTaskSignReq) (*VipTaskSignResp, error) {
	var (
		url   = "https://interface.music.163.com/eapi/vip-center-bff/task/sign"
		reply VipTaskSignResp
		opts  = api.NewOptions("eapi.VipTaskSign").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipSignInfoReq struct {
	types.EApiReqCommon
}

type VipSignInfoResp struct {
	Code    int               `json:"code"`
	Data    []VipSignInfoData `json:"data"`
	Message string            `json:"message"`
}

type VipSignInfoData struct {
	RecordId  int64  `json:"recordId"`
	UserId    int64  `json:"userId"`
	Time      int64  `json:"time"`
	TimeStr   string `json:"timeStr"`
	SongId    int64  `json:"songId"`
	SongCover any    `json:"songCover"`
	Score     int64  `json:"score"`
	Today     bool   `json:"today"`
}

// VipSignInfo 获取黑胶乐签最近签到记录 (EAPI)。
func (a *Api) VipSignInfo(ctx context.Context, req *VipSignInfoReq) (*VipSignInfoResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/user/sign/info"
		reply VipSignInfoResp
		opts  = api.NewOptions("eapi.VipSignInfo").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipGrowPointReq struct {
	types.EApiReqCommon
}

type VipGrowPointResp struct {
	Code    int              `json:"code"`
	Data    VipGrowPointData `json:"data"`
	Message string           `json:"message"`
}

type VipGrowPointData struct {
	UserLevel VipGrowPointUserLevel `json:"userLevel"`
}

type VipGrowPointUserLevel struct {
	UserId          int64  `json:"userId"`
	Level           int64  `json:"level"`
	GrowthPoint     int64  `json:"growthPoint"`
	LevelName       string `json:"levelName"`
	ExtJson         string `json:"extJson"`
	LatestVipStatus int64  `json:"latestVipStatus"`
}

// VipGrowPoint 获取黑胶成长值状态 (EAPI)。
func (a *Api) VipGrowPoint(ctx context.Context, req *VipGrowPointReq) (*VipGrowPointResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/level/growhpoint/basic"
		reply VipGrowPointResp
		opts  = api.NewOptions("eapi.VipGrowPoint").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

// VipOldSignPrizeList 获取旧版连续乐签奖品列表，用于模拟 App 打卡后的刷新链路。
func (a *Api) VipOldSignPrizeList(ctx context.Context, req *VipCommonReq) (*VipCommonResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/level/user/checkin/old/sign-prize/list"
		reply VipCommonResp
		opts  = api.NewOptions("eapi.VipOldSignPrizeList").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipMonthPrizeListResp struct {
	Code int `json:"code"`
	Data struct {
		MonthCheckInTotalDay int64 `json:"monthCheckInTotalDay"`
		NextPrzieRemaingDay  int64 `json:"nextPrzieRemaingDay"`
		TodayDailyGrowth     int64 `json:"todayDailyGrowth"`
		PrizeList            []struct {
			Day               int64  `json:"day"`
			PrizeId           int64  `json:"prizeId"`
			PrizeShowName     string `json:"prizeShowName"`
			PrizeType         int64  `json:"prizeType"`
			ShowSubTitle      string `json:"showSubTitle"`
			Time              int64  `json:"time"`
			UnitNum           int64  `json:"unitNum"`
			UserPrizeRecordId int64  `json:"userPrizeRecordId"`
			VipType           int64  `json:"vipType"`
		} `json:"przeList"`
	} `json:"data"`
	Message string `json:"message"`
}

// VipMonthPrizeList 获取本月乐签奖品列表，用于模拟 App 打卡后的刷新链路。
func (a *Api) VipMonthPrizeList(ctx context.Context, req *VipCommonReq) (*VipMonthPrizeListResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/level/user/checkin/month-prize/list"
		reply VipMonthPrizeListResp
		opts  = api.NewOptions("eapi.VipMonthPrizeList").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

// VipFrontInfo 获取会员前台信息，用于模拟 App 打卡后的刷新链路。
func (a *Api) VipFrontInfo(ctx context.Context, req *VipCommonReq) (*VipCommonResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/music-vip-membership/front/vip/info"
		reply VipCommonResp
		opts  = api.NewOptions("eapi.VipFrontInfo").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipCheckinHistoryListReq struct {
	types.EApiReqCommon
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
		url   = "https://interface.music.163.com/eapi/vipnewcenter/app/level/user/checkin/history/list"
		reply VipCheckinHistoryListResp
		opts  = api.NewOptions("eapi.VipCheckinHistoryList").SetEAPI()
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
	types.EApiReqCommon

	SignDayTime string `json:"signDayTime,omitempty"` // 乐签时间，Unix 毫秒时间戳；发送时编码为字符串。 当type为1时必传。
	Type        string `json:"type"`                  // 详情类型；详情类型；1,2。
	RecordId    string `json:"recordId,omitempty"`    // 当type为2时必传。为/api/vipnewcenter/app/level/user/checkin/history/list列表中得recordId
}

// VipCheckinHistoryDetailResp 指定日期的乐签详情响应。
type VipCheckinHistoryDetailResp struct {
	Code    int                         `json:"code"`    // 业务状态码，200 表示成功。
	Data    VipCheckinHistoryDetailData `json:"data"`    // 乐签记录、歌曲、寄语和当月奖励信息。
	Message string                      `json:"message"` // 服务端消息，成功时通常为空。
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
		url   = "https://interface.music.163.com/eapi/vipnewcenter/app/level/user/checkin/history/detail"
		reply VipCheckinHistoryDetailResp
		opts  = api.NewOptions("eapi.VipCheckinHistoryDetail").SetEAPI()
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
	types.EApiReqCommon

	Type int `json:"-"` // 卡片类型；抓包中 0 返回简版提示，1 返回月度乐签详情。
}

type vipMinideskMusicSignPCReq struct {
	types.EApiReqCommon

	Type string `json:"type"`
}

// VipMinideskMusicSignPCResp 桌面端乐签卡片响应。
type VipMinideskMusicSignPCResp struct {
	Code    int                        `json:"code"`    // 业务状态码，200 表示成功。
	Data    VipMinideskMusicSignPCData `json:"data"`    // 卡片文案和近期乐签记录。
	Message string                     `json:"message"` // 服务端消息，成功时通常为空。
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
		url     = "https://interface.music.163.com/eapi/vipnewcenter/app/minidesk/music/sign/pc"
		reply   VipMinideskMusicSignPCResp
		opts    = api.NewOptions("eapi.VipMinideskMusicSignPC").SetEAPI()
		request = vipMinideskMusicSignPCReq{
			EApiReqCommon: req.EApiReqCommon,
			Type:          strconv.Itoa(req.Type),
		}
	)

	resp, err := a.client.Request(ctx, url, &request, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipRewardGetAllReq struct {
	types.EApiReqCommon
}

type VipRewardGetAllResp struct {
	Code int `json:"code"`
	Data struct {
		Result bool `json:"result"`
	} `json:"data"`
	Message string `json:"message"`
}

// VipRewardGetAll 一键领取所有黑胶 VIP 成长值 (EAPI).
func (a *Api) VipRewardGetAll(ctx context.Context, req *VipRewardGetAllReq) (*VipRewardGetAllResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/level/task/reward/getall"
		reply VipRewardGetAllResp
		opts  = api.NewOptions("eapi.VipRewardGetAll").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipWelfareListReq struct {
	types.EApiReqCommon
}

type VipWelfareListResp struct {
	Code int `json:"code"`
	Data any `json:"data"`
}

// VipWelfareList 获取会员等级福利列表 (EAPI).
func (a *Api) VipWelfareList(ctx context.Context, req *VipWelfareListReq) (*VipWelfareListResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/level/welfare/new/list"
		reply VipWelfareListResp
		opts  = api.NewOptions("eapi.VipWelfareList").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipBenefitCategoryListReq struct {
	types.EApiReqCommon

	Category string `json:"category"`
}

type VipBenefitCategoryListResp struct {
	Code int                      `json:"code"`
	Data []VipBenefitCategoryData `json:"data"`
}

type VipBenefitCategoryData struct {
	Id         int64  `json:"id"`
	Name       string `json:"name"`
	BenefitGet bool   `json:"benefitGet"`
}

// VipBenefitCategoryList 获取分类下免费福利券列表.
func (a *Api) VipBenefitCategoryList(ctx context.Context, req *VipBenefitCategoryListReq) (*VipBenefitCategoryListResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipnewcenter/app/benefitcenter/benefits/category/list"
		reply VipBenefitCategoryListResp
		opts  = api.NewOptions("eapi.VipBenefitCategoryList").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type VipBenefitGetReq struct {
	types.EApiReqCommon

	Id string `json:"id"`
}

type VipBenefitGetResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		BenefitGet bool `json:"benefitGet"`
	} `json:"result"`
}

// VipBenefitGet 领取免费商家福利券.
func (a *Api) VipBenefitGet(ctx context.Context, req *VipBenefitGetReq) (*VipBenefitGetResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipcenter/benefits/get"
		reply VipBenefitGetResp
		opts  = api.NewOptions("eapi.VipBenefitGet").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

type TrialsongListenReq struct {
	types.EApiReqCommon

	SongId  string `json:"songId"`
	AlbumId string `json:"albumId"`
	Scene   int    `json:"scene"`
}

type TrialsongListenResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    bool   `json:"data"`
}

// TrialsongListen 上报听歌状态（黑胶/小众歌曲打卡）.
func (a *Api) TrialsongListen(ctx context.Context, req *TrialsongListenReq) (*TrialsongListenResp, error) {
	var (
		url   = "https://interface3.music.163.com/eapi/vipmall/interest/trialsong/listen"
		reply TrialsongListenResp
		opts  = api.NewOptions("eapi.TrialsongListen").SetEAPI()
	)

	resp, err := a.client.Request(ctx, url, req, &reply, opts)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	_ = resp
	return &reply, nil
}

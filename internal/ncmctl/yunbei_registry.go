// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"time"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
)

const (
	yunBeiCleanupTimeout       = 60 * time.Second
	yunBeiObserveInterval      = time.Second
	yunBeiVIPCenterViewTime    = 10 * time.Second
	yunBeiIndieListenTime      = 30 * time.Second
	yunBeiIndieAudioFetchLimit = 512 << 10
	yunBeiIndieBatchSize       = 4
	yunBeiIndieDailyLimit      = 10
	yunBeiArtistPageSize       = 1000
	yunBeiArtistMaxPages       = 100
	yunBeiMaxRounds            = 3
	yunBeiObserveReads         = 3
	yunBeiNoteObserveInterval  = 15 * time.Second
	yunBeiNoteObserveReads     = 7
)

type yunBeiTaskID string

const (
	yunBeiSign               yunBeiTaskID = "sign"
	yunBeiClaim              yunBeiTaskID = "claim"
	yunBeiReserve            yunBeiTaskID = "reserve"
	yunBeiViewVIPCenter      yunBeiTaskID = "view-vip-center"
	yunBeiLikeComment        yunBeiTaskID = "like-comment"
	yunBeiListenIndie        yunBeiTaskID = "listen-indie"
	yunBeiFollowArtist       yunBeiTaskID = "follow-artist"
	yunBeiCollectSong        yunBeiTaskID = "collect-song"
	yunBeiLikeSong           yunBeiTaskID = "like-song"
	yunBeiPublishNote        yunBeiTaskID = "publish-note"
	yunBeiShareSong          yunBeiTaskID = "share-song"
	yunBeiPlayDailyRecommend yunBeiTaskID = "play-daily-recommend"
)

const (
	yunBeiServerTaskPlayDailyRecommend int64 = 3801300
	yunBeiServerTaskListenIndie        int64 = 6718451
	yunBeiServerTaskFollowArtist       int64 = 3809301
	yunBeiServerTaskViewVIPCenter      int64 = 6758460
	yunBeiServerTaskPublishNote        int64 = 6566461
	yunBeiServerTaskLikeSong           int64 = 3804300
)

type yunBeiTaskState string

const (
	yunBeiUnavailable        yunBeiTaskState = "unavailable"
	yunBeiPending            yunBeiTaskState = "pending"
	yunBeiRunning            yunBeiTaskState = "running"
	yunBeiCompletedUnclaimed yunBeiTaskState = "completed-unclaimed"
	yunBeiClaimed            yunBeiTaskState = "claimed"
	yunBeiUnconfirmed        yunBeiTaskState = "unconfirmed"
	yunBeiFailed             yunBeiTaskState = "failed"
	yunBeiUnsupported        yunBeiTaskState = "unsupported"
)

type yunBeiTaskSnapshot struct {
	ID          yunBeiTaskID
	MissionCode string
	TaskID      int64
	UserTaskID  int64
	Name        string
	Description string
	Current     int64
	Target      int64
	Reward      int64
	State       yunBeiTaskState
	Conflicted  bool
	Claim       *yunBeiClaimRef
	Action      *yunBeiActionRef
}

type yunBeiSnapshot struct {
	Tasks          []*yunBeiTaskSnapshot
	UnmatchedTodos []eapi.YunBeiTaskTodoRespData
	Conflicts      []error
}

type yunBeiClaimRef struct {
	Period      int64
	UserTaskID  int64
	DepositCode int64
}

type yunBeiActionRef struct {
	TaskID     int64
	SubAction  string
	Type       string
	CheckToken string
}

type yunBeiStage uint8

const (
	stageAccount yunBeiStage = iota
	stageFast
	stageSlow
	stageClaim
)

type yunBeiTaskDef struct {
	ID           yunBeiTaskID
	Stage        yunBeiStage
	MissionCodes []string
	TaskIDs      []int64
	SideEffects  string
	EvidenceGap  string
	Run          func(context.Context, *yunBeiRuntime, *yunBeiTaskSnapshot) error
}

// yunBeiRegistry 的 TaskIDs 与 MissionCodes 只允许写入 2026-09-05 只读 V3 响应取证到的身份。
// MissionCodes 目前整体为空: spec-004 的门禁 G1 与开放问题 Q4 仍在等待 like-comment 和
// collect-song 的 V3/todo 配对样本。两类任务因此没有稳定身份映射, 在 correlateYunBeiTasks 里
// 拿不到本地 ID, 每轮都被判为 unavailable 并在处理器之前安全跳过——这正是 spec-004「无法稳定
// 关联的任务必须只展示并安全跳过」要求的行为。它们的处理器是待取证的实现, G1 关闭前不得视为
// 已生效; 注意 spec-004 G6 同时要求这两类「不能注册写路由」, 与当前仍作为 --task 可选值注册
// 的形态存在差异, 收紧对外文案前需先确认。
var yunBeiRegistry = []yunBeiTaskDef{
	{ID: yunBeiSign, Stage: stageAccount, SideEffects: "签到并领取已确认可领的连签奖励", Run: runYunBeiSign},
	{ID: yunBeiReserve, Stage: stageAccount, SideEffects: "预约活动并领取预约奖励", Run: runYunBeiReserve},
	{ID: yunBeiViewVIPCenter, Stage: stageFast, TaskIDs: []int64{yunBeiServerTaskViewVIPCenter}, SideEffects: "上报会员中心浏览动作", Run: runYunBeiViewVIPCenter},
	{ID: yunBeiLikeComment, Stage: stageFast, SideEffects: "点赞评论并恢复原状态", Run: runYunBeiLikeComment},
	{ID: yunBeiFollowArtist, Stage: stageFast, TaskIDs: []int64{yunBeiServerTaskFollowArtist}, SideEffects: "关注歌手并恢复原状态", Run: runYunBeiFollowArtist},
	{ID: yunBeiCollectSong, Stage: stageFast, SideEffects: "收藏歌曲并恢复原状态", Run: runYunBeiCollectSong},
	{ID: yunBeiLikeSong, Stage: stageFast, TaskIDs: []int64{yunBeiServerTaskLikeSong}, SideEffects: "红心歌曲并恢复原状态", Run: runYunBeiLikeSong},
	{ID: yunBeiPublishNote, Stage: stageFast, TaskIDs: []int64{yunBeiServerTaskPublishNote}, SideEffects: "公开发布图文动态", Run: runYunBeiPublishNote},
	{ID: yunBeiShareSong, Stage: stageFast, SideEffects: "触发站外分享", Run: unsupportedYunBeiTask("G1/G8")},
	{ID: yunBeiListenIndie, Stage: stageSlow, TaskIDs: []int64{yunBeiServerTaskListenIndie}, SideEffects: "播放并上报小众歌曲", Run: runYunBeiListenIndie},
	{
		ID: yunBeiPlayDailyRecommend, Stage: stageSlow, TaskIDs: []int64{yunBeiServerTaskPlayDailyRecommend},
		SideEffects: "长时间播放并上报每日推荐",
		// 2026-11-03 实测: startplay/play 经 /weapi/feedback/weblog 返回 code=200 data=success,
		// 但 taskId 3801300 整轮 27 次 V3 读取恒为 0/1800 (上报后 +189 秒仍未计入), 而同轮走云贝
		// 专属上报路由的任务都在亚秒级计入。计数来源未取证, 见 spec-004 Q7/G9。
		EvidenceGap: "播放上报不计数，日推秒数的计数来源未取证（spec-004 Q7）",
		Run:         runYunBeiPlayDailyRecommend,
	},
	{ID: yunBeiClaim, Stage: stageClaim, SideEffects: "领取已完成任务的云贝奖励", Run: runYunBeiClaim},
}

type yunBeiTaskIdentityIndex struct {
	missionCodes map[string]yunBeiTaskID
	taskIDs      map[int64]yunBeiTaskID
}

func yunBeiTaskIdentities() yunBeiTaskIdentityIndex {
	return yunBeiTaskIdentityIndex{
		missionCodes: map[string]yunBeiTaskID{},
		taskIDs: map[int64]yunBeiTaskID{
			yunBeiServerTaskPlayDailyRecommend: yunBeiPlayDailyRecommend,
			yunBeiServerTaskListenIndie:        yunBeiListenIndie,
			yunBeiServerTaskFollowArtist:       yunBeiFollowArtist,
			yunBeiServerTaskViewVIPCenter:      yunBeiViewVIPCenter,
			yunBeiServerTaskPublishNote:        yunBeiPublishNote,
			yunBeiServerTaskLikeSong:           yunBeiLikeSong,
		},
	}
}

type yunBeiResult struct {
	ID    yunBeiTaskID
	State yunBeiTaskState
	Err   error
}

// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

const (
	defaultFansGroupID       = "1872529203038486609" // 音乐合伙人的乐迷团 (PRD 待确认项 11.1 Q8, 帮助文本明示)
	audioFetchLimit    int64 = 512 << 10             // 单次音频拉取上限 512 KiB (PRD FR-04)
	fansAvatarLimit    int64 = 20 << 20              // 头像下载上限 20 MiB, 对齐 share 命令封面约定
)

// FansGroupOpts fansgroup 命令选项。
type FansGroupOpts struct {
	GroupID []string // 乐迷团 ID 列表; 默认 defaultFansGroupID
	Title   string   // 笔记标题覆盖
	Message string   // 笔记正文覆盖
	Image   string   // 本地图片路径; 空时下载乐迷团头像
	Delete  bool     // 任务循环完成后延时删除本次动态
}

// FansGroup 乐迷团任务命令。
type FansGroup struct {
	root   *Root
	cmd    *cobra.Command
	opts   FansGroupOpts
	l      *log.Logger
	uid    int64 // GetUserInfo 校验后的当前用户 ID, 点赞过滤剔除本人帖子
	status bool  // 位置参数解析结果
}

// task 调度器通过 scheduledCommand 接口注册本命令 (SPEC 7)。
var _ scheduledCommand = (*FansGroup)(nil)

func NewFansGroup(root *Root, l *log.Logger) *FansGroup {
	c := &FansGroup{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:   "fansgroup [status]",
			Short: "Run fans group (乐迷团) daily missions",
			Long: "Query and run NetEase Cloud Music fans group daily missions: play songs, share songs, " +
				"like fan notes, publish image-text notes and the daily speed-up task. " +
				"This changes account state (play logs, liked songs, likes and public notes); published notes are kept by default. " +
				"Use 'fansgroup status' for a read-only view. Random waits are applied between actions; " +
				"automating this command carries account risk and the usage frequency is up to you.",
			Example: "  # 查看乐迷团任务状态(只读)\n" +
				"  ncmctl fansgroup status\n\n" +
				"  # 执行默认乐迷团的全部任务\n" +
				"  ncmctl fansgroup\n\n" +
				"  # 指定乐迷团, 任务完成后删除本次发布的笔记\n" +
				"  ncmctl fansgroup --group-id 1872529203038486609 --delete\n\n" +
				"  # 等价的短参数写法\n" +
				"  ncmctl fansgroup -g 1872529203038486609 -d",
			Args: func(cmd *cobra.Command, args []string) error {
				if len(args) > 1 {
					return fmt.Errorf("only one optional 'status' argument is allowed, got %v", args)
				}

				for _, a := range args {
					if a != "status" {
						return fmt.Errorf("unknown argument %q, expected 'status'", a)
					}
				}
				return nil
			},
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		c.status = len(args) > 0
		if c.status {
			return c.executeStatus(cmd.Context())
		}
		return c.execute(cmd.Context())
	}
	return c
}

func (c *FansGroup) Command() *cobra.Command {
	return c.cmd
}

func (c *FansGroup) addFlags() {
	f := c.cmd.Flags()
	f.StringSliceVarP(&c.opts.GroupID, "group-id", "g", []string{defaultFansGroupID}, "fans group IDs (digits only); comma-separated or repeated")
	f.StringVarP(&c.opts.Title, "title", "t", "", "note title override")
	f.StringVarP(&c.opts.Message, "message", "m", "", "note message override; at least 10 Unicode characters")
	f.StringVarP(&c.opts.Image, "image", "i", "", "local image file; empty downloads the fans group avatar")
	f.BoolVarP(&c.opts.Delete, "delete", "d", false, "delete notes published by this run after the mission loop")
}

func (c *FansGroup) validate() error {
	// --group-id 每个值必须为非空纯数字 (AC-006)
	for _, id := range c.opts.GroupID {
		if !isNumericString(id) {
			return fmt.Errorf("group-id must be a non-empty numeric string, got %q", id)
		}
	}

	if c.status {
		// status 为只读模式, 与写操作 flag 互斥 (AC-004); --group-id 兼容 (AC-005)
		var conflicts []string
		if c.opts.Delete {
			conflicts = append(conflicts, "--delete")
		}

		if c.opts.Title != "" {
			conflicts = append(conflicts, "--title")
		}

		if c.opts.Message != "" {
			conflicts = append(conflicts, "--message")
		}

		if c.opts.Image != "" {
			conflicts = append(conflicts, "--image")
		}

		if len(conflicts) > 0 {
			return fmt.Errorf("'status' is read-only and cannot be combined with %s", strings.Join(conflicts, ", "))
		}
		return nil
	}

	// 标题/正文/图片规则与 share 命令一致, 共用校验
	return validateNoteContent(c.opts.Title, c.opts.Message, c.opts.Image)
}

// load 创建 API 客户端并校验登录, 复用 share.go load() 模式。
func (c *FansGroup) load(ctx context.Context) (*api.Client, *weapi.Api, *eapi.Api, error) {
	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create API client: %w", err)
	}

	e := eapi.New(cli)
	w := weapi.New(cli)

	u, err := w.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		_ = cli.Close(ctx)
		return nil, nil, nil, fmt.Errorf("get user info: %w", err)
	}

	if u.Code != 200 || u.Profile == nil || u.Profile.UserId <= 0 {
		_ = cli.Close(ctx)
		return nil, nil, nil, errors.New("need login")
	}

	c.uid = u.Profile.UserId

	c.cmd.Printf("\n🎵 乐迷团\n\n")
	c.cmd.Printf("  账号: %s(UID %d)\n", u.Profile.Nickname, u.Profile.UserId)

	return cli, w, e, nil
}

// readGroup 读取单团的详情、加入状态与任务列表; 任一接口失败即视为团前置失败。
func (c *FansGroup) readGroup(ctx context.Context, e *eapi.Api, gid string) (*eapi.FansGroupDetailGetResp, *eapi.FansGroupUserGroupDetailGetResp, *eapi.FansGroupMissionAllResp, error) {
	detail, err := e.FansGroupDetailGet(ctx, &eapi.FansGroupDetailGetReq{GroupId: gid})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fans group detail: %w", err)
	}

	if detail.Code != 200 {
		return nil, nil, nil, fmt.Errorf("fans group detail: code=%d message=%s", detail.Code, detail.Message)
	}

	member, err := e.FansGroupUserGroupDetailGet(ctx, &eapi.FansGroupUserGroupDetailGetReq{GroupId: gid})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fans group member detail: %w", err)
	}

	if member.Code != 200 {
		return nil, nil, nil, fmt.Errorf("fans group member detail: code=%d message=%s", member.Code, member.Message)
	}

	missions, err := e.FansGroupMissionAll(ctx, &eapi.FansGroupMissionAllReq{FansGroupId: gid})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fans group mission all: %w", err)
	}

	if missions.Code != 200 {
		return nil, nil, nil, fmt.Errorf("fans group mission all: code=%d message=%s", missions.Code, missions.Message)
	}
	return detail, member, missions, nil
}

func (c *FansGroup) executeStatus(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return err
	}

	cli, _, e, err := c.load(ctx)
	if err != nil {
		return err
	}
	defer closeAPIClient(ctx, cli, c.l)

	// 只读流程: 逐团读取输出; 不等待、不执行、不上传、不发布、不点赞、不红心、不删除
	var errs []error

	for _, gid := range c.opts.GroupID {
		if err := c.statusGroup(ctx, e, gid); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	c.cmd.Printf("\n状态查看完成\n")
	return nil
}

// statusGroup 只读输出单团详情、加入状态与任务进度。
func (c *FansGroup) statusGroup(ctx context.Context, e *eapi.Api, gid string) error {
	detail, member, missions, err := c.readGroup(ctx, e, gid)
	if err != nil {
		return fmt.Errorf("fans group %s: %w", gid, err)
	}

	info := detail.Data.FansGroupInfo
	c.cmd.Printf("\n👥 乐迷团: %s(groupID %s)\n", info.FansGroupName, gid)
	c.cmd.Printf("  歌手: %s\n", info.ArtistName)

	m := member.Data.FansGroupMemberDetail
	if !m.Joined {
		c.cmd.Printf("  加入状态: 未加入\n")
		return nil
	}

	c.cmd.Printf("  加入状态: 已加入 等级%s(%s) 头衔进度%s\n", m.Level.Level, m.Level.FanTitle, m.Level.Segment)

	c.printMissions(missions)
	return nil
}

func (c *FansGroup) printMissions(missions *eapi.FansGroupMissionAllResp) {
	c.cmd.Printf("  任务列表:\n")

	for i := range missions.Data.Normal.Data {
		v := &missions.Data.Normal.Data[i]
		c.cmd.Printf("    %s: 状态=%s 进度=%d/%d 积分=%s\n", v.Title, v.Status, v.CurrentProgress, v.AllProgress, v.Integral)
	}

	if o := missions.Data.Originality.Data; o.Title != "" {
		c.cmd.Printf("    %s(%s): 状态=%s 进度=%d/%d 积分=%s\n", o.Title, o.Subtitle, o.Status, o.CurrentProgress, o.AllProgress, o.Integral)
	}

	c.cmd.Printf("  剩余积分: %d 今日亲密度上限: %d\n", missions.Data.RemainingIntegral, missions.Data.DailyMaxIntimacy)
}

func (c *FansGroup) execute(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return err
	}

	cli, w, e, err := c.load(ctx)
	if err != nil {
		return err
	}
	defer closeAPIClient(ctx, cli, c.l)

	// 逐团串行执行, 单团失败隔离 (AC-008)
	var groupErrs []error

	for i, gid := range c.opts.GroupID {
		if i > 0 {
			// 乐迷团之间 3~10s (D5)
			if err := utils.Sleep(ctx, 3*time.Second, 10*time.Second); err != nil {
				return err
			}
		}

		if err := c.runGroup(ctx, e, w, gid); err != nil {
			// ctx 取消/超时不是团失败: 立即整体退出, 不再进入下一团 (AC-031)
			if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}

			c.cmd.Printf("  [失败] %v\n", err)
			groupErrs = append(groupErrs, err)
		}
	}

	if len(groupErrs) > 0 {
		return errors.Join(groupErrs...)
	}

	c.cmd.Printf("\n乐迷团任务完成\n")
	return nil
}

// runGroup 单团编排: 详情 → 加入状态 → 任务列表 → 分发循环 → 加速任务 → 最终进度 → 可选删除 → 聚合。
// 返回 error 即该团失败 (前置接口失败或全部任务失败), 由上层继续下一团 (AC-008)。
func (c *FansGroup) runGroup(ctx context.Context, e *eapi.Api, w *weapi.Api, gid string) error {
	detail, member, missions, err := c.readGroup(ctx, e, gid)
	if err != nil {
		return fmt.Errorf("fans group %s: %w", gid, err)
	}

	// todo: 后续考虑自动加入相应的乐迷团
	if !member.Data.FansGroupMemberDetail.Joined {
		c.cmd.Printf("\n👥 乐迷团 %s(groupID %s): 未加入,跳过任务执行\n", detail.Data.FansGroupInfo.FansGroupName, gid)
		return nil // 未加入不判失败 (D3)
	}

	var (
		info = detail.Data.FansGroupInfo
		rt   = &fansGroupRuntime{
			groupID:   gid,
			boardID:   info.BoardId,
			groupName: info.FansGroupName,
			avatarURL: info.HeadAvatarUrl,
		}
	)

	c.cmd.Printf("\n👥 乐迷团: %s(groupID %s)\n", rt.groupName, gid)
	c.printMissions(missions)

	// 执行普通任务
	for i := range missions.Data.Normal.Data {
		m := &missions.Data.Normal.Data[i]
		if missionCompleted(m.Status, m.CurrentProgress, m.AllProgress) {
			c.cmd.Printf("  [%s] 已完成(%d/%d),跳过\n", m.Title, m.CurrentProgress, m.AllProgress)
			rt.results = append(rt.results, taskResult{Title: m.Title, Status: taskSkipped})
			continue
		}

		if sleepErr := utils.Sleep(ctx, 2*time.Second, 5*time.Second); sleepErr != nil {
			return sleepErr
		}

		var (
			result      taskResult
			dispatchErr error
		)

		switch {
		case strings.Contains(m.Title, "播放"):
			result, dispatchErr = c.runPlayMission(ctx, w, m, rt)
		case strings.Contains(m.Title, "分享"):
			result, dispatchErr = c.runShareMission(ctx, e, m)
		case strings.Contains(m.Title, "点赞"):
			result, dispatchErr = c.runLikeMission(ctx, e, m, rt)
		case strings.Contains(m.Title, "笔记"), strings.Contains(m.Title, "发布"):
			result, dispatchErr = c.runNoteMission(ctx, e, m, rt)
		default:
			c.cmd.Printf("  [%s] 未知任务类型,跳过\n", m.Title)
			result = taskResult{Title: m.Title, Status: taskSkipped}
		}

		if dispatchErr != nil {
			return dispatchErr // context 取消等致命错误, 尽快退出 (AC-031)
		}

		rt.results = append(rt.results, result)
	}

	// 执行加速任务，无任务或已完成时静默跳过
	// 注意: 加速任务存在随机性，存在评论、点赞、红心等，目前只实现了红心。
	if o := missions.Data.Originality.Data; o.Title != "" && !missionCompleted(o.Status, o.CurrentProgress, o.AllProgress) {
		result, speedErr := c.runSpeedUpMission(ctx, e, &o, rt)
		if speedErr != nil {
			return speedErr
		}

		rt.results = append(rt.results, result)
	}

	// 最终进度回显 (5.1.8): 尽力而为, 进度可能异步更新, 不作为成功判定
	c.printFinalProgress(ctx, e, gid)

	// 可选删除 (5.1.8, D2 时序: 发布循环全部完成后统一延时删除)
	if c.opts.Delete {
		if deleteErr := c.deleteNotes(ctx, e, rt); deleteErr != nil {
			return deleteErr
		}
	}

	// 输出团内各任务结果并按 D3 规则聚合: 至少执行过一个任务且全部 failed 才判团失败
	// (空集/全 skipped 不判失败)。
	if len(rt.results) == 0 {
		c.cmd.Printf("\n  本轮无待执行任务\n")
		return nil
	}

	c.cmd.Printf("\n  任务结果:\n")

	for _, r := range rt.results {
		c.cmd.Printf("    %s: %s\n", r.Title, r.Status)
	}

	// D3: 至少执行过一个任务且最终结果全部 failed 才判团失败 (空集/全 skipped 已在上面提前返回)
	if !slices.ContainsFunc(rt.results, func(r taskResult) bool { return r.Status != taskFailed }) {
		return fmt.Errorf("fans group %s: all %d missions failed", rt.groupID, len(rt.results))
	}
	return nil
}

// runPlayMission 播放歌曲任务 (5.1.3), 经 weapi 上报播放日志故不依赖 eapi 客户端。
func (c *FansGroup) runPlayMission(ctx context.Context, w *weapi.Api, m *eapi.FansGroupMissionAllRespDataNormalData, rt *fansGroupRuntime) (taskResult, error) {
	params, err := parseMissionParams(m.Button.Url, m.IconUi.TargetUrl)
	if err != nil {
		c.cmd.Printf("  [%s] 任务参数解析失败: %v\n", m.Title, err)
		return taskResult{Title: m.Title, Status: taskFailed}, nil
	}

	songIDs := mergeSongIDs(params)
	if len(songIDs) == 0 {
		c.cmd.Printf("  [%s] 任务参数中无可用歌曲ID\n", m.Title)
		return taskResult{Title: m.Title, Status: taskFailed}, nil
	}

	rt.songIDs = append(rt.songIDs, songIDs...) // 供加速任务回退 (5.1.7)

	playIDs := toInt64SongIDs(songIDs)
	if len(playIDs) == 0 {
		c.cmd.Printf("  [%s] 任务参数中无可用的数字歌曲ID\n", m.Title)
		return taskResult{Title: m.Title, Status: taskFailed}, nil
	}

	remaining := missionRemaining(m.CurrentProgress, m.AllProgress)

	// 播放迭代之间 2~5s (D5)
	return runIterations(ctx, m.Title, remaining, 2*time.Second, 5*time.Second, func(int, int) (bool, error) {
		return c.playOnce(ctx, w, playIDs[rand.Int64N(int64(len(playIDs)))]), nil
	})
}

// playOnce 执行单次播放链路: 歌曲详情(可选) → startplay 上报 → 获取播放地址 →
// 限量拉取音频 → play 上报。链路内部步骤之间不插入 sleep (FR-04)。
func (c *FansGroup) playOnce(ctx context.Context, w *weapi.Api, songID int64) bool {
	// 获取歌名/时长/专辑信息, 属可选步骤: 失败仅记录, 不中断播放链路 (5.1.3)
	var song playSong

	resp, derr := w.SongDetail(ctx, &weapi.SongDetailReq{C: []weapi.SongDetailReqList{{Id: strconv.FormatInt(songID, 10)}}})

	switch {
	case derr != nil:
		c.l.Debugf("[fansgroup] song detail %d: %v", songID, derr)
	case resp.Code != 200 || len(resp.Songs) == 0:
		c.l.Debugf("[fansgroup] song detail %d: code=%d songs=%d", songID, resp.Code, len(resp.Songs))
	default:
		v := resp.Songs[0]
		song.name = v.Name
		song.duration = v.Dt / 1000
		song.albumID = v.Al.Id
	}

	// startplay 载荷对齐 api/weapi/feedback.go 样本注释 (4.3): id 为数字, content/mainsite 为字符串
	if err := c.reportLog(ctx, w, []map[string]any{{
		"action": "startplay",
		"json": map[string]any{
			"id":       songID,
			"type":     "song",
			"content":  "id=" + strconv.FormatInt(songID, 10),
			"mainsite": "1",
		},
	}}); err != nil {
		c.cmd.Printf("    歌曲 %d(%s): startplay 上报失败: %v\n", songID, song.name, err)
		return false
	}

	player, err := w.SongPlayerV1(ctx, &weapi.SongPlayerV1Req{
		Ids:   types.IntsString{songID}, // 单曲, 不传 _uid 后缀
		Level: types.LevelStandard,      // 标准品质 128000
	})
	if err != nil {
		c.cmd.Printf("    歌曲 %d(%s): 获取播放地址失败: %v\n", songID, song.name, err)
		return false
	}

	if player.Code != 200 {
		c.cmd.Printf("    歌曲 %d(%s): 获取播放地址失败: code=%d\n", songID, song.name, player.Code)
		return false
	}

	if len(player.Data) == 0 {
		c.cmd.Printf("    歌曲 %d(%s): 播放地址响应为空\n", songID, song.name)
		return false
	}
	// Data[0].Code 与外层业务 Code 分别判定: 404 表示歌曲下架变灰 (5.4)
	if d := player.Data[0]; d.Code != 200 || d.Url == "" {
		c.cmd.Printf("    歌曲 %d(%s): 无可用播放地址(code=%d)\n", songID, song.name, d.Code)
		return false
	}

	if err := fetchAudioSample(ctx, player.Data[0].Url, audioFetchLimit, c.root.Cfg.Network.Timeout); err != nil {
		c.cmd.Printf("    歌曲 %d(%s): 拉取音频失败: %v\n", songID, song.name, err)
		return false
	}

	// 上报时长 3~5 秒随机且不超过歌曲时长; 与实际拉取解耦 (FR-04)
	seconds := 3 + rand.Int64N(3)
	if song.duration > 0 && seconds > song.duration {
		seconds = song.duration
	}

	// 播放来源: 有专辑用 album/albumID, 否则按样本回退 toplist (11.1 Q4 待验证)
	source, sourceID := "toplist", ""
	if song.albumID > 0 {
		source, sourceID = "album", strconv.FormatInt(song.albumID, 10)
	}

	// 播放结束载荷 (4.3/11.1 Q4): id/time/wifi/download 为数字, sourceId/content 为字符串;
	// end=interrupt 表示播放中途切歌。
	if err := c.reportLog(ctx, w, []map[string]any{{
		"action": "play",
		"json": map[string]any{
			"type":     "song",
			"wifi":     0,
			"download": 0,
			"id":       songID,
			"time":     seconds,
			"end":      "interrupt",
			"source":   source,
			"sourceId": sourceID,
			"mainsite": "1",
			"content":  "id=" + strconv.FormatInt(songID, 10),
		},
	}}); err != nil {
		c.cmd.Printf("    歌曲 %d(%s): play 上报失败: %v\n", songID, song.name, err)
		return false
	}

	c.cmd.Printf("    歌曲 %s(%d): 播放上报完成(%d秒,来源%s)\n", song.name, songID, seconds, source)
	return true
}

func (c *FansGroup) reportLog(ctx context.Context, w *weapi.Api, logs []map[string]any) error {
	resp, err := w.WebLog(ctx, &weapi.WebLogReq{Logs: logs})
	if err != nil {
		return fmt.Errorf("weblog: %w", err)
	}

	if resp.Code != 200 {
		return fmt.Errorf("weblog: code=%d message=%s", resp.Code, resp.Message)
	}
	return nil
}

// runShareMission 分享歌曲任务 (5.1.4): 仅做分享进度上报。
func (c *FansGroup) runShareMission(ctx context.Context, e *eapi.Api, m *eapi.FansGroupMissionAllRespDataNormalData) (taskResult, error) {
	params, err := parseMissionParams(m.Button.Url, "")
	if err != nil {
		c.cmd.Printf("  [%s] 任务参数解析失败: %v\n", m.Title, err)
		return taskResult{Title: m.Title, Status: taskFailed}, nil
	}

	// 不猜测资源 ID (D8): 参数缺失直接失败
	resourceID := string(params.ActionCustomParams.ProgressParams.ResourceID)
	if resourceID == "" {
		c.cmd.Printf("  [%s] 任务参数中无 resourceId\n", m.Title)
		return taskResult{Title: m.Title, Status: taskFailed}, nil
	}

	resourceType := string(params.ActionCustomParams.ProgressParams.ResourceType)
	if resourceType == "" {
		resourceType = "4" // 缺省按歌曲 (5.1.4)
	}

	remaining := missionRemaining(m.CurrentProgress, m.AllProgress)

	// 分享迭代之间 2~5s (D5)
	return runIterations(ctx, m.Title, remaining, 2*time.Second, 5*time.Second, func(_, ok int) (bool, error) {
		// Action/FansGroupId 由 wrapper 默认 (share / null)
		resp, reportErr := e.FansGroupMissionForwardProgress(ctx, &eapi.FansGroupMissionForwardProgressReq{
			ResourceId:   resourceID,
			ResourceType: resourceType,
		})
		if reportErr != nil {
			c.cmd.Printf("    分享上报失败: %v\n", reportErr)
			return false, nil
		}

		if resp.Code != 200 {
			c.cmd.Printf("    分享上报失败: code=%d message=%s\n", resp.Code, resp.Message)
			return false, nil
		}

		c.cmd.Printf("    分享上报成功(%d/%d)\n", ok+1, remaining)
		return true, nil
	})
}

// runLikeMission 点赞乐迷笔记任务 (5.1.5), 前置获取推荐 Feed 作为点赞候选。
func (c *FansGroup) runLikeMission(ctx context.Context, e *eapi.Api, m *eapi.FansGroupMissionAllRespDataNormalData, rt *fansGroupRuntime) (taskResult, error) {
	remaining := missionRemaining(m.CurrentProgress, m.AllProgress)

	// likeFeedRequest 保证 FansGroupId 显式传入: wrapper 对该字段无默认值,
	// 空串会拼进 URL, 导致帖子不按团过滤、点赞不被任务计数 (SPEC 5.1.5, P1 防回归)
	feed, err := e.FansGroupFeedRecommend(ctx, &eapi.FansGroupFeedRecommendReq{
		FansGroupId: rt.groupID,
		Size:        strconv.Itoa(remaining + 5), // 覆盖剩余次数并留余量
		Cursor:      "0",
		ArtistSelf:  "0",
	})
	if err != nil {
		c.cmd.Printf("  [%s] 获取推荐Feed失败: %v\n", m.Title, err)
		return taskResult{Title: m.Title, Status: taskFailed}, nil
	}

	if feed.Code != 200 {
		c.cmd.Printf("  [%s] 获取推荐Feed失败: code=%d message=%s\n", m.Title, feed.Code, feed.Message)
		return taskResult{Title: m.Title, Status: taskFailed}, nil
	}

	// 过滤可点赞帖子: threadId 非空、未点赞、非本人帖子 (5.1.5)
	posts := make([]eapi.FansGroupFeedRecommendRespDataRecords, 0)

	for i := range feed.Data.Records {
		p := &feed.Data.Records[i]
		if p.ThreadId == "" || p.Info.Liked || p.User.UserId == c.uid {
			continue
		}

		posts = append(posts, *p)
	}

	if len(posts) == 0 {
		c.cmd.Printf("  [%s] 无可点赞帖子\n", m.Title)
		return taskResult{Title: m.Title, Status: taskSkipped}, nil
	}

	// 可用帖子少于剩余次数是合法完成场景 (FR-06): 点完全部可用帖子即视为完成
	n := min(remaining, len(posts))

	// 点赞迭代之间 1~3s (D5)
	result, err := runIterations(ctx, m.Title, n, 1*time.Second, 3*time.Second, func(round, ok int) (bool, error) {
		resp, likeErr := e.ResourceLike(ctx, &eapi.ResourceLikeReq{
			ThreadId: posts[round].ThreadId,
			// appLogExt 构造点赞日志扩展字段, 携带乐迷团归属标记。
			// addRefer/multiRefer 指向当前乐迷团 ID 的完整 JSON 结构待 Phase 1 验证 (SPEC 11.1 Q3),
			// 若点赞不被任务计数仅需调整本函数。
			AppLogExt: fmt.Sprintf(`{"addRefer":{"resourceId":%q,"resourceType":"E"},"multiRefer":[],"fansGroupId":%q}`, rt.groupID, rt.groupID),
		})
		if likeErr != nil {
			c.cmd.Printf("    点赞帖子失败: %v\n", likeErr)
			return false, nil
		}

		if resp.Code != 200 {
			c.cmd.Printf("    点赞帖子失败: code=%d message=%s\n", resp.Code, resp.Message)
			return false, nil
		}

		c.cmd.Printf("    点赞成功(%d/%d)\n", ok+1, n)
		return true, nil
	})
	if err != nil {
		return taskResult{}, err
	}

	if result.Status == taskDone && len(posts) < remaining {
		c.cmd.Printf("    可用帖子(%d)少于剩余次数(%d),已全部点赞\n", len(posts), remaining)
	}
	return result, nil
}

// runNoteMission 发布图文笔记任务 (5.1.6)。
func (c *FansGroup) runNoteMission(ctx context.Context, e *eapi.Api, m *eapi.FansGroupMissionAllRespDataNormalData, rt *fansGroupRuntime) (taskResult, error) {
	// --image 非空时直接使用本地文件(校验已在 validate 完成); 否则下载乐迷团头像 (5.1.6)
	var (
		image   string
		cleanup func()
		prepErr error
	)

	switch {
	case c.opts.Image != "":
		image, cleanup = c.opts.Image, func() {}
	case rt.avatarURL == "":
		prepErr = errors.New("fans group has no avatar URL; specify --image")
	default:
		image, cleanup, prepErr = downloadImageToTemp(ctx, c.root.Cfg.Network.Timeout, rt.avatarURL, fansAvatarLimit, "ncmctl-fansgroup-*.img")
		if prepErr != nil {
			prepErr = fmt.Errorf("download avatar: %w", prepErr)
		}
	}

	if prepErr != nil {
		c.cmd.Printf("  [%s] 准备图片失败: %v\n", m.Title, prepErr)
		return taskResult{Title: m.Title, Status: taskFailed}, nil
	}

	defer cleanup()

	pics, err := e.EventUploadImage(ctx, image)
	if err != nil {
		c.cmd.Printf("  [%s] 上传图片失败: %v\n", m.Title, err)
		return taskResult{Title: m.Title, Status: taskFailed}, nil
	}

	// activityInfoList 的 id 来自详情接口 boardId, 不硬编码 (5.1.6)
	activityInfo, err := json.Marshal([]noteActivity{{
		ID:        rt.boardID,
		Type:      3,
		SubType:   11,
		Name:      rt.groupName,
		Selected:  true,
		CanChange: true,
	}})
	if err != nil {
		c.cmd.Printf("  [%s] 构造活动信息失败: %v\n", m.Title, err)
		return taskResult{Title: m.Title, Status: taskFailed}, nil
	}

	remaining := missionRemaining(m.CurrentProgress, m.AllProgress)

	// 发布失败时不删除、不重发 (AC-019)
	// 发布迭代之间 2~5s (D5)
	// 每次发布都重新生成文案: 内置默认正文带随机编号, 避免同轮多次发布内容相同被服务端去重 (5.1.6)。
	return runIterations(ctx, m.Title, remaining, 2*time.Second, 5*time.Second, func(_, ok int) (bool, error) {
		title, message := c.noteText(rt.groupName)

		resp, publishErr := e.EventPublish(ctx, &eapi.EventPublishReq{
			Type:             "noresource",
			Title:            title,
			Msg:              message,
			Pics:             pics,
			ActivityInfoList: string(activityInfo),
		})
		if publishErr != nil {
			c.cmd.Printf("    发布笔记失败: %v\n", publishErr)
			return false, nil
		}

		if resp.Code != 200 {
			c.cmd.Printf("    发布笔记失败: code=%d message=%s\n", resp.Code, resp.Message)
			return false, nil
		}

		if resp.Id > 0 {
			// 服务端去重检测: 同一动态ID已在本次执行链内出现过, 说明本次发布被服务端
			// 合并到已有动态、未产生新进度。不计入 eventIDs(避免 --delete 重复删除),
			// 本轮按失败计使最终状态如实反映为 partial, 并提示差异化内容。
			if isDuplicateEventID(rt, resp.Id) {
				c.cmd.Printf("    发布被服务端去重(动态ID %d 重复), 本次内容未产生新进度; 若使用 --message 固定文案请改为差异化内容\n", resp.Id)
				return false, nil
			}

			c.cmd.Printf("    发布笔记成功: 动态ID %d(%d/%d)\n", resp.Id, ok+1, remaining)
			rt.eventIDs = append(rt.eventIDs, resp.Id) // 仅供 --delete 使用
		} else {
			c.cmd.Printf("    发布笔记成功但无有效动态ID(%d/%d)\n", ok+1, remaining)
		}
		return true, nil
	})
}

// isDuplicateEventID 判断动态ID是否已在本次执行链内发布过 (服务端内容去重的标志)。
// 仅对 id>0 且已存在于 rt.eventIDs 中者返回 true; id<=0 属"成功但无ID"分支, 不算去重。
func isDuplicateEventID(rt *fansGroupRuntime, id int64) bool {
	return id > 0 && slices.Contains(rt.eventIDs, id)
}

// noteText 生成笔记标题与正文: --title/--message 优先 (校验已在 validate 完成);
// 默认正文带随机元素, 每次调用编号不同, 避免同轮多次发布及连续多日内容相同被服务端去重 (5.1.6)。
func (c *FansGroup) noteText(groupName string) (string, string) {
	title := strings.TrimSpace(c.opts.Title)
	if title == "" {
		if groupName != "" {
			title = groupName + " | 今日乐迷团打卡"
		} else {
			title = "今日乐迷团打卡"
		}
	}

	message := strings.TrimSpace(c.opts.Message)
	if message == "" {
		// 内置默认正文带随机编号, 避免连续多日内容相同 (5.1.6)
		message = fmt.Sprintf("乐迷团今日打卡完成,今日份的好音乐已就位(No.%d)。", rand.Int64N(1_000_000_000))
	}
	return title, message
}

// runSpeedUpMission 今日加速任务 (5.1.7): 通过红心/取消红心完成收藏类加速。
// 副标题无法识别时同样按红心流程处理并输出原始副标题 (FR-08)。
func (c *FansGroup) runSpeedUpMission(ctx context.Context, e *eapi.Api, m *eapi.FansGroupMissionAllRespDataNormalData, rt *fansGroupRuntime) (taskResult, error) {
	var (
		songIDs   = speedUpSongIDs(m, rt.songIDs)
		likeIDs   = toInt64SongIDs(songIDs)
		remaining = missionRemaining(m.CurrentProgress, m.AllProgress)
	)

	if len(likeIDs) == 0 {
		// 不使用硬编码歌曲 (AC-022)
		c.cmd.Printf("  [%s] 任务参数中无可用歌曲ID且无播放任务可回退,跳过\n", m.Title)
		return taskResult{Title: m.Title, Status: taskSkipped}, nil
	}

	if !strings.Contains(m.Subtitle, "收藏") && !strings.Contains(m.Subtitle, "红心") {
		c.cmd.Printf("  [%s] 未知副标题 %q,按红心流程处理\n", m.Title, m.Subtitle)
	}

	// 加速任务迭代之间 2~5s (D5)
	return runIterations(ctx, m.Title, remaining, 2*time.Second, 5*time.Second, func(int, int) (bool, error) {
		return c.speedUpOnce(ctx, e, likeIDs[rand.Int64N(int64(len(likeIDs)))])
	})
}

// speedUpOnce 单轮红心流程: 归一化为未红心 → 红心计数 → 恢复原状, 不在账号遗留红心 (AC-021)。
// 任一步失败该轮即失败; context 取消时返回 error 由上层尽快退出。
func (c *FansGroup) speedUpOnce(ctx context.Context, e *eapi.Api, songID int64) (bool, error) {
	if err := c.songLike(ctx, e, songID, false); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		c.cmd.Printf("    歌曲 %d: 归一化取消红心失败: %v\n", songID, err)
		return false, nil
	}

	// 归一化→红心 1~3s (D5)
	if err := utils.Sleep(ctx, 1*time.Second, 3*time.Second); err != nil {
		return false, err
	}

	if err := c.songLike(ctx, e, songID, true); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		c.cmd.Printf("    歌曲 %d: 红心失败: %v\n", songID, err)
		return false, nil
	}

	// 红心→取消恢复 3~10s (D5)
	if err := utils.Sleep(ctx, 3*time.Second, 10*time.Second); err != nil {
		return false, err
	}

	// AC-021 要求红心任务结束恢复原状: 此步失败会在账号遗留红心, 明确提示用户手工恢复。
	// 传输层已按 cfg.Retry 重试 (api/api.go), 应用层不再叠加重试。
	if err := c.songLike(ctx, e, songID, false); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		c.cmd.Printf("    歌曲 %d: 恢复取消红心失败: %v (账号将残留红心, 需手工取消)\n", songID, err)
		return false, nil
	}

	c.cmd.Printf("    歌曲 %d: 红心加速完成\n", songID)
	return true, nil
}

// songLike 红心/取消红心歌曲; SongLikeResp 仅有 Code 无 Message, 错误降级输出 code (2.5)。
func (c *FansGroup) songLike(ctx context.Context, e *eapi.Api, songID int64, like bool) error {
	resp, err := e.SongLike(ctx, &eapi.SongLikeReq{
		TrackId: strconv.FormatInt(songID, 10),
		Like:    strconv.FormatBool(like),
		Time:    "3",
	})
	if err != nil {
		return fmt.Errorf("song like: %w", err)
	}

	if resp.Code != 200 {
		return fmt.Errorf("song like: code=%d", resp.Code)
	}
	return nil
}

// deleteNotes 延时逐条删除本次执行链内发布成功的动态 (D2: 发布循环全部完成后统一删除)。
// 删除失败按部分成功处理: 输出 event ID 与原因, 不重发, 不改任务状态与退出码 (AC-019)。
func (c *FansGroup) deleteNotes(ctx context.Context, e *eapi.Api, rt *fansGroupRuntime) error {
	if len(rt.eventIDs) == 0 {
		return nil // 未发布新动态时不删除
	}

	for _, eventID := range rt.eventIDs {
		// 每条删除前 5~30s; 发布循环已全部完成 (D2 时序, D5)
		if err := utils.Sleep(ctx, 5*time.Second, 30*time.Second); err != nil {
			return err
		}

		resp, err := e.EventDelete(ctx, &eapi.EventDeleteReq{Id: eventID})
		if err != nil {
			c.cmd.Printf("  删除动态 %d 失败: %v\n", eventID, err)
			continue
		}

		if resp.Code != 200 {
			c.cmd.Printf("  删除动态 %d 失败: code=%d message=%s\n", eventID, resp.Code, resp.Message)
			continue
		}

		c.cmd.Printf("  删除动态: 已删除(动态ID %d)\n", eventID)
	}
	return nil
}

// printFinalProgress 尽力而为回显最终任务进度与剩余积分; 进度可能异步更新,
// 失败仅记录, 不作为成功判定 (5.1.8)。
func (c *FansGroup) printFinalProgress(ctx context.Context, e *eapi.Api, gid string) {
	final, err := e.FansGroupMissionAll(ctx, &eapi.FansGroupMissionAllReq{FansGroupId: gid})
	if err != nil {
		c.l.Warnf("[fansgroup] final progress: %v", err)
		return
	}

	if final.Code != 200 {
		c.l.Warnf("[fansgroup] final progress: code=%d message=%s", final.Code, final.Message)
		return
	}

	c.cmd.Printf("  最终进度(可能存在异步延迟):\n")

	for i := range final.Data.Normal.Data {
		v := &final.Data.Normal.Data[i]
		c.cmd.Printf("    %s: 状态=%s 进度=%d/%d\n", v.Title, v.Status, v.CurrentProgress, v.AllProgress)
	}

	c.cmd.Printf("    剩余积分: %d\n", final.Data.RemainingIntegral)
}

// fansGroupRuntime 保存单个乐迷团执行周期内的中间状态, 不持久化, 随命令结束丢弃 (3.2)。
type fansGroupRuntime struct {
	groupID   string   // 当前乐迷团 ID
	boardID   string   // 详情返回的 boardId, 发布笔记 activityInfoList 用
	groupName string   // 乐迷团名, activityInfoList.name 用
	avatarURL string   // 详情返回的头像 URL, 未指定 --image 时用
	songIDs   []string // normal 任务解析到的歌曲 ID, 加速任务回退用
	eventIDs  []int64  // 本次执行链内发布成功的动态 ID, --delete 用
	results   []taskResult
}

// taskResult 单个任务的最终结果, 团级聚合输入。
type taskResult struct {
	Title  string
	Status taskStatus // done / partial / skipped / failed
}

type taskStatus string

const (
	taskDone    taskStatus = "done"
	taskPartial taskStatus = "partial"
	taskSkipped taskStatus = "skipped"
	taskFailed  taskStatus = "failed"
)

// flexString 接收既可能是字符串也可能是数字的 JSON 值: 服务端在同一批参数里混用两种类型
// (实测分享任务 progressParams 的 resourceId 为字符串而 resourceType 为数字, songId 亦为数字)。
// null 视为空值; 对象与数组等非标量直接报错, 避免静默吞掉类型错误。
type flexString string

func (f *flexString) UnmarshalJSON(data []byte) error {
	switch {
	case len(data) == 0:
		return errors.New("flexString: empty JSON value")
	case string(data) == "null":
		*f = ""
	case data[0] == '"': // 字符串: 走标准解码以还原转义序列
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}

		*f = flexString(s)
	case data[0] == '-' || (data[0] >= '0' && data[0] <= '9'): // 数字: 按字面量保留
		*f = flexString(data)
	default:
		return fmt.Errorf("flexString: unsupported JSON value %s", data)
	}
	return nil
}

// missionButtonParams 是任务 button.url / iconUi.targetUrl 携带的参数 JSON 的结构化表示。
// 字段均为可选: 不同任务类型只填充其中的子集, 解析后按非空原则取值 (D8, 禁止正则兜底)。
// ID 与类型字段统一用 flexString, 兼容服务端字符串/数字混发。
type missionButtonParams struct {
	SongID   flexString   `json:"songId"`
	SongIDs  []flexString `json:"songIds"`
	TrackID  flexString   `json:"trackId"`
	TrackIDs []flexString `json:"trackIds"`

	// ActionMnbParams 是 mnb(客户端播放器动作) 任务参数内的嵌套结构。
	// 实测播放任务的 songIds 仅存在于此嵌套层 (button.url 形如
	// {"actionType":"mnb","actionMnbName":"nm.play.playSongs",
	//  "actionMnbParams":{"songIds":[...],"songIndex":0,...}}), 顶层 songId 为空。
	ActionMnbParams struct {
		SongID  flexString   `json:"songId"`
		SongIDs []flexString `json:"songIds"`
		// 服务端同层也可能携带 trackId(s), 一并解析以防后续任务复用同一结构。
		TrackID  flexString   `json:"trackId"`
		TrackIDs []flexString `json:"trackIds"`
	} `json:"actionMnbParams"`

	ActionCustomParams struct {
		ProgressParams struct {
			ResourceID   flexString `json:"resourceId"`
			ResourceType flexString `json:"resourceType"`
		} `json:"progressParams"`
	} `json:"actionCustomParams"`
}

// noteActivity 发布笔记 activityInfoList 的单条活动信息,
// 格式对齐 api/eapi/event.go EventPublishReq.ActivityInfoList 注释。
type noteActivity struct {
	ID        string `json:"id"`
	Type      int    `json:"type"`
	SubType   int    `json:"subType"`
	Name      string `json:"name"`
	Selected  bool   `json:"selected"`
	CanChange bool   `json:"canChange"`
}

// playSong 播放链路用到的歌曲概要信息。
type playSong struct {
	name     string
	duration int64 // 歌曲时长(秒), 0 表示未知
	albumID  int64
}

// missionCompleted 以服务端进度为唯一事实源判定任务是否已完成 (FR-02/FR-03),
// 客户端不推算自然日。
func missionCompleted(status string, current, all int) bool {
	return status == "COMPLETED" || (all > 0 && current >= all)
}

// missionRemaining 计算剩余执行次数; 服务端进度异常 (<=0) 时按 1 次处理 (PRD FR-03 规则 5)。
func missionRemaining(current, all int) int {
	if n := all - current; n > 0 {
		return n
	}
	return 1
}

// parseMissionParams 结构化解析任务参数 JSON (D8): primary 为空时回退 fallback
// (播放任务的 IconUi.TargetUrl); 失败即报错, 错误信息携带原文片段, 不做正则兜底。
func parseMissionParams(primary, fallback string) (*missionButtonParams, error) {
	raw := strings.TrimSpace(primary)
	if raw == "" {
		raw = strings.TrimSpace(fallback)
	}

	if raw == "" {
		return nil, errors.New("mission params is empty")
	}

	var p missionButtonParams
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, fmt.Errorf("unmarshal mission params: %w (raw: %.200s)", err, raw)
	}
	return &p, nil
}

// toInt64SongIDs 将数字字符串歌曲 ID 转为 int64, 跳过转换失败与非正数项
// (负数 ID 不应进入播放/红心链路)。
func toInt64SongIDs(ids []string) []int64 {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if v, err := strconv.ParseInt(id, 10, 64); err == nil && v > 0 {
			out = append(out, v)
		}
	}
	return out
}

// mergeSongIDs 非空合并任务参数中的歌曲 ID (5.1.3)。
// 合并顺序: 顶层 SongIDs→SongID→TrackIDs→TrackID → 嵌套(actionMnbParams) SongIDs→SongID→TrackIDs→TrackID。
// 顶层优先是因为分享/红心任务把 ID 放在顶层, 播放任务仅嵌套层有值、顶层为空;
// 二者互斥, 顺序不影响实际取值, 仅作稳定约定。
// null 与空值项直接丢弃, 避免空串流入播放与红心链路。
func mergeSongIDs(p *missionButtonParams) []string {
	capHint := 0
	for _, n := range [...]int{
		len(p.SongIDs),
		len(p.TrackIDs),
		len(p.ActionMnbParams.SongIDs),
		len(p.ActionMnbParams.TrackIDs),
		4,
	} {
		if n > math.MaxInt-capHint {
			capHint = math.MaxInt
			break
		}
		capHint += n
	}
	ids := make([]string, 0, capHint)

	appendIDs := func(list ...flexString) {
		for _, id := range list {
			if id != "" {
				ids = append(ids, string(id))
			}
		}
	}

	appendIDs(p.SongIDs...)
	appendIDs(p.SongID)
	appendIDs(p.TrackIDs...)
	appendIDs(p.TrackID)
	appendIDs(p.ActionMnbParams.SongIDs...)
	appendIDs(p.ActionMnbParams.SongID)
	appendIDs(p.ActionMnbParams.TrackIDs...)
	appendIDs(p.ActionMnbParams.TrackID)
	return ids
}

// songIDsFromJSON 解析 JSON 原文中的歌曲 ID; 解析失败或为空返回 nil。
func songIDsFromJSON(raw string) []string {
	p, err := parseMissionParams(raw, "")
	if err != nil {
		return nil
	}
	return mergeSongIDs(p)
}

// speedUpSongIDs 解析加速任务歌曲 ID: 依次尝试 Button.Url / LogInfo / MissionDetail 中的
// JSON (5.1.7); 单个来源解析失败按空处理并继续尝试, 全部为空时回退到 normal 播放任务
// 累积的歌曲 ID, 仍为空则跳过 (AC-022), 不使用硬编码歌曲。
func speedUpSongIDs(m *eapi.FansGroupMissionAllRespDataNormalData, fallback []string) []string {
	for _, raw := range []string{m.Button.Url, m.LogInfo} {
		if ids := songIDsFromJSON(raw); len(ids) > 0 {
			return ids
		}
	}

	// MissionDetail 是已反序列化的 any, 可能是 JSON 字符串或对象, 统一转 JSON 再解析
	switch v := m.MissionDetail.(type) {
	case string:
		if ids := songIDsFromJSON(v); len(ids) > 0 {
			return ids
		}
	case map[string]any:
		if data, err := json.Marshal(v); err == nil {
			if ids := songIDsFromJSON(string(data)); len(ids) > 0 {
				return ids
			}
		}
	}
	return fallback
}

// fetchAudioSample 以 Range 请求限量拉取音频数据模拟播放缓冲, 遵循 ctx 取消;
// 上报时长与实际拉取解耦 (FR-04)。
func fetchAudioSample(ctx context.Context, url string, limit int64, timeout time.Duration) error {
	// fail-fast: limit<=0 会构造非法 Range 头 (bytes=0--1)
	if limit <= 0 {
		return errors.New("audio sample limit must be positive")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return err
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", limit-1))

	client := &http.Client{Timeout: timeout}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode/100 != 2 {
		return fmt.Errorf("HTTP %s", res.Status)
	}

	_, err = io.Copy(io.Discard, io.LimitReader(res.Body, limit))
	return err
}

// runIterations 任务迭代统一脚手架 (5.1.2/5.3): 每轮先检查 ctx 取消, 轮间在
// [gapMin, gapMax] 内随机等待。action 返回本轮是否成功; 返回非 nil error 表示
// 致命错误 (如 ctx 取消), 立即中止迭代。round 为本轮下标 (0 起, 供按序取候选),
// ok 为此前累计成功数 (供输出进度)。
func runIterations(ctx context.Context, title string, total int, gapMin, gapMax time.Duration, action func(round, ok int) (bool, error)) (taskResult, error) {
	var ok int

	for i := range total {
		if err := ctx.Err(); err != nil {
			return taskResult{}, err
		}

		if i > 0 {
			if err := utils.Sleep(ctx, gapMin, gapMax); err != nil {
				return taskResult{}, err
			}
		}

		done, err := action(i, ok)
		if err != nil {
			return taskResult{}, err
		}

		if done {
			ok++
		}
	}

	// 按迭代成功数聚合任务结果状态 (5.3): 全部成功 done, 部分成功 partial, 全部失败 failed
	status := taskFailed

	switch {
	case ok == total:
		status = taskDone
	case ok > 0:
		status = taskPartial
	}
	return taskResult{Title: title, Status: status}, nil
}

// isNumericString 判断 s 是否为非空纯 ASCII 数字字符串 (AC-006)。
func isNumericString(s string) bool {
	return s != "" && !strings.ContainsFunc(s, func(r rune) bool { return r < '0' || r > '9' })
}

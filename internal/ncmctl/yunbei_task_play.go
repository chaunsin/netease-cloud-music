// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
	apitypes "github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
)

func runYunBeiViewVIPCenter(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	// 会员中心浏览是会员专属福利：非会员账号上报返回包裹成功但 V3 进度永不推进（2026-10-30 实测），
	// 只会制造 unconfirmed 并连锁阻断 claim。
	if runtime.vipType == 0 {
		snapshot.State = yunBeiUnavailable
		return nil
	}

	if err := validateYunBeiActionableTask(snapshot); err != nil {
		return err
	}

	if snapshot.TaskID <= 0 {
		return errors.New("VIP-center task is missing taskId")
	}

	key := yunBeiActionKey{
		TaskID:   yunBeiViewVIPCenter,
		Kind:     "view",
		TargetID: strconv.FormatInt(snapshot.TaskID, 10),
	}
	if runtime.beginAction(key) {
		if err := runtime.sleep(ctx, yunBeiVIPCenterViewTime); err != nil {
			return fmt.Errorf("wait before VIP-center completion: %w", err)
		}

		reply, err := runtime.weapi.YunBeiVIPCenterTaskExternal(ctx, &weapi.YunBeiVIPCenterTaskExternalReq{Type: 1})
		if err != nil {
			return fmt.Errorf("YunBeiVIPCenterTaskExternal: %w", err)
		}

		if reply.Code != 200 || !reply.Data.Data {
			return fmt.Errorf(
				"YunBeiVIPCenterTaskExternal: code=%d data=%t message=%q",
				reply.Code,
				reply.Data.Data,
				reply.Data.Message,
			)
		}

		runtime.println("  会员中心: 已完成 10 秒浏览上报")
	}

	_, err := runtime.observeTask(ctx, snapshot, yunBeiQuickObserve)
	return err
}

func (runtime *yunBeiRuntime) loadDistributionList(ctx context.Context) (*weapi.YunBeiDistributionListRespData, error) {
	reply, err := runtime.weapi.YunBeiDistributionList(ctx, &weapi.YunBeiDistributionListReq{})
	if err != nil {
		return nil, fmt.Errorf("YunBeiDistributionList: %w", err)
	}

	if reply.Code != 200 {
		return nil, fmt.Errorf("YunBeiDistributionList: code=%d message=%q", reply.Code, reply.Message)
	}

	if reply.Data.Times < 0 || reply.Data.Times > yunBeiIndieDailyLimit {
		return nil, fmt.Errorf("YunBeiDistributionList: invalid times %d", reply.Data.Times)
	}

	if reply.Data.Amount < 0 || reply.Data.SingleAmount < 0 {
		return nil, errors.New("YunBeiDistributionList: amount must not be negative")
	}
	return &reply.Data, nil
}

func runYunBeiListenIndie(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	if snapshot.Target <= 0 || snapshot.Target > yunBeiIndieDailyLimit {
		return fmt.Errorf("listen-indie target %d is outside the supported daily range", snapshot.Target)
	}

	status, err := runtime.loadDistributionList(ctx)
	if err != nil {
		return err
	}

	if status.Times < snapshot.Current || status.Times > snapshot.Target {
		return fmt.Errorf(
			"listen-indie progress mismatch: task=%d/%d distribution=%d",
			snapshot.Current,
			snapshot.Target,
			status.Times,
		)
	}

	snapshot.Current = status.Times
	if status.Times >= snapshot.Target {
		_, observeErr := runtime.observeTask(ctx, snapshot, yunBeiQuickObserve)
		return observeErr
	}

	if status.SingleAmount <= 0 {
		return errors.New("YunBeiDistributionList: singleAmount is missing or invalid")
	}

	limit := min(yunBeiIndieBatchSize, int(snapshot.Target-status.Times))

	recommend, err := runtime.weapi.YunBeiDistributionRecommendSong(ctx, &weapi.YunBeiDistributionRecommendSongReq{
		Offset: int(status.Times),
		Limit:  limit,
	})
	if err != nil {
		return fmt.Errorf("YunBeiDistributionRecommendSong: %w", err)
	}

	if recommend.Code != 200 {
		return fmt.Errorf(
			"YunBeiDistributionRecommendSong: code=%d message=%q",
			recommend.Code,
			recommend.Message,
		)
	}

	var progressed bool

	for itemIndex := range recommend.Data {
		item := &recommend.Data[itemIndex]
		if item.SongId <= 0 || item.CompleteStatus {
			continue
		}

		key := yunBeiActionKey{
			TaskID:   yunBeiListenIndie,
			Kind:     "distribution-create",
			TargetID: strconv.FormatInt(item.SongId, 10),
		}
		if !runtime.beginAction(key) {
			continue
		}

		player, err := runtime.weapi.SongPlayerV1(ctx, &weapi.SongPlayerV1Req{
			Ids:   apitypes.IntsString{item.SongId},
			Level: apitypes.LevelStandard,
		})
		if err != nil {
			return fmt.Errorf("SongPlayerV1(%d): %w", item.SongId, err)
		}

		if player.Code != 200 || len(player.Data) == 0 || player.Data[0].Code != 200 || player.Data[0].Url == "" {
			return fmt.Errorf("SongPlayerV1(%d): no playable URL", item.SongId)
		}

		if runtime.fetchAudio == nil {
			return errors.New("listen-indie audio fetch hook is missing")
		}

		if fetchErr := runtime.fetchAudio(
			ctx,
			runtime.mediaTimeout,
			player.Data[0].Url,
			yunBeiIndieAudioFetchLimit,
		); fetchErr != nil {
			return fmt.Errorf("fetch listen-indie song %d: %w", item.SongId, fetchErr)
		}

		runtime.printf("  小众歌曲: 正在播放 %s (%d)，等待 30 秒\n", item.SongName, item.SongId)

		if sleepErr := runtime.sleep(ctx, yunBeiIndieListenTime); sleepErr != nil {
			return fmt.Errorf("listen to indie song %d: %w", item.SongId, sleepErr)
		}

		created, err := runtime.weapi.YunBeiDistributionCreate(ctx, &weapi.YunBeiDistributionCreateReq{
			YunbeiAmount: status.SingleAmount,
		})
		if err != nil {
			return fmt.Errorf("YunBeiDistributionCreate(%d): %w", item.SongId, err)
		}

		if created.Code != 200 || !created.Data {
			return fmt.Errorf(
				"YunBeiDistributionCreate(%d): code=%d data=%t message=%q",
				item.SongId,
				created.Code,
				created.Data,
				created.Message,
			)
		}

		refreshed, err := runtime.loadDistributionList(ctx)
		if err != nil {
			return fmt.Errorf("refresh distribution after song %d: %w", item.SongId, err)
		}

		if refreshed.Times <= status.Times {
			snapshot.State = yunBeiUnconfirmed
			return fmt.Errorf("YunBeiDistributionCreate(%d): progress was not confirmed", item.SongId)
		}

		status = refreshed
		snapshot.Current = status.Times
		progressed = true

		runtime.printf(
			"  小众歌曲: %s (%d) 已确认，今日进度 %d/%d\n",
			item.SongName,
			item.SongId,
			snapshot.Current,
			snapshot.Target,
		)

		after, reloadErr := runtime.reloadTask(ctx, yunBeiListenIndie)
		if reloadErr != nil {
			return fmt.Errorf("refresh listen-indie task: %w", reloadErr)
		}

		if after.State != yunBeiUnavailable {
			if after.Target == 0 {
				after.Target = snapshot.Target
			}

			if after.Current < snapshot.Current {
				after.Current = snapshot.Current
			}

			*snapshot = *after
		}

		if snapshot.State == yunBeiCompletedUnclaimed || snapshot.State == yunBeiClaimed || status.Times >= snapshot.Target {
			break
		}
	}

	if !progressed {
		return errors.New("YunBeiDistributionRecommendSong returned no usable uncompleted song")
	}
	return nil
}

func (runtime *yunBeiRuntime) reportWebLog(ctx context.Context, logs []map[string]any) error {
	reply, err := runtime.weapi.WebLog(ctx, &weapi.WebLogReq{Logs: logs})
	if err != nil {
		return fmt.Errorf("WebLog: %w", err)
	}

	if reply.Code != 200 {
		return fmt.Errorf("WebLog: code=%d message=%q", reply.Code, reply.Message)
	}
	return nil
}

func runYunBeiPlayDailyRecommend(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	if snapshot.Target <= 0 || snapshot.Current < 0 || snapshot.Current >= snapshot.Target {
		return fmt.Errorf("play-daily-recommend progress %d/%d is not actionable", snapshot.Current, snapshot.Target)
	}

	role, err := runtime.eapi.MusicianRoleGet(ctx, &eapi.MusicianRoleGetReq{})
	if err != nil {
		return fmt.Errorf("MusicianRoleGet: %w", err)
	}

	if role.Code != 200 {
		return fmt.Errorf("MusicianRoleGet: code=%d message=%q", role.Code, role.Message)
	}

	var ownArtistID int64

	if role.Data.IsMusician {
		if role.Data.UserId != runtime.uid || role.Data.ArtistId <= 0 {
			return fmt.Errorf(
				"MusicianRoleGet: musician identity does not map UID %d to a positive artist ID",
				runtime.uid,
			)
		}

		ownArtistID = role.Data.ArtistId
	}

	recommend, err := runtime.weapi.RecommendSongs(ctx, &weapi.RecommendSongsReq{})
	if err != nil {
		return fmt.Errorf("RecommendSongs: %w", err)
	}

	if recommend.Code != 200 {
		return fmt.Errorf("RecommendSongs: code=%d message=%q", recommend.Code, recommend.Message)
	}

	var progressed bool

	for candidateIndex := range recommend.Data.DailySongs {
		candidate := &recommend.Data.DailySongs[candidateIndex]
		if candidate.Id <= 0 {
			continue
		}

		ownSong := false

		for _, artist := range candidate.Ar {
			if ownArtistID > 0 && artist.Id == ownArtistID {
				ownSong = true
				break
			}
		}

		if ownSong {
			continue
		}

		detail, err := runtime.weapi.SongDetail(ctx, &weapi.SongDetailReq{
			C: []weapi.SongDetailReqList{{Id: strconv.FormatInt(candidate.Id, 10)}},
		})
		if err != nil {
			return fmt.Errorf("SongDetail(%d): %w", candidate.Id, err)
		}

		if detail.Code != 200 || len(detail.Songs) != 1 || detail.Songs[0].Id != candidate.Id {
			continue
		}

		song := &detail.Songs[0]
		if ownArtistID > 0 && slices.ContainsFunc(song.Ar, func(artist apitypes.Artist) bool {
			return artist.Id == ownArtistID
		}) {
			continue
		}

		durationSeconds := song.Dt / int64(time.Second/time.Millisecond)
		if durationSeconds <= 0 {
			continue
		}

		remaining := snapshot.Target - snapshot.Current
		if remaining <= 0 {
			break
		}

		playSeconds := min(durationSeconds, remaining)

		key := yunBeiActionKey{
			TaskID:   yunBeiPlayDailyRecommend,
			Kind:     "play",
			TargetID: strconv.FormatInt(song.Id, 10),
		}
		if !runtime.beginAction(key) {
			continue
		}

		if startErr := runtime.reportWebLog(ctx, []map[string]any{{
			"action": "startplay",
			"json": map[string]any{
				"id":       song.Id,
				"type":     "song",
				"content":  "id=" + strconv.FormatInt(song.Id, 10),
				"mainsite": "1",
			},
		}}); startErr != nil {
			return fmt.Errorf("startplay song %d: %w", song.Id, startErr)
		}

		player, err := runtime.weapi.SongPlayerV1(ctx, &weapi.SongPlayerV1Req{
			Ids:   apitypes.IntsString{song.Id},
			Level: apitypes.LevelStandard,
		})
		if err != nil {
			return fmt.Errorf("SongPlayerV1(%d): %w", song.Id, err)
		}

		if player.Code != 200 || len(player.Data) == 0 || player.Data[0].Code != 200 || player.Data[0].Url == "" {
			return fmt.Errorf("SongPlayerV1(%d): no playable URL", song.Id)
		}

		if runtime.fetchAudio == nil {
			return errors.New("play-daily-recommend audio fetch hook is missing")
		}

		if fetchErr := runtime.fetchAudio(
			ctx,
			runtime.mediaTimeout,
			player.Data[0].Url,
			yunBeiIndieAudioFetchLimit,
		); fetchErr != nil {
			return fmt.Errorf("fetch daily recommendation %d: %w", song.Id, fetchErr)
		}

		started := runtime.now()

		if sleepErr := runtime.sleep(ctx, time.Duration(playSeconds)*time.Second); sleepErr != nil {
			return fmt.Errorf("play daily recommendation %d: %w", song.Id, sleepErr)
		}

		elapsed := int64(runtime.now().Sub(started) / time.Second)
		if elapsed <= 0 {
			return fmt.Errorf("play daily recommendation %d: elapsed time was not positive", song.Id)
		}

		elapsed = min(elapsed, playSeconds, durationSeconds)

		end := "interrupt"
		if elapsed >= durationSeconds {
			end = "playend"
		}

		source := "toplist"
		sourceID := ""

		if song.Al.Id > 0 {
			source = "album"
			sourceID = strconv.FormatInt(song.Al.Id, 10)
		}

		if finishErr := runtime.reportWebLog(ctx, []map[string]any{{
			"action": "play",
			"json": map[string]any{
				"type":     "song",
				"wifi":     0,
				"download": 0,
				"id":       song.Id,
				"time":     elapsed,
				"end":      end,
				"source":   source,
				"sourceId": sourceID,
				"mainsite": "1",
				"content":  "id=" + strconv.FormatInt(song.Id, 10),
			},
		}}); finishErr != nil {
			return fmt.Errorf("finish song %d: %w", song.Id, finishErr)
		}

		beforeCurrent := snapshot.Current

		confirmed, err := runtime.observeTask(ctx, snapshot, yunBeiQuickObserve)
		if err != nil {
			return err
		}

		if !confirmed {
			return nil
		}

		progressed = progressed || snapshot.Current > beforeCurrent ||
			snapshot.State == yunBeiCompletedUnclaimed || snapshot.State == yunBeiClaimed
		runtime.printf(
			"  每日推荐: %s (%d) 上报 %d 秒，任务进度 %d/%d\n",
			song.Name,
			song.Id,
			elapsed,
			snapshot.Current,
			snapshot.Target,
		)

		if snapshot.State == yunBeiCompletedUnclaimed || snapshot.State == yunBeiClaimed {
			return nil
		}
	}

	if !progressed {
		snapshot.State = yunBeiUnavailable
	}
	return nil
}

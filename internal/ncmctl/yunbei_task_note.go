// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
)

func runYunBeiPublishNote(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	if err := validateYunBeiActionableTask(snapshot); err != nil {
		return err
	}

	key := yunBeiActionKey{
		TaskID:   yunBeiPublishNote,
		Kind:     "publish",
		TargetID: strconv.FormatInt(snapshot.TaskID, 10),
	}
	if !runtime.beginAction(key) {
		confirmed, err := runtime.observeTask(ctx, snapshot, yunBeiQuickObserve)
		if err != nil {
			return err
		}

		if confirmed {
			if err := runtime.deleteConfirmedNoteEvents(ctx, snapshot); err != nil {
				return err
			}
		}

		return nil
	}

	selector := &DailySongShare{uid: runtime.uid}

	song, err := selector.selectSong(ctx, runtime.weapi)
	if err != nil {
		return fmt.Errorf("select note song: %w", err)
	}

	var (
		imagePath string
		cleanup   func()
	)

	if runtime.noteImage != "" {
		imagePath = runtime.noteImage
		cleanup = func() {}
	} else {
		imagePath, cleanup, err = downloadMediaToTemp(
			ctx,
			mediaStrict,
			runtime.mediaTimeout,
			song.cover,
			dailySongCoverLimit,
			"ncmctl-yunbei-note-*",
		)
		if err != nil {
			return fmt.Errorf("download note cover: %w", err)
		}
	}

	defer cleanup()

	if runtime.uploadImage == nil || runtime.publishEvent == nil {
		return errors.New("publish-note API hooks are missing")
	}

	pics, err := runtime.uploadImage(ctx, imagePath)
	if err != nil {
		return fmt.Errorf("EventUploadImage: %w", err)
	}

	if strings.TrimSpace(pics) == "" {
		return errors.New("EventUploadImage: response did not contain picture metadata")
	}

	title, message := noteText(runtime.noteTitle, runtime.noteMessage, yunBeiNoteMessage, song)

	// 2026-11-02 取证：这条请求此前一直「发布成功但任务不推进」的原因是内置正文只有 19 字,
	// 未达 V3 taskDescription 的「发布一条≥20字的图文笔记」。正文改为 ≥20 字后, 同一账号在
	// 发布后数分钟内复核 V3 得到 taskId=6566461 completed=true progressRate=1/1
	// detailStatus=20(已完成待领取), 服务端自行创建了 userMissionId, 不需要 activityInfoList
	// 或 YunbeiClickTask 之类的归属载荷。
	// 但计入是异步的 (实测延迟介于 3 秒与 9.5 分钟之间), 所以复核用 yunBeiNoteObserve 而不是
	// 秒级窗口; 超出窗口仍未确认时按 unconfirmed 非零退出, 用 `yunbei status` 复核即可。
	published, err := runtime.publishEvent(ctx, &eapi.EventPublishReq{
		Title:              title,
		Msg:                message,
		Type:               "noresource",
		Pics:               pics,
		PrivacySetting:     "0",
		SocialSpaceVisible: 1,
	})
	if err != nil {
		return fmt.Errorf("EventPublish: %w", err)
	}

	if published.Code != 200 {
		return fmt.Errorf("EventPublish: code=%d message=%q", published.Code, published.Message)
	}

	if published.Id <= 0 {
		return errors.New("EventPublish: response did not contain a positive event ID")
	}

	if recordErr := runtime.addEventID(published.Id); recordErr != nil {
		return recordErr
	}

	// EventPublish 挂了 SetRedactLogs, 正文不落盘; 这里保留能把「发布动作」和
	// 「未推进的任务」对上的最小定位字段, 否则排查只能靠翻 V3 全量读取。
	runtime.l.Debugf("[yunbei] publish-note code=%d eventID=%d missionTaskID=%d bodyRedacted=true",
		published.Code, published.Id, snapshot.TaskID)

	runtime.printf("  发布图文: 动态ID %d\n", published.Id)

	confirmed, err := runtime.observeTask(ctx, snapshot, yunBeiNoteObserve)
	if err != nil {
		return err
	}

	if confirmed {
		if err := runtime.deleteConfirmedNoteEvents(ctx, snapshot); err != nil {
			return err
		}
	}

	return nil
}

func (runtime *yunBeiRuntime) deleteConfirmedNoteEvents(ctx context.Context, snapshot *yunBeiTaskSnapshot) error {
	if !runtime.deleteEvents {
		return nil
	}

	for _, eventID := range runtime.eventIDs {
		if !slices.Contains(runtime.deleteEventIDs, eventID) {
			if err := runtime.addEventDeleteIntent(eventID); err != nil {
				snapshot.State = yunBeiUnconfirmed
				return err
			}
		}
	}

	if err := runtime.deletePendingEvents(ctx); err != nil {
		snapshot.State = yunBeiUnconfirmed
		return err
	}

	return nil
}

// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

type yunBeiRuntime struct {
	command        *YunBei
	l              *log.Logger
	weapi          *weapi.Api
	eapi           *eapi.Api
	uid            int64
	vipType        int64
	round          int
	deleteEvents   bool
	noteImage      string
	noteTitle      string
	noteMessage    string
	mediaTimeout   time.Duration
	attempted      map[yunBeiActionKey]struct{}
	results        map[yunBeiTaskID]*yunBeiResult
	compensations  []yunBeiCompensation
	eventIDs       []int64
	deleteEventIDs []int64
	sleep          func(context.Context, time.Duration, ...time.Duration) error
	fetchAudio     func(context.Context, time.Duration, string, int64) error
	uploadImage    func(context.Context, string) (string, error)
	publishEvent   func(context.Context, *eapi.EventPublishReq) (*eapi.EventPublishResp, error)
	loadSnapshot   yunBeiTaskSnapshotLoader
	now            func() time.Time
	withTimeout    func(context.Context, time.Duration) (context.Context, context.CancelFunc)
}

// printf 与 println 是任务处理器唯一的输出出口, 避免各处理器穿透到 command.cmd。
func (runtime *yunBeiRuntime) printf(format string, args ...any) {
	runtime.command.cmd.Printf(format, args...)
}

func (runtime *yunBeiRuntime) println(args ...any) {
	runtime.command.cmd.Println(args...)
}

type yunBeiActionKey struct {
	TaskID   yunBeiTaskID
	Kind     string
	TargetID string
}

func (runtime *yunBeiRuntime) beginAction(key yunBeiActionKey) bool {
	if _, exists := runtime.attempted[key]; exists {
		return false
	}

	runtime.attempted[key] = struct{}{}
	return true
}

func (runtime *yunBeiRuntime) attemptedTask(taskID yunBeiTaskID) bool {
	for key := range runtime.attempted {
		if key.TaskID == taskID {
			return true
		}
	}
	return false
}

func (runtime *yunBeiRuntime) addEventID(eventID int64) error {
	if eventID <= 0 {
		return errors.New("event ID must be positive")
	}

	if slices.Contains(runtime.eventIDs, eventID) {
		return fmt.Errorf("event ID %d is already recorded", eventID)
	}

	runtime.eventIDs = append(runtime.eventIDs, eventID)
	return nil
}

func (runtime *yunBeiRuntime) addEventDeleteIntent(eventID int64) error {
	if !slices.Contains(runtime.eventIDs, eventID) {
		return fmt.Errorf("event ID %d was not recorded by this run", eventID)
	}

	if slices.Contains(runtime.deleteEventIDs, eventID) {
		return fmt.Errorf("event ID %d already has a delete intent", eventID)
	}

	runtime.deleteEventIDs = append(runtime.deleteEventIDs, eventID)
	return nil
}

type yunBeiCompensation struct {
	TaskID   yunBeiTaskID
	Kind     string
	TargetID string
	ScopeID  string
	Done     bool
	restore  func(context.Context) error
}

func (runtime *yunBeiRuntime) addCompensation(taskID yunBeiTaskID, kind, targetID, scopeID string, restore func(context.Context) error) {
	runtime.compensations = append(runtime.compensations, yunBeiCompensation{
		TaskID:   taskID,
		Kind:     kind,
		TargetID: targetID,
		ScopeID:  scopeID,
		restore:  restore,
	})
}

func (runtime *yunBeiRuntime) hasPendingCompensation(start int, taskID yunBeiTaskID) bool {
	if start < 0 || start > len(runtime.compensations) {
		return false
	}

	for index := start; index < len(runtime.compensations); index++ {
		compensation := &runtime.compensations[index]
		if compensation.TaskID == taskID && !compensation.Done {
			return true
		}
	}
	return false
}

func (runtime *yunBeiRuntime) cleanup(ctx context.Context) error {
	deleteErr := runtime.deletePendingEvents(ctx)
	restoreErr := runtime.restoreCompensations(ctx, 0)
	return errors.Join(deleteErr, restoreErr)
}

func (runtime *yunBeiRuntime) deletePendingEvents(ctx context.Context) error {
	if !runtime.deleteEvents {
		return nil
	}

	var errs []error

	for index, eventID := range slices.Backward(runtime.deleteEventIDs) {
		if eventID <= 0 {
			continue
		}

		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}

		reply, err := runtime.eapi.EventDelete(ctx, &eapi.EventDeleteReq{Id: eventID})
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("delete event %d: %w", eventID, err))
		case reply.Code != 200:
			errs = append(errs, fmt.Errorf("delete event %d: code=%d message=%q", eventID, reply.Code, reply.Message))
		default:
			runtime.deleteEventIDs[index] = 0
			runtime.printf("  删除图文: 动态ID %d 已恢复\n", eventID)
		}
	}

	if len(errs) != 0 {
		runtime.markPartial(yunBeiPublishNote, errors.Join(errs...))
		runtime.printPendingEventIDs()
	}
	return errors.Join(errs...)
}

func (runtime *yunBeiRuntime) printPendingEventIDs() {
	for _, eventID := range runtime.deleteEventIDs {
		if eventID > 0 {
			runtime.printf("  删除图文未完成: 动态ID %d\n", eventID)
		}
	}
}

func (runtime *yunBeiRuntime) restoreCompensations(ctx context.Context, start int, taskIDs ...yunBeiTaskID) error {
	if start < 0 || start > len(runtime.compensations) {
		return fmt.Errorf("invalid compensation start index %d", start)
	}

	var errs []error

	for index := len(runtime.compensations) - 1; index >= start; index-- {
		compensation := &runtime.compensations[index]
		if compensation.Done || len(taskIDs) != 0 && !slices.Contains(taskIDs, compensation.TaskID) {
			continue
		}

		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}

		if compensation.restore == nil {
			errs = append(errs, fmt.Errorf(
				"restore %s target=%s scope=%s: handler is missing",
				compensation.Kind,
				compensation.TargetID,
				compensation.ScopeID,
			))
			continue
		}

		if err := compensation.restore(ctx); err != nil {
			errs = append(errs, fmt.Errorf(
				"restore %s target=%s scope=%s: %w",
				compensation.Kind,
				compensation.TargetID,
				compensation.ScopeID,
				err,
			))
			runtime.markPartial(compensation.TaskID, err)
			runtime.printf(
				"  恢复失败: %s target=%s scope=%s\n",
				compensation.Kind,
				compensation.TargetID,
				compensation.ScopeID,
			)
			continue
		}

		compensation.Done = true
		runtime.printf(
			"  恢复成功: %s target=%s scope=%s\n",
			compensation.Kind,
			compensation.TargetID,
			compensation.ScopeID,
		)
	}

	if ctx.Err() != nil {
		for index := len(runtime.compensations) - 1; index >= start; index-- {
			compensation := &runtime.compensations[index]
			if !compensation.Done && (len(taskIDs) == 0 || slices.Contains(taskIDs, compensation.TaskID)) {
				runtime.markPartial(compensation.TaskID, ctx.Err())
				runtime.printf(
					"  恢复未完成: %s target=%s scope=%s\n",
					compensation.Kind,
					compensation.TargetID,
					compensation.ScopeID,
				)
			}
		}
	}
	return errors.Join(errs...)
}

func (runtime *yunBeiRuntime) markPartial(taskID yunBeiTaskID, err error) {
	result := runtime.results[taskID]
	if result == nil {
		return
	}

	result.State = yunBeiUnconfirmed
	result.Err = errors.Join(result.Err, err)
}

func newYunBeiRuntime(command *YunBei, w *weapi.Api, e *eapi.Api, uid int64) *yunBeiRuntime {
	runtime := &yunBeiRuntime{
		command:        command,
		l:              command.l,
		weapi:          w,
		eapi:           e,
		uid:            uid,
		round:          1,
		deleteEvents:   command.opts.Delete,
		noteImage:      command.opts.Image,
		noteTitle:      command.opts.Title,
		noteMessage:    command.opts.Message,
		mediaTimeout:   command.networkTimeout(),
		attempted:      make(map[yunBeiActionKey]struct{}),
		results:        make(map[yunBeiTaskID]*yunBeiResult),
		compensations:  make([]yunBeiCompensation, 0),
		eventIDs:       make([]int64, 0),
		deleteEventIDs: make([]int64, 0),
		sleep:          utils.Sleep,
		fetchAudio: func(ctx context.Context, timeout time.Duration, rawURL string, limit int64) error {
			return fetchMediaAudioSample(ctx, mediaStrict, timeout, rawURL, limit)
		},
		uploadImage: func(ctx context.Context, path string) (string, error) {
			return e.EventUploadImage(ctx, path)
		},
		publishEvent: func(ctx context.Context, req *eapi.EventPublishReq) (*eapi.EventPublishResp, error) {
			return e.EventPublish(ctx, req)
		},
		now:         time.Now,
		withTimeout: context.WithTimeout,
	}
	runtime.loadSnapshot = runtime.loadTaskSnapshot
	return runtime
}

func (runtime *yunBeiRuntime) reloadTask(ctx context.Context, id yunBeiTaskID) (*yunBeiTaskSnapshot, error) {
	if runtime.loadSnapshot == nil {
		return nil, errors.New("YunBei task snapshot loader is missing")
	}

	snapshot, err := runtime.loadSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	task := snapshot.task(id)
	if task == nil {
		return &yunBeiTaskSnapshot{ID: id, State: yunBeiUnavailable}, nil
	}
	return task, nil
}

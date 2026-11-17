// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

type yunBeiTaskSnapshotLoader func(context.Context) (*yunBeiSnapshot, error)

type yunBeiTaskReloader func(context.Context) (*yunBeiTaskSnapshot, error)

// yunBeiObserveWindow 是一次写操作之后复核服务端状态所用的读取次数与间隔。
type yunBeiObserveWindow struct {
	reads     int
	pollDelay time.Duration
}

var (
	// yunBeiQuickObserve 覆盖服务端秒级生效的写操作。
	yunBeiQuickObserve = yunBeiObserveWindow{reads: yunBeiObserveReads, pollDelay: yunBeiObserveInterval}

	// yunBeiNoteObserve 给 publish-note 使用: 服务端在笔记发布后异步创建 userMissionId,
	// 2026-11-02 实测计入延迟介于 3 秒与 9.5 分钟之间, 秒级窗口必然读不到。
	yunBeiNoteObserve = yunBeiObserveWindow{reads: yunBeiNoteObserveReads, pollDelay: yunBeiNoteObserveInterval}
)

type yunBeiRoundPlan struct {
	registry []yunBeiTaskDef
	selected map[yunBeiTaskID]struct{}
	load     yunBeiTaskSnapshotLoader
}

func (runtime *yunBeiRuntime) observe(ctx context.Context, before *yunBeiTaskSnapshot, window yunBeiObserveWindow, reload yunBeiTaskReloader) (*yunBeiTaskSnapshot, bool, error) {
	if before == nil {
		return nil, false, errors.New("observe YunBei task: before snapshot is missing")
	}

	if window.reads < 0 {
		return nil, false, errors.New("observe YunBei task: reads must not be negative")
	}

	if window.pollDelay < 0 {
		return nil, false, errors.New("observe YunBei task: poll delay must not be negative")
	}

	var after *yunBeiTaskSnapshot

	for attempt := range window.reads {
		if attempt > 0 {
			if err := runtime.sleep(ctx, window.pollDelay); err != nil {
				return after, false, fmt.Errorf("observe YunBei task wait: %w", err)
			}
		}

		var err error

		after, err = reload(ctx)
		if err != nil {
			return after, false, fmt.Errorf("observe YunBei task reload: %w", err)
		}

		if after == nil {
			return nil, false, errors.New("observe YunBei task: reloaded snapshot is missing")
		}

		switch after.State {
		case yunBeiCompletedUnclaimed, yunBeiClaimed:
			return after, true, nil
		case yunBeiPending, yunBeiRunning, yunBeiUnconfirmed:
			if after.Current > before.Current {
				return after, true, nil
			}
		case yunBeiUnavailable:
			continue
		case yunBeiUnsupported, yunBeiFailed:
			return after, false, fmt.Errorf("observe YunBei task: unsafe state %s", after.State)
		}
	}
	return after, false, nil
}

func (c *YunBei) runYunBeiTaskRounds(ctx context.Context, runtime *yunBeiRuntime, plan *yunBeiRoundPlan) []error {
	if !hasSelectedYunBeiTasks(plan, stageFast, stageSlow) {
		return nil
	}

	var errs []error

	for round := 1; round <= yunBeiMaxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return append(errs, err)
		}

		runtime.round = round

		taskSnapshot, err := plan.load(ctx)
		if err != nil {
			for defIndex := range plan.registry {
				def := &plan.registry[defIndex]
				if def.Stage != stageFast && def.Stage != stageSlow {
					continue
				}

				if _, ok := plan.selected[def.ID]; !ok {
					continue
				}

				if !yunBeiResultNeedsRound(runtime.results[def.ID]) {
					continue
				}

				result := &yunBeiResult{ID: def.ID, State: yunBeiFailed, Err: err}
				runtime.results[def.ID] = result
				errs = append(errs, fmt.Errorf("%s task state: %w", def.ID, err))
				c.cmd.Printf("\n  [%s] 状态源失败: %v\n", def.ID, err)
			}
			break
		}

		if round == 1 && len(taskSnapshot.Conflicts) != 0 {
			c.cmd.Printf("  任务关联待确认: %d 项（仅隔离对应条目）\n", len(taskSnapshot.Conflicts))
		}

		var (
			roundProgress bool
			stillPending  bool
		)

		for defIndex := range plan.registry {
			def := &plan.registry[defIndex]
			if def.Stage != stageFast && def.Stage != stageSlow {
				continue
			}

			if _, ok := plan.selected[def.ID]; !ok {
				continue
			}

			if !yunBeiResultNeedsRound(runtime.results[def.ID]) {
				continue
			}

			snapshot := taskSnapshot.task(def.ID)
			if snapshot == nil {
				snapshot = &yunBeiTaskSnapshot{ID: def.ID, State: yunBeiUnavailable}
			}

			progressed, runErr := c.runYunBeiDefinition(ctx, runtime, def, snapshot)
			if runErr != nil {
				errs = append(errs, runErr)
				continue
			}

			roundProgress = roundProgress || progressed
			stillPending = stillPending || snapshot.State == yunBeiPending || snapshot.State == yunBeiRunning
		}

		if !roundProgress || !stillPending {
			break
		}
	}

	for defIndex := range plan.registry {
		def := &plan.registry[defIndex]
		if def.Stage != stageFast && def.Stage != stageSlow {
			continue
		}

		if _, ok := plan.selected[def.ID]; !ok {
			continue
		}

		result := runtime.results[def.ID]
		if result == nil || result.State != yunBeiRunning {
			continue
		}

		roundErr := fmt.Errorf("%s: task remained incomplete after %d rounds", def.ID, yunBeiMaxRounds)
		result.State = yunBeiUnconfirmed
		result.Err = errors.Join(result.Err, roundErr)
		errs = append(errs, roundErr)

		c.cmd.Printf("  [%s] 已达到 %d 轮上限，任务未确认\n", def.ID, yunBeiMaxRounds)
	}
	return errs
}

func yunBeiResultNeedsRound(result *yunBeiResult) bool {
	if result == nil {
		return true
	}

	switch result.State {
	case yunBeiPending, yunBeiRunning:
		return true
	case yunBeiUnavailable, yunBeiCompletedUnclaimed, yunBeiClaimed, yunBeiUnconfirmed, yunBeiFailed, yunBeiUnsupported:
		return false
	}
	return false
}

func hasSelectedYunBeiTasks(plan *yunBeiRoundPlan, stages ...yunBeiStage) bool {
	for _, def := range plan.registry {
		if _, ok := plan.selected[def.ID]; ok && slices.Contains(stages, def.Stage) {
			return true
		}
	}
	return false
}

// runStandaloneStage 运行不依赖 V3 快照的阶段。spec 允许 sign/reserve/claim 独立于常规
// 任务列表，因此这里用最小合成快照驱动，而不是等待 correlateYunBeiTasks 给出身份。
func (c *YunBei) runStandaloneStage(ctx context.Context, runtime *yunBeiRuntime, selected map[yunBeiTaskID]struct{}, stage yunBeiStage) []error {
	var errs []error

	for _, def := range yunBeiRegistry {
		if def.Stage != stage {
			continue
		}

		if _, ok := selected[def.ID]; !ok {
			continue
		}

		snapshot := &yunBeiTaskSnapshot{ID: def.ID, State: yunBeiPending}
		if _, err := c.runYunBeiDefinition(ctx, runtime, &def, snapshot); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (c *YunBei) runYunBeiDefinition(ctx context.Context, runtime *yunBeiRuntime, def *yunBeiTaskDef, snapshot *yunBeiTaskSnapshot) (bool, error) {
	result := &yunBeiResult{ID: def.ID, State: yunBeiRunning}
	runtime.results[def.ID] = result
	c.cmd.Printf("\n  [%s] 开始（第 %d 轮）\n", def.ID, runtime.round)

	if err := ctx.Err(); err != nil {
		result.State = yunBeiFailed
		result.Err = err
		return false, fmt.Errorf("%s: %w", def.ID, err)
	}

	// 取证未闭环的处理器一律不执行: 这类动作可能每步都返回成功却从不计数（2026-11-03 的
	// play-daily-recommend 就是几十分钟流量与等待换来 0/1800），还会稳定制造 partial 非零退出。
	// 归入 skipped 而不是 failed——失败要留给真实的写入/校验错误。
	if def.EvidenceGap != "" {
		result.State = yunBeiUnavailable

		c.cmd.Printf("  [%s] 取证未闭环，已跳过: %s\n", def.ID, def.EvidenceGap)
		return false, nil
	}

	switch snapshot.State {
	case yunBeiUnavailable:
		result.State = yunBeiUnavailable

		c.cmd.Printf("  [%s] 今日无需执行\n", def.ID)
		return false, nil
	case yunBeiClaimed, yunBeiCompletedUnclaimed:
		result.State = yunBeiUnavailable

		c.cmd.Printf("  [%s] 服务端已完成，已跳过\n", def.ID)
		return false, nil
	case yunBeiUnsupported, yunBeiUnconfirmed, yunBeiFailed:
		stateErr := fmt.Errorf("unsafe task state %s", snapshot.State)
		result.State = snapshot.State
		result.Err = stateErr

		c.cmd.Printf("  [%s] 状态不可执行: %s\n", def.ID, snapshot.State)
		return false, fmt.Errorf("%s: %w", def.ID, stateErr)
	case yunBeiPending, yunBeiRunning:
	}

	beforeCurrent := snapshot.Current

	beforeState := snapshot.State
	compensationStart := len(runtime.compensations)
	runErr := def.Run(ctx, runtime, snapshot)

	if snapshot.State != yunBeiUnconfirmed && runtime.hasPendingCompensation(compensationStart, def.ID) {
		restoreErr := runtime.restoreCompensations(ctx, compensationStart, def.ID)
		if restoreErr == nil && runErr == nil {
			restoreErr = runtime.verifyRestored(ctx, snapshot)
		}

		if restoreErr != nil {
			snapshot.State = yunBeiUnconfirmed
			runErr = errors.Join(runErr, restoreErr)
		}
	}

	if runErr != nil {
		result.State = snapshot.State
		if result.State == yunBeiPending || result.State == yunBeiRunning {
			result.State = yunBeiFailed
		}

		result.Err = runErr
		c.cmd.Printf("  [%s] 失败: %v\n", def.ID, runErr)
		return false, fmt.Errorf("%s: %w", def.ID, runErr)
	}

	progressed := snapshot.Current > beforeCurrent ||
		(beforeState == yunBeiPending || beforeState == yunBeiRunning) &&
			(snapshot.State == yunBeiCompletedUnclaimed || snapshot.State == yunBeiClaimed)
	switch snapshot.State {
	case yunBeiUnavailable:
		result.State = yunBeiUnavailable

		c.cmd.Printf("  [%s] 今日无需执行\n", def.ID)
		return false, nil
	case yunBeiPending, yunBeiRunning:
		if progressed {
			result.State = yunBeiRunning

			c.cmd.Printf("  [%s] 进度已确认: %d\n", def.ID, snapshot.Current)
			return true, nil
		}

		unconfirmedErr := errors.New("action succeeded but task state did not change")
		snapshot.State = yunBeiUnconfirmed
		result.State = yunBeiUnconfirmed
		result.Err = unconfirmedErr

		c.cmd.Printf("  [%s] 动作已发送，任务未确认\n", def.ID)
		return false, fmt.Errorf("%s: %w", def.ID, unconfirmedErr)
	case yunBeiCompletedUnclaimed, yunBeiClaimed:
		result.State = snapshot.State

		c.cmd.Printf("  [%s] 完成\n", def.ID)
		return progressed, nil
	case yunBeiUnsupported, yunBeiUnconfirmed, yunBeiFailed:
		stateErr := fmt.Errorf("handler returned state %s", snapshot.State)
		result.State = snapshot.State
		result.Err = stateErr
		return false, fmt.Errorf("%s: %w", def.ID, stateErr)
	}
	return false, errors.New("unreachable YunBei task state")
}

func (c *YunBei) printExecutionPlan() {
	c.cmd.Println("\n  本次选择及副作用:")

	selected := make(map[yunBeiTaskID]struct{}, len(c.selected))
	for _, id := range c.selected {
		selected[id] = struct{}{}
	}

	for _, def := range yunBeiRegistry {
		if _, ok := selected[def.ID]; ok {
			c.cmd.Printf("    - %s: %s\n", def.ID, def.SideEffects)
		}
	}
}

func (c *YunBei) printSummary(runtime *yunBeiRuntime) {
	counts := map[string]int{"success": 0, "skipped": 0, "partial": 0, "failed": 0}

	for _, result := range runtime.results {
		switch result.State {
		case yunBeiUnavailable:
			counts["skipped"]++
		case yunBeiUnconfirmed:
			counts["partial"]++
		case yunBeiFailed, yunBeiUnsupported:
			counts["failed"]++
		default:
			counts["success"]++
		}
	}

	c.cmd.Printf("\n  汇总: success=%d skipped=%d partial=%d failed=%d\n",
		counts["success"], counts["skipped"], counts["partial"], counts["failed"])
}

func (runtime *yunBeiRuntime) observeTask(ctx context.Context, before *yunBeiTaskSnapshot, window yunBeiObserveWindow) (bool, error) {
	after, confirmed, err := runtime.observe(
		ctx,
		before,
		window,
		func(ctx context.Context) (*yunBeiTaskSnapshot, error) {
			return runtime.reloadTask(ctx, before.ID)
		},
	)
	if err != nil {
		return false, err
	}

	if after != nil && after.State != yunBeiUnavailable {
		*before = *after
	}
	return confirmed, nil
}

func (runtime *yunBeiRuntime) verifyRestored(ctx context.Context, expected *yunBeiTaskSnapshot) error {
	after, err := runtime.reloadTask(ctx, expected.ID)
	if err != nil {
		return fmt.Errorf("reload task after restore: %w", err)
	}

	if after.State == yunBeiUnavailable {
		return errors.New("task disappeared after restoring account state")
	}

	expectedCompleted := expected.State == yunBeiCompletedUnclaimed || expected.State == yunBeiClaimed

	afterCompleted := after.State == yunBeiCompletedUnclaimed || after.State == yunBeiClaimed
	if expectedCompleted && !afterCompleted {
		return fmt.Errorf("task regressed from %s to %s after restore", expected.State, after.State)
	}

	if after.Current < expected.Current {
		return fmt.Errorf("task progress regressed from %d to %d after restore", expected.Current, after.Current)
	}

	*expected = *after
	return nil
}

// pollRestore 在至多 yunBeiObserveReads 次读取内等待回滚在服务端生效, 每次重读前等待
// yunBeiObserveInterval; confirmed 自身报错立即传播, 读满仍未生效则返回 failed。
func (runtime *yunBeiRuntime) pollRestore(ctx context.Context, reason string, confirmed func(context.Context) (bool, error), failed error) error {
	for attempt := range yunBeiObserveReads {
		if attempt > 0 {
			if err := runtime.sleep(ctx, yunBeiObserveInterval); err != nil {
				return fmt.Errorf("wait for %s: %w", reason, err)
			}
		}

		ok, err := confirmed(ctx)
		if err != nil {
			return err
		}

		if ok {
			return nil
		}
	}
	return failed
}

// restoreAndVerify 回滚本轮产生的补偿动作, 并确认任务进度没有因回滚倒退。
func (runtime *yunBeiRuntime) restoreAndVerify(ctx context.Context, snapshot *yunBeiTaskSnapshot, start int, taskIDs ...yunBeiTaskID) error {
	if err := runtime.restoreCompensations(ctx, start, taskIDs...); err != nil {
		snapshot.State = yunBeiUnconfirmed
		return err
	}

	if err := runtime.verifyRestored(ctx, snapshot); err != nil {
		snapshot.State = yunBeiUnconfirmed
		return err
	}
	return nil
}

func validateYunBeiActionableTask(snapshot *yunBeiTaskSnapshot) error {
	if snapshot == nil {
		return errors.New("YunBei task snapshot is missing")
	}

	if snapshot.Current < 0 || snapshot.Target <= 0 || snapshot.Current >= snapshot.Target {
		return fmt.Errorf("task %s progress %d/%d is not actionable", snapshot.ID, snapshot.Current, snapshot.Target)
	}
	return nil
}

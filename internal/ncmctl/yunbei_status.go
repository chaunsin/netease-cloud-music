// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
)

func (c *YunBei) printStatus(ctx context.Context, runtime *yunBeiRuntime) error {
	var (
		errs    []error
		items   []weapi.YunBeiTaskListV3Item
		todos   []eapi.YunBeiTaskTodoRespData
		itemsOK bool
		todosOK bool
	)

	balance, err := runtime.weapi.YunBeiBalance(ctx, &weapi.YunBeiBalanceReq{Currency: "credit"})
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("YunBeiBalance: %w", err))
	case balance.Code != 200:
		errs = append(errs, fmt.Errorf("YunBeiBalance: code=%d message=%q", balance.Code, balance.Message))
	case balance.Data.UserId != 0 && balance.Data.UserId != runtime.uid:
		errs = append(errs, fmt.Errorf("YunBeiBalance: response user ID %d does not match login user ID %d", balance.Data.UserId, runtime.uid))
	default:
		c.cmd.Printf("  云贝: 可用 %d，冻结 %d\n", balance.Data.Balance, balance.Data.BlockBalance)
	}

	calendar, err := runtime.weapi.YunBeiSignInCalendar(ctx, &weapi.YunBeiSignInCalendarReq{})
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("YunBeiSignInCalendar: %w", err))
	case calendar.Code != 200:
		errs = append(errs, fmt.Errorf("YunBeiSignInCalendar: code=%d message=%q", calendar.Code, calendar.Message))
	default:
		signed, signedErr := yunBeiSignedToday(&calendar.Data)
		switch {
		case signedErr != nil:
			errs = append(errs, signedErr)
		case signed:
			c.cmd.Println("  今日签到: 已签到")
		default:
			c.cmd.Println("  今日签到: 未签到")
		}
	}

	progress, err := runtime.weapi.YunBeiSignInProgress(ctx, &weapi.YunBeiSignInProgressReq{})
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("YunBeiSignInProgress: %w", err))
	case progress.Code != 200:
		errs = append(errs, fmt.Errorf("YunBeiSignInProgress: code=%d message=%q", progress.Code, progress.Message))
	default:
		c.cmd.Printf("  连签奖励: %d 项\n", len(progress.Data.LotteryConfig))

		for itemIndex := range progress.Data.LotteryConfig {
			item := &progress.Data.LotteryConfig[itemIndex]
			c.cmd.Printf(
				"    - 第 %d 天: base(status=%d id=%d) extra(status=%d id=%d)\n",
				item.SignDay,
				item.BaseLotteryStatus,
				item.BaseLotteryId,
				item.ExtraLotteryStatus,
				item.ExtraLotteryId,
			)
		}
	}

	reserve, err := runtime.eapi.YunbeiReserveInfo(ctx, &eapi.YunbeiReserveInfoReq{})
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("YunbeiReserveInfo: %w", err))
	case reserve.Code != 200 || !reserve.Success:
		errs = append(errs, fmt.Errorf("YunbeiReserveInfo: code=%d success=%t message=%q", reserve.Code, reserve.Success, reserve.Message))
	default:
		if reserve.Data.CurrentAmount > 0 {
			c.cmd.Printf("  预约活动: %s（当前 %d 云贝）\n", reserve.Data.Type, reserve.Data.CurrentAmount)
		} else {
			c.cmd.Printf("  预约活动: %s\n", reserve.Data.Type)
		}
	}

	list, err := runtime.weapi.YunBeiTaskListV3(ctx, &weapi.YunBeiTaskListV3Req{})
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("YunBeiTaskListV3: %w", err))
	case list.Code != 200:
		errs = append(errs, fmt.Errorf("YunBeiTaskListV3: code=%d message=%q", list.Code, list.Message))
	default:
		items = list.Data.Normal.List
		itemsOK = true

		if len(list.Data.UnknownBucket) != 0 {
			names := make([]string, 0, len(list.Data.UnknownBucket))
			for name := range list.Data.UnknownBucket {
				names = append(names, name)
			}

			slices.Sort(names)
			c.cmd.Printf("    - 未识别任务 bucket: %s（仅展示摘要）\n", strings.Join(names, ", "))
		}
	}

	todo, err := runtime.eapi.YunBeiTaskTodo(ctx, &eapi.YunBeiTaskTodoReq{})
	switch {
	case err != nil:
		errs = append(errs, fmt.Errorf("YunBeiTaskTodo: %w", err))
	case todo.Code != 200:
		errs = append(errs, fmt.Errorf("YunBeiTaskTodo: code=%d message=%q", todo.Code, todo.Message))
	default:
		todos = todo.Data
		todosOK = true

		pendingClaims := 0

		for _, item := range todo.Data {
			if item.Completed {
				pendingClaims++
			}
		}

		c.cmd.Printf("  待领奖励: %d 项\n", pendingClaims)
	}

	if itemsOK && todosOK {
		snapshot := correlateYunBeiTasks(items, todos, yunBeiTaskIdentities())
		c.printTaskStatus(items, snapshot)

		if len(snapshot.Conflicts) != 0 {
			c.cmd.Printf("  任务关联待确认: %d 项（执行模式会安全跳过）\n", len(snapshot.Conflicts))
		}
	}

	return errors.Join(errs...)
}

func (c *YunBei) printTaskStatus(items []weapi.YunBeiTaskListV3Item, snapshot *yunBeiSnapshot) {
	c.cmd.Println("  任务:")

	for index, task := range snapshot.Tasks {
		localID := "-"
		if task.ID != "" {
			localID = string(task.ID)
		}

		missionCode := task.MissionCode
		if missionCode == "" {
			missionCode = "-"
		}

		name := task.Name
		if name == "" {
			name = "未命名任务"
		}

		if index < len(items) {
			item := &items[index]

			subAction := item.SubAction
			if subAction == "" {
				subAction = "-"
			}

			if current, target, ok := yunBeiTaskProgress(item); ok {
				c.cmd.Printf(
					"    - %s | 本地标识 %s | taskId %d | missionCode %s | 原始计数 %d/%d | 奖励 %d | %s\n",
					name,
					localID,
					item.TaskId,
					missionCode,
					current,
					target,
					task.Reward,
					task.State,
				)
			} else {
				c.cmd.Printf(
					"    - %s | 本地标识 %s | taskId %d | missionCode %s | 原始计数 - | 奖励 %d | %s\n",
					name,
					localID,
					item.TaskId,
					missionCode,
					task.Reward,
					task.State,
				)
			}

			c.cmd.Printf(
				"      服务端动作: userTaskId=%d actionType=%d subAction=%s\n",
				item.UserTaskId,
				item.ActionType,
				subAction,
			)
			continue
		}

		c.cmd.Printf(
			"    - %s | 本地标识 %s | missionCode %s | 原始计数 - | 奖励 %d | %s\n",
			name,
			localID,
			missionCode,
			task.Reward,
			task.State,
		)
	}

	for todoIndex := range snapshot.UnmatchedTodos {
		todo := &snapshot.UnmatchedTodos[todoIndex]

		state := yunBeiPending
		if todo.Completed {
			state = yunBeiCompletedUnclaimed
		}

		c.cmd.Printf(
			"    - %s | 本地标识 - | 待办未关联(taskId=%d userTaskId=%d) | 原始计数 - | 奖励 %d | %s\n",
			todo.TaskName,
			todo.TaskId,
			todo.UserTaskId,
			todo.TaskPoint,
			state,
		)
	}
}

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

type yunBeiCorrelation struct {
	snapshot           *yunBeiSnapshot
	byUserTaskID       map[int64]int
	byTaskID           map[int64]int
	seenTodoUserTaskID map[int64]struct{}
	seenTodoTaskID     map[int64]struct{}
}

func correlateYunBeiTasks(items []weapi.YunBeiTaskListV3Item, todos []eapi.YunBeiTaskTodoRespData, identities yunBeiTaskIdentityIndex) *yunBeiSnapshot {
	snapshot := &yunBeiSnapshot{Tasks: make([]*yunBeiTaskSnapshot, 0, len(items)+len(todos))}
	byUserTaskID := make(map[int64]int, len(items))
	byTaskID := make(map[int64]int, len(items))
	byMissionCode := make(map[string]int, len(items))
	byLocalID := make(map[yunBeiTaskID]int, len(items))

	for itemIndex := range items {
		item := &items[itemIndex]

		task := &yunBeiTaskSnapshot{
			TaskID:      item.TaskId,
			UserTaskID:  item.UserTaskId,
			Name:        item.TaskName,
			Description: item.TaskDescription,
			Reward:      item.TaskPoint,
			State:       yunBeiPending,
			Action: &yunBeiActionRef{
				TaskID:    item.TaskId,
				SubAction: item.SubAction,
			},
		}
		if current, target, ok := yunBeiTaskProgress(item); ok {
			task.Current = current
			task.Target = target
		}

		if item.Completed {
			task.State = yunBeiClaimed
		}

		var missionID yunBeiTaskID

		if item.ExtInfoMap != nil {
			task.MissionCode = strings.TrimSpace(item.ExtInfoMap.MissionCode)
			missionID = identities.missionCodes[task.MissionCode]
		}

		taskID := identities.taskIDs[item.TaskId]
		switch {
		case missionID != "" && taskID != "" && missionID != taskID:
			task.State = yunBeiUnsupported
			task.Conflicted = true
			snapshot.Conflicts = append(snapshot.Conflicts, fmt.Errorf(
				"missionCode %q and taskId %d map to different local tasks",
				task.MissionCode,
				item.TaskId,
			))
		case missionID != "":
			task.ID = missionID
		case taskID != "":
			task.ID = taskID
		}

		if task.ID == "" {
			task.State = yunBeiUnsupported
		}

		index := len(snapshot.Tasks)
		snapshot.Tasks = append(snapshot.Tasks, task)
		indexYunBeiStableKey(snapshot, byUserTaskID, "userTaskId", item.UserTaskId, index)

		for detailIndex := range item.TaskPointDetail {
			indexYunBeiStableKey(
				snapshot,
				byUserTaskID,
				"userMissionId",
				item.TaskPointDetail[detailIndex].UserMissionId,
				index,
			)
		}

		indexYunBeiStableKey(snapshot, byTaskID, "taskId", item.TaskId, index)

		if task.MissionCode != "" {
			if previous, ok := byMissionCode[task.MissionCode]; ok {
				snapshot.Tasks[previous].State = yunBeiUnsupported
				snapshot.Tasks[previous].Conflicted = true
				task.State = yunBeiUnsupported
				task.Conflicted = true
				snapshot.Conflicts = append(snapshot.Conflicts,
					fmt.Errorf("missionCode %q is duplicated", task.MissionCode))
			} else {
				byMissionCode[task.MissionCode] = index
			}
		}

		if task.ID != "" {
			if previous, ok := byLocalID[task.ID]; ok {
				snapshot.Tasks[previous].State = yunBeiUnsupported
				snapshot.Tasks[previous].Conflicted = true
				task.State = yunBeiUnsupported
				task.Conflicted = true
				snapshot.Conflicts = append(snapshot.Conflicts,
					fmt.Errorf("local task ID %q matches multiple V3 records", task.ID))
			} else {
				byLocalID[task.ID] = index
			}
		}
	}

	correlation := &yunBeiCorrelation{
		snapshot:           snapshot,
		byUserTaskID:       byUserTaskID,
		byTaskID:           byTaskID,
		seenTodoUserTaskID: make(map[int64]struct{}, len(todos)),
		seenTodoTaskID:     make(map[int64]struct{}, len(todos)),
	}

	for todoIndex := range todos {
		todo := &todos[todoIndex]

		index, conflict := correlation.todo(todo)
		if conflict != nil {
			snapshot.Conflicts = append(snapshot.Conflicts, conflict)
			snapshot.UnmatchedTodos = append(snapshot.UnmatchedTodos, *todo)
			continue
		}

		task := snapshot.Tasks[index]
		if task.Conflicted {
			snapshot.UnmatchedTodos = append(snapshot.UnmatchedTodos, *todo)
			continue
		}

		if task.Claim != nil {
			task.State = yunBeiUnsupported
			task.Conflicted = true

			snapshot.Conflicts = append(snapshot.Conflicts,
				fmt.Errorf("task userTaskId=%d taskId=%d has multiple todo records", todo.UserTaskId, todo.TaskId))
			snapshot.UnmatchedTodos = append(snapshot.UnmatchedTodos, *todo)
			continue
		}

		if task.State == yunBeiClaimed && !todo.Completed {
			task.State = yunBeiUnsupported
			task.Conflicted = true

			snapshot.Conflicts = append(snapshot.Conflicts, fmt.Errorf(
				"completed V3 task userTaskId=%d taskId=%d has a pending todo record",
				todo.UserTaskId,
				todo.TaskId,
			))
			snapshot.UnmatchedTodos = append(snapshot.UnmatchedTodos, *todo)
			continue
		}

		task.Claim = &yunBeiClaimRef{
			Period:      todo.Period,
			UserTaskID:  todo.UserTaskId,
			DepositCode: todo.DepositCode,
		}
		if todo.Completed {
			task.State = yunBeiCompletedUnclaimed
		} else if task.ID != "" {
			task.State = yunBeiPending
		}
	}

	return snapshot
}

func indexYunBeiStableKey(snapshot *yunBeiSnapshot, index map[int64]int, name string, key int64, taskIndex int) {
	if key == 0 {
		return
	}

	if key < 0 {
		snapshot.Tasks[taskIndex].State = yunBeiUnsupported
		snapshot.Tasks[taskIndex].Conflicted = true
		snapshot.Conflicts = append(snapshot.Conflicts, fmt.Errorf("%s %d is invalid", name, key))
		return
	}

	if previous, ok := index[key]; ok {
		if previous == taskIndex {
			return
		}

		if previous >= 0 {
			snapshot.Tasks[previous].State = yunBeiUnsupported
			snapshot.Tasks[previous].Conflicted = true
		}

		snapshot.Tasks[taskIndex].State = yunBeiUnsupported
		snapshot.Tasks[taskIndex].Conflicted = true
		index[key] = -1
		snapshot.Conflicts = append(snapshot.Conflicts, fmt.Errorf("%s %d is duplicated", name, key))
		return
	}

	index[key] = taskIndex
}

func (c *yunBeiCorrelation) todo(todo *eapi.YunBeiTaskTodoRespData) (int, error) {
	if todo.UserTaskId < 0 || todo.TaskId < 0 {
		return 0, fmt.Errorf("todo record has invalid userTaskId=%d taskId=%d", todo.UserTaskId, todo.TaskId)
	}

	if todo.UserTaskId == 0 && todo.TaskId == 0 {
		return 0, errors.New("todo record has no stable userTaskId or taskId")
	}

	if todo.UserTaskId != 0 {
		if _, duplicate := c.seenTodoUserTaskID[todo.UserTaskId]; duplicate {
			if index, ok := c.byUserTaskID[todo.UserTaskId]; ok && index >= 0 {
				c.snapshot.Tasks[index].State = yunBeiUnsupported
				c.snapshot.Tasks[index].Conflicted = true
			}
			return 0, fmt.Errorf("todo userTaskId %d is duplicated", todo.UserTaskId)
		}

		c.seenTodoUserTaskID[todo.UserTaskId] = struct{}{}
	}

	if todo.TaskId != 0 {
		if _, duplicate := c.seenTodoTaskID[todo.TaskId]; duplicate {
			if index, ok := c.byTaskID[todo.TaskId]; ok && index >= 0 {
				c.snapshot.Tasks[index].State = yunBeiUnsupported
				c.snapshot.Tasks[index].Conflicted = true
			}
			return 0, fmt.Errorf("todo taskId %d is duplicated", todo.TaskId)
		}

		c.seenTodoTaskID[todo.TaskId] = struct{}{}
	}

	userIndex, userFound := c.byUserTaskID[todo.UserTaskId]

	taskIndex, taskFound := c.byTaskID[todo.TaskId]
	if userFound && userIndex < 0 {
		return 0, fmt.Errorf("todo userTaskId %d matches duplicate V3 records", todo.UserTaskId)
	}

	if taskFound && taskIndex < 0 {
		return 0, fmt.Errorf("todo taskId %d matches duplicate V3 records", todo.TaskId)
	}

	if todo.UserTaskId != 0 {
		if !userFound {
			return 0, fmt.Errorf("todo userTaskId %d has no matching V3 record", todo.UserTaskId)
		}

		if todo.TaskId != 0 && taskFound && taskIndex != userIndex {
			c.snapshot.Tasks[userIndex].State = yunBeiUnsupported
			c.snapshot.Tasks[userIndex].Conflicted = true
			c.snapshot.Tasks[taskIndex].State = yunBeiUnsupported
			c.snapshot.Tasks[taskIndex].Conflicted = true
			return 0, fmt.Errorf("todo userTaskId %d and taskId %d match different V3 records", todo.UserTaskId, todo.TaskId)
		}
		return userIndex, nil
	}

	if !taskFound {
		return 0, fmt.Errorf("todo taskId %d has no matching V3 record", todo.TaskId)
	}
	return taskIndex, nil
}

func (runtime *yunBeiRuntime) loadTaskSnapshot(ctx context.Context) (*yunBeiSnapshot, error) {
	list, err := runtime.weapi.YunBeiTaskListV3(ctx, &weapi.YunBeiTaskListV3Req{})
	if err != nil {
		return nil, fmt.Errorf("YunBeiTaskListV3: %w", err)
	}

	if list.Code != 200 {
		return nil, fmt.Errorf("YunBeiTaskListV3: code=%d message=%q", list.Code, list.Message)
	}

	if len(list.Data.UnknownBucket) != 0 {
		names := make([]string, 0, len(list.Data.UnknownBucket))
		for name := range list.Data.UnknownBucket {
			names = append(names, name)
		}

		slices.Sort(names)
		return nil, fmt.Errorf("YunBeiTaskListV3: non-empty untyped buckets: %s", strings.Join(names, ", "))
	}

	todo, err := runtime.eapi.YunBeiTaskTodo(ctx, &eapi.YunBeiTaskTodoReq{})
	if err != nil {
		return nil, fmt.Errorf("YunBeiTaskTodo: %w", err)
	}

	if todo.Code != 200 {
		return nil, fmt.Errorf("YunBeiTaskTodo: code=%d message=%q", todo.Code, todo.Message)
	}

	snapshot := correlateYunBeiTasks(list.Data.Normal.List, todo.Data, yunBeiTaskIdentities())
	return snapshot, nil
}

func (snapshot *yunBeiSnapshot) task(id yunBeiTaskID) *yunBeiTaskSnapshot {
	for _, task := range snapshot.Tasks {
		if task.ID == id {
			return task
		}
	}
	return nil
}

func yunBeiTaskProgress(item *weapi.YunBeiTaskListV3Item) (int64, int64, bool) {
	if len(item.TaskPointDetail) == 1 && item.TaskPointDetail[0].SumTarget > 0 {
		return item.TaskPointDetail[0].ProgressRate, item.TaskPointDetail[0].SumTarget, true
	}

	if item.TargetPoint > 0 {
		return item.CompletedPoint, item.TargetPoint, true
	}
	return 0, 0, false
}

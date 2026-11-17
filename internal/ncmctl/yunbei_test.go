// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	projectlog "github.com/chaunsin/netease-cloud-music/pkg/log"
)

func TestNormalizeYunBeiTasksPreservesFirstOccurrence(t *testing.T) {
	got, err := normalizeYunBeiTasks([]string{"claim, sign", "claim", "publish-note"}, false)
	require.NoError(t, err)
	assert.Equal(t, []yunBeiTaskID{yunBeiClaim, yunBeiSign, yunBeiPublishNote}, got)
}

func TestNormalizeYunBeiTasksRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{name: "empty", input: []string{"sign,"}, want: "empty value"},
		{name: "blank", input: []string{"   "}, want: "empty value"},
		{name: "unknown", input: []string{"not-a-task"}, want: "unknown YunBei task"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeYunBeiTasks(tt.input, false)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestValidateYunBeiOptsSelectionAndPublishScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts YunBeiOpts
		want string
	}{
		{name: "no selection", want: "no YunBei task selected"},
		{name: "task and all", opts: YunBeiOpts{Tasks: []string{"sign"}, All: true}, want: "mutually exclusive"},
		{name: "publish options outside publish", opts: YunBeiOpts{Tasks: []string{"sign"}, Title: "title"}, want: "require publish-note"},
		{name: "blank title", opts: YunBeiOpts{Tasks: []string{"publish-note"}, Title: "  "}, want: "title must not be empty"},
		{name: "short message", opts: YunBeiOpts{Tasks: []string{"publish-note"}, Message: "不足二十个字"}, want: "at least 20 Unicode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := validateYunBeiOpts(&tt.opts, false)
			require.ErrorContains(t, err, tt.want)
		})
	}

	selected, err := validateYunBeiOpts(&YunBeiOpts{Tasks: []string{"sign"}}, false)
	require.NoError(t, err)
	assert.Equal(t, []yunBeiTaskID{yunBeiSign}, selected)

	selected, err = validateYunBeiOpts(&YunBeiOpts{All: true}, false)
	require.NoError(t, err)
	require.Len(t, selected, 12)
}

// TestYunBeiNoteMessageMeetsServerLengthRule 钉住 publish-note 的正文长度下限:
// 服务端 V3 的 taskDescription 是「发布一条≥20字的图文笔记」, 正文短于此值时动态照发、
// 返回正动态 ID, 但服务端不创建用户任务实例, 任务进度永远不推进。
func TestYunBeiNoteMessageMeetsServerLengthRule(t *testing.T) {
	t.Parallel()

	_, builtin := noteText("", "", yunBeiNoteMessage, dailySong{name: "song"})
	assert.GreaterOrEqual(t, utf8.RuneCountInString(strings.TrimSpace(builtin)), yunBeiNoteMinMessage)

	tooShort := strings.Repeat("字", yunBeiNoteMinMessage-1)
	_, err := validateYunBeiOpts(&YunBeiOpts{Tasks: []string{"publish-note"}, Message: tooShort}, false)
	require.ErrorContains(t, err, "at least 20 Unicode")

	exact := strings.Repeat("字", yunBeiNoteMinMessage)
	_, err = validateYunBeiOpts(&YunBeiOpts{Tasks: []string{"publish-note"}, Message: exact}, false)
	require.NoError(t, err)
}

func TestValidateYunBeiImageBoundaries(t *testing.T) {
	dir := t.TempDir()
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	exact := filepath.Join(dir, "exact.png")
	require.NoError(t, os.WriteFile(exact, pngHeader, 0o600))
	require.NoError(t, os.Truncate(exact, dailySongCoverLimit))
	require.NoError(t, validateYunBeiImage(exact))

	over := filepath.Join(dir, "over.png")
	require.NoError(t, os.WriteFile(over, pngHeader, 0o600))
	require.NoError(t, os.Truncate(over, dailySongCoverLimit+1))
	require.ErrorContains(t, validateYunBeiImage(over), "exceeds")

	empty := filepath.Join(dir, "empty.png")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))
	require.ErrorContains(t, validateYunBeiImage(empty), "non-empty regular file")

	link := filepath.Join(dir, "link.png")
	require.NoError(t, os.Symlink(exact, link))
	require.ErrorContains(t, validateYunBeiImage(link), "symlink")

	wrongExtension := filepath.Join(dir, "wrong.jpg")
	require.NoError(t, os.WriteFile(wrongExtension, pngHeader, 0o600))
	require.ErrorContains(t, validateYunBeiImage(wrongExtension), "does not match")

	notImage := filepath.Join(dir, "text.png")
	require.NoError(t, os.WriteFile(notImage, []byte("not an image"), 0o600))
	require.ErrorContains(t, validateYunBeiImage(notImage), "supported image types")
}

func TestYunBeiStatusRejectsEveryExplicitExecutionFlag(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "task", value: "sign"},
		{name: "all", value: "false"},
		{name: "title", value: ""},
		{name: "message", value: ""},
		{name: "image", value: ""},
		{name: "delete", value: "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := NewYunBei(&Root{}, nil)
			command.status = true
			require.NoError(t, command.cmd.Flags().Set(tt.name, tt.value))
			require.ErrorContains(t, command.validate(), "status cannot be combined with --"+tt.name)
		})
	}
}

func TestYunBeiValidationFailsBeforeClientCreation(t *testing.T) {
	command := NewYunBei(&Root{}, nil)
	require.ErrorContains(t, command.execute(context.Background()), "no YunBei task selected")

	command = NewYunBei(&Root{}, nil)
	command.opts.Tasks = []string{"unknown"}
	require.ErrorContains(t, command.execute(context.Background()), "unknown YunBei task")
}

func TestYunBeiRegistryExecutionOrder(t *testing.T) {
	got := make([]yunBeiTaskID, 0, len(yunBeiRegistry))
	for _, def := range yunBeiRegistry {
		got = append(got, def.ID)
	}

	assert.Equal(t, []yunBeiTaskID{
		yunBeiSign,
		yunBeiReserve,
		yunBeiViewVIPCenter,
		yunBeiLikeComment,
		yunBeiFollowArtist,
		yunBeiCollectSong,
		yunBeiLikeSong,
		yunBeiPublishNote,
		yunBeiShareSong,
		yunBeiListenIndie,
		yunBeiPlayDailyRecommend,
		yunBeiClaim,
	}, got)

	missionCodes := make(map[string]yunBeiTaskID)
	taskIDs := make(map[int64]yunBeiTaskID)

	for _, def := range yunBeiRegistry {
		assert.NotNil(t, def.Run, string(def.ID))

		for _, missionCode := range def.MissionCodes {
			require.NotEmpty(t, strings.TrimSpace(missionCode), string(def.ID))
			_, duplicate := missionCodes[missionCode]
			require.False(t, duplicate, "duplicate missionCode %q", missionCode)
			missionCodes[missionCode] = def.ID
		}

		for _, taskID := range def.TaskIDs {
			require.Positive(t, taskID, string(def.ID))
			_, duplicate := taskIDs[taskID]
			require.False(t, duplicate, "duplicate taskId %d", taskID)
			taskIDs[taskID] = def.ID
		}
	}

	identities := yunBeiTaskIdentities()
	assert.Equal(t, missionCodes, identities.missionCodes)
	assert.Equal(t, taskIDs, identities.taskIDs)
}

func TestCorrelateYunBeiTasksUsesUserTaskIDBeforeTaskID(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{
		{TaskId: 10, UserTaskId: 100, TaskName: "first", ExtInfoMap: &weapi.YunBeiTaskExtInfo{MissionCode: "first-code"}},
		{TaskId: 20, UserTaskId: 200, TaskName: "second", ExtInfoMap: &weapi.YunBeiTaskExtInfo{MissionCode: "second-code"}},
	}
	todos := []eapi.YunBeiTaskTodoRespData{{
		Completed: true, TaskId: 10, UserTaskId: 200, Period: 1, DepositCode: 2,
	}}

	snapshot := correlateYunBeiTasks(items, todos, yunBeiTaskIdentityIndex{missionCodes: map[string]yunBeiTaskID{
		"first-code":  yunBeiLikeComment,
		"second-code": yunBeiFollowArtist,
	}})
	require.Len(t, snapshot.Conflicts, 1)
	require.ErrorContains(t, snapshot.Conflicts[0], "match different V3 records")
	assert.Equal(t, yunBeiUnsupported, snapshot.Tasks[0].State)
	assert.Equal(t, yunBeiUnsupported, snapshot.Tasks[1].State)
}

func TestCorrelateYunBeiTasksFallsBackToTaskID(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{{
		TaskId: 10, TaskName: "fixed", ExtInfoMap: &weapi.YunBeiTaskExtInfo{MissionCode: "fixed-code"},
	}}
	todos := []eapi.YunBeiTaskTodoRespData{{
		Completed: true, TaskId: 10, Period: 1, DepositCode: 2,
	}}

	snapshot := correlateYunBeiTasks(items, todos, yunBeiTaskIdentityIndex{
		missionCodes: map[string]yunBeiTaskID{"fixed-code": yunBeiLikeSong},
	})
	require.Empty(t, snapshot.Conflicts)
	require.Len(t, snapshot.Tasks, 1)
	assert.Equal(t, yunBeiLikeSong, snapshot.Tasks[0].ID)
	assert.Equal(t, yunBeiCompletedUnclaimed, snapshot.Tasks[0].State)
	require.NotNil(t, snapshot.Tasks[0].Claim)
}

func TestCorrelateYunBeiTasksRejectsMissingAndDuplicateKeys(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{
		{TaskId: 10, UserTaskId: 100, TaskName: "first", ExtInfoMap: &weapi.YunBeiTaskExtInfo{MissionCode: "same-code"}},
		{TaskId: 20, UserTaskId: 100, TaskName: "second", ExtInfoMap: &weapi.YunBeiTaskExtInfo{MissionCode: "same-code"}},
	}
	todos := []eapi.YunBeiTaskTodoRespData{
		{Completed: true},
		{Completed: true, UserTaskId: 100},
	}

	snapshot := correlateYunBeiTasks(items, todos, yunBeiTaskIdentityIndex{
		missionCodes: map[string]yunBeiTaskID{"same-code": yunBeiLikeSong},
	})
	require.Len(t, snapshot.Conflicts, 5)
	require.Len(t, snapshot.Tasks, 2)
	require.Len(t, snapshot.UnmatchedTodos, 2)
	assert.Equal(t, yunBeiUnsupported, snapshot.Tasks[0].State)
	assert.Equal(t, yunBeiUnsupported, snapshot.Tasks[1].State)
}

func TestCorrelateYunBeiTasksUsesPointDetailMissionID(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{{
		TaskId:     10,
		UserTaskId: 100,
		TaskName:   "fixed task",
		TaskPointDetail: []weapi.YunBeiTaskPointDetail{{
			UserMissionId: 101,
		}},
	}}
	todos := []eapi.YunBeiTaskTodoRespData{{
		Completed: true, UserTaskId: 101, Period: 1, DepositCode: 2,
	}}

	snapshot := correlateYunBeiTasks(items, todos, yunBeiTaskIdentityIndex{})
	require.Empty(t, snapshot.Conflicts)
	require.Empty(t, snapshot.UnmatchedTodos)
	require.Len(t, snapshot.Tasks, 1)
	assert.Equal(t, yunBeiCompletedUnclaimed, snapshot.Tasks[0].State)
	require.NotNil(t, snapshot.Tasks[0].Claim)
	assert.Equal(t, int64(101), snapshot.Tasks[0].Claim.UserTaskID)
}

func TestCorrelateYunBeiTasksUsesEvidenceBackedTaskID(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{{TaskId: 6758460, TaskName: "fixed task"}}

	snapshot := correlateYunBeiTasks(items, nil, yunBeiTaskIdentities())
	require.Empty(t, snapshot.Conflicts)
	require.Len(t, snapshot.Tasks, 1)
	assert.Equal(t, yunBeiViewVIPCenter, snapshot.Tasks[0].ID)
	assert.Equal(t, yunBeiPending, snapshot.Tasks[0].State)
}

func TestYunBeiTaskProgressUsesOnlyUnambiguousFields(t *testing.T) {
	current, target, ok := yunBeiTaskProgress(&weapi.YunBeiTaskListV3Item{
		CompletedPoint: 9,
		TargetPoint:    10,
		TaskPointDetail: []weapi.YunBeiTaskPointDetail{{
			ProgressRate: 3,
			SumTarget:    5,
		}},
	})
	assert.True(t, ok)
	assert.Equal(t, int64(3), current)
	assert.Equal(t, int64(5), target)

	current, target, ok = yunBeiTaskProgress(&weapi.YunBeiTaskListV3Item{
		CompletedPoint: 9,
		TargetPoint:    10,
		TaskPointDetail: []weapi.YunBeiTaskPointDetail{
			{ProgressRate: 3, SumTarget: 5},
			{ProgressRate: 1, SumTarget: 2},
		},
	})
	assert.True(t, ok)
	assert.Equal(t, int64(9), current)
	assert.Equal(t, int64(10), target)

	_, _, ok = yunBeiTaskProgress(&weapi.YunBeiTaskListV3Item{})
	assert.False(t, ok)
}

func TestValidateYunBeiActionableTask(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, validateYunBeiActionableTask(nil), "snapshot is missing")
	require.ErrorContains(t, validateYunBeiActionableTask(&yunBeiTaskSnapshot{
		ID: yunBeiLikeComment, Current: 0, Target: 0,
	}), "not actionable")
	require.ErrorContains(t, validateYunBeiActionableTask(&yunBeiTaskSnapshot{
		ID: yunBeiLikeComment, Current: 1, Target: 1,
	}), "not actionable")
	require.NoError(t, validateYunBeiActionableTask(&yunBeiTaskSnapshot{
		ID: yunBeiLikeComment, Current: 0, Target: 1,
	}))
}

func TestCorrelateYunBeiTasksRejectsMultipleRecordsForLocalID(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{
		{TaskId: 10, UserTaskId: 100, ExtInfoMap: &weapi.YunBeiTaskExtInfo{MissionCode: "first-code"}},
		{TaskId: 20, UserTaskId: 200, ExtInfoMap: &weapi.YunBeiTaskExtInfo{MissionCode: "second-code"}},
	}
	missionIDs := map[string]yunBeiTaskID{
		"first-code":  yunBeiLikeSong,
		"second-code": yunBeiLikeSong,
	}

	snapshot := correlateYunBeiTasks(items, nil, yunBeiTaskIdentityIndex{missionCodes: missionIDs})
	require.Len(t, snapshot.Conflicts, 1)
	require.ErrorContains(t, snapshot.Conflicts[0], "matches multiple V3 records")
	assert.Equal(t, yunBeiUnsupported, snapshot.Tasks[0].State)
	assert.Equal(t, yunBeiUnsupported, snapshot.Tasks[1].State)
}

func TestCorrelateYunBeiTasksRejectsThreeDuplicateStableKeysWithoutPanic(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{
		{TaskId: 10, UserTaskId: 100},
		{TaskId: 20, UserTaskId: 100},
		{TaskId: 30, UserTaskId: 100},
	}

	var snapshot *yunBeiSnapshot

	require.NotPanics(t, func() {
		snapshot = correlateYunBeiTasks(items, nil, yunBeiTaskIdentityIndex{})
	})
	require.Len(t, snapshot.Tasks, 3)
	require.Len(t, snapshot.Conflicts, 2)

	for _, task := range snapshot.Tasks {
		assert.Equal(t, yunBeiUnsupported, task.State)
	}
}

func TestCorrelateYunBeiTasksDoesNotReviveConflictingTaskFromTodo(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{
		{TaskId: 10, UserTaskId: 100, ExtInfoMap: &weapi.YunBeiTaskExtInfo{MissionCode: "first-code"}},
		{TaskId: 20, UserTaskId: 200, ExtInfoMap: &weapi.YunBeiTaskExtInfo{MissionCode: "second-code"}},
	}
	todos := []eapi.YunBeiTaskTodoRespData{{UserTaskId: 100, Period: 1}}

	snapshot := correlateYunBeiTasks(items, todos, yunBeiTaskIdentityIndex{missionCodes: map[string]yunBeiTaskID{
		"first-code":  yunBeiLikeSong,
		"second-code": yunBeiLikeSong,
	}})
	require.Len(t, snapshot.Conflicts, 1)
	require.Len(t, snapshot.UnmatchedTodos, 1)

	for _, task := range snapshot.Tasks {
		assert.Equal(t, yunBeiUnsupported, task.State)
		assert.Nil(t, task.Claim)
	}
}

func TestCorrelateYunBeiTasksMarksDuplicateTodosUnsafe(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{{TaskId: 6758460, UserTaskId: 100}}
	todos := []eapi.YunBeiTaskTodoRespData{
		{Completed: true, UserTaskId: 100, Period: 1, DepositCode: 2},
		{Completed: true, UserTaskId: 100, Period: 1, DepositCode: 2},
	}

	snapshot := correlateYunBeiTasks(items, todos, yunBeiTaskIdentities())
	require.Len(t, snapshot.Conflicts, 1)
	require.ErrorContains(t, snapshot.Conflicts[0], "todo userTaskId 100 is duplicated")
	require.Len(t, snapshot.UnmatchedTodos, 1)
	require.Len(t, snapshot.Tasks, 1)
	assert.True(t, snapshot.Tasks[0].Conflicted)
	assert.Equal(t, yunBeiUnsupported, snapshot.Tasks[0].State)
}

func TestCorrelateYunBeiTasksKeepsSecondTodoForOneV3TaskUnmatched(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{{
		TaskId: 6758460, UserTaskId: 100,
		TaskPointDetail: []weapi.YunBeiTaskPointDetail{{UserMissionId: 101}},
	}}
	todos := []eapi.YunBeiTaskTodoRespData{
		{Completed: true, UserTaskId: 100, Period: 1, DepositCode: 2},
		{Completed: true, UserTaskId: 101, Period: 1, DepositCode: 3},
	}

	snapshot := correlateYunBeiTasks(items, todos, yunBeiTaskIdentities())
	require.Len(t, snapshot.Conflicts, 1)
	require.ErrorContains(t, snapshot.Conflicts[0], "multiple todo records")
	require.Len(t, snapshot.UnmatchedTodos, 1)
	assert.Equal(t, int64(101), snapshot.UnmatchedTodos[0].UserTaskId)
	require.Len(t, snapshot.Tasks, 1)
	assert.True(t, snapshot.Tasks[0].Conflicted)
	assert.Equal(t, yunBeiUnsupported, snapshot.Tasks[0].State)
}

func TestCorrelateYunBeiTasksRejectsCompletedTaskWithPendingTodo(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{{
		TaskId: 6758460, UserTaskId: 100, Completed: true,
	}}
	todos := []eapi.YunBeiTaskTodoRespData{{UserTaskId: 100, Period: 1}}

	snapshot := correlateYunBeiTasks(items, todos, yunBeiTaskIdentities())
	require.Len(t, snapshot.Conflicts, 1)
	require.ErrorContains(t, snapshot.Conflicts[0], "completed V3 task")
	require.Len(t, snapshot.UnmatchedTodos, 1)
	require.Len(t, snapshot.Tasks, 1)
	assert.Equal(t, yunBeiUnsupported, snapshot.Tasks[0].State)
	assert.Nil(t, snapshot.Tasks[0].Claim)
}

func TestCorrelateYunBeiTasksRejectsNegativeStableKeys(t *testing.T) {
	items := []weapi.YunBeiTaskListV3Item{{
		TaskId: -2, UserTaskId: -1,
		ExtInfoMap:      &weapi.YunBeiTaskExtInfo{MissionCode: "fixed-code"},
		TaskPointDetail: []weapi.YunBeiTaskPointDetail{{UserMissionId: -3}},
	}}
	todos := []eapi.YunBeiTaskTodoRespData{{UserTaskId: -1}}

	snapshot := correlateYunBeiTasks(items, todos, yunBeiTaskIdentityIndex{
		missionCodes: map[string]yunBeiTaskID{"fixed-code": yunBeiLikeSong},
	})
	require.Len(t, snapshot.Conflicts, 4)
	require.Len(t, snapshot.UnmatchedTodos, 1)
	require.Len(t, snapshot.Tasks, 1)
	assert.True(t, snapshot.Tasks[0].Conflicted)
	assert.Equal(t, yunBeiUnsupported, snapshot.Tasks[0].State)
}

// TestYunBeiTasksWithoutEvidenceIdentityStayUnreachable 用生产的 yunBeiTaskIdentities 与真实
// correlateYunBeiTasks 驱动轮次编排（而不是测试里手搓的快照），断言 spec-004 的取证边界：
// MissionCodes 未取证时, like-comment 与 collect-song 从任何 V3 响应都拿不到本地标识, 只能被判为
// unavailable 并在处理器之前安全跳过。G1/Q4 补齐身份取证后, 本用例应改为断言它们可达。
func TestYunBeiTasksWithoutEvidenceIdentityStayUnreachable(t *testing.T) {
	identities := yunBeiTaskIdentities()
	assert.Empty(t, identities.missionCodes, "missionCode 取证完成后应移走本用例的空值断言")

	candidates := []int64{0, -1, 1, 3801301, 4444444, 1<<40 - 1}
	for _, def := range yunBeiRegistry {
		candidates = append(candidates, def.TaskIDs...)
	}

	mapped := map[yunBeiTaskID]struct{}{}

	for _, taskID := range candidates {
		for _, missionCode := range []string{"", "likeCommentMission", "collectSongMission", "unknown-code"} {
			snapshot := correlateYunBeiTasks([]weapi.YunBeiTaskListV3Item{{
				TaskId:     taskID,
				UserTaskId: taskID + 1000,
				ExtInfoMap: &weapi.YunBeiTaskExtInfo{MissionCode: missionCode},
			}}, nil, identities)

			require.Len(t, snapshot.Tasks, 1)

			if id := snapshot.Tasks[0].ID; id != "" {
				mapped[id] = struct{}{}
			}
		}
	}

	assert.NotContains(t, mapped, yunBeiLikeComment)
	assert.NotContains(t, mapped, yunBeiCollectSong)

	command := NewYunBei(&Root{}, nil)

	var output bytes.Buffer

	command.cmd.SetOut(&output)

	items := make([]weapi.YunBeiTaskListV3Item, 0, len(yunBeiRegistry))

	for defIndex, def := range yunBeiRegistry {
		for _, taskID := range def.TaskIDs {
			items = append(items, weapi.YunBeiTaskListV3Item{
				TaskId:     taskID,
				UserTaskId: taskID + int64(defIndex),
				TaskName:   string(def.ID),
			})
		}
	}

	runtime := newYunBeiRuntime(command, nil, nil, 42)
	runtime.loadSnapshot = func(context.Context) (*yunBeiSnapshot, error) {
		return correlateYunBeiTasks(items, nil, yunBeiTaskIdentities()), nil
	}

	plan := &yunBeiRoundPlan{
		registry: yunBeiRegistry,
		selected: testYunBeiSelection(yunBeiLikeComment, yunBeiCollectSong),
		load:     runtime.loadSnapshot,
	}
	require.Empty(t, command.runYunBeiTaskRounds(context.Background(), runtime, plan))

	require.NotNil(t, runtime.results[yunBeiLikeComment])
	require.NotNil(t, runtime.results[yunBeiCollectSong])
	assert.Equal(t, yunBeiUnavailable, runtime.results[yunBeiLikeComment].State)
	assert.Equal(t, yunBeiUnavailable, runtime.results[yunBeiCollectSong].State)
	assert.Empty(t, runtime.attempted, "处理器不得在未取证身份的任务上发起账号动作")
	assert.Contains(t, output.String(), "[like-comment] 今日无需执行")
	assert.Contains(t, output.String(), "[collect-song] 今日无需执行")
}

func TestYunBeiTaskRoundsCompleteOnThirdRound(t *testing.T) {
	command := NewYunBei(&Root{}, nil)

	var output bytes.Buffer
	command.cmd.SetOut(&output)
	runtime := newYunBeiRuntime(command, nil, nil, 42)

	progress := int64(0)
	writes := 0
	loads := 0
	definition := yunBeiTaskDef{
		ID:           yunBeiViewVIPCenter,
		Stage:        stageFast,
		MissionCodes: []string{"fixed-code"},
		Run: func(_ context.Context, _ *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
			writes++
			progress++

			snapshot.Current = progress
			if progress == 3 {
				snapshot.State = yunBeiCompletedUnclaimed
			}
			return nil
		},
	}
	plan := &yunBeiRoundPlan{
		registry: []yunBeiTaskDef{definition},
		selected: testYunBeiSelection(yunBeiViewVIPCenter),
		load: func(context.Context) (*yunBeiSnapshot, error) {
			loads++
			return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
				ID: yunBeiViewVIPCenter, Current: progress, Target: 3, State: yunBeiPending,
			}}}, nil
		},
	}

	assert.Empty(t, command.runYunBeiTaskRounds(context.Background(), runtime, plan))
	assert.Equal(t, 3, loads)
	assert.Equal(t, 3, writes)
	assert.Equal(t, 3, runtime.round)
	assert.Equal(t, yunBeiCompletedUnclaimed, runtime.results[yunBeiViewVIPCenter].State)
}

func TestYunBeiTaskRoundsDoNotRerunCompletedTask(t *testing.T) {
	command := NewYunBei(&Root{}, nil)
	runtime := newYunBeiRuntime(command, nil, nil, 42)

	firstWrites := 0
	secondWrites := 0
	secondProgress := int64(0)
	registry := []yunBeiTaskDef{
		{
			ID:           yunBeiViewVIPCenter,
			Stage:        stageFast,
			MissionCodes: []string{"first-code"},
			Run: func(_ context.Context, _ *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
				firstWrites++
				snapshot.State = yunBeiCompletedUnclaimed
				return nil
			},
		},
		{
			ID:           yunBeiLikeComment,
			Stage:        stageFast,
			MissionCodes: []string{"second-code"},
			Run: func(_ context.Context, _ *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
				secondWrites++
				secondProgress++

				snapshot.Current = secondProgress
				if secondProgress == 2 {
					snapshot.State = yunBeiCompletedUnclaimed
				}
				return nil
			},
		},
	}
	plan := &yunBeiRoundPlan{
		registry: registry,
		selected: testYunBeiSelection(yunBeiViewVIPCenter, yunBeiLikeComment),
		load: func(context.Context) (*yunBeiSnapshot, error) {
			return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{
				{ID: yunBeiViewVIPCenter, State: yunBeiPending},
				{ID: yunBeiLikeComment, Current: secondProgress, Target: 2, State: yunBeiPending},
			}}, nil
		},
	}

	assert.Empty(t, command.runYunBeiTaskRounds(context.Background(), runtime, plan))
	assert.Equal(t, 1, firstWrites)
	assert.Equal(t, 2, secondWrites)
	assert.Equal(t, yunBeiCompletedUnclaimed, runtime.results[yunBeiViewVIPCenter].State)
	assert.Equal(t, yunBeiCompletedUnclaimed, runtime.results[yunBeiLikeComment].State)
}

func TestYunBeiTaskRoundsContinueAfterOneTaskFails(t *testing.T) {
	command := NewYunBei(&Root{}, nil)
	runtime := newYunBeiRuntime(command, nil, nil, 42)

	var order []yunBeiTaskID

	plan := &yunBeiRoundPlan{
		registry: []yunBeiTaskDef{
			{
				ID:    yunBeiViewVIPCenter,
				Stage: stageFast,
				Run: func(context.Context, *yunBeiRuntime, *yunBeiTaskSnapshot) error {
					order = append(order, yunBeiViewVIPCenter)
					return errors.New("first failed")
				},
			},
			{
				ID:    yunBeiLikeComment,
				Stage: stageFast,
				Run: func(_ context.Context, _ *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
					order = append(order, yunBeiLikeComment)
					snapshot.State = yunBeiClaimed
					return nil
				},
			},
		},
		selected: testYunBeiSelection(yunBeiViewVIPCenter, yunBeiLikeComment),
		load: func(context.Context) (*yunBeiSnapshot, error) {
			return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{
				{ID: yunBeiViewVIPCenter, Current: 0, Target: 1, State: yunBeiPending},
				{ID: yunBeiLikeComment, Current: 0, Target: 1, State: yunBeiPending},
			}}, nil
		},
	}

	errs := command.runYunBeiTaskRounds(context.Background(), runtime, plan)
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[0], "first failed")
	assert.Equal(t, []yunBeiTaskID{yunBeiViewVIPCenter, yunBeiLikeComment}, order)
	assert.Equal(t, yunBeiFailed, runtime.results[yunBeiViewVIPCenter].State)
	assert.Equal(t, yunBeiClaimed, runtime.results[yunBeiLikeComment].State)
}

func TestYunBeiDefinitionRestoresNewStateBeforeReturningActionError(t *testing.T) {
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), nil, nil, 42)
	restored := false
	definition := &yunBeiTaskDef{
		ID: yunBeiLikeSong,
		Run: func(_ context.Context, runtime *yunBeiRuntime, _ *yunBeiTaskSnapshot) error {
			runtime.addCompensation(
				yunBeiLikeSong,
				"song-unlike",
				"9",
				"",
				func(context.Context) error {
					restored = true
					return nil
				},
			)
			return errors.New("observe failed")
		},
	}
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiLikeSong, Current: 0, Target: 1, State: yunBeiPending}

	_, err := runtime.command.runYunBeiDefinition(context.Background(), runtime, definition, snapshot)
	require.ErrorContains(t, err, "observe failed")
	assert.True(t, restored)
	require.Len(t, runtime.compensations, 1)
	assert.True(t, runtime.compensations[0].Done)
}

func TestYunBeiTaskRoundsStopAfterNoProgress(t *testing.T) {
	command := NewYunBei(&Root{}, nil)
	runtime := newYunBeiRuntime(command, nil, nil, 42)

	writes := 0
	loads := 0
	plan := &yunBeiRoundPlan{
		registry: []yunBeiTaskDef{{
			ID:           yunBeiViewVIPCenter,
			Stage:        stageFast,
			MissionCodes: []string{"fixed-code"},
			Run: func(context.Context, *yunBeiRuntime, *yunBeiTaskSnapshot) error {
				writes++
				return nil
			},
		}},
		selected: testYunBeiSelection(yunBeiViewVIPCenter),
		load: func(context.Context) (*yunBeiSnapshot, error) {
			loads++
			return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
				ID: yunBeiViewVIPCenter, State: yunBeiPending,
			}}}, nil
		},
	}

	errs := command.runYunBeiTaskRounds(context.Background(), runtime, plan)
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[0], "task state did not change")
	assert.Equal(t, 1, loads)
	assert.Equal(t, 1, writes)
	assert.Equal(t, yunBeiUnconfirmed, runtime.results[yunBeiViewVIPCenter].State)
}

func TestYunBeiTaskRoundsMarkRoundLimitUnconfirmed(t *testing.T) {
	command := NewYunBei(&Root{}, nil)
	runtime := newYunBeiRuntime(command, nil, nil, 42)

	progress := int64(0)
	writes := 0
	plan := &yunBeiRoundPlan{
		registry: []yunBeiTaskDef{{
			ID:           yunBeiViewVIPCenter,
			Stage:        stageFast,
			MissionCodes: []string{"fixed-code"},
			Run: func(_ context.Context, _ *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
				writes++
				progress++
				snapshot.Current = progress
				return nil
			},
		}},
		selected: testYunBeiSelection(yunBeiViewVIPCenter),
		load: func(context.Context) (*yunBeiSnapshot, error) {
			return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
				ID: yunBeiViewVIPCenter, Current: progress, Target: 10, State: yunBeiPending,
			}}}, nil
		},
	}

	errs := command.runYunBeiTaskRounds(context.Background(), runtime, plan)
	require.Len(t, errs, 1)
	require.ErrorContains(t, errs[0], "remained incomplete after 3 rounds")
	assert.Equal(t, yunBeiMaxRounds, writes)
	assert.Equal(t, yunBeiUnconfirmed, runtime.results[yunBeiViewVIPCenter].State)
}

func TestYunBeiTaskRoundsSkipMissingServerTask(t *testing.T) {
	command := NewYunBei(&Root{}, nil)
	runtime := newYunBeiRuntime(command, nil, nil, 42)

	writes := 0
	plan := &yunBeiRoundPlan{
		registry: []yunBeiTaskDef{{
			ID:           yunBeiViewVIPCenter,
			Stage:        stageFast,
			MissionCodes: []string{"fixed-code"},
			Run: func(context.Context, *yunBeiRuntime, *yunBeiTaskSnapshot) error {
				writes++
				return nil
			},
		}},
		selected: testYunBeiSelection(yunBeiViewVIPCenter),
		load: func(context.Context) (*yunBeiSnapshot, error) {
			return &yunBeiSnapshot{}, nil
		},
	}

	assert.Empty(t, command.runYunBeiTaskRounds(context.Background(), runtime, plan))
	assert.Zero(t, writes)
	assert.Equal(t, yunBeiUnavailable, runtime.results[yunBeiViewVIPCenter].State)
}

// TestYunBeiTaskRoundsSkipEvidenceGatedTask 锁定「取证未闭环就不执行」这条边界:
// 2026-11-03 实测 play-daily-recommend 的 startplay/play 上报全部返回成功却不计数
// (3801300 恒为 0/1800), 跑完整链路只换来几十分钟流量与一次 partial 非零退出。
// 因此这类任务必须归入 skipped、不得调用处理器, 直到对应问题取证关闭。
func TestYunBeiTaskRoundsSkipEvidenceGatedTask(t *testing.T) {
	command := NewYunBei(&Root{}, nil)
	runtime := newYunBeiRuntime(command, nil, nil, 42)

	writes := 0
	plan := &yunBeiRoundPlan{
		registry: []yunBeiTaskDef{{
			ID:          yunBeiViewVIPCenter,
			Stage:       stageFast,
			EvidenceGap: "计数来源未取证",
			Run: func(context.Context, *yunBeiRuntime, *yunBeiTaskSnapshot) error {
				writes++
				return nil
			},
		}},
		selected: testYunBeiSelection(yunBeiViewVIPCenter),
		load: func(context.Context) (*yunBeiSnapshot, error) {
			return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
				ID: yunBeiViewVIPCenter, Target: 1, State: yunBeiPending,
			}}}, nil
		},
	}

	assert.Empty(t, command.runYunBeiTaskRounds(context.Background(), runtime, plan))
	assert.Zero(t, writes)
	assert.Equal(t, yunBeiUnavailable, runtime.results[yunBeiViewVIPCenter].State)

	// 生产注册表里的门禁本身也要被钉住, 否则删掉标记就能静默恢复一条已证伪的写链路。
	var playDef *yunBeiTaskDef

	for index := range yunBeiRegistry {
		if yunBeiRegistry[index].ID == yunBeiPlayDailyRecommend {
			playDef = &yunBeiRegistry[index]
		}
	}

	require.NotNil(t, playDef)
	assert.NotEmpty(t, playDef.EvidenceGap)
}

func TestObserveYunBeiTaskConfirmsOnEachRead(t *testing.T) {
	for updateAt := 1; updateAt <= yunBeiObserveReads; updateAt++ {
		t.Run(fmt.Sprintf("read-%d", updateAt), func(t *testing.T) {
			command := NewYunBei(&Root{}, nil)
			runtime := newYunBeiRuntime(command, nil, nil, 42)
			sleepCalls := 0
			runtime.sleep = func(context.Context, time.Duration, ...time.Duration) error {
				sleepCalls++
				return nil
			}

			reads := 0
			writes := 1
			after, confirmed, err := runtime.observe(
				context.Background(),
				&yunBeiTaskSnapshot{ID: yunBeiViewVIPCenter, Current: 0, State: yunBeiPending},
				yunBeiObserveWindow{reads: yunBeiObserveReads, pollDelay: time.Minute},
				func(context.Context) (*yunBeiTaskSnapshot, error) {
					reads++
					if reads == updateAt {
						return &yunBeiTaskSnapshot{
							ID: yunBeiViewVIPCenter, Current: 1, State: yunBeiCompletedUnclaimed,
						}, nil
					}
					return &yunBeiTaskSnapshot{ID: yunBeiViewVIPCenter, State: yunBeiPending}, nil
				},
			)
			require.NoError(t, err)
			assert.True(t, confirmed)
			require.NotNil(t, after)
			assert.Equal(t, yunBeiCompletedUnclaimed, after.State)
			assert.Equal(t, updateAt, reads)
			assert.Equal(t, updateAt-1, sleepCalls)
			assert.Equal(t, 1, writes)
		})
	}
}

func TestObserveYunBeiTaskStopsAfterThreeUnchangedReads(t *testing.T) {
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), nil, nil, 42)
	sleepCalls := 0
	runtime.sleep = func(context.Context, time.Duration, ...time.Duration) error {
		sleepCalls++
		return nil
	}

	reads := 0
	after, confirmed, err := runtime.observe(
		context.Background(),
		&yunBeiTaskSnapshot{ID: yunBeiViewVIPCenter, Current: 1, State: yunBeiPending},
		yunBeiObserveWindow{reads: yunBeiObserveReads, pollDelay: time.Minute},
		func(context.Context) (*yunBeiTaskSnapshot, error) {
			reads++
			return &yunBeiTaskSnapshot{ID: yunBeiViewVIPCenter, Current: 1, State: yunBeiPending}, nil
		},
	)
	require.NoError(t, err)
	assert.False(t, confirmed)
	require.NotNil(t, after)
	assert.Equal(t, yunBeiObserveReads, reads)
	assert.Equal(t, yunBeiObserveReads-1, sleepCalls)
}

func TestObserveYunBeiTaskRetriesTemporaryDisappearance(t *testing.T) {
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), nil, nil, 42)
	runtime.sleep = func(context.Context, time.Duration, ...time.Duration) error { return nil }
	reads := 0

	after, confirmed, err := runtime.observe(
		context.Background(),
		&yunBeiTaskSnapshot{ID: yunBeiViewVIPCenter, Current: 0, State: yunBeiPending},
		yunBeiObserveWindow{reads: yunBeiObserveReads, pollDelay: time.Minute},
		func(context.Context) (*yunBeiTaskSnapshot, error) {
			reads++
			if reads == 1 {
				return &yunBeiTaskSnapshot{ID: yunBeiViewVIPCenter, State: yunBeiUnavailable}, nil
			}
			return &yunBeiTaskSnapshot{
				ID: yunBeiViewVIPCenter, Current: 1, Target: 1, State: yunBeiCompletedUnclaimed,
			}, nil
		},
	)
	require.NoError(t, err)
	assert.True(t, confirmed)
	require.NotNil(t, after)
	assert.Equal(t, yunBeiCompletedUnclaimed, after.State)
	assert.Equal(t, 2, reads)
}

func TestObserveYunBeiTaskCancellationStopsPolling(t *testing.T) {
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), nil, nil, 42)
	ctx, cancel := context.WithCancel(context.Background())
	reads := 0

	_, confirmed, err := runtime.observe(
		ctx,
		&yunBeiTaskSnapshot{ID: yunBeiViewVIPCenter, State: yunBeiPending},
		yunBeiObserveWindow{reads: yunBeiObserveReads, pollDelay: time.Hour},
		func(context.Context) (*yunBeiTaskSnapshot, error) {
			reads++

			cancel()
			return &yunBeiTaskSnapshot{ID: yunBeiViewVIPCenter, State: yunBeiPending}, nil
		},
	)
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, confirmed)
	assert.Equal(t, 1, reads)
}

func TestYunBeiSignUsesIsolatedQueryPlan(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"sign":false}}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[]}}`),
	})

	var output bytes.Buffer

	command := NewYunBei(&Root{}, nil)
	command.cmd.SetOut(&output)
	command.selected = []yunBeiTaskID{yunBeiSign}

	require.NoError(t, command.run(context.Background(), client))
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/pointmall/user/sign",
		"/weapi/pointmall/user/sign/config",
	}, transport.paths)
	assert.Contains(t, output.String(), "云贝签到: 已签到")
}

func TestYunBeiSignAcceptsClaimedLotteryWithPersistentID(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"sign":false}}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[{"signDay":1,"baseLotteryId":123,"baseLotteryStatus":1,"extraLotteryId":0,"extraLotteryStatus":0}]}}`),
	})

	command := NewYunBei(&Root{}, nil)
	command.selected = []yunBeiTaskID{yunBeiSign}
	require.NoError(t, command.run(context.Background(), client))
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/pointmall/user/sign",
		"/weapi/pointmall/user/sign/config",
	}, transport.paths)
}

func TestYunBeiSignClaimsBaseLotteryAndRefreshes(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"sign":false}}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[{"signDay":1,"baseGrant":{"name":"10云贝"},"baseLotteryId":123,"baseLotteryStatus":0}]}}`),
		[]byte(`{"code":200,"data":true}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[{"signDay":1,"baseGrant":{"name":"10云贝"},"baseLotteryId":123,"baseLotteryStatus":1}]}}`),
	})

	var output bytes.Buffer

	command := NewYunBei(&Root{}, nil)
	command.cmd.SetOut(&output)
	command.selected = []yunBeiTaskID{yunBeiSign}
	require.NoError(t, command.run(context.Background(), client))
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/pointmall/user/sign",
		"/weapi/pointmall/user/sign/config",
		"/weapi/pointmall/user/sign/lottery/get",
		"/weapi/pointmall/user/sign/config",
	}, transport.paths)
	assert.Contains(t, output.String(), "第 1 天（奖励「10云贝」已领取）")
}

func TestYunBeiSignRechecksLaterClaimsAgainstLatestProgress(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"sign":false}}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[{"signDay":1,"baseLotteryId":101,"baseLotteryStatus":0},{"signDay":3,"baseLotteryId":303,"baseLotteryStatus":0}]}}`),
		[]byte(`{"code":200,"data":true}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[{"signDay":1,"baseLotteryId":101,"baseLotteryStatus":1},{"signDay":3,"baseLotteryId":303,"baseLotteryStatus":1}]}}`),
	})

	command := NewYunBei(&Root{}, nil)
	command.selected = []yunBeiTaskID{yunBeiSign}
	require.NoError(t, command.run(context.Background(), client))
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/pointmall/user/sign",
		"/weapi/pointmall/user/sign/config",
		"/weapi/pointmall/user/sign/lottery/get",
		"/weapi/pointmall/user/sign/config",
	}, transport.paths)
}

func TestYunBeiSignDoesNotGuessExtraLotteryClaimPath(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"sign":false}}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[{"signDay":28,"baseLotteryId":123,"baseLotteryStatus":1,"extraLotteryId":456,"extraLotteryStatus":0}]}}`),
	})

	command := NewYunBei(&Root{}, nil)
	command.selected = []yunBeiTaskID{yunBeiSign}
	require.ErrorContains(t, command.run(context.Background(), client), "extra lottery is claimable")
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/pointmall/user/sign",
		"/weapi/pointmall/user/sign/config",
	}, transport.paths)
}

func TestClassifyYunBeiLotteryState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    int64
		lotteryID int64
		want      yunBeiLotteryState
		wantErr   bool
	}{
		{name: "unavailable", status: 0, want: yunBeiLotteryUnavailable},
		{name: "claimable", status: 0, lotteryID: 1, want: yunBeiLotteryClaimable},
		{name: "claimed", status: 1, lotteryID: 1, want: yunBeiLotteryClaimed},
		{name: "claimed without ID", status: 1, wantErr: true},
		{name: "unknown status", status: 2, lotteryID: 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := classifyYunBeiLotteryState(tt.status, tt.lotteryID)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestYunBeiRegularTaskPreflightsCompletedStateBeforeHandler(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[{"taskId":3801300,"taskName":"fixed task","completed":true}]}}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[{"completed":false,"taskName":"unmatched","userTaskId":0}]}`)),
	})

	command := NewYunBei(&Root{}, nil)
	command.selected = []yunBeiTaskID{yunBeiPlayDailyRecommend}
	require.NoError(t, command.run(context.Background(), client))
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiReserveBooksAndConfirmsRefreshedState(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"type":"NO_PREV_NO_BOOK","reqId":"request-id"}}`)),
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true}`)),
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"type":"NO_PREV_BOOKED","reqId":"request-id"}}`)),
	})

	command := NewYunBei(&Root{}, nil)
	runtime := newYunBeiRuntime(command, weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiReserve, State: yunBeiPending}

	require.NoError(t, runYunBeiReserve(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	assert.Equal(t, []string{
		"/eapi/new/yunbei/activity/reserve/info",
		"/eapi/new/yunbei/activity/reserve/booked",
		"/eapi/new/yunbei/activity/reserve/info",
	}, transport.paths)
}

func TestYunBeiReserveAcceptsNoPreviousBookedWithoutWrite(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"type":"NO_PREV_BOOKED","reqId":"request-id"}}`)),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiReserve, State: yunBeiPending}

	require.NoError(t, runYunBeiReserve(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	assert.Equal(t, []string{"/eapi/new/yunbei/activity/reserve/info"}, transport.paths)
}

func TestYunBeiReserveDoesNotClaimWithoutServerToken(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"type":"PREV_BOOKED_UNCLAIMED","reqId":"request-id"}}`)),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	err := runYunBeiReserve(context.Background(), runtime, &yunBeiTaskSnapshot{ID: yunBeiReserve, State: yunBeiPending})
	require.ErrorContains(t, err, "missing reqId or checkToken")
	assert.Equal(t, []string{"/eapi/new/yunbei/activity/reserve/info"}, transport.paths)
}

func TestYunBeiReserveClaimsThenBooksUsingRefreshedRequestID(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"type":"PREV_BOOKED_UNCLAIMED","reqId":"old-request","checkToken":"claim-token"}}`)),
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"currentAmount":100}}`)),
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"type":"PREV_CLAIMED_NO_BOOKED","reqId":"new-request"}}`)),
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true}`)),
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"type":"PREV_CLAIMED_BOOKED","reqId":"new-request"}}`)),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiReserve, State: yunBeiPending}

	require.NoError(t, runYunBeiReserve(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	assert.Equal(t, []string{
		"/eapi/new/yunbei/activity/reserve/info",
		"/eapi/new/yunbei/activity/reserve/reward/receive",
		"/eapi/new/yunbei/activity/reserve/info",
		"/eapi/new/yunbei/activity/reserve/booked",
		"/eapi/new/yunbei/activity/reserve/info",
	}, transport.paths)
	assert.Contains(t, runtime.attempted, yunBeiActionKey{
		TaskID: yunBeiReserve, Kind: "claim", TargetID: "old-request",
	})
	assert.Contains(t, runtime.attempted, yunBeiActionKey{
		TaskID: yunBeiReserve, Kind: "book", TargetID: "new-request",
	})
}

func TestYunBeiReserveRejectsUnknownStatusWithoutWrite(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"type":"FUTURE_STATUS","reqId":"request-id"}}`)),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiReserve, State: yunBeiPending}

	require.ErrorContains(t, runYunBeiReserve(context.Background(), runtime, snapshot), "unknown reserve status")
	assert.Equal(t, yunBeiUnsupported, snapshot.State)
	assert.Equal(t, []string{"/eapi/new/yunbei/activity/reserve/info"}, transport.paths)
}

func TestYunBeiViewVIPCenterWaitsThenConfirms(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"code":200,"data":true,"message":""},"message":""}`),
		[]byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[{"taskId":6758460,"completed":true,"taskPointDetail":[{"progressRate":1,"sumTarget":1}]}]}}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	runtime.vipType = 2
	runtime.sleep = func(_ context.Context, duration time.Duration, _ ...time.Duration) error {
		assert.Equal(t, yunBeiVIPCenterViewTime, duration)
		return nil
	}
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiViewVIPCenter, TaskID: yunBeiServerTaskViewVIPCenter, Target: 1, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiViewVIPCenter(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	assert.Equal(t, []string{
		"/weapi/vipnewcenter/app/level/task/external",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiViewVIPCenterRequiresAcceptedBusinessResponse(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"code":200,"data":false,"message":""},"message":""}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	runtime.vipType = 2
	runtime.sleep = func(context.Context, time.Duration, ...time.Duration) error { return nil }
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiViewVIPCenter, TaskID: yunBeiServerTaskViewVIPCenter, Target: 1, State: yunBeiPending,
	}

	require.ErrorContains(t, runYunBeiViewVIPCenter(context.Background(), runtime, snapshot), "data=false")
	assert.Equal(t, []string{"/weapi/vipnewcenter/app/level/task/external"}, transport.paths)
}

func TestYunBeiViewVIPCenterSkipsForNonVipAccount(t *testing.T) {
	client, transport := newSignFlowClient(t, nil)

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiViewVIPCenter, TaskID: yunBeiServerTaskViewVIPCenter, Target: 1, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiViewVIPCenter(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiUnavailable, snapshot.State)
	assert.Empty(t, transport.paths)
}

func TestYunBeiLikeCommentRestoresOriginalState(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song"}]}}`),
		[]byte(`{"code":200,"comments":[{"commentId":7,"liked":false,"user":{"userId":99}}]}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200,"comments":[{"commentId":7,"liked":false,"user":{"userId":99}}]}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	runtime.loadSnapshot = func(context.Context) (*yunBeiSnapshot, error) {
		return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
			ID: yunBeiLikeComment, State: yunBeiClaimed, Current: 1, Target: 1,
		}}}, nil
	}
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiLikeComment, State: yunBeiPending, Target: 1}

	require.NoError(t, runYunBeiLikeComment(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	require.Len(t, runtime.compensations, 1)
	assert.True(t, runtime.compensations[0].Done)
	assert.Equal(t, []string{
		"/weapi/v3/discovery/recommend/songs",
		"/weapi/v1/resource/comments/R_SO_4_9",
		"/weapi/v1/comment/like",
		"/weapi/v1/comment/unlike",
		"/weapi/v1/resource/comments/R_SO_4_9",
	}, transport.paths)
}

func TestYunBeiLikeCommentDoesNotTouchAlreadyLikedComments(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song"}]}}`),
		[]byte(`{"code":200,"comments":[{"commentId":7,"liked":true,"user":{"userId":99}}]}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiLikeComment, State: yunBeiPending, Target: 1}

	require.NoError(t, runYunBeiLikeComment(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiUnavailable, snapshot.State)
	assert.Empty(t, runtime.compensations)
	assert.Equal(t, []string{
		"/weapi/v3/discovery/recommend/songs",
		"/weapi/v1/resource/comments/R_SO_4_9",
	}, transport.paths)
}

func TestYunBeiLikeCommentRejectsCommentWithoutUserIdentity(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song"}]}}`),
		[]byte(`{"code":200,"comments":[{"commentId":7,"liked":false,"user":{"userId":0}}]}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiLikeComment, Target: 1, State: yunBeiPending}

	require.NoError(t, runYunBeiLikeComment(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiUnavailable, snapshot.State)
	assert.Equal(t, []string{
		"/weapi/v3/discovery/recommend/songs",
		"/weapi/v1/resource/comments/R_SO_4_9",
	}, transport.paths)
}

func TestYunBeiCollectSongRestoresOriginalState(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"playlist":[{"id":5,"name":"mine","userId":42,"specialType":0,"creator":{"userId":42}}]}`),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song"}]}}`),
		[]byte(`{"code":200,"playlist":{"id":5,"name":"mine","userId":42,"specialType":0,"creator":{"userId":42},"trackIds":[{"id":1}]}}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200,"playlist":{"id":5,"name":"mine","userId":42,"specialType":0,"creator":{"userId":42},"trackIds":[{"id":1}]}}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	runtime.loadSnapshot = func(context.Context) (*yunBeiSnapshot, error) {
		return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
			ID: yunBeiCollectSong, State: yunBeiClaimed, Current: 1, Target: 1,
		}}}, nil
	}
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiCollectSong, State: yunBeiPending, Target: 1}

	require.NoError(t, runYunBeiCollectSong(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	require.Len(t, runtime.compensations, 1)
	assert.True(t, runtime.compensations[0].Done)
	assert.Equal(t, []string{
		"/weapi/user/playlist/",
		"/weapi/v3/discovery/recommend/songs",
		"/api/v6/playlist/detail",
		"/weapi/playlist/manipulate/tracks",
		"/weapi/playlist/manipulate/tracks",
		"/api/v6/playlist/detail",
	}, transport.paths)
}

func TestYunBeiCollectSongDoesNotTouchExistingSong(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"playlist":[{"id":5,"name":"mine","userId":42,"specialType":0,"creator":{"userId":42}}]}`),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song"}]}}`),
		[]byte(`{"code":200,"playlist":{"id":5,"name":"mine","userId":42,"specialType":0,"creator":{"userId":42},"trackIds":[{"id":9}]}}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiCollectSong, State: yunBeiPending, Target: 1}

	require.NoError(t, runYunBeiCollectSong(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiUnavailable, snapshot.State)
	assert.Empty(t, runtime.compensations)
	assert.Equal(t, []string{
		"/weapi/user/playlist/",
		"/weapi/v3/discovery/recommend/songs",
		"/api/v6/playlist/detail",
	}, transport.paths)
}

func TestYunBeiCollectSongSkipsBrokenPlaylistAndUsesNext(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"playlist":[{"id":5,"name":"broken","userId":42,"specialType":0,"creator":{"userId":42}},{"id":6,"name":"valid","userId":42,"specialType":0,"creator":{"userId":42}}]}`),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song"}]}}`),
		// 歌单 5 详情损坏（trackCount=2 但只有 1 个 trackId）
		[]byte(`{"code":200,"playlist":{"id":5,"name":"broken","userId":42,"specialType":0,"creator":{"userId":42},"trackCount":2,"trackIds":[{"id":1}]}}`),
		// 歌单 6 详情正常
		[]byte(`{"code":200,"playlist":{"id":6,"name":"valid","userId":42,"specialType":0,"creator":{"userId":42},"trackCount":1,"trackIds":[{"id":1}]}}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200,"playlist":{"id":6,"name":"valid","userId":42,"specialType":0,"creator":{"userId":42},"trackCount":1,"trackIds":[{"id":1}]}}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	runtime.loadSnapshot = func(context.Context) (*yunBeiSnapshot, error) {
		return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
			ID: yunBeiCollectSong, State: yunBeiClaimed, Current: 1, Target: 1,
		}}}, nil
	}
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiCollectSong, State: yunBeiPending, Target: 1}

	require.NoError(t, runYunBeiCollectSong(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	require.Len(t, runtime.compensations, 1)
	assert.True(t, runtime.compensations[0].Done)
	assert.Equal(t, []string{
		"/weapi/user/playlist/",
		"/weapi/v3/discovery/recommend/songs",
		"/api/v6/playlist/detail",
		"/api/v6/playlist/detail",
		"/weapi/playlist/manipulate/tracks",
		"/weapi/playlist/manipulate/tracks",
		"/api/v6/playlist/detail",
	}, transport.paths)
}

func TestYunBeiCollectSongReturnsErrorWhenAllPlaylistsFail(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"playlist":[{"id":5,"name":"broken","userId":42,"specialType":0,"creator":{"userId":42}}]}`),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song"}]}}`),
		[]byte(`{"code":200,"playlist":{"id":5,"name":"broken","userId":42,"specialType":0,"creator":{"userId":42},"trackCount":2,"trackIds":[{"id":1}]}}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiCollectSong, State: yunBeiPending, Target: 1}

	err := runYunBeiCollectSong(context.Background(), runtime, snapshot)
	require.ErrorContains(t, err, "incomplete trackIds")
	assert.Equal(t, []string{
		"/weapi/user/playlist/",
		"/weapi/v3/discovery/recommend/songs",
		"/api/v6/playlist/detail",
	}, transport.paths)
}

func TestYunBeiPlaylistOwnershipRequiresOrdinaryOwnedList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                                      string
		playlistID, ownerID, creatorID, specialID int64
		want                                      bool
	}{
		{name: "owned ordinary", playlistID: 1, ownerID: 42, creatorID: 42, want: true},
		{name: "missing playlist ID", ownerID: 42, creatorID: 42},
		{name: "foreign owner", playlistID: 1, ownerID: 99, creatorID: 42},
		{name: "foreign creator", playlistID: 1, ownerID: 42, creatorID: 99},
		{name: "special list", playlistID: 1, ownerID: 42, creatorID: 42, specialID: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, yunBeiPlaylistOwnedAndOrdinary(
				42,
				tt.playlistID,
				tt.ownerID,
				tt.creatorID,
				tt.specialID,
			))
		})
	}
}

func TestLoadYunBeiPlaylistDetailRejectsIncompleteTrackIDs(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"playlist":{"id":5,"trackCount":2,"trackIds":[{"id":9}]}}`),
	})
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)

	_, err := runtime.loadPlaylistDetail(context.Background(), 5)
	require.ErrorContains(t, err, "incomplete trackIds")
	assert.Equal(t, []string{"/api/v6/playlist/detail"}, transport.paths)
}

func TestYunBeiFollowArtistRestoresOriginalState(t *testing.T) {
	artistResponse := func(followed bool) []byte {
		return encryptSignTestResponse(t, fmt.Appendf(nil,
			`{"code":200,"data":[{"artists":[{"id":7,"name":"artist","followed":%t}]}]}`,
			followed,
		))
	}
	completedTask := []byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[{"taskId":3809301,"completed":true,"taskPointDetail":[{"progressRate":1,"sumTarget":1}]}]}}}`)
	emptyTodo := encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`))
	client, transport := newSignFlowClient(t, [][]byte{
		artistResponse(false),
		[]byte(`{"code":200}`),
		completedTask,
		emptyTodo,
		[]byte(`{"code":200}`),
		[]byte(`{"code":200,"data":[]}`),
		completedTask,
		emptyTodo,
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiFollowArtist, TaskID: yunBeiServerTaskFollowArtist, Target: 1, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiFollowArtist(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	require.Len(t, runtime.compensations, 1)
	assert.True(t, runtime.compensations[0].Done)
	assert.Equal(t, []string{
		"/eapi/artist/hot",
		"/weapi/artist/sub",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
		"/weapi/artist/unsub",
		"/weapi/artist/sublist",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiArtistFollowedReadsEveryReportedPage(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":[{"id":8}],"hasMore":true,"count":2}`),
		[]byte(`{"code":200,"data":[{"id":7}],"hasMore":false,"count":2}`),
	})
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)

	followed, err := runtime.artistFollowed(context.Background(), 7)
	require.NoError(t, err)
	assert.True(t, followed)
	assert.Equal(t, []string{
		"/weapi/artist/sublist",
		"/weapi/artist/sublist",
	}, transport.paths)
}

func TestYunBeiArtistFollowedRejectsTruncatedPagination(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":[],"hasMore":true,"count":1}`),
	})
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)

	_, err := runtime.artistFollowed(context.Background(), 7)
	require.ErrorContains(t, err, "another page")
	assert.Equal(t, []string{"/weapi/artist/sublist"}, transport.paths)
}

func TestYunBeiLikeSongRestoresOriginalState(t *testing.T) {
	completedTask := []byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[{"taskId":3804300,"completed":true,"taskPointDetail":[{"progressRate":1,"sumTarget":1}]}]}}}`)
	emptyTodo := encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`))
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"ids":[1,2]}`),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":2,"name":"old"},{"id":9,"name":"new","alg":"daily"}]}}`),
		[]byte(`{"code":200}`),
		completedTask,
		emptyTodo,
		[]byte(`{"code":200}`),
		[]byte(`{"code":200,"ids":[1,2]}`),
		completedTask,
		emptyTodo,
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiLikeSong, TaskID: yunBeiServerTaskLikeSong, Target: 1, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiLikeSong(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	require.Len(t, runtime.compensations, 1)
	assert.True(t, runtime.compensations[0].Done)
	assert.Equal(t, []string{
		"/weapi/song/like/get",
		"/weapi/v3/discovery/recommend/songs",
		"/weapi/song/like",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
		"/weapi/song/like",
		"/weapi/song/like/get",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiLikeSongDoesNotTouchAlreadyLikedSongs(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"ids":[9]}`),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"old"}]}}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiLikeSong, State: yunBeiPending, Target: 1}

	require.NoError(t, runYunBeiLikeSong(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiUnavailable, snapshot.State)
	assert.Empty(t, runtime.compensations)
	assert.Equal(t, []string{
		"/weapi/song/like/get",
		"/weapi/v3/discovery/recommend/songs",
	}, transport.paths)
}

func TestYunBeiLikeSongCancellationStillRestoresWithCleanupContext(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"ids":[]}`),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"new"}]}}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200,"ids":[]}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	runtime.loadSnapshot = func(context.Context) (*yunBeiSnapshot, error) {
		return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
			ID: yunBeiLikeSong, State: yunBeiPending, Current: 0, Target: 1,
		}}}, nil
	}
	runtime.sleep = func(context.Context, time.Duration, ...time.Duration) error {
		return context.Canceled
	}
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiLikeSong, State: yunBeiPending, Target: 1}

	err := runYunBeiLikeSong(context.Background(), runtime, snapshot)
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, runtime.compensations, 1)
	assert.False(t, runtime.compensations[0].Done)

	runtime.sleep = func(context.Context, time.Duration, ...time.Duration) error { return nil }
	require.NoError(t, runtime.restoreCompensations(context.Background(), 0))
	assert.True(t, runtime.compensations[0].Done)
	assert.Equal(t, []string{
		"/weapi/song/like/get",
		"/weapi/v3/discovery/recommend/songs",
		"/weapi/song/like",
		"/weapi/song/like",
		"/weapi/song/like/get",
	}, transport.paths)
}

func TestYunBeiPublishNoteRecordsEventAndConfirms(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song","al":{"picUrl":"https://p1.music.126.net/cover.jpg"}}]}}`),
		[]byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[{"taskId":6566461,"completed":true,"taskPointDetail":[{"progressRate":1,"sumTarget":1}]}]}}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
	})
	imagePath := filepath.Join(t.TempDir(), "note.jpg")
	require.NoError(t, os.WriteFile(imagePath, []byte("image"), 0o600))

	command := NewYunBei(&Root{}, nil)
	command.opts.Image = imagePath
	runtime := newYunBeiRuntime(command, weapi.New(client), eapi.New(client), 42)
	runtime.uploadImage = func(_ context.Context, path string) (string, error) {
		assert.Equal(t, imagePath, path)
		return `[{"originId":"1"}]`, nil
	}
	runtime.publishEvent = func(_ context.Context, request *eapi.EventPublishReq) (*eapi.EventPublishResp, error) {
		assert.Equal(t, "今日推荐: song", request.Title)
		assert.Equal(t, yunBeiNoteMessage, request.Msg)
		assert.Equal(t, "noresource", request.Type)
		assert.JSONEq(t, `[{"originId":"1"}]`, request.Pics)

		response := &eapi.EventPublishResp{Id: 99}
		response.Code = 200
		return response, nil
	}
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiPublishNote, TaskID: yunBeiServerTaskPublishNote, Target: 1, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiPublishNote(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	assert.Equal(t, []int64{99}, runtime.eventIDs)
	assert.Equal(t, []string{
		"/weapi/v3/discovery/recommend/songs",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

// TestYunBeiPublishNoteWaitsForAsyncCredit 锁定 publish-note 的确认窗口: 服务端在笔记发布后
// 异步创建 userMissionId (2026-11-02 实测计入延迟大于 3 秒), 因此该任务必须按
// yunBeiNoteObserve 的间隔重试; 若退回秒级窗口, 已计入的笔记会被误判为 unconfirmed。
func TestYunBeiPublishNoteWaitsForAsyncCredit(t *testing.T) {
	client, _ := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song","al":{"picUrl":"https://p1.music.126.net/cover.jpg"}}]}}`),
	})
	imagePath := filepath.Join(t.TempDir(), "note.jpg")
	require.NoError(t, os.WriteFile(imagePath, []byte("image"), 0o600))

	command := NewYunBei(&Root{}, nil)
	command.opts.Image = imagePath
	runtime := newYunBeiRuntime(command, weapi.New(client), eapi.New(client), 42)

	var waits []time.Duration

	runtime.sleep = func(_ context.Context, delay time.Duration, _ ...time.Duration) error {
		waits = append(waits, delay)
		return nil
	}
	runtime.uploadImage = func(context.Context, string) (string, error) {
		return `[{"originId":"1"}]`, nil
	}
	runtime.publishEvent = func(context.Context, *eapi.EventPublishReq) (*eapi.EventPublishResp, error) {
		response := &eapi.EventPublishResp{Id: 99}
		response.Code = 200
		return response, nil
	}

	reads := 0
	runtime.loadSnapshot = func(context.Context) (*yunBeiSnapshot, error) {
		reads++
		task := &yunBeiTaskSnapshot{
			ID: yunBeiPublishNote, TaskID: yunBeiServerTaskPublishNote, Target: 1, State: yunBeiPending,
		}

		if reads >= 3 {
			task.Current = 1
			task.State = yunBeiCompletedUnclaimed
		}

		return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{task}}, nil
	}

	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiPublishNote, TaskID: yunBeiServerTaskPublishNote, Target: 1, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiPublishNote(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiCompletedUnclaimed, snapshot.State)
	assert.Equal(t, []time.Duration{yunBeiNoteObserveInterval, yunBeiNoteObserveInterval}, waits)
}

func TestYunBeiPublishNoteDoesNotDeleteBeforeTaskConfirmation(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song","al":{"picUrl":"https://p1.music.126.net/cover.jpg"}}]}}`),
	})
	imagePath := filepath.Join(t.TempDir(), "note.jpg")
	require.NoError(t, os.WriteFile(imagePath, []byte("image"), 0o600))

	command := NewYunBei(&Root{}, nil)
	command.opts.Image = imagePath
	command.opts.Delete = true
	runtime := newYunBeiRuntime(command, weapi.New(client), eapi.New(client), 42)
	runtime.sleep = func(context.Context, time.Duration, ...time.Duration) error { return nil }
	runtime.loadSnapshot = func(context.Context) (*yunBeiSnapshot, error) {
		return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
			ID: yunBeiPublishNote, State: yunBeiPending, Current: 0, Target: 1,
		}}}, nil
	}
	runtime.uploadImage = func(context.Context, string) (string, error) {
		return `[{"originId":"1"}]`, nil
	}
	runtime.publishEvent = func(context.Context, *eapi.EventPublishReq) (*eapi.EventPublishResp, error) {
		response := &eapi.EventPublishResp{Id: 99}
		response.Code = 200
		return response, nil
	}
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiPublishNote, TaskID: yunBeiServerTaskPublishNote, Target: 1, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiPublishNote(context.Background(), runtime, snapshot))
	assert.Equal(t, []int64{99}, runtime.eventIDs)
	assert.Empty(t, runtime.deleteEventIDs)
	assert.Equal(t, []string{"/weapi/v3/discovery/recommend/songs"}, transport.paths)
}

func TestYunBeiPublishNoteDeletesConfirmedEvent(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song","al":{"picUrl":"https://p1.music.126.net/cover.jpg"}}]}}`),
		encryptSignTestResponse(t, []byte(`{"code":200}`)),
	})
	imagePath := filepath.Join(t.TempDir(), "note.jpg")
	require.NoError(t, os.WriteFile(imagePath, []byte("image"), 0o600))

	command := NewYunBei(&Root{}, nil)
	command.opts.Image = imagePath
	command.opts.Delete = true
	runtime := newYunBeiRuntime(command, weapi.New(client), eapi.New(client), 42)
	runtime.loadSnapshot = func(context.Context) (*yunBeiSnapshot, error) {
		return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
			ID: yunBeiPublishNote, State: yunBeiClaimed, Current: 1, Target: 1,
		}}}, nil
	}
	runtime.uploadImage = func(context.Context, string) (string, error) {
		return `[{"originId":"1"}]`, nil
	}
	runtime.publishEvent = func(context.Context, *eapi.EventPublishReq) (*eapi.EventPublishResp, error) {
		response := &eapi.EventPublishResp{Id: 99}
		response.Code = 200
		return response, nil
	}
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiPublishNote, TaskID: yunBeiServerTaskPublishNote, Target: 1, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiPublishNote(context.Background(), runtime, snapshot))
	assert.Equal(t, []int64{99}, runtime.eventIDs)
	assert.Equal(t, []int64{0}, runtime.deleteEventIDs)
	assert.Equal(t, []string{
		"/weapi/v3/discovery/recommend/songs",
		"/eapi/event/delete",
	}, transport.paths)
}

func TestYunBeiPublishNoteDeletesConfirmedEventInSecondRound(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song","al":{"picUrl":"https://p1.music.126.net/cover.jpg"}}]}}`),
		encryptSignTestResponse(t, []byte(`{"code":200}`)),
	})
	imagePath := filepath.Join(t.TempDir(), "note.jpg")
	require.NoError(t, os.WriteFile(imagePath, []byte("image"), 0o600))

	command := NewYunBei(&Root{}, nil)
	command.opts.Image = imagePath
	command.opts.Delete = true
	runtime := newYunBeiRuntime(command, weapi.New(client), eapi.New(client), 42)
	runtime.sleep = func(context.Context, time.Duration, ...time.Duration) error { return nil }

	confirmed := false
	runtime.loadSnapshot = func(context.Context) (*yunBeiSnapshot, error) {
		state := yunBeiPending
		current := int64(0)

		if confirmed {
			state = yunBeiClaimed
			current = 1
		}
		return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
			ID: yunBeiPublishNote, TaskID: yunBeiServerTaskPublishNote, State: state, Current: current, Target: 1,
		}}}, nil
	}
	runtime.uploadImage = func(context.Context, string) (string, error) {
		return `[{"originId":"1"}]`, nil
	}
	runtime.publishEvent = func(context.Context, *eapi.EventPublishReq) (*eapi.EventPublishResp, error) {
		response := &eapi.EventPublishResp{Id: 99}
		response.Code = 200
		return response, nil
	}
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiPublishNote, TaskID: yunBeiServerTaskPublishNote, Target: 1, State: yunBeiPending,
	}

	// 第 1 轮：发布成功但异步未确认，不应触发删除
	require.NoError(t, runYunBeiPublishNote(context.Background(), runtime, snapshot))
	assert.Equal(t, []int64{99}, runtime.eventIDs)
	assert.Empty(t, runtime.deleteEventIDs)
	assert.Equal(t, []string{"/weapi/v3/discovery/recommend/songs"}, transport.paths)

	// 第 2 轮：服务端异步计入并确认完成，应触发删除已发布的动态
	confirmed = true

	require.NoError(t, runYunBeiPublishNote(context.Background(), runtime, snapshot))
	assert.Equal(t, []int64{99}, runtime.eventIDs)
	assert.Equal(t, []int64{0}, runtime.deleteEventIDs)
	assert.Equal(t, []string{
		"/weapi/v3/discovery/recommend/songs",
		"/eapi/event/delete",
	}, transport.paths)
}

// TestYunBeiPublishNoteLogsEventIDDespiteRedactedBody 保证 EventPublish 挂了 SetRedactLogs
// 隐去正文之后，日志仍能把「实际发布的动态」和「未推进的任务」对上。
func TestYunBeiPublishNoteLogsEventIDDespiteRedactedBody(t *testing.T) {
	client, _ := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song","al":{"picUrl":"https://p1.music.126.net/cover.jpg"}}]}}`),
	})

	imagePath := filepath.Join(t.TempDir(), "note.jpg")

	require.NoError(t, os.WriteFile(imagePath, []byte("image"), 0o600))

	logFile := filepath.Join(t.TempDir(), "test.log")
	logger := projectlog.New(&projectlog.Config{
		Level:  "debug",
		Rotate: lumberjack.Logger{Filename: logFile},
	})

	t.Cleanup(func() { _ = logger.Close() })

	command := NewYunBei(&Root{}, logger)
	command.opts.Image = imagePath

	runtime := newYunBeiRuntime(command, weapi.New(client), eapi.New(client), 42)
	runtime.sleep = func(context.Context, time.Duration, ...time.Duration) error { return nil }
	runtime.uploadImage = func(context.Context, string) (string, error) {
		return `[{"originId":"1"}]`, nil
	}
	runtime.publishEvent = func(context.Context, *eapi.EventPublishReq) (*eapi.EventPublishResp, error) {
		response := &eapi.EventPublishResp{Id: 99}
		response.Code = 200

		return response, nil
	}
	runtime.loadSnapshot = func(context.Context) (*yunBeiSnapshot, error) {
		return &yunBeiSnapshot{Tasks: []*yunBeiTaskSnapshot{{
			ID: yunBeiPublishNote, State: yunBeiPending, Current: 0, Target: 1,
		}}}, nil
	}

	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiPublishNote, TaskID: yunBeiServerTaskPublishNote, Target: 1, State: yunBeiPending,
	}
	require.NoError(t, runYunBeiPublishNote(context.Background(), runtime, snapshot))

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	output := string(data)
	assert.Contains(t, output, "publish-note")
	assert.Contains(t, output, "eventID=99")
	assert.Contains(t, output, "missionTaskID=6566461")
	assert.Contains(t, output, "bodyRedacted=true")
	assert.NotContains(t, output, "originId", "上传后的图片元数据不得进入日志")
}

func TestYunBeiListenIndieUsesServerSingleAmount(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"data":{"times":0,"amount":0,"singleAmount":15}}`),
		[]byte(`{"code":200,"data":[{"songId":9,"songName":"indie","completeStatus":false}]}`),
		[]byte(`{"code":200,"data":[{"id":9,"url":"https://m1.music.126.net/audio.mp3","code":200}]}`),
		[]byte(`{"code":200,"data":true}`),
		[]byte(`{"code":200,"data":{"times":1,"amount":15,"singleAmount":15}}`),
		[]byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[{"taskId":6718451,"completed":true,"taskPointDetail":[{"progressRate":1,"sumTarget":1}]}]}}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	runtime.fetchAudio = func(_ context.Context, timeout time.Duration, rawURL string, limit int64) error {
		assert.Equal(t, time.Minute, timeout)
		assert.Equal(t, "https://m1.music.126.net/audio.mp3", rawURL)
		assert.Equal(t, int64(yunBeiIndieAudioFetchLimit), limit)
		return nil
	}
	runtime.sleep = func(_ context.Context, duration time.Duration, _ ...time.Duration) error {
		assert.Equal(t, yunBeiIndieListenTime, duration)
		return nil
	}
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiListenIndie, TaskID: yunBeiServerTaskListenIndie, Target: 1, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiListenIndie(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	assert.Equal(t, []string{
		"/weapi/ad/power/yunbei/distribution/list",
		"/weapi/ad/power/yunbei/distribution/recommend/song",
		"/weapi/song/enhance/player/url/v1",
		"/weapi/ad/power/yunbei/distribution/create",
		"/weapi/ad/power/yunbei/distribution/list",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiPlayDailyRecommendUsesMeasuredElapsedTime(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		encryptSignTestResponse(t, []byte(`{"code":200,"data":{"isMusician":false}}`)),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"daily","dt":10000,"al":{"id":2},"ar":[{"id":3}]}]}}`),
		[]byte(`{"code":200,"songs":[{"id":9,"name":"daily","dt":10000,"al":{"id":2}}]}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200,"data":[{"id":9,"url":"https://m1.music.126.net/audio.mp3","code":200}]}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[{"taskId":3801300,"completed":true,"taskPointDetail":[{"progressRate":5,"sumTarget":5}]}]}}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	runtime.fetchAudio = func(_ context.Context, _ time.Duration, rawURL string, limit int64) error {
		assert.Equal(t, "https://m1.music.126.net/audio.mp3", rawURL)
		assert.Equal(t, int64(yunBeiIndieAudioFetchLimit), limit)
		return nil
	}
	clock := time.Unix(100, 0)
	runtime.now = func() time.Time { return clock }
	runtime.sleep = func(_ context.Context, duration time.Duration, _ ...time.Duration) error {
		assert.Equal(t, 5*time.Second, duration)
		clock = clock.Add(duration)
		return nil
	}
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiPlayDailyRecommend, TaskID: yunBeiServerTaskPlayDailyRecommend,
		Current: 0, Target: 5, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiPlayDailyRecommend(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiClaimed, snapshot.State)
	assert.Equal(t, []string{
		"/eapi/nmusician/workbench/musician/role/get",
		"/weapi/v3/discovery/recommend/songs",
		"/weapi/v3/song/detail",
		"/weapi/feedback/weblog",
		"/weapi/song/enhance/player/url/v1",
		"/weapi/feedback/weblog",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiPlayDailyRecommendRejectsAmbiguousMusicianIdentity(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "UID mismatch",
			body: `{"code":200,"data":{"isMusician":true,"userId":99,"artistId":7}}`,
			want: "does not map UID 42",
		},
		{
			name: "missing artist ID",
			body: `{"code":200,"data":{"isMusician":true,"userId":42,"artistId":0}}`,
			want: "positive artist ID",
		},
		{
			name: "business failure",
			body: `{"code":500,"message":"failed"}`,
			want: "code=500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := newSignFlowClient(t, [][]byte{
				encryptSignTestResponse(t, []byte(tt.body)),
			})
			runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
			snapshot := &yunBeiTaskSnapshot{
				ID: yunBeiPlayDailyRecommend, Current: 0, Target: 5, State: yunBeiPending,
			}

			require.ErrorContains(t, runYunBeiPlayDailyRecommend(context.Background(), runtime, snapshot), tt.want)
			assert.Equal(t, []string{"/eapi/nmusician/workbench/musician/role/get"}, transport.paths)
		})
	}
}

func TestYunBeiPlayDailyRecommendFiltersCurrentMusicianSongs(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		encryptSignTestResponse(t, []byte(`{"code":200,"data":{"isMusician":true,"userId":42,"artistId":7}}`)),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":8,"name":"own","dt":10000,"ar":[{"id":7}]},{"id":9,"name":"other","dt":10000,"al":{"id":2},"ar":[{"id":3}]}]}}`),
		[]byte(`{"code":200,"songs":[{"id":9,"name":"other","dt":10000,"al":{"id":2}}]}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200,"data":[{"id":9,"url":"https://m1.music.126.net/audio.mp3","code":200}]}`),
		[]byte(`{"code":200}`),
		[]byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[{"taskId":3801300,"completed":true,"taskPointDetail":[{"progressRate":5,"sumTarget":5}]}]}}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	runtime.fetchAudio = func(context.Context, time.Duration, string, int64) error { return nil }
	clock := time.Unix(100, 0)
	runtime.now = func() time.Time { return clock }
	runtime.sleep = func(_ context.Context, duration time.Duration, _ ...time.Duration) error {
		clock = clock.Add(duration)
		return nil
	}
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiPlayDailyRecommend, Current: 0, Target: 5, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiPlayDailyRecommend(context.Background(), runtime, snapshot))
	assert.Equal(t, []string{
		"/eapi/nmusician/workbench/musician/role/get",
		"/weapi/v3/discovery/recommend/songs",
		"/weapi/v3/song/detail",
		"/weapi/feedback/weblog",
		"/weapi/song/enhance/player/url/v1",
		"/weapi/feedback/weblog",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiPlayDailyRecommendRechecksMusicianOnSongDetail(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		encryptSignTestResponse(t, []byte(`{"code":200,"data":{"isMusician":true,"userId":42,"artistId":7}}`)),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"candidate","dt":10000,"ar":[{"id":3}]}]}}`),
		[]byte(`{"code":200,"songs":[{"id":9,"name":"own detail","dt":10000,"ar":[{"id":7}]}]}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiPlayDailyRecommend, Current: 0, Target: 5, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiPlayDailyRecommend(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiUnavailable, snapshot.State)
	assert.Equal(t, []string{
		"/eapi/nmusician/workbench/musician/role/get",
		"/weapi/v3/discovery/recommend/songs",
		"/weapi/v3/song/detail",
	}, transport.paths)
}

func TestYunBeiUnmappedTaskIsUnavailableWithoutWrite(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[]}}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
	})

	command := NewYunBei(&Root{}, nil)
	command.selected = []yunBeiTaskID{yunBeiLikeComment}
	require.NoError(t, command.run(context.Background(), client))
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiRegularTasksIsolateUnmatchedTodo(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(
			`{"code":200,"data":{"newbie":null,"normal":{"list":[{"taskId":6758460,"taskName":"view","completed":true,"taskPointDetail":[{"progressRate":1,"sumTarget":1}]},{"taskId":3801300,"taskName":"play","completed":true,"taskPointDetail":[{"userMissionId":101,"progressRate":1,"sumTarget":1}]}]}}}`,
		),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[{"completed":false,"taskName":"unmatched","userTaskId":0}]}`)),
	})

	var output bytes.Buffer

	command := NewYunBei(&Root{}, nil)
	command.cmd.SetOut(&output)
	command.selected = []yunBeiTaskID{yunBeiViewVIPCenter, yunBeiPlayDailyRecommend}
	require.NoError(t, command.run(context.Background(), client))
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
	assert.Contains(t, output.String(), "任务关联待确认: 1 项（仅隔离对应条目）")
	assert.Contains(t, output.String(), "[view-vip-center] 服务端已完成，已跳过")
}

func TestYunBeiRegularTaskRejectsUnknownBucketBeforeAction(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"newbie":{"list":[{"taskId":1}]},"normal":{"list":[]}}}`),
	})

	command := NewYunBei(&Root{}, nil)
	command.selected = []yunBeiTaskID{yunBeiViewVIPCenter}
	err := command.run(context.Background(), client)
	require.ErrorContains(t, err, "non-empty untyped buckets: newbie")
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/usertool/task/list/all/v3",
	}, transport.paths)
}

func TestYunBeiSignCompletesBeforeRegularStateFailure(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"sign":true}}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[]}}`),
		[]byte(`{"code":200,"data":{"newbie":{"list":[{"taskId":1}]},"normal":{"list":[]}}}`),
	})

	var output bytes.Buffer

	command := NewYunBei(&Root{}, nil)
	command.cmd.SetOut(&output)
	command.selected = []yunBeiTaskID{yunBeiSign, yunBeiViewVIPCenter}
	err := command.run(context.Background(), client)
	require.ErrorContains(t, err, "non-empty untyped buckets: newbie")
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/pointmall/user/sign",
		"/weapi/pointmall/user/sign/config",
		"/weapi/usertool/task/list/all/v3",
	}, transport.paths)
	assert.Contains(t, output.String(), "云贝签到: 成功")
}

func TestYunBeiAllPrintsRiskSummaryBeforeFirstWrite(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"sign":false}}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[]}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"type":"PREV_CLAIMED_BOOKED","reqId":"fixed"}}`)),
		[]byte(`{"code":200,"data":{"newbie":null,"normal":{"list":[]}}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
	})

	var output bytes.Buffer

	command := NewYunBei(&Root{}, nil)
	command.cmd.SetOut(&output)

	selected, err := normalizeYunBeiTasks(nil, true)
	require.NoError(t, err)

	command.selected = selected

	sawFirstWrite := false
	transport.beforeRoundTrip = func(request *http.Request) {
		if request.URL.Path != "/weapi/pointmall/user/sign" {
			return
		}

		sawFirstWrite = true

		for _, expected := range []string{
			"公开发布图文动态",
			"点赞评论并恢复原状态",
			"关注歌手并恢复原状态",
			"收藏歌曲并恢复原状态",
			"红心歌曲并恢复原状态",
			"触发站外分享",
			"长时间播放并上报每日推荐",
		} {
			assert.Contains(t, output.String(), expected)
		}
	}

	require.NoError(t, command.run(context.Background(), client))
	assert.True(t, sawFirstWrite)
}

func TestYunBeiClaimUsesServerFieldsAndRefreshes(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[{"completed":true,"depositCode":1304,"period":1,"taskId":123,"taskName":"fixed task","taskPoint":5,"userTaskId":456}]}`)),
		[]byte(`{"code":200,"data":true}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
	})

	var output bytes.Buffer

	command := NewYunBei(&Root{}, nil)
	command.cmd.SetOut(&output)
	command.selected = []yunBeiTaskID{yunBeiClaim}

	require.NoError(t, command.run(context.Background(), client))
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/eapi/usertool/task/todo/query",
		"/weapi/usertool/task/point/receive",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
	assert.Contains(t, output.String(), "[fixed task] 获得 5 云贝")
}

func TestYunBeiClaimAcceptsZeroPeriodSpecialWelfare(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[{"completed":true,"depositCode":2111,"period":0,"taskName":"特殊福利","taskPoint":50,"userTaskId":3826123692}]}`)),
		[]byte(`{"code":200,"data":true}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
	})

	var output bytes.Buffer

	command := NewYunBei(&Root{}, nil)
	command.cmd.SetOut(&output)
	command.selected = []yunBeiTaskID{yunBeiClaim}

	require.NoError(t, command.run(context.Background(), client))
	assert.Contains(t, output.String(), "[特殊福利] 获得 50 云贝")
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/eapi/usertool/task/todo/query",
		"/weapi/usertool/task/point/receive",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiClaimRejectsMissingServerFields(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[{"completed":true,"depositCode":0,"period":1,"taskName":"fixed task","taskPoint":5,"userTaskId":456}]}`)),
	})

	command := NewYunBei(&Root{}, nil)
	command.selected = []yunBeiTaskID{yunBeiClaim}
	err := command.run(context.Background(), client)
	require.ErrorContains(t, err, "claim fields are missing or invalid")
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiClaimRejectsDuplicateUserTaskIDWithoutWrite(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[{"completed":true,"depositCode":1304,"period":1,"userTaskId":456},{"completed":true,"depositCode":1305,"period":1,"userTaskId":456}]}`)),
	})

	command := NewYunBei(&Root{}, nil)
	command.selected = []yunBeiTaskID{yunBeiClaim}
	err := command.run(context.Background(), client)
	require.ErrorContains(t, err, "duplicate todo records")
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiClaimRechecksRemainingItemsAfterEachWrite(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		encryptSignTestResponse(t, []byte(`{
			"code": 200,
			"data": [
				{"completed":true,"depositCode":1304,"period":1,"taskName":"first","taskPoint":5,"userTaskId":456},
				{"completed":true,"depositCode":1305,"period":1,"taskName":"second","taskPoint":6,"userTaskId":789}
			]
		}`)),
		[]byte(`{"code":200,"data":true}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[]}`)),
	})

	command := NewYunBei(&Root{}, nil)
	command.selected = []yunBeiTaskID{yunBeiClaim}
	require.NoError(t, command.run(context.Background(), client))
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/eapi/usertool/task/todo/query",
		"/weapi/usertool/task/point/receive",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiClaimStopsBeforeWriteWhenContextCancelsAfterTodo(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"data":[{"completed":true,"depositCode":1304,"period":1,"userTaskId":456}]}`)),
	})

	ctx, cancel := context.WithCancel(context.Background())
	transport.beforeRoundTrip = func(request *http.Request) {
		if request.URL.Path == "/eapi/usertool/task/todo/query" {
			cancel()
		}
	}

	command := NewYunBei(&Root{}, nil)
	command.selected = []yunBeiTaskID{yunBeiClaim}
	require.ErrorIs(t, command.run(ctx, client), context.Canceled)
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiClaimBlocksUnsafeTaskAndContinuesIndependentReward(t *testing.T) {
	todo := `{"code":200,"data":[` +
		`{"completed":true,"depositCode":1304,"period":1,"taskName":"unsafe","taskPoint":5,"userTaskId":456},` +
		`{"completed":true,"depositCode":1305,"period":1,"taskName":"safe","taskPoint":6,"userTaskId":789}]}`
	v3 := `{"code":200,"data":{"newbie":null,"normal":{"list":[` +
		`{"taskId":3804300,"completed":true,"taskPointDetail":[{"userMissionId":456,"progressRate":1,"sumTarget":1}]},` +
		`{"taskId":3801300,"completed":true,"taskPointDetail":[{"userMissionId":789,"progressRate":1,"sumTarget":1}]}]}}}`
	refreshed := `{"code":200,"data":[` +
		`{"completed":true,"depositCode":1304,"period":1,"taskName":"unsafe","taskPoint":5,"userTaskId":456}]}`
	client, transport := newSignFlowClient(t, [][]byte{
		encryptSignTestResponse(t, []byte(todo)),
		[]byte(v3),
		[]byte(`{"code":200,"data":true}`),
		encryptSignTestResponse(t, []byte(refreshed)),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	runtime.results[yunBeiLikeSong] = &yunBeiResult{ID: yunBeiLikeSong, State: yunBeiUnconfirmed}
	snapshot := &yunBeiTaskSnapshot{ID: yunBeiClaim, State: yunBeiPending}

	err := runYunBeiClaim(context.Background(), runtime, snapshot)
	require.ErrorContains(t, err, "task 456: reward claim blocked")
	assert.Equal(t, yunBeiUnconfirmed, snapshot.State)
	assert.Equal(t, []string{
		"/eapi/usertool/task/todo/query",
		"/weapi/usertool/task/list/all/v3",
		"/weapi/usertool/task/point/receive",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
}

func TestYunBeiStatusUsesReadOnlyEndpoints(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"account":{"id":42},"profile":{"userId":42,"nickname":"tester"}}`),
		[]byte(`{"code":200,"data":{"userId":42,"balance":12,"blockBalance":3}}`),
		[]byte(`{"code":200,"data":{"signStr":"111110000000000000000000000000","curTimeStamp":1788607883137}}`),
		[]byte(`{"code":200,"data":{"lotteryConfig":[{"signDay":3,"baseLotteryId":11,"baseLotteryStatus":1,"extraLotteryId":22,"extraLotteryStatus":0}]}}`),
		encryptSignTestResponse(t, []byte(`{"code":200,"success":true,"data":{"type":"FIXED_STATUS","currentAmount":1}}`)),
		[]byte(
			`{"code":200,"data":{"newbie":null,"normal":{"list":[{"completed":true,"completedPoint":2,"targetPoint":3,"taskId":3801300,"taskName":"fixed task","taskPoint":5,"userTaskId":100,"actionType":618,"subAction":"play","taskPointDetail":[{"userMissionId":101,"progressRate":2,"sumTarget":3}]}]}}}`,
		),
		encryptSignTestResponse(
			t,
			[]byte(
				`{"code":200,"data":[{"completed":true,"depositCode":1304,"period":1,"taskName":"fixed task","taskPoint":5,"userTaskId":101},{"completed":false,"taskName":"unmatched task","taskPoint":2,"userTaskId":0}]}`,
			),
		),
	})

	var output bytes.Buffer

	command := NewYunBei(&Root{}, nil)
	command.cmd.SetOut(&output)
	command.status = true

	require.NoError(t, command.run(context.Background(), client))
	assert.Equal(t, []string{
		"/weapi/w/nuser/account/get",
		"/weapi/middle/mall/balance",
		"/weapi/pointmall/sign/calendar",
		"/weapi/pointmall/user/sign/config",
		"/eapi/new/yunbei/activity/reserve/info",
		"/weapi/usertool/task/list/all/v3",
		"/eapi/usertool/task/todo/query",
	}, transport.paths)
	assert.NotContains(t, output.String(), "开始执行")
	assert.Contains(t, output.String(), "本地标识 play-daily-recommend")
	assert.Contains(t, output.String(), "taskId 3801300")
	assert.Contains(t, output.String(), "userTaskId=100 actionType=618 subAction=play")
	assert.Contains(t, output.String(), "原始计数 2/3")
	assert.Contains(t, output.String(), string(yunBeiCompletedUnclaimed))
	assert.Contains(t, output.String(), "今日签到: 已签到")
	assert.Contains(t, output.String(), "第 3 天: base(status=1 id=11) extra(status=0 id=22)")
	assert.Contains(t, output.String(), "待办未关联")
	assert.Contains(t, output.String(), "任务关联待确认: 1 项")
}

func TestYunBeiSignedTodayUsesServerCalendar(t *testing.T) {
	timestamp := time.Date(2026, time.September, 5, 19, 31, 0, 0, yunBeiCalendarLocation).UnixMilli()

	signed, err := yunBeiSignedToday(&weapi.YunBeiSignInCalendarRespData{
		SignStr:      "111110000000000000000000000000",
		CurTimeStamp: timestamp,
	})
	require.NoError(t, err)
	assert.True(t, signed)

	signed, err = yunBeiSignedToday(&weapi.YunBeiSignInCalendarRespData{
		SignStr:      "111100000000000000000000000000",
		CurTimeStamp: timestamp,
	})
	require.NoError(t, err)
	assert.False(t, signed)

	_, err = yunBeiSignedToday(&weapi.YunBeiSignInCalendarRespData{
		SignStr:      "1111x0000000000000000000000000",
		CurTimeStamp: timestamp,
	})
	require.ErrorContains(t, err, "invalid value")
}

func TestYunBeiLoginResponseValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "account", body: `{"code":200,"profile":{"userId":42}}`, want: "account is missing"},
		{name: "profile", body: `{"code":200,"account":{"id":42}}`, want: "profile is missing"},
		{name: "user ID", body: `{"code":200,"account":{"id":42},"profile":{"userId":0}}`, want: "user ID is invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newSignFlowClient(t, [][]byte{[]byte(tt.body)})
			command := NewYunBei(&Root{}, nil)
			command.selected = []yunBeiTaskID{yunBeiSign}

			require.ErrorContains(t, command.run(context.Background(), client), tt.want)
		})
	}
}

func TestNewYunBeiRuntimeProvidesProductionHooks(t *testing.T) {
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), nil, nil, 42)
	assert.Equal(t, int64(42), runtime.uid)
	assert.Equal(t, 1, runtime.round)
	assert.NotNil(t, runtime.sleep)
	assert.NotNil(t, runtime.fetchAudio)
	assert.NotNil(t, runtime.uploadImage)
	assert.NotNil(t, runtime.publishEvent)
	assert.NotNil(t, runtime.loadSnapshot)
	assert.NotNil(t, runtime.now)
	assert.NotNil(t, runtime.withTimeout)
	assert.NotNil(t, runtime.attempted)
	assert.NotNil(t, runtime.results)
	assert.NotNil(t, runtime.compensations)
	assert.NotNil(t, runtime.eventIDs)
	assert.NotNil(t, runtime.deleteEventIDs)
	assert.False(t, runtime.now().IsZero())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, runtime.sleep(ctx, time.Hour), context.Canceled)

	timeoutCtx, timeoutCancel := runtime.withTimeout(context.Background(), time.Hour)
	timeoutCancel()
	require.ErrorIs(t, timeoutCtx.Err(), context.Canceled)
}

func TestYunBeiRuntimeDeduplicatesActionsAcrossRounds(t *testing.T) {
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), nil, nil, 42)
	first := yunBeiActionKey{TaskID: yunBeiLikeComment, Kind: "like", TargetID: "comment-1"}

	assert.True(t, runtime.beginAction(first))
	runtime.round = 2
	assert.False(t, runtime.beginAction(first))
	assert.True(t, runtime.beginAction(yunBeiActionKey{
		TaskID:   yunBeiLikeComment,
		Kind:     "like",
		TargetID: "comment-2",
	}))
	assert.True(t, runtime.beginAction(yunBeiActionKey{
		TaskID:   yunBeiFollowArtist,
		Kind:     "like",
		TargetID: "comment-1",
	}))
}

func TestYunBeiLikeSongKeepsPriorAttemptPendingWhenNoNewCandidateExists(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		[]byte(`{"code":200,"ids":[]}`),
		[]byte(`{"code":200,"data":{"dailySongs":[{"id":9,"name":"song"}]}}`),
	})

	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), weapi.New(client), eapi.New(client), 42)
	require.True(t, runtime.beginAction(yunBeiActionKey{
		TaskID: yunBeiLikeSong, Kind: "like", TargetID: "9",
	}))
	snapshot := &yunBeiTaskSnapshot{
		ID: yunBeiLikeSong, Current: 1, Target: 2, State: yunBeiPending,
	}

	require.NoError(t, runYunBeiLikeSong(context.Background(), runtime, snapshot))
	assert.Equal(t, yunBeiPending, snapshot.State)
	assert.Equal(t, []string{
		"/weapi/song/like/get",
		"/weapi/v3/discovery/recommend/songs",
	}, transport.paths)
}

func TestYunBeiRuntimeRecordsOnlyUniquePositiveEventIDs(t *testing.T) {
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), nil, nil, 42)
	require.ErrorContains(t, runtime.addEventID(0), "must be positive")
	require.ErrorContains(t, runtime.addEventDeleteIntent(99), "was not recorded")
	require.NoError(t, runtime.addEventID(99))
	require.ErrorContains(t, runtime.addEventID(99), "already recorded")
	require.NoError(t, runtime.addEventDeleteIntent(99))
	require.ErrorContains(t, runtime.addEventDeleteIntent(99), "already has a delete intent")
	assert.Equal(t, []int64{99}, runtime.eventIDs)
	assert.Equal(t, []int64{99}, runtime.deleteEventIDs)
}

func TestYunBeiCompensationsAreReverseAndIdempotentAfterCancellation(t *testing.T) {
	command := NewYunBei(&Root{}, nil)

	var output bytes.Buffer
	command.cmd.SetOut(&output)
	runtime := newYunBeiRuntime(command, nil, nil, 42)

	var order []string

	for _, target := range []string{"first", "second", "third"} {
		runtime.addCompensation(yunBeiLikeComment, "comment-unlike", target, "thread-1", func(context.Context) error {
			order = append(order, target)
			return nil
		})
	}

	parent, cancel := context.WithCancel(context.Background())
	cancel()

	cleanupCtx, cleanupCancel := runtime.withTimeout(context.WithoutCancel(parent), yunBeiCleanupTimeout)
	defer cleanupCancel()

	require.NoError(t, runtime.restoreCompensations(cleanupCtx, 1))
	assert.Equal(t, []string{"third", "second"}, order)
	assert.False(t, runtime.compensations[0].Done)
	assert.True(t, runtime.compensations[1].Done)
	assert.True(t, runtime.compensations[2].Done)

	require.NoError(t, runtime.restoreCompensations(cleanupCtx, 0))
	assert.Equal(t, []string{"third", "second", "first"}, order)
	assert.True(t, runtime.compensations[0].Done)

	require.NoError(t, runtime.restoreCompensations(cleanupCtx, 0))
	assert.Equal(t, []string{"third", "second", "first"}, order)
	assert.Contains(t, output.String(), "target=third scope=thread-1")
}

func TestYunBeiCompensationsCanRestoreOneTaskOnly(t *testing.T) {
	runtime := newYunBeiRuntime(NewYunBei(&Root{}, nil), nil, nil, 42)

	var order []string

	runtime.addCompensation(yunBeiLikeComment, "comment-unlike", "comment", "thread", func(context.Context) error {
		order = append(order, "comment")
		return nil
	})
	runtime.addCompensation(yunBeiFollowArtist, "artist-unsub", "artist", "", func(context.Context) error {
		order = append(order, "artist")
		return nil
	})

	require.NoError(t, runtime.restoreCompensations(context.Background(), 0, yunBeiLikeComment))
	assert.Equal(t, []string{"comment"}, order)
	assert.True(t, runtime.compensations[0].Done)
	assert.False(t, runtime.compensations[1].Done)

	require.NoError(t, runtime.restoreCompensations(context.Background(), 0))
	assert.Equal(t, []string{"comment", "artist"}, order)
	assert.True(t, runtime.compensations[1].Done)
}

func TestYunBeiCompensationFailureContinuesAndMarksPartial(t *testing.T) {
	command := NewYunBei(&Root{}, nil)

	var output bytes.Buffer
	command.cmd.SetOut(&output)
	runtime := newYunBeiRuntime(command, nil, nil, 42)
	runtime.results[yunBeiLikeComment] = &yunBeiResult{ID: yunBeiLikeComment, State: yunBeiClaimed}
	runtime.results[yunBeiFollowArtist] = &yunBeiResult{ID: yunBeiFollowArtist, State: yunBeiClaimed}

	var order []string

	runtime.addCompensation(yunBeiLikeComment, "comment-unlike", "comment-1", "thread-1", func(context.Context) error {
		order = append(order, "comment")
		return nil
	})

	restoreErr := errors.New("restore rejected")

	runtime.addCompensation(yunBeiFollowArtist, "artist-unsub", "artist-1", "", func(context.Context) error {
		order = append(order, "artist")
		return restoreErr
	})

	err := runtime.restoreCompensations(context.Background(), 0)
	require.ErrorIs(t, err, restoreErr)
	assert.Equal(t, []string{"artist", "comment"}, order)
	assert.True(t, runtime.compensations[0].Done)
	assert.False(t, runtime.compensations[1].Done)
	assert.Equal(t, yunBeiUnconfirmed, runtime.results[yunBeiFollowArtist].State)
	assert.Equal(t, yunBeiClaimed, runtime.results[yunBeiLikeComment].State)
	assert.Contains(t, output.String(), "恢复失败: artist-unsub target=artist-1")
}

func TestYunBeiCanceledCleanupReportsPendingTargets(t *testing.T) {
	command := NewYunBei(&Root{}, nil)

	var output bytes.Buffer
	command.cmd.SetOut(&output)
	runtime := newYunBeiRuntime(command, nil, nil, 42)
	runtime.results[yunBeiLikeSong] = &yunBeiResult{ID: yunBeiLikeSong, State: yunBeiClaimed}

	called := false

	runtime.addCompensation(yunBeiLikeSong, "song-unlike", "song-1", "", func(context.Context) error {
		called = true
		return nil
	})

	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	err := runtime.restoreCompensations(canceled, 0)
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
	assert.False(t, runtime.compensations[0].Done)
	assert.Equal(t, yunBeiUnconfirmed, runtime.results[yunBeiLikeSong].State)
	assert.Contains(t, output.String(), "恢复未完成: song-unlike target=song-1")
}

func TestYunBeiCleanupDeletesOnlyRecordedEventsBeforeCompensation(t *testing.T) {
	client, transport := newSignFlowClient(t, [][]byte{
		encryptSignTestResponse(t, []byte(`{"code":200}`)),
	})
	command := NewYunBei(&Root{}, nil)
	command.opts.Delete = true

	var output bytes.Buffer
	command.cmd.SetOut(&output)
	runtime := newYunBeiRuntime(command, weapi.New(client), eapi.New(client), 42)
	require.NoError(t, runtime.addEventID(99))
	require.NoError(t, runtime.addEventDeleteIntent(99))
	runtime.addCompensation(yunBeiLikeSong, "song-unlike", "song-1", "", func(context.Context) error {
		assert.Equal(t, []string{"/eapi/event/delete"}, transport.paths)
		return nil
	})

	require.NoError(t, runtime.cleanup(context.Background()))
	assert.Equal(t, []string{"/eapi/event/delete"}, transport.paths)
	assert.Equal(t, int64(99), runtime.eventIDs[0])
	assert.Zero(t, runtime.deleteEventIDs[0])
	assert.True(t, runtime.compensations[0].Done)
	assert.Contains(t, output.String(), "动态ID 99 已恢复")

	require.NoError(t, runtime.cleanup(context.Background()))
	assert.Equal(t, []string{"/eapi/event/delete"}, transport.paths)
}

func TestYunBeiCleanupKeepsEventsByDefault(t *testing.T) {
	command := NewYunBei(&Root{}, nil)
	runtime := newYunBeiRuntime(command, nil, nil, 42)
	require.NoError(t, runtime.addEventID(99))

	require.NoError(t, runtime.deletePendingEvents(context.Background()))
	assert.Equal(t, int64(99), runtime.eventIDs[0])
	assert.Empty(t, runtime.deleteEventIDs)
}

func TestTaskYunBeiSelectionAndRunAll(t *testing.T) {
	command := NewTask(&Root{}, nil)
	command.opts.YunBei = true
	command.opts.YunBeiOpts.Tasks = []string{"sign"}

	selection, err := command.taskSelection()
	require.NoError(t, err)
	assert.True(t, selection.YunBei)
	assert.False(t, selection.SignIn)
	require.NoError(t, command.validate())

	command = NewTask(&Root{}, nil)
	command.opts.RunAll = true
	selection, err = command.taskSelection()
	require.NoError(t, err)
	assert.True(t, selection.SignIn)
	assert.True(t, selection.YunBei)
	assert.True(t, selection.Partner)
	assert.True(t, selection.Scrobble)
	assert.True(t, selection.SongShare)
	assert.True(t, selection.FansGroup)
	require.NoError(t, command.validate())
	assert.True(t, command.effectiveYunBeiOpts().All)
}

func TestTaskYunBeiValidation(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Task)
		want  string
	}{
		{
			name: "selector needs scope",
			setup: func(command *Task) {
				require.NoError(t, command.cmd.PersistentFlags().Set("yunbei.title", "title"))
			},
			want: "require --yunbei or --runAll",
		},
		{
			name: "yunbei needs tasks",
			setup: func(command *Task) {
				command.opts.YunBei = true
			},
			want: "no YunBei task selected",
		},
		{
			name: "invalid cron",
			setup: func(command *Task) {
				command.opts.YunBei = true
				command.opts.YunBeiOpts.Tasks = []string{"sign"}
				command.opts.YunBeiCron = "not-a-cron"
			},
			want: "ParseStandard",
		},
		{
			name: "runAll task conflict",
			setup: func(command *Task) {
				command.opts.RunAll = true
				require.NoError(t, command.cmd.PersistentFlags().Set("yunbei.task", "sign"))
			},
			want: "--yunbei.task conflicts",
		},
		{
			name: "runAll explicit selector false",
			setup: func(command *Task) {
				command.opts.RunAll = true
				require.NoError(t, command.cmd.PersistentFlags().Set("yunbei", "false"))
			},
			want: "--yunbei=false conflicts",
		},
		{
			name: "runAll explicit all false",
			setup: func(command *Task) {
				command.opts.RunAll = true
				require.NoError(t, command.cmd.PersistentFlags().Set("yunbei.all", "false"))
			},
			want: "--yunbei.all=false conflicts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := NewTask(&Root{}, nil)
			tt.setup(command)
			require.ErrorContains(t, command.validate(), tt.want)
		})
	}

	command := NewTask(&Root{}, nil)
	command.opts.RunAll = true
	require.NoError(t, command.cmd.PersistentFlags().Set("yunbei.all", "true"))
	require.NoError(t, command.validate())
}

func TestTaskEffectiveYunBeiOptsClonesTasks(t *testing.T) {
	command := NewTask(&Root{}, nil)
	assert.Equal(t, "0 12 * * *", command.opts.YunBeiCron)
	command.opts.YunBeiOpts = YunBeiOpts{
		Tasks:   []string{"sign", "claim"},
		Title:   "title",
		Message: "一二三四五六七八九十",
		Image:   "cover.img",
		Delete:  true,
	}

	got := command.effectiveYunBeiOpts()
	assert.Equal(t, command.opts.YunBeiOpts.Title, got.Title)
	assert.Equal(t, command.opts.YunBeiOpts.Message, got.Message)
	assert.Equal(t, command.opts.YunBeiOpts.Image, got.Image)
	assert.Equal(t, command.opts.YunBeiOpts.Delete, got.Delete)
	got.Tasks[0] = "reserve"
	assert.Equal(t, "sign", command.opts.YunBeiOpts.Tasks[0])
}

func TestScheduledYunBeiRevalidatesImageAtTrigger(t *testing.T) {
	dir := t.TempDir()
	image := filepath.Join(dir, "cover.png")
	require.NoError(t, os.WriteFile(image, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, 0o600))

	logger := newYunBeiTaskTestLogger(t)
	task := NewTask(&Root{}, logger)
	command := NewYunBei(&Root{}, logger)
	command.opts = YunBeiOpts{Tasks: []string{"publish-note"}, Image: image}
	command.cmd.SilenceErrors = true
	command.cmd.SilenceUsage = true
	job := cron.New()

	require.NoError(t, task.registerScheduledCommand(
		context.Background(),
		job,
		"yunbei",
		"0 12 * * *",
		"[yunbei] crontab error",
		command,
	))
	require.NoError(t, os.Remove(image))
	require.NoError(t, os.WriteFile(image, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0}, 0o600))

	require.NotPanics(t, func() {
		job.Entries()[0].Job.Run()
	})
}

func TestSignYunBeiGateRetriesAndRunsAfterRelease(t *testing.T) {
	logger := newYunBeiTaskTestLogger(t)

	var output bytes.Buffer

	task := NewTask(&Root{}, logger)
	task.cmd.SetOut(&output)

	var gate sync.Mutex
	gate.Lock()
	sleepCalls := 0
	task.gateSleep = func(context.Context, time.Duration, ...time.Duration) error {
		sleepCalls++
		gate.Unlock()
		return nil
	}

	executions := 0
	command := fakeScheduledCommand{cmd: &cobra.Command{
		Use: "yunbei-test",
		RunE: func(*cobra.Command, []string) error {
			executions++
			return nil
		},
	}}

	var running atomic.Bool
	task.runScheduledCommand(
		context.Background(),
		"[yunbei]",
		command,
		&scheduledCommandGuard{gate: &gate, running: &running},
	)

	assert.Equal(t, 1, sleepCalls)
	assert.Equal(t, 1, executions)
	assert.False(t, running.Load())
	assert.True(t, gate.TryLock())
	gate.Unlock()
}

func TestSignYunBeiGateSkipsWithoutQueueAndAllowsNextTrigger(t *testing.T) {
	logger := newYunBeiTaskTestLogger(t)

	var output bytes.Buffer

	task := NewTask(&Root{}, logger)
	task.cmd.SetOut(&output)

	var gate sync.Mutex
	gate.Lock()
	sleepCalls := 0
	task.gateSleep = func(context.Context, time.Duration, ...time.Duration) error {
		sleepCalls++
		return nil
	}

	executions := 0
	command := fakeScheduledCommand{cmd: &cobra.Command{
		Use: "sign-test",
		RunE: func(*cobra.Command, []string) error {
			executions++
			return nil
		},
	}}

	var running atomic.Bool

	guard := &scheduledCommandGuard{gate: &gate, running: &running}
	task.runScheduledCommand(context.Background(), "[sign]", command, guard)
	assert.Equal(t, signYunBeiGateRetryCount, sleepCalls)
	assert.Zero(t, executions)
	assert.False(t, running.Load())
	assert.Contains(t, output.String(), "已重试 3 次并跳过，不会补跑")

	gate.Unlock()
	task.runScheduledCommand(context.Background(), "[sign]", command, guard)
	assert.Equal(t, 1, executions)
}

func TestScheduledYunBeiDuplicateTriggerIsRejected(t *testing.T) {
	logger := newYunBeiTaskTestLogger(t)

	var output bytes.Buffer

	task := NewTask(&Root{}, logger)
	task.cmd.SetOut(&output)

	var gate sync.Mutex
	gate.Lock()
	waiting := make(chan struct{})
	resume := make(chan struct{})
	task.gateSleep = func(ctx context.Context, _ time.Duration, _ ...time.Duration) error {
		close(waiting)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-resume:
			return nil
		}
	}

	executions := 0
	command := fakeScheduledCommand{cmd: &cobra.Command{
		Use: "yunbei-test",
		RunE: func(*cobra.Command, []string) error {
			executions++
			return nil
		},
	}}

	var running atomic.Bool

	guard := &scheduledCommandGuard{gate: &gate, running: &running}

	done := make(chan struct{})
	go func() {
		defer close(done)

		task.runScheduledCommand(context.Background(), "[yunbei]", command, guard)
	}()

	<-waiting

	task.runScheduledCommand(
		context.Background(),
		"[yunbei]",
		command,
		guard,
	)
	assert.Zero(t, executions)
	assert.Contains(t, output.String(), "同名任务仍在运行或等待")

	gate.Unlock()
	close(resume)
	<-done
	assert.Equal(t, 1, executions)
	assert.False(t, running.Load())
}

func TestTaskShutdownCancelsBeforeStopAndWaits(t *testing.T) {
	task := NewTask(&Root{}, nil)
	runCtx, cancel := context.WithCancel(context.Background())
	doneCtx, done := context.WithCancel(context.Background())

	var steps []string

	task.withTimeout = func(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
		assert.Equal(t, taskShutdownTimeout, timeout)
		return context.WithCancel(parent)
	}

	err := task.stopScheduledTasks(context.Background(), func() {
		steps = append(steps, "cancel")

		cancel()
	}, func() context.Context {
		steps = append(steps, "stop")

		require.ErrorIs(t, runCtx.Err(), context.Canceled)
		done()
		return doneCtx
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"cancel", "stop"}, steps)
}

func TestTaskShutdownTimeoutIsBounded(t *testing.T) {
	task := NewTask(&Root{}, nil)
	task.withTimeout = func(context.Context, time.Duration) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, func() {}
	}

	err := task.stopScheduledTasks(context.Background(), func() {}, context.Background)
	require.ErrorContains(t, err, "wait for scheduled tasks after 1m5s")
}

func newYunBeiTaskTestLogger(t *testing.T) *projectlog.Logger {
	t.Helper()

	logger := projectlog.New(&projectlog.Config{Level: "error"})

	t.Cleanup(func() { require.NoError(t, logger.Close()) })
	return logger
}

func testYunBeiSelection(ids ...yunBeiTaskID) map[yunBeiTaskID]struct{} {
	selected := make(map[yunBeiTaskID]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	return selected
}

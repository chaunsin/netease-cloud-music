// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
)

var yunBeiCalendarLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

const (
	yunBeiReserveNoPreviousNoBook = "NO_PREV_NO_BOOK"
	yunBeiReserveNoPreviousBooked = "NO_PREV_BOOKED"
	yunBeiReserveClaimedNoBook    = "PREV_CLAIMED_NO_BOOKED"
	yunBeiReserveBookedUnclaimed  = "PREV_BOOKED_UNCLAIMED"
	yunBeiReserveClaimedBooked    = "PREV_CLAIMED_BOOKED"
)

func (runtime *yunBeiRuntime) loadReserveInfo(ctx context.Context) (*eapi.YunbeiReserveInfoRespData, error) {
	reply, err := runtime.eapi.YunbeiReserveInfo(ctx, &eapi.YunbeiReserveInfoReq{})
	if err != nil {
		return nil, fmt.Errorf("YunbeiReserveInfo: %w", err)
	}

	if reply.Code != 200 || !reply.Success {
		return nil, fmt.Errorf(
			"YunbeiReserveInfo: code=%d success=%t message=%q",
			reply.Code,
			reply.Success,
			reply.Message,
		)
	}
	return &reply.Data, nil
}

func runYunBeiReserve(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	info, err := runtime.loadReserveInfo(ctx)
	if err != nil {
		return err
	}

	for range 3 {
		status := strings.TrimSpace(info.Type)
		switch status {
		case "":
			snapshot.State = yunBeiUnavailable
			return nil
		case yunBeiReserveNoPreviousBooked, yunBeiReserveClaimedBooked:
			snapshot.State = yunBeiClaimed

			runtime.println("  预约活动: 已预约，本期无需操作")
			return nil
		case yunBeiReserveNoPreviousNoBook, yunBeiReserveClaimedNoBook:
			requestID := strings.TrimSpace(info.ReqId)
			if requestID == "" {
				return fmt.Errorf("YunbeiReserveInfo: status %s is missing reqId", status)
			}

			if !runtime.beginAction(yunBeiActionKey{TaskID: yunBeiReserve, Kind: "book", TargetID: requestID}) {
				snapshot.State = yunBeiUnconfirmed
				return errors.New("reserve booking was already attempted without a confirmed state transition")
			}

			booked, bookErr := runtime.eapi.YunbeiReserveBooked(ctx, &eapi.YunbeiReserveBookedReq{ReqId: requestID})
			if bookErr != nil {
				return fmt.Errorf("YunbeiReserveBooked: %w", bookErr)
			}

			if booked.Code != 200 || !booked.Success {
				return fmt.Errorf(
					"YunbeiReserveBooked: code=%d success=%t message=%q",
					booked.Code,
					booked.Success,
					booked.Message,
				)
			}

			runtime.println("  预约活动: 预约请求已接受")
		case yunBeiReserveBookedUnclaimed:
			requestID, checkToken := strings.TrimSpace(info.ReqId), strings.TrimSpace(info.CheckToken)
			if requestID == "" || checkToken == "" {
				return fmt.Errorf("YunbeiReserveInfo: status %s is missing reqId or checkToken", status)
			}

			if !runtime.beginAction(yunBeiActionKey{TaskID: yunBeiReserve, Kind: "claim", TargetID: requestID}) {
				snapshot.State = yunBeiUnconfirmed
				return errors.New("reserve reward claim was already attempted without a confirmed state transition")
			}

			received, receiveErr := runtime.eapi.YunbeiReserveRewardReceive(ctx, &eapi.YunbeiReserveRewardReceiveReq{
				ReqId:      requestID,
				CheckToken: checkToken,
			})
			if receiveErr != nil {
				return fmt.Errorf("YunbeiReserveRewardReceive: %w", receiveErr)
			}

			if received.Code != 200 || !received.Success {
				return fmt.Errorf(
					"YunbeiReserveRewardReceive: code=%d success=%t message=%q",
					received.Code,
					received.Success,
					received.Message,
				)
			}

			runtime.printf("  预约活动: 已领取 %d 云贝\n", received.Data.CurrentAmount)
		default:
			snapshot.State = yunBeiUnsupported
			return fmt.Errorf("YunbeiReserveInfo: unknown reserve status %q", status)
		}

		info, err = runtime.loadReserveInfo(ctx)
		if err != nil {
			snapshot.State = yunBeiUnconfirmed
			return fmt.Errorf("refresh reserve info: %w", err)
		}
	}

	snapshot.State = yunBeiUnconfirmed
	return errors.New("reserve state did not converge after 3 actions")
}

func unsupportedYunBeiTask(gate string) func(context.Context, *yunBeiRuntime, *yunBeiTaskSnapshot) error {
	return func(_ context.Context, _ *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
		snapshot.State = yunBeiUnsupported
		return fmt.Errorf("接口未对齐（协议门禁 %s 未关闭）", gate)
	}
}

func runYunBeiSign(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	reply, err := runtime.weapi.YunBeiSignIn(ctx, &weapi.YunBeiSignInReq{})
	if err != nil {
		return fmt.Errorf("YunBeiSignIn: %w", err)
	}

	if reply.Code != 200 {
		return fmt.Errorf("YunBeiSignIn: code=%d message=%q", reply.Code, reply.Message)
	}

	if reply.Data.Sign {
		runtime.println("  云贝签到: 成功")
	} else {
		runtime.println("  云贝签到: 已签到")
	}

	progress, err := runtime.loadSignProgress(ctx)
	if err != nil {
		return err
	}

	type baseClaim struct {
		signDay int64
		name    string
	}

	var (
		claims         []baseClaim
		validationErrs []error
		resultErrs     []error
	)

	for itemIndex := range progress.Data.LotteryConfig {
		item := &progress.Data.LotteryConfig[itemIndex]

		baseState, stateErr := classifyYunBeiLotteryState(item.BaseLotteryStatus, item.BaseLotteryId)
		if stateErr != nil {
			validationErrs = append(validationErrs, fmt.Errorf("sign day %d base: %w", item.SignDay, stateErr))
		} else if baseState == yunBeiLotteryClaimable {
			claims = append(claims, baseClaim{
				signDay: item.SignDay,
				name:    item.BaseGrant.Name,
			})
		}

		extraState, stateErr := classifyYunBeiLotteryState(item.ExtraLotteryStatus, item.ExtraLotteryId)
		if stateErr != nil {
			validationErrs = append(validationErrs, fmt.Errorf("sign day %d extra: %w", item.SignDay, stateErr))
		} else if extraState == yunBeiLotteryClaimable {
			resultErrs = append(resultErrs, fmt.Errorf(
				"sign day %d extra lottery is claimable but its claim path requires protocol evidence (gate G2)",
				item.SignDay,
			))
		}
	}

	if len(validationErrs) != 0 {
		snapshot.State = yunBeiUnconfirmed
		return errors.Join(validationErrs...)
	}

	for _, claim := range claims {
		if ctxErr := ctx.Err(); ctxErr != nil {
			resultErrs = append(resultErrs, ctxErr)
			break
		}

		currentID := int64(0)
		currentState := yunBeiLotteryUnavailable

		for itemIndex := range progress.Data.LotteryConfig {
			item := &progress.Data.LotteryConfig[itemIndex]
			if item.SignDay != claim.signDay {
				continue
			}

			currentID = item.BaseLotteryId

			currentState, err = classifyYunBeiLotteryState(item.BaseLotteryStatus, item.BaseLotteryId)
			if err != nil {
				resultErrs = append(resultErrs, fmt.Errorf("sign day %d base before claim: %w", claim.signDay, err))
			}
			break
		}

		if currentState == yunBeiLotteryClaimed {
			continue
		}

		if currentState != yunBeiLotteryClaimable || currentID <= 0 {
			resultErrs = append(resultErrs, fmt.Errorf("sign day %d base reward is no longer claimable", claim.signDay))
			continue
		}

		key := yunBeiActionKey{
			TaskID:   yunBeiSign,
			Kind:     "base-lottery",
			TargetID: strconv.FormatInt(currentID, 10),
		}
		if !runtime.beginAction(key) {
			resultErrs = append(resultErrs, fmt.Errorf("base lottery %d was already attempted", currentID))
			continue
		}

		lottery, claimErr := runtime.weapi.YunBeiSignLottery(ctx, &weapi.YunBeiSignLotteryReq{
			UserLotteryId: strconv.FormatInt(currentID, 10),
		})
		if claimErr != nil {
			resultErrs = append(resultErrs, fmt.Errorf("YunBeiSignLottery(%d): %w", currentID, claimErr))
			continue
		}

		if lottery.Code != 200 || !lottery.Data {
			resultErrs = append(resultErrs, fmt.Errorf(
				"YunBeiSignLottery(%d): code=%d data=%t message=%q",
				currentID,
				lottery.Code,
				lottery.Data,
				lottery.Message,
			))
			continue
		}

		refreshed, refreshErr := runtime.loadSignProgress(ctx)
		if refreshErr != nil {
			resultErrs = append(resultErrs, fmt.Errorf("refresh sign day %d: %w", claim.signDay, refreshErr))
			continue
		}

		confirmed := false

		for itemIndex := range refreshed.Data.LotteryConfig {
			item := &refreshed.Data.LotteryConfig[itemIndex]
			if item.SignDay != claim.signDay {
				continue
			}

			state, stateErr := classifyYunBeiLotteryState(item.BaseLotteryStatus, item.BaseLotteryId)
			confirmed = stateErr == nil && state == yunBeiLotteryClaimed
			break
		}

		if !confirmed {
			resultErrs = append(resultErrs, fmt.Errorf(
				"YunBeiSignLottery(%d): refreshed sign day %d was not claimed",
				currentID,
				claim.signDay,
			))
			continue
		}

		progress = refreshed

		runtime.printf("  云贝连续签到: 第 %d 天（奖励「%s」已领取）\n", claim.signDay, claim.name)
	}

	if len(resultErrs) != 0 {
		snapshot.State = yunBeiUnconfirmed
		return errors.Join(resultErrs...)
	}

	snapshot.State = yunBeiClaimed
	return nil
}

func (runtime *yunBeiRuntime) loadSignProgress(ctx context.Context) (*weapi.YunBeiSignInProgressResp, error) {
	progress, err := runtime.weapi.YunBeiSignInProgress(ctx, &weapi.YunBeiSignInProgressReq{})
	if err != nil {
		return nil, fmt.Errorf("YunBeiSignInProgress: %w", err)
	}

	if progress.Code != 200 {
		return nil, fmt.Errorf("YunBeiSignInProgress: code=%d message=%q", progress.Code, progress.Message)
	}
	return progress, nil
}

type yunBeiLotteryState uint8

const (
	yunBeiLotteryUnavailable yunBeiLotteryState = iota
	yunBeiLotteryClaimable
	yunBeiLotteryClaimed
)

func classifyYunBeiLotteryState(status, lotteryID int64) (yunBeiLotteryState, error) {
	switch {
	case status == 0 && lotteryID == 0:
		return yunBeiLotteryUnavailable, nil
	case status == 0 && lotteryID > 0:
		return yunBeiLotteryClaimable, nil
	case status == 1 && lotteryID > 0:
		return yunBeiLotteryClaimed, nil
	default:
		return yunBeiLotteryUnavailable, fmt.Errorf("unsupported lottery state status=%d id=%d", status, lotteryID)
	}
}

func (runtime *yunBeiRuntime) loadBlockedClaimIDs(ctx context.Context, todos []eapi.YunBeiTaskTodoRespData) (map[int64]string, error) {
	unsafeTasks := make(map[yunBeiTaskID]yunBeiTaskState)

	for taskID, result := range runtime.results {
		if result == nil || taskID == yunBeiSign || taskID == yunBeiReserve || taskID == yunBeiClaim {
			continue
		}

		switch result.State {
		case yunBeiPending, yunBeiRunning, yunBeiUnconfirmed, yunBeiFailed, yunBeiUnsupported:
			unsafeTasks[taskID] = result.State
		case yunBeiUnavailable, yunBeiCompletedUnclaimed, yunBeiClaimed:
		}
	}

	if len(unsafeTasks) == 0 {
		return map[int64]string{}, nil
	}

	list, err := runtime.weapi.YunBeiTaskListV3(ctx, &weapi.YunBeiTaskListV3Req{})
	if err != nil {
		return nil, fmt.Errorf("load V3 claim safety state: %w", err)
	}

	if list.Code != 200 {
		return nil, fmt.Errorf("load V3 claim safety state: code=%d message=%q", list.Code, list.Message)
	}

	if len(list.Data.UnknownBucket) != 0 {
		return nil, errors.New("load V3 claim safety state: response contains non-empty untyped buckets")
	}

	state := correlateYunBeiTasks(list.Data.Normal.List, todos, yunBeiTaskIdentities())
	blocked := make(map[int64]string)

	for _, task := range state.Tasks {
		if task.Claim == nil || task.Claim.UserTaskID <= 0 {
			continue
		}

		if task.Conflicted {
			blocked[task.Claim.UserTaskID] = "task correlation is conflicted"
			continue
		}

		if unsafeState, exists := unsafeTasks[task.ID]; exists {
			blocked[task.Claim.UserTaskID] = fmt.Sprintf("task %s result is %s", task.ID, unsafeState)
		}
	}

	for _, todo := range state.UnmatchedTodos {
		if todo.Completed && todo.UserTaskId > 0 {
			blocked[todo.UserTaskId] = "todo ownership cannot be established while another selected task is unsafe"
		}
	}
	return blocked, nil
}

func runYunBeiClaim(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	todo, err := runtime.eapi.YunBeiTaskTodo(ctx, &eapi.YunBeiTaskTodoReq{})
	if err != nil {
		return fmt.Errorf("YunBeiTaskTodo: %w", err)
	}

	if todo.Code != 200 {
		return fmt.Errorf("YunBeiTaskTodo: code=%d message=%q", todo.Code, todo.Message)
	}

	blockedClaims, err := runtime.loadBlockedClaimIDs(ctx, todo.Data)
	if err != nil {
		return err
	}

	var (
		claimed        int
		errs           []error
		currentTodos   = todo.Data
		userTaskCounts = make(map[int64]int, len(todo.Data))
		reported       = make(map[int64]struct{})
	)

	for _, item := range todo.Data {
		if item.UserTaskId > 0 {
			userTaskCounts[item.UserTaskId]++
		}
	}

	for _, original := range todo.Data {
		if !original.Completed {
			continue
		}

		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}

		if original.UserTaskId <= 0 {
			errs = append(errs, fmt.Errorf("task %d: claim fields are missing or invalid", original.UserTaskId))
			continue
		}

		if userTaskCounts[original.UserTaskId] > 1 {
			if _, exists := reported[original.UserTaskId]; !exists {
				errs = append(errs, fmt.Errorf("task %d: duplicate todo records", original.UserTaskId))
				reported[original.UserTaskId] = struct{}{}
			}
			continue
		}

		if reason, blocked := blockedClaims[original.UserTaskId]; blocked {
			errs = append(errs, fmt.Errorf("task %d: reward claim blocked because %s", original.UserTaskId, reason))
			continue
		}

		var (
			item    *eapi.YunBeiTaskTodoRespData
			matches int
		)

		for index := range currentTodos {
			if currentTodos[index].UserTaskId == original.UserTaskId {
				item = &currentTodos[index]
				matches++
			}
		}

		if matches > 1 {
			errs = append(errs, fmt.Errorf("task %d: duplicate refreshed todo records", original.UserTaskId))
			continue
		}

		if item == nil || !item.Completed {
			continue
		}

		claim := *item
		// period=0 是合法值（2026-10-30 实测「特殊福利」todo 携带 period=0/depositCode=2111），
		// 按服务端值原样发送；userTaskId 已在上方校验。
		if claim.DepositCode <= 0 {
			errs = append(errs, fmt.Errorf("task %d: claim fields are missing or invalid", claim.UserTaskId))
			continue
		}

		if !runtime.beginAction(yunBeiActionKey{
			TaskID:   yunBeiClaim,
			Kind:     "claim",
			TargetID: strconv.FormatInt(claim.UserTaskId, 10),
		}) {
			errs = append(errs, fmt.Errorf("task %d: reward claim was already attempted", claim.UserTaskId))
			continue
		}

		reply, finishErr := runtime.weapi.YunBeiTaskFinish(ctx, &weapi.YunBeiTaskFinishReq{
			Period:      strconv.FormatInt(claim.Period, 10),
			UserTaskId:  strconv.FormatInt(claim.UserTaskId, 10),
			DepositCode: strconv.FormatInt(claim.DepositCode, 10),
		})
		if finishErr != nil {
			errs = append(errs, fmt.Errorf("YunBeiTaskFinish(%d): %w", claim.UserTaskId, finishErr))
			continue
		}

		if reply.Code != 200 || !reply.Data {
			errs = append(errs, fmt.Errorf("YunBeiTaskFinish(%d): code=%d data=%t", claim.UserTaskId, reply.Code, reply.Data))
			continue
		}

		refreshed, refreshErr := runtime.eapi.YunBeiTaskTodo(ctx, &eapi.YunBeiTaskTodoReq{})
		if refreshErr != nil {
			errs = append(errs, fmt.Errorf("refresh YunBeiTaskTodo(%d): %w", claim.UserTaskId, refreshErr))
			break
		}

		if refreshed.Code != 200 {
			errs = append(errs, fmt.Errorf("refresh YunBeiTaskTodo(%d): code=%d message=%q", claim.UserTaskId, refreshed.Code, refreshed.Message))
			break
		}

		currentTodos = refreshed.Data
		confirmed := true

		for _, candidate := range currentTodos {
			if candidate.UserTaskId == claim.UserTaskId && candidate.Completed {
				confirmed = false
				break
			}
		}

		if !confirmed {
			errs = append(errs, fmt.Errorf("YunBeiTaskFinish(%d): reward claim was not confirmed", claim.UserTaskId))
			continue
		}

		claimed++

		runtime.printf("  云贝任务: [%s] 获得 %d 云贝\n", claim.TaskName, claim.TaskPoint)
	}

	if claimed == 0 && len(errs) == 0 {
		snapshot.State = yunBeiUnavailable
	} else if claimed > 0 {
		snapshot.State = yunBeiClaimed
	}

	if len(errs) != 0 {
		snapshot.State = yunBeiUnconfirmed
	}

	return errors.Join(errs...)
}

func yunBeiSignedToday(data *weapi.YunBeiSignInCalendarRespData) (bool, error) {
	if data == nil || data.CurTimeStamp <= 0 {
		return false, errors.New("YunBeiSignInCalendar: server timestamp is missing or invalid")
	}

	serverTime := time.UnixMilli(data.CurTimeStamp).In(yunBeiCalendarLocation)

	day := serverTime.Day()
	if len(data.SignStr) < day || len(data.SignStr) > 31 {
		return false, fmt.Errorf(
			"YunBeiSignInCalendar: signStr length %d cannot represent day %d",
			len(data.SignStr),
			day,
		)
	}

	for _, value := range []byte(data.SignStr) {
		if value != '0' && value != '1' {
			return false, fmt.Errorf("YunBeiSignInCalendar: signStr contains invalid value %q", value)
		}
	}
	return data.SignStr[day-1] == '1', nil
}

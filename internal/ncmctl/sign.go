// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type SignInOpts struct {
	Automatic bool
}

type SignIn struct {
	root *Root
	cmd  *cobra.Command
	l    *log.Logger
	opts SignInOpts
}

func NewSignIn(root *Root, l *log.Logger) *SignIn {
	c := &SignIn{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:   "sign",
			Short: "Run YunBei and VIP daily sign-in actions",
			Long: "Perform the YunBei and VIP sign-in actions once. Login is required; VIP sign-in " +
				"does not require an active VIP entitlement. --automatic also claims available YunBei " +
				"and eligible VIP rewards, which performs " +
				"additional account actions and may increase risk-control exposure.",
			Example: "  ncmctl sign\n" +
				"  ncmctl sign --automatic",
			Args: cobra.NoArgs,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context())
	}
	return c
}

func (c *SignIn) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *SignIn) Command() *cobra.Command {
	return c.cmd
}

func (c *SignIn) addFlags() {
	c.cmd.Flags().BoolVarP(&c.opts.Automatic, "automatic", "a", false, "claim available YunBei and eligible VIP rewards after sign-in")
}

func (c *SignIn) validate() error {
	return nil
}

func (c *SignIn) execute(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	c.cmd.Printf("\n🔔 签到任务\n\n")

	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer closeAPIClient(ctx, cli, c.l)

	request := weapi.New(cli)

	// 判断是否需要登录
	if request.NeedLogin(ctx) {
		return errors.New("need login")
	}

	if err := c.executeYunBeiSign(ctx, request); err != nil {
		return err
	}

	if err := c.executeVipSign(ctx, request, eapi.New(cli), time.Now().UnixMilli()); err != nil {
		return err
	}

	c.cmd.Printf("\n签到完成\n")
	return nil
}

// executeYunBeiSign 执行云贝签到.
func (c *SignIn) executeYunBeiSign(ctx context.Context, request *weapi.Api) error {
	resp, err := request.YunBeiSignIn(ctx, &weapi.YunBeiSignInReq{})
	if err != nil {
		return fmt.Errorf("YunBeiSignIn: %w", err)
	}

	if resp.Code != 200 {
		return fmt.Errorf("YunBeiSignIn: %+v", resp)
	}

	if resp.Data.Sign {
		c.cmd.Println("  云贝签到: 成功")
	} else {
		c.cmd.Println("  云贝签到: 已签到")
	}

	// 获取签到进度
	if c.opts.Automatic {
		progress, progressErr := request.YunBeiSignInProgress(ctx, &weapi.YunBeiSignInProgressReq{})
		if progressErr != nil {
			return fmt.Errorf("YunBeiSignInProgress: %w", progressErr)
		}

		for _, v := range progress.Data.LotteryConfig {
			if v.BaseLotteryId <= 0 && v.ExtraLotteryId <= 0 {
				continue
			}

			c.l.Debugf("天数=%v,奖励内容=%v,id=%v,extId=%v,status=%v",
				v.SignDay, v.BaseGrant.Name, v.BaseLotteryId, v.ExtraLotteryId, v.BaseLotteryStatus)
			// 领取奖励
			reply, lotteryErr := request.YunBeiSignLottery(ctx, &weapi.YunBeiSignLotteryReq{
				UserLotteryId: strconv.FormatInt(v.BaseLotteryId, 10),
			})
			if lotteryErr != nil {
				c.l.Errorf("YunBeiSignLottery(%v): %s", v.BaseLotteryId, lotteryErr)
			}

			if reply.Data {
				c.cmd.Printf("  云贝连续签到: 第 %v 天（奖励「%v」已领取）\n", v.SignDay, v.BaseGrant.Name)
			}
			// Pending: 满勤签到领取抽奖机会使用ExtraLotteryId,同时也是YunBeiSignLottery方法?
		}

		// 领取奖励
		if err = yunbeiClaim(ctx, request, c.l, c.cmd); err != nil {
			return fmt.Errorf("yunbeiClaim: %w", err)
		}
	}

	return nil
}

// executeVipSign 执行VIP签到.
func (c *SignIn) executeVipSign(ctx context.Context, weapiRequest *weapi.Api, eapiRequest *eapi.Api, signDayTime int64) error {
	vip, err := weapiRequest.VipGrowPoint(ctx, &weapi.VipGrowPointReq{})
	if err != nil {
		return fmt.Errorf("VipGrowPoint: %w", err)
	}

	if vip.Code != 200 {
		return fmt.Errorf("VipGrowPoint: code=%d message=%q", vip.Code, vip.Message)
	}

	// VIP entitlement gates growth rewards only; Music Sign is available without it.
	hasVipEntitlement := vip.Data.UserLevel.LatestVipStatus == 1
	if !hasVipEntitlement {
		c.cmd.Println("  VIP 权益: 暂无（仅执行乐签）")
	}

	maxLevel := vip.Data.UserLevel.MaxLevel
	if maxLevel {
		c.cmd.Println("  VIP 等级: 已满级")
	}

	vipSign, err := eapiRequest.VipTaskSign(ctx, &eapi.VipTaskSignReq{})
	if err != nil {
		return fmt.Errorf("VipTaskSign: %w", err)
	}

	if vipSign.Code != 200 {
		return fmt.Errorf("VipTaskSign: code=%d message=%q", vipSign.Code, vipSign.Message)
	}

	if vipSign.Data {
		c.cmd.Println("  黑胶乐签: 成功")
	} else if message := strings.TrimSpace(vipSign.Message); message != "" {
		c.cmd.Printf("  黑胶乐签: %s\n", message)
	} else {
		c.cmd.Println("  黑胶乐签: 本次未完成")
	}

	// The desktop client refreshes the detail and both card variants after signing.
	detail, err := eapiRequest.VipCheckinHistoryDetail(ctx, &eapi.VipCheckinHistoryDetailReq{
		SignDayTime: strconv.FormatInt(signDayTime, 10),
		Type:        "1",
	})
	if err != nil {
		return fmt.Errorf("VipCheckinHistoryDetail after VipTaskSign: %w", err)
	}

	if detail.Code != 200 {
		return fmt.Errorf("VipCheckinHistoryDetail after VipTaskSign: code=%d message=%q", detail.Code, detail.Message)
	}

	// TDDO: 每次请求的歌曲都不一样原因,官方每次都是一样得和type=2有关.
	if detail.Data.SongInfo != nil {
		songName := strings.TrimSpace(detail.Data.SongInfo.SongName)
		artistName := strings.TrimSpace(detail.Data.SongInfo.ArtistName)

		switch {
		case songName != "" && artistName != "":
			c.cmd.Printf("    今日歌曲: %s - %s\n", songName, artistName)
		case songName != "":
			c.cmd.Printf("    今日歌曲: %s\n", songName)
		case artistName != "":
			c.cmd.Printf("    今日歌手: %s\n", artistName)
		}
	}

	var monthCard eapi.VipMinideskMusicSignPCData

	for _, typ := range []int{0, 1} {
		card, cardErr := eapiRequest.VipMinideskMusicSignPC(ctx, &eapi.VipMinideskMusicSignPCReq{Type: typ})
		if cardErr != nil {
			return fmt.Errorf("VipMinideskMusicSignPC(type=%d) after VipTaskSign: %w", typ, cardErr)
		}

		if card.Code != 200 {
			return fmt.Errorf(
				"VipMinideskMusicSignPC(type=%d) after VipTaskSign: code=%d message=%q",
				typ,
				card.Code,
				card.Message,
			)
		}

		if typ == 1 {
			monthCard = card.Data
		}
	}

	monthTitle := "本月黑胶乐签"
	if monthCard.Text != nil && strings.TrimSpace(*monthCard.Text) != "" {
		monthTitle = strings.TrimSpace(*monthCard.Text)
	}

	monthTip := strings.TrimSpace(monthCard.SubText)

	switch {
	case detail.Data.MonthCheckInTotalDay > 0 && monthTip != "":
		c.cmd.Printf("    %s: 已签 %d 天，%s\n", monthTitle, detail.Data.MonthCheckInTotalDay, monthTip)
	case detail.Data.MonthCheckInTotalDay > 0:
		c.cmd.Printf("    %s: 已签 %d 天\n", monthTitle, detail.Data.MonthCheckInTotalDay)
	case monthTip != "":
		c.cmd.Printf("    %s: %s\n", monthTitle, monthTip)
	}

	if c.opts.Automatic && hasVipEntitlement && !maxLevel {
		reward, rewardErr := weapiRequest.VipRewardGetAll(ctx, &weapi.VipRewardGetAllReq{})
		if rewardErr != nil {
			return fmt.Errorf("VipRewardGetAll: %w", rewardErr)
		}

		if reward.Data.Result {
			c.cmd.Println("  VIP 成长值: 领取成功")
		} else if message := strings.TrimSpace(reward.Message); message != "" {
			c.cmd.Printf("  VIP 成长值: %s\n", message)
		} else {
			c.cmd.Println("  VIP 成长值: 未领取")
		}
	}

	// 刷新token过期时间
	refresh, refreshErr := weapiRequest.TokenRefresh(ctx, &weapi.TokenRefreshReq{})
	if refreshErr != nil {
		c.l.Warnf("TokenRefresh: %v", refreshErr)
	} else if refresh.Code != 200 {
		c.l.Warnf("TokenRefresh: code=%d message=%q", refresh.Code, refresh.Message)
	}
	return nil
}

// yunbeiClaim 完成当前时刻可以领取的任务奖励.
func yunbeiClaim(ctx context.Context, request *weapi.Api, l *log.Logger, cmd *cobra.Command) error {
	task, err := request.YunBeiTaskTodo(ctx, &weapi.YunBeiTaskTodoReq{})
	if err != nil {
		return fmt.Errorf("YunBeiTaskTodo: %w", err)
	}

	for _, v := range task.Data {
		if !v.Completed {
			continue
		}

		reply, finishErr := request.YunBeiTaskFinish(ctx, &weapi.YunBeiTaskFinishReq{
			Period:      strconv.FormatInt(v.Period, 10),
			UserTaskId:  strconv.FormatInt(v.UserTaskId, 10),
			DepositCode: strconv.FormatInt(v.DepositCode, 10),
		})
		if finishErr != nil {
			l.Errorf("YunBeiTaskFinish(%v): %s", v.UserTaskId, finishErr)
		}

		if reply.Code != 200 {
			l.Errorf("YunBeiTaskFinish(%v) detail:%+v", v.UserTaskId, reply)
		} else {
			cmd.Printf("  云贝任务: [%s] 获得 %v 云贝\n", v.TaskName, v.TaskPoint)
		}
	}
	return nil
}

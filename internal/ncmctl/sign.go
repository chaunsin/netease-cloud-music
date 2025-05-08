// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package ncmctl

import (
	"context"
	"fmt"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/spf13/cobra"
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
			Use:     "sign",
			Short:   "[need login] Sign perform daily cloud shell check-in",
			Example: `  ncmctl sign`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context())
	}
	return c
}

func (c *SignIn) addFlags() {
	c.cmd.Flags().BoolVarP(&c.opts.Automatic, "automatic", "a", true, "automatically claim sign-in rewards")
}

func (c *SignIn) validate() error {
	return nil
}

func (c *SignIn) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *SignIn) Command() *cobra.Command {
	return c.cmd
}

func (c *SignIn) execute(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)
	request := weapi.New(cli)

	// 判断是否需要登录
	if request.NeedLogin(ctx) {
		return fmt.Errorf("need login")
	}

	// 执行云贝签到
	resp, err := request.YunBeiSignIn(ctx, &weapi.YunBeiSignInReq{})
	if err != nil {
		return fmt.Errorf("YunBeiSignIn: %w", err)
	}
	if resp.Code != 200 {
		return fmt.Errorf("YunBeiSignIn: %+v", resp)
	}
	if resp.Data.Sign {
		c.cmd.Println("云贝签到成功")
	} else {
		c.cmd.Println("云贝已签到")
	}

	// 获取签到进度
	if c.opts.Automatic {
		progress, err := request.YunBeiSignInProgress(ctx, &weapi.YunBeiSignInProgressReq{})
		if err != nil {
			return fmt.Errorf("YunBeiSignInProgress: %w", err)
		}
		for _, v := range progress.Data.LotteryConfig {
			if v.BaseLotteryId <= 0 && v.ExtraLotteryId <= 0 {
				continue
			}
			log.Debug("天数=%v,奖励内容=%v,id=%v,extId=%v,status=%v",
				v.SignDay, v.BaseGrant.Name, v.BaseLotteryId, v.ExtraLotteryId, v.BaseLotteryStatus)
			// 领取奖励
			reply, err := request.YunBeiSignLottery(ctx, &weapi.YunBeiSignLotteryReq{
				UserLotteryId: fmt.Sprintf("%d", v.BaseLotteryId),
			})
			if err != nil {
				log.Error("YunBeiSignLottery(%v): %w", v.BaseLotteryId, err)
			}
			if reply.Data {
				c.cmd.Printf("云贝连续签到天数=%v,奖励内容=%v 领取成功\n", v.SignDay, v.BaseGrant.Name)
			}
			// todo: 满勤签到领取抽奖机会使用ExtraLotteryId,同时也是YunBeiSignLottery方法?
		}

		// 完成当前时刻可以领取的任务奖励
		task, err := request.YunBeiTaskTodo(ctx, &weapi.YunBeiTaskTodoReq{})
		if err != nil {
			return fmt.Errorf("YunBeiTaskTodo: %w", err)
		}
		for _, v := range task.Data {
			if !v.Completed {
				continue
			}

			reply, err := request.YunBeiTaskFinish(ctx, &weapi.YunBeiTaskFinishReq{
				Period:      fmt.Sprintf("%d", v.Period),
				UserTaskId:  fmt.Sprintf("%d", v.UserTaskId),
				DepositCode: fmt.Sprintf("%d", v.DepositCode),
			})
			if err != nil {
				log.Error("YunBeiTaskFinish(%v): %w", v.UserTaskId, err)
			}
			if reply.Code != 200 {
				log.Error("YunBeiTaskFinish(%v) detail:%+v", v.UserTaskId, reply)
			} else {
				c.cmd.Printf("云贝 [%s] 任务完成获得云贝数量 %v\n", v.TaskName, v.TaskPoint)
			}
		}
	}

	// 查询vip权益
	vip, err := request.VipGrowPoint(ctx, &weapi.VipGrowPointReq{})
	if err != nil {
		return fmt.Errorf("VipGrowPoint: %w", err)
	}
	if vip.Code != 200 {
		return fmt.Errorf("VipGrowPoint: %+v", vip)
	}
	if vip.Data.UserLevel.LatestVipStatus != 1 {
		c.cmd.Printf("vip status invalid: %v\n", vip.Data.UserLevel.LatestVipStatus)
		return nil
	}
	if vip.Data.UserLevel.MaxLevel {
		c.cmd.Println("vip等级已达到最大值")
		return nil
	}

	// 黑胶乐签
	vipSign, err := request.VipTaskSign(ctx, &weapi.VipTaskSignReq{IsNew: ""}) // 使用isNew=1?
	if err != nil {
		return fmt.Errorf("VipTaskSign: %w", err)
	}
	if vipSign.Data {
		c.cmd.Println("vip乐签成功")
	} else {
		c.cmd.Printf("vip乐签失败: %+v", vipSign)
	}

	// 领取当前时刻所有可领得成长值
	if c.opts.Automatic {
		reward, err := request.VipRewardGetAll(ctx, &weapi.VipRewardGetAllReq{})
		if err != nil {
			return fmt.Errorf("VipRewardGetAll: %w", err)
		}
		if reward.Data.Result {
			c.cmd.Println("vip成长值领取成功")
		} else {
			c.cmd.Printf("vip成长值领取失败: %+v", reward)
		}
	}

	// 刷新token过期时间
	refresh, err := request.TokenRefresh(ctx, &weapi.TokenRefreshReq{})
	if err != nil || refresh.Code != 200 {
		log.Warn("TokenRefresh resp:%+v err: %s", refresh, err)
	}
	return nil
}

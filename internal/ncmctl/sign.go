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

type SignInOpts struct{}

type SignIn struct {
	root *Root
	cmd  *cobra.Command
	opts SignInOpts
	l    *log.Logger
}

func NewSignIn(root *Root, l *log.Logger) *SignIn {
	c := &SignIn{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "sign",
			Short:   "[need login] Sign perform daily cloud shell check-in and vip check-in",
			Example: `  ncmctl sign`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context())
	}
	return c
}

func (c *SignIn) addFlags() {}

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
		c.cmd.Println("yunbei signed in success")
	} else {
		c.cmd.Println("yunbei repeat signed in")
	}

	// // 完成当前时刻可以领取的任务奖励
	// task, err := request.YunBeiTaskListV3(ctx, &weapi.YunBeiTaskListV3Req{})
	// if err != nil {
	// 	return fmt.Errorf("YunBeiTaskTodo: %w", err)
	// }
	// for _, v := range task.Data.Normal.List {
	// 	if !v.Completed {
	// 		continue
	// 	}
	//
	// 	// todo: 获取depositCode
	//
	// 	reply, err := request.YunBeiTaskFinish(ctx, &weapi.YunBeiTaskFinishReq{
	// 		Period:      fmt.Sprintf("%d", v.Period),
	// 		UserTaskId:  fmt.Sprintf("%d", v.UserTaskId),
	// 		DepositCode: fmt.Sprintf("%d", v.DepositCode),
	// 	})
	// 	if err != nil {
	// 		log.Error("YunBeiTaskFinish(%v): %w", v.UserTaskId, err)
	// 	}
	// 	if reply.Code != "200" {
	// 		log.Error("YunBeiTaskFinish(%v) detail:%+v", v.UserTaskId, reply)
	// 	}
	// 	c.cmd.Printf("[%s] finish\n", v.TaskName)
	// }

	// 刷新token过期时间
	refresh, err := request.TokenRefresh(ctx, &weapi.TokenRefreshReq{})
	if err != nil || refresh.Code != 200 {
		log.Warn("TokenRefresh resp:%+v err: %s", refresh, err)
	}
	return nil
}

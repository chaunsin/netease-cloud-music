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

package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/nohup"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

type PartnerOpts struct {
	Crontab string
	Star    []int64
	// Tags    []string
}

type Partner struct {
	root *Root
	cmd  *cobra.Command
	opts PartnerOpts
	l    *log.Logger
}

func NewPartner(root *Root, l *log.Logger) *Partner {
	c := &Partner{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "partner",
			Short:   "Partner async execute music partner",
			Example: `ncm partner`,
		},
	}
	c.addFlags()
	c.cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := c.execute(cmd.Context()); err != nil {
			cmd.Println(err)
		}
	}
	return c
}

func (c *Partner) addFlags() {
	c.cmd.PersistentFlags().StringVar(&c.opts.Crontab, "crontab", "* 18 * * *", "https://crontab.guru/")
	c.cmd.PersistentFlags().Int64SliceVarP(&c.opts.Star, "star", "s", []int64{3, 4}, "star level")
	// c.cmd.PersistentFlags().StringSliceVarP(&c.opts.Tags, "tags", "t", []string{"情感到位", "有节奏感", "音色独特"}, "tags")
}

func (c *Partner) validate() error {
	if c.opts.Crontab == "" {
		return fmt.Errorf("crontab is required")
	}
	_, err := cron.ParseStandard(c.opts.Crontab)
	if err != nil {
		return fmt.Errorf("ParseStandard: %w", err)
	}
	if len(c.opts.Star) == 0 || len(c.opts.Star) > 5 {
		return fmt.Errorf("star level must be 1-5")
	}
	for _, v := range c.opts.Star {
		for _, vv := range c.opts.Star {
			if v == vv {
				return fmt.Errorf("star level must be unique")
			}
		}
	}
	return nil
}

func (c *Partner) Version(version string) {
	c.cmd.Version = version
}

func (c *Partner) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Partner) Command() *cobra.Command {
	return c.cmd
}

func (c *Partner) execute(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	// if err := c.job(c.cmd.Context()); err != nil {
	// 	fmt.Println("job:", err)
	// 	return err
	// }
	// fmt.Println("execute success ", time.Now())
	// return nil

	cr := cron.New(cron.WithLocation(time.Local))
	id, err := cr.AddFunc(c.opts.Crontab, func() {
		if err := c.job(c.cmd.Context()); err != nil {
			log.Error("err:", err)
			return
		}
		log.Info("execute success ", time.Now())
	})
	if err != nil {
		return fmt.Errorf("crontab error: %v", err)
	}
	cr.Start()

	log.Info("Next execute: ", cr.Entry(id).Schedule.Next(time.Now()))

	nohup.Run(nohup.CloseHook(func(ctx context.Context) error {
		cr.Stop()
		return nil
	}))
	return nil
}

func (c *Partner) job(ctx context.Context) error {
	cli, err := api.NewWithErr(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewWithErr: %w", err)
	}
	defer cli.Close(ctx)
	request := weapi.New(cli)

	// 判断是否需要登录
	if cli.NeedLogin(ctx) {
		return fmt.Errorf("need login")
	}

	// 判断是否有音乐合伙人资格
	// todo:

	// 获取任务列表
	task, err := request.PartnerTask(ctx, &weapi.PartnerTaskReq{ReqCommon: types.ReqCommon{}})
	if err != nil {
		return fmt.Errorf("PartnerTask: %w", err)
	}
	for _, work := range task.Data.Works {
		_ = work.Work.ResourceId

		// 随机一个分数,然后取一个tag从对应分数组中
		star := c.opts.Star[rand.Int31n(int32(len(c.opts.Star)))]
		group := weapi.PartnerTagsGroup[star]
		tags := group[rand.Int31n(int32(len(group)))]

		// 模拟听歌消耗得时间,随机15-25秒
		time.Sleep(time.Second * time.Duration(15+int(rand.Int31n(10))))

		// 判断任务是否执行过

		// 上报听歌事件

		// 执行测评
		var req = &weapi.PartnerEvaluateReq{
			ReqCommon:     types.ReqCommon{},
			TaskId:        task.Data.Id,
			WorkId:        work.Work.Id,
			Score:         star,
			Tags:          tags,
			CustomTags:    "[]",
			Comment:       "",
			SyncYunCircle: false,
			SyncComment:   true,               // ?
			Source:        "mp-music-partner", // 定死的值？
		}
		resp, err := request.PartnerEvaluate(ctx, req)
		if err != nil {
			return fmt.Errorf("PartnerEvaluate: %w", err)
		}
		switch resp.Code {
		case 200:
			// ok
		case 405:
			// 当前任务歌曲已完成评
		default:
			log.Error("PartnerEvaluate(%+v) err: %+v\n", req, resp)
			// return fmt.Errorf("PartnerEvaluate: %v", resp.Message)
		}
		break
	}

	return nil
}

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
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"time"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/spf13/cobra"
)

type PartnerOpts struct {
	Star    []int64
	ExtStar []int64
	ExtNum  string
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
			Short:   "[need login] executive music partner daily reviews\nrule details: https://y.music.163.com/g/yida/9fecf6a378be49a7a109ae9befb1b8d3",
			Example: "  ncmctl partner (default)\n  ncmctl partner -s 3,4 (set the base song evaluation score level random range 3-4)\n  ncmctl partner -s 3,4 -e 2,3,4 (set the random range of song evaluation rating 3-4, and the random range of additional songs 2-4)\n  ncmctl partner -n 5 (set the number of additional evaluation songs)",
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
	c.cmd.PersistentFlags().Int64SliceVarP(&c.opts.Star, "star", "s", []int64{3, 4}, "set the base song evaluation score level random range 1-5")
	c.cmd.PersistentFlags().Int64SliceVarP(&c.opts.ExtStar, "extra", "e", []int64{2, 3, 4}, "set the extra song evaluation score level random range 1-5")
	c.cmd.PersistentFlags().StringVarP(&c.opts.ExtNum, "num", "n", "random", "extra evaluation number of songs,'random' means 2 to 7")
}

func (c *Partner) validate() error {
	if len(c.opts.Star) == 0 || len(c.opts.Star) > 5 {
		return fmt.Errorf("star level must be range 1-5")
	}
	if slices.ContainsFunc(c.opts.Star, func(i int64) bool {
		if i < 1 || i > 5 {
			return true
		}
		return false
	}) {
		return fmt.Errorf("star level must be range 1-5")
	}
	if !utils.IsUnique(c.opts.Star) {
		return fmt.Errorf("star level must be unique")
	}

	if len(c.opts.ExtStar) == 0 || len(c.opts.ExtStar) > 5 {
		return fmt.Errorf("extra star level must be range 1-5")
	}
	if slices.ContainsFunc(c.opts.ExtStar, func(i int64) bool {
		if i < 1 || i > 5 {
			return true
		}
		return false
	}) {
		return fmt.Errorf("extra star level must be range 1-5")
	}
	if !utils.IsUnique(c.opts.ExtStar) {
		return fmt.Errorf("extra star level must be unique")
	}

	if c.opts.ExtNum == "" {
		return fmt.Errorf("num is empty")
	}
	if c.opts.ExtNum != "random" {
		num, err := strconv.ParseInt(c.opts.ExtNum, 10, 64)
		if err != nil {
			return fmt.Errorf("num must be int or 'random'")
		}
		if num < 0 || num > 15 {
			return fmt.Errorf("num must be >= 0 and <= 15")
		}
	}
	return nil
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

	if err := c.do(ctx); err != nil {
		return err
	}
	c.cmd.Printf("%s execute success\n", time.Now())
	return nil
}

func (c *Partner) do(ctx context.Context) error {
	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)

	// 判断是否需要登录
	request := weapi.New(cli)
	if request.NeedLogin(ctx) {
		return fmt.Errorf("need login")
	}

	// 判断是否有音乐合伙人资格
	info, err := request.PartnerUserinfo(ctx, &weapi.PartnerUserinfoReq{ReqCommon: types.ReqCommon{}})
	if err != nil {
		return fmt.Errorf("PartnerUserinfo: %w", err)
	}
	if info.Code == 703 {
		return fmt.Errorf("您不是音乐合伙人不能进行测评 detail: %+v\n", info)
	}
	if info.Code != 200 {
		return fmt.Errorf("PartnerUserinfo err: %+v\n", info)
	}
	switch status := info.Data.Status; status {
	case "NORMAL":
	case "ELIMINATED":
		return fmt.Errorf("您没有测评资格或失去测评资格")
	default:
		return fmt.Errorf("账号状态异常,未知状态[%s]\n", status)
	}

	var (
		baseNum   int64
		extNum    int64
		randomNum int32
	)
	defer func() {
		c.cmd.Printf("report: 基础歌曲完成数量(%v) 扩展歌曲完成数量(%v/%v)\n", baseNum, extNum, randomNum)
	}()

	// 获取每日基本任务5首歌曲列表并执行测评
	task, err := request.PartnerDailyTask(ctx, &weapi.PartnerTaskReq{ReqCommon: types.ReqCommon{}})
	if err != nil {
		return fmt.Errorf("PartnerDailyTask: %w", err)
	}
	for _, work := range task.Data.Works {
		// 判断任务是否执行过
		if work.Completed {
			baseNum++
			log.Warn("task completed: %+v\n", work)
			continue
		}

		// 模拟听歌消耗得时间,随机15-25秒
		time.Sleep(time.Second * time.Duration(15+int(rand.Int31n(10))))

		// 随机一个分数,然后从对应分数组中取一个tag
		star := c.opts.Star[rand.Int31n(int32(len(c.opts.Star)))]
		group := weapi.PartnerTagsGroup[star]
		tags := group[rand.Int31n(int32(len(group)))]

		// 上报
		var reportReq = &weapi.PartnerExtraReportReq{
			ReqCommon:     types.ReqCommon{},
			WorkId:        fmt.Sprintf("%v", work.Work.Id),
			ResourceId:    fmt.Sprintf("%v", work.Work.ResourceId),
			BizResourceId: "",
			InteractType:  "PLAY_END",
		}
		resp, err := request.PartnerExtraReport(ctx, reportReq)
		if err != nil {
			return fmt.Errorf("PartnerExtraReport: %w", err)
		}
		switch resp.Code {
		case 200:
			// ok
		default:
			log.Error("PartnerExtraReport(%+v) err: %+v\n", reportReq, resp)
			continue
		}

		// 如果发表评论按照正常逻辑需要过内容安审

		// 上报听歌事件

		// 执行测评
		var extScore = make(map[string]int64, 3)
		for _, t := range work.SupportExtraEvaTypes {
			extScore[fmt.Sprintf("%v", t)] = c.opts.ExtStar[rand.Int31n(int32(len(c.opts.Star)))]
		}
		extraScore, err := json.Marshal(extScore)
		if err != nil {
			return fmt.Errorf("json.Marshal(%+v) err: %+v\n", extScore, err)
		}
		var req = &weapi.PartnerEvaluateReq{
			ReqCommon:     types.ReqCommon{},
			TaskId:        fmt.Sprintf("%v", task.Data.Id),
			WorkId:        fmt.Sprintf("%v", work.Work.Id),
			Score:         fmt.Sprintf("%v", star),
			Tags:          tags,
			CustomTags:    "[]",
			Comment:       "",
			SyncYunCircle: false,
			SyncComment:   true,               // ?
			Source:        "mp-music-partner", // 目前定死的值
			ExtraScore:    string(extraScore), //
			ExtraResource: false,
		}
		evalResp, err := request.PartnerEvaluate(ctx, req)
		if err != nil {
			return fmt.Errorf("PartnerEvaluate: %w", err)
		}
		switch evalResp.Code {
		case 200:
			baseNum++
			// ok
		case 405:
			baseNum++
			// 当前任务歌曲已完成评
		default:
			log.Error("PartnerEvaluate(%+v) err: %+v\n", req, resp)
			// return fmt.Errorf("PartnerEvaluate: %v", resp.Message)
		}
	}

	// 获取扩展任务列表并执行扩展任务测评 2024年10月21日推出的新功能测评
	var taskId = task.Data.Id
	if c.opts.ExtNum == "random" {
		randomNum = 2 + rand.Int31n(6) // 2~7
	} else {
		num, _ := strconv.ParseInt(c.opts.ExtNum, 10, 64)
		randomNum = int32(num)
	}
	if randomNum > 0 {
		extraTask, err := request.PartnerExtraTask(ctx, &weapi.PartnerExtraTaskReq{ReqCommon: types.ReqCommon{}})
		if err != nil {
			return fmt.Errorf("PartnerExtraTask: %w", err)
		}
		for _, work := range extraTask.Data {
			// 判断任务是否执行过
			if work.Completed {
				extNum++
				log.Warn("extra task completed: %+v\n", work)
				continue
			}

			// 模拟听歌消耗得时间,随机15-25秒
			time.Sleep(time.Second * time.Duration(15+int(rand.Int31n(10))))

			// 随机一个分数,然后从对应分数组中取一个tag
			star := c.opts.Star[rand.Int31n(int32(len(c.opts.Star)))]
			group := weapi.PartnerTagsGroup[star]
			tags := group[rand.Int31n(int32(len(group)))]

			// 上报听歌事件

			// 如果发表评论按照正常逻辑需要过内容安审

			// 上报
			var req = &weapi.PartnerExtraReportReq{
				ReqCommon:     types.ReqCommon{},
				WorkId:        fmt.Sprintf("%v", work.Work.Id),
				ResourceId:    fmt.Sprintf("%v", work.Work.ResourceId),
				BizResourceId: "",
				InteractType:  "PLAY_END",
			}
			resp, err := request.PartnerExtraReport(ctx, req)
			if err != nil {
				return fmt.Errorf("PartnerExtraReport: %w", err)
			}
			switch resp.Code {
			case 200:
				// ok
			default:
				log.Error("PartnerExtraReport(%+v) err: %+v\n", req, resp)
				continue
			}

			// 执行测评
			var extScore = make(map[string]int64, 3)
			for _, t := range work.SupportExtraEvaTypes {
				extScore[fmt.Sprintf("%v", t)] = c.opts.ExtStar[rand.Int31n(int32(len(c.opts.Star)))]
			}
			extraScore, err := json.Marshal(extScore)
			if err != nil {
				return fmt.Errorf("json.Marshal(%+v) err: %+v\n", extScore, err)
			}
			var evaluateReq = &weapi.PartnerEvaluateReq{
				ReqCommon:     types.ReqCommon{},
				TaskId:        fmt.Sprintf("%v", taskId),
				WorkId:        fmt.Sprintf("%v", work.Work.Id),
				Score:         fmt.Sprintf("%v", star),
				Tags:          tags,
				CustomTags:    "[]",
				Comment:       "",
				SyncYunCircle: false,
				SyncComment:   true,               // ?
				Source:        "mp-music-partner", // 暂时定死的值
				ExtraScore:    string(extraScore), // todo
				ExtraResource: true,
			}
			evaluateResp, err := request.PartnerEvaluate(ctx, evaluateReq)
			if err != nil {
				return fmt.Errorf("PartnerEvaluate: %w", err)
			}
			switch evaluateResp.Code {
			case 200:
				extNum++
				randomNum--
				if randomNum <= 0 {
					goto end
				}
			case 405:
				extNum++
				// 当前任务歌曲已完成评
			default:
				log.Error("PartnerEvaluate(%+v) err: %+v\n", req, resp)
				// return fmt.Errorf("PartnerEvaluate: %v", resp.Message)
			}
		}
	}
end:

	// 刷新token过期时间
	refresh, err := request.TokenRefresh(ctx, &weapi.TokenRefreshReq{})
	if err != nil || refresh.Code != 200 {
		log.Warn("TokenRefresh resp:%+v err: %s", refresh, err)
	}
	return nil
}

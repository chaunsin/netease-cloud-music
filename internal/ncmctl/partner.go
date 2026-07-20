// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
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
			Use:   "partner",
			Short: "Submit music-partner evaluations",
			Long: "Report play events and submit music-partner evaluations once. Login and partner " +
				"eligibility are required. The command changes account state and waits 15-24 seconds " +
				"between evaluated items. Rules: " +
				"https://y.music.163.com/g/yida/9fecf6a378be49a7a109ae9befb1b8d3",
			Example: "  ncmctl partner\n" +
				"  ncmctl partner --star 3,4 --extra 2,3,4\n" +
				"  ncmctl partner --num 5",
			Args: cobra.NoArgs,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context())
	}
	return c
}

func (c *Partner) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Partner) Command() *cobra.Command {
	return c.cmd
}

func (c *Partner) addFlags() {
	c.cmd.PersistentFlags().Int64SliceVarP(&c.opts.Star, "star", "s", []int64{3, 4}, "base evaluation score choices (unique values from 1 to 5)")
	c.cmd.PersistentFlags().Int64SliceVarP(&c.opts.ExtStar, "extra", "e", []int64{2, 3, 4}, "extra evaluation score choices (unique values from 1 to 5)")
	c.cmd.PersistentFlags().StringVarP(&c.opts.ExtNum, "num", "n", "random", "extra evaluation count: 'random' (2-7) or an integer from 0 to 15")
}

func (c *Partner) validate() error {
	if len(c.opts.Star) == 0 || len(c.opts.Star) > 5 {
		return errors.New("star level must be range 1-5")
	}

	if slices.ContainsFunc(c.opts.Star, func(i int64) bool {
		if i < 1 || i > 5 {
			return true
		}
		return false
	}) {
		return errors.New("star level must be range 1-5")
	}

	if !utils.IsUnique(c.opts.Star) {
		return errors.New("star level must be unique")
	}

	if len(c.opts.ExtStar) == 0 || len(c.opts.ExtStar) > 5 {
		return errors.New("extra star level must be range 1-5")
	}

	if slices.ContainsFunc(c.opts.ExtStar, func(i int64) bool {
		if i < 1 || i > 5 {
			return true
		}
		return false
	}) {
		return errors.New("extra star level must be range 1-5")
	}

	if !utils.IsUnique(c.opts.ExtStar) {
		return errors.New("extra star level must be unique")
	}

	if c.opts.ExtNum == "" {
		return errors.New("num is empty")
	}

	if c.opts.ExtNum != "random" {
		num, err := strconv.Atoi(c.opts.ExtNum)
		if err != nil {
			return errors.New("num must be int or 'random'")
		}

		if num < 0 || num > 15 {
			return errors.New("num must be >= 0 and <= 15")
		}
	}
	return nil
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
	defer closeAPIClient(ctx, cli)

	// 判断是否需要登录
	request := weapi.New(cli)
	if request.NeedLogin(ctx) {
		return errors.New("need login")
	}

	// 判断是否有音乐合伙人资格
	info, err := request.PartnerUserinfo(ctx, &weapi.PartnerUserinfoReq{ReqCommon: types.ReqCommon{}})
	if err != nil {
		return fmt.Errorf("PartnerUserinfo: %w", err)
	}

	if info.Code == 703 {
		return fmt.Errorf("您不是音乐合伙人不能进行测评 detail: %+v", info)
	}

	if info.Code != 200 {
		return fmt.Errorf("PartnerUserinfo err: %+v", info)
	}

	switch status := info.Data.Status; status {
	case "NORMAL":
	case "ELIMINATED":
		return errors.New("您没有测评资格或失去测评资格! ")
	default:
		return fmt.Errorf("账号状态异常,未知状态[%s]", status)
	}

	var (
		baseNum   int
		extNum    int // 扩展歌曲实际成功执行次数
		randomNum int // 扩展歌曲总共要执行的次数
	)
	defer func() {
		c.cmd.Printf("report: 基础歌曲完成数量(%v) 扩展歌曲完成数量(%v/%v)\n", baseNum, extNum, randomNum)
	}()

	// 获取每日基本任务5首歌曲列表并执行测评
	task, err := request.PartnerDailyTask(ctx, &weapi.PartnerTaskReq{ReqCommon: types.ReqCommon{}})
	if err != nil {
		return fmt.Errorf("PartnerDailyTask: %w", err)
	}

	for i := range task.Data.Works {
		work := &task.Data.Works[i]
		// 判断任务是否执行过
		if work.Completed {
			baseNum++

			log.Warnf("task completed: %+v\n", work)
			continue
		}

		// 模拟听歌消耗得时间,随机15-24秒
		time.Sleep(time.Second * time.Duration(15+rand.IntN(10)))

		// 随机一个分数,然后从对应分数组中取一个tag
		star := c.opts.Star[rand.IntN(len(c.opts.Star))]
		group := weapi.PartnerTagsGroup[star]
		tags := group[rand.IntN(len(group))]

		// 上报
		reportReq := &weapi.PartnerExtraReportReq{
			ReqCommon:     types.ReqCommon{},
			WorkId:        strconv.FormatInt(work.Work.Id, 10),
			ResourceId:    strconv.FormatInt(work.Work.ResourceId, 10),
			BizResourceId: "",
			InteractType:  "PLAY_END",
		}

		resp, reportErr := request.PartnerExtraReport(ctx, reportReq)
		if reportErr != nil {
			return fmt.Errorf("PartnerExtraReport: %w", reportErr)
		}

		switch resp.Code {
		case 200:
			// ok
		default:
			log.Errorf("PartnerExtraReport(%+v) err: %+v\n", reportReq, resp)
			continue
		}

		// 如果发表评论按照正常逻辑需要过内容安审

		// 上报听歌事件

		// 执行测评
		extScore := make(map[string]int64, 3)
		for _, t := range work.SupportExtraEvaTypes {
			extScore[strconv.FormatInt(t, 10)] = c.opts.ExtStar[rand.IntN(len(c.opts.ExtStar))]
		}

		extraScore, marshalErr := json.Marshal(extScore)
		if marshalErr != nil {
			return fmt.Errorf("json.Marshal(%+v) err: %w", extScore, marshalErr)
		}

		req := &weapi.PartnerEvaluateReq{
			ReqCommon:     types.ReqCommon{},
			TaskId:        strconv.FormatInt(task.Data.Id, 10),
			WorkId:        strconv.FormatInt(work.Work.Id, 10),
			Score:         strconv.FormatInt(star, 10),
			Tags:          tags,
			CustomTags:    "[]",
			Comment:       "",
			SyncYunCircle: false,
			SyncComment:   true,               // ?
			Source:        "mp-music-partner", // 目前定死的值
			ExtraScore:    string(extraScore), //
			ExtraResource: false,
		}

		evalResp, evaluateErr := request.PartnerEvaluate(ctx, req)
		if evaluateErr != nil {
			return fmt.Errorf("PartnerEvaluate: %w", evaluateErr)
		}

		switch evalResp.Code {
		case 200:
			baseNum++
			// ok
		case 405:
			baseNum++
			// 当前任务歌曲已完成评
		default:
			log.Errorf("PartnerEvaluate(%+v) err: %+v\n", req, resp)
			// return fmt.Errorf("PartnerEvaluate: %v", resp.Message)
		}
	}

	// 获取扩展任务列表并执行扩展任务测评 2024年10月21日推出的新功能测评
	var (
		taskId     = task.Data.Id
		executeNum int
	)
	if c.opts.ExtNum == "random" {
		executeNum = 2 + rand.IntN(6) // 2~7
	} else {
		num, parseErr := strconv.Atoi(c.opts.ExtNum)
		if parseErr != nil {
			return fmt.Errorf("strconv.Atoi(%q): %w", c.opts.ExtNum, parseErr)
		}

		executeNum = num
	}

	randomNum = executeNum
	if executeNum > 0 {
		extraTask, extraTaskErr := request.PartnerExtraTask(ctx, &weapi.PartnerExtraTaskReq{ReqCommon: types.ReqCommon{}})
		if extraTaskErr != nil {
			return fmt.Errorf("PartnerExtraTask: %w", extraTaskErr)
		}

		for i := range extraTask.Data {
			work := &extraTask.Data[i]
			// 判断任务是否执行过
			if work.Completed {
				extNum++

				log.Warnf("extra task completed: %+v\n", work)
				continue
			}

			// 模拟听歌消耗得时间,随机15-24秒
			time.Sleep(time.Second * time.Duration(15+rand.IntN(10)))

			// 随机一个分数,然后从对应分数组中取一个tag
			star := c.opts.Star[rand.IntN(len(c.opts.Star))]
			group := weapi.PartnerTagsGroup[star]
			tags := group[rand.IntN(len(group))]

			// 上报听歌事件

			// 如果发表评论按照正常逻辑需要过内容安审

			// 上报
			req := &weapi.PartnerExtraReportReq{
				ReqCommon:     types.ReqCommon{},
				WorkId:        strconv.FormatInt(work.Work.Id, 10),
				ResourceId:    strconv.FormatInt(work.Work.ResourceId, 10),
				BizResourceId: "",
				InteractType:  "PLAY_END",
			}

			resp, reportErr := request.PartnerExtraReport(ctx, req)
			if reportErr != nil {
				return fmt.Errorf("PartnerExtraReport: %w", reportErr)
			}

			switch resp.Code {
			case 200:
				// ok
			default:
				log.Errorf("PartnerExtraReport(%+v) err: %+v\n", req, resp)
				continue
			}

			// 执行测评
			extScore := make(map[string]int64, 3)
			for _, t := range work.SupportExtraEvaTypes {
				extScore[strconv.FormatInt(t, 10)] = c.opts.ExtStar[rand.IntN(len(c.opts.ExtStar))]
			}

			extraScore, marshalErr := json.Marshal(extScore)
			if marshalErr != nil {
				return fmt.Errorf("json.Marshal(%+v) err: %w", extScore, marshalErr)
			}

			evaluateReq := &weapi.PartnerEvaluateReq{
				ReqCommon:     types.ReqCommon{},
				TaskId:        strconv.FormatInt(taskId, 10),
				WorkId:        strconv.FormatInt(work.Work.Id, 10),
				Score:         strconv.FormatInt(star, 10),
				Tags:          tags,
				CustomTags:    "[]",
				Comment:       "",
				SyncYunCircle: false,
				SyncComment:   true,               // ?
				Source:        "mp-music-partner", // 暂时定死的值
				ExtraScore:    string(extraScore), // todo
				ExtraResource: true,
			}

			evaluateResp, evaluateErr := request.PartnerEvaluate(ctx, evaluateReq)
			if evaluateErr != nil {
				return fmt.Errorf("PartnerEvaluate: %w", evaluateErr)
			}

			switch evaluateResp.Code {
			case 200:
				extNum++

				executeNum--
				if executeNum <= 0 {
					goto end
				}
			case 405:
				extNum++
				// 当前任务歌曲已完成评
			default:
				log.Errorf("PartnerEvaluate(%+v) err: %+v\n", req, resp)
				// return fmt.Errorf("PartnerEvaluate: %v", resp.Message)
			}
		}
	}

end:

	// 刷新token过期时间
	refresh, err := request.TokenRefresh(ctx, &weapi.TokenRefreshReq{})

	if err != nil || refresh.Code != 200 {
		log.Warnf("TokenRefresh resp:%+v err: %s", refresh, err)
	}
	return nil
}

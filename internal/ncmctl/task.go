// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/nohup"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

const (
	signYunBeiGateRetryCount    = 3
	signYunBeiGateRetryInterval = 30 * time.Second
	taskShutdownTimeout         = 65 * time.Second
)

type scheduledCommand interface {
	Command() *cobra.Command
	validate() error
}

type TaskOpts struct {
	PartnerOpts
	ScrobbleOpts
	SignInOpts
	ShareOpts
	FansGroupOpts

	Location string
	RunAll   bool

	Partner            bool
	PartnerOptsCrontab string

	Scrobble            bool
	ScrobbleOptsCrontab string

	SignIn            bool
	SignInOptsCrontab string

	YunBeiOpts YunBeiOpts
	YunBei     bool
	YunBeiCron string

	SongShare        bool
	ShareOptsCrontab string

	FansGroup            bool
	FansGroupOptsCrontab string
}

type Task struct {
	root *Root
	cmd  *cobra.Command
	opts TaskOpts
	l    *log.Logger

	signYunBeiGate sync.Mutex
	gateSleep      func(context.Context, time.Duration, ...time.Duration) error
	withTimeout    func(context.Context, time.Duration) (context.Context, context.CancelFunc)
}

func NewTask(root *Root, l *log.Logger) *Task {
	c := &Task{
		root:        root,
		l:           l,
		gateSleep:   utils.Sleep,
		withTimeout: context.WithTimeout,
		cmd: &cobra.Command{
			Use:   "task",
			Short: "Schedule account tasks as a long-running service",
			Long: "Schedule sign, YunBei, partner, scrobble, daily song challenge and fans group jobs using five-field cron expressions. " +
				"Login is required. Without --runAll or any task selector the command fails fast. " +
				"Explicit selectors register only those jobs; --runAll schedules all six jobs. The service runs until interrupted. " +
				"YunBei and fans group jobs change account state through public notes and interactions, consume network traffic, " +
				"and YunBei playback can take more than 45 minutes; published notes are kept by default.",
			Example: "  # Schedule all tasks\n" +
				"  ncmctl task --runAll\n\n" +
				"  # Schedule only sign and scrobble\n" +
				"  ncmctl task --sign --scrobble\n\n" +
				"  # Schedule selected YunBei tasks at noon\n" +
				"  ncmctl task --yunbei --yunbei.task sign,claim\n\n" +
				"  # Run scrobble daily at 20:00 in the selected time zone\n" +
				"  ncmctl task --scrobble --scrobble.cron '0 20 * * *' --location Asia/Shanghai\n\n" +
				"  # Run fans group missions daily at 10:30\n" +
				"  ncmctl task --fansgroup --fansgroup.cron '30 10 * * *'",
			Args: cobra.NoArgs,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}
	return c
}

func (c *Task) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Task) Command() *cobra.Command {
	return c.cmd
}

func (c *Task) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Location, "location", "l", "Asia/Shanghai", "IANA time zone used for cron schedules")
	c.cmd.PersistentFlags().
		BoolVar(&c.opts.RunAll, "runAll", false, "schedule all six tasks, including the daily song challenge plus YunBei public notes, interactions, network traffic and playback longer than 45 minutes")

	c.cmd.PersistentFlags().BoolVar(&c.opts.Partner, "partner", false, "schedule the music-partner evaluation task")
	c.cmd.PersistentFlags().StringVar(&c.opts.PartnerOptsCrontab, "partner.cron", "0 18 * * *", "five-field cron schedule for the partner task")
	c.cmd.PersistentFlags().Int64SliceVar(&c.opts.Star, "partner.star", []int64{3, 4}, "base evaluation score choices (unique values from 1 to 5)")
	c.cmd.PersistentFlags().Int64SliceVar(&c.opts.ExtStar, "partner.extStar", []int64{2, 3, 4}, "extra evaluation score choices (unique values from 1 to 5)")
	c.cmd.PersistentFlags().StringVar(&c.opts.ExtNum, "partner.extNum", "random", "extra evaluation count: 'random' (2-7) or an integer from 0 to 15")

	c.cmd.PersistentFlags().BoolVar(&c.opts.Scrobble, "scrobble", false, "schedule the play-log scrobble task")
	c.cmd.PersistentFlags().StringVar(&c.opts.ScrobbleOptsCrontab, "scrobble.cron", "0 18 * * *", "five-field cron schedule for the scrobble task")
	c.cmd.PersistentFlags().Int64Var(&c.opts.Num, "scrobble.num", 300, "requested play-log count per run (1-300)")

	c.cmd.PersistentFlags().BoolVar(&c.opts.SignIn, "sign", false, "schedule the YunBei and VIP sign-in task")
	c.cmd.PersistentFlags().StringVar(&c.opts.SignInOptsCrontab, "sign.cron", "0 10 * * *", "five-field cron schedule for the sign task")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Automatic, "sign.automatic", false, "claim eligible rewards during sign-in (increased account risk)")

	c.cmd.PersistentFlags().BoolVar(&c.opts.YunBei, "yunbei", false, "schedule the YunBei task job")
	c.cmd.PersistentFlags().StringVar(&c.opts.YunBeiCron, "yunbei.cron", "0 12 * * *", "five-field cron schedule for the YunBei task")
	c.cmd.PersistentFlags().StringSliceVar(&c.opts.YunBeiOpts.Tasks, "yunbei.task", nil, "YunBei tasks to run; comma-separated or repeated")
	c.cmd.PersistentFlags().BoolVar(&c.opts.YunBeiOpts.All, "yunbei.all", false, "run every YunBei task, including account-changing and long-running actions")
	c.cmd.PersistentFlags().StringVar(&c.opts.YunBeiOpts.Title, "yunbei.title", "", "title override for the public YunBei note")
	c.cmd.PersistentFlags().StringVar(&c.opts.YunBeiOpts.Message, "yunbei.message", "", "message override for the public YunBei note; at least 20 Unicode characters")
	c.cmd.PersistentFlags().StringVar(&c.opts.YunBeiOpts.Image, "yunbei.image", "", "local JPEG, PNG, GIF or WebP image for the public YunBei note; up to 20 MiB")
	c.cmd.PersistentFlags().BoolVar(&c.opts.YunBeiOpts.Delete, "yunbei.delete", false, "delete only the public YunBei note created and confirmed by that run")

	c.cmd.PersistentFlags().BoolVar(&c.opts.SongShare, "share", false, "schedule the public daily song challenge task")
	c.cmd.PersistentFlags().StringVar(&c.opts.ShareOptsCrontab, "share.cron", "0 9 * * *", "five-field cron schedule for daily song challenge")
	// ShareOpts 与 FansGroupOpts 存在同名字段, 此处显式限定
	c.cmd.PersistentFlags().Int64Var(&c.opts.SongID, "share.song-id", 0, "fixed song ID for daily song challenge")
	c.cmd.PersistentFlags().StringVar(&c.opts.ShareOpts.Image, "share.image", "", "fixed image file for daily song challenge")
	c.cmd.PersistentFlags().StringVar(&c.opts.ShareOpts.Title, "share.title", "", "title override for daily song challenge")
	c.cmd.PersistentFlags().StringVar(&c.opts.ShareOpts.Message, "share.message", "", "message override for daily song challenge")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Draw, "share.draw", true, "draw rewards after daily song challenge")
	c.cmd.PersistentFlags().BoolVar(&c.opts.ShareOpts.Delete, "share.delete", false, "delete each new note after lottery")

	c.cmd.PersistentFlags().BoolVar(&c.opts.FansGroup, "fansgroup", false, "schedule the fans group missions task (plays, hearts, likes and public notes; notes are kept by default)")
	c.cmd.PersistentFlags().StringVar(&c.opts.FansGroupOptsCrontab, "fansgroup.cron", "30 10 * * *", "five-field cron schedule for the fans group task")
	c.cmd.PersistentFlags().StringSliceVar(&c.opts.GroupID, "fansgroup.group-id", []string{defaultFansGroupID}, "fans group IDs (digits only); comma-separated or repeated")
	c.cmd.PersistentFlags().StringVar(&c.opts.FansGroupOpts.Title, "fansgroup.title", "", "title override for fans group notes")
	c.cmd.PersistentFlags().StringVar(&c.opts.FansGroupOpts.Message, "fansgroup.message", "", "message override for fans group notes; at least 10 Unicode characters")
	c.cmd.PersistentFlags().StringVar(&c.opts.FansGroupOpts.Image, "fansgroup.image", "", "fixed image file for fans group notes")
	c.cmd.PersistentFlags().BoolVar(&c.opts.FansGroupOpts.Delete, "fansgroup.delete", false, "delete notes published by the fans group task after the mission loop")
}

func (c *Task) validate() error {
	var (
		partner = func() error {
			if c.opts.PartnerOptsCrontab == "" {
				return errors.New("partner.crontab is required")
			}

			if _, err := cron.ParseStandard(c.opts.PartnerOptsCrontab); err != nil {
				return fmt.Errorf("ParseStandard: %w", err)
			}
			return nil
		}
		signIn = func() error {
			if c.opts.SignInOptsCrontab == "" {
				return errors.New("sign.crontab is required")
			}

			if _, err := cron.ParseStandard(c.opts.SignInOptsCrontab); err != nil {
				return fmt.Errorf("ParseStandard: %w", err)
			}
			return nil
		}
		yunBei = func() error {
			if c.opts.YunBeiCron == "" {
				return errors.New("yunbei.cron is required")
			}

			if _, err := cron.ParseStandard(c.opts.YunBeiCron); err != nil {
				return fmt.Errorf("ParseStandard: %w", err)
			}
			return nil
		}
		scrobble = func() error {
			if c.opts.ScrobbleOptsCrontab == "" {
				return errors.New("scrobble.crontab is required")
			}

			if _, err := cron.ParseStandard(c.opts.ScrobbleOptsCrontab); err != nil {
				return fmt.Errorf("ParseStandard: %w", err)
			}
			return nil
		}
		dailySongShare = func() error {
			if c.opts.ShareOptsCrontab == "" {
				return errors.New("share.crontab is required")
			}

			if _, err := cron.ParseStandard(c.opts.ShareOptsCrontab); err != nil {
				return fmt.Errorf("ParseStandard: %w", err)
			}

			if c.opts.ShareOpts.Delete && !c.opts.Draw {
				return errors.New("share.delete requires draw=true")
			}
			return nil
		}
		fansGroup = func() error {
			if c.opts.FansGroupOptsCrontab == "" {
				return errors.New("fansgroup.crontab is required")
			}

			if _, err := cron.ParseStandard(c.opts.FansGroupOptsCrontab); err != nil {
				return fmt.Errorf("ParseStandard: %w", err)
			}

			for _, id := range c.opts.GroupID {
				if !isNumericString(id) {
					return fmt.Errorf("fansgroup.group-id must be a non-empty numeric string, got %q", id)
				}
			}
			return nil
		}
	)

	if err := c.validateYunBeiSelection(); err != nil {
		return err
	}

	sel, err := c.taskSelection()
	if err != nil {
		return err
	}

	if sel.SignIn {
		if err := signIn(); err != nil {
			return err
		}
	}

	if sel.YunBei {
		if err := yunBei(); err != nil {
			return err
		}
	}

	if sel.Partner {
		if err := partner(); err != nil {
			return err
		}
	}

	if sel.Scrobble {
		if err := scrobble(); err != nil {
			return err
		}
	}

	if sel.SongShare {
		if err := dailySongShare(); err != nil {
			return err
		}
	}

	if sel.FansGroup {
		if err := fansGroup(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Task) validateYunBeiSelection() error {
	yunBeiFlagChanged := slices.ContainsFunc([]string{
		"yunbei.cron", "yunbei.task", "yunbei.all", "yunbei.title", "yunbei.message", "yunbei.image", "yunbei.delete",
	}, func(name string) bool {
		return c.cmd.PersistentFlags().Changed(name)
	})
	if yunBeiFlagChanged && !c.opts.YunBei && !c.opts.RunAll {
		return errors.New("--yunbei.* options require --yunbei or --runAll")
	}

	if c.opts.RunAll {
		if c.cmd.PersistentFlags().Changed("yunbei") && !c.opts.YunBei {
			return errors.New("--yunbei=false conflicts with --runAll")
		}

		if c.cmd.PersistentFlags().Changed("yunbei.all") && !c.opts.YunBeiOpts.All {
			return errors.New("--yunbei.all=false conflicts with --runAll")
		}

		if len(c.opts.YunBeiOpts.Tasks) != 0 {
			return errors.New("--yunbei.task conflicts with --runAll")
		}
	}

	if !c.opts.YunBei && !c.opts.RunAll {
		return nil
	}

	opts := c.effectiveYunBeiOpts()
	if _, err := validateYunBeiOpts(&opts, false); err != nil {
		return fmt.Errorf("yunbei options: %w", err)
	}
	return nil
}

func (c *Task) effectiveYunBeiOpts() YunBeiOpts {
	opts := c.opts.YunBeiOpts

	opts.Tasks = slices.Clone(opts.Tasks)
	if c.opts.RunAll {
		opts.All = true
	}
	return opts
}

type taskSelection struct {
	SignIn    bool
	YunBei    bool
	Partner   bool
	Scrobble  bool
	SongShare bool
	FansGroup bool
}

func (c *Task) taskSelection() (taskSelection, error) {
	o := c.opts
	if o.RunAll {
		return taskSelection{SignIn: true, YunBei: true, Partner: true, Scrobble: true, SongShare: true, FansGroup: true}, nil
	}

	if !o.SignIn && !o.YunBei && !o.Partner && !o.Scrobble && !o.SongShare && !o.FansGroup {
		return taskSelection{}, errors.New("no task selected: specify at least one of --sign/--yunbei/--partner/--scrobble/--share/--fansgroup or --runAll")
	}
	return taskSelection{SignIn: o.SignIn, YunBei: o.YunBei, Partner: o.Partner, Scrobble: o.Scrobble, SongShare: o.SongShare, FansGroup: o.FansGroup}, nil
}

func (c *Task) registerScheduledCommand(ctx context.Context, job *cron.Cron, name, schedule, cronError string, command scheduledCommand) error {
	return c.registerScheduledCommandWithGate(ctx, job, name, schedule, cronError, command, nil)
}

type scheduledCommandGuard struct {
	gate    *sync.Mutex
	running *atomic.Bool
}

func (c *Task) registerScheduledCommandWithGate(ctx context.Context, job *cron.Cron, name, schedule, cronError string, command scheduledCommand, guard *scheduledCommandGuard) error {
	label := "[" + name + "]"

	command.Command().DisableFlagParsing = true
	command.Command().SetArgs([]string{})

	if err := command.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	id, err := job.AddFunc(schedule, func() {
		c.runScheduledCommand(ctx, label, command, guard)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", cronError, err)
	}

	desc := describeSchedule(job.Entry(id).Schedule, job.Location())

	c.l.Infof("%s task registered: schedule=%q tz=%s next=%s", label, schedule, job.Location(), desc)
	c.cmd.Printf("  %s 下次执行: %s\n", label, desc)
	return nil
}

func (c *Task) runScheduledCommand(ctx context.Context, label string, command scheduledCommand, guard *scheduledCommandGuard) {
	if guard != nil && guard.running != nil {
		if !guard.running.CompareAndSwap(false, true) {
			c.l.Warnf("%s task already running or waiting; skip this trigger", label)
			c.cmd.Printf("  %s 同名任务仍在运行或等待，本次跳过，不会补跑\n", label)
			return
		}
		defer guard.running.Store(false)
	}

	if guard != nil && guard.gate != nil {
		if err := c.lockSignYunBeiGate(ctx, guard.gate); err != nil {
			if errors.Is(err, errSignYunBeiGateBusy) {
				c.l.Warnf("%s sign/yunbei gate remained busy after %d retries; skip this trigger", label, signYunBeiGateRetryCount)
				c.cmd.Printf("  %s sign/yunbei 仍在运行，本次已重试 %d 次并跳过，不会补跑\n", label, signYunBeiGateRetryCount)
			} else {
				c.l.Warnf("%s stopped waiting for sign/yunbei gate: %v", label, err)
			}
			return
		}
		defer guard.gate.Unlock()
	}

	start := time.Now()

	c.l.Infof("%s task start", label)
	c.cmd.Printf("  %s 开始执行\n", label)

	if err := command.Command().ExecuteContext(ctx); err != nil {
		c.l.Errorf(label+" execute err: %s", err)
		return
	}

	c.l.Infof("%s execute success (cost=%s)", label, time.Since(start).Round(time.Second))
}

var errSignYunBeiGateBusy = errors.New("sign/yunbei gate is busy")

func (c *Task) lockSignYunBeiGate(ctx context.Context, gate *sync.Mutex) error {
	if gate.TryLock() {
		return nil
	}
	for range signYunBeiGateRetryCount {
		if err := c.gateSleep(ctx, signYunBeiGateRetryInterval); err != nil {
			return err
		}

		if gate.TryLock() {
			return nil
		}
	}
	return errSignYunBeiGateBusy
}

// describeSchedule renders the next activation of sched in a human-readable form,
// e.g. "2026-08-26 18:00:00 CST (6h30m0s)". The relative duration comes from the
// standard library time.Duration string, so no extra dependency is needed.
func describeSchedule(sched cron.Schedule, loc *time.Location) string {
	next := sched.Next(time.Now().In(loc))
	return fmt.Sprintf("%s（%s）", next.Format("2006-01-02 15:04:05 MST"), time.Until(next).Round(time.Second))
}

func (c *Task) execute(ctx context.Context, _ []string) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	c.l.Debugf(
		"task selection: location=%q runAll=%t sign=%t yunbei=%t partner=%t scrobble=%t share=%t fansgroup=%t",
		c.opts.Location,
		c.opts.RunAll,
		c.opts.SignIn,
		c.opts.YunBei,
		c.opts.Partner,
		c.opts.Scrobble,
		c.opts.SongShare,
		c.opts.FansGroup,
	)

	local, err := time.LoadLocation(c.opts.Location)
	if err != nil {
		return fmt.Errorf("wrong time zone: %w", err)
	}

	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskShutdownTimeout)
		defer cancel()

		closeAPIClient(closeCtx, cli, c.l)
	}()

	request := weapi.New(cli)
	if request.NeedLogin(ctx) {
		return errors.New("need login")
	}

	c.cmd.Printf("\n⚙️ 定时任务\n\n")

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		job           = cron.New(cron.WithLocation(local))
		signRunning   atomic.Bool
		yunBeiRunning atomic.Bool
		partner       = func() error {
			command := NewPartner(c.root, c.l)
			command.opts = c.opts.PartnerOpts
			return c.registerScheduledCommand(runCtx, job, "partner", c.opts.PartnerOptsCrontab, "crontab error", command)
		}
		scrobble = func() error {
			command := NewScrobble(c.root, c.l)
			command.opts = c.opts.ScrobbleOpts
			return c.registerScheduledCommand(runCtx, job, "scrobble", c.opts.ScrobbleOptsCrontab, "[scrobble] crontab error", command)
		}
		signIn = func() error {
			command := NewSignIn(c.root, c.l)
			command.opts = c.opts.SignInOpts
			return c.registerScheduledCommandWithGate(
				runCtx, job, "sign", c.opts.SignInOptsCrontab, "[sign] crontab error", command,
				&scheduledCommandGuard{gate: &c.signYunBeiGate, running: &signRunning},
			)
		}
		yunBei = func() error {
			command := NewYunBei(c.root, c.l)
			command.opts = c.effectiveYunBeiOpts()
			return c.registerScheduledCommandWithGate(
				runCtx, job, "yunbei", c.opts.YunBeiCron, "[yunbei] crontab error", command,
				&scheduledCommandGuard{gate: &c.signYunBeiGate, running: &yunBeiRunning},
			)
		}
		dailySongShare = func() error {
			command := NewDailySongShare(c.root, c.l)
			command.opts = c.opts.ShareOpts
			return c.registerScheduledCommand(runCtx, job, "share", c.opts.ShareOptsCrontab, "[share] crontab error", command)
		}
		fansGroup = func() error {
			// 复用一次性命令完整流程, 不在 task.go 复制乐迷团 API 编排 (US-005)
			command := NewFansGroup(c.root, c.l)
			command.opts = c.opts.FansGroupOpts
			return c.registerScheduledCommand(runCtx, job, "fansgroup", c.opts.FansGroupOptsCrontab, "[fansgroup] crontab error", command)
		}
	)

	sel, err := c.taskSelection()
	if err != nil {
		return err
	}

	if sel.SignIn {
		if err := signIn(); err != nil {
			return err
		}
	}

	if sel.YunBei {
		if err := yunBei(); err != nil {
			return err
		}
	}

	if sel.Partner {
		if err := partner(); err != nil {
			return err
		}
	}

	if sel.Scrobble {
		if err := scrobble(); err != nil {
			return err
		}
	}

	if sel.SongShare {
		if err := dailySongShare(); err != nil {
			return err
		}
	}

	if sel.FansGroup {
		if err := fansGroup(); err != nil {
			return err
		}
	}

	if sel.YunBei {
		c.cmd.Println("  [yunbei] 风险: 会公开发布图文并执行点赞、关注、收藏、红心和站外分享，产生网络流量，长时播放可能超过 45 分钟")
	}

	job.Start()

	c.l.Infof("task service started: jobs=%d tz=%s", len(job.Entries()), local)
	c.cmd.Printf("  已注册任务: %d\n", len(job.Entries()))
	c.cmd.Printf("  时区: %s\n", local)
	c.cmd.Printf("\n服务已启动（Ctrl+C 退出）\n")

	nohup.Daemon(nohup.CloseHook(func(ctx context.Context) error {
		return c.stopScheduledTasks(ctx, cancel, job.Stop)
	}))
	return nil
}

func (c *Task) stopScheduledTasks(ctx context.Context, cancel context.CancelFunc, stop func() context.Context) error {
	cancel()

	done := stop()

	waitCtx, waitCancel := c.withTimeout(context.WithoutCancel(ctx), taskShutdownTimeout)
	defer waitCancel()

	select {
	case <-done.Done():
		return nil
	case <-waitCtx.Done():
		return fmt.Errorf("wait for scheduled tasks after %s: %w", taskShutdownTimeout, waitCtx.Err())
	}
}

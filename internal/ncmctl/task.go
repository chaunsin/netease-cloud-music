// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/nohup"
)

type TaskOpts struct {
	PartnerOpts
	ScrobbleOpts
	SignInOpts
	ShareOpts

	Location string
	RunAll   bool

	Partner            bool
	PartnerOptsCrontab string

	Scrobble            bool
	ScrobbleOptsCrontab string

	SignIn            bool
	SignInOptsCrontab string

	SongShare        bool
	ShareOptsCrontab string
}

type Task struct {
	root *Root
	cmd  *cobra.Command
	opts TaskOpts
	l    *log.Logger
}

type scheduledCommand interface {
	Command() *cobra.Command
	validate() error
}

func NewTask(root *Root, l *log.Logger) *Task {
	c := &Task{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:   "task",
			Short: "Schedule account tasks as a long-running service",
			Long: "Schedule sign, partner, scrobble and daily song challenge jobs using five-field cron expressions. " +
				"Login is required. Without --runAll or any task selector the command fails fast. " +
				"Explicit selectors register only those jobs; --runAll schedules all four jobs. The service runs until interrupted.",
			Example: "  # Schedule all tasks\n" +
				"  ncmctl task --runAll\n\n" +
				"  # Schedule only sign and scrobble\n" +
				"  ncmctl task --sign --scrobble\n\n" +
				"  # Run scrobble daily at 20:00 in the selected time zone\n" +
				"  ncmctl task --scrobble --scrobble.cron '0 20 * * *' --location Asia/Shanghai",
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
	c.cmd.PersistentFlags().BoolVar(&c.opts.RunAll, "runAll", false, "schedule all tasks (sign, partner, scrobble and daily song challenge)")

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

	c.cmd.PersistentFlags().BoolVar(&c.opts.SongShare, "share", false, "schedule the public daily song challenge task")
	c.cmd.PersistentFlags().StringVar(&c.opts.ShareOptsCrontab, "share.cron", "0 9 * * *", "five-field cron schedule for daily song challenge")
	c.cmd.PersistentFlags().Int64Var(&c.opts.SongID, "share.song-id", 0, "fixed song ID for daily song challenge")
	c.cmd.PersistentFlags().StringVar(&c.opts.Image, "share.image", "", "fixed image file for daily song challenge")
	c.cmd.PersistentFlags().StringVar(&c.opts.Title, "share.title", "", "title override for daily song challenge")
	c.cmd.PersistentFlags().StringVar(&c.opts.Message, "share.message", "", "message override for daily song challenge")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Draw, "share.draw", true, "draw rewards after daily song challenge")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Delete, "share.delete", false, "delete each new note after lottery")
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

			if c.opts.Delete && !c.opts.Draw {
				return errors.New("share.delete requires draw=true")
			}
			return nil
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
	return nil
}

type taskSelection struct {
	SignIn    bool
	Partner   bool
	Scrobble  bool
	SongShare bool
}

func (c *Task) taskSelection() (taskSelection, error) {
	o := c.opts
	if o.RunAll {
		return taskSelection{SignIn: true, Partner: true, Scrobble: true, SongShare: true}, nil
	}

	if !o.SignIn && !o.Partner && !o.Scrobble && !o.SongShare {
		return taskSelection{}, errors.New("no task selected: specify at least one of --sign/--partner/--scrobble/--share or --runAll")
	}
	return taskSelection{SignIn: o.SignIn, Partner: o.Partner, Scrobble: o.Scrobble, SongShare: o.SongShare}, nil
}

func (c *Task) registerScheduledCommand(ctx context.Context, job *cron.Cron, name, schedule, cronError string, command scheduledCommand) error {
	label := "[" + name + "]"
	c.cmd.Println(label + " task register")
	c.l.Infof("%s task register", label)

	command.Command().DisableFlagParsing = true
	command.Command().SetArgs([]string{})

	if err := command.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	id, err := job.AddFunc(schedule, func() {
		c.l.Infof("%s task start", label)

		if err := command.Command().ExecuteContext(ctx); err != nil {
			c.l.Errorf(label+" execute err: %s", err)
			return
		}

		c.l.Infof("%s execute success", label)
	})
	if err != nil {
		return fmt.Errorf("%s: %w", cronError, err)
	}

	c.l.Infof(label+" next execute: %s", job.Entry(id).Schedule.Next(time.Now()))
	return nil
}

func (c *Task) execute(ctx context.Context, _ []string) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	c.l.Debugf("task args: %+v", c.opts)

	local, err := time.LoadLocation(c.opts.Location)
	if err != nil {
		return fmt.Errorf("wrong time zone: %w", err)
	}

	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer closeAPIClient(ctx, cli, c.l)

	request := weapi.New(cli)
	if request.NeedLogin(ctx) {
		return errors.New("need login")
	}

	var (
		job     = cron.New(cron.WithLocation(local))
		partner = func() error {
			command := NewPartner(c.root, c.l)
			command.opts = c.opts.PartnerOpts
			return c.registerScheduledCommand(ctx, job, "partner", c.opts.PartnerOptsCrontab, "crontab error", command)
		}
		scrobble = func() error {
			command := NewScrobble(c.root, c.l)
			command.opts = c.opts.ScrobbleOpts
			return c.registerScheduledCommand(ctx, job, "scrobble", c.opts.ScrobbleOptsCrontab, "[scrobble] crontab error", command)
		}
		signIn = func() error {
			command := NewSignIn(c.root, c.l)
			command.opts = c.opts.SignInOpts
			return c.registerScheduledCommand(ctx, job, "sign", c.opts.SignInOptsCrontab, "[sign] crontab error", command)
		}
		dailySongShare = func() error {
			command := NewDailySongShare(c.root, c.l)
			command.opts = c.opts.ShareOpts
			return c.registerScheduledCommand(ctx, job, "share", c.opts.ShareOptsCrontab, "[share] crontab error", command)
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

	job.Start()

	nohup.Daemon(nohup.CloseHook(func(ctx context.Context) error {
		job.Stop()
		return nil
	}))
	return nil
}

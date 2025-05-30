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
	"errors"
	"fmt"
	"time"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/nohup"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

type TaskOpts struct {
	Location string
	RunAll   bool

	Partner            bool
	PartnerOptsCrontab string
	PartnerOpts

	Scrobble            bool
	ScrobbleOptsCrontab string
	ScrobbleOpts

	SignIn            bool
	SignInOptsCrontab string
	SignInOpts
}

type Task struct {
	root *Root
	cmd  *cobra.Command
	opts TaskOpts
	l    *log.Logger
}

func NewTask(root *Root, l *log.Logger) *Task {
	c := &Task{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "task",
			Short:   "[need login] Daily tasks are executed asynchronously [partner、scrobble、sign]",
			Example: `  ncmctl task`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}
	return c
}

func (c *Task) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Location, "location", "l", "Asia/Shanghai", "crontab time zone setting")
	c.cmd.PersistentFlags().BoolVar(&c.opts.RunAll, "runAll", false, "default enabled all task")

	c.cmd.PersistentFlags().BoolVar(&c.opts.Partner, "partner", false, "enabled partner task")
	c.cmd.PersistentFlags().StringVar(&c.opts.PartnerOptsCrontab, "partner.cron", "0 18 * * *", "partner crontab expression. usage detail: https://crontab.guru")
	c.cmd.PersistentFlags().Int64SliceVar(&c.opts.PartnerOpts.Star, "partner.star", []int64{3, 4}, "set the base song evaluation score level random range 1-5")
	c.cmd.PersistentFlags().Int64SliceVar(&c.opts.PartnerOpts.ExtStar, "partner.extStar", []int64{2, 3, 4}, "set the extra song evaluation score level random range 1-5")
	c.cmd.PersistentFlags().StringVar(&c.opts.PartnerOpts.ExtNum, "partner.extNum", "random", "extra evaluation number of songs,'random' means 2 to 7")

	c.cmd.PersistentFlags().BoolVar(&c.opts.Scrobble, "scrobble", false, "enabled scrobble task")
	c.cmd.PersistentFlags().StringVar(&c.opts.ScrobbleOptsCrontab, "scrobble.cron", "0 18 * * *", "scrobble crontab expression. usage detail: https://crontab.guru")
	c.cmd.PersistentFlags().Int64Var(&c.opts.ScrobbleOpts.Num, "scrobble.num",  300, "scrobble num of songs")

	c.cmd.PersistentFlags().BoolVar(&c.opts.SignIn, "sign", false, "enabled sign task")
	c.cmd.PersistentFlags().StringVar(&c.opts.SignInOptsCrontab, "sign.cron", "0 10 * * *", "sign crontab expression. usage detail: https://crontab.guru")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Automatic, "sign.automatic", false, "automatically claim sign-in rewards")
}

func (c *Task) validate() error {
	var (
		partner = func() error {
			if c.opts.PartnerOptsCrontab == "" {
				return fmt.Errorf("partner.crontab is required")
			}
			if _, err := cron.ParseStandard(c.opts.PartnerOptsCrontab); err != nil {
				return fmt.Errorf("ParseStandard: %w", err)
			}
			return nil
		}
		signIn = func() error {
			if c.opts.SignInOptsCrontab == "" {
				return fmt.Errorf("sign.crontab is required")
			}
			if _, err := cron.ParseStandard(c.opts.SignInOptsCrontab); err != nil {
				return fmt.Errorf("ParseStandard: %w", err)
			}
			return nil
		}
		scrobble = func() error {
			if c.opts.ScrobbleOptsCrontab == "" {
				return fmt.Errorf("scrobble.crontab is required")
			}
			if _, err := cron.ParseStandard(c.opts.ScrobbleOptsCrontab); err != nil {
				return fmt.Errorf("ParseStandard: %w", err)
			}
			return nil
		}
	)

	var o = c.opts
	if o.RunAll || (!o.SignIn && !o.Partner && !o.Scrobble) {
		return errors.Join(signIn(), partner(), scrobble())
	} else {
		if o.SignIn {
			if err := signIn(); err != nil {
				return err
			}
		}
		if o.Partner {
			if err := partner(); err != nil {
				return err
			}
		}
		if o.Scrobble {
			if err := scrobble(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Task) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Task) Command() *cobra.Command {
	return c.cmd
}

func (c *Task) execute(ctx context.Context, args []string) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	log.Debug("task args: %+v", c.opts)

	local, err := time.LoadLocation(c.opts.Location)
	if err != nil {
		return fmt.Errorf("wrong time zone: %w", err)
	}

	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)

	request := weapi.New(cli)
	if request.NeedLogin(ctx) {
		return fmt.Errorf("need login")
	}

	var (
		job     = cron.New(cron.WithLocation(local))
		partner = func() error {
			c.cmd.Println("[partner] task register")
			log.Info("[partner] task register")
			partner := NewPartner(c.root, c.l)
			partner.cmd.DisableFlagParsing = true // 关闭子命令解析比如出现unknown flag错误
			partner.opts = c.opts.PartnerOpts
			if err := partner.validate(); err != nil {
				return fmt.Errorf("validate: %w", err)
			}

			id, err := job.AddFunc(c.opts.PartnerOptsCrontab, func() {
				log.Info("[partner] task start")
				if err := partner.Command().ExecuteContext(ctx); err != nil {
					log.Error("[partner] execute err: %s", err)
					return
				}
				log.Info("[partner] execute success")
			})
			if err != nil {
				return fmt.Errorf("crontab error: %v", err)
			}
			log.Info("[partner] next execute: %s", job.Entry(id).Schedule.Next(time.Now()))
			return nil
		}
		scrobble = func() error {
			c.cmd.Println("[scrobble] task register")
			log.Info("[scrobble] task register")
			s := NewScrobble(c.root, c.l)
			s.cmd.DisableFlagParsing = true
			s.opts = c.opts.ScrobbleOpts
			if err := s.validate(); err != nil {
				return fmt.Errorf("validate: %w", err)
			}

			id, err := job.AddFunc(c.opts.ScrobbleOptsCrontab, func() {
				log.Info("[scrobble] task start")
				if err := s.Command().ExecuteContext(ctx); err != nil {
					log.Error("[scrobble] execute err: %s", err)
					return
				}
				log.Info("[scrobble] execute success")
			})
			if err != nil {
				return fmt.Errorf("[scrobble] crontab error: %v", err)
			}
			log.Info("[scrobble] next execute: %s", job.Entry(id).Schedule.Next(time.Now()))
			return nil
		}
		signIn = func() error {
			c.cmd.Println("[sign] task register")
			log.Info("[sign] task register")
			signIn := NewSignIn(c.root, c.l)
			signIn.cmd.DisableFlagParsing = true
			signIn.opts = c.opts.SignInOpts
			if err := signIn.validate(); err != nil {
				return fmt.Errorf("validate: %w", err)
			}

			id, err := job.AddFunc(c.opts.SignInOptsCrontab, func() {
				log.Info("[sign] task start")
				if err := signIn.Command().ExecuteContext(ctx); err != nil {
					log.Error("[sign] execute err: %s", err)
					return
				}
				log.Info("[sign] execute success")
			})
			if err != nil {
				return fmt.Errorf("[sign] crontab error: %v", err)
			}
			log.Info("[sign] next execute: %s", job.Entry(id).Schedule.Next(time.Now()))
			return nil
		}
	)

	var o = c.opts
	if o.RunAll || (!o.SignIn && !o.Partner && !o.Scrobble) {
		if err := errors.Join(signIn(), partner(), scrobble()); err != nil {
			return err
		}
	} else {
		if o.SignIn {
			if err := signIn(); err != nil {
				return err
			}
		}
		if o.Partner {
			if err := partner(); err != nil {
				return err
			}
		}
		if o.Scrobble {
			if err := scrobble(); err != nil {
				return err
			}
		}
	}

	job.Start()

	nohup.Daemon(nohup.CloseHook(func(ctx context.Context) error {
		job.Stop()
		return nil
	}))
	return nil
}

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
	"sync"

	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

type TaskOpts struct {
	PartnerOpts
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
			Short:   "Task async execute daily task",
			Example: `  ncm task`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}
	return c
}

func (c *Task) addFlags() {
	c.cmd.PersistentFlags().StringVar(&c.opts.PartnerOpts.Crontab, "partner.crontab", "* 18 * * *", "https://crontab.guru/")
	c.cmd.PersistentFlags().Int64SliceVar(&c.opts.PartnerOpts.Star, "partner.star", []int64{3, 4}, "star level")
	c.cmd.PersistentFlags().BoolVar(&c.opts.PartnerOpts.Once, "partner.once", false, "real-time execution once")
}

func (c *Task) validate() error {
	if c.opts.Crontab == "" {
		return fmt.Errorf("crontab is required")
	}
	_, err := cron.ParseStandard(c.opts.Crontab)
	if err != nil {
		return fmt.Errorf("ParseStandard: %w", err)
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
	log.Debug("task args: %+v\n", c.opts)

	var wg sync.WaitGroup

	partner := NewPartner(c.root, c.l)
	partner.cmd.DisableFlagParsing = true // 关闭子命令解析比如出现unknown flag错误
	partner.opts = c.opts.PartnerOpts
	if err := partner.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	// NotifyContext TODO:信号量考虑退出问题，由于partner子命令中也存在着处理，此处处理需要兼容考虑

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := partner.Command().ExecuteContext(ctx); err != nil {
			// return fmt.Errorf("ExecuteContext: %w", err)
			log.Error("[partner] execute err: %s", err)
		}
	}()
	wg.Wait()

	return nil
}

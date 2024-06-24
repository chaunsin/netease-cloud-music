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

	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/robfig/cron/v3"

	"github.com/spf13/cobra"
)

type ScrobbleOpts struct {
	Crontab string
	Once    bool
	// Tags    []string
}

type Scrobble struct {
	root *Root
	cmd  *cobra.Command
	opts ScrobbleOpts
	l    *log.Logger
}

func NewScrobble(root *Root, l *log.Logger) *Scrobble {
	c := &Scrobble{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "scrobble",
			Short:   "Scrobble async execute refresh 300 songs",
			Example: `  ncm partner`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context())
	}
	return c
}

func (c *Scrobble) addFlags() {
	c.cmd.PersistentFlags().StringVar(&c.opts.Crontab, "crontab", "* 18 * * *", "https://crontab.guru/")
	c.cmd.PersistentFlags().BoolVarP(&c.opts.Once, "once", "", false, "real-time execution once")
}

func (c *Scrobble) validate() error {
	if c.opts.Crontab == "" {
		return fmt.Errorf("crontab is required")
	}
	_, err := cron.ParseStandard(c.opts.Crontab)
	if err != nil {
		return fmt.Errorf("ParseStandard: %w", err)
	}
	return nil
}

func (c *Scrobble) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Scrobble) Command() *cobra.Command {
	return c.cmd
}

func (c *Scrobble) execute(ctx context.Context) error {
	return nil
}

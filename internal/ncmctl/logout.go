// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"fmt"
	"os"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/spf13/cobra"
)

type LogoutOpts struct{}

type Logout struct {
	root *Root
	cmd  *cobra.Command
	opts LogoutOpts
	l    *log.Logger
}

func NewLogout(root *Root, l *log.Logger) *Logout {
	c := &Logout{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "logout",
			Short:   "Logout netease cloud music",
			Example: "  ncmctl logout",
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}

	return c
}

func (c *Logout) addFlags() {}

func (c *Logout) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Logout) Command() *cobra.Command {
	return c.cmd
}

func (c *Logout) execute(ctx context.Context, _ []string) error {
	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)

	request := weapi.New(cli)
	resp, err := request.Layout(ctx, &weapi.LayoutReq{})
	if err != nil {
		return fmt.Errorf("layout: %w", err)
	}
	if resp.Code != 200 {
		return fmt.Errorf("layout: %+v", resp)
	}

	// 只清理默认目录下得文件
	if err := os.Remove(c.root.Opts.Home + "/.ncmctl/cookie.json"); err != nil {
		log.Debug("remove cookie.json: %s", err)
	}
	c.cmd.Println("Logout success")
	return nil
}

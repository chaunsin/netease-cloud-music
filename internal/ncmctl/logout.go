// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type Logout struct {
	root *Root
	cmd  *cobra.Command
	l    *log.Logger
}

func NewLogout(root *Root, l *log.Logger) *Logout {
	c := &Logout{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:   "logout",
			Short: "Log out and remove the persisted Cookie",
			Long: "Call the NetEase logout endpoint and remove <home>/.ncmctl/cookie.json. " +
				"A custom Cookie path selected through configuration is not removed automatically.",
			Example: "  ncmctl logout\n" +
				"  ncmctl --home /srv/ncmctl logout",
			Args: cobra.NoArgs,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}

	return c
}

func (c *Logout) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Logout) Command() *cobra.Command {
	return c.cmd
}

func (c *Logout) addFlags() {}

func closeAndRemoveDefaultCookie(ctx context.Context, cli *api.Client, home string) error {
	if err := cli.Close(ctx); err != nil {
		return fmt.Errorf("close API client: %w", err)
	}

	cookiePath := filepath.Join(home, ".ncmctl", "cookie.json")
	if err := os.Remove(cookiePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove cookie file: %w", err)
	}
	return nil
}

func (c *Logout) execute(ctx context.Context, _ []string) error {
	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer closeAPIClient(ctx, cli)

	request := weapi.New(cli)

	resp, err := request.Layout(ctx, &weapi.LayoutReq{})
	if err != nil {
		return fmt.Errorf("layout: %w", err)
	}

	if resp.Code != 200 {
		return fmt.Errorf("layout: %+v", resp)
	}

	// Flush the logged-out jar before deleting the default credential file.
	if err := closeAndRemoveDefaultCookie(ctx, cli, c.root.Opts.Home); err != nil {
		return err
	}

	c.cmd.Println("Logout success")
	return nil
}

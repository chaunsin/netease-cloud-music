// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type LogoutOpts struct {
	ClearAnonymousToken bool
}

type Logout struct {
	root *Root
	cmd  *cobra.Command
	l    *log.Logger
	opts LogoutOpts
}

func NewLogout(root *Root, l *log.Logger) *Logout {
	c := &Logout{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:   "logout",
			Short: "Log out and remove persisted session state",
			Long: "Call the NetEase logout endpoint and remove <home>/.ncmctl/cookie.json and <home>/.ncmctl/xeapi.yaml. " +
				"Use --clear-anonymous-token to also remove <home>/.ncmctl/anonymous_token. " +
				"A custom Cookie path selected through configuration is not removed automatically.",
			Example: "  ncmctl logout\n" +
				"  ncmctl logout --clear-anonymous-token\n" +
				"  ncmctl --home /srv/ncmctl logout --clear-anonymous-token",
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

func (c *Logout) addFlags() {
	c.cmd.Flags().BoolVar(
		&c.opts.ClearAnonymousToken,
		"clear-anonymous-token",
		false,
		"also remove <home>/.ncmctl/anonymous_token (preserved by default)",
	)
}

func (c *Logout) execute(ctx context.Context, _ []string) error {
	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}

	closeOnReturn := true
	defer func() {
		if closeOnReturn {
			closeAPIClient(ctx, cli)
		}
	}()

	request := weapi.New(cli)

	resp, err := request.Layout(ctx, &weapi.LayoutReq{})
	if err != nil {
		return fmt.Errorf("layout: %w", err)
	}

	if resp.Code != 200 {
		return fmt.Errorf("layout: %+v", resp)
	}

	// This cleanup owns the final Close; closing again would resync deleted state files.
	closeOnReturn = false

	if err := c.closeAndclear(context.WithoutCancel(ctx), cli, c.root.Cfg.Network.HomeDir, c.opts.ClearAnonymousToken); err != nil {
		return err
	}

	c.cmd.Println("Logout success")
	return nil
}

func (c *Logout) closeAndclear(ctx context.Context, cli *api.Client, home string, clearAnonymousToken bool) error {
	var errs []error

	if err := cli.Close(ctx); err != nil {
		errs = append(errs, fmt.Errorf("close API client: %w", err))
	}

	files := []string{"cookie.json", "xeapi.yaml", "xeapi.json"}
	if clearAnonymousToken {
		files = append(files, "anonymous_token")
	}

	stateDir := filepath.Join(home, ".ncmctl")
	for _, name := range files {
		if err := os.Remove(filepath.Join(stateDir, name)); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove %s: %w", name, err))
		}
	}

	return errors.Join(errs...)
}

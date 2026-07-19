// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type Login struct {
	root *Root
	cmd  *cobra.Command
	l    *log.Logger
}

func NewLogin(root *Root, l *log.Logger) *Login {
	c := &Login{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "login",
			Short:   "Login netease cloud music",
			Example: "  ncmctl login -h\n  ncmctl login qrcode\n  ncmctl login phone\n  ncmctl login cookiecloud\n  ncmctl login cookie",
		},
	}
	c.addFlags()
	c.Add(qrcode(c, l))
	c.Add(phone(c, l))
	c.Add(cookieCloud(c, l))
	c.Add(cookie(c, l))

	return c
}

func (c *Login) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Login) Command() *cobra.Command {
	return c.cmd
}

func (c *Login) addFlags() {}

// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api/weapi"
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
			Use:   "login",
			Short: "Authenticate with NetEase Cloud Music",
			Long: "Authenticate by phone, browser Cookie, CookieCloud, or QR code. Successful " +
				"login validates the account and persists cookies under the configured runtime home. " +
				"Authentication contacts live NetEase services; treat every credential as sensitive.",
			Example: "  ncmctl login qrcode\n" +
				"  ncmctl login phone 18800008888\n" +
				"  ncmctl login cookie --file cookie.txt\n" +
				"  ncmctl login cookiecloud --uuid '<uuid>' --password '<password>'",
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

func validateLoginAccount(user *weapi.GetUserInfoResp) error {
	if user == nil {
		return errors.New("account validation failed: empty response")
	}

	if user.Code != 200 || user.Account == nil || user.Profile == nil {
		return fmt.Errorf("account validation failed: login is invalid or expired (code %d)", user.Code)
	}
	return nil
}

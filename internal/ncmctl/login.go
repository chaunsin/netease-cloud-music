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
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/spf13/cobra"
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

func (c *Login) addFlags() {}

func (c *Login) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Login) Command() *cobra.Command {
	return c.cmd
}

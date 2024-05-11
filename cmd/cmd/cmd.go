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

package cmd

import (
	"github.com/spf13/cobra"
)

type RootOpts struct {
	Debug  bool // 是否开启命令行debug模式
	Stdout bool // 生成内容是否打印到标准数据中
}

type Root struct {
	cmd  *cobra.Command
	Opts RootOpts
}

func New() *Root {
	c := &Root{
		cmd: &cobra.Command{
			Use:   "ncm",
			Short: "ncm is a tool for encrypting and decrypting the ncm file.",
			Example: `ncm -h
ncm crypto decrypt -k eapi -c xxx
ncm crypto encrypt -k eapi -P xxx
ncm login qrcode -a xx`,
		},
	}
	c.addFlags()
	c.Add(NewCrypto(c).Command())
	c.Add(NewLogin(c).Command())
	c.Add(NewPartner(c).Command())

	return c
}

func (c *Root) addFlags() {
	c.cmd.PersistentFlags().BoolVar(&c.Opts.Debug, "debug", false, "")
	c.cmd.PersistentFlags().BoolVar(&c.Opts.Stdout, "stdout", false, "")
}

func (c *Root) Version(version string) {
	c.cmd.Version = version
}

func (c *Root) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Root) Execute() {
	if err := c.cmd.Execute(); err != nil {
		panic(err)
	}
}

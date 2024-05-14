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
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/spf13/cobra"
)

type CryptoOpts struct {
	File   string // 加载文件路径
	Output string // 生成文件路径
	Kind   string // api类型
}

type Crypto struct {
	root *Root
	cmd  *cobra.Command
	opts CryptoOpts
	l    *log.Logger
}

func NewCrypto(root *Root, l *log.Logger) *Crypto {
	c := &Crypto{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:   "crypto",
			Short: "crypto is a tool for encrypting and decrypting the http data",
			Example: `ncm crypto -h
ncm crypto decrypt -k eapi -c xxx
ncm crypto encrypt -k eapi -P xxx`,
		},
	}
	c.addFlags()
	c.Add(encrypt(c, l))
	c.Add(decrypt(c, l))
	return c
}

func (c *Crypto) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.File, "file", "f", "", "ncm [command] -f ./file.har")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "", "Generate decrypt file directory location")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Kind, "kind", "k", "weapi", "weapi|eapi|linux")
}

func (c *Crypto) Version(version string) {
	c.cmd.Version = version
}

func (c *Crypto) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Crypto) Execute() {
	if err := c.cmd.Execute(); err != nil {
		panic(err)
	}
}

func (c *Crypto) Command() *cobra.Command {
	return c.cmd
}

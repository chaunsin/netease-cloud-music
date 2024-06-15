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
	"context"

	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/spf13/cobra"
)

type CloudOpts struct {
	Input    string // 加载文件路径,或文件
	Parallel int64  // 并发上传文件数量

	Output string // 生成文件路径
}

type Cloud struct {
	root *Root
	cmd  *cobra.Command
	opts CloudOpts
	l    *log.Logger
}

func NewCloud(root *Root, l *log.Logger) *Cloud {
	c := &Cloud{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "cloud",
			Short:   "Cloud is a tool for encrypting and decrypting the http data",
			Example: "  ncm cloud -h\n  ncm cloud xxx",
			Args:    cobra.RangeArgs(0, 1),
		},
	}
	c.addFlags()
	c.cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := c.execute(cmd.Context(), args); err != nil {
			cmd.Println(err)
		}
	}

	return c
}

func (c *Cloud) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Input, "input", "i", "", "*.har file path、text")
	c.cmd.PersistentFlags().Int64VarP(&c.opts.Parallel, "parallel", "p", 10, "concurrent upload count")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "", "generate decrypt file directory location")
}

func (c *Cloud) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Cloud) Command() *cobra.Command {
	return c.cmd
}

func (c *Cloud) execute(ctx context.Context, args []string) error {
	// 执行命令行指定文件上传
	if len(args) > 0 {
		// 判断是否是单个文件
		utils.IsFile(args[0])

		return nil
	}

	// 执行目录文件上传

	c.cmd.Println(args)
	return nil
}

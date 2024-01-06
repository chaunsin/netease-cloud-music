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

package crypto

import (
	"os"

	"github.com/spf13/cobra"
)

type RootOpts struct {
	Debug  bool   // 是否开启命令行debug模式
	Input  string // 加载文件路径
	Output string // 生成文件路径
	Stdout bool   // 生成内容是否打印到标准数据中
}

type Cmd struct {
	root     *cobra.Command
	RootOpts RootOpts
}

func New() *Cmd {
	c := &Cmd{
		root: &cobra.Command{
			Use:     "ncm",
			Example: "ncm -h",
		},
	}
	c.addFlags()
	c.Add(NewEncrypt(c))
	c.Add(NewDecrypt(c))

	return c
}

func (c *Cmd) addFlags() {
	c.root.PersistentFlags().BoolVar(&c.RootOpts.Debug, "debug", false, "")
	c.root.PersistentFlags().StringVarP(&c.RootOpts.Input, "input", "i", "", "ncm [command] -i ./crypto.text")
	c.root.PersistentFlags().StringVarP(&c.RootOpts.Output, "output", "o", "", "Generate decrypt file directory location")
	c.root.PersistentFlags().BoolVar(&c.RootOpts.Stdout, "stdout", false, "")
}

func (c *Cmd) Version(version string) {
	c.root.Version = version
}

func (c *Cmd) Add(command ...*cobra.Command) {
	c.root.AddCommand(command...)
}

func (c *Cmd) Execute() {
	if err := c.root.Execute(); err != nil {
		panic(err)
	}
}

func defaultString(env, value string) string {
	v := os.Getenv(env)
	if v == "" {
		return value
	}
	return v
}

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
	"fmt"

	"github.com/chaunsin/netease-cloud-music/config"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/spf13/cobra"
)

type RootOpts struct {
	Debug  bool   // 是否开启命令行debug模式
	Stdout bool   // 生成内容是否打印到标准数据中
	Config string // 配置文件路径
}

type Root struct {
	Cfg  *config.Config
	Opts RootOpts
	cmd  *cobra.Command
	l    *log.Logger
}

func New() *Root {
	c := &Root{
		cmd: &cobra.Command{
			Use:   "ncmctl",
			Short: "ncmctl command.",
			Long:  `ncmctl is a toolbox for netease cloud music.`,
			Example: `  ncmctl cloud
  ncmctl crypto
  ncmctl login
  ncmctl curl
  ncmctl partner`,
		},
	}
	c.cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if c.Opts.Config == "" {
			return fmt.Errorf("config file not specified")
		}
		if !utils.FileExists(c.Opts.Config) {
			return fmt.Errorf("config file not exists: %s", c.Opts.Config)
		}
		c.Cfg = config.New(c.Opts.Config)

		// 命令行开启了debug模式优先级大于配置文件中得优先级
		if c.Opts.Debug {
			c.Cfg.Log.Stdout = true
			c.Cfg.Log.Level = "debug"
		}

		// 初始化日志
		c.l = log.New(c.Cfg.Log)
		log.Default = c.l
		return nil
	}
	c.cmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		return c.l.Close()
	}

	c.addFlags()
	c.Add(NewCrypto(c, c.l).Command())
	c.Add(NewLogin(c, c.l).Command())
	c.Add(NewPartner(c, c.l).Command())
	c.Add(NewCurl(c, c.l).Command())
	c.Add(NewCloud(c, c.l).Command())
	c.Add(NewTask(c, c.l).Command())
	c.Add(NewScrobble(c, c.l).Command())
	c.Add(NewSignIn(c, c.l).Command())
	c.Add(NewNCM(c, c.l).Command())
	return c
}

func (c *Root) addFlags() {
	c.cmd.PersistentFlags().BoolVar(&c.Opts.Debug, "debug", false, "")
	c.cmd.PersistentFlags().BoolVar(&c.Opts.Stdout, "stdout", false, "")
	c.cmd.PersistentFlags().StringVarP(&c.Opts.Config, "config", "c", "./config.yaml", "")
}

func (c *Root) Version(version string) {
	c.cmd.Version = version
}

func (c *Root) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Root) Execute() {
	if err := c.cmd.Execute(); err != nil {
		c.cmd.PrintErrln(err)
	}
}

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
	"os"
	"path/filepath"
	"runtime"

	"github.com/chaunsin/netease-cloud-music/config"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/spf13/cobra"
)

type RootOpts struct {
	Debug  bool   // 是否开启命令行debug模式
	Config string // 配置文件路径
	Home   string
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
	c.cmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)
	c.cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		var (
			cfgPath = c.Opts.Config
			home    = filepath.Clean(utils.Ternary(c.Opts.Home != "", c.Opts.Home, config.HomeDir))
		)
		if c.Opts.Config != "" {
			var err error
			if !utils.FileExists(c.Opts.Config) {
				return fmt.Errorf("config file not exists: %s", c.Opts.Config)
			}
			c.Cfg, err = config.New(c.Opts.Config)
			if err != nil {
				return fmt.Errorf("init config error: %s", err)
			}
		} else {
			cfgPath = "default"
			c.Cfg = config.GetDefault()
		}

		c.Cfg.ReplaceMagicVariables("HOME", home)
		if err := c.Cfg.Validate(); err != nil {
			return fmt.Errorf("config validate error: %s", err)
		}

		// todo: 暂时关闭debug模式,api中得resty日志需要统一输出本库中得logger里
		c.Cfg.Network.Debug = false
		// 命令行开启了debug模式优先级大于配置文件中得优先级
		if c.Opts.Debug {
			c.Cfg.Log.Stdout = true
			c.Cfg.Log.Level = "debug"
			c.Cfg.Network.Debug = true
		}

		// init logger
		c.l = log.New(c.Cfg.Log)
		log.Default = c.l
		log.Debug("[config] init home=%s path=%s log=%+v network=%+v", home, cfgPath, c.Cfg.Log, c.Cfg.Network)
		return nil
	}
	c.cmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		return c.l.Close()
	}

	c.addFlags()

	// add sub commands
	c.Add(NewCrypto(c, c.l).Command())
	c.Add(NewLogin(c, c.l).Command())
	c.Add(NewLogout(c, c.l).Command())
	c.Add(NewPartner(c, c.l).Command())
	c.Add(NewCurl(c, c.l).Command())
	c.Add(NewCloud(c, c.l).Command())
	c.Add(NewTask(c, c.l).Command())
	c.Add(NewScrobble(c, c.l).Command())
	c.Add(NewSignIn(c, c.l).Command())
	c.Add(NewNCM(c, c.l).Command())
	c.Add(NewDownload(c, c.l).Command())
	return c
}

func (c *Root) addFlags() {
	c.cmd.PersistentFlags().BoolVar(&c.Opts.Debug, "debug", false, "run in debug mode")
	c.cmd.PersistentFlags().StringVarP(&c.Opts.Config, "config", "c", "", "configuration file path")
	c.cmd.PersistentFlags().StringVar(&c.Opts.Home, "home", config.HomeDir, "configuration home path. the home path is used to store running information")
}

func (c *Root) Version(version, buildTime, commitHash string) {
	c.cmd.Version = fmt.Sprintf(" Version: \t%s\n Go version: \t%s\n Git commit: \t%s\n OS/Arch: \t%s\n Build time: \t%s",
		version, runtime.Version(), commitHash, runtime.GOOS+"/"+runtime.GOARCH, buildTime)
}

func (c *Root) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Root) Execute() {
	if err := c.cmd.Execute(); err != nil {
		c.cmd.PrintErrln(err)
		os.Exit(1)
	}
}

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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/linux"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/spf13/cobra"
)

type CurlOpts struct {
	Method  string        // 请求方法
	Input   string        // 加载文件路径,或文本内容
	Output  string        // 生成文件路径
	Kind    string        // api类型
	Timeout time.Duration // 超时时间
}

type Curl struct {
	root *Root
	cmd  *cobra.Command
	opts CurlOpts
	l    *log.Logger
}

func NewCurl(root *Root, l *log.Logger) *Curl {
	c := &Curl{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "curl",
			Short:   "Like curl invoke netease cloud music api",
			Example: "ncm curl -h\nncm curl -k weapi -m Ping -i '{}'",
		},
	}
	c.addFlags()
	c.cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := c.execute(cmd.Context()); err != nil {
			cmd.Println(err)
		}
	}

	return c
}

func (c *Curl) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Method, "method", "m", "", "request method")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Input, "input", "i", `{}`, `request params. eg:'{"id":1,"name":"bob"}'`)
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "", "generate response file directory location")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Kind, "kind", "k", "weapi", "weapi|eapi|linux")
	c.cmd.PersistentFlags().DurationVarP(&c.opts.Timeout, "timeout", "t", 15*time.Second, "request timeout eg:1s、1m")
}

func (c *Curl) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Curl) Command() *cobra.Command {
	return c.cmd
}

func (c *Curl) execute(ctx context.Context) error {
	if c.opts.Method == "" {
		return fmt.Errorf("method is required")
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(c.opts.Input), &args); err != nil {
		return err
	}

	if c.root.Opts.Debug {
		c.root.Cfg.Network.Debug = true
	} else {
		c.root.Cfg.Network.Debug = false
	}

	cli, err := api.NewWithErr(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewWithErr: %w", err)
	}
	defer cli.Close(ctx)

	ctx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	var request any
	switch c.opts.Kind {
	case "epai":
		request = eapi.New(cli)
	case "linux":
		request = linux.New(cli)
	case "weapi":
		fallthrough
	default:
		request = weapi.New(cli)
	}

	method, ok := reflect.TypeOf(request).MethodByName(c.opts.Method)
	if !ok {
		return fmt.Errorf("method %s not found", c.opts.Method)
	}
	if !method.IsExported() {
		return fmt.Errorf("method %s not exported", c.opts.Method)
	}
	// 判断调用方法参数是否为2个
	if n := method.Func.Type().NumIn() - 1; n != 2 {
		return fmt.Errorf("method %s args length %d invalid", c.opts.Method, n)
	}
	log.Debug("method: %+v", method)

	var (
		t        = method.Func.Type()
		req      = t.In(2).Elem() // 0:为当前请求对应 1:context.Context 2:req请求参数
		instance = reflect.New(req).Elem()
	)

	decoder := json.NewDecoder(strings.NewReader(c.opts.Input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(instance.Addr().Interface()); err != nil {
		return err
	}
	log.Debug("request: %+v", instance)

	resp := method.Func.Call([]reflect.Value{reflect.ValueOf(request), reflect.ValueOf(ctx), instance.Addr()})
	if len(resp) != 2 {
		return fmt.Errorf("method %s resp length %d invalid", c.opts.Method, len(resp))
	}
	if !resp[1].IsNil() {
		return resp[1].Interface().(error)
	}
	var data = resp[0].Interface() // 请求返回值
	binary, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return fmt.Errorf("MarshalIndent: %w", err)
	}
	return writefile(c.cmd, c.opts.Output, binary)
}

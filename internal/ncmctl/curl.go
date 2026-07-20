// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"

	client "github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/api"
	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/linux"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type CurlOpts struct {
	Method  string        // 请求方法
	Data    string        // 参数内容
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
			Use:   "curl [method]",
			Short: "Invoke an exported NetEase API wrapper method",
			Long: "Invoke an exported Go method from the selected API wrapper package. This is not " +
				"the system curl command: the method is a Go method name, and --data is decoded into " +
				"its request struct. The selected endpoint determines login requirements and account effects.",
			Example: "  ncmctl curl --kind weapi --data '{}' GetUserInfo\n" +
				"  ncmctl curl --kind weapi --method GetUserInfo --data '{}'\n" +
				"  ncmctl curl --kind weapi --data '{}' GetUserInfo --output response.json",
			Args: cobra.MaximumNArgs(1),
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}
	return c
}

func (c *Curl) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Curl) Command() *cobra.Command {
	return c.cmd
}

func (c *Curl) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Method, "method", "m", "", "exported Go API method name (overrides the positional method; not an HTTP verb)")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Data, "data", "d", `{}`, "JSON object decoded into the method request type; unknown fields are rejected")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "", "write the formatted JSON response to a file instead of stdout")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Kind, "kind", "k", "weapi", "API wrapper package: weapi, eapi, linux, or api")
	c.cmd.PersistentFlags().DurationVarP(&c.opts.Timeout, "timeout", "t", 15*time.Second, "overall API call timeout (for example 1s or 1m)")
}

func validateCurlKind(kind string) error {
	switch kind {
	case "api", "eapi", "linux", "weapi":
		return nil
	default:
		return fmt.Errorf("unsupported API kind %q", kind)
	}
}

func (c *Curl) execute(ctx context.Context, args []string) error {
	var method string
	if len(args) > 0 {
		method = strings.TrimSpace(args[0])
	}

	if c.opts.Method != "" {
		method = c.opts.Method
	}

	if method == "" {
		return errors.New("method is required")
	}

	if err := validateCurlKind(c.opts.Kind); err != nil {
		return err
	}

	cli, err := client.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer closeAPIClient(ctx, cli)

	ctx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	var request any

	switch c.opts.Kind {
	case "api":
		request = api.New(cli)
	case "eapi":
		request = eapi.New(cli)
	case "linux":
		request = linux.New(cli)
	case "weapi":
		request = weapi.New(cli)
	}

	methodName, ok := reflect.TypeOf(request).MethodByName(method)
	if !ok {
		return fmt.Errorf("method %s not found", method)
	}

	if !methodName.IsExported() {
		return fmt.Errorf("method %s not exported", method)
	}
	// 判断调用方法参数是否为2个
	if n := methodName.Func.Type().NumIn() - 1; n != 2 {
		return fmt.Errorf("method %s args length %d invalid", c.opts.Method, n)
	}

	log.Debugf("method: %+v", methodName)

	var (
		t        = methodName.Func.Type()
		req      = t.In(2).Elem() // 0:为当前请求 1:context.Context 2:req请求参数
		instance = reflect.New(req).Elem()
	)

	decoder := json.NewDecoder(strings.NewReader(c.opts.Data))
	decoder.DisallowUnknownFields()

	if decodeErr := decoder.Decode(instance.Addr().Interface()); decodeErr != nil {
		return fmt.Errorf("decode: %w", decodeErr)
	}

	log.Debugf("request type: %s", req)

	resp := methodName.Func.Call([]reflect.Value{reflect.ValueOf(request), reflect.ValueOf(ctx), instance.Addr()})
	if len(resp) != 2 {
		return fmt.Errorf("method %s resp length %d invalid", c.opts.Method, len(resp))
	}

	if !resp[1].IsNil() {
		responseErr, ok := resp[1].Interface().(error)
		if !ok {
			return fmt.Errorf("method %s returned non-error value %T", c.opts.Method, resp[1].Interface())
		}
		return responseErr
	}

	data := resp[0].Interface() // 请求返回值

	binary, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return fmt.Errorf("MarshalIndent: %w", err)
	}
	return writeFile(c.cmd, c.opts.Output, binary)
}

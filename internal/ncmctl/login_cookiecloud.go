// MIT License
//
// Copyright (c) 2025 chaunsin
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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/cookiecloud"
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/spf13/cobra"
)

type loginCookieCloudCmd struct {
	root *Login
	cmd  *cobra.Command
	l    *log.Logger

	timeout  time.Duration // 超时时间
	server   string
	uuid     string
	password string
	headers  string
}

func cookieCloud(root *Login, l *log.Logger) *cobra.Command {
	c := &loginCookieCloudCmd{
		root: root,
		l:    l,
	}
	c.cmd = &cobra.Command{
		Use:     "cookiecloud",
		Short:   "use cookiecloud login\n  detail: https://github.com/easychen/CookieCloud",
		Example: "  ncmctl login cookiecloud -u <your uuid> -p <your password> -s http://127.0.0.1:8088",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute(cmd.Context(), args)
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *loginCookieCloudCmd) addFlags() {
	c.cmd.Flags().DurationVarP(&c.timeout, "timeout", "t", time.Second*30, "timeout, eg: 1s、1m")
	c.cmd.Flags().StringVarP(&c.server, "server", "s", "http://127.0.0.1:8088", "cookiecloud server address")
	c.cmd.Flags().StringVarP(&c.uuid, "uuid", "u", "", "login account uuid")
	c.cmd.Flags().StringVarP(&c.password, "password", "p", "", "use when logging in with a password.")
	c.cmd.Flags().StringVarP(&c.headers, "headers", "H", "", "custom headers, eg: key1=value1,key2=value2")
}

func (c *loginCookieCloudCmd) execute(_ctx context.Context, args []string) error {
	var headers = make(map[string]string)
	if c.headers != "" {
		for _, header := range strings.Split(c.headers, ",") {
			kv := strings.Split(header, "=")
			if len(kv) != 2 {
				return fmt.Errorf("invalid header format: %s", header)
			}
			headers[kv[0]] = kv[1]
		}
	}
	if c.server == "" {
		return fmt.Errorf("server is required")
	}
	if c.uuid == "" {
		return fmt.Errorf("uuid is required")
	}
	if c.password == "" {
		return fmt.Errorf("password is required")
	}

	ctx, cancel := context.WithTimeout(_ctx, c.timeout)
	defer cancel()

	cli, err := api.NewClient(c.root.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)

	var cfg = cookiecloud.Config{
		ApiUrl:  c.server,
		Timeout: c.timeout,
		Retry:   3,
		Debug:   c.root.root.Opts.Debug,
	}
	cc, err := cookiecloud.NewClient(&cfg)
	if err != nil {
		return fmt.Errorf("cookiecloud.NewClient: %w", err)
	}
	if len(headers) > 0 {
		cc.SetHeaders(headers)
	}
	resp, err := cc.Get(ctx, &cookiecloud.GetReq{Uuid: c.uuid})
	if err != nil {
		return fmt.Errorf("cookiecloud.Get: %w", err)
	}
	c.cmd.Printf("cookie最后更新时间为: %s", resp.UpdateTime)
	var cnt int
	for domain, cookies := range resp.CookieData {
		if !strings.HasSuffix(domain, "music.163.com") {
			continue
		}
		// Parse the domain into a URL (adjust a scheme if needed)
		u, err := url.Parse("https://music.163.com")
		if err != nil {
			return fmt.Errorf("failed to parse domain URL: %v", err)
		}

		// Convert a custom cookie type to http.Cookie
		var httpCookies []*http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "MUSIC_U" {
				cnt++
			}
			httpCookies = append(httpCookies, &http.Cookie{
				Domain:   domain, // Use original domain value
				Expires:  cookie.GetExpired(),
				HttpOnly: cookie.HttpOnly,
				Name:     cookie.Name,
				Path:     cookie.Path,
				Secure:   cookie.Secure,
				Value:    cookie.Value,
			})
		}
		if len(httpCookies) > 0 {
			cli.SetCookies(u, httpCookies)
		}
	}

	if cnt == 0 {
		return fmt.Errorf("请确认已登录网页版网易云音乐，并且cookie已经同步到cookiecloud")
	}

	// 查询登录信息是否成功
	request := weapi.New(cli)
	user, err := request.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		return fmt.Errorf("GetUserInfo: %s", err)
	}
	c.cmd.Printf("login success: %+v\n", user)
	return nil
}

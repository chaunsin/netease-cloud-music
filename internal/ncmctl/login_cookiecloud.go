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
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/spf13/cobra"
)

type loginCookieCloudCmd struct {
	root *Login
	cmd  *cobra.Command
	l    *log.Logger

	server   string // Server address
	uuid     string // User KEY · UUID
	password string // End-to-End Encryption Password

}

func cookieCloud(root *Login, l *log.Logger) *cobra.Command {
	c := &loginCookieCloudCmd{
		root: root,
		l:    l,
	}
	c.cmd = &cobra.Command{
		Use:     "cookiecloud",
		Short:   "use cookiecloud login, https://github.com/easychen/CookieCloud",
		Example: "  ncmctl login cookiecloud -u 'xxx' -p 'yyy'",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute(cmd.Context(), args)
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *loginCookieCloudCmd) addFlags() {
	c.cmd.Flags().StringVarP(&c.uuid, "uuid", "u", "", "user key (uuid)")
	c.cmd.Flags().StringVarP(&c.password, "password", "p", "", "end-to-end encryption password")
	c.cmd.Flags().StringVarP(&c.server, "server", "s", "http://localhost:3000/cookiecloud", "cookiecloud server address")
}

func (c *loginCookieCloudCmd) execute(ctx context.Context, _ []string) error {

	if c.uuid == "" {
		return fmt.Errorf("uuid is required")
	}
	if c.password == "" {
		return fmt.Errorf("password is required")
	}

	cli, err := api.NewClient(c.root.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)
	request := weapi.New(cli)

	cookieCli := &cookieCloudClient{
		server:   c.server,
		uuid:     c.uuid,
		password: c.password,
	}
	cookieData, err := cookieCli.DownloadCookieData()
	if err != nil {
		return fmt.Errorf("error while fetching and decrypting cookie: %s", err)
	}

	cnt := 0
	for domain, cookies := range cookieData {
		if !strings.HasSuffix(domain, "music.163.com") {
			continue
		}
		// Parse the domain into a URL (adjust scheme if needed)
		u, err := url.Parse("https://music.163.com")
		if err != nil {
			return fmt.Errorf("failed to parse domain URL: %v", err)
		}

		// Convert custom cookie type to http.Cookie
		var httpCookies []*http.Cookie
		for _, cookie := range cookies {
			secs := int64(cookie.ExpirationDate)
			nsecs := int64((cookie.ExpirationDate - float64(secs)) * 1e9)
			httpCookies = append(httpCookies, &http.Cookie{
				Domain:   domain, // Use original domain value
				Expires:  time.Unix(secs, nsecs),
				HttpOnly: cookie.HttpOnly,
				Name:     cookie.Name,
				Path:     cookie.Path,
				Secure:   cookie.Secure,
				Value:    cookie.Value,
			})
		}
		cli.SetCookies(u, httpCookies)
		cnt++
	}

	if cnt == 0 {
		return fmt.Errorf("no cookies found. 请确定你已经登录网页版网易云音乐，并且cookiecloud已经完成上传")
	}

	// 查询登录信息是否成功
	user, err := request.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		return fmt.Errorf("GetUserInfo: %s", err)
	}
	c.cmd.Printf("login success: %+v\n", user)
	return nil
}

package ncmctl

import (
	"context"
	"fmt"

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

func loginCookieCloud(root *Login, l *log.Logger) *cobra.Command {
	c := &loginCookieCloudCmd{
		root: root,
		l:    l,
	}
	c.cmd = &cobra.Command{
		Use:     "cookiecloud",
		Short:   "use cookiecloud login",
		Example: "  ncmctl login cookiecloud",
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

	cookieCloudRef := &cookieCloud{
		server:   c.server,
		uuid:     c.uuid,
		password: c.password,
	}
	cookie, err := cookieCloudRef.FetchCookie()
	if err != nil {
		return fmt.Errorf("error while fetching and decrypting cookie: %s", err)
	}
	fmt.Println("cookie:", string(cookie))
	// 查询登录信息是否成功
	user, err := request.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		return fmt.Errorf("GetUserInfo: %s", err)
	}
	c.cmd.Printf("login success: %+v\n", user)
	return nil
}

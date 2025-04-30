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
	"strconv"
	"time"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/spf13/cobra"
)

type loginPhoneCmd struct {
	root *Login
	cmd  *cobra.Command
	l    *log.Logger

	timeout     time.Duration // 登录超时时间
	countrycode int64
	password    string
}

func phone(root *Login, l *log.Logger) *cobra.Command {
	c := &loginPhoneCmd{
		root: root,
		l:    l,
	}
	c.cmd = &cobra.Command{
		Use:     "phone",
		Short:   "use phone login",
		Example: "  ncmctl login phone 188xxxx8888\n  ncmctl login phone 188xxxx8888 -p password",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute(cmd.Context(), args)
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *loginPhoneCmd) addFlags() {
	c.cmd.Flags().DurationVarP(&c.timeout, "timeout", "t", time.Minute*10, "login timeout, eg: 1s、1m")
	c.cmd.Flags().Int64Var(&c.countrycode, "countrycode", 86, "country code")
	c.cmd.Flags().StringVar(&c.password, "p", "", "use when logging in with a password.")
}

func (c *loginPhoneCmd) execute(ctx context.Context, args []string) error {
	if len(args) <= 0 {
		return fmt.Errorf("requrid phone number")
	}
	var cellphone = args[0]
	if len(cellphone) < 5 {
		return fmt.Errorf("phone number is too short")
	}
	if _, err := strconv.ParseInt(cellphone, 10, 64); err != nil {
		return fmt.Errorf("invalid phone number: %s", cellphone)
	}

	cli, err := api.NewClient(c.root.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)
	request := weapi.New(cli)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 如果密码为空则走短信验证登录逻辑
	var captcha string
	if c.password == "" {
		sms, err := request.SendSMS(ctx, &weapi.SendSMSReq{
			Cellphone: cellphone,
			CtCode:    c.countrycode,
		})
		if err != nil {
			return fmt.Errorf("SendSMS: %s", err)
		}
		if sms.Code == 200 && sms.Data {
			c.cmd.Println("send sms success")
		} else {
			return fmt.Errorf("send sms failed, code: %d, msg: %s\n", sms.Code, sms.Msg)
		}

		// 等待用户在终端输入验证码
		var fail int
	retry:
		if fail > 5 {
			return fmt.Errorf("too many failed attempts")
		}
		c.cmd.Printf("please input sms captcha: ")
		if _, err := fmt.Scanln(&captcha); err != nil {
			return fmt.Errorf("input sms captcha: %s", err)
		}
		if captcha == "" || len(captcha) < 4 {
			c.cmd.Println("invalid captcha, please retry")
			goto retry
		}

		verify, err := request.SMSVerify(ctx, &weapi.SMSVerifyReq{
			Cellphone: cellphone,
			Captcha:   captcha,
			CtCode:    c.countrycode,
		})
		if err != nil {
			return fmt.Errorf("SMSVerify: %s", err)
		}
		if verify.Code == 200 && verify.Data {
			c.cmd.Println("verify sms success")
		} else {
			fail++
			c.cmd.Printf("verify sms failed, code: %d, msg: %s\n", verify.Code, verify.Msg)
			goto retry
		}
	}

	login, err := request.LoginCellphone(ctx, &weapi.LoginCellphoneReq{
		Phone:       cellphone,
		Countrycode: c.countrycode,
		Remember:    true,
		Password:    c.password,
		Captcha:     captcha,
	})
	if err != nil {
		return fmt.Errorf("LoginCellphone: %s", err)
	}
	if login.Code != 200 {
		return fmt.Errorf("login failed, code: %d, msg: %s", login.Code, login.Msg)
	}

	// 查询登录信息是否成功
	user, err := request.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		return fmt.Errorf("GetUserInfo: %s", err)
	}
	c.cmd.Printf("login success: %+v\n", user)
	return nil
}

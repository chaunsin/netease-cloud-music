// Copyright (c) 2025-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
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
		Use:   "phone <number>",
		Short: "Log in by phone using SMS or a password",
		Long: "Log in with one phone number. Without --password, the command sends an SMS and " +
			"prompts for its captcha; with --password, it uses password login instead. Successful " +
			"login persists cookies. Password values may be visible in shell history and process lists.",
		Example: "  ncmctl login phone 18800008888\n" +
			"  ncmctl login phone --countrycode 86 18800008888\n" +
			"  ncmctl login phone 18800008888 --password '<password>'",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute(cmd.Context(), args)
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *loginPhoneCmd) addFlags() {
	c.cmd.Flags().DurationVarP(
		&c.timeout,
		"timeout",
		"t",
		time.Minute*10,
		"network request deadline; waiting for SMS input is not interrupted (for example 30s or 10m)",
	)
	c.cmd.Flags().Int64Var(&c.countrycode, "countrycode", 86, "telephone country calling code without '+'")
	c.cmd.Flags().StringVarP(
		&c.password,
		"password",
		"p",
		"",
		"use password login instead of SMS (visible in shell history and process lists)",
	)
}

func (c *loginPhoneCmd) execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("requrid phone number")
	}

	cellphone := args[0]
	if len(cellphone) < 5 {
		return errors.New("phone number is too short")
	}

	if _, err := strconv.ParseInt(cellphone, 10, 64); err != nil {
		return fmt.Errorf("invalid phone number: %s", cellphone)
	}

	cli, err := api.NewClient(c.root.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer closeAPIClient(ctx, cli)

	request := weapi.New(cli)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 如果密码为空则走短信验证登录逻辑
	var captcha string

	if c.password == "" {
		sms, smsErr := request.SendSMS(ctx, &weapi.SendSMSReq{
			Cellphone: cellphone,
			CtCode:    c.countrycode,
		})
		if smsErr != nil {
			return fmt.Errorf("SendSMS: %w", smsErr)
		}

		if sms.Code == 200 && sms.Data {
			c.cmd.Println("send sms success")
		} else {
			return fmt.Errorf("send sms failed, code: %d, msg: %s", sms.Code, sms.Msg)
		}

		// 等待用户在终端输入验证码
		var fail int

	retry:
		if fail > 5 {
			return errors.New("too many failed attempts")
		}

		c.cmd.Printf("please input sms captcha: ")

		if _, scanErr := fmt.Scanln(&captcha); scanErr != nil {
			return fmt.Errorf("input sms captcha: %w", scanErr)
		}

		if captcha == "" || len(captcha) < 4 {
			c.cmd.Println("invalid captcha, please retry")
			goto retry
		}

		verify, verifyErr := request.SMSVerify(ctx, &weapi.SMSVerifyReq{
			Cellphone: cellphone,
			Captcha:   captcha,
			CtCode:    c.countrycode,
		})
		if verifyErr != nil {
			return fmt.Errorf("SMSVerify: %w", verifyErr)
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
		return fmt.Errorf("LoginCellphone: %w", err)
	}

	if login.Code != 200 {
		return fmt.Errorf("login failed, code: %d, msg: %s, message: %s", login.Code, login.Msg, login.Message)
	}

	// 查询登录信息是否成功
	user, err := request.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		return fmt.Errorf("GetUserInfo: %w", err)
	}

	if err := validateLoginAccount(user); err != nil {
		return err
	}

	c.cmd.Printf("login success: %+v\n", user)
	return nil
}

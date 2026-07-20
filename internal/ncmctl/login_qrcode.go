// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	qrcode2 "github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type loginQrcodeCmd struct {
	root *Login
	cmd  *cobra.Command
	l    *log.Logger

	timeout time.Duration // 登录超时时间
	dir     string        // 二维码文件路径
	level   int           // 二维码恢复能力等级
}

func qrcode(root *Login, l *log.Logger) *cobra.Command {
	c := &loginQrcodeCmd{
		root: root,
		l:    l,
	}
	c.cmd = &cobra.Command{
		Use:   "qrcode",
		Short: "Log in by scanning a QR code",
		Long: "Generate qrcode.png and print the QR content in the terminal, then wait for " +
			"confirmation in the NetEase Cloud Music mobile app. Successful login persists cookies " +
			"and removes the image; a failed or cancelled attempt may leave the image on disk.",
		Example: "  ncmctl login qrcode\n" +
			"  ncmctl login qrcode --timeout 5m --dir ./private-qr",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute(cmd.Context(), args)
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *loginQrcodeCmd) addFlags() {
	c.cmd.Flags().DurationVarP(&c.timeout, "timeout", "t", time.Minute*5, "maximum time to wait for QR confirmation")
	c.cmd.Flags().StringVarP(&c.dir, "dir", "d", "", "directory for qrcode.png (default: current directory)")
	c.cmd.Flags().IntVarP(&c.level, "level", "l", 1, "QR error-correction level: 0=7%, 1=15%, 2=25%, 3=30%")
}

func (c *loginQrcodeCmd) execute(ctx context.Context, _ []string) error {
	if c.level < 0 || c.level > 3 {
		return errors.New("qrcode level must be 0-3")
	}

	cli, err := api.NewClient(c.root.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer closeAPIClient(ctx, cli)

	request := weapi.New(cli)

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 1. 生成key
	key, err := request.QrcodeCreateKey(ctx, &weapi.QrcodeCreateKeyReq{Type: 1})
	if err != nil {
		return fmt.Errorf("QrcodeCreateKey: %w", err)
	}

	if key.UniKey == "" {
		return fmt.Errorf("QrcodeCreateKey resp: %+v", key)
	}

	// 2. 生成二维码
	qr, err := request.QrcodeGenerate(ctx, &weapi.QrcodeGenerateReq{
		CodeKey:  key.UniKey,
		Level:    qrcode2.RecoveryLevel(c.level),
		Platform: "web",
		DeviceId: "",
	})
	if err != nil {
		return fmt.Errorf("QrcodeGenerate: %w", err)
	}

	// 3. 手机扫码
	if c.dir == "" {
		dir, getwdErr := os.Getwd()
		if getwdErr != nil {
			return getwdErr
		}

		c.dir = dir
	}

	if mkdirErr := os.MkdirAll(c.dir, 0o700); mkdirErr != nil {
		return fmt.Errorf("MkdirAll: %w", mkdirErr)
	}

	file := filepath.Join(c.dir, "qrcode.png")
	if writeErr := os.WriteFile(file, qr.Qrcode, 0o600); writeErr != nil {
		return writeErr
	}

	c.cmd.Println(">>>>> please scan qrcode in your phone <<<<<")
	c.cmd.Printf("qrcode content: https://music.163.com/login?codekey=%s\n", key.UniKey)
	c.cmd.Printf("qrcode file: %s\n", file)
	c.cmd.Printf("qrcode: \n%s\n", qr.QrcodePrint)

	// 4. 轮训获取扫码状态
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		time.Sleep(time.Second * 3)

		resp, checkErr := request.QrcodeCheck(ctx, &weapi.QrcodeCheckReq{Type: 1, Key: key.UniKey})
		if checkErr != nil {
			return fmt.Errorf("QrcodeCheck: %w", checkErr)
		}

		log.Debugf("QrcodeCheck code=%d", resp.Code)

		switch resp.Code {
		case 800: // 二维码不存在、已过期、用户取消授权
			return fmt.Errorf("current QrcodeCheck resp: %v", resp)
		case 801: // 等待扫码
			continue
		case 802: // 正在扫码授权中
			continue
		case 803: // 授权登录成功
			goto ok
		default:
			return fmt.Errorf("登录失败 QrcodeCheck resp: %v", resp)
		}
	}

ok:

	if removeErr := os.Remove(file); removeErr != nil {
		if !os.IsNotExist(removeErr) {
			log.Infof("remove qrcode file: %s", removeErr)
		}
	}

	// 5. 查询登录信息是否成功
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

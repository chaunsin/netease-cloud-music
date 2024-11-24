package example

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/skip2/go-qrcode"
)

// 二维码登录
func TestWeapiLoginByQrcode(t *testing.T) {
	t.Logf("start login")

	api := weapi.New(cli)

	// 1. 生成key
	key, err := api.QrcodeCreateKey(ctx, &weapi.QrcodeCreateKeyReq{Type: 1})
	if err != nil {
		t.Fatalf("QrcodeCreateKey: %s", err)
	}
	if key.UniKey == "" {
		t.Fatalf("QrcodeCreateKey resp: %+v\n", key)
	}

	// 2. 生成二维码
	qr, err := api.QrcodeGenerate(ctx, &weapi.QrcodeGenerateReq{CodeKey: key.UniKey, Level: qrcode.Medium})
	if err != nil {
		t.Fatalf("QrcodeGenerate: %s", err)
	}

	// 3. 手机扫码
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %s", err)
	}
	p := filepath.Join(dir, "./")
	if err := os.MkdirAll(p, os.ModePerm); err != nil {
		t.Fatalf("Mkdir: %s", err)
	}
	p = filepath.Join(p, "qrcode.png")
	if err := os.WriteFile(p, qr.Qrcode, os.ModePerm); err != nil {
		t.Fatalf("WriteFile: %s", err)
	}
	fmt.Printf(">>>>> please scan qrcode in your phone <<<<<\n")
	fmt.Printf("qrcode file %s\n", p)
	fmt.Printf("qrcode: \n%s\n", qr.QrcodePrint)

	// 4. 轮训获取扫码状态
	for {
		time.Sleep(time.Second * 3)
		resp, err := api.QrcodeCheck(ctx, &weapi.QrcodeCheckReq{Type: 1, Key: key.UniKey})
		if err != nil {
			t.Fatalf("QrcodeCheck: %s", err)
		}
		switch resp.Code {
		case 800: // 二维码不存在或已过期
			t.Fatalf("current QrcodeCheck resp: %v\n", resp)
		case 801: // 等待扫码
			continue
		case 802: // 正在扫码授权中
			continue
		case 803: // 授权登录成功
			t.Logf("current QrcodeCheck resp: %v\n", resp)
			goto ok
		default:
			t.Logf("current QrcodeCheck resp: %v\n", resp)
		}
	}
ok:

	// 5. 查询登录信息是否成功
	user, err := api.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		t.Fatalf("GetUserInfo: %s", err)
	}
	t.Logf("login sussess: %+v\n", user)
}

// 二维码登录
func TestEapiLoginByQrcode(t *testing.T) {
	t.Logf("start login")

	api := eapi.New(cli)

	// 1. 生成key
	key, err := api.QrcodeCreateKey(ctx, &eapi.QrcodeCreateKeyReq{Type: 3})
	if err != nil {
		t.Fatalf("QrcodeCreateKey: %s", err)
	}
	if key.UniKey == "" {
		t.Fatalf("QrcodeCreateKey resp: %+v\n", key)
	}

	// 2. 生成二维码
	qr, err := api.QrcodeGenerate(ctx, &eapi.QrcodeGenerateReq{CodeKey: key.UniKey})
	if err != nil {
		t.Fatalf("QrcodeGenerate: %s", err)
	}

	// 3. 手机扫码
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %s", err)
	}
	p := filepath.Join(dir, "./")
	if err := os.MkdirAll(p, os.ModePerm); err != nil {
		t.Fatalf("Mkdir: %s", err)
	}
	p = filepath.Join(p, "qrcode.png")
	if err := os.WriteFile(p, qr.Qrcode, os.ModePerm); err != nil {
		t.Fatalf("WriteFile: %s", err)
	}
	fmt.Printf(">>>>> please scan qrcode in your phone <<<<<\n")
	fmt.Printf("qrcode file %s\n", p)
	fmt.Printf("qrcode: \n%s\n", qr.QrcodePrint)

	// 4. 轮训获取扫码状态
	for {
		time.Sleep(time.Second * 3)
		resp, err := api.QrcodeCheck(ctx, &eapi.QrcodeCheckReq{Type: 3, Key: key.UniKey})
		if err != nil {
			t.Fatalf("QrcodeCheck: %s", err)
		}
		switch resp.Code {
		case 800: // 二维码不存在或已过期
			t.Fatalf("current QrcodeCheck resp: %v\n", resp)
		case 801: // 等待扫码
			continue
		case 802: // 正在扫码授权中
			continue
		case 803: // 授权登录成功
			t.Logf("current QrcodeCheck resp: %v\n", resp)
			goto ok
		default:
			t.Logf("current QrcodeCheck resp: %v\n", resp)
		}
	}
ok:

	// 5. 查询登录信息是否成功
	user, err := api.GetUserInfo(ctx, &eapi.GetUserInfoReq{})
	if err != nil {
		t.Fatalf("GetUserInfo: %s", err)
	}
	t.Logf("login sussess: %+v\n", user)
}

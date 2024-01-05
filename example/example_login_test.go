package example

import (
	"testing"
	"time"

	"github.com/chaunsin/netease-cloud-music/api/weapi"
)

// 二维码登录
func TestLoginByQrcode(t *testing.T) {
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
	_, err = api.QrcodeGenerate(ctx, &weapi.QrcodeGenerateReq{CodeKey: key.UniKey})
	if err != nil {
		t.Fatalf("QrcodeGetReq: %s", err)
	}

	// 3. 手机扫码

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

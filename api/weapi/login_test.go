// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"testing"

	"github.com/skip2/go-qrcode"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api/types"
)

func TestQrcodeCreateKey(t *testing.T) {
	cli := newLiveWEAPI(t)

	req := QrcodeCreateKeyReq{
		ReqCommon: types.ReqCommon{CSRFToken: ""}, // 可不传
		Type:      1,
	}
	got, err := cli.QrcodeCreateKey(t.Context(), &req)
	require.NoError(t, err)
	t.Logf("QrcodeCreateKey: %+v\n", got)
}

func TestQrcodeGetReq(t *testing.T) {
	cli := newOfflineWEAPI(t)
	req := QrcodeGenerateReq{
		CodeKey:  "",
		Level:    qrcode.Medium,
		Platform: "web",
	}
	got, err := cli.QrcodeGenerate(t.Context(), &req)
	require.NoError(t, err)
	t.Logf("QrcodeGenerate: %+v\n", got)
}

func TestQrcodeCheck(t *testing.T) {
	cli := newLiveWEAPI(t)

	req := QrcodeCheckReq{
		Key:  "8ddf7539-2b30-4350-962e-b8045762164b",
		Type: 1,
	}
	got, err := cli.QrcodeCheck(t.Context(), &req)
	require.NoError(t, err)
	t.Logf("QrcodeCheck: %+v\n", got)
}

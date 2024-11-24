// MIT License
//
// Copyright (c) 2024 chaunsin
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

package weapi

import (
	"testing"

	"github.com/chaunsin/netease-cloud-music/api/types"

	"github.com/skip2/go-qrcode"
	"github.com/stretchr/testify/assert"
)

func TestQrcodeCreateKey(t *testing.T) {
	var req = QrcodeCreateKeyReq{
		ReqCommon: types.ReqCommon{CSRFToken: ""}, // 可不传
		Type:      1,
	}
	got, err := cli.QrcodeCreateKey(ctx, &req)
	assert.NoError(t, err)
	t.Logf("QrcodeCreateKey: %+v\n", got)
}

func TestQrcodeGetReq(t *testing.T) {
	var req = QrcodeGenerateReq{
		CodeKey: "",
		Level:   qrcode.Medium,
	}
	got, err := cli.QrcodeGenerate(ctx, &req)
	assert.NoError(t, err)
	t.Logf("QrcodeGenerate: %+v\n", got)
}

func TestQrcodeCheck(t *testing.T) {
	var req = QrcodeCheckReq{
		Key:  "8ddf7539-2b30-4350-962e-b8045762164b",
		Type: 1,
	}
	got, err := cli.QrcodeCheck(ctx, &req)
	assert.NoError(t, err)
	t.Logf("QrcodeCheck: %+v\n", got)
}

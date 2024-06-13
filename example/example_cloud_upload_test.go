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

package example

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/chaunsin/netease-cloud-music/api/weapi"
)

func TestCloudUpload(t *testing.T) {
	api := weapi.New(cli)

	// 1.读取文件
	var filename = "../testdata/窦唯 - 童嫄世界 (选段 郊外).flac"
	file, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	m := md5.New()
	_, err = io.Copy(m, file)
	if err != nil {
		t.Fatalf("copy file error: %v", err)
	}
	md5Sum := hex.EncodeToString(m.Sum(nil))

	// 2.检查此文件是否需要上传
	var checkReq = weapi.CloudUploadCheckReq{
		Bitrate: "999000",
		Ext:     "",
		Length:  fmt.Sprintf("%d", stat.Size()),
		Md5:     md5Sum,
		SongId:  "0",
		Version: "1",
	}
	resp, err := api.CloudUploadCheck(ctx, &checkReq)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("check resp: %+v\n", resp)

	if resp.NeedUpload {
		// 3.获取上传凭证
		var allocReq = weapi.CloudTokenAllocReq{
			Bucket:     "",
			Ext:        "mp3",
			Filename:   filepath.Base(filename),
			Local:      "false",
			NosProduct: "3",
			Type:       "audio",
			Md5:        md5Sum,
		}
		allocResp, err := api.CloudTokenAlloc(ctx, &allocReq)
		if err != nil {
			t.Fatalf("CloudTokenAlloc: %v", err)
		}
		t.Logf("CloudTokenAlloc resp: %+v\n", allocResp)

		// 4.上传文件
	}

	// 5.上传封面
}

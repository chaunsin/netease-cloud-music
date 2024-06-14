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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

func TestCloudUpload(t *testing.T) {
	api := weapi.New(cli)

	// 1.读取文件
	var (
		filename = "../testdata/music/record3.m4a"
		ext      = "mp3"
	)

	file, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	md5, err := utils.MD5Hex(data)
	if err != nil {
		t.Fatalf("MD5Hex: %v", err)
	}

	// 2.检查是否需要登录
	if api.NeedLogin(ctx) {
		t.Fatal("need login")
	}

	// 3.检查此文件是否需要上传
	var checkReq = weapi.CloudUploadCheckReq{
		Bitrate: "128000",
		Ext:     ext,
		Length:  fmt.Sprintf("%d", stat.Size()),
		Md5:     md5,
		SongId:  "0",
		Version: "1",
	}
	resp, err := api.CloudUploadCheck(ctx, &checkReq)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("check resp: %+v\n", resp)

	// 4.获取上传凭证
	var allocReq = weapi.CloudTokenAllocReq{
		Bucket:     "", // jd-musicrep-privatecloud-audio-public
		Ext:        ext,
		Filename:   filepath.Base(filename),
		Local:      "false",
		NosProduct: "3",
		Type:       "audio",
		Md5:        md5,
	}
	allocResp, err := api.CloudTokenAlloc(ctx, &allocReq)
	if err != nil {
		t.Fatalf("CloudTokenAlloc: %v", err)
	}
	t.Logf("CloudTokenAlloc resp: %+v\n", allocResp)

	if resp.NeedUpload {
		var allocReq2 = weapi.CloudTokenAllocReq{
			Bucket:     "jd-musicrep-privatecloud-audio-public",
			Ext:        ext,
			Filename:   filepath.Base(filename),
			Local:      "false",
			NosProduct: "3",
			Type:       "audio",
			Md5:        md5,
		}
		allocResp2, err := api.CloudTokenAlloc(ctx, &allocReq2)
		if err != nil {
			t.Fatalf("CloudTokenAlloc: %v", err)
		}
		t.Logf("CloudTokenAlloc resp2: %+v\n", allocResp2)

		// 5.上传文件
		var uploadReq = weapi.CloudUploadReq{
			Bucket:    allocResp.Bucket,
			ObjectKey: allocResp.ObjectKey,
			Token:     allocResp.Token,
			Filepath:  filename,
		}
		uploadResp, err := api.CloudUpload(ctx, &uploadReq)
		if err != nil {
			t.Fatalf("CloudUpload: %v", err)
		}
		t.Logf("CloudUpload resp: %+v\n", uploadResp)
	}

	// todo: 解决接口返回400问题
	// 6.上传歌曲相关信息
	var InfoReq = weapi.CloudInfoReq{
		Md5:        md5,
		SongId:     resp.SongId,
		Filename:   stat.Name(),
		Song:       "record3",
		Album:      "未知专辑",
		Artist:     "未知艺术家",
		Bitrate:    "128000",
		ResourceId: allocResp.ResourceID,
	}
	infoResp, err := api.CloudInfo(ctx, &InfoReq)
	if err != nil {
		t.Fatalf("CloudInfo: %v", err)
	}
	t.Logf("CloudInfo resp: %+v\n", infoResp)

	// // 7.对上传得歌曲进行发布，和自己账户做关联,不然云盘列表看不到上传得歌曲信息
	// var publishReq = weapi.CloudPublishReq{
	// 	SongId: infoResp.SongId,
	// }
	// publishResp, err := api.CloudPublish(ctx, &publishReq)
	// if err != nil {
	// 	t.Fatalf("CloudPublish: %v", err)
	// }
	// t.Logf("CloudPublish resp: %+v\n", publishResp)

	t.Log("upload success!!!")
}

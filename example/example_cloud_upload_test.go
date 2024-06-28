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

	"github.com/dhowden/tag"
)

func TestCloudUpload(t *testing.T) {
	api := weapi.New(cli)

	// 1.读取文件
	var (
		filename = "../testdata/music/本兮 - 逢场作戏.mp3"
		ext      = "mp3"
		bitrate  = "999000"
		// bitrate = "128000"
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

	// 重新设置文件指针到开头
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek: %v", err)
	}

	// 2.检查是否需要登录
	if api.NeedLogin(ctx) {
		t.Fatal("need login")
	}

	// 3.检查此文件是否需要上传
	var checkReq = weapi.CloudUploadCheckReq{
		Bitrate: bitrate,
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
	t.Logf("CloudUploadCheck resp: %+v\n", resp)
	if resp.Code != 200 {
		t.Logf("CloudUploadCheck resp: %+v\n", resp)
	}

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
	if allocResp.Code != 200 {
		t.Logf("CloudTokenAlloc resp: %+v\n", allocResp)
	}

	// 5.上传文件
	if resp.NeedUpload {
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
		if uploadResp.ErrCode != "0" {
			t.Logf("CloudUpload resp: %+v\n", uploadResp)
		}
	}

	// 6.上传歌曲相关信息
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	var InfoReq = weapi.CloudInfoReq{
		Md5:        md5,
		SongId:     resp.SongId,
		Filename:   stat.Name(),
		Song:       utils.Ternary(metadata.Title() != "", metadata.Title(), filepath.Base(filename)),
		Album:      utils.Ternary(metadata.Album() != "", metadata.Album(), "未知专辑"),
		Artist:     utils.Ternary(metadata.Artist() != "", metadata.Artist(), "未知艺术家"),
		Bitrate:    bitrate,
		ResourceId: allocResp.ResourceID,
	}
	infoResp, err := api.CloudInfo(ctx, &InfoReq)
	if err != nil {
		t.Fatalf("CloudInfo: %v", err)
	}
	t.Logf("CloudInfo resp: %+v\n", infoResp)
	if infoResp.Code != 200 {
		t.Fatalf("CloudInfo: %v", infoResp)
	}

	// 7.对上传得歌曲进行发布，和自己账户做关联,不然云盘列表看不到上传得歌曲信息
	var publishReq = weapi.CloudPublishReq{
		SongId: infoResp.SongId,
	}
	publishResp, err := api.CloudPublish(ctx, &publishReq)
	if err != nil {
		t.Fatalf("CloudPublish: %v", err)
	}
	t.Logf("CloudPublish resp: %+v\n", publishResp)
	switch publishResp.Code {
	case 200:
		t.Logf("上传成功: %s", filename)
	case 201:
		t.Logf("重复上传: %s", filename)
	default:
		t.Fatalf("上传失败: %s: %+v", filename, publishResp)
	}
}

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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

// TestDownloadMusic 音乐下载.执行之前需要执行一次登录example_login_test.go
func TestDownload(t *testing.T) {
	var (
		// 歌曲id
		songId    = 2161154646
		songIdStr = fmt.Sprintf("%d", songId)
		// 音质
		level = types.LevelLossless
		// 严格模式,如果开启则检查音乐是否有对应得level,有则下载没有则不下载
		strict = true
		// 下载到目录
		output = "../testdata/"
		// api请求对象
		request = weapi.New(cli)
	)

	if cli.NeedLogin(ctx) {
		t.Fatal("need login")
	}

	// var playReq = &weapi.SongPlayerReqV1{
	// 	Ids:         []int64{songId},
	// 	Level:       level,
	// 	EncodeType:  "mp3",
	// 	ImmerseType: "",
	// }
	// playResp, err := request.SongPlayerV1(ctx, playReq)
	// if err != nil {
	// 	t.Fatalf("SongPlayerV1(%v): %s", songId, err)
	// }
	// if playResp.Code != 200 {
	// 	t.Fatalf("SongPlayerV1(%v) err: %+v", songId, playResp)
	// }
	// if len(playResp.Data) <= 0 {
	// 	t.Fatalf("SongPlayerV1(%v) data is empty", songId)
	// }
	// var songDetail = playResp.Data[0]
	// if songDetail.Level != string(level) {
	// 	t.Logf("id=%v 没有找到%v品质的资源,当前品质为%v\n", songId, types.LevelString[level], types.LevelString[level])
	// }

	var detailReq = &weapi.SongDetailReq{
		C: []weapi.SongDetailReqList{{
			Id: songIdStr,
			V:  0,
		}},
	}
	detail, err := request.SongDetail(ctx, detailReq)
	if err != nil {
		t.Fatalf("SongDetail(%v): %s", songId, err)
	}
	if detail.Code != 200 {
		t.Fatalf("SongDetail(%v) err: %+v", songId, detail)
	}
	if len(detail.Songs) <= 0 {
		t.Fatalf("SongDetail(%v) data is empty", songId)
	}
	var songDetail = detail.Songs[0]

	// 查询音乐支持哪些音质
	qualityResp, err := request.SongMusicQuality(ctx, &weapi.SongMusicQualityReq{SongId: songIdStr})
	if err != nil {
		t.Fatalf("SongMusicQuality(%v): %s", songId, err)
	}
	if qualityResp.Code != 200 {
		t.Fatalf("SongMusicQuality(%v) err: %+v", songId, qualityResp)
	}
	quality, lv, ok := qualityResp.Data.Qualities.FindBetter(level)
	t.Logf("SongMusicQuality(%v) quality level=%s info=%+v\n", songId, types.LevelString[lv], quality)
	if !ok && strict {
		t.Fatalf("SongMusicQuality(%v) not support %v", songId, lv)
	}

	// 获取下载链接地址
	var downReq = &weapi.SongDownloadUrlReq{
		Id: songIdStr,
		Br: fmt.Sprintf("%d", quality.Br),
	}
	downResp, err := request.SongDownloadUrl(ctx, downReq)
	if err != nil {
		t.Fatalf("SongDownloadUrl(%v): %s", songId, err)
	}
	if downResp.Code != 200 {
		t.Fatalf("SongDownloadUrl(%v) err: %+v", songId, downResp)
	}
	// 歌曲变灰则不能下载
	if downResp.Data.Code != 200 || downResp.Data.Url == "" {
		t.Fatalf("资源已下架或无版权(%v) code: %v", songId, downResp.Data.Code)
	}

	var artistList = make([]string, 0, len(songDetail.Ar))
	for _, ar := range songDetail.Ar {
		artistList = append(artistList, strings.TrimSpace(ar.Name))
	}
	var (
		drd      = downResp.Data
		artist   = strings.Join(artistList, ",")
		dest     = filepath.Join(output, fmt.Sprintf("%s - %s.%s", artist, songDetail.Name, drd.Type))
		tmpDir   = os.TempDir()
		tempName = fmt.Sprintf("ncmctl-*-%s.tmp", songDetail.Name)
	)
	t.Logf("id=%v downloadUrl=%v outDir=%s tempDir=%s%s br=%v encodeType=%v type=%v\n",
		drd.Id, drd.Url, dest, tmpDir, tempName, drd.Br, drd.EncodeType, drd.Type)

	// 创建临时文件以及下载目录
	if err := utils.MkdirIfNotExist(output, 0755); err != nil {
		t.Fatalf("MkdirIfNotExist: %s", err)
	}
	file, err := os.CreateTemp(tmpDir, tempName)
	if err != nil {
		t.Fatalf("CreateTemp: %s", err)
	}
	defer file.Close()

	// 下载
	resp, err := cli.Download(ctx, drd.Url, nil, nil, file, nil)
	if err != nil {
		t.Fatalf("download: %s", err)
	}
	_ = resp
	// dump, err := httputil.DumpResponse(resp, false)
	// if err != nil {
	// 	t.Logf("DumpResponse err: %s\n", err)
	// }
	// t.Logf("Download DumpResponse: %s", dump)

	// 避免文件重名
	for i := 1; utils.FileExists(dest); i++ {
		dest = filepath.Join(output, fmt.Sprintf("%s - %s(%d).%s", artist, songDetail.Name, i, drd.Type))
	}
	if err := os.Rename(file.Name(), dest); err != nil {
		t.Fatalf("rename: %s", err)
	}
	if err := os.Chmod(dest, 0644); err != nil {
		t.Fatalf("chmod: %s", err)
	}
	t.Logf("download success: %s\n", dest)
}

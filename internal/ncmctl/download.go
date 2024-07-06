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

package ncmctl

import (
	"context"
	"fmt"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/spf13/cobra"
)

type DownloadOpts struct {
	Input    string // 可以是歌曲id、歌单id、专辑、歌手(https://music.163.com/#/discover/artist)
	Output   string // 输出目录
	Parallel int64  // 并发下载数量
	Level    string // 歌曲品质 types.Level
	Strict   bool   // 严格模式。当开起时指定的歌曲品质不符合要求,则不进行下载
	Tag      bool
	Replace  bool
}

type Download struct {
	root *Root
	cmd  *cobra.Command
	opts DownloadOpts
	l    *log.Logger
}

func NewDownload(root *Root, l *log.Logger) *Download {
	c := &Download{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "download",
			Short:   "[need login] Download songs",
			Example: `  ncmctl download`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}
	return c
}

func (c *Download) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Input, "input", "i", "", "music file path")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "./download", "music file output path")
	c.cmd.PersistentFlags().Int64VarP(&c.opts.Parallel, "parallel", "p", 5, "concurrent upload count")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Level, "level", "l", string(types.LevelLossless), "num of songs")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Strict, "strict", false, "strict mode. When the downloaded song does not find the corresponding quality, it will not be downloaded.")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Tag, "tag", true, "whether to set song tag information,default set")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Replace, "replace", true, "whether replace exist music")
}

func (c *Download) validate() error {
	if c.opts.Parallel <= 0 || c.opts.Parallel > 10 {
		return fmt.Errorf("parallel <= 0 or > 10")
	}
	switch types.Level(c.opts.Level) {
	case "":
		return fmt.Errorf("quality is empty")
	case types.LevelStandard,
		types.LevelHigher,
		types.LevelExhigh,
		types.LevelLossless,
		types.LevelHires,
		types.LevelJyeffect,
		types.LevelSky,
		types.LevelJymaster:
	default:
		return fmt.Errorf("[%s] quality is not support", c.opts.Level)
	}
	return nil
}

func (c *Download) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Download) Command() *cobra.Command {
	return c.cmd
}

func (c *Download) execute(ctx context.Context, args []string) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	// var bar = pb.Full.Start64(1)

	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)
	request := weapi.New(cli)

	// 判断是否需要登录
	if cli.NeedLogin(ctx) {
		return fmt.Errorf("need login")
	}

	// 支持交互式输入搜索歌名、歌手、专辑、歌单进行下载

	// // 解析输入入得内容是什么类型
	// kind, id, err := ParseUrl(c.opts.Input)
	// if err != nil {
	// 	return fmt.Errorf("ParseUrl: %w", err)
	// }

	var songIdStr = c.opts.Input
	// 查询歌曲相关信息,下载地址等
	songId, err := strconv.ParseInt(c.opts.Input, 10, 64)
	if err != nil {
		return err
	}

	// var playReq = &weapi.SongPlayerReqV1{
	// 	Ids:         []int64{songId},
	// 	Level:       types.Level(c.opts.Level),
	// 	EncodeType:  "mp3",
	// 	ImmerseType: "",
	// }
	// playResp, err := request.SongPlayerV1(ctx, playReq)
	// if err != nil {
	// 	return fmt.Errorf("SongPlayerV1(%v): %w", id, err)
	// }
	// if playResp.Code != 200 {
	// 	return fmt.Errorf("SongPlayerV1(%v) err: %+v", id, playResp)
	// }
	// if len(playResp.Data) <= 0 {
	// 	return fmt.Errorf("SongPlayerV1(%v) data is empty", id)
	// }
	// var songDetail = playResp.Data[0]
	// if songDetail.Level != c.opts.Level {
	// 	log.Warn("id=%v 没有找到%v品质的资源,当前品质为%v",
	// 		id, types.LevelString[types.Level(c.opts.Level)],
	// 		types.LevelString[types.Level(songDetail.Level)])
	// }

	var detailReq = &weapi.SongDetailReq{
		C: []weapi.SongDetailReqList{{
			Id: songIdStr,
			V:  0,
		}},
	}
	detail, err := request.SongDetail(ctx, detailReq)
	if err != nil {
		return fmt.Errorf("SongDetail(%v): %w", songId, err)
	}
	if detail.Code != 200 {
		return fmt.Errorf("SongDetail(%v) err: %+v", songId, detail)
	}
	if len(detail.Songs) <= 0 {
		return fmt.Errorf("SongDetail(%v) data is empty", songId)
	}
	var songDetail = detail.Songs[0]

	// 查询音乐支持哪些音质
	qualityResp, err := request.SongMusicQuality(ctx, &weapi.SongMusicQualityReq{SongId: songIdStr})
	if err != nil {
		return fmt.Errorf("SongMusicQuality(%v): %w", songId, err)
	}
	if qualityResp.Code != 200 {
		return fmt.Errorf("SongMusicQuality(%v) err: %+v", songId, qualityResp)
	}
	quality, level, ok := qualityResp.Data.Qualities.FindBetter(types.Level(c.opts.Level))
	log.Debug("SongMusicQuality(%v) quality level=%s info=%+v", songId, types.LevelString[level], quality)
	if !ok && c.opts.Strict {
		return fmt.Errorf("SongMusicQuality(%v) not support %v", songId, types.Level(c.opts.Level))
	}

	// 构造下载列表

	// 获取下载链接地址
	var downReq = &weapi.SongDownloadUrlReq{
		Id: songIdStr,
		Br: fmt.Sprintf("%d", quality.Br),
	}
	downResp, err := request.SongDownloadUrl(ctx, downReq)
	if err != nil {
		return fmt.Errorf("SongDownloadUrl(%v): %w", songId, err)
	}
	if downResp.Code != 200 {
		return fmt.Errorf("SongDownloadUrl(%v) err: %+v", songId, downResp)
	}
	if downResp.Data.Url == "" {
		return fmt.Errorf("SongDownloadUrl(%v) url is empty", songId)
	}

	var artistList = make([]string, 0, len(songDetail.Ar))
	for _, ar := range songDetail.Ar {
		artistList = append(artistList, strings.TrimSpace(ar.Name))
	}
	var (
		drd      = downResp.Data
		artist   = strings.Join(artistList, ",")
		dest     = filepath.Join(c.opts.Output, fmt.Sprintf("%s - %s.%s", artist, songDetail.Name, drd.Type))
		tmpDir   = os.TempDir()
		tempName = fmt.Sprintf("ncmctl-*-%s.tmp", songDetail.Name)
	)
	log.Debug("id=%v downloadUrl=%v outDir=%s tempDir=%s%s br=%v encodeType=%v type=%v",
		drd.Id, drd.Url, dest, tmpDir, tempName, drd.Br, drd.EncodeType, drd.Type)

	// 创建临时文件以及下载目录
	if err := utils.MkdirIfNotExist(c.opts.Output, 0755); err != nil {
		return fmt.Errorf("MkdirIfNotExist: %w", err)
	}
	file, err := os.CreateTemp(tmpDir, tempName)
	if err != nil {
		return fmt.Errorf("CreateTemp: %w", err)
	}
	defer file.Close()

	// 下载
	resp, err := cli.Download(ctx, drd.Url, nil, nil, file, nil)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if c.root.Opts.Debug {
		dump, err := httputil.DumpResponse(resp, false)
		if err != nil {
			log.Debug("DumpResponse err: %s", err)
		}
		log.Debug("Download DumpResponse: %s", dump)
	}

	// 设置歌曲tag值
	if c.opts.Tag {
		// todo:
	}

	// TODO: 处理是否覆盖文件

	for i := 1; utils.FileExists(dest); i++ {
		dest = filepath.Join(c.opts.Output, fmt.Sprintf("%s - %s(%d).%s", artist, songDetail.Name, i, drd.Type))
	}
	if err := os.Rename(file.Name(), dest); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	if err := os.Chmod(dest, 0644); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	c.cmd.Printf("download success\n")
	return nil
}

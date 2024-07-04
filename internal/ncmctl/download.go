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
	"strconv"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"

	"github.com/spf13/cobra"
)

type DownloadOpts struct {
	Input    string // 可以是歌曲id、歌单id、专辑、歌手(https://music.163.com/#/discover/artist)
	Parallel int64  // 并发下载数量
	Level    string // 歌曲品质 types.Level
	Strict   bool   // 严格模式。当开起时指定的歌曲品质不符合要求,则不进行下载
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
	c.cmd.PersistentFlags().Int64VarP(&c.opts.Parallel, "parallel", "p", 5, "concurrent upload count")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Level, "level", "l", string(types.LevelLossless), "num of songs")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Strict, "strict", false, "strict mode. When the downloaded song does not find the corresponding quality, it will not be downloaded.")
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

	// var bar = pb.Full.Start64(c.opts.Num)

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

	var total int
	defer func() {
		log.Debug("download success: %d", total)
		// bar.Finish()
	}()

	_ = request

	// 支持交互式输入搜索歌名、歌手、专辑、歌单进行下载

	// // 解析输入入得内容是什么类型
	// kind, id, err := ParseUrl(c.opts.Input)
	// if err != nil {
	// 	return fmt.Errorf("ParseUrl: %w", err)
	// }

	// 查询歌曲相关信息,下载地址等
	id, err := strconv.ParseInt(c.opts.Input, 10, 64)
	if err != nil {
		return err
	}
	var playReq = &weapi.SongPlayerReqV1{
		Ids:         []int64{id},
		Level:       types.Level(c.opts.Level),
		EncodeType:  "mp3",
		ImmerseType: "",
	}
	playResp, err := request.SongPlayerV1(ctx, playReq)
	if err != nil {
		return fmt.Errorf("SongPlayerV1(%v): %w", id, err)
	}
	if playResp.Code != 200 {
		return fmt.Errorf("SongPlayerV1(%v) err: %+v", id, playResp)
	}
	if len(playResp.Data) <= 0 {
		return fmt.Errorf("SongPlayerV1(%v) data is empty", id)
	}
	var songDetail = playResp.Data[0]
	if songDetail.Level != c.opts.Level {
		log.Warn("id=%v 没有找到%v品质的资源,当前品质为%v",
			id, types.LevelString[types.Level(c.opts.Level)],
			types.LevelString[types.Level(songDetail.Level)])
	}

	// 查询音乐支持哪些音质
	qualityResp, err := request.SongMusicQuality(ctx, &weapi.SongMusicQualityReq{SongId: fmt.Sprintf("%d", id)})
	if err != nil {
		return fmt.Errorf("SongMusicQuality(%v): %w", id, err)
	}
	if qualityResp.Code != 200 {
		return fmt.Errorf("SongMusicQuality(%v) err: %+v", id, qualityResp)
	}

	// 构造下载列表

	// 下载
	var downReq = &weapi.SongDownloadUrlReq{
		Id: fmt.Sprintf("%d", id),
		Br: fmt.Sprintf("%d", songDetail.Br),
	}
	downResp, err := request.SongDownloadUrl(ctx, downReq)
	if err != nil {
		return fmt.Errorf("SongDownloadUrl(%v): %w", id, err)
	}
	if downResp.Code != 200 {
		return fmt.Errorf("SongDownloadUrl(%v) err: %+v", id, downResp)
	}
	if downResp.Data.Url == "" {
		return fmt.Errorf("SongDownloadUrl(%v) url is empty", id)
	}
	var drd = downResp.Data
	log.Debug("SongDownloadUrl id=%v url=%v br=%v encodeType=%v type=%v", drd.Id, drd.Url, drd.Br, drd.EncodeType, drd.Type)

	// 设置歌曲tag值

	return nil
}

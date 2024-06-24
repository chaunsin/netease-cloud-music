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
	"errors"
	"fmt"
	"time"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/cheggaaa/pb/v3"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

type ScrobbleOpts struct {
	Crontab    string
	Once       bool
	PlaylistId string
}

type Scrobble struct {
	root *Root
	cmd  *cobra.Command
	opts ScrobbleOpts
	l    *log.Logger
}

func NewScrobble(root *Root, l *log.Logger) *Scrobble {
	c := &Scrobble{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "scrobble",
			Short:   "Scrobble async execute refresh 300 songs",
			Example: `  ncm partner`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context())
	}
	return c
}

func (c *Scrobble) addFlags() {
	c.cmd.PersistentFlags().StringVar(&c.opts.Crontab, "crontab", "* 18 * * *", "https://crontab.guru/")
	c.cmd.PersistentFlags().BoolVarP(&c.opts.Once, "once", "", false, "real-time execution once")
	c.cmd.PersistentFlags().StringVar(&c.opts.PlaylistId, "id", "1981392816", "playlist id")
}

func (c *Scrobble) validate() error {
	if c.opts.Crontab == "" {
		return fmt.Errorf("crontab is required")
	}
	_, err := cron.ParseStandard(c.opts.Crontab)
	if err != nil {
		return fmt.Errorf("ParseStandard: %w", err)
	}
	if c.opts.PlaylistId == "" {
		return errors.New("playlist id is required")
	}
	return nil
}

func (c *Scrobble) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Scrobble) Command() *cobra.Command {
	return c.cmd
}

func (c *Scrobble) execute(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	c.root.Cfg.Network.Debug = false
	if c.root.Opts.Debug {
		c.root.Cfg.Network.Debug = true
	}

	var (
		sum = 300
		bar = pb.Full.Start64(int64(sum))
	)

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

	// 获取一个歌单
	info, err := request.PlaylistDetail(ctx, &weapi.PlaylistDetailReq{Id: c.opts.PlaylistId})
	if err != nil {
		return fmt.Errorf("PlaylistDetail: %w", err)
	}
	if info.Code != 200 {
		return fmt.Errorf("PlaylistDetail err: %+v\n", info)
	}
	if count := len(info.Playlist.TrackIds); count < sum {
		return fmt.Errorf("%v there are less than 300 songs in the playlist", count)
	}
	var ids = make([]weapi.SongDetailReqList, 0, 600)
	for _, v := range info.Playlist.TrackIds {
		if len(ids) >= 600 {
			break
		}
		ids = append(ids, weapi.SongDetailReqList{Id: fmt.Sprintf("%d", v.Id), V: 0})
	}

	// 根据歌单id查询歌曲详情信息
	list, err := request.SongDetail(ctx, &weapi.SongDetailReq{C: ids})
	if err != nil {
		return fmt.Errorf("SongDetail: %w", err)
	}

	var total int
	defer func() {
		log.Debug("scrobble success: %d", total)
		bar.Finish()
	}()

	// 执行刷歌
	for _, v := range list.Songs {
		if total >= sum {
			break
		}

		var req = &weapi.WebLogReq{CsrfToken: "", Logs: []map[string]interface{}{
			{
				"action": "play",
				"json": map[string]interface{}{
					"type":     "song",
					"wifi":     0,
					"download": 0,
					"id":       v.Id,                                   // 歌曲id
					"time":     v.Dt / 1000,                            // 听歌消耗时间单位秒
					"end":      "playend",                              // 何种方式结束听歌 eg:ui(在网页端播放完成之后的状态) playend:参考https://gitlab.com/Binaryify/neteasecloudmusicapi/-/blob/main/module/scrobble.js interrupt:播放中途切歌
					"source":   "list",                                 // 播放歌曲资源来源 例如toplist等
					"sourceId": info.Playlist.Id,                       // 歌单id或者专辑id
					"mainsite": "1",                                    // 未知暂时为1
					"content":  fmt.Sprintf("id=%d", info.Playlist.Id), // 格式 "id=1981392816" 其中id通常为歌单id也就是和sourceId一样
				},
			},
		}}

		resp, err := request.WebLog(ctx, req)
		if err != nil {
			log.Error("[scrobble] WebLog: %w", err)
			continue
		}
		if resp.Code != 200 {
			log.Error("[scrobble] WebLog err: %+v\n", resp)
			time.Sleep(time.Second)
			continue
		}
		if resp.Code == 200 {
			total++
			bar.Increment()
			time.Sleep(time.Millisecond * 500)
		}
	}

	return nil
}

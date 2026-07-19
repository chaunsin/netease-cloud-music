// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/database"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

type ScrobbleOpts struct {
	Num int64
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
			Short:   "[need login] Scrobble execute refresh 300 songs",
			Example: `  ncmctl scrobble`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context())
	}
	return c
}

func (c *Scrobble) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Scrobble) Command() *cobra.Command {
	return c.cmd
}

func (c *Scrobble) addFlags() {
	c.cmd.PersistentFlags().Int64VarP(&c.opts.Num, "num", "n", 300, "num of songs")
}

func (c *Scrobble) validate() error {
	if c.opts.Num <= 0 || c.opts.Num > 300 {
		return errors.New("num <= 0 or > 300")
	}
	return nil
}

func (c *Scrobble) execute(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer closeAPIClient(ctx, cli)

	request := weapi.New(cli)

	// 获取用户id
	user, err := request.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		return fmt.Errorf("UserInfo: %w", err)
	}

	if user.Code != 200 || user.Profile == nil || user.Account == nil {
		return errors.New("need login")
	}

	uid := strconv.FormatInt(user.Account.Id, 10)

	// 判断是否满级，满级则不再执行。
	detail, err := request.GetUserInfoDetail(ctx, &weapi.GetUserInfoDetailReq{UserId: user.Account.Id})
	if err != nil {
		return fmt.Errorf("GetUserInfoDetail: %w", err)
	}

	if detail.Code != 200 {
		return fmt.Errorf("GetUserInfoDetail: %w", err)
	}

	if detail.Level >= 10 {
		c.cmd.Println("账号已满级")
		return nil
	}

	// 刷新token过期时间
	defer func() {
		refresh, refreshErr := request.TokenRefresh(ctx, &weapi.TokenRefreshReq{})
		if refreshErr != nil || refresh.Code != 200 {
			log.Warnf("TokenRefresh resp:%+v err: %s", refresh, refreshErr)
		}
	}()

	// 初始化数据库如果文件不存在则直接创建
	db, err := database.New(c.root.Cfg.Database)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(ctx); closeErr != nil {
			log.Errorf("close database: %v", closeErr)
		}
	}()

	// return db.Del(ctx, scrobbleTodayNumKey(uid))
	// 判断今日刷歌数量
	record, err := db.Get(ctx, scrobbleTodayNumKey(uid))
	if err != nil {
		if strings.Contains(err.Error(), "Key not found") {
			record = "0"
		} else {
			return fmt.Errorf("get scrobble today num: %w", err)
		}
	}

	finish, err := strconv.ParseInt(record, 10, 64)
	if err != nil {
		return fmt.Errorf("ParseInt(%v): %w", record, err)
	}

	if finish >= 300 {
		c.cmd.Println("today scrobble 300 completed")
		return nil
	}

	var (
		left = 300 - finish
		num  = utils.Ternary(left > c.opts.Num, c.opts.Num, left)
		bar  = pb.Full.Start64(num)
	)

	// 获取未听过得歌曲
	list, err := c.neverHeardSongs(ctx, request, db, uid, num)
	if err != nil {
		return fmt.Errorf("neverHeardSongs: %w", err)
	}

	log.Debugf("ready execute num(%d)", len(list))

	var total int
	defer func() {
		log.Debugf("scrobble success: %d", total)
		bar.Finish()
	}()

	expire, err := utils.TimeUntilMidnight("Local") // Warning: 目前由于badger过期时间使用得是本地时间因此此处也使用本地时间
	if err != nil {
		return fmt.Errorf("TimeUntilMidnight: %w", err)
	}

	// 执行刷歌
	for _, v := range list {
		req := &weapi.WebLogReq{CsrfToken: "", Logs: []map[string]any{
			{
				"action": "play",
				"json": map[string]any{
					"type":     "song",
					"wifi":     0,
					"download": 0,
					"id":       v.SongsId,                        // 歌曲id
					"time":     v.SongsTime,                      // 听歌消耗时间单位秒
					"end":      "playend",                        // 何种方式结束听歌 eg:ui(在网页端播放完成之后的状态) playend:参考https://gitlab.com/Binaryify/neteasecloudmusicapi/-/blob/main/module/scrobble.js interrupt:播放中途切歌
					"source":   v.Source,                         // 播放歌曲资源来源 例如toplist等
					"sourceId": v.SourceId,                       // [选填] 歌单id或者专辑id
					"mainsite": "1",                              // 未知暂时为1
					"content":  fmt.Sprintf("id=%v", v.SourceId), // 格式 "id=1981392816" 其中id通常为歌单id也就是和sourceId一样
				},
			},
		}}

		resp, err := request.WebLog(ctx, req)
		if err != nil {
			log.Errorf("[scrobble] WebLog: %s", err)
			continue
		}

		if resp.Code != 200 {
			log.Errorf("[scrobble] WebLog err: %+v\n", resp)
			time.Sleep(time.Second)
			continue
		}

		if resp.Code == 200 {
			if err := db.Set(ctx, scrobbleRecordKey(uid, v.SongsId), strconv.FormatInt(time.Now().UnixMilli(), 10)); err != nil {
				log.Warnf("[scrobble] set %v record err: %s", v.SongsId, err)
			}

			_, err := db.Increment(ctx, scrobbleTodayNumKey(uid), 1, expire)
			if err != nil {
				log.Warnf("[scrobble] set %v record err: %s", v.SongsId, err)
			}

			total++

			bar.Increment()
			time.Sleep(time.Millisecond * 100)
		}
	}
	return nil
}

type NeverHeardSongsList struct {
	Source    string // 资源类型
	SourceId  string // 歌单id
	SongsId   string // 歌单歌曲id
	SongsTime int64  // 歌曲时长
}

func (c *Scrobble) neverHeardSongs(ctx context.Context, request *weapi.Api, db database.Database, uid string, num int64) ([]NeverHeardSongsList, error) {
	// 获取top歌单列表
	tops, err := request.TopList(ctx, &weapi.TopListReq{})
	if err != nil {
		return nil, fmt.Errorf("TopList: %w", err)
	}

	if tops.Code != 200 {
		return nil, fmt.Errorf("TopList err: %+v", tops)
	}

	if len(tops.List) == 0 {
		return nil, errors.New("TopList is empty")
	}

	// 根据歌单返回顺序顺次刷歌直到300首歌曲
	var (
		req = make([]weapi.SongDetailReqList, 0, num)
		set = make(map[int64]string) // k:歌曲id v:歌单id
	)

	for i := range tops.List {
		list := &tops.List[i]
		// 获取一个歌单
		info, playlistErr := request.PlaylistDetail(ctx, &weapi.PlaylistDetailReq{Id: strconv.FormatInt(list.Id, 10)})
		if playlistErr != nil {
			return nil, fmt.Errorf("PlaylistDetail(%v): %w", list.Id, playlistErr)
		}

		if info.Code != 200 {
			return nil, fmt.Errorf("PlaylistDetail(%v) err: %+v", list.Id, info)
		}

		if len(info.Playlist.TrackIds) == 0 {
			log.Warnf("PlaylistDetail(%v) is empty", list.Id)
			continue
		}

		sourceId := list.Id

		for i := range info.Playlist.TrackIds {
			v := &info.Playlist.TrackIds[i]

			if int64(len(req)) >= num {
				break
			}

			// 判断是否执行过
			exist, existsErr := db.Exists(ctx, scrobbleRecordKey(uid, strconv.FormatInt(v.Id, 10)))
			if existsErr != nil || exist {
				continue
			}

			// 由于同一首歌可能会在不同得歌单中存在因此需要去重
			if _, ok := set[v.Id]; !ok {
				set[v.Id] = strconv.FormatInt(sourceId, 10)
				req = append(req, weapi.SongDetailReqList{Id: strconv.FormatInt(v.Id, 10), V: 0})
			}
		}

		if int64(len(req)) >= num {
			log.Debugf("SongDetailReqList num(%d)", len(req))
			break
		}
	}

	// 根据歌单trickIds.Id查询歌曲详情信息
	resp := make([]NeverHeardSongsList, 0, num)

	details, err := request.SongDetail(ctx, &weapi.SongDetailReq{C: req})
	if err != nil {
		return nil, fmt.Errorf("SongDetail: %w", err)
	}

	for i := range details.Songs {
		v := &details.Songs[i]
		resp = append(resp, NeverHeardSongsList{
			Source:    "toplist",
			SourceId:  set[v.Id],
			SongsId:   strconv.FormatInt(v.Id, 10),
			SongsTime: v.Dt / 1000, // 换成秒
		})
	}

	return resp, nil
}

func scrobbleRecordKey(uid, songId string) string {
	return fmt.Sprintf("scrobble:record:%v:%v", uid, songId)
}

func scrobbleTodayNumKey(uid string) string {
	return fmt.Sprintf("scrobble:today:%v", uid)
}

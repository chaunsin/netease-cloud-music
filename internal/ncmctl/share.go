// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/eapi"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

const dailySongCoverLimit int64 = 20 << 20

type dailySongState string

const (
	stateCompleted     dailySongState = "completed"      // 今日已发布动态
	stateNotRegistered dailySongState = "not-registered" // 未参与活动报名
	stateIsRegister    dailySongState = "is-register"    // 已报名
)

func classifyGuide(g *eapi.DailySongShareRegistrationGuideResp) dailySongState {
	d := g.Data
	if d.RegisteredGuide.AlreadyPubEvent {
		return stateCompleted
	}

	switch v := d.RegisterStatus; v {
	case "NOREGISTER":
		return stateNotRegistered
	case "REGISTER":
		return stateIsRegister
	default:
		return dailySongState("unknow status: " + v)
	}
}

type ShareOpts struct {
	SongID                int64
	Image, Title, Message string
	Draw, Delete, DryRun  bool
	Count                 int64
	countSet              bool
}

type DailySongShare struct {
	root *Root
	cmd  *cobra.Command
	opts ShareOpts
	l    *log.Logger
	uid  int64
}

func NewDailySongShare(root *Root, l *log.Logger) *DailySongShare {
	c := &DailySongShare{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:   "share [status|draw]",
			Short: "Publish one daily song challenge note",
			Long:  "Publish one public song note for the daily song challenge. This changes account dynamics, draws available rewards by default, and keeps the note instead of deleting it. Use 'share status' for read-only state and 'share draw' to draw rewards; --delete may affect full-attendance eligibility.",
			Args: func(cmd *cobra.Command, args []string) error {
				if len(args) > 1 {
					return fmt.Errorf("only one of 'status' or 'draw' is allowed, got %v", args)
				}

				for _, a := range args {
					if a != "status" && a != "draw" {
						return fmt.Errorf("unknown argument %q, expected 'status' or 'draw'", a)
					}
				}
				return nil
			},
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		switch {
		case slices.Contains(args, "status"):
			return c.executeStatus(cmd.Context())
		case slices.Contains(args, "draw"):
			c.opts.countSet = cmd.Flags().Changed("count")
			return c.executeDraw(cmd.Context())
		default:
			c.opts.countSet = cmd.Flags().Changed("count")
			return c.execute(cmd.Context())
		}
	}
	return c
}

func (c *DailySongShare) Command() *cobra.Command {
	return c.cmd
}

func (c *DailySongShare) addFlags() {
	f := c.cmd.Flags()
	f.Int64Var(&c.opts.SongID, "song-id", 0, "song ID; empty selects from daily recommendations")
	f.StringVar(&c.opts.Image, "image", "", "local image file; empty downloads the song cover")
	f.StringVar(&c.opts.Title, "title", "", "note title; default is 今日推荐：<song name>")
	f.StringVar(&c.opts.Message, "message", "", "note message; must contain at least 10 Unicode characters")
	f.BoolVar(&c.opts.Draw, "draw", true, "draw available rewards after publish")
	f.BoolVar(&c.opts.Delete, "delete", false, "delete this note after a successful lottery; may affect full attendance")
	f.BoolVar(&c.opts.DryRun, "dry-run", false, "read state and prepare text without changing account state or uploading")
	f.Int64Var(&c.opts.Count, "count", 0, "number of draws (1-8); default uses all server-reported chances")
}

func (c *DailySongShare) validateFlags(parent bool) error {
	if c.opts.SongID < 0 || (c.opts.SongID == 0 && c.cmd.Flags().Changed("song-id")) {
		return errors.New("song-id must be a positive integer")
	}

	if c.opts.Title != "" && strings.TrimSpace(c.opts.Title) == "" {
		return errors.New("title must not be empty")
	}

	if c.opts.Message != "" && utf8.RuneCountInString(strings.TrimSpace(c.opts.Message)) < 10 {
		return errors.New("message must contain at least 10 Unicode characters")
	}

	if c.opts.Image != "" {
		i, e := os.Lstat(c.opts.Image)
		if e != nil {
			return fmt.Errorf("image: %w", e)
		}

		if i.Mode()&os.ModeSymlink != 0 || !i.Mode().IsRegular() || i.Size() == 0 {
			return errors.New("image must be a non-empty regular file, not a symlink")
		}
	}

	// if c.opts.Delete && (!parent || !c.opts.Draw) {
	// 	return errors.New("--delete requires the publish command with --draw enabled")
	// }

	if c.opts.countSet && (c.opts.Count < 1 || c.opts.Count > 8) {
		return errors.New("count must be between 1 and 8")
	}

	if !parent && (c.opts.DryRun || c.opts.Delete) {
		return errors.New("flag is only valid for the publish command")
	}
	return nil
}

func (c *DailySongShare) validate() error {
	return c.validateFlags(true)
}

func (c *DailySongShare) load(ctx context.Context) (*api.Client, *weapi.Api, *eapi.Api, *eapi.DailySongShareRegistrationGuideResp, error) {
	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create API client: %w", err)
	}

	e := eapi.New(cli)
	w := weapi.New(cli)

	u, err := w.GetUserInfo(ctx, &weapi.GetUserInfoReq{})
	if err != nil {
		_ = cli.Close(ctx)
		return nil, nil, nil, nil, fmt.Errorf("get user info: %w", err)
	}

	if u.Code != 200 || u.Profile == nil || u.Profile.UserId <= 0 {
		_ = cli.Close(ctx)
		return nil, nil, nil, nil, errors.New("need login")
	}

	c.uid = u.Profile.UserId

	c.cmd.Printf("\n📤 每日歌曲挑战\n\n")

	c.cmd.Printf("  账号: %s(UID %d)\n", u.Profile.Nickname, u.Profile.UserId)

	g, err := c.guide(ctx, e)
	if err != nil {
		closeAPIClient(ctx, cli, c.l)
		return nil, nil, nil, nil, err
	}

	d := g.Data

	c.cmd.Printf("  活动周期: %s(周期ID %d)\n", d.Duration, d.ActivityCycleId)
	c.cmd.Printf("  报名状态: %s\n", d.RegisterStatus)
	c.cmd.Printf("  活动ID: %d\n", d.ActivityId)
	c.cmd.Printf("  已发布笔记: %d 篇\n", d.RegisteredGuide.PubEventCount)
	c.cmd.Printf("  提示: %s\n", d.RegisteredGuide.SignUp)
	c.cmd.Printf("  抽奖机会提示: %s\n", d.RegisteredGuide.SignTip)
	c.cmd.Printf("  抽奖机会: RewardCount=%v HaveRewardCount=%v\n", d.RegisteredGuide.RewardCount, d.RegisteredGuide.HaveRewardCount)

	if d.RewardJumpUrl != "" {
		c.cmd.Printf("  奖励领取: %s\n", d.RewardJumpUrl)
	}
	return cli, w, e, g, nil
}

func (c *DailySongShare) guide(ctx context.Context, e *eapi.Api) (*eapi.DailySongShareRegistrationGuideResp, error) {
	g, err := e.DailySongShareRegistrationGuide(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("registration guide: %w", err)
	}

	if g.Code != 200 {
		return nil, fmt.Errorf("registration guide: code=%d message=%s", g.Code, g.Message)
	}
	return g, nil
}

func (c *DailySongShare) executeStatus(ctx context.Context) error {
	if err := c.validateFlags(false); err != nil {
		return err
	}

	cli, _, _, _, err := c.load(ctx)
	if err != nil {
		return err
	}
	defer closeAPIClient(ctx, cli, c.l)

	c.cmd.Printf("\n状态查看完成\n")
	return nil
}

func (c *DailySongShare) executeDraw(ctx context.Context) error {
	if err := c.validateFlags(false); err != nil {
		return err
	}

	cli, w, e, g, err := c.load(ctx)
	if err != nil {
		return err
	}
	defer closeAPIClient(ctx, cli, c.l)

	if err := c.draw(ctx, e, w, g, c.opts.Count, c.opts.countSet); err != nil {
		return err
	}

	c.cmd.Printf("\n抽奖完成\n")
	return nil
}

func (c *DailySongShare) execute(ctx context.Context) error {
	if err := c.validateFlags(true); err != nil {
		return err
	}

	a, w, e, g, err := c.load(ctx)
	if err != nil {
		return err
	}
	defer closeAPIClient(ctx, a, c.l)

	if c.opts.DryRun {
		song, selectErr := c.selectSong(ctx, w)
		if selectErr != nil {
			return selectErr
		}

		t, m := c.text(song)
		c.cmd.Println("  dry-run 预览:")
		c.cmd.Printf("    歌曲: %s(ID %d)\n", song.name, song.id)
		c.cmd.Printf("    标题: %s\n", t)
		c.cmd.Printf("    正文: %s\n", m)
		c.cmd.Printf("\ndry-run 预览结束\n")
		return nil
	}

	state := classifyGuide(g)
	if strings.HasPrefix(string(state), "unknow status:") {
		c.cmd.Printf("  活动状态异常 %q,已终止\n", g.Data.RegisterStatus)
		return nil
	}

	// 执行自动参加活动报名
	if state == stateNotRegistered {
		// todo: 此接口功能未知待探索。
		resp, err := e.DailySongShareRegister(ctx, &eapi.DailySongShareRegisterReq{})
		if err != nil {
			return fmt.Errorf("register activity: %w", err)
		}

		if resp.Code != 200 {
			return fmt.Errorf("register activity: code=%d message=%s", resp.Code, resp.Message)
		}

		// todo:
		// if resp.Data.NoteAttendance {
		// }

		if g.Data.ActivityId <= 0 || g.Data.ActivityCycleId <= 0 {
			return errors.New("registration guide lacks activity or cycle ID")
		}

		// 参加报名活动
		arResp, err := e.DailySongShareAttendanceRegister(ctx, &eapi.DailySongShareAttendanceRegisterReq{
			ActivityId:      g.Data.ActivityId,
			ActivityCycleId: g.Data.ActivityCycleId,
			AutoRegister:    true,
		})
		if err != nil {
			return fmt.Errorf("register activity cycle: %w", err)
		}

		if arResp.Code != 200 {
			return fmt.Errorf("register activity cycle: code=%d message=%s", arResp.Code, arResp.Message)
		}

		g, err = c.guide(ctx, e)
		if err != nil {
			return err
		}

		if g.Data.RegisterStatus != "REGISTER" {
			return fmt.Errorf("register status is invalid: %s", state)
		}
	}

	var publish *eapi.DailySonSharePublishResp

	// 执行发布动态逻辑
	if state == stateCompleted {
		c.cmd.Println("  发布动态: 今日已打卡，已跳过")
	} else {
		// 选择分享一首歌
		song, err := c.selectSong(ctx, w)
		if err != nil {
			return err
		}

		// 下载歌曲封面图
		image, cleanup, ime := c.prepareImage(ctx, song.cover)
		if ime != nil {
			return ime
		}
		defer cleanup()

		// 上传图片
		pics, upe := e.EventUploadImage(ctx, image)
		if upe != nil {
			return fmt.Errorf("upload image: %w", upe)
		}

		var (
			id         = strconv.FormatInt(song.id, 10)
			title, msg = c.text(song)
		)

		// 发布动态
		// TODO: 待 live 接口联调确认发布请求是否还需 PubSource / ActivityInfoList 等字段，
		// 当前按需留空，避免无验证地提交不完整请求。
		pub, err := e.DailySongSharePublish(ctx, &eapi.DailySongSharePublishReq{
			Type:               "song",
			Id:                 id,
			Uid:                strconv.FormatInt(song.uid, 10),
			Title:              title,
			Msg:                msg,
			Pics:               pics,
			PrivacySetting:     "0",
			SocialSpaceVisible: 1,
		})
		if err != nil {
			return fmt.Errorf("publish note: %w", err)
		}

		if pub.Code != 200 {
			return fmt.Errorf("publish note: code=%d message=%s", pub.Code, pub.Message)
		}

		if pub.ID <= 0 {
			return errors.New("publish note succeeded without a valid event ID")
		}

		c.cmd.Printf("  发布动态: 成功(动态ID %d)\n", pub.ID)

		// 关联触发事件
		tr, err := e.DailySongShareTrigger(ctx, &eapi.DailySongShareTriggerReq{SongId: id, Channel: "cloudmusic"})
		if err != nil || tr.Code != 200 || !tr.Data {
			if err != nil {
				return fmt.Errorf("trigger share: %w", err)
			}
			return fmt.Errorf("trigger share: code=%d message=%s", tr.Code, tr.Message)
		}

		c.cmd.Println("  触发挑战: 成功")

		g, err = c.guide(ctx, e)
		if err != nil {
			return fmt.Errorf("confirm published note: %w", err)
		}

		if classifyGuide(g) != stateCompleted {
			return errors.New("published note was not confirmed by activity guide")
		}

		publish = pub
	}

	// 删除动态
	if c.opts.Delete && publish == nil {
		c.cmd.Println("  删除动态: 本次未发布新动态，--delete 已忽略")
	} else if c.opts.Delete && publish != nil && publish.ID > 0 {
		// 避免风控睡眠5到10秒再删除
		time.Sleep(time.Second * time.Duration(5+rand.IntN(10)))

		d, err := e.EventDelete(ctx, &eapi.EventDeleteReq{Id: publish.ID})
		if err != nil {
			return fmt.Errorf("delete event %d: %w", publish.ID, err)
		}

		if d.Code != 200 {
			return fmt.Errorf("delete event %d: code=%d message=%s", publish.ID, d.Code, d.Message)
		}

		c.cmd.Printf("  删除动态: 已删除(动态ID %d)\n", publish.ID)
	}

	// 抽奖
	if c.opts.Draw {
		if err := c.draw(ctx, e, w, g, c.opts.Count, c.opts.countSet); err != nil {
			return fmt.Errorf("lottery: %w", err)
		}
	}

	c.cmd.Printf("\n每日歌曲挑战完成\n")
	return nil
}

func (c *DailySongShare) draw(ctx context.Context, a *eapi.Api, w *weapi.Api, g *eapi.DailySongShareRegistrationGuideResp, count int64, explicit bool) error {
	const maxDraws = 8 // SPEC: 单次活动抽奖上限为 8 次？

	// n := min(g.Data.RegisteredGuide.HaveRewardCount, maxDraws)
	n := min(g.Data.RegisteredGuide.RewardCount, maxDraws)

	if n <= 0 {
		c.cmd.Println("  抽奖: 暂无可用的抽奖机会")
		return nil
	}

	c.cmd.Println("  抽奖:")

	if explicit && count < n {
		n = count
	}

	for i := int64(0); i < n; i++ {
		resp, err := a.DailySongShareLottery(ctx, &eapi.DailySongShareLotteryReq{ActivityId: g.Data.ActivityInterestId})
		if err != nil {
			return err
		}

		if resp.Code != 200 {
			return fmt.Errorf("code=%d message=%s", resp.Code, resp.Message)
		}

		c.cmd.Printf("    第 %d 次: 剩余 %d 次", i+1, resp.Data.RestChance)

		// 会出现抽不到奖品的情况返回结果为: {"code":200,"data":{"userId":1289504343,"batchIdemKey":null,"idempotentId":"5ffed82c-1a6d-4fc5-b1f5-265862cc36e3","activityId":11066304,"prizeSchemeId":11147804,"drawPrizeTime":1787627166087,"drawPrizeInfoList":[],"prizeDetailInfoMap":{},"noLotteryContent":null,"restChance":0,"collectDTO":null},"message":""}
		if len(resp.Data.PrizeDetailInfoMap) == 0 {
			c.cmd.Println("，很遗憾没抽到")
		} else {
			for _, p := range resp.Data.PrizeDetailInfoMap {
				c.cmd.Printf("，奖品「%s」,说明: %s,兑换: %s\n", p.PrizeName, p.WinPrizeDesc, p.ExchangeUrl)
				// 当抽到云贝时需要24小时领取不然会出现过期。
				if strings.Contains(p.PrizeName, "云贝") {
					c.cmd.Println("    开始自动领取云贝")

					if err = yunbeiClaim(ctx, w, c.l, c.cmd); err != nil {
						return fmt.Errorf("yunbeiClaim: %w", err)
					}
				}
				break
			}
		}

		// 以服务端剩余次数为准，避免单次消耗多次机会时超额抽奖
		if resp.Data.RestChance <= 0 {
			break
		}

		if resp.Data.RestChance < n-i-1 {
			n = i + 1 + resp.Data.RestChance
		}

		// 避免风控睡眠1到5秒
		time.Sleep(time.Second * time.Duration(1+rand.IntN(5)))
	}
	return nil
}

type dailySong struct {
	id, uid     int64
	name, cover string
}

func (c *DailySongShare) selectSong(ctx context.Context, w *weapi.Api) (dailySong, error) {
	if c.opts.SongID > 0 {
		resp, err := w.SongDetail(ctx, &weapi.SongDetailReq{C: []weapi.SongDetailReqList{{Id: strconv.FormatInt(c.opts.SongID, 10)}}})
		if err != nil {
			return dailySong{}, fmt.Errorf("song detail: %w", err)
		}

		if len(resp.Songs) != 1 || resp.Songs[0].Id <= 0 || resp.Songs[0].Name == "" {
			return dailySong{}, errors.New("song detail returned no usable song")
		}
		return dailySong{
			id:    resp.Songs[0].Id,
			uid:   c.uid,
			name:  resp.Songs[0].Name,
			cover: resp.Songs[0].Al.PicUrl,
		}, nil
	}

	resp, err := w.RecommendSongs(ctx, &weapi.RecommendSongsReq{})
	if err != nil {
		return dailySong{}, fmt.Errorf("recommend songs: %w", err)
	}

	valid := make([]dailySong, 0, len(resp.Data.DailySongs))
	for i := range resp.Data.DailySongs {
		v := &resp.Data.DailySongs[i]
		if v.Id > 0 && v.Name != "" && v.Al.PicUrl != "" {
			valid = append(valid, dailySong{id: v.Id, uid: c.uid, name: v.Name, cover: v.Al.PicUrl})
		}
	}

	if len(valid) == 0 {
		return dailySong{}, errors.New("daily recommendations contain no usable song")
	}

	var b [2]byte

	_, _ = cryptorand.Read(b[:])
	return valid[int((uint64(b[0])<<8|uint64(b[1]))%uint64(len(valid)))], nil
}

func (c *DailySongShare) text(s dailySong) (string, string) {
	t := strings.TrimSpace(c.opts.Title)
	if t == "" {
		t = "今日推荐: " + s.name
	}

	m := strings.TrimSpace(c.opts.Message)
	if m == "" {
		m = "分享一首今天听到的好歌，欢迎一起听听。"
	}

	return t, m
}

func (c *DailySongShare) prepareImage(ctx context.Context, cover string) (string, func(), error) {
	if c.opts.Image != "" {
		return c.opts.Image, func() {}, nil
	}

	if cover == "" {
		return "", func() {}, errors.New("song has no cover URL; specify --image")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cover, http.NoBody)
	if err != nil {
		return "", func() {}, err
	}

	client := &http.Client{Timeout: c.root.Cfg.Network.Timeout}

	res, err := client.Do(req)
	if err != nil {
		return "", func() {}, fmt.Errorf("download cover: %w", err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode/100 != 2 {
		return "", func() {}, fmt.Errorf("download cover: HTTP %s", res.Status)
	}

	tmp, err := os.CreateTemp("", "ncmctl-daily-song-*.img")
	if err != nil {
		return "", func() {}, err
	}

	var (
		name    = filepath.Clean(tmp.Name())
		cleanup = func() { _ = os.Remove(name) }
	)

	n, err := io.Copy(tmp, io.LimitReader(res.Body, dailySongCoverLimit+1))
	if ce := tmp.Close(); err == nil {
		err = ce
	}

	if err != nil {
		cleanup()
		return "", func() {}, err
	}

	if n > dailySongCoverLimit {
		cleanup()
		return "", func() {}, errors.New("cover exceeds 20 MiB")
	}
	return name, cleanup, nil
}

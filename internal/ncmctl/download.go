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
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/ncm"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	pb "github.com/cheggaaa/pb/v3"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
	"golang.org/x/sync/semaphore"
)

const (
	barNameWidth        = 35
	downloadBarTemplate = `{{string . "prefix"}} {{bar . }} {{percent . "%6.2f%%"}}`
)

func fixedWidthName(s string, width int) string {
	return runewidth.FillRight(runewidth.Truncate(s, width, ".."), width)
}

type DownloadOpts struct {
	Output   string // 输出目录
	Parallel int64  // 并发下载数量
	Level    string // 歌曲品质 types.Level
	Strict   bool   // 严格模式。当开起时指定的歌曲品质不符合要求,则不进行下载
	Tag      bool
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
			Example: `  ncmctl download 2161154646`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("input is empty, please enter the song id or song link")
		}
		return c.execute(cmd.Context(), args)
	}
	return c
}

func (c *Download) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "./download", "music file output path")
	c.cmd.PersistentFlags().Int64VarP(&c.opts.Parallel, "parallel", "p", 5, "concurrent download count")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Level, "level", "l", string(types.LevelLossless), "song quality level. support: standard/128,higher/192,exhigh/HQ/320,lossless/SQ,hires/HR")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Strict, "strict", false, "strict mode. when the downloaded song does not find the corresponding quality, it will not be downloaded.")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Tag, "tag", true, "whether to set song tag information,default enable")
}

func (c *Download) validate() error {
	if c.opts.Parallel <= 0 || c.opts.Parallel > 20 {
		return fmt.Errorf("parallel <= 0 or > 10")
	}

	lv, err := strconv.ParseInt(c.opts.Level, 10, 64)
	if err == nil {
		switch lv {
		case 128:
			c.opts.Level = string(types.LevelStandard)
		case 192:
			c.opts.Level = string(types.LevelHigher)
		case 320:
			c.opts.Level = string(types.LevelExhigh)
		default:
			return fmt.Errorf("%v level is not support", lv)
		}
	}

	switch types.Level(c.opts.Level) {
	case "":
		return fmt.Errorf("level is empty")
	case types.LevelStandard,
		types.LevelHigher,
		types.LevelExhigh,
		types.LevelLossless,
		types.LevelHires:
		// types.LevelJyeffect,
		// types.LevelSky,
		// types.LevelJymaster:
		// validate ok
	default:
		switch strings.ToUpper(c.opts.Level) {
		case "HQ":
			c.opts.Level = string(types.LevelExhigh)
		case "SQ":
			c.opts.Level = string(types.LevelLossless)
		case "HR":
			c.opts.Level = string(types.LevelHires)
		default:
			return fmt.Errorf("[%s] quality is not support", c.opts.Level)
		}
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

	cli, err := api.NewClient(c.root.Cfg.Network, c.l)
	if err != nil {
		return fmt.Errorf("NewClient: %w", err)
	}
	defer cli.Close(ctx)
	request := weapi.New(cli)

	// 判断是否需要登录
	if request.NeedLogin(ctx) {
		return fmt.Errorf("need login")
	}

	// 刷新token过期时间
	defer func() {
		refresh, err := request.TokenRefresh(ctx, &weapi.TokenRefreshReq{})
		if err != nil || refresh.Code != 200 {
			log.Warn("TokenRefresh resp:%+v err: %s", refresh, err)
		}
	}()

	if err := utils.MkdirIfNotExist(c.opts.Output, 0755); err != nil {
		return fmt.Errorf("MkdirIfNotExist: %w", err)
	}

	// 解析处理输入的资源类型
	songs, err := c.inputParse(ctx, args, request)
	if err != nil {
		return fmt.Errorf("inputParse: %w", err)
	}

	var (
		failed atomic.Int64
		sema   = semaphore.NewWeighted(c.opts.Parallel)
	)

	pool, err := pb.StartPool()
	if err != nil {
		return fmt.Errorf("StartPool: %w", err)
	}
	defer pool.Stop()

	for _, song := range songs {
		var song = song
		if err := sema.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("acquire: %w", err)
		}
		go func() {
			defer sema.Release(1)
			if err := c.download(ctx, cli, request, &song, pool); err != nil {
				failed.Add(1)
				log.Error("download %s err: %v", song.String(), err)
				return
			}
		}()
	}
	if err := sema.Acquire(ctx, c.opts.Parallel); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	return nil
}

func (c *Download) inputParse(ctx context.Context, args []string, request *weapi.Api) ([]Music, error) {
	var (
		source = make(map[string][]int64)
		set    = make(map[int64]struct{})
		list   []Music
	)
	for _, arg := range args {
		kind, id, err := Parse(arg)
		if err != nil {
			return nil, fmt.Errorf("Parse: %w", err)
		}
		if v, ok := source[kind]; ok {
			source[kind] = append(v, id)
		} else {
			source[kind] = []int64{id}
		}
	}

	for k, ids := range source {
		switch k {
		case "song":
			{
				var tmp = make([]int64, 0, len(ids))
				for _, id := range ids {
					if _, ok := set[id]; ok {
						continue
					}
					set[id] = struct{}{}
					tmp = append(tmp, id)
				}

				// 分页处理
				pages, _ := utils.SplitSlice(tmp, 500)
				for _, p := range pages {
					var c = make([]weapi.SongDetailReqList, 0, len(p))
					for _, v := range p {
						c = append(c, weapi.SongDetailReqList{Id: fmt.Sprintf("%v", v), V: 0})
					}
					resp, err := request.SongDetail(ctx, &weapi.SongDetailReq{C: c})
					if err != nil {
						return nil, fmt.Errorf("SongDetail: %w", err)
					}
					if resp.Code != 200 {
						return nil, fmt.Errorf("SongDetail err: %+v", resp)
					}
					if len(resp.Songs) <= 0 {
						log.Warn("SongDetail() Songs is empty")
						continue
					}
					for _, v := range resp.Songs {
						list = append(list, Music{
							Id:      v.Id,
							Name:    v.Name,
							Artist:  v.Ar,
							Album:   v.Al,
							AlbumId: v.Al.Id,
							Time:    v.Dt,
						})
					}
					// todo: 处理版权,状态等有效性校验
				}
			}
		case "artist":
			for _, id := range ids {
				for i := 1; ; i++ {
					artist, err := request.ArtistSongs(ctx, &weapi.ArtistSongsReq{
						Id:           id,
						PrivateCloud: "true",
						WorkType:     1,
						Order:        "hot",
						Offset:       int64(i),
						Limit:        500,
					})
					if err != nil {
						return nil, fmt.Errorf("ArtistSongs(%v): %w", id, err)
					}
					if artist.Code != 200 {
						return nil, fmt.Errorf("ArtistSongs(%v) err: %+v", id, artist)
					}
					if len(artist.Songs) <= 0 {
						log.Warn("ArtistSongs(%v) songs is empty", id)
						break
					}
					if !artist.More {
						break
					}
					for _, v := range artist.Songs {
						if _, ok := set[v.Id]; ok {
							continue
						}
						set[id] = struct{}{}
						list = append(list, Music{
							Id:      v.Id,
							Name:    v.Name,
							Artist:  v.Ar,
							Album:   v.Al,
							AlbumId: v.Al.Id,
							Time:    v.Dt,
						})
					}
					// todo: 处理版权,状态等有效性校验
				}
			}
		case "album":
			for _, id := range ids {
				album, err := request.Album(ctx, &weapi.AlbumReq{Id: fmt.Sprintf("%d", id)})
				if err != nil {
					return nil, fmt.Errorf("Album(%v): %w", id, err)
				}
				if album.Code != 200 {
					return nil, fmt.Errorf("Album(%v) err: %+v", id, album)
				}
				if len(album.Songs) <= 0 {
					log.Warn("Album(%v) Songs is empty", id)
					continue
				}
				for _, v := range album.Songs {
					if _, ok := set[v.Id]; ok {
						continue
					}
					set[id] = struct{}{}
					list = append(list, Music{
						Id:      v.Id,
						Name:    v.Name,
						Artist:  v.Ar,
						Album:   v.Al,
						AlbumId: v.Al.Id,
						Time:    v.Dt,
					})
				}
				// todo: 处理版权,状态等有效性校验
			}
		case "playlist":
			for _, id := range ids {
				playlist, err := request.PlaylistDetail(ctx, &weapi.PlaylistDetailReq{Id: fmt.Sprintf("%d", id)})
				if err != nil {
					return nil, fmt.Errorf("PlaylistDetail(%v): %w", id, err)
				}
				if playlist.Code != 200 {
					return nil, fmt.Errorf("PlaylistDetail(%v) err: %+v", id, playlist)
				}
				if playlist.Playlist.TrackIds == nil {
					log.Warn("PlaylistDetail(%v) Tracks is nil", id)
					continue
				}
				var tmp = make([]int64, 0, len(playlist.Playlist.TrackIds))
				for _, v := range playlist.Playlist.TrackIds {
					if _, ok := set[v.Id]; ok {
						continue
					}
					set[id] = struct{}{}
					tmp = append(tmp, v.Id)
				}
				var trackMap = make(map[int64]Music)
				for _, v := range playlist.Playlist.Tracks {
					trackMap[v.Id] = Music{
						Id:      v.Id,
						Name:    v.Name,
						Artist:  v.Ar,
						Album:   v.Al,
						AlbumId: v.Al.Id,
						Time:    v.Dt,
					}
				}

				// 分页处理
				pages, _ := utils.SplitSlice(tmp, 500)
				for _, p := range pages {
					// 检查这些 ID 是否都在 trackMap 中
					var missingIds []int64
					for _, id := range p {
						if _, ok := trackMap[id]; !ok {
							missingIds = append(missingIds, id)
						} else {
							list = append(list, trackMap[id])
						}
					}

					if len(missingIds) == 0 {
						continue
					}

					var c = make([]weapi.SongDetailReqList, 0, len(missingIds))
					for _, v := range missingIds {
						c = append(c, weapi.SongDetailReqList{Id: fmt.Sprintf("%v", v), V: 0})
					}
					resp, err := request.SongDetail(ctx, &weapi.SongDetailReq{C: c})
					if err != nil {
						return nil, fmt.Errorf("SongDetail: %w", err)
					}
					if resp.Code != 200 {
						return nil, fmt.Errorf("SongDetail err: %+v", resp)
					}
					if len(resp.Songs) <= 0 {
						log.Warn("SongDetail Songs is empty")
						continue
					}
					for _, v := range resp.Songs {
						list = append(list, Music{
							Id:      v.Id,
							Name:    v.Name,
							Artist:  v.Ar,
							Album:   v.Al,
							AlbumId: v.Al.Id,
							Time:    v.Dt,
						})
					}
					// todo: 处理版权,状态等有效性校验
				}
			}
		default:
			return nil, fmt.Errorf("[%s] is not support", k)
		}
	}
	if len(list) <= 0 {
		return nil, fmt.Errorf("input resource is empty or the song is copyrighted")
	}
	return list, nil
}

func (c *Download) download(ctx context.Context, cli *api.Client, request *weapi.Api, music *Music, pool *pb.Pool) error {
	var (
		songId    = music.Id
		songIdStr = fmt.Sprintf("%d", songId)
	)

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

	// 获取下载链接地址
	var downReq = &weapi.SongPlayerV1Req{
		Ids:         types.IntsString{songId},
		Level:       types.Level(c.opts.Level),
		EncodeType:  "flac",
		ImmerseType: "c51",
	}
	downResp, err := request.SongPlayerV1(ctx, downReq)
	if err != nil {
		return fmt.Errorf("SongPlayerV1(%v): %w", songId, err)
	}
	if downResp.Code != 200 {
		return fmt.Errorf("SongPlayerV1(%v) err: %+v", songId, downResp)
	}
	if len(downResp.Data) <= 0 {
		return fmt.Errorf("SongPlayerV1(%v) is empty: %+v", songId, downResp)
	}
	// 歌曲变灰则不能下载
	if downResp.Data[0].Code != 200 || downResp.Data[0].Url == "" {
		var msg error
		switch downResp.Data[0].Code {
		case -110:
			msg = fmt.Errorf("无音源(%v) br: %v code: %v", songId, quality.Br, downResp.Data[0].Code)
		case -105: // todo: 待确定完善,目前测试发现,当用户没有会员权益时,会返回-105，其他情况可能也会返回此值
			fallthrough
		default:
			msg = fmt.Errorf("资源已下架或无版权(%v) br: %v code: %v", songId, quality.Br, downResp.Data[0].Code)
		}
		log.Warn("资源已下架或无版权(%v) detail: %+v", songId, downResp)
		return msg
	}

	var (
		drd      = downResp.Data[0]
		dest     = filepath.Join(c.opts.Output, fmt.Sprintf("%s - %s.%s", music.ArtistString(), music.NameString(), strings.ToLower(drd.Type)))
		tempName = fmt.Sprintf("download-*-%s.tmp", music.NameString())
	)

	// 创建临时文件
	file, err := os.CreateTemp(c.opts.Output, tempName)
	if err != nil {
		return fmt.Errorf("CreateTemp: %w", err)
	}
	defer file.Close()

	// 下载
	bar := pb.New64(drd.Size).
		Set(pb.Bytes, true).
		Set("prefix", fixedWidthName(fmt.Sprintf("%s - %s", music.ArtistString(), music.NameString()), barNameWidth)).
		SetTemplateString(downloadBarTemplate)
	pool.Add(bar)
	defer bar.Finish()

	resp, err := cli.Download(ctx, drd.Url, nil, nil, file, bar)
	if err != nil {
		_ = os.Remove(file.Name())
		return fmt.Errorf("download: %w", err)
	}
	if c.root.Opts.Debug {
		dump, err := httputil.DumpResponse(resp, false)
		if err != nil {
			log.Debug("DumpResponse err: %s", err)
		} else {
			log.Debug("Download DumpResponse: %s", dump)
		}
	}

	size, _ := strconv.ParseFloat(resp.Header.Get("Content-Length"), 10)
	log.Debug("id=%v downloadUrl=%v wantLevel=%v-%v realLevel=%v-%v encodeType=%v type=%v size=%0.2fM,%vKB free=%v tempFile=%s outDir=%s",
		drd.Id, drd.Url, c.opts.Level, quality.Br, drd.Level, drd.Br, drd.EncodeType, drd.Type, size/float64(utils.MB), int64(size), types.Free(drd.Fee), file.Name(), dest)

	// 校验md5文件完整性
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = os.Remove(file.Name())
		return fmt.Errorf("Seek: %w", err)
	}
	var m = md5.New()
	if _, err := io.Copy(m, file); err != nil {
		_ = os.Remove(file.Name())
		return err
	}
	if m := hex.EncodeToString(m.Sum(nil)); m != drd.Md5 {
		_ = os.Remove(file.Name())
		return fmt.Errorf("file %v md5 not match, want=%s, got=%s", file.Name(), drd.Md5, m)
	}

	// 设置歌曲tag值
	if c.opts.Tag {
		// 显示关闭文件避免Windows系统无法重命名错误: The process cannot access the file because it is being used by another process
		if err := file.Close(); err != nil {
			log.Error("close %s file err: %s", file.Name(), err)
			_ = os.Remove(file.Name())
			return err
		}

		var meta = &ncm.MetadataMusic{
			Id:       music.Id,
			Name:     music.Name,
			Album:    music.Album.Name,
			AlbumPic: music.Album.PicUrl,
			Format:   drd.Type,
		}
		for _, ar := range music.Artist {
			meta.Artists = append(meta.Artists, ncm.Artist{Name: ar.Name, Id: ar.Id})
		}

		// 获取歌词
		lyricResp, err := request.Lyric(ctx, &weapi.LyricReq{Id: music.Id})
		if err != nil {
			log.Warn("get lyric %d err: %v", music.Id, err)
		} else if lyricResp.Code == 200 {
			if lyricResp.Lrc.Lyric != "" {
				// todo: 翻译歌词
				meta.Comment = lyricResp.Lrc.Lyric
			}
		}

		// 下载封面
		var coverData []byte
		//fmt.Printf("meta.AlbumPic: %s\n", meta.AlbumPic)
		if meta.AlbumPic != "" {
			// 移除 URL 中的 query 参数，通常能获取到原图
			if idx := strings.Index(meta.AlbumPic, "?"); idx > 0 {
				meta.AlbumPic = meta.AlbumPic[:idx]
			}
			resp, err := http.Get(meta.AlbumPic)
			if err == nil && resp.StatusCode == 200 {
				coverData, _ = io.ReadAll(resp.Body)
				resp.Body.Close()
			} else {
				log.Warn("download cover %s err: %v", meta.AlbumPic, err)
			}
		}

		if len(coverData) == 0 {
			if music.AlbumId != 0 {
				albumResp, err := request.Album(ctx, &weapi.AlbumReq{Id: fmt.Sprintf("%d", music.AlbumId)})
				if err == nil && albumResp.Code == 200 && albumResp.Album.PicUrl != "" {
					meta.AlbumPic = albumResp.Album.PicUrl
					// 移除 URL 中的 query 参数，通常能获取到原图
					if idx := strings.Index(meta.AlbumPic, "?"); idx > 0 {
						meta.AlbumPic = meta.AlbumPic[:idx]
					}
					resp, err := http.Get(meta.AlbumPic)
					if err == nil && resp.StatusCode == 200 {
						coverData, _ = io.ReadAll(resp.Body)
						resp.Body.Close()
					}
				}
			}
		}

		switch strings.ToLower(drd.Type) {
		case "mp3":
			if err := writeID3v2(file.Name(), meta, coverData); err != nil {
				log.Warn("writeID3v2 %s err: %v", file.Name(), err)
			}
		case "flac":
			if err := writeFlac(file.Name(), meta, coverData); err != nil {
				log.Warn("writeFlac %s err: %v", file.Name(), err)
			}
		default:
			log.Warn("unsupported tag format: %s", drd.Type)
		}
	} else {
		// 显示关闭文件避免Windows系统无法重命名错误: The process cannot access the file because it is being used by another process
		if err := file.Close(); err != nil {
			log.Error("close %s file err: %s", file.Name(), err)
			_ = os.Remove(file.Name())
			return err
		}
	}
	if err := os.Rename(file.Name(), dest); err != nil {
		_ = os.Remove(file.Name())
		return fmt.Errorf("rename: %w", err)
	}
	if err := os.Chmod(dest, 0644); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	return nil
}

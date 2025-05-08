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
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/cheggaaa/pb/v3"
	"github.com/dhowden/tag"
	"github.com/spf13/cobra"
	"golang.org/x/sync/semaphore"
)

const maxSize = 500 * utils.MB

type CloudOpts struct {
	Parallel int64  // 并发上传文件数量
	MinSize  string // 上传文件最低大小限制
	Regexp   string // 上传过滤正则表达式
}

type Cloud struct {
	root *Root
	cmd  *cobra.Command
	opts CloudOpts
	l    *log.Logger
}

func NewCloud(root *Root, l *log.Logger) *Cloud {
	c := &Cloud{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "cloud",
			Short:   "[need login] Used to upload music files to netease cloud disk",
			Example: "  ncmctl cloud -h\n  ncmctl cloud ./mymusic.mp3\n  ncmctl cloud ./my/music/ (Use directory)",
			Args:    cobra.RangeArgs(0, 1),
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}

	return c
}

func (c *Cloud) addFlags() {
	c.cmd.PersistentFlags().Int64VarP(&c.opts.Parallel, "parallel", "p", 3, "concurrent upload count")
	c.cmd.PersistentFlags().StringVarP(&c.opts.MinSize, "minsize", "m", "", "upload music minimum file size limit. supporting unit:b、k/kb/KB、m/mb/MB")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Regexp, "regexp", "r", "", "upload music file name filter regular expression")
}

func (c *Cloud) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Cloud) Command() *cobra.Command {
	return c.cmd
}

func (c *Cloud) execute(ctx context.Context, input []string) error {
	if c.opts.Parallel < 0 || c.opts.Parallel > 10 {
		return fmt.Errorf("parallel must be between 1 and 10")
	}
	if len(input) <= 0 {
		c.cmd.Println("nothing was entered")
		return nil
	}
	var (
		fileList = make([]string, 0, len(input))
		barSize  int64
		minSize  *int64
		skip     atomic.Int64
		fail     atomic.Int64
		reg      *regexp.Regexp
	)

	if c.opts.MinSize != "" {
		size, err := utils.ParseBytes(c.opts.MinSize)
		if err != nil {
			return fmt.Errorf("bytesize.Parse: %w", err)
		}
		minSize = &size
	}
	if c.opts.Regexp != "" {
		var err error
		reg, err = regexp.Compile(c.opts.Regexp)
		if err != nil {
			return fmt.Errorf("regexp.Compile: %w", err)
		}
	}

	// 命令行指定文件上传检验处理
	for _, fd := range slices.Compact(input) {
		// 处理自动展开波浪号 ~/file
		file, err := utils.ExpandTilde(fd)
		if err != nil {
			return fmt.Errorf("ExpandTilde: %w", err)
		}
		exist, isDir, err := utils.CheckPath(fd)
		if err != nil {
			return fmt.Errorf("CheckPath: %w", err)
		}
		if !exist {
			return fmt.Errorf("%s not found", file)
		}
		// 文件处理
		if !isDir {
			if ok := utils.IsMusicExt(file); !ok {
				return fmt.Errorf("%s is not music file", file)
			}
			stat, err := os.Stat(file)
			if err != nil {
				return fmt.Errorf("%s stat: %w", file, err)
			}
			if stat.Size() > maxSize {
				c.cmd.Printf("%s file size too large. limit %vMB", file, maxSize)
				skip.Add(1)
				continue
			}
			if stat.Size() <= 0 || (minSize != nil && stat.Size() < *minSize) {
				c.cmd.Printf("%s file size too samll %vKB", file, stat.Size())
				skip.Add(1)
				continue
			}
			if reg != nil {
				if reg.MatchString(file) {
					barSize += stat.Size()
					fileList = append(fileList, file)
					continue
				}
				skip.Add(1)
				c.cmd.Printf("%s file name does not match the regular expression %s", file, c.opts.Regexp)
			} else {
				barSize += stat.Size()
				fileList = append(fileList, file)
				continue
			}
		}
		// 目录处理
		if err := fs.WalkDir(os.DirFS(file), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				if depth := len(filepath.SplitList(path)); depth > 3 {
					return fmt.Errorf("maximum supported directory depth is 3: %s", path)
				}
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			var f = filepath.Join(file, path)
			if ok := utils.IsMusicExt(f); !ok {
				return nil
			}

			// 忽略大文件、小于0字节的文件以及用户配置忽略的最小文件大小
			if info.Size() > maxSize {
				c.cmd.Printf("%s file size too large. limit %vMB", file, maxSize)
				skip.Add(1)
				return nil
			}
			if info.Size() <= 0 || (minSize != nil && info.Size() < *minSize) {
				skip.Add(1)
				c.cmd.Printf("%s file size too samll %vKB", file, info.Size())
				return nil
			}

			if reg != nil {
				if reg.MatchString(file) {
					barSize += info.Size()
					fileList = append(fileList, file)
					return nil
				}
				skip.Add(1)
				c.cmd.Printf("%s file name does not match the regular expression %s", file, c.opts.Regexp)
				return nil
			} else {
				barSize += info.Size()
				fileList = append(fileList, f)
				return nil
			}
		}); err != nil {
			return fmt.Errorf("WalkDir: %w", err)
		}
	}

	fileList = slices.Compact(fileList)
	log.Debug("Ready to upload list: %v", fileList)
	var total = int64(len(fileList))
	defer func() {
		c.cmd.Printf("report total: %v success: %v failed: %v skip: %v\n",
			total, total-fail.Load(), fail.Load(), skip.Load())
	}()
	if total <= 0 {
		c.cmd.Printf("no input file or the file does not meet the upload conditions\n")
		return nil
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

	// 执行目录文件上传
	var (
		sema = semaphore.NewWeighted(c.opts.Parallel)
		bar  = pb.Full.Start64(barSize)
	)
	defer func() {
		bar.Finish()
	}()

	for _, v := range fileList {
		if err := sema.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("acquire: %w", err)
		}
		go func(filename string) {
			defer sema.Release(1)
			if err := c.upload(ctx, request, filename, bar); err != nil {
				fail.Add(1)
				c.cmd.Printf("%s upload failed: %s", filepath.Base(filename), err)
				log.Error("upload(%s): %s", filename, err)
			}
		}(v)
	}
	if err := sema.Acquire(ctx, c.opts.Parallel); err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	return nil
}

func (c *Cloud) upload(ctx context.Context, client *weapi.Api, filename string, bar *pb.ProgressBar) error {
	// 1.读取文件
	var (
		ext     = filepath.Ext(filename)
		bitrate = "999000" // todo: 另外bitrate值有何影响？
	)

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Open: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Stat: %w", err)
	}
	var fileSize = stat.Size()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("ReadAll: %w", err)
	}

	md5, err := utils.MD5Hex(data)
	if err != nil {
		return fmt.Errorf("MD5Hex: %w", err)
	}

	// 重新设置文件指针到开头
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("Seek: %w", err)
	}

	// 2.检查此文件是否需要上传
	var checkReq = weapi.CloudUploadCheckReq{
		Bitrate: bitrate,
		Ext:     ext,
		Length:  fmt.Sprintf("%d", fileSize),
		Md5:     md5,
		SongId:  "0",
		Version: "1",
	}
	resp, err := client.CloudUploadCheck(ctx, &checkReq)
	if err != nil {
		return fmt.Errorf("CloudUploadCheck: %w", err)
	}
	log.Debug("CloudUploadCheck resp: %+v\n", resp)
	if resp.Code != 200 {
		return fmt.Errorf("CloudUploadCheck resp: %+v\n", resp)
	}

	// 3.获取上传凭证
	var allocReq = weapi.CloudTokenAllocReq{
		Bucket:     "", // jd-musicrep-privatecloud-audio-public
		Ext:        ext,
		Filename:   filepath.Base(filename),
		Local:      "false",
		NosProduct: "3",
		Type:       "audio",
		Md5:        md5,
	}
	allocResp, err := client.CloudTokenAlloc(ctx, &allocReq)
	if err != nil {
		return fmt.Errorf("CloudTokenAlloc: %w", err)
	}
	log.Debug("CloudTokenAlloc resp: %+v\n", allocResp)
	if allocResp.Code != 200 {
		return fmt.Errorf("CloudTokenAlloc resp: %+v\n", allocResp)
	}

	// 4.上传文件
	if resp.NeedUpload {
		log.Info("[%s] need upload", filename)
		var uploadReq = weapi.CloudUploadReq{
			Bucket:      allocResp.Bucket,
			ObjectKey:   allocResp.ObjectKey,
			Token:       allocResp.Token,
			Filepath:    filename,
			ProgressBar: bar,
		}
		uploadResp, err := client.CloudUpload(ctx, &uploadReq)
		if err != nil {
			return fmt.Errorf("CloudUpload: %w", err)
		}
		log.Debug("CloudUpload resp: %+v\n", uploadResp)
		if uploadResp.ErrCode != "" {
			return fmt.Errorf("CloudUpload resp: %+v\n", uploadResp)
		}
	}

	// 5.上传歌曲相关信息
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return fmt.Errorf("ReadFrom: %w", err)
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
		// CoverId:    "",
		// ObjectKey: allocResp.ObjectKey, // 不能穿入此值不然会报告 {"msg":"rep create failed","code":404}
	}
	log.Debug("CloudInfo req: %+v", InfoReq)
	infoResp, err := client.CloudInfo(ctx, &InfoReq)
	if err != nil {
		return fmt.Errorf("CloudInfo: %w", err)
	}
	log.Debug("CloudInfo resp: %+v\n", infoResp)
	if infoResp.Code != 200 {
		return fmt.Errorf("CloudInfo: %+v", infoResp.RespCommon)
	}

	// todo: 此步骤貌似是判断上传文件转码状态,具体有待商榷,另外此处貌似不用进行重试处理？
	var retryNum int64
retry:
	retryNum++
	if retryNum > 3 {
		return fmt.Errorf("CloudInfo retry too many times")
	}
	songId, _ := strconv.ParseInt(infoResp.SongId, 10, 64)
	statusResp, err := client.CloudMusicStatus(ctx, &weapi.CloudMusicStatusReq{SongIds: []int64{songId}})
	if err != nil {
		return fmt.Errorf("CloudMusicStatus: %w", err)
	}
	log.Debug("CloudMusicStatus #%v resp: %+v\n", retryNum, statusResp)
	if statusResp.Code != 200 {
		log.Error("CloudMusicStatus #%v resp: %+v\n", retryNum, statusResp)
	}
	// v.Status=9得条件下出现过云盘上传成功的情况,即使不走下面的CloudPublish逻辑,目前暂时未找到原因
	if v, ok := statusResp.Statuses[infoResp.SongId]; ok && v.Status != 0 {
		log.Warn("CloudMusicStatus status: %v retry #%v\n", statusResp.Statuses, retryNum)
		time.Sleep(time.Second * 30)
		goto retry
	}

	// 6.对上传得歌曲进行发布，和自己账户做关联,不然云盘列表看不到上传得歌曲信息
	publishResp, err := client.CloudPublish(ctx, &weapi.CloudPublishReq{SongId: infoResp.SongId})
	if err != nil {
		return fmt.Errorf("CloudPublish: %w", err)
	}
	log.Debug("CloudPublish resp: %+v\n", publishResp)
	switch publishResp.Code {
	case 200:
		if !resp.NeedUpload {
			bar.Add64(fileSize)
		}
		log.Debug("上传成功: %s", filename)
	case 201:
		if !resp.NeedUpload {
			bar.Add64(fileSize)
		}
		log.Debug("重复上传: %s", filename)
	default:
		return fmt.Errorf("CloudPublish: %+v", publishResp)
	}
	return nil
}

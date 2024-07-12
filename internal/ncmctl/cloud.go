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
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
	Input    string // 加载文件路径
	Parallel int64  // 并发上传文件数量
	MinSize  string // 上传文件最低大小限制
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
			Example: "  ncmctl cloud -h\n  ncmctl cloud ./mymusic.mp3\n  ncmctl cloud -i ./my/music/",
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
	c.cmd.PersistentFlags().StringVarP(&c.opts.Input, "input", "i", "", "music file path")
	c.cmd.PersistentFlags().Int64VarP(&c.opts.Parallel, "parallel", "p", 3, "concurrent upload count")
	c.cmd.PersistentFlags().StringVarP(&c.opts.MinSize, "minsize", "m", "500kb", "upload music minimum file size limit. supporting unit:b、k/kb/KB、m/mb/MB")
}

func (c *Cloud) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Cloud) Command() *cobra.Command {
	return c.cmd
}

func (c *Cloud) execute(ctx context.Context, fileList []string) error {
	if c.opts.Parallel < 0 || c.opts.Parallel > 10 {
		return fmt.Errorf("parallel must be between 1 and 10")
	}

	var barSize int64

	// 命令行指定文件上传检验处理
	for _, file := range fileList {
		exist, isDir, err := utils.CheckPath(file)
		if err != nil {
			return fmt.Errorf("CheckPath: %w", err)
		}
		if !exist {
			return fmt.Errorf("%s not found", file)
		}
		if isDir {
			return fmt.Errorf("%s is directory need file", file)
		}
		if ok := utils.IsMusicExt(file); !ok {
			return fmt.Errorf("%s is not music file", file)
		}
		stat, err := os.Stat(file)
		if err != nil {
			return fmt.Errorf("%s stat: %w", file, err)
		}
		if stat.Size() > maxSize {
			return fmt.Errorf("%s file size too large.limit %vkb", file, maxSize)
		}
		if stat.Size() <= 0 {
			return fmt.Errorf("%s file size too small", file)
		}
		barSize += stat.Size()
	}

	// 目录上传检验处理
	if c.opts.Input != "" {
		minSize, err := utils.ParseBytes(c.opts.MinSize)
		if err != nil {
			return fmt.Errorf("bytesize.Parse: %w", err)
		}

		if err := fs.WalkDir(os.DirFS(c.opts.Input), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			var file = filepath.Join(c.opts.Input, path)
			if ok := utils.IsMusicExt(file); !ok {
				return nil
			}

			// 忽略大于300M的文件、小于0字节的文件以及用户配置得忽略的最小文件大小
			if info.Size() > maxSize || info.Size() <= 0 || info.Size() < minSize {
				return nil
			}

			barSize += info.Size()
			fileList = append(fileList, file)
			return nil
		}); err != nil {
			return fmt.Errorf("WalkDir: %w", err)
		}
	}

	if len(fileList) <= 0 {
		return errors.New("no input file or the file does not meet the upload conditions")
	}
	fileList = slices.Compact(fileList)
	log.Debug("Ready to upload list: %v", fileList)

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

	// 执行目录文件上传
	var (
		total = int64(len(fileList))
		fail  atomic.Int64
		sema  = semaphore.NewWeighted(c.opts.Parallel)
		bar   = pb.Full.Start64(barSize)
	)
	defer func() {
		bar.Finish()
		c.cmd.Printf("report total: %v success: %v failed: %v\n", total, total-fail.Load(), fail.Load())
	}()

	for _, v := range fileList {
		if err := sema.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("acquire: %w", err)
		}
		go func(filename string) {
			defer sema.Release(1)
			if err := c.upload(ctx, request, filename, bar); err != nil {
				fail.Add(1)
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
		return fmt.Errorf("CloudInfo: %+v", infoResp)
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

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
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/ncm"
	"github.com/chaunsin/netease-cloud-music/pkg/ncm/tag"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"golang.org/x/sync/semaphore"
)

type NCMOpts struct {
	Output   string // 生成文件路径
	Parallel int64
	Tag      bool
}

type NCM struct {
	root *Root
	cmd  *cobra.Command
	opts NCMOpts
	l    *log.Logger
}

func NewNCM(root *Root, l *log.Logger) *NCM {
	c := &NCM{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "ncm",
			Short:   "Automatically parses xxx.ncm to .mp3/.flac",
			Example: "  ncmctl /music/Hello - Adele.ncm -o ./ncm\n  ncmctl /music/dir/ -o ./ncm (Use directory)",
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}
	return c
}

func (c *NCM) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "./ncm", "output music dir")
	c.cmd.PersistentFlags().Int64VarP(&c.opts.Parallel, "parallel", "p", 10, "concurrent decrypt count")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Tag, "tag", false, "disable set a music tag info")
}

func (c *NCM) validate() error {
	if c.opts.Parallel > 50 || c.opts.Parallel < 1 {
		return fmt.Errorf("parallel must be between 1 and 50")
	}
	return nil
}

func (c *NCM) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *NCM) Command() *cobra.Command {
	return c.cmd
}

func (c *NCM) execute(ctx context.Context, input []string) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	if len(input) <= 0 {
		c.cmd.Println("nothing was entered")
		return nil
	}
	var fileList = make([]string, 0, len(input))

	// 处理命令行输入的内容
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
			c.cmd.Printf("%s not found\n", fd)
			return nil
		}

		// 文件
		if !isDir {
			if filepath.Ext(file) != ".ncm" {
				return fmt.Errorf("%s is not .ncm file", file)
			}
			fileList = append(fileList, file)
			continue
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

			var f = filepath.Join(file, path)
			if filepath.Ext(f) != ".ncm" {
				return nil
			}
			fileList = append(fileList, f)
			return nil
		}); err != nil {
			return fmt.Errorf("WalkDir: %w", err)
		}
	}

	fileList = slices.Compact(fileList)
	if len(fileList) <= 0 {
		return errors.New("no input file or the file does not meet the conditions")
	}
	log.Debug("filelist: %+v", fileList)

	if err := os.MkdirAll(c.opts.Output, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var (
		sema = semaphore.NewWeighted(c.opts.Parallel)
		bar  = pb.Full.Start64(int64(len(fileList)))
	)
	defer bar.Finish()

	for _, f := range fileList {
		if err := sema.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("acquire: %w", err)
		}
		go func(file string) {
			defer func() {
				sema.Release(1)
				if x := recover(); x != nil {
					stack := string(debug.Stack())
					c.cmd.Printf("decode fail [%s]: %v, stack: %v\n", file, x, stack)
					log.Error("decode fail [%s]: %v, stack:%v", file, x, stack)
				}
			}()
			if err := c.decode(file); err != nil {
				c.cmd.Printf("decode[%s]: %v\n", file, err)
				log.Error("decode[%s]: %v", file, err)
				return
			}
			bar.Increment()
		}(f)
	}
	if err := sema.Acquire(ctx, c.opts.Parallel); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	return nil
}

func (c *NCM) decode(filename string) error {
	_ncm, err := ncm.Open(filename)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer _ncm.Close()

	var (
		meta   = _ncm.Metadata()
		format string
	)
	switch meta.GetType() {
	case ncm.MetadataTypeMusic:
		format = meta.GetMusic().Format
	case ncm.MetadataTypeDJ:
		format = meta.GetDJ().MainMusic.Format
	}

	var (
		_filename = filepath.Base(filename)
		ext       = filepath.Ext(_filename)
		name      = utils.Filename(strings.TrimSuffix(_filename, ext), "_")
		extend    = utils.Ternary(format == "", strings.TrimPrefix(ext, "."), format)
		dest      = filepath.Join(c.opts.Output, name+"."+extend)
	)

	if err := utils.MkdirIfNotExist(c.opts.Output, 0755); err != nil {
		return fmt.Errorf("MkdirIfNotExist: %w", err)
	}
	tmp, err := os.CreateTemp(c.opts.Output, fmt.Sprintf("ncm-*-%s.%s.tmp", name, extend))
	if err != nil {
		return fmt.Errorf("CreateTemp: %w", err)
	}
	defer tmp.Close()
	log.Debug("tempdir: %s", tmp.Name())

	if err := _ncm.DecodeMusic(tmp); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("DecodeMusic: %w", err)
	}

	// 设置歌曲tag相关信息
	if !c.opts.Tag {
		if err := tag.NewFromNCM(_ncm.NCM, tmp.Name()); err != nil {
			_ = os.Remove(tmp.Name())
			return fmt.Errorf("NewFromNCM: %w", err)
		}
	}

	for i := 1; utils.FileExists(dest); i++ {
		dest = filepath.Join(c.opts.Output, fmt.Sprintf("%s(%d).%s", name, i, extend))
	}
	// 显示关闭文件避免Windows系统无法重命名错误:The process cannot access the file because it is being used by another process
	if err := tmp.Close(); err != nil {
		log.Error("close %s file err: %s", tmp.Name(), err)
		_ = os.Remove(tmp.Name())
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("rename: %w", err)
	}

	if err := os.Chmod(dest, 0644); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	return nil
}

// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

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

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"golang.org/x/sync/semaphore"

	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/ncm"
	"github.com/chaunsin/netease-cloud-music/pkg/ncm/tag"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
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
			Use:   "ncm <input> [input...]",
			Short: "Decode .ncm files to .mp3/.flac",
			Long: "Decode one or more .ncm files or directories. Every positional argument is " +
				"treated as an input path; set the destination directory with --output. Directory scans " +
				"are limited to three levels. Audio tags are written by default.",
			Example: "  ncmctl ncm \"/music/Hello - Adele.ncm\" --output ./ncm\n" +
				"  ncmctl ncm /music/dir/ --output ./ncm\n" +
				"  ncmctl ncm first.ncm second.ncm --output ./ncm",
			Args: cobra.MinimumNArgs(1),
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}
	return c
}

func (c *NCM) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *NCM) Command() *cobra.Command {
	return c.cmd
}

func (c *NCM) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "./ncm", "directory for decoded music files")
	c.cmd.PersistentFlags().Int64VarP(&c.opts.Parallel, "parallel", "p", 10, "maximum concurrent decodes (1-50)")
	c.cmd.PersistentFlags().BoolVar(&c.opts.Tag, "tag", false, "disable audio tag writing (tags are written by default)")
}

func (c *NCM) validate() error {
	if c.opts.Parallel > 50 || c.opts.Parallel < 1 {
		return errors.New("parallel must be between 1 and 50")
	}
	return nil
}

func (c *NCM) execute(ctx context.Context, input []string) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	fileList, err := c.scanInputs(input)
	if err != nil {
		return err
	}

	if len(fileList) == 0 {
		return errors.New("no input file or the file does not meet the conditions")
	}

	log.Debugf("filelist: %+v", fileList)

	if err := os.MkdirAll(c.opts.Output, 0o755); err != nil { //nolint:gosec // User-selected media output is intentionally world-readable.
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
					log.Errorf("decode fail [%s]: %v, stack:%v", file, x, stack)
				}
			}()

			if err := c.decode(file); err != nil {
				c.cmd.Printf("decode[%s]: %v\n", file, err)
				log.Errorf("decode[%s]: %v", file, err)
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

func (c *NCM) scanInputs(input []string) ([]string, error) {
	if len(input) == 0 {
		return nil, errors.New("at least one input path is required")
	}

	fileList := make([]string, 0, len(input))

	// 处理命令行输入的内容
	for _, fd := range slices.Compact(input) {
		// 处理自动展开波浪号 ~/file
		file, err := utils.ExpandTilde(fd)
		if err != nil {
			return nil, fmt.Errorf("ExpandTilde: %w", err)
		}

		exist, isDir, err := utils.CheckPath(file)
		if err != nil {
			return nil, fmt.Errorf("CheckPath: %w", err)
		}

		if !exist {
			return nil, fmt.Errorf("input %q not found", fd)
		}

		// 文件
		if !isDir {
			if filepath.Ext(file) != ".ncm" {
				return nil, fmt.Errorf("%s is not .ncm file", file)
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
				if depth := relativePathDepth(path); depth > 3 {
					return fmt.Errorf(
						"maximum supported input directory depth is 3 at %q; all positional arguments are input paths, so pass a destination with --output instead",
						path,
					)
				}
				return nil
			}

			f := filepath.Join(file, path)
			if filepath.Ext(f) != ".ncm" {
				return nil
			}

			fileList = append(fileList, f)
			return nil
		}); err != nil {
			return nil, fmt.Errorf("scan input %q: %w", fd, err)
		}
	}

	fileList = slices.Compact(fileList)
	return fileList, nil
}

func (c *NCM) decode(filename string) error {
	_ncm, err := ncm.Open(filename)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() {
		if closeErr := _ncm.Close(); closeErr != nil {
			log.Errorf("close ncm file: %v", closeErr)
		}
	}()

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

	if mkdirErr := utils.MkdirIfNotExist(c.opts.Output, 0o755); mkdirErr != nil {
		return fmt.Errorf("MkdirIfNotExist: %w", mkdirErr)
	}

	tmp, err := os.CreateTemp(c.opts.Output, fmt.Sprintf("ncm-*-%s.%s.tmp", name, extend))
	if err != nil {
		return fmt.Errorf("CreateTemp: %w", err)
	}
	defer tmp.Close()

	log.Debugf("tempdir: %s", tmp.Name())

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
		log.Errorf("close %s file err: %s", tmp.Name(), err)
		_ = os.Remove(tmp.Name())
	}

	if err := os.Rename(tmp.Name(), dest); err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("rename: %w", err)
	}

	if err := os.Chmod(dest, 0o644); err != nil { //nolint:gosec // Decrypted media keeps conventional read permissions.
		return fmt.Errorf("chmod: %w", err)
	}
	return nil
}

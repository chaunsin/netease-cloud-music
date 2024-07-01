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

	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/ncm"
	"github.com/chaunsin/netease-cloud-music/pkg/ncm/tag"
	"github.com/spf13/cobra"
)

type NCMOpts struct {
	Input    string // 加载文件路径,或文本内容
	Output   string // 生成文件路径
	Parallel int64
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
			Short:   "NCM used to parse.ncm music files",
			Example: `  ncm -h`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context(), args)
	}
	return c
}

func (c *NCM) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Input, "input", "i", "./", "input music dir")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "./", "output music dir")
	c.cmd.PersistentFlags().Int64VarP(&c.opts.Parallel, "parallel", "p", 10, "concurrent decrypt count")
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

func (c *NCM) execute(ctx context.Context, args []string) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	c.root.Cfg.Network.Debug = false
	if c.root.Opts.Debug {
		c.root.Cfg.Network.Debug = true
	}

	var (
		barSize  int64
		fileList []string
	)

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
		if filepath.Ext(file) != ".ncm" {
			return nil
		}

		barSize += info.Size()
		fileList = append(fileList, file)
		return nil
	}); err != nil {
		return fmt.Errorf("WalkDir: %w", err)
	}

	if len(fileList) <= 0 {
		return errors.New("no input file or the file does not meet the conditions")
	}
	log.Debug("filelist:%+v", fileList)

	for _, f := range fileList {
		n, err := ncm.Open(f)
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}

		// todo: fix file ext
		name := filepath.Base(f)
		var ext = n.Metadata().Format
		if n.Metadata().Format == "" {
			ext = "mp3"
		}

		p := filepath.Join(c.opts.Output, name+"."+ext)

		if err := os.WriteFile(p, n.Music(), 0644); err != nil {
			return fmt.Errorf("writeFile: %w", err)
		}

		if err := tag.NewFromNCM(n, p); err != nil {
			return fmt.Errorf("NewFromNCM: %w", err)
		}
	}

	return nil
}

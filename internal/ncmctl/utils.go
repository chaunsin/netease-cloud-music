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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/spf13/cobra"
)

func writeFile(cmd *cobra.Command, out string, data []byte) error {
	if out == "" {
		cmd.Println(string(data))
		return nil
	}

	// 写入文件
	var file string
	if !filepath.IsAbs(out) {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		file = filepath.Join(wd, out)
		if !utils.DirExists(file) {
			if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
				return fmt.Errorf("MkdirAll: %w", err)
			}
		}
	}
	if err := os.WriteFile(file, data, os.ModePerm); err != nil {
		return fmt.Errorf("WriteFile: %w", err)
	}
	cmd.Printf("generate file path: %s\n", file)
	return nil
}

var (
	urlPattern = "/(song|artist|album|playlist)\\?id=(\\d+)"
	reg        = regexp.MustCompile(urlPattern)
)

func ParseUrl(url string) (string, int64, error) {
	matched, ok := reg.FindStringSubmatch(url), reg.MatchString(url)
	if !ok || len(matched) < 3 {
		return "", 0, fmt.Errorf("could not parse the url: %s", url)
	}

	id, err := strconv.ParseInt(matched[2], 10, 64)
	if err != nil {
		return "", 0, err
	}

	switch matched[1] {
	case "song":
	case "artist":
	case "album":
	case "playlist":
	}
	return matched[1], id, nil
}

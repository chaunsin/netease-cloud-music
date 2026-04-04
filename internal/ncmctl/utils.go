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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/pkg/cookiecloud"
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

func Parse(source string) (string, int64, error) {
	// 歌曲id
	id, err := strconv.ParseInt(source, 10, 64)
	if err == nil {
		return "song", id, nil
	}

	if !strings.Contains(source, "music.163.com") {
		return "", 0, fmt.Errorf("could not parse the url: %s", source)
	}

	matched, ok := reg.FindStringSubmatch(source), reg.MatchString(source)
	if !ok || len(matched) < 3 {
		return "", 0, fmt.Errorf("could not parse the url: %s", source)
	}

	id, err = strconv.ParseInt(matched[2], 10, 64)
	if err != nil {
		return "", 0, err
	}
	return matched[1], id, nil
}

// IsPrint returns whether s is ASCII and printable according to
// https://tools.ietf.org/html/rfc20#section-4.2.
func isPrint(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < ' ' || s[i] > '~' {
			return false
		}
	}
	return true
}

// ToLower returns the lowercase version of s if s is ASCII and printable.
func toLower(s string) (lower string, ok bool) {
	if !isPrint(s) {
		return "", false
	}
	return strings.ToLower(s), true
}

func sameSite(val string) http.SameSite {
	lowerVal, ascii := toLower(val)
	if !ascii {
		return http.SameSiteDefaultMode
	}
	switch lowerVal {
	case "strict":
		return http.SameSiteStrictMode
	case "lax":
		return http.SameSiteLaxMode
	case "none":
		return http.SameSiteNoneMode
	case "unspecified": // is means http.SameSiteDefaultMode or http.SameSiteNoneMode ?
		return http.SameSiteDefaultMode
	default:
		return http.SameSiteDefaultMode
	}
}

// ParseCookeJson 解析cookie.json文件
func ParseCookeJson(r io.Reader) ([]*http.Cookie, error) {
	var (
		temp    []cookiecloud.CookieData
		cookies []*http.Cookie
	)
	if err := json.NewDecoder(r).Decode(&temp); err != nil {
		return nil, fmt.Errorf("could not read cookies: %+v", err)
	}
	for _, v := range temp {
		cookies = append(cookies, &http.Cookie{
			Domain:   v.Domain,
			Expires:  v.GetExpired(),
			HttpOnly: v.HttpOnly,
			Name:     v.Name,
			Path:     v.Path,
			Secure:   v.Secure,
			Value:    v.Value,
			SameSite: sameSite(v.SameSite),
			// Quoted:   false,
		})
	}
	return cookies, nil
}

type Music struct {
	Id      int64
	Name    string
	Artist  []types.Artist
	Album   types.Album
	AlbumId int64
	Time    int64
}

// NameString 返回去除特殊符号的歌曲名
func (m Music) NameString() string {
	return utils.Filename(m.Name, "_")
}

func (m Music) ArtistString() string {
	if len(m.Artist) <= 0 {
		return ""
	}
	var artistList = make([]string, 0, len(m.Artist))
	for _, ar := range m.Artist {
		artistList = append(artistList, utils.Filename(ar.Name, "_")) // #11 避免文件名中包含特殊字符
	}
	return strings.Join(artistList, ",")
}

func (m Music) String() string {
	var (
		seconds = m.Time / 1000 // 毫秒换成秒
		hours   = seconds / 3600
		minutes = (seconds % 3600) / 60
		secs    = seconds % 60
		format  = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
	)
	return fmt.Sprintf("%s-%s(%v) [%s]", m.ArtistString(), m.Name, m.Id, format)
}

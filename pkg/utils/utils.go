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

package utils

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	B int64 = 1 << (10 * iota)
	KB
	MB
	GB
	TB
	PB
)

var (
	parseBytesRegexp = regexp.MustCompile(`(?i)^(\d+)([a-zA-Z]*)$`)
	filenameRegexp   = regexp.MustCompile("[\\\\/:*?\"<>|]")
	unitMap          = map[string]int64{
		"B":  B,
		"K":  KB,
		"KB": KB,
		"M":  MB,
		"MB": MB,
	}
)

// ParseBytes 将输入字符串转换为字节数
func ParseBytes(input string) (int64, error) {
	if input == "" {
		return 0, nil
	}

	matches := parseBytesRegexp.FindStringSubmatch(input)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid input format: %s", input)
	}

	valueStr := matches[1]
	unit := matches[2]

	// 转换数字部分
	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", valueStr)
	}

	// 默认单位是字节
	if unit == "" {
		unit = "B"
	}

	// 将单位转换为小写
	unit = strings.ToUpper(unit)

	// 获取对应的字节数乘数
	multiplier, exists := unitMap[unit]
	if !exists {
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
	return value * multiplier, nil
}

// FileExists 判断文件是否存在
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

// DirExists 判断目录是否存在
func DirExists(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}

// IsFile 判断是否为文件
func IsFile(path string) bool {
	d, err := os.Stat(path)
	if err != nil {
		return false
	}
	if d.IsDir() {
		return false
	}
	return true
}

func MkdirIfNotExist(path string, perm os.FileMode) error {
	if !DirExists(path) {
		return os.MkdirAll(path, perm)
	}
	return nil
}

// ExpandTilde 扩展波浪号路径
func ExpandTilde(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("UserHomeDir: %w", err)
	}

	// 处理纯 ~ 或 ~/ 路径
	if path == "~" || (len(path) >= 2 && (path[1] == '/' || path[1] == '\\')) {
		return filepath.Join(home, path[1:]), nil
	}

	// todo:
	// ~file 分为两种情况:
	// 1：如果 file 是用户名，则指向该用户的主目录。
	// 例如：~bob 表示用户 bob 的主目录（如 /home/bob）。
	// 2：如果 file 不是用户名，则可能被解析为字面量（即一个实际的文件名）。
	// 例如：~file.txt 可能被误认为是用户 file.txt 的主目录，但更常见的是被当作普通文件名处理（需转义或引号包裹）。
	return path, nil
}

// CheckPath 检查路径是否存在，并返回是否为目录, 支持~路径检测
func CheckPath(path string) (exists bool, isDir bool, err error) {
	expandedPath, err := ExpandTilde(path)
	if err != nil {
		return false, false, fmt.Errorf("ExpandTilde: %w", err)
	}

	stat, err := os.Stat(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil // Path does not exist
		}
		return false, false, err // Some other error occurred
	}
	// Path exists, determine if it is a directory
	return true, stat.IsDir(), nil
}

func MD5Hex(data []byte) (string, error) {
	var m = md5.New()
	_, err := m.Write(data)
	return hex.EncodeToString(m.Sum(nil)), err
}

// Ternary is a generic function that mimics a ternary expression
func Ternary[T any](condition bool, trueVal, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}

func IsUnique[T comparable](arr []T) bool {
	var set = make(map[T]struct{})
	for _, v := range arr {
		if _, ok := set[v]; ok {
			return false
		}
		set[v] = struct{}{}
	}
	return true
}

var musicExts = map[string]struct{}{
	".mp3":  {},
	".flac": {},
	".wav":  {},
	".m4a":  {},
	".ogg":  {},
	".ape":  {},
	".wma":  {},
	".aac":  {},
	".aiff": {},
	".ac3":  {},
	".dts":  {},
	".wv":   {},
	".mpc":  {},
	".opus": {},
	".mka":  {},
	".m3u":  {},
	".m3u8": {},
	".pls":  {},
}

func IsMusicExt(ext string) bool {
	_, exist := musicExts[filepath.Ext(ext)]
	return exist
}

func DetectContentType(data []byte, ext string) string {
	if ext == ".flac" {
		return "audio/flac"
	}

	var (
		ct = http.DetectContentType(data)
		k  = strings.SplitN(ct, "/", 1)
	)
	if len(k) > 0 && k[0] != "audio" {
		ct = "audio/mpeg"
	}
	return ct
}

func SplitSlice[T any](input []T, chunkSize int) ([][]T, error) {
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunkSize must be greater than 0")
	}

	var result [][]T
	for i := 0; i < len(input); i += chunkSize {
		end := i + chunkSize
		if end > len(input) {
			end = len(input)
		}
		result = append(result, input[i:end])
	}
	return result, nil
}

// TimeUntilMidnight 计算当前时间到明天零点的时间差
func TimeUntilMidnight(timeZone string) (time.Duration, error) {
	var (
		loc *time.Location
		err error
	)

	// Get the time zone location
	if timeZone == "" {
		loc = time.Local
	} else {
		loc, err = time.LoadLocation(timeZone)
		if err != nil {
			return 0, fmt.Errorf("invalid time zone: %v", err)
		}
	}
	now := time.Now().In(loc)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
	return midnight.Sub(now), nil
}

// Filename 清理文件名中的非法字符
func Filename(path string, new ...string) string {
	path = strings.TrimSpace(path)
	if len(new) > 0 {
		return filenameRegexp.ReplaceAllString(path, new[0])
	}
	return filenameRegexp.ReplaceAllString(path, "")
}

// IsGzipHeader 判断字节数据是否以 Gzip 文件头开头
// Gzip 文件头特征：前 2 个字节为 0x1F 0x8B，第三个字节为压缩方法（通常 0x08 表示 DEFLATE）
func IsGzipHeader(data []byte) bool {
	// Gzip 最小头长度检查
	if len(data) < 3 {
		return false
	}

	// 魔术字节校验
	magicBytes := []byte{0x1F, 0x8B}
	if !bytes.Equal(data[:2], magicBytes) {
		return false
	}

	// 压缩方法校验（0x08 = DEFLATE）
	return data[2] == 0x08
}

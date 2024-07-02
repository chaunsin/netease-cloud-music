package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

func CheckPath(path string) (exists bool, isDir bool, err error) {
	stat, err := os.Stat(path)
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
	".mp3":  struct{}{},
	".flac": struct{}{},
	".wav":  struct{}{},
	".m4a":  struct{}{},
	".ogg":  struct{}{},
	".ape":  struct{}{},
	".wma":  struct{}{},
	".aac":  struct{}{},
	".aiff": struct{}{},
	".ac3":  struct{}{},
	".dts":  struct{}{},
	".wv":   struct{}{},
	".mpc":  struct{}{},
	".opus": struct{}{},
	".mka":  struct{}{},
	".m3u":  struct{}{},
	".m3u8": struct{}{},
	".pls":  struct{}{},
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

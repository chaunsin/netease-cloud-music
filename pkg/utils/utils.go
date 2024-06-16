package utils

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
)

const (
	B int64 = 1 << (10 * iota)
	KB
	MB
	GB
	TB
	PB
)

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func IsFile(path string) bool {
	d, err := os.Stat(path)
	if err != nil {
		return os.IsNotExist(err)
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

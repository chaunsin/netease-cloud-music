package utils

import (
	"crypto/md5"
	"encoding/hex"
	"os"
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

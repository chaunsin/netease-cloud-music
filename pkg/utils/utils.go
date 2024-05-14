package utils

import (
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

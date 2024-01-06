package utils

import "os"

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

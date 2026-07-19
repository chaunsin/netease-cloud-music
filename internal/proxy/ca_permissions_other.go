// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

//go:build !windows

package proxy

import (
	"fmt"
	"os"
)

func secureCAPrivateKey(path string) error {
	return os.Chmod(path, 0o600)
}

func secureCAPrivateDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect CA directory %q: %w", path, err)
	}

	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("CA directory %q permissions are too broad: %04o", path, info.Mode().Perm())
	}
	return nil
}

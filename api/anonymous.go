// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

type Anonymous struct {
	token     string
	storePath string
	mux       sync.RWMutex
}

// NewAnonymous 匿名token管理器。
func NewAnonymous(storepath string) *Anonymous {
	return &Anonymous{storePath: storepath}
}

func (a *Anonymous) Get() string {
	a.mux.RLock()
	t := a.token
	a.mux.RUnlock()
	return t
}

func (a *Anonymous) Set(token string) {
	a.mux.Lock()
	a.token = token
	a.mux.Unlock()
}

func (a *Anonymous) LoadConfig() error {
	if a.storePath == "" {
		return errors.New("anoymous file is empty. ")
	}

	data, err := os.ReadFile(a.storePath)
	if err != nil {
		return fmt.Errorf("read anoymous token err: %w", err)
	}

	a.mux.Lock()
	a.token = strings.TrimSpace(string(data))
	a.mux.Unlock()

	return nil
}

func (a *Anonymous) Sync() error {
	a.mux.RLock()
	token := a.token
	a.mux.RUnlock()

	if token == "" {
		return nil
	}

	file := a.storePath
	if file == "" {
		file = utils.BaseDir("anonymous_token")
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return fmt.Errorf("MkdirAll: %w", err)
	}

	if err := os.WriteFile(file, []byte(token), 0o600); err != nil {
		return fmt.Errorf("anonymous write %s err: %w", file, err)
	}
	return nil
}

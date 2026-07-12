// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package database

import (
	"context"
	"fmt"
	"time"

	"github.com/chaunsin/netease-cloud-music/pkg/database/badger"
)

type Database interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl ...time.Duration) error
	Exists(ctx context.Context, key string) (bool, error)
	Increment(ctx context.Context, key string, value int64, ttl ...time.Duration) (int64, error)
	Del(ctx context.Context, key string) error
	Close(ctx context.Context) error
}

type Config struct {
	Driver string
	Path   string
}

func New(cfg *Config) (Database, error) {
	var (
		db  Database
		err error
	)
	switch cfg.Driver {
	case "", "badger":
		db, err = badger.New(cfg.Path)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
	if err != nil {
		return nil, err
	}
	return db, nil
}

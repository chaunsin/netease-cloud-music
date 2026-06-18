// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package example

import (
	"context"
	"os"
	"testing"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

var (
	cli *api.Client
	ctx = context.TODO()
)

func TestMain(t *testing.M) {
	log.Default = log.New(&log.Config{
		Level:  "info",
		Stdout: true,
	})
	cfg := api.Config{
		Debug:   false,
		Timeout: 0,
		Retry:   0,
		Cookie: cookie.Config{
			Options:  nil,
			Filepath: "../testdata/cookie.json",
			Interval: 0,
		},
	}
	cli = api.New(&cfg)
	defer cli.Close(ctx)
	os.Exit(t.Run())
}

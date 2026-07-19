// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"context"
	"os"
	"testing"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

var (
	cli *Api
	ctx = context.TODO()
)

func TestMain(t *testing.M) {
	log.Default = log.New(&log.Config{
		Level:  "debug",
		Stdout: true,
	})

	cfg := api.Config{
		Debug:   true,
		Timeout: 0,
		Retry:   0,
		Cookie: cookie.Config{
			Options:  nil,
			Filepath: "../../testdata/cookie.json",
			Interval: 0,
		},
	}
	client := api.New(&cfg)
	cli = New(client)

	os.Exit(t.Run())
}

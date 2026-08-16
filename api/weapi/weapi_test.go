// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/api/internal/testutil"
	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

func newLiveWEAPI(t *testing.T) *Api {
	t.Helper()
	testutil.RequireLiveAPI(t)

	return newTestWEAPI(t, &api.Config{
		Debug: true,
		Cookie: cookie.Config{
			Filepath: "../../testdata/cookie.json",
		},
	})
}

func newOfflineWEAPI(t *testing.T) *Api {
	t.Helper()
	home := t.TempDir()

	return newTestWEAPI(t, &api.Config{
		Timeout: time.Second,
		HomeDir: home,
		Cookie: cookie.Config{
			Filepath: filepath.Join(home, "cookie.json"),
		},
	})
}

func newTestWEAPI(t *testing.T, cfg *api.Config) *Api {
	t.Helper()

	logger := log.New(&log.Config{Level: "error"})
	client, err := api.NewClient(cfg, logger)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, client.Close(context.Background()))
		require.NoError(t, logger.Close())
	})
	return New(client)
}

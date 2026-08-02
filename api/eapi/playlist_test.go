// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api/internal/testutil"
)

func TestPlaylist(t *testing.T) {
	testutil.RequireLiveAPI(t)

	req := PlaylistReq{
		Uid:    "1289504343",
		Offset: "",
		Limit:  "1",
	}
	got, err := cli.Playlist(ctx, &req)
	require.NoError(t, err)
	t.Logf("Playlist: %+v\n", got)
}

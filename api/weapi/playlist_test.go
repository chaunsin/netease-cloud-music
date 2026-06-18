// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlaylist(t *testing.T) {
	var req = PlaylistReq{
		Uid:    "1289504343",
		Offset: "",
		Limit:  "30",
	}
	got, err := cli.Playlist(ctx, &req)
	assert.NoError(t, err)
	t.Logf("Playlist: %+v\n", got)
}

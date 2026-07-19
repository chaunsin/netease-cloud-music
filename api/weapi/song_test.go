// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api/types"
)

func TestSongPlayer(t *testing.T) {
	got, err := cli.SongPlayer(ctx, &SongPlayerReq{Ids: types.IntsString{2115747785}, Br: "128000"})
	require.NoError(t, err)
	t.Logf("resp:%+v\n", got)
}

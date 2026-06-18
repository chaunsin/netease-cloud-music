// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"testing"

	"github.com/chaunsin/netease-cloud-music/api/types"

	"github.com/stretchr/testify/assert"
)

func TestSongPlayer(t *testing.T) {
	got, err := cli.SongPlayer(ctx, &SongPlayerReq{Ids: types.IntsString{2115747785}, Br: "128000"})
	assert.NoError(t, err)
	t.Logf("resp:%+v\n", got)
}

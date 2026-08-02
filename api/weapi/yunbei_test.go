// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/api/internal/testutil"
)

func TestYunBeiSign(t *testing.T) {
	testutil.RequireLiveAPI(t)

	req := YunBeiSignInReq{}
	got, err := cli.YunBeiSignIn(ctx, &req)
	require.NoError(t, err)
	t.Logf("YunBeiSignIn: %+v\n", got)
}

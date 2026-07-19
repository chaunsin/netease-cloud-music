// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestYunBeiSign(t *testing.T) {
	req := YunBeiSignInReq{}
	got, err := cli.YunBeiSignIn(ctx, &req)
	require.NoError(t, err)
	t.Logf("YunBeiSignIn: %+v\n", got)
}

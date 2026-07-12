// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package eapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYunBeiInSign(t *testing.T) {
	req := YunBeiSignInReq{
		Type: 1,
	}
	got, err := cli.YunBeiSignIn(ctx, &req)
	assert.NoError(t, err)
	t.Logf("YunBeiSignIn: %+v\n", got)
}

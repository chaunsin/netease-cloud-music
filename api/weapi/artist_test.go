// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package weapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtistSublistHandlesNilRequest(t *testing.T) {
	t.Parallel()

	api := newOfflineWEAPI(t)
	assert.NotPanics(t, func() {
		_, _ = api.ArtistSublist(context.Background(), nil)
	})
}

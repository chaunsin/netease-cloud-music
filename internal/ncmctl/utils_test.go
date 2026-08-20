// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRelativePathDepth(t *testing.T) {
	t.Parallel()

	tests := map[string]int{
		".":       0,
		"artist":  1,
		"a/b":     2,
		"a/b/c":   3,
		"a/b/c/d": 4,
	}

	for path, want := range tests {
		assert.Equal(t, want, relativePathDepth(path), path)
	}
}

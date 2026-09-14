// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindBetter(t *testing.T) {
	standard := &Quality{Br: 128000}
	lossless := &Quality{Br: 999000}

	tests := []struct {
		name        string
		qualities   Qualities
		level       Level
		wantQuality *Quality
		wantLevel   Level
		wantOK      bool
	}{
		{
			name:        "empty qualities returns nil quality",
			qualities:   Qualities{},
			level:       LevelLossless,
			wantQuality: nil,
			wantLevel:   LevelStandard,
			wantOK:      false,
		},
		{
			name:        "exact match",
			qualities:   Qualities{L: standard, Sq: lossless},
			level:       LevelLossless,
			wantQuality: lossless,
			wantLevel:   LevelLossless,
			wantOK:      true,
		},
		{
			name:        "degrade to lower quality",
			qualities:   Qualities{L: standard},
			level:       LevelLossless,
			wantQuality: standard,
			wantLevel:   LevelStandard,
			wantOK:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			quality, level, ok := tc.qualities.FindBetter(tc.level)
			assert.Equal(t, tc.wantQuality, quality)
			assert.Equal(t, tc.wantLevel, level)
			assert.Equal(t, tc.wantOK, ok)
		})
	}
}

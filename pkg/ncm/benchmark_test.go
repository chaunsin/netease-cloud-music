// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncm

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func Benchmark_Open(b *testing.B) {
	b.ReportAllocs()

	ncmName := "./testdata/BOE - 822.ncm"

	for b.Loop() {
		func() {
			file, err := Open(ncmName)

			require.NoError(b, err)
			defer func() {
				require.NoError(b, file.Close())
			}()

			require.NoError(b, file.DecodeCover(io.Discard))
			require.NoError(b, file.DecodeMusic(io.Discard))
			require.NoError(b, file.DecodeCover(io.Discard))
		}()
	}
}

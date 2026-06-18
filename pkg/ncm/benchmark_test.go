// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncm

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Benchmark_Open(b *testing.B) {
	b.ReportAllocs()
	var ncmName = "./testdata/BOE - 822.ncm"
	for i := 0; i < b.N; i++ {
		func() {
			file, err := Open(ncmName)
			defer file.Close()
			assert.NoError(b, err)
			assert.NoError(b, file.DecodeCover(io.Discard))
			assert.NoError(b, file.DecodeMusic(io.Discard))
			assert.NoError(b, file.DecodeCover(io.Discard))
		}()
	}
}

// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"math"
	"strings"
	"testing"
)

func TestReadLimitedAvoidsMaxInt64Overflow(t *testing.T) {
	data, truncated, err := readLimited(strings.NewReader("small body"), math.MaxInt64)
	if err != nil || truncated || string(data) != "small body" {
		t.Fatalf("readLimited(MaxInt64) = %q, %v, %v", data, truncated, err)
	}

	data, truncated, err = readLimited(strings.NewReader("three"), 2)
	if err != nil || !truncated || string(data) != "th" {
		t.Fatalf("readLimited(2) = %q, %v, %v", data, truncated, err)
	}
}

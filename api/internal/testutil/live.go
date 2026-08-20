// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package testutil

import (
	"os"
	"testing"
)

const liveTestEnv = "NCMCTL_RUN_LIVE_TESTS"

// RequireLiveAPI skips the caller unless live API tests are explicitly enabled.
func RequireLiveAPI(tb testing.TB) {
	tb.Helper()

	if os.Getenv(liveTestEnv) != "1" {
		tb.Skipf("set %s=1 to run tests against live NetEase APIs", liveTestEnv)
	}
}

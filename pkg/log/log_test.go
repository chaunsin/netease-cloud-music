// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package log

import (
	"fmt"
	"log/slog"
	"testing"
)

func init() {
	Default = New(nil)
}

func TestPrint(t *testing.T) {
	Debug("hello debug")
	Info("hello info:%s", "chaunsin")
	InfoW(fmt.Sprintf("hello info:%s", "chaunsin"), "sex", slog.StringValue("man"))

	Default.SetLevel(slog.LevelWarn)
	Info("can not print")
	Fatal("hello fatal")
}

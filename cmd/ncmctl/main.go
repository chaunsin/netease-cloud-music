// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/chaunsin/netease-cloud-music/internal/ncmctl"
)

var (
	Version   string
	Commit    string
	BuildTime string
)

func main() {
	c := ncmctl.New()
	c.Version(Version, BuildTime, Commit)
	c.Execute()
}

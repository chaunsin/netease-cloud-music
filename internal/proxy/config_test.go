// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateListenAddress(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateListenAddress("127.0.0.1:9000", false))
	require.NoError(t, validateListenAddress("[::1]:9000", false))
	require.Error(t, validateListenAddress(":9000", false))
	require.Error(t, validateListenAddress("127.0.0.1:0", false))
	require.Error(t, validateListenAddress("127.0.0.1:65536", false))
}

func TestNormalizeConfigCAPolicy(t *testing.T) {
	defaultConfig, err := normalizeConfig(Config{})
	require.NoError(t, err)
	require.True(t, defaultConfig.RequirePrivateCAPath)

	explicitConfig, err := normalizeConfig(Config{
		CACertPath: "testdata/ca.crt",
		CAKeyPath:  "testdata/ca.key",
	})
	require.NoError(t, err)
	require.False(t, explicitConfig.RequirePrivateCAPath)

	strictExplicitConfig, err := normalizeConfig(Config{
		CACertPath:           "testdata/ca.crt",
		CAKeyPath:            "testdata/ca.key",
		RequirePrivateCAPath: true,
	})
	require.NoError(t, err)
	require.True(t, strictExplicitConfig.RequirePrivateCAPath)
}

func TestNormalizeConfigRejectsBodyLimitWithoutTruncationSentinel(t *testing.T) {
	_, err := normalizeConfig(Config{MaxBodyBytes: math.MaxInt64})
	require.ErrorContains(t, err, "less than")

	config, err := normalizeConfig(Config{MaxBodyBytes: math.MaxInt64 - 1})
	require.NoError(t, err)
	require.Equal(t, int64(math.MaxInt64-1), config.MaxBodyBytes)
}

func TestIsLoopbackListenAddress(t *testing.T) {
	t.Parallel()

	require.True(t, isLoopbackListenAddress("127.0.0.1:9000"))
	require.True(t, isLoopbackListenAddress("[::1]:9000"))
	require.True(t, isLoopbackListenAddress("localhost:9000"))
	require.False(t, isLoopbackListenAddress("0.0.0.0:9000"))
}

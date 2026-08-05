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
	defaultConfig, err := normalizeConfig(&Config{})
	require.NoError(t, err)
	require.True(t, defaultConfig.RequirePrivateCAPath)

	explicitConfig, err := normalizeConfig(&Config{
		CACertPath: "testdata/ca.crt",
		CAKeyPath:  "testdata/ca.key",
	})
	require.NoError(t, err)
	require.False(t, explicitConfig.RequirePrivateCAPath)

	strictExplicitConfig, err := normalizeConfig(&Config{
		CACertPath:           "testdata/ca.crt",
		CAKeyPath:            "testdata/ca.key",
		RequirePrivateCAPath: true,
	})
	require.NoError(t, err)
	require.True(t, strictExplicitConfig.RequirePrivateCAPath)
}

func TestNormalizeConfigRejectsUnboundedBodyLimit(t *testing.T) {
	_, err := normalizeConfig(&Config{MaxBodyBytes: math.MaxInt64})
	require.ErrorContains(t, err, "less than")

	config, err := normalizeConfig(&Config{MaxBodyBytes: math.MaxInt64 - 1})
	require.NoError(t, err)
	require.Equal(t, int64(math.MaxInt64-1), config.MaxBodyBytes)
}

func TestNormalizeConfigValidatesAndCopiesXeapiSeeds(t *testing.T) {
	seeds := []XeapiSessionSeed{{
		ID: "session", Key: "0123456789abcdef", Source: XeapiSessionSourceCommandLine,
	}}
	config, err := normalizeConfig(&Config{XeapiSessions: seeds})
	require.NoError(t, err)

	seeds[0].Key = "changed-by-caller"
	require.Equal(t, "0123456789abcdef", config.XeapiSessions[0].Key)

	_, err = normalizeConfig(&Config{XeapiSessions: []XeapiSessionSeed{{
		ID: "session", Key: "short", Source: XeapiSessionSourceCommandLine,
	}}})
	require.ErrorContains(t, err, "session key length is invalid")

	_, err = normalizeConfig(&Config{XeapiSessions: []XeapiSessionSeed{{
		ID: "session", Key: "0123456789abcdef",
	}}})
	require.ErrorContains(t, err, "unknown source")
}

func TestIsLoopbackListenAddress(t *testing.T) {
	t.Parallel()

	require.True(t, isLoopbackListenAddress("127.0.0.1:9000"))
	require.True(t, isLoopbackListenAddress("[::1]:9000"))
	require.True(t, isLoopbackListenAddress("localhost:9000"))
	require.False(t, isLoopbackListenAddress("0.0.0.0:9000"))
}

// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestConfig(t *testing.T, extra string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `version: "1.0"
log:
  app: ncm-test
  format: text
  level: info
  stdout: false
  rotate:
    filename: "${HOME}/logs/ncm.log"
    maxsize: 10
    maxage: 2
    maxbackups: 1
    localtime: true
    compress: false
network:
  debug: false
  timeout: 15s
  retry: 2
  cookie:
    filepath: "${HOME}/cookies/cookie.json"
    interval: 5s
database:
  driver: badger
  path: "${HOME}/database/"
` + extra

	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestNewLoadsExactConfigAndEnvironment(t *testing.T) {
	path := writeTestConfig(t, "")
	t.Setenv("NCMCTL_LOG_LEVEL", "debug")
	t.Setenv("NCMCTL_NETWORK_TIMEOUT", "30s")

	cfg, err := New(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.Log)
	require.NotNil(t, cfg.Network)
	require.NotNil(t, cfg.Database)

	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, 30*time.Second, cfg.Network.Timeout)
	assert.Equal(t, 2, cfg.Network.Retry)
	assert.Equal(t, 5*time.Second, cfg.Network.Cookie.Interval)
	assert.Equal(t, "badger", cfg.Database.Driver)
}

func TestNewLoadsRepositoryConfig(t *testing.T) {
	t.Parallel()

	cfg, err := New("config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "1.0", cfg.Version)
	assert.Equal(t, time.Minute, cfg.Network.Timeout)
}

func TestNewRejectsUnknownFields(t *testing.T) {
	path := writeTestConfig(t, "unknown: true\n")

	_, err := New(path)
	require.ErrorContains(t, err, "invalid keys")
}

func TestNewRejectsMissingSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.yaml")
	require.NoError(t, os.WriteFile(path, []byte("version: \"1.0\"\n"), 0o600))

	_, err := New(path)
	require.ErrorContains(t, err, "log config is required")
}

func TestNewRejectsInvalidLogConfig(t *testing.T) {
	tests := []struct {
		name      string
		envKey    string
		envValue  string
		wantError string
	}{
		{name: "format", envKey: "NCMCTL_LOG_FORMAT", envValue: "yaml", wantError: `unsupported log format "yaml"`},
		{name: "level", envKey: "NCMCTL_LOG_LEVEL", envValue: "verbose", wantError: `unsupported log level "verbose"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envValue)

			_, err := New(writeTestConfig(t, ""))
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}

func TestReplaceMagicVariablesUsesRuntimeHome(t *testing.T) {
	cfg, err := New(writeTestConfig(t, ""))
	require.NoError(t, err)

	home := filepath.Join(t.TempDir(), "runtime")
	_, replaced := cfg.ReplaceMagicVariables("HOME", home)

	assert.True(t, replaced)
	assert.Equal(t, filepath.Join(home, "logs", "ncm.log"), cfg.Log.Rotate.Filename)
	assert.Equal(t, filepath.Join(home, "cookies", "cookie.json"), cfg.Network.Cookie.Filepath)
	assert.Equal(t, filepath.Join(home, "database"), filepath.Clean(cfg.Database.Path))
}

// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	proxyserver "github.com/chaunsin/netease-cloud-music/internal/proxy"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

func TestProxyValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*ProxyOpts)
		wantBytes int64
		wantErr   bool
	}{
		{name: "defaults"},
		{name: "hostname", mutate: func(o *ProxyOpts) { o.ListenAddr = "localhost:8080" }},
		{name: "ipv6", mutate: func(o *ProxyOpts) { o.ListenAddr = "[::1]:8080" }},
		{name: "custom max body", mutate: func(o *ProxyOpts) { o.MaxBody = "2KB" }, wantBytes: 2 * utils.KB},
		{name: "missing host", mutate: func(o *ProxyOpts) { o.ListenAddr = ":8080" }, wantErr: true},
		{name: "missing port", mutate: func(o *ProxyOpts) { o.ListenAddr = "localhost" }, wantErr: true},
		{name: "invalid port", mutate: func(o *ProxyOpts) { o.ListenAddr = "localhost:abc" }, wantErr: true},
		{name: "zero port", mutate: func(o *ProxyOpts) { o.ListenAddr = "localhost:0" }, wantErr: true},
		{name: "high port", mutate: func(o *ProxyOpts) { o.ListenAddr = "localhost:65536" }, wantErr: true},
		{name: "certificate only", mutate: func(o *ProxyOpts) { o.CACertPath = "ca.crt" }, wantErr: true},
		{name: "key only", mutate: func(o *ProxyOpts) { o.CAKeyPath = "ca.key" }, wantErr: true},
		{name: "missing ca files", mutate: func(o *ProxyOpts) { o.CACertPath, o.CAKeyPath = "missing.crt", "missing.key" }, wantErr: true},
		{name: "invalid max body", mutate: func(o *ProxyOpts) { o.MaxBody = "1GiB" }, wantErr: true},
		{name: "empty max body", mutate: func(o *ProxyOpts) { o.MaxBody = "" }, wantErr: true},
		{name: "zero max body", mutate: func(o *ProxyOpts) { o.MaxBody = "0" }, wantErr: true},
		{name: "max int64 max body", mutate: func(o *ProxyOpts) { o.MaxBody = "9223372036854775807" }, wantErr: true},
		{name: "overflow max body", mutate: func(o *ProxyOpts) { o.MaxBody = "17592186044417MB" }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := NewProxy(&Root{})
			if tt.mutate != nil {
				tt.mutate(&proxy.opts)
			}

			err := proxy.validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			wantBytes := tt.wantBytes
			if wantBytes == 0 {
				wantBytes = utils.MB
			}

			if err == nil && proxy.opts.MaxBodyBytes != wantBytes {
				t.Fatalf("MaxBodyBytes = %d, want %d", proxy.opts.MaxBodyBytes, wantBytes)
			}
		})
	}
}

func TestProxyValidateCustomCA(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")

	if err := os.WriteFile(certPath, []byte("certificate"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(keyPath, []byte("private key"), 0o600); err != nil {
		t.Fatal(err)
	}

	proxy := NewProxy(&Root{})
	proxy.opts.CACertPath = certPath

	proxy.opts.CAKeyPath = keyPath
	if err := proxy.validate(); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
}

func TestProxyCAPaths(t *testing.T) {
	home := t.TempDir()
	proxy := NewProxy(&Root{Opts: RootOpts{Home: home}})

	certPath, keyPath := proxy.caPaths()
	if want := filepath.Join(home, ".ncmctl", "proxy", "ca.crt"); certPath != want {
		t.Fatalf("certPath = %q, want %q", certPath, want)
	}

	if want := filepath.Join(home, ".ncmctl", "proxy", "ca.key"); keyPath != want {
		t.Fatalf("keyPath = %q, want %q", keyPath, want)
	}

	proxy.opts.CACertPath = "custom.crt"
	proxy.opts.CAKeyPath = "custom.key"

	certPath, keyPath = proxy.caPaths()
	if certPath != "custom.crt" || keyPath != "custom.key" {
		t.Fatalf("custom CA paths = %q, %q", certPath, keyPath)
	}
}

func TestProxyRejectsArguments(t *testing.T) {
	proxy := NewProxy(&Root{})
	if err := proxy.cmd.Args(proxy.cmd, []string{"unexpected"}); err == nil {
		t.Fatal("expected positional argument to be rejected")
	}
}

func TestRootRegistersProxyCommand(t *testing.T) {
	root := New()

	command, _, err := root.cmd.Find([]string{"proxy"})
	if err != nil {
		t.Fatal(err)
	}

	if command == nil || command.Name() != "proxy" {
		t.Fatalf("proxy command not registered: %#v", command)
	}

	if got := command.Flag("listen").DefValue; got != "127.0.0.1:9000" {
		t.Fatalf("listen default = %q", got)
	}

	if got := command.Flag("max-body").DefValue; got != "1MB" {
		t.Fatalf("max-body default = %q", got)
	}

	for _, name := range []string{"xeapi-session-id", "xeapi-session-key", "xeapi-state-file"} {
		if got := command.Flag(name).DefValue; got != "" {
			t.Fatalf("%s default = %q, want empty", name, got)
		}
	}
}

func TestProxyValidateXeapiCommandLineSession(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		key     string
		wantErr string
	}{
		{name: "16 bytes", id: "session", key: "0123456789abcdef"},
		{name: "24 bytes", id: "session", key: "0123456789abcdefghijklmn"},
		{name: "hex-looking raw 32 bytes", id: "session", key: "00112233445566778899aabbccddeeff"},
		{name: "surrounding ID whitespace", id: " \tsession\n ", key: "0123456789abcdef"},
		{name: "non-ASCII raw bytes", id: "session", key: "éééééééé", wantErr: "ASCII"},
		{name: "oversized id", id: strings.Repeat("s", 1025), key: "0123456789abcdef", wantErr: "1024 bytes"},
		{name: "oversized raw padded id", id: strings.Repeat(" ", 1024) + "session", key: "0123456789abcdef", wantErr: "1024 bytes"},
		{name: "missing key", id: "session", wantErr: "must be provided together"},
		{name: "missing id", key: "0123456789abcdef", wantErr: "must be provided together"},
		{name: "blank id", id: "  ", key: "0123456789abcdef", wantErr: "must be provided together"},
		{name: "invalid key length", id: "session", key: "sensitive-short-key", wantErr: "length is invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := NewProxy(&Root{})
			command.opts.XeapiSessionID = tt.id
			command.opts.XeapiSessionKey = tt.key

			err := command.validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				if tt.key != "" {
					assert.NotContains(t, err.Error(), tt.key)
				}
				return
			}

			require.NoError(t, err)
			require.Len(t, command.opts.XeapiSessions, 1)
			assert.Equal(t, strings.TrimSpace(tt.id), command.opts.XeapiSessions[0].ID)
			assert.Equal(t, tt.key, command.opts.XeapiSessions[0].Key)
			assert.Equal(t, proxyserver.XeapiSessionSourceCommandLine, command.opts.XeapiSessions[0].Source)
		})
	}
}

func TestProxyLoadsOnlyExplicitXeapiStateFile(t *testing.T) {
	home := t.TempDir()
	autoPath := filepath.Join(home, ".ncmctl", "xeapi.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(autoPath), 0o700))
	require.NoError(t, os.WriteFile(autoPath, []byte("not: [valid"), 0o600))

	command := NewProxy(&Root{Opts: RootOpts{Home: home}})
	require.NoError(t, command.validate())
	assert.Empty(t, command.opts.XeapiSessions, "proxy must not auto-discover home xeapi.yaml")

	explicitPath := filepath.Join(t.TempDir(), "xeapi.yaml")
	require.NoError(t, os.WriteFile(explicitPath, []byte(`publicKeyState:
  publicKey: ""
  version: ""
  nextUpdateTime: 0
  sk: ""
session:
  id: "  state-session  "
  key: 0123456789abcdef
`), 0o600))
	command.opts.XeapiStateFile = explicitPath
	require.NoError(t, command.validate())
	require.Len(t, command.opts.XeapiSessions, 1)
	assert.Equal(t, proxyserver.XeapiSessionSeed{
		ID: "state-session", Key: "0123456789abcdef", Source: proxyserver.XeapiSessionSourceStateFile,
	}, command.opts.XeapiSessions[0])
}

func TestProxyXeapiStateAndCommandLinePrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xeapi.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`session:
  id: same-session
  key: state-file-key!!
`), 0o600))

	command := NewProxy(&Root{})
	command.opts.XeapiStateFile = path
	command.opts.XeapiSessionID = "same-session"
	command.opts.XeapiSessionKey = "command-line-key"
	require.NoError(t, command.validate())
	require.Len(t, command.opts.XeapiSessions, 2)
	assert.Equal(t, proxyserver.XeapiSessionSourceStateFile, command.opts.XeapiSessions[0].Source)
	assert.Equal(t, proxyserver.XeapiSessionSourceCommandLine, command.opts.XeapiSessions[1].Source)
}

func TestProxyRejectsInvalidCommandLineSessionWithValidStateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xeapi.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`session:
  id: state-session
  key: 0123456789abcdef
`), 0o600))

	command := NewProxy(&Root{})
	command.opts.XeapiStateFile = path
	command.opts.XeapiSessionID = "command-session"
	command.opts.XeapiSessionKey = "invalid-key"

	err := command.validate()
	require.ErrorContains(t, err, "length is invalid")
	assert.NotContains(t, err.Error(), command.opts.XeapiSessionKey)
	assert.Empty(t, command.opts.XeapiSessions)
}

func TestProxyRejectsInvalidExplicitXeapiState(t *testing.T) {
	oversized := filepath.Join(t.TempDir(), "oversized.yaml")
	require.NoError(t, os.WriteFile(oversized, bytes.Repeat([]byte{'x'}, int(proxyXeapiStateFileSize+1)), 0o600))

	tests := []struct {
		name    string
		path    func() string
		wantErr string
	}{
		{name: "missing", path: func() string { return filepath.Join(t.TempDir(), "missing.yaml") }, wantErr: "no such file"},
		{name: "directory", path: func() string { return t.TempDir() }, wantErr: "regular file"},
		{name: "invalid YAML", path: func() string {
			path := filepath.Join(t.TempDir(), "invalid.yaml")
			require.NoError(t, os.WriteFile(path, []byte("session: ["), 0o600))
			return path
		}, wantErr: "decode YAML"},
		{name: "missing session", path: func() string {
			path := filepath.Join(t.TempDir(), "empty.yaml")
			require.NoError(t, os.WriteFile(path, []byte("publicKeyState: {}\n"), 0o600))
			return path
		}, wantErr: "session.id and session.key are required"},
		{name: "incomplete session", path: func() string {
			path := filepath.Join(t.TempDir(), "incomplete.yaml")
			require.NoError(t, os.WriteFile(path, []byte("session:\n  id: only-id\n"), 0o600))
			return path
		}, wantErr: "session.id and session.key are required"},
		{name: "invalid key length", path: func() string {
			path := filepath.Join(t.TempDir(), "short.yaml")
			require.NoError(t, os.WriteFile(path, []byte("session:\n  id: session\n  key: secret-short\n"), 0o600))
			return path
		}, wantErr: "length is invalid"},
		{name: "oversized raw padded id", path: func() string {
			path := filepath.Join(t.TempDir(), "padded-id.yaml")
			data := "session:\n  id: \"" + strings.Repeat(" ", 1024) + "session\"\n  key: 0123456789abcdef\n"
			require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
			return path
		}, wantErr: "1024 bytes"},
		{name: "oversized", path: func() string { return oversized }, wantErr: "exceeds 1048576 bytes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := NewProxy(&Root{})
			command.opts.XeapiStateFile = tt.path()
			command.opts.XeapiSessionID = "valid-command-line"
			command.opts.XeapiSessionKey = "0123456789abcdef"
			err := command.validate()
			require.ErrorContains(t, err, tt.wantErr)
			assert.NotContains(t, err.Error(), "0123456789abcdef")
			assert.Empty(t, command.opts.XeapiSessions)
		})
	}
}

func TestProxyHelpWarnsAboutXeapiSessionKeyExposure(t *testing.T) {
	command := NewProxy(&Root{}).Command()

	var output bytes.Buffer
	command.SetOut(&output)
	require.NoError(t, command.Help())

	help := output.String()
	for _, text := range []string{
		"--xeapi-session-id", "--xeapi-session-key", "--xeapi-state-file",
		"1024 bytes", "raw ASCII", "shell history", "process arguments",
	} {
		assert.Contains(t, help, text, "help missing %q:\n%s", text, help)
	}
}

// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"os"
	"path/filepath"
	"testing"

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
			proxy := NewProxy(&Root{}, nil)
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

	proxy := NewProxy(&Root{}, nil)
	proxy.opts.CACertPath = certPath

	proxy.opts.CAKeyPath = keyPath
	if err := proxy.validate(); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
}

func TestProxyCAPaths(t *testing.T) {
	home := t.TempDir()
	proxy := NewProxy(&Root{Opts: RootOpts{Home: home}}, nil)

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
	proxy := NewProxy(&Root{}, nil)
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
}

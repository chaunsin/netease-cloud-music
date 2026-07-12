// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListenAddr      = "127.0.0.1:9000"
	defaultMaxBodyBytes    = int64(1 << 20)
	defaultShutdownTimeout = 5 * time.Second
)

// Config controls the local HTTP(S) monitoring proxy.
type Config struct {
	ListenAddr string
	CACertPath string
	CAKeyPath  string
	// RequirePrivateCAPath applies the managed-CA directory policy to the
	// configured certificate and key paths. It is set for ncmctl's default CA.
	RequirePrivateCAPath bool
	MaxBodyBytes         int64
	ShowSensitive        bool
	Debug                bool
	Domains              []string
	Out                  io.Writer
	ErrOut               io.Writer
	ShutdownTimeout      time.Duration
}

func normalizeConfig(cfg Config) (Config, error) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = defaultShutdownTimeout
	}
	if len(cfg.Domains) == 0 {
		cfg.Domains = DefaultDomains()
	}
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}
	if cfg.ErrOut == nil {
		cfg.ErrOut = os.Stderr
	}

	if cfg.CACertPath == "" && cfg.CAKeyPath == "" {
		cfg.RequirePrivateCAPath = true
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("resolve home directory: %w", err)
		}
		cfg.CACertPath = filepath.Join(home, ".ncmctl", "proxy", "ca.crt")
		cfg.CAKeyPath = filepath.Join(home, ".ncmctl", "proxy", "ca.key")
	}
	if (cfg.CACertPath == "") != (cfg.CAKeyPath == "") {
		return Config{}, fmt.Errorf("ca-cert and ca-key must be provided together")
	}
	if cfg.MaxBodyBytes <= 0 {
		return Config{}, fmt.Errorf("max body bytes must be greater than zero")
	}
	// Capture code uses MaxBodyBytes + 1 as a truncation sentinel.
	if cfg.MaxBodyBytes == math.MaxInt64 {
		return Config{}, fmt.Errorf("max body bytes must be less than %d", math.MaxInt64)
	}
	if cfg.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("shutdown timeout must be greater than zero")
	}
	if err := validateListenAddress(cfg.ListenAddr, true); err != nil {
		return Config{}, err
	}

	cfg.CACertPath = filepath.Clean(cfg.CACertPath)
	cfg.CAKeyPath = filepath.Clean(cfg.CAKeyPath)
	return cfg, nil
}

func validateListenAddress(address string, allowPortZero bool) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid listen address %q: %w", address, err)
	}
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("listen address must include an explicit host")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 0 || port > 65535 || (!allowPortZero && port == 0) {
		return fmt.Errorf("listen port must be between 1 and 65535")
	}
	return nil
}

func isLoopbackListenAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

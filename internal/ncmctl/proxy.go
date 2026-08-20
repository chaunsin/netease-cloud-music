// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"

	"github.com/chaunsin/netease-cloud-music/config"
	proxyserver "github.com/chaunsin/netease-cloud-music/internal/proxy"
	ncmcrypto "github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

const (
	proxyShutdownTimeout    = 5 * time.Second
	proxyXeapiStateFileSize = int64(1 << 20)
)

type ProxyOpts struct {
	ListenAddr      string
	CACertPath      string
	CAKeyPath       string
	MaxBody         string
	MaxBodyBytes    int64
	ShowSensitive   bool
	XeapiSessionID  string
	XeapiSessionKey string
	XeapiStateFile  string
	XeapiSessions   []proxyserver.XeapiSessionSeed
}

type Proxy struct {
	root *Root
	cmd  *cobra.Command
	opts ProxyOpts
}

func NewProxy(root *Root) *Proxy {
	c := &Proxy{
		root: root,
		cmd: &cobra.Command{
			Use:   "proxy",
			Short: "Monitor NetEase Cloud Music HTTP(S) API traffic",
			Long: "Start an explicit HTTP(S) proxy for a client you control. NetEase-related " +
				"traffic is captured and redacted by default; other traffic is forwarded without capture. " +
				"XEAPI sessions are learned from response headers at runtime; optional startup sessions are " +
				"loaded only from explicitly supplied flags or a state file. " +
				"The command generates or reuses a CA but never modifies a trust store. Non-loopback " +
				"listeners expose an unauthenticated proxy and should be used only on trusted networks.",
			Args: cobra.NoArgs,
			Example: `  # Listen locally, then configure the client proxy as 127.0.0.1:9000
  ncmctl proxy

  # Listen on the LAN (only use this on a trusted network)
  ncmctl proxy --listen 0.0.0.0:9000

  # Use an existing CA certificate and private key
  ncmctl proxy --ca-cert ./ca.crt --ca-key ./ca.key

  # Seed passive XEAPI request decryption from an explicit state file
  ncmctl proxy --xeapi-state-file ~/.ncmctl/xeapi.yaml`,
		},
	}
	c.addFlags()
	c.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return c.execute(cmd.Context())
	}
	return c
}

func (c *Proxy) Command() *cobra.Command {
	return c.cmd
}

func (c *Proxy) addFlags() {
	c.cmd.Flags().StringVar(&c.opts.ListenAddr, "listen", "127.0.0.1:9000", "proxy listen address in host:port form")
	c.cmd.Flags().StringVar(&c.opts.CACertPath, "ca-cert", "", "existing CA certificate path (requires --ca-key)")
	c.cmd.Flags().StringVar(&c.opts.CAKeyPath, "ca-key", "", "existing CA private key path (requires --ca-cert)")
	c.cmd.Flags().StringVar(&c.opts.MaxBody, "max-body", "1MB", "maximum body bytes displayed per request or response; forwarding is unaffected")
	c.cmd.Flags().BoolVar(&c.opts.ShowSensitive, "show-sensitive", false, "disable redaction and print credentials and identifiers")
	c.cmd.Flags().StringVar(&c.opts.XeapiSessionID, "xeapi-session-id", "", "XEAPI session ID, at most 1024 bytes (requires --xeapi-session-key)")
	c.cmd.Flags().StringVar(&c.opts.XeapiSessionKey, "xeapi-session-key", "", "XEAPI raw ASCII session key, 16/24/32 bytes (visible in shell history and process arguments)")
	c.cmd.Flags().StringVar(&c.opts.XeapiStateFile, "xeapi-state-file", "", "explicit canonical xeapi.yaml used only to seed the proxy session cache")
}

func (c *Proxy) validate() error {
	host, port, err := net.SplitHostPort(c.opts.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen address must be in host:port form: %w", err)
	}

	if strings.TrimSpace(host) == "" {
		return errors.New("listen host is required")
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("listen port must be between 1 and 65535")
	}

	if (c.opts.CACertPath == "") != (c.opts.CAKeyPath == "") {
		return errors.New("ca-cert and ca-key must be provided together")
	}

	if c.opts.CACertPath != "" {
		if validateErr := validateProxyCAFile("ca-cert", c.opts.CACertPath); validateErr != nil {
			return validateErr
		}

		if validateErr := validateProxyCAFile("ca-key", c.opts.CAKeyPath); validateErr != nil {
			return validateErr
		}
	}

	c.opts.MaxBodyBytes, err = utils.ParseBytes(c.opts.MaxBody)
	if err != nil {
		return fmt.Errorf("parse max-body: %w", err)
	}

	if c.opts.MaxBodyBytes <= 0 {
		return errors.New("max-body must be greater than zero")
	}
	// Capture helpers reserve one extra byte to distinguish truncation.
	if c.opts.MaxBodyBytes == math.MaxInt64 {
		return fmt.Errorf("max-body must be less than %d bytes", math.MaxInt64)
	}

	seeds, err := c.xeapiSessionSeeds()
	if err != nil {
		return err
	}

	c.opts.XeapiSessions = seeds
	return nil
}

func (c *Proxy) xeapiSessionSeeds() ([]proxyserver.XeapiSessionSeed, error) {
	seeds := make([]proxyserver.XeapiSessionSeed, 0, 2)

	if c.opts.XeapiStateFile != "" {
		seed, err := loadProxyXeapiState(c.opts.XeapiStateFile)
		if err != nil {
			return nil, fmt.Errorf("xeapi-state-file %q: %w", c.opts.XeapiStateFile, err)
		}

		seeds = append(seeds, seed)
	}

	hasID := strings.TrimSpace(c.opts.XeapiSessionID) != ""

	hasKey := c.opts.XeapiSessionKey != ""
	if hasID != hasKey {
		return nil, errors.New("xeapi-session-id and xeapi-session-key must be provided together")
	}

	if hasID {
		seed, err := validatedProxyXeapiSessionSeed(
			c.opts.XeapiSessionID,
			c.opts.XeapiSessionKey,
			proxyserver.XeapiSessionSourceCommandLine,
		)
		if err != nil {
			return nil, fmt.Errorf("invalid command-line XEAPI session: %w", err)
		}

		seeds = append(seeds, seed)
	}
	return seeds, nil
}

func loadProxyXeapiState(path string) (_ proxyserver.XeapiSessionSeed, returnErr error) {
	info, err := os.Stat(path)
	if err != nil {
		return proxyserver.XeapiSessionSeed{}, err
	}

	if !info.Mode().IsRegular() {
		return proxyserver.XeapiSessionSeed{}, errors.New("must be a regular file")
	}

	if info.Size() > proxyXeapiStateFileSize {
		return proxyserver.XeapiSessionSeed{}, fmt.Errorf("exceeds %d bytes", proxyXeapiStateFileSize)
	}

	file, err := os.Open(path)
	if err != nil {
		return proxyserver.XeapiSessionSeed{}, err
	}
	defer func() {
		returnErr = errors.Join(returnErr, file.Close())
	}()

	openedInfo, err := file.Stat()
	if err != nil {
		return proxyserver.XeapiSessionSeed{}, fmt.Errorf("stat opened file: %w", err)
	}

	if !openedInfo.Mode().IsRegular() {
		return proxyserver.XeapiSessionSeed{}, errors.New("must remain a regular file while opening")
	}

	data, err := io.ReadAll(io.LimitReader(file, proxyXeapiStateFileSize+1))
	if err != nil {
		return proxyserver.XeapiSessionSeed{}, fmt.Errorf("read: %w", err)
	}

	if int64(len(data)) > proxyXeapiStateFileSize {
		return proxyserver.XeapiSessionSeed{}, fmt.Errorf("exceeds %d bytes", proxyXeapiStateFileSize)
	}

	var state struct {
		Session ncmcrypto.XeapiSession `yaml:"session"`
	}
	if decodeErr := yaml.Unmarshal(data, &state); decodeErr != nil {
		return proxyserver.XeapiSessionSeed{}, fmt.Errorf("decode YAML: %w", decodeErr)
	}

	if strings.TrimSpace(state.Session.ID) == "" || state.Session.Key == "" {
		return proxyserver.XeapiSessionSeed{}, errors.New("session.id and session.key are required")
	}

	seed, err := validatedProxyXeapiSessionSeed(
		state.Session.ID,
		state.Session.Key,
		proxyserver.XeapiSessionSourceStateFile,
	)
	if err != nil {
		return proxyserver.XeapiSessionSeed{}, fmt.Errorf("invalid session: %w", err)
	}
	return seed, nil
}

func validatedProxyXeapiSessionSeed(id, key, source string) (proxyserver.XeapiSessionSeed, error) {
	seed := proxyserver.XeapiSessionSeed{ID: id, Key: key, Source: source}
	if err := seed.Validate(); err != nil {
		return proxyserver.XeapiSessionSeed{}, err
	}

	seed.ID = strings.TrimSpace(seed.ID)
	return seed, nil
}

func validateProxyCAFile(name, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s %q: %w", name, path, err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s %q must be a regular file", name, path)
	}
	return nil
}

func (c *Proxy) caPaths() (string, string) {
	if c.opts.CACertPath != "" {
		return c.opts.CACertPath, c.opts.CAKeyPath
	}

	home := c.root.Opts.Home
	if home == "" {
		home = config.HomeDir
	}

	caDir := filepath.Join(filepath.Clean(home), ".ncmctl", "proxy")
	return filepath.Join(caDir, "ca.crt"), filepath.Join(caDir, "ca.key")
}

func (c *Proxy) execute(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	caCertPath, caKeyPath := c.caPaths()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := proxyserver.Config{
		ListenAddr:           c.opts.ListenAddr,
		CACertPath:           caCertPath,
		CAKeyPath:            caKeyPath,
		MaxBodyBytes:         c.opts.MaxBodyBytes,
		ShowSensitive:        c.opts.ShowSensitive,
		RequirePrivateCAPath: c.opts.CACertPath == "",
		Debug:                c.root.Opts.Debug,
		Domains:              proxyserver.DefaultDomains(),
		Out:                  c.cmd.OutOrStdout(),
		ErrOut:               c.cmd.ErrOrStderr(),
		ShutdownTimeout:      proxyShutdownTimeout,
		XeapiSessions:        c.opts.XeapiSessions,
	}
	if err := proxyserver.Run(ctx, &cfg); err != nil {
		return fmt.Errorf("run proxy: %w", err)
	}
	return nil
}

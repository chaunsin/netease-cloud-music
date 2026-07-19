// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

type Config struct {
	Host     string        `json:"host" yaml:"host"`
	Username string        `json:"username" yaml:"username"`
	Password string        `json:"password" yaml:"password"`
	Timeout  time.Duration `json:"timeout" yaml:"timeout"`
}

func (c Config) Validate() error {
	if c.Host == "" {
		return errors.New("host is empty")
	}
	return nil
}

type Client struct {
	cli *resty.Client
	cfg *Config
}

func New(cfg *Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("http: Validate: %w", err)
	}

	cli := resty.New()
	cli.SetTimeout(cfg.Timeout)
	cli.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	// cli.SetDebug(cfg.Debug)

	m := &Client{
		cli: cli,
		cfg: cfg,
	}
	return m, nil
}

func (c *Client) Send(ctx context.Context, content string) error {
	resp, err := c.cli.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBasicAuth(c.cfg.Username, c.cfg.Password).
		SetBody(content).
		Post(c.cfg.Host)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("http: status code: %d", resp.StatusCode())
	}
	return nil
}

func (c *Client) Close(ctx context.Context) error {
	c.cli.SetCloseConnection(true)
	return nil
}

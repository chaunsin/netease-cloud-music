// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package mail

import (
	"context"
	"errors"
	"fmt"

	"github.com/wneessen/go-mail"
)

type Config struct {
	Host     string
	Port     int64
	Username string
	Password string
	To       []string
}

func (c Config) Validate() error {
	if c.Host == "" {
		return errors.New("host is empty")
	}
	if c.Port == 0 {
		return errors.New("port is empty")
	}
	if c.Username == "" {
		return errors.New("username is empty")
	}
	if c.Password == "" {
		return errors.New("password is empty")
	}
	if len(c.To) == 0 {
		return errors.New("to is empty")
	}
	return nil
}

type Client struct {
	cli *mail.Client
	cfg *Config
}

func New(cfg *Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("mail: Validate: %w", err)
	}

	cli, err := mail.NewClient(
		cfg.Host,
		mail.WithPort(int(cfg.Port)),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(cfg.Username),
		mail.WithPassword(cfg.Password))
	if err != nil {
		return nil, err
	}

	m := &Client{
		cli: cli,
		cfg: cfg,
	}
	return m, nil
}

func (c *Client) Send(ctx context.Context, content string) error {
	var msg []*mail.Msg
	for _, to := range c.cfg.To {
		m := mail.NewMsg()
		if err := m.From(c.cfg.Username); err != nil {
			return fmt.Errorf("from: %w", err)
		}
		if err := m.To(to); err != nil {
			return fmt.Errorf("to: %w", err)
		}
		m.Subject("This is my first mail with go-mail!")
		m.SetBodyString(mail.TypeTextPlain, content)
		msg = append(msg, m)
	}
	if err := c.cli.DialAndSendWithContext(ctx, msg...); err != nil {
		return fmt.Errorf("DialAndSendWithContext: %w", err)
	}
	return nil
}

func (c *Client) Close(ctx context.Context) error {
	return c.cli.Close()
}

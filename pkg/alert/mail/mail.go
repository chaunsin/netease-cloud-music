// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

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
			return fmt.Errorf("From: %w", err)
		}
		if err := m.To(to); err != nil {
			return fmt.Errorf("To: %w", err)
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

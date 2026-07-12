// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
)

type cryptoCmd struct {
	root *Crypto
	cmd  *cobra.Command
	l    *log.Logger

	url string
}

func encrypt(root *Crypto, l *log.Logger) *cobra.Command {
	c := &cryptoCmd{
		root: root,
		l:    l,
	}
	c.cmd = &cobra.Command{
		Use:     "encrypt",
		Short:   "Encrypt data",
		Example: "  ncmctl crypto encrypt -k weapi -u /eapi/sms/captcha/sent\n  ncmctl crypto encrypt -k weapi '{\"key\":\"value\"}'",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute(cmd.Context(), args)
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *cryptoCmd) addFlags() {
	c.cmd.Flags().StringVarP(&c.url, "url", "u", "", "url params value,used closely in 'k=eapi' mode")
}

func (c *cryptoCmd) execute(_ context.Context, args []string) error {
	var (
		opts  = c.root.opts
		input string
	)
	if len(args) <= 0 {
		return fmt.Errorf("nothing was entered")
	}
	input = args[0]

	if utils.IsFile(input) {
		data, err := os.ReadFile(input)
		if err != nil {
			return fmt.Errorf("readFile: %w", err)
		}
		input = string(data)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(input), &payload); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	var data []byte
	switch kind := opts.Kind; kind {
	case "eapi":
		{
			if c.url == "" {
				return fmt.Errorf("url params is empty")
			}
			parsed, err := url.Parse(c.url)
			if err != nil {
				return fmt.Errorf("parse: %w", err)
			}
			ciphertext, err := crypto.EApiEncrypt(parsed.Path, payload)
			if err != nil {
				return fmt.Errorf("加密失败: %w", err)
			}
			data, err = json.MarshalIndent(ciphertext, "", "\t")
			if err != nil {
				return fmt.Errorf("MarshalIndent: %w", err)
			}
		}
	case "weapi":
		ciphertext, err := crypto.WeApiEncrypt(payload)
		if err != nil {
			return fmt.Errorf("加密失败: %w", err)
		}
		data, err = json.MarshalIndent(ciphertext, "", "\t")
		if err != nil {
			return fmt.Errorf("MarshalIndent: %w", err)
		}
	case "linux":
		ciphertext, err := crypto.LinuxApiEncrypt(payload)
		if err != nil {
			return fmt.Errorf("加密失败: %w", err)
		}
		data, err = json.MarshalIndent(ciphertext, "", "\t")
		if err != nil {
			return fmt.Errorf("MarshalIndent: %w", err)
		}
	default:
		return fmt.Errorf("%s known kind", kind)
	}
	return writeFile(c.cmd, opts.Output, data)
}

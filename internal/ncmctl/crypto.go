// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type CryptoOpts struct {
	Output string // 生成文件路径
	Kind   string // api类型
}

type Crypto struct {
	root *Root
	cmd  *cobra.Command
	opts CryptoOpts
	l    *log.Logger
}

func NewCrypto(root *Root, l *log.Logger) *Crypto {
	c := &Crypto{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "crypto",
			Short:   "Crypto is a tool for encrypting and decrypting the http data",
			Example: "  ncmctl crypto -h\n  ncmctl crypto decrypt -k eapi 'ciphertext'\n  ncmctl crypto decrypt http_request.har\n  ncmctl crypto encrypt -k weapi '{\"key\":\"value\"}'",
		},
	}
	c.addFlags()
	c.Add(encrypt(c, l))
	c.Add(decrypt(c, l))
	return c
}

func (c *Crypto) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Crypto) Command() *cobra.Command {
	return c.cmd
}

func (c *Crypto) addFlags() {
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "", "generate decrypt file directory location")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Kind, "kind", "k", "weapi", "encryption and decryption mode, weapi|eapi|linux")
}

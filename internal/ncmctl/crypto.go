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
			Use:   "crypto",
			Short: "Encrypt and decrypt legacy NetEase API payloads",
			Long: "Inspect supported NetEase API encryption formats locally. Encryption supports " +
				"WEAPI, EAPI, and Linux API payloads; direct request decryption currently supports EAPI " +
				"only. HAR files and decrypted output may contain credentials and personal data.",
			Example: "  ncmctl crypto encrypt --kind weapi '{\"key\":\"value\"}'\n" +
				"  ncmctl crypto encrypt --kind eapi --url /eapi/v3/song/detail request.json\n" +
				"  ncmctl crypto decrypt --kind eapi --encode hex 'CIPHERTEXT'\n" +
				"  ncmctl crypto decrypt --url '/eapi/*' capture.har",
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
	c.cmd.PersistentFlags().StringVarP(&c.opts.Output, "output", "o", "", "write the JSON result to a file instead of stdout")
	c.cmd.PersistentFlags().StringVarP(&c.opts.Kind, "kind", "k", "weapi", "payload mode: weapi, eapi, or linux (decrypt supports eapi only)")
}

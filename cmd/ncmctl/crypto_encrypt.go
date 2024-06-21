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

package ncmctl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/spf13/cobra"
)

type cryptoCmd struct {
	root *Crypto
	cmd  *cobra.Command
	l    *log.Logger

	url string
	// encode string
}

func encrypt(root *Crypto, l *log.Logger) *cobra.Command {
	c := &cryptoCmd{
		root: root,
		l:    l,
	}
	c.cmd = &cobra.Command{
		Use:     "encrypt",
		Short:   "Encrypt data",
		Example: "  ncm crypto encrypt -k weapi -u /eapi/sms/captcha/sent -p \"plaintext\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute(cmd.Context())
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *cryptoCmd) addFlags() {
	c.cmd.Flags().StringVarP(&c.url, "url", "u", "", "url params value")
	// c.cmd.Flags().StringVarP(&c.encode, "encode", "e", "hex", "string|hex|base64")
}

func (c *cryptoCmd) execute(ctx context.Context) error {
	var opts = c.root.opts
	// if c.encode != "string" && c.encode != "base64" && c.encode != "hex" {
	// 	return fmt.Errorf("%s is unknown encode", c.encode)
	// }
	if opts.Input == "" {
		return fmt.Errorf("nothing was entered")
	}
	if utils.IsFile(opts.Input) {
		data, err := os.ReadFile(opts.Input)
		if err != nil {
			return fmt.Errorf("ReadFile: %w", err)
		}
		opts.Input = string(data)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(opts.Input), &payload); err != nil {
		return fmt.Errorf("Unmarshal: %w", err)
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
		return fmt.Errorf("%s to be realized", kind)
	default:
		return fmt.Errorf("%s known kind", kind)
	}
	return writefile(c.cmd, opts.Output, data)
}

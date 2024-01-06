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

package crypto

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/utils"

	"github.com/spf13/cobra"
)

type cryptoCmd struct {
	root *Cmd
	cmd  *cobra.Command

	kind      string
	url       string
	plaintext string
	encode    string
}

func NewEncrypt(root *Cmd) *cobra.Command {
	c := &cryptoCmd{
		root: root,
	}
	c.cmd = &cobra.Command{
		Use:     "encrypt",
		Short:   "Encrypt data",
		Example: "ncm encrypt -k weapi -u /eapi/sms/captcha/sent -P xxx",
		Run: func(cmd *cobra.Command, args []string) {
			if err := c.execute(); err != nil {
				fmt.Println(err)
			}
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *cryptoCmd) addFlags() {
	c.cmd.Flags().StringVarP(&c.kind, "kind", "k", "weapi", "weapi|eapi|linux")
	c.cmd.Flags().StringVarP(&c.plaintext, "plaintext", "p", "", "plaintext json value")
	c.cmd.Flags().StringVarP(&c.url, "url", "u", "", "url")
	// c.cmd.Flags().StringVarP(&c.encode, "encode", "e", "hex", "string|hex|base64")
}

func (c *cryptoCmd) execute() error {
	var plaintext string
	// if c.encode != "string" && c.encode != "base64" && c.encode != "hex" {
	// 	return fmt.Errorf("%s is unknown encode", c.encode)
	// }
	if c.plaintext == "" && c.root.RootOpts.Input == "" {
		return fmt.Errorf("nothing was entered")
	}
	if c.root.RootOpts.Input != "" {
		data, err := os.ReadFile(c.root.RootOpts.Input)
		if err != nil {
			return fmt.Errorf("ReadFile: %w", err)
		}
		plaintext = string(data)
	}
	if c.plaintext != "" {
		plaintext = c.plaintext
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
		return fmt.Errorf("Unmarshal: %w", err)
	}

	var data []byte
	switch c.kind {
	case "eapi":
		{
			parsed, err := url.Parse(c.url)
			if err != nil {
				return fmt.Errorf("parse: %w", err)
			}
			ciphertext, err := api.EApiEncrypt(parsed.Path, payload)
			if err != nil {
				return fmt.Errorf("加密失败: %w", err)
			}
			data, err = json.MarshalIndent(ciphertext, "", "\t")
			if err != nil {
				return fmt.Errorf("MarshalIndent: %w", err)
			}
		}
	case "weapi":
		ciphertext, err := api.WeApiEncrypt(payload)
		if err != nil {
			return fmt.Errorf("加密失败: %w", err)
		}
		data, err = json.MarshalIndent(ciphertext, "", "\t")
		if err != nil {
			return fmt.Errorf("MarshalIndent: %w", err)
		}
	case "linux":
		return fmt.Errorf("%s to be realized", c.kind)
	default:
		return fmt.Errorf("%s known kind", c.kind)
	}

	if out := c.root.RootOpts.Output; out != "" {
		var file string
		if !filepath.IsAbs(out) {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			file = filepath.Join(wd, out)
			if !utils.PathExists(file) {
				if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
					return fmt.Errorf("MkdirAll: %w", err)
				}
			}
		}
		if err := os.WriteFile(file, data, os.ModePerm); err != nil {
			return fmt.Errorf("WriteFile: %w", err)
		}
		fmt.Printf("generate file path: %s\n", file)
		return nil
	}
	fmt.Println("ciphertext:\n" + string(data) + "\n")
	return nil
}

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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chaunsin/netease-cloud-music/api"
	"github.com/chaunsin/netease-cloud-music/utils"

	"github.com/spf13/cobra"
)

type decryptCmd struct {
	root *Cmd
	cmd  *cobra.Command

	kind       string
	ciphertext string
	encode     string
}

func NewDecrypt(root *Cmd) *cobra.Command {
	c := &decryptCmd{
		root: root,
	}
	c.cmd = &cobra.Command{
		Use:     "decrypt",
		Short:   "Decrypt data",
		Example: "ncm decrypt -k weapi -c xxx",
		Run: func(cmd *cobra.Command, args []string) {
			if err := c.execute(); err != nil {
				fmt.Println(err)
			}
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *decryptCmd) addFlags() {
	c.cmd.Flags().StringVarP(&c.kind, "kind", "k", "weapi", "weapi|eapi|linux")
	c.cmd.Flags().StringVarP(&c.ciphertext, "ciphertext", "c", "", "ciphertext")
	c.cmd.Flags().StringVarP(&c.encode, "encode", "e", "hex", "string|hex|base64")
}

func (c *decryptCmd) execute() error {
	var ciphertext string
	if c.encode != "string" && c.encode != "base64" && c.encode != "hex" {
		return fmt.Errorf("%s is unknown encode", c.encode)
	}
	if c.ciphertext == "" && c.root.RootOpts.Input == "" {
		return fmt.Errorf("nothing was entered")
	}
	if c.root.RootOpts.Input != "" {
		data, err := os.ReadFile(c.root.RootOpts.Input)
		if err != nil {
			return fmt.Errorf("ReadFile: %w", err)
		}
		ciphertext = string(data)
	}
	if c.ciphertext != "" {
		ciphertext = c.ciphertext
	}

	switch c.kind {
	case "eapi":
		{
			data, err := api.EApiDecrypt(ciphertext, c.encode)
			if err != nil {
				return fmt.Errorf("解密失败: %w", err)
			}

			var (
				str     = string(data)
				payload string
				temp    map[string]interface{}
				buf     bytes.Buffer
			)

			// 如果根据标识分隔成3段则说明此数据是包含url和digest摘要形式拼接的数据,反之是结构体数据
			value := strings.Split(str, "-36cd479b6b5-")
			if len(value) == 3 {
				payload = value[1]
				buf.WriteString("url: " + value[0] + "\n")
				buf.WriteString("digest: " + value[2] + "\n")
			} else {
				payload = str
			}
			if err := json.Unmarshal([]byte(payload), &temp); err != nil {
				return fmt.Errorf("Unmarshal: %w", err)
			}
			format, err := json.MarshalIndent(temp, "", "\t")
			if err != nil {
				return fmt.Errorf("MarshalIndent: %w", err)
			}

			buf.WriteString("payload: " + string(format) + "\n")
			buf.WriteString("plaintext:\n" + str + "\n")

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
				if err := os.WriteFile(file, buf.Bytes(), os.ModePerm); err != nil {
					return fmt.Errorf("WriteFile: %w", err)
				}
				fmt.Printf("generate file path: %s\n", file)
				return nil
			}
			fmt.Println(buf.String())
		}
	case "weapi":
		return fmt.Errorf("this [%s] method is not supported", c.kind)
	case "linux":
		return fmt.Errorf("%s to be realized", c.kind)
	default:
		return fmt.Errorf("%s known kind", c.kind)
	}
	return nil
}

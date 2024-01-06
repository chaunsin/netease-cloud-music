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

import "github.com/spf13/cobra"

type cryptoCmd struct {
	root *Cmd
	cmd  *cobra.Command

	kind      string
	url       string
	plaintext string
}

func NewEncrypt(root *Cmd) *cobra.Command {
	c := &cryptoCmd{
		root: root,
	}
	c.cmd = &cobra.Command{
		Use:     "encrypt",
		Short:   "Encrypt data",
		Example: "ncm encrypt -k weapi -u /eapi/sms/captcha/sent -P xxx",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute()
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *cryptoCmd) addFlags() {
	c.cmd.Flags().StringVarP(&c.kind, "kind", "k", "weapi", "weapi|eapi|linux")
	c.cmd.Flags().StringVarP(&c.plaintext, "plaintext", "P", "", "string value")
	c.cmd.Flags().StringVarP(&c.url, "url", "u", "", "url")
}

func (c *cryptoCmd) execute() error {

	return nil
}

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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	har "github.com/chaunsin/go-har"
	"github.com/spf13/cobra"
)

type decryptCmd struct {
	root *Crypto
	cmd  *cobra.Command
	l    *log.Logger

	url    string
	encode string
}

func decrypt(root *Crypto, l *log.Logger) *cobra.Command {
	c := &decryptCmd{
		root: root,
		l:    l,
	}
	c.cmd = &cobra.Command{
		Use:     "decrypt",
		Short:   "Decrypt data",
		Example: "  ncmctl crypto decrypt -k weapi -e base64 \"ciphertext\"\n  ncmctl decrypt example.har",
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute(cmd.Context(), args)
		},
	}
	c.addFlags()
	return c.cmd
}

func (c *decryptCmd) addFlags() {
	c.cmd.Flags().StringVarP(&c.encode, "encode", "e", "hex", "ciphertext content encoding: string|hex|base64")
	c.cmd.Flags().StringVarP(&c.url, "url", "u", "*", "routing address matching example: https://music.163.com/*")
}

func (c *decryptCmd) execute(ctx context.Context, args []string) error {
	var (
		opts  = c.root.opts
		input string
	)
	if c.encode != "string" && c.encode != "base64" && c.encode != "hex" {
		return fmt.Errorf("%s is unknown encode", c.encode)
	}
	if len(args) <= 0 {
		return fmt.Errorf("nothing was entered")
	}
	input = args[0]

	if utils.IsFile(input) {
		data, err := os.ReadFile(input)
		if err != nil {
			return fmt.Errorf("ReadFile: %w", err)
		}
		if filepath.Ext(input) == ".har" {
			list, err := c.parseHar(data)
			if err != nil {
				return fmt.Errorf("parseHar: %w", err)
			}
			log.Debug("parseHar data: %+v", list)
			for i := range list {
				if err := c.decryptReq(&list[i], "hex"); err != nil {
					return fmt.Errorf("decryptReq: %w", err)
				}
				if err := c.decryptRes(&list[i], ""); err != nil {
					return fmt.Errorf("decryptRes: %w", err)
				}
			}
			content, err := json.MarshalIndent(list, "", "  ")
			if err != nil {
				return err
			}
			return writeFile(c.cmd, opts.Output, content)
		}
		input = string(data)
	}

	var payload = &Payload{
		Kind:   opts.Kind,
		Status: "ok",
		Request: Request{
			Ciphertext: input,
		},
		// Response: Response{
		// 	Ciphertext: ciphertext,
		// },
	}

	if err := c.decryptReq(payload, c.encode); err != nil {
		return fmt.Errorf("decryptReq: %w", err)
	}
	// c.decryptRes(ciphertext, c.encode)

	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(c.cmd, opts.Output, content)
}

func (c *decryptCmd) decryptReq(p *Payload, encode string) error {
	if p == nil || p.Request.Ciphertext == "" {
		return fmt.Errorf("request chiphertext is nil or empty")
	}
	switch p.Kind {
	case "eapi":
		{
			data, err := crypto.EApiDecrypt(p.Request.Ciphertext, encode)
			if err != nil {
				return fmt.Errorf("解密失败: %w", err)
			}

			var (
				str     = string(data)
				payload string
			)

			// 如果根据标识分隔成3段则说明此数据是包含url和digest摘要形式拼接的数据,反之是结构体数据
			value := strings.Split(str, "-36cd479b6b5-")
			if len(value) == 3 {
				payload = value[1]
				p.Request.Url = value[0]
				p.Request.Digest = value[2]
			} else {
				payload = str
			}
			p.Request.RawPlaintext = str
			p.Request.Plaintext = payload
		}
	case "weapi":
		return fmt.Errorf("this [%s] method is not supported", p.Kind)
	case "api":
		return fmt.Errorf("%s to be realized", p.Kind)
	case "linux":
		return fmt.Errorf("%s to be realized", p.Kind)
	default:
		return fmt.Errorf("%s known kind", p.Kind)
	}
	return nil
}

func (c *decryptCmd) decryptRes(p *Payload, encode string) error {
	if p == nil || p.Response.Ciphertext == "" {
		return fmt.Errorf("response chiphertext is nil or empty")
	}
	switch p.Kind {
	case "eapi":
		{
			data, err := crypto.EApiDecrypt(p.Response.Ciphertext, encode)
			if err != nil {
				return fmt.Errorf("解密失败: %w", err)
			}

			var (
				str     = string(data)
				payload string
				// 如果根据标识分隔成3段则说明此数据是包含url和digest摘要形式拼接的数据,反之是结构体数据
				value = strings.Split(str, "-36cd479b6b5-")
			)

			if len(value) == 3 {
				payload = value[1]
			} else {
				payload = str
			}
			p.Response.Plaintext = payload
		}
	case "weapi":
		return fmt.Errorf("this [%s] method is not supported", p.Kind)
	case "api":
		return fmt.Errorf("%s to be realized", p.Kind)
	case "linux":
		return fmt.Errorf("%s to be realized", p.Kind)
	default:
		return fmt.Errorf("%s known kind", p.Kind)
	}
	return nil
}

func (c *decryptCmd) parseHar(data []byte) ([]Payload, error) {
	h, err := har.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("NewReader: %w", err)
	}
	if h.EntryTotal() <= 0 {
		return nil, fmt.Errorf("request data is empty")
	}
	var resp = make([]Payload, 0, h.EntryTotal())
	for _, entry := range h.Export().Log.Entries {
		var (
			req  = entry.Request
			res  = entry.Response
			item = Payload{Api: req.URL, Method: req.Method}
		)
		_url, err := url.Parse(req.URL)
		if err != nil {
			return nil, fmt.Errorf("Parse: %w", err)
		}
		value := strings.Split(_url.Path, "/")
		if len(value) < 2 {
			return nil, fmt.Errorf("")
		}
		// 如果地址是这样 https://music.163.com/api/eapi/nos/token/alloc 则返回eapi
		var kind = value[1]
		for _, v := range value {
			switch v {
			case "eapi":
				kind = v
				break
			case "weapi":
				kind = v
				break
			}
		}
		item.Kind = kind
		if !isMatch(c.url, _url.Path) {
			continue
		}

		// 解析request请求参数
		var pd = req.PostData
		if len(pd.Params) > 0 {
			switch kind {
			case "eapi":
				for _, param := range pd.Params {
					if param.Name != "params" {
						return nil, fmt.Errorf("not found params fields: %s", param.Name)
					}
					item.Request.RawPlaintext = param.Value
					break
				}
			case "weapi":
				// 不支持请求加密参数解析，可采通过在前端打断点进行查看
				c.cmd.Printf("weapi %s request params not support parsing\n", req.URL)
			case "api":
				c.cmd.Printf("api %s request params not support parsing\n", req.URL)
				for _, param := range pd.Params {
					_ = param
					item.Request.RawPlaintext = param.Value
					break
				}
			default:
				return nil, fmt.Errorf("parsing not supported: %s ", req.URL)
			}
		} else {
			if strings.HasPrefix(pd.MimeType, "application/x-www-form-urlencoded") {
				values, err := url.ParseQuery(pd.Text)
				if err != nil {
					return nil, fmt.Errorf("ParseQuery: %w", err)
				}
				if values.Has("params") {
					item.Request.Ciphertext = values.Get("params")
				} else {
					fmt.Printf("params is not found,detail:%+v\n", pd)
				}
			}
		}

		// 解析response参数
		if res.Content == nil {
			c.cmd.Printf("%s content is nil\n", req.URL)
			continue
		}

		switch kind {
		case "eapi":
			item.Response.Ciphertext = string(res.Content.Text)
		case "weapi":
			item.Response.Ciphertext = string(res.Content.Text)
		case "api":
			item.Response.Ciphertext = string(res.Content.Text)
		default:
			return nil, fmt.Errorf("parsing not supported: %s ", req.URL)
		}
		resp = append(resp, item)
	}
	return resp, nil
}

func isMatch(pattern, text string) bool {
	pattern, _ = url.PathUnescape(pattern)
	pattern = strings.ReplaceAll(pattern, ".", `\.`)
	pattern = strings.ReplaceAll(pattern, "*", `.*`)
	pattern = "^" + pattern + "$"

	match, err := regexp.MatchString(pattern, text)
	if err != nil {
		fmt.Println("Error matching pattern:", err)
		return false
	}
	return match
}

type Payload struct {
	Api      string   `json:"api,omitempty"`
	Method   string   `json:"method,omitempty"`
	Kind     string   `json:"kind,omitempty"`
	Status   string   `json:"status,omitempty"`
	Request  Request  `json:"request,omitempty"`
	Response Response `json:"response,omitempty"`
}

type Request struct {
	Ciphertext   string `json:"ciphertext,omitempty"`
	RawPlaintext string `json:"rawPlaintext,omitempty"`
	Url          string `json:"url,omitempty"`
	Digest       string `json:"digest,omitempty"`
	Plaintext    string `json:"plaintext,omitempty"`
}

type Response struct {
	Ciphertext string `json:"ciphertext,omitempty"`
	Plaintext  string `json:"plaintext,omitempty"`
}

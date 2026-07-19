// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	har "github.com/chaunsin/go-har"
	"github.com/spf13/cobra"

	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
	"github.com/chaunsin/netease-cloud-music/pkg/utils"
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
		Example: "  ncmctl crypto decrypt -k weapi 'ciphertext'\n  ncmctl crypto decrypt http_request.har (automatic identification of encryption types)",
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

func (c *decryptCmd) execute(_ context.Context, args []string) error {
	var (
		opts  = c.root.opts
		input string
	)
	if c.encode != "string" && c.encode != "base64" && c.encode != "hex" {
		return fmt.Errorf("%s is unknown encode", c.encode)
	}

	if len(args) == 0 {
		return errors.New("nothing was entered")
	}

	input = args[0]

	if utils.IsFile(input) {
		data, err := os.ReadFile(input)
		if err != nil {
			return fmt.Errorf("ReadFile: %w", err)
		}

		if filepath.Ext(input) == ".har" {
			list, parseErr := c.parseHar(data)
			if parseErr != nil {
				return fmt.Errorf("parseHar: %w", parseErr)
			}

			log.Debugf("parseHar data: %+v", list)

			for i := range list {
				if decryptErr := c.decryptReq(&list[i], "hex"); decryptErr != nil {
					return fmt.Errorf("decryptReq: %w", decryptErr)
				}

				if decryptErr := c.decryptRes(&list[i], ""); decryptErr != nil {
					return fmt.Errorf("decryptRes: %w", decryptErr)
				}
			}

			content, marshalErr := json.MarshalIndent(list, "", "  ")
			if marshalErr != nil {
				return marshalErr
			}
			return writeFile(c.cmd, opts.Output, content) // 执行结束
		}

		input = string(data)
	}

	payload := &Payload{
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
		return errors.New("request chiphertext is nil or empty")
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
			p.Request.Plaintext = []byte(payload)
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
		return errors.New("response chiphertext is nil or empty")
	}

	log.Debugf("[decryptRes] response: %+v", p.Response)

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
			log.Debugf("[decryptRes] EApiDecrypt: %s", string(data))

			// 当请返回的内容content-encoding: br时,返回的内容是加密后的需要gzip在次解析,简单来说就是解密流程是这样
			// 1. br解压缩
			// 2. 调用eapi解密方法
			// 3. 调用gzip进行解压缩
			if utils.IsGzipHeader(data) {
				gr, err := gzip.NewReader(bytes.NewReader(data))
				if err != nil {
					return fmt.Errorf("gzip.NewReader: %w", err)
				}

				gdata, err := io.ReadAll(gr)
				if err != nil {
					return fmt.Errorf("ReadAll: %w", err)
				}

				str = string(gdata)
				log.Debugf("[decryptRes] gzip.NewReader: %s", str)
			}

			if len(value) == 3 {
				payload = value[1]
			} else {
				payload = str
			}

			p.Response.Plaintext = []byte(payload)
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
		return nil, errors.New("request data is empty")
	}

	resp := make([]Payload, 0, h.EntryTotal())
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
		// 如果地址不匹配则跳过
		matched, err := isMatch(c.url, _url.Path)
		if err != nil {
			return nil, fmt.Errorf("match URL path: %w", err)
		}

		if !matched {
			continue
		}

		value := strings.Split(_url.Path, "/")

		var kind string
		if len(value) >= 2 {
			// 如果地址是这样 https://music.163.com/api/eapi/nos/token/alloc 则返回eapi
			kind = value[1]
			for _, v := range value {
				var found bool

				switch v {
				case "eapi":
					kind = v
					found = true
				case "weapi":
					kind = v
					found = true
				}

				if found {
					break
				}
			}

			item.Kind = kind
		} else {
			log.Warnf("request url invalid: %s", _url.Path)
			// 如果没有匹配到kind,则使用默认的kind
			item.Kind = c.root.opts.Kind
		}

		// 解析request请求参数
		pd := req.PostData
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
		} else if strings.HasPrefix(pd.MimeType, "application/x-www-form-urlencoded") {
			values, err := url.ParseQuery(pd.Text)
			if err != nil {
				return nil, fmt.Errorf("ParseQuery: %w", err)
			}

			if values.Has("params") {
				item.Request.Ciphertext = values.Get("params")
			} else {
				c.cmd.Printf("params is not found, detail: %+v\n", pd)
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

func isMatch(pattern, text string) (bool, error) {
	decodedPattern, err := url.PathUnescape(pattern)
	if err != nil {
		return false, fmt.Errorf("PathUnescape: %w", err)
	}

	pattern = decodedPattern
	pattern = strings.ReplaceAll(pattern, ".", `\.`)
	pattern = strings.ReplaceAll(pattern, "*", `.*`)
	pattern = "^" + pattern + "$"

	match, err := regexp.MatchString(pattern, text)
	if err != nil {
		return false, fmt.Errorf("MatchString: %w", err)
	}
	return match, nil
}

type Payload struct {
	Api      string   `json:"api,omitempty"`
	Method   string   `json:"method,omitempty"`
	Kind     string   `json:"kind,omitempty"`
	Status   string   `json:"status,omitempty"`
	Request  Request  `json:"request"`
	Response Response `json:"response,omitzero"`
}

type Request struct {
	Ciphertext   string          `json:"ciphertext,omitempty"`
	RawPlaintext string          `json:"rawPlaintext,omitempty"`
	Url          string          `json:"url,omitempty"`
	Digest       string          `json:"digest,omitempty"`
	Plaintext    json.RawMessage `json:"plaintext,omitempty"`
}

type Response struct {
	Ciphertext string          `json:"ciphertext,omitempty"`
	Plaintext  json.RawMessage `json:"plaintext,omitempty"`
}

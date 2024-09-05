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

package api

import "net/http"

type CryptoMode string

const (
	CryptoModeAPI   CryptoMode = "api"
	CryptoModeEAPI  CryptoMode = "eapi"
	CryptoModeWEAPI CryptoMode = "weapi"
	CryptoModeLinux CryptoMode = "linux"
)

type Options struct {
	Method     string
	CryptoMode CryptoMode
	Headers    map[string]string
	Cookies    []*http.Cookie
}

func (o *Options) SetCookies(c ...*http.Cookie) {
	o.Cookies = append(o.Cookies, c...)
}

func (o *Options) SetHeader(key, value string) *Options {
	o.Headers[key] = value
	return o
}

func (o *Options) SetHeaders(h map[string]string) *Options {
	for k, v := range h {
		o.Headers[k] = v
	}
	return o
}

func NewOptions() *Options {
	return &Options{
		Method:     http.MethodPost,
		CryptoMode: CryptoModeWEAPI,
		Headers:    make(map[string]string),
		Cookies:    []*http.Cookie{},
	}
}

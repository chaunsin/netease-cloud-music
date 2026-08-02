// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import "net/http"

type CryptoMode string

const (
	CryptoModeAPI   CryptoMode = "api"
	CryptoModeEAPI  CryptoMode = "eapi"
	CryptoModeWEAPI CryptoMode = "weapi"
	CryptoModeLinux CryptoMode = "linux"
	CryptoModeXEAPI CryptoMode = "xeapi"
)

type Options struct {
	Method     string
	CryptoMode CryptoMode
	Headers    http.Header
	Cookies    []*http.Cookie
}

func NewOptions() *Options {
	return &Options{
		Method:     http.MethodPost,
		CryptoMode: CryptoModeWEAPI,
		Headers:    make(http.Header),
		Cookies:    []*http.Cookie{},
	}
}

func (o *Options) SetAPI() *Options {
	o.CryptoMode = CryptoModeAPI
	return o
}

func (o *Options) SetEAPI() *Options {
	o.CryptoMode = CryptoModeEAPI
	return o
}

func (o *Options) SetWEAPI() *Options {
	o.CryptoMode = CryptoModeWEAPI
	return o
}

func (o *Options) SetXEAPI() *Options {
	o.CryptoMode = CryptoModeXEAPI
	return o
}

func (o *Options) SetLinuxAPI() *Options {
	o.CryptoMode = CryptoModeLinux
	return o
}

func (o *Options) SetMethod(method string) *Options {
	o.Method = method
	return o
}

func (o *Options) SetCookies(c ...*http.Cookie) {
	for _, cookie := range c {
		if cookie != nil {
			o.Cookies = append(o.Cookies, cookie)
		}
	}
}

func (o *Options) GetCookie(key string) *http.Cookie {
	for _, c := range o.Cookies {
		if c == nil {
			continue
		}

		if c.Name == key {
			return c // TODO: 是否存在问题，返回的对象用户操作是否会影响此值，应该采用copy方式
		}
	}
	return nil
}

func (o *Options) SetHeader(key, value string) *Options {
	if o.Headers == nil {
		o.Headers = make(http.Header)
	}

	o.Headers.Set(key, value)
	return o
}

func (o *Options) SetHeaders(h map[string]string) *Options {
	for k, v := range h {
		o.SetHeader(k, v)
	}
	return o
}

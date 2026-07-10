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
	Method      string
	CryptoMode  CryptoMode
	Headers     map[string]string
	Cookies     []*http.Cookie
	XeapiOS     string
	XeapiAppVer string
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

func (o *Options) SetXeapiOS(os string) *Options {
	o.XeapiOS = os
	return o
}

func (o *Options) SetXeapiAppVer(appVer string) *Options {
	o.XeapiAppVer = appVer
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

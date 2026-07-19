// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"maps"
	"net/http"
)

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

func NewOptions() *Options {
	return &Options{
		Method:     http.MethodPost,
		CryptoMode: CryptoModeWEAPI,
		Headers:    make(map[string]string),
		Cookies:    []*http.Cookie{},
	}
}

func (o *Options) SetCryptoModeAPI() *Options {
	o.CryptoMode = CryptoModeAPI
	return o
}

func (o *Options) SetCryptoModeEAPI() *Options {
	o.CryptoMode = CryptoModeEAPI
	return o
}

func (o *Options) SetCryptoModeWEAPI() *Options {
	o.CryptoMode = CryptoModeWEAPI
	return o
}

func (o *Options) SetCryptoModeLinux() *Options {
	o.CryptoMode = CryptoModeLinux
	return o
}

func (o *Options) SetCryptoModeXEAPI() *Options {
	o.CryptoMode = CryptoModeXEAPI
	return o
}

func (o *Options) SetMethod(method string) *Options {
	o.Method = method
	return o
}

func (o *Options) SetCookies(c ...*http.Cookie) {
	o.Cookies = append(o.Cookies, c...)
}

func (o *Options) SetHeader(key, value string) *Options {
	o.Headers[key] = value
	return o
}

func (o *Options) SetHeaders(h map[string]string) *Options {
	maps.Copy(o.Headers, h)
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

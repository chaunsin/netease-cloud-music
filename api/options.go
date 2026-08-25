// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
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
	Func       string     // 请求接口方法名用于log记录使用
	Method     string     // http 请求方法
	CryptoMode CryptoMode // 加密算法类型
	Headers    http.Header

	cookies     []*http.Cookie
	cookieIndex map[string]int
	cookieErr   error
}

func NewOptions(fn ...string) *Options {
	var f string
	if len(fn) > 0 {
		f = fn[0]
	}
	return &Options{
		Func:       f,
		Method:     http.MethodPost,
		CryptoMode: CryptoModeWEAPI,
		Headers:    make(http.Header),
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

// SetCookies sets request-scoped Cookies. A failed batch leaves the existing
// layer unchanged and is reported when the Options are used by Request.
func (o *Options) SetCookies(cookies ...*http.Cookie) {
	if o.cookieErr != nil {
		return
	}

	batch := make([]*http.Cookie, 0, len(cookies))
	for index, cookie := range cookies {
		if cookie == nil {
			continue
		}

		clone := cloneCookie(cookie)
		if err := clone.Valid(); err != nil {
			o.cookieErr = fmt.Errorf("option Cookie %q at index %d is invalid: %w", clone.Name, index, err)
			return
		}

		batch = append(batch, clone)
	}

	for _, cookie := range batch {
		o.setCookie(cookie)
	}
}

func (o *Options) GetCookie(key string) *http.Cookie {
	index, ok := o.cookieIndex[key]
	if !ok {
		return nil
	}

	return cloneCookie(o.cookies[index])
}

// SetHeader 设置请求头采用覆盖模式，禁止设置Cookie请求头，应该使用 SetCookies方法。
func (o *Options) SetHeader(key, value string) *Options {
	if o.Headers == nil {
		o.Headers = make(http.Header)
	}

	if strings.EqualFold(key, "cookie") {
		return o
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

func (o *Options) clone() *Options {
	clone := &Options{
		Method:     o.Method,
		CryptoMode: o.CryptoMode,
		Headers:    o.Headers.Clone(),
		cookies:    cloneCookies(o.cookies),
		cookieErr:  o.cookieErr,
	}

	if clone.Method == "" {
		clone.Method = http.MethodPost
	}

	if len(clone.cookies) > 0 {
		clone.cookieIndex = make(map[string]int, len(clone.cookies))
		for index, cookie := range clone.cookies {
			clone.cookieIndex[cookie.Name] = index
		}
	}
	return clone
}

func (o *Options) setCookie(cookie *http.Cookie) {
	if o.cookieIndex == nil {
		o.cookieIndex = make(map[string]int)
	}

	if index, ok := o.cookieIndex[cookie.Name]; ok {
		o.cookies = append(o.cookies[:index], o.cookies[index+1:]...)
		for shifted := index; shifted < len(o.cookies); shifted++ {
			o.cookieIndex[o.cookies[shifted].Name] = shifted
		}
	}

	o.cookies = append(o.cookies, cookie)
	o.cookieIndex[cookie.Name] = len(o.cookies) - 1
}

func (o *Options) cookieSnapshot() []*http.Cookie {
	return cloneCookies(o.cookies)
}

func cloneCookies(cookies []*http.Cookie) []*http.Cookie {
	clones := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}

		clones = append(clones, cloneCookie(cookie))
	}
	return clones
}

func cloneCookie(cookie *http.Cookie) *http.Cookie {
	if cookie == nil {
		return nil
	}

	clone := *cookie
	clone.Unparsed = slices.Clone(cookie.Unparsed)
	return &clone
}

func cloneOptions(options *Options) *Options {
	if options == nil {
		return NewOptions()
	}

	return options.clone()
}

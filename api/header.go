// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// defaultHeaders 由于是手动配置需要满足规范, cookie注意大写,header需要满足 http.CanonicalHeaderKey() 规范。
var defaultHeaders = &Headers{
	// API 需要一个真实场景的内容填充.
	API: HeaderItem{
		Cookie: map[string]string{
			"channel":       "channel",
			"ntes_kaola_ad": "1",
			"WEVNSM":        "1.0",
			"deviceId":      "",
			"os":            "osx",
			"osver":         "15.3.2",
		},
		Header: map[string][]string{
			"User-Agent": {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"},
		},
	},
	EAPI: HeaderItem{
		Cookie: map[string]string{
			"channel":       "netease",
			"ntes_kaola_ad": "1",
			"WEVNSM":        "1.0",
			"deviceId":      "",
			"appver":        "9.2.85",       // 要和UserAgent中一致
			"os":            "android",      // 要和UserAgent中一致
			"osver":         "9",            // 要和UserAgent中一致
			"buildver":      "250418145357", // 要和UserAgent中一致
			"resolution":    "1600x900",
			"mobilename":    "SM-S9180",
			"brand":         "samsung",
			"versioncode":   "9002085", // 要和UserAgent中一致
		},
		Header: map[string][]string{
			"User-Agent": {"NeteaseMusic/9.2.85.250418145357(9002085);Dalvik/2.1.0 (Linux; U; Android 9; SM-S9180 Build/PQ3B.190801.10101846)"},
		},
	},
	WEAPI: HeaderItem{
		Cookie: map[string]string{
			"channel":       "appstore",
			"ntes_kaola_ad": "1",
			"WEVNSM":        "1.0",
			"deviceId":      "",
			"appver":        "3.0.12",         // 要和UserAgent中一致
			"os":            "osx",            // 要和UserAgent中一致
			"osver":         "15.3.2",         // 要和UserAgent中一致
			"mode":          "MacBookPro16,1", // 要和UserAgent中一致
			"_iuqxldmzr_":   "33",
		},
		Header: map[string][]string{
			"User-Agent": {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/3.0.12.2443"},
		},
	},
	XEAPI: HeaderItem{
		Cookie: map[string]string{
			"channel":       "netease",
			"ntes_kaola_ad": "1",
			"WEVNSM":        "1.0",
			"deviceId":      "",
			"appver":        "9.2.85",       // 要和UserAgent中一致
			"os":            "android",      // 要和UserAgent中一致
			"osver":         "9",            // 要和UserAgent中一致
			"buildver":      "250418145357", // 要和UserAgent中一致
			"resolution":    "1600x900",
			"mobilename":    "SM-S9180",
			"brand":         "samsung",
			"versioncode":   "9002085", // 要和UserAgent中一致
		},
		Header: map[string][]string{
			"User-Agent": {defaultXeapiUserAgent},
		},
	},
	LinuxAPI: HeaderItem{
		Cookie: map[string]string{
			"channel":       "netease",
			"ntes_kaola_ad": "1",
			"WEVNSM":        "1.0",
			"deviceId":      "",
			"appver":        "1.2.1.0428",
			"os":            "linux",
			"osver":         "Deepin 20.9",
		},
		Header: map[string][]string{
			"User-Agent": {"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.90 Safari/537.36"},
		},
	},
}

var (
	cookieNameSet      = map[string]struct{}{"channel": {}, "ntes_kaola_ad": {}, "WEVNSM": {}}
	xeapiCookieNameSet = map[string]struct{}{"appver": {}, "buildver": {}, "mobilename": {}, "os": {}, "osver": {}}
	headerNameSet      = map[string]struct{}{"User-Agent": {}}
)

type HeaderItem struct {
	Cookie map[string]string `json:"cookie" yaml:"cookie"` // cookie值的key有大写小限制
	Header http.Header       `json:"header" yaml:"header"` // header无大小写限制
}

// Validate 校验通用必须要的值，未判定值有效性。
func (h *HeaderItem) Validate() error {
	if len(h.Cookie) == 0 || len(h.Header) == 0 {
		return errors.New("cookie or header is empty. ")
	}

	for k := range cookieNameSet {
		v, ok := h.Cookie[k]
		if !ok || strings.TrimSpace(v) == "" {
			return fmt.Errorf("'%s' cookie value required. ", k)
		}
	}

	for k := range headerNameSet {
		v := h.Header.Get(k)
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("'%s' header value required. ", k)
		}
	}

	return nil
}

// GetCookie 获取cookie value值,区分大小写.
func (h *HeaderItem) GetCookie(name string) string {
	if h == nil || len(h.Cookie) == 0 {
		return ""
	}
	return h.Cookie[name]
}

// GetHeader 获取header value值，不区分大小写.
func (h *HeaderItem) GetHeader(name string) string {
	if h == nil || len(h.Header) == 0 {
		return ""
	}
	return h.Header.Get(name)
}

// Get 从cookie和header中获取值，优先获取cookie然后header，遇到第一个不为空则返回。
func (h *HeaderItem) Get(name string) string {
	value := h.GetCookie(name)
	if value != "" {
		return value
	}
	return h.GetHeader(name)
}

func (h *HeaderItem) clone() HeaderItem {
	return HeaderItem{Cookie: maps.Clone(h.Cookie), Header: h.Header.Clone()}
}

type Headers struct {
	API      HeaderItem `json:"api" yaml:"api"`
	EAPI     HeaderItem `json:"eapi" yaml:"eapi"`
	WEAPI    HeaderItem `json:"weapi" yaml:"weapi"`
	XEAPI    HeaderItem `json:"xeapi" yaml:"xeapi"`
	LinuxAPI HeaderItem `json:"linuxapi" yaml:"linuxapi"`
}

func (h *Headers) Validate() error {
	if err := h.API.Validate(); err != nil {
		return fmt.Errorf("api: %w", err)
	}

	if err := h.EAPI.Validate(); err != nil {
		return fmt.Errorf("eapi: %w", err)
	}

	if err := h.WEAPI.Validate(); err != nil {
		return fmt.Errorf("weapi: %w", err)
	}

	if err := h.XEAPI.Validate(); err != nil {
		return fmt.Errorf("xeapi: %w", err)
	}

	for k := range xeapiCookieNameSet {
		if strings.TrimSpace(h.XEAPI.Cookie[k]) == "" {
			return fmt.Errorf("xeapi: '%s' cookie value required. ", k)
		}
	}

	if err := h.LinuxAPI.Validate(); err != nil {
		return fmt.Errorf("linuxapi: %w", err)
	}

	return nil
}

// Load 从配置中加载并校验,考虑局部覆盖和整体覆盖两种方式.
func (h *Headers) Load(filename string) error {
	if h == nil {
		return errors.New("headers is nil")
	}

	if filename == "" {
		return errors.New("config file name is empty")
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("readfile err: %w", err)
	}

	candidate := h.clone()

	switch ext := filepath.Ext(filename); ext {
	case ".json":
		err = json.Unmarshal(data, &candidate)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &candidate)
	default:
		return fmt.Errorf("not support file ext type'%s'", ext)
	}

	if err != nil {
		return err
	}

	if err = candidate.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	*h = candidate
	return nil
}

// GetDefValue 根据加密模式获取对应请求头相关默认配置.
func (h *Headers) GetDefValue(mode CryptoMode) *HeaderItem {
	switch mode {
	case CryptoModeAPI:
		return &h.API
	case CryptoModeEAPI:
		return &h.EAPI
	case CryptoModeWEAPI:
		return &h.WEAPI
	case CryptoModeXEAPI:
		return &h.XEAPI
	case CryptoModeLinux:
		return &h.LinuxAPI
	default:
		return nil
	}
}

func (h *Headers) clone() Headers {
	if h == nil {
		return Headers{}
	}

	return Headers{
		API:      h.API.clone(),
		EAPI:     h.EAPI.clone(),
		WEAPI:    h.WEAPI.clone(),
		XEAPI:    h.XEAPI.clone(),
		LinuxAPI: h.LinuxAPI.clone(),
	}
}

// getDefHeaderVal header头加载顺序与优先级:
// 用户接口传入 > 系统默认.
func (c *Client) getDefHeaderVal(name string, userHeader http.Header, defVal string) string {
	// 返回用户传入值
	var userVal string
	if userHeader != nil {
		userVal = userHeader.Get(name)
	}

	if userVal != "" {
		return userVal
	}

	// 返回系统默认
	return defVal
}

// getDefCookie 从cookie中加载顺序与优先级:
// 用户接口传入 > 已写入cookiejar > 系统默认.
func (c *Client) getDefCookie(name string, userCookie []*http.Cookie, defVal string) *http.Cookie {
	// 返回用户传入值
	if len(userCookie) > 0 {
		for _, ck := range userCookie {
			if ck == nil {
				continue
			}

			val := strings.TrimSpace(ck.Value)
			if ck.Name == name && val != "" {
				return ck
			}
		}
	}

	// 从当前cookiejar中获取值.
	// NOTE: 暂时定位从 https://music.163.com 中获取，其他127.com、126.com等等域名得数据不支持获取。
	cookieVal, ok := c.Cookie("https://music.163.com", name)
	if ok && cookieVal.Value != "" {
		return &cookieVal
	}

	value := strings.TrimSpace(defVal)
	if value == "" {
		return nil
	}

	// 返回系统默认,domain暂时为 music.163.com
	return &http.Cookie{Name: name, Value: value, Domain: defDomain}
}

// getDefVal 从header和cookie中获取，先从cookie中获取，然后从header中获取,找到第一个对象后返回.
// 注意: name区分大小写cookie获取场景.
func (c *Client) getDefVal(name string, opts *Options, defVal *HeaderItem) string {
	if opts == nil {
		if defVal == nil {
			return ""
		}
		return defVal.Get(name)
	}

	// value := c.getDefCookieVal(name, opts.Cookies, defVal.GetCookie(name))
	// if value != "" {
	// 	return value
	// }

	cookie := c.getDefCookie(name, opts.Cookies, defVal.GetCookie(name))
	if cookie != nil && cookie.Value != "" {
		return cookie.Value
	}

	return c.getDefHeaderVal(name, opts.Headers, defVal.GetHeader(name))
}

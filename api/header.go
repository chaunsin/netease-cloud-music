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
	"slices"

	"gopkg.in/yaml.v3"
)

const defUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) NeteaseMusicDesktop/2.3.17.1034"

// defaultHeaders 由于是手动配置需要满足规范, cookie注意大写,header需要满足 http.CanonicalHeaderKey() 规范。
// 当前配置的值不能随意修改，以免造成apl中得Request空指针问题。
var defaultHeaders = &Headers{
	// API 需要一个真实场景的内容填充.
	API: HeaderItem{
		Cookie: map[string]string{
			"channel":       "netease",
			"ntes_kaola_ad": "1",
			"WEVNSM":        "1.0",
			"os":            "osx",
			"osver":         "15.3.2",
			// "deviceId":      "7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62|5FD718A3-0602-4389-B612-EBEFAA7F108B",
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
			"appver":        "9.2.85",       // 要和UserAgent中一致
			"os":            "android",      // 要和UserAgent中一致
			"osver":         "9",            // 要和UserAgent中一致
			"buildver":      "250418145357", // 要和UserAgent中一致
			"resolution":    "1600x900",
			"mobilename":    "SM-S9180",
			"brand":         "samsung",
			"versioncode":   "9002085", // 要和UserAgent中一致
			"packageType":   "release", // beta、release
			"deviceId":      "MzUxNTY0MTAxMTE4NDEyCTA4OjAwOjI3OjRmOjEyOmJhCThjMzczZDE5ODk3ODc2M2EJOTZiOGEzZjBmYzgyMDgxNw==",
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
			"appver":        "3.0.12",         // 要和UserAgent中一致
			"os":            "osx",            // 要和UserAgent中一致
			"osver":         "15.3.2",         // 要和UserAgent中一致
			"mode":          "MacBookPro16,1", // 要和UserAgent中一致
			"_iuqxldmzr_":   "33",
			// "deviceId":      "7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62|5FD718A3-0602-4389-B612-EBEFAA7F108B",
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
			"appver":        "9.2.85",       // 要和UserAgent中一致
			"os":            "android",      // 要和UserAgent中一致
			"osver":         "9",            // 要和UserAgent中一致
			"buildver":      "250418145357", // 要和UserAgent中一致
			"resolution":    "1600x900",
			"mobilename":    "SM-S9180",
			"brand":         "samsung",
			"versioncode":   "9002085", // 要和UserAgent中一致
			"packageType":   "release", // beta、release
			"deviceId":      "MzUxNTY0MTAxMTE4NDEyCTA4OjAwOjI3OjRmOjEyOmJhCThjMzczZDE5ODk3ODc2M2EJOTZiOGEzZjBmYzgyMDgxNw==",
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
			"appver":        "1.2.1.0428",
			"os":            "linux",
			"osver":         "Deepin 20.9",
			// "deviceId":      "7A8EB581-E60B-5230-BB5B-E6DAB1FBFA62|5FD718A3-0602-4389-B612-EBEFAA7F108B",
		},
		Header: map[string][]string{
			"User-Agent": {"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.90 Safari/537.36"},
		},
	},
}

type HeaderItem struct {
	Cookie map[string]string `json:"cookie" yaml:"cookie"` // cookie值的key有大写小限制
	Header http.Header       `json:"header" yaml:"header"` // header无大小写限制
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
	sections := []struct {
		name     string
		actual   *HeaderItem
		expected *HeaderItem
	}{
		{name: "api", actual: &h.API, expected: &defaultHeaders.API},
		{name: "eapi", actual: &h.EAPI, expected: &defaultHeaders.EAPI},
		{name: "weapi", actual: &h.WEAPI, expected: &defaultHeaders.WEAPI},
		{name: "xeapi", actual: &h.XEAPI, expected: &defaultHeaders.XEAPI},
		{name: "linuxapi", actual: &h.LinuxAPI, expected: &defaultHeaders.LinuxAPI},
	}

	for _, sec := range sections {
		for k := range sec.expected.Header {
			if sec.actual.Header.Get(k) == "" {
				return fmt.Errorf("%s: %v header value required. ", sec.name, k)
			}
		}

		for k := range sec.expected.Cookie {
			if sec.actual.GetCookie(k) == "" {
				return fmt.Errorf("%s: %v cookie value required. ", sec.name, k)
			}
		}

		if err := validateHeaderCookies(sec.name, sec.actual.Cookie); err != nil {
			return err
		}
	}

	return nil
}

func validateHeaderCookies(mode string, cookies map[string]string) error {
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}

	slices.Sort(names)

	for _, name := range names {
		if err := (&http.Cookie{Name: name, Value: cookies[name]}).Valid(); err != nil {
			return fmt.Errorf("%s: Cookie %q is invalid", mode, name)
		}
	}
	return nil
}

// LoadConfig 从配置中加载并校验,考虑局部覆盖和整体覆盖两种方式.
func (h *Headers) LoadConfig(filename string) error {
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

// HeaderItemForCryptoMode 根据加密模式获取对应请求头配置.
func (h *Headers) HeaderItemForCryptoMode(mode CryptoMode) *HeaderItem {
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

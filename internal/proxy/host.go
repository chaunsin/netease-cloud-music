// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"errors"
	"net"
	"strings"
)

var defaultProxyDomains = []string{
	"music.163.com",
	"music.126.net",
	"vod.126.net",
	"iplay.163.com",
	"look.163.com",
	"y.163.com",
	"163yun.com",
	"163jiasu.com",
	"netease.com",
	"acstatic-dun.126.net",
}

// DefaultDomains returns the NetEase domains intercepted by the proxy.
func DefaultDomains() []string {
	return append([]string(nil), defaultProxyDomains...)
}

type hostMatcher struct {
	domains []string
}

func newHostMatcher(domains []string) (*hostMatcher, error) {
	if len(domains) == 0 {
		return nil, errors.New("at least one proxy domain is required")
	}

	normalized := make([]string, 0, len(domains))

	seen := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		domain = canonicalHostname(domain)
		if domain == "" {
			return nil, errors.New("proxy domain cannot be empty")
		}

		if _, ok := seen[domain]; ok {
			continue
		}

		seen[domain] = struct{}{}
		normalized = append(normalized, domain)
	}

	return &hostMatcher{domains: normalized}, nil
}

func (m *hostMatcher) Match(host string) bool {
	if m == nil {
		return false
	}

	host = canonicalHostname(host)
	if host == "" {
		return false
	}

	for _, domain := range m.domains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func canonicalHostname(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}

	return strings.ToLower(strings.TrimRight(host, "."))
}

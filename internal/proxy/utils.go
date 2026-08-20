// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net"
	"net/url"
	"strings"
)

func formatFingerprint(raw []byte) string {
	var (
		fingerprint = sha256.Sum256(raw)
		parts       = make([]string, len(fingerprint))
	)
	for i, value := range fingerprint {
		parts[i] = fmt.Sprintf("%02X", value)
	}
	return strings.Join(parts, ":")
}

func cloneURL(input *url.URL) *url.URL {
	if input == nil {
		return &url.URL{}
	}

	cloned := *input
	return &cloned
}

func isLoopbackListenAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}

	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isIPConnectTarget(target string) bool {
	host, _, err := net.SplitHostPort(target)
	return err == nil && net.ParseIP(host) != nil
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

func hasPathPrefix(value, prefix string) bool {
	return value == prefix || strings.HasPrefix(value, prefix+"/")
}

func isHex(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || len(trimmed)%2 != 0 {
		return false
	}

	for _, c := range trimmed {
		if (c < '0' || c > '9') &&
			(c < 'a' || c > 'f') &&
			(c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

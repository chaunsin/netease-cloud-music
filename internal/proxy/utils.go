package proxy

import (
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

func hasPathPrefix(value, prefix string) bool {
	return value == prefix || strings.HasPrefix(value, prefix+"/")
}

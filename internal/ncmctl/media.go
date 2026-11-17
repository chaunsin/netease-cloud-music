// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// mediaPolicy 选择远程媒体下载的 URL 校验与传输加固强度。
type mediaPolicy struct {
	// normalizeURL 在请求前校验并可能改写 URL；nil 表示按原样请求。
	normalizeURL func(string) (string, error)
	// newClient 构造下载客户端，返回的 func 在请求结束后释放它持有的连接。
	newClient func(time.Duration) (*http.Client, func())
	// sniffSuffix 从响应内容前缀推断临时文件后缀；nil 表示沿用 prefix。
	sniffSuffix func([]byte) (string, error)
	// rejectEmpty 为 true 时把零字节响应视为失败。
	rejectEmpty bool
}

var (
	// mediaStrict 用于云贝：URL 必须落在网易媒体域并升级为 HTTPS，解析出的每个地址
	// 都要通过公网校验，重定向逐跳复校验，且不经过环境代理。
	mediaStrict = mediaPolicy{
		normalizeURL: normalizeMediaURL,
		newClient: func(timeout time.Duration) (*http.Client, func()) {
			client := newHardenedMediaClient(timeout)
			return client, client.CloseIdleConnections
		},
		sniffSuffix: sniffImageSuffix,
		rejectEmpty: true,
	}

	// mediaPlain 保留 share 与 fansgroup 既有的下载行为：不校验 URL、
	// 使用只设超时的默认客户端、不嗅探后缀、允许零字节响应。
	mediaPlain = mediaPolicy{
		newClient: func(timeout time.Duration) (*http.Client, func()) {
			return &http.Client{Timeout: timeout}, func() {}
		},
	}
)

// downloadMediaToTemp 按 limit 上限把 rawURL 下载到临时文件, 返回路径与删除它的 cleanup.
func downloadMediaToTemp(ctx context.Context, policy mediaPolicy, timeout time.Duration, rawURL string, limit int64, prefix string) (string, func(), error) {
	client, release := policy.newClient(timeout)
	defer release()

	return downloadMediaToTempWithClient(ctx, policy, client, rawURL, limit, prefix)
}

func downloadMediaToTempWithClient(ctx context.Context, policy mediaPolicy, client *http.Client, rawURL string, limit int64, prefix string) (string, func(), error) {
	if client == nil {
		return "", func() {}, errors.New("media download client is missing")
	}

	if limit <= 0 {
		return "", func() {}, errors.New("media download limit must be positive")
	}

	response, err := mediaRequest(ctx, policy, client, rawURL, nil, "media download")
	if err != nil {
		return "", func() {}, err
	}

	defer func() { _ = response.Body.Close() }()

	reader := bufio.NewReader(response.Body)

	suffix := ""

	if policy.sniffSuffix != nil {
		sample, peekErr := reader.Peek(512)
		if peekErr != nil && !errors.Is(peekErr, io.EOF) {
			return "", func() {}, fmt.Errorf("inspect downloaded media: %w", peekErr)
		}

		detected, sniffErr := policy.sniffSuffix(sample)
		if sniffErr != nil {
			return "", func() {}, fmt.Errorf("downloaded media: %w", sniffErr)
		}

		suffix = detected
	}

	if !strings.Contains(prefix, "*") {
		prefix += "*"
	}

	temporary, err := os.CreateTemp("", prefix+suffix)
	if err != nil {
		return "", func() {}, fmt.Errorf("create media temporary file: %w", err)
	}

	name := filepath.Clean(temporary.Name())
	cleanup := func() { _ = os.Remove(name) }

	written, copyErr := io.Copy(temporary, io.LimitReader(reader, limit+1))
	if closeErr := temporary.Close(); copyErr == nil {
		copyErr = closeErr
	}

	if copyErr != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("write media temporary file: %w", copyErr)
	}

	if written == 0 && policy.rejectEmpty {
		cleanup()
		return "", func() {}, errors.New("downloaded media is empty")
	}

	if written > limit {
		cleanup()
		return "", func() {}, fmt.Errorf("downloaded %d bytes exceeds %d limit", written, limit)
	}
	return name, cleanup, nil
}

// fetchMediaAudioSample 以 Range 请求限量拉取音频数据模拟播放缓冲, 遵循 ctx 取消;
// 上报时长与实际拉取解耦。
func fetchMediaAudioSample(ctx context.Context, policy mediaPolicy, timeout time.Duration, rawURL string, limit int64) error {
	client, release := policy.newClient(timeout)
	defer release()

	return fetchMediaAudioSampleWithClient(ctx, policy, client, rawURL, limit)
}

func fetchMediaAudioSampleWithClient(ctx context.Context, policy mediaPolicy, client *http.Client, rawURL string, limit int64) error {
	if client == nil {
		return errors.New("audio sample client is missing")
	}

	// fail-fast: limit<=0 会构造非法 Range 头 (bytes=0--1)
	if limit <= 0 {
		return errors.New("audio sample limit must be positive")
	}

	header := http.Header{"Range": []string{fmt.Sprintf("bytes=0-%d", limit-1)}}

	response, err := mediaRequest(ctx, policy, client, rawURL, header, "audio sample")
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()

	read, err := io.Copy(io.Discard, io.LimitReader(response.Body, limit))
	if err != nil {
		return fmt.Errorf("read audio sample: %w", err)
	}

	if read == 0 && policy.rejectEmpty {
		return errors.New("audio sample is empty")
	}
	return nil
}

// mediaRequest 按策略校验并发起 GET, 只接受 2xx; 调用方负责关闭成功响应的 Body。
func mediaRequest(ctx context.Context, policy mediaPolicy, client *http.Client, rawURL string, header http.Header, op string) (*http.Response, error) {
	target := rawURL

	if policy.normalizeURL != nil {
		normalized, err := policy.normalizeURL(rawURL)
		if err != nil {
			return nil, err
		}

		target = normalized
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", op, err)
	}

	maps.Copy(request.Header, header)

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if response.StatusCode/100 != 2 {
		defer func() { _ = response.Body.Close() }()

		return nil, fmt.Errorf("%s: HTTP %s", op, response.Status)
	}
	return response, nil
}

func sniffImageSuffix(sample []byte) (string, error) {
	if len(sample) == 0 {
		return "", errors.New("image content is empty")
	}

	switch http.DetectContentType(sample) {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/gif":
		return ".gif", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", errors.New("supported image types are JPEG, PNG, GIF and WebP")
	}
}

func normalizeMediaURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse media URL: %w", err)
	}

	if parsed.Scheme == "https" {
		if err := validateMediaURL(parsed.String()); err != nil {
			return "", err
		}
		return parsed.String(), nil
	}

	if parsed.Scheme != "http" {
		return "", errors.New("media URL must use HTTP or HTTPS")
	}

	if parsed.User != nil {
		return "", errors.New("media URL must not contain user information")
	}

	if parsed.Port() != "" {
		return "", errors.New("HTTP media URL must use the default port")
	}

	host := strings.ToLower(parsed.Hostname())
	if !isMusicMediaHost(host) {
		return "", fmt.Errorf("media host %q is not allowed", host)
	}

	parsed.Scheme = "https"
	return parsed.String(), nil
}

func validateMediaURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse media URL: %w", err)
	}

	if parsed.Scheme != "https" {
		return errors.New("media URL must use HTTPS")
	}

	if parsed.User != nil {
		return errors.New("media URL must not contain user information")
	}

	if parsed.Port() != "" && parsed.Port() != "443" {
		return errors.New("media URL must use the default HTTPS port")
	}

	host := strings.ToLower(parsed.Hostname())
	if !isMusicMediaHost(host) {
		return fmt.Errorf("media host %q is not allowed", host)
	}
	return nil
}

func isMusicMediaHost(host string) bool {
	for _, suffix := range []string{"music.163.com", "music.126.net"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

func validateMediaIP(address netip.Addr) error {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return fmt.Errorf("media host resolved to disallowed address %s", address)
	}
	return nil
}

func newHardenedMediaClient(timeout time.Duration) *http.Client {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		panic("http.DefaultTransport is not an *http.Transport")
	}

	transport := defaultTransport.Clone()
	transport.Proxy = nil
	transport.ForceAttemptHTTP2 = true
	transport.DialContext = hardenedMediaDialer(timeout)

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return errors.New("media download exceeded 5 redirects")
			}
			return validateMediaURL(req.URL.String())
		},
	}
}

func hardenedMediaDialer(timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return hardenedMediaDialerWithLookup(timeout, net.DefaultResolver.LookupNetIP)
}

type lookupNetIPFunc func(context.Context, string, string) ([]netip.Addr, error)

// hardenedMediaDialerWithLookup 自行解析并校验每个地址后按 IP 字面量拨号,
// 以免校验与拨号之间的 DNS 重绑定把请求导向内网地址。
func hardenedMediaDialerWithLookup(timeout time.Duration, lookup lookupNetIPFunc) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}

	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("split media address: %w", err)
		}

		addresses, err := lookup(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve media host %q: %w", host, err)
		}

		if len(addresses) == 0 {
			return nil, fmt.Errorf("resolve media host %q: no addresses", host)
		}

		for _, resolved := range addresses {
			if err := validateMediaIP(resolved); err != nil {
				return nil, err
			}
		}

		var dialErrors []error

		for _, resolved := range addresses {
			if network == "tcp4" && !resolved.Is4() || network == "tcp6" && !resolved.Is6() {
				continue
			}

			connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
			if dialErr == nil {
				return connection, nil
			}

			dialErrors = append(dialErrors, dialErr)
		}

		if len(dialErrors) == 0 {
			return nil, fmt.Errorf("dial media host %q: no address for network %s", host, network)
		}

		return nil, fmt.Errorf("dial media host %q: %w", host, errors.Join(dialErrors...))
	}
}

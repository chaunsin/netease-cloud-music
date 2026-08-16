// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
)

type cookieTransport struct {
	jar http.CookieJar

	mu        sync.RWMutex
	transport http.RoundTripper
	inFlight  sync.WaitGroup
	closing   bool
	idleMu    sync.Mutex
	closeOnce sync.Once
	closed    chan struct{}
}

var errCookieTransportLoop = errors.New("cookie transport loop detected")

func newCookieTransport(jar http.CookieJar, transport http.RoundTripper) *cookieTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &cookieTransport{jar: jar, transport: transport, closed: make(chan struct{})}
}

func (t *cookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var rejectErr error

	switch {
	case req.Context().Value(t) != nil:
		rejectErr = errCookieTransportLoop
	case !t.beginRoundTrip():
		rejectErr = ErrClientClosed
	}

	if rejectErr != nil {
		if req.Body != nil {
			rejectErr = errors.Join(rejectErr, req.Body.Close())
		}
		return nil, rejectErr
	}

	defer t.inFlight.Done()

	cookieURL := cookieURLForHost(req.URL, req.Host)
	policy, _ := req.Context().Value(cookiePolicyContextKey{}).(*requestCookiePolicy)

	clone := req.Clone(context.WithValue(req.Context(), t, struct{}{}))
	clone.Header.Del("Cookie")

	for _, cookie := range mergeRequestCookies(req.Cookies(), t.jar.Cookies(cookieURL), policy, cookieURL) {
		clone.AddCookie(cookie)
	}

	t.mu.RLock()
	transport := t.transport
	t.mu.RUnlock()

	resp, err := transport.RoundTrip(clone)
	if resp != nil {
		if resp.Request == nil {
			resp.Request = clone
		}

		resp.Request = resp.Request.WithContext(req.Context())
	}

	if err != nil {
		return resp, err
	}

	if resp == nil {
		return nil, errors.New("cookie transport returned nil response")
	}

	// Match http.Client: every received HTTP response updates the Jar.
	// The Jar decides whether an individual Cookie is acceptable.
	if cookies := resp.Cookies(); len(cookies) > 0 {
		t.jar.SetCookies(cookieURL, cookies)
	}
	return resp, nil
}

func (t *cookieTransport) SetTransport(transport http.RoundTripper) {
	if transport == nil {
		return
	}

	t.mu.Lock()
	t.transport = transport
	t.mu.Unlock()
}

// CloseIdleConnections preserves http.Client's optional transport capability.
func (t *cookieTransport) CloseIdleConnections() {
	if !t.idleMu.TryLock() {
		return
	}
	defer t.idleMu.Unlock()

	t.mu.RLock()
	transport := t.transport
	t.mu.RUnlock()

	if closer, ok := transport.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func (t *cookieTransport) Close() {
	t.startClose()
	// Keep the wait outside closeOnce so every caller observes the same drain.
	<-t.closed
}

func (t *cookieTransport) startClose() {
	t.closeOnce.Do(func() {
		t.mu.Lock()
		t.closing = true
		t.mu.Unlock()

		go func() {
			t.inFlight.Wait()
			close(t.closed)
		}()
	})
}

func (t *cookieTransport) beginRoundTrip() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closing {
		return false
	}

	t.inFlight.Add(1)
	return true
}

type cookiePolicyContextKey struct{}

// withRequestCookiePolicy 将一次逻辑请求确定的 Cookie 策略放入 context。
// Resty 重试和 net/http 重定向会生成新的物理请求，context 用于把首跳快照传递到每次 RoundTrip。
func withRequestCookiePolicy(ctx context.Context, policy *requestCookiePolicy) context.Context {
	return context.WithValue(ctx, cookiePolicyContextKey{}, policy)
}

// requestCookiePolicy freezes protocol identity for one logical request while
// keeping automatic Cookie data outside the explicit request Header.
type requestCookiePolicy struct {
	originURL             *url.URL
	options               []*http.Cookie
	optionByName          map[string]*http.Cookie
	jarByName             map[string][]*http.Cookie
	defaults              map[string]*http.Cookie
	frozen                map[string][]*http.Cookie
	frozenNames           []string
	allowProtocolDefaults bool
	finalized             bool
}

func newRequestCookiePolicy(cookieURL *url.URL, optionCookies, jarCookies []*http.Cookie) (*requestCookiePolicy, error) {
	options := cloneCookies(optionCookies)
	jar := cloneCookies(jarCookies)

	for index, cookie := range jar {
		if err := cookie.Valid(); err != nil {
			return nil, fmt.Errorf("jar Cookie %q at index %d is invalid: %w", cookie.Name, index, err)
		}
	}

	return &requestCookiePolicy{
		originURL:             cloneURL(cookieURL),
		options:               options,
		optionByName:          indexUniqueCookies(options),
		jarByName:             groupCookiesByName(jar),
		defaults:              make(map[string]*http.Cookie),
		allowProtocolDefaults: cookieURL != nil && isProtocolCookieDomain(cookieURL.Hostname()),
	}, nil
}

func (r *requestCookiePolicy) cookieScalar(names ...string) (string, bool) {
	for _, name := range names {
		if cookie := r.optionByName[name]; cookie != nil {
			return cookie.Value, true
		}
	}

	for _, name := range names {
		if cookies := r.jarByName[name]; len(cookies) > 0 {
			return cookies[0].Value, true
		}
	}

	if r.allowProtocolDefaults {
		for _, name := range names {
			if cookie := r.defaults[name]; cookie != nil {
				return cookie.Value, true
			}
		}
	}
	return "", false
}

func (r *requestCookiePolicy) setDefaultCookies(cookies map[string]string) error {
	for name, value := range cookies {
		if err := r.setDefaultCookie(name, value); err != nil {
			return err
		}
	}
	return nil
}

func (r *requestCookiePolicy) setDefaultCookie(name, value string) error {
	if r == nil || r.finalized || !r.allowProtocolDefaults || value == "" {
		return nil
	}

	cookie := &http.Cookie{Name: name, Value: value}
	if err := cookie.Valid(); err != nil {
		return fmt.Errorf("default Cookie %q is invalid", name)
	}

	r.defaults[name] = cookie
	return nil
}

func (r *requestCookiePolicy) setDefaultIfMissing(name, value string) error {
	if r == nil || r.defaults[name] != nil {
		return nil
	}

	return r.setDefaultCookie(name, value)
}

func (r *requestCookiePolicy) deleteDefault(name string) {
	if r == nil || r.finalized {
		return
	}

	delete(r.defaults, name)
}

func (r *requestCookiePolicy) finalize(frozenNames []string) {
	if r.finalized {
		return
	}

	r.frozen = make(map[string][]*http.Cookie, len(frozenNames))
	r.frozenNames = make([]string, 0, len(frozenNames))

	for _, name := range frozenNames {
		// Options stay explicit so a stripped value can fall back to the current Jar.
		if r.optionByName[name] != nil {
			continue
		}

		r.frozenNames = append(r.frozenNames, name)

		if cookies := r.jarByName[name]; len(cookies) > 0 {
			r.frozen[name] = cloneCookies(cookies)
		} else if r.allowProtocolDefaults && r.defaults[name] != nil {
			r.frozen[name] = cloneCookies([]*http.Cookie{r.defaults[name]})
		}
	}

	r.finalized = true
}

func groupCookiesByName(cookies []*http.Cookie) map[string][]*http.Cookie {
	grouped := make(map[string][]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}

		grouped[cookie.Name] = append(grouped[cookie.Name], cookie)
	}
	return grouped
}

func indexUniqueCookies(cookies []*http.Cookie) map[string]*http.Cookie {
	indexed := make(map[string]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		if cookie != nil {
			indexed[cookie.Name] = cookie
		}
	}
	return indexed
}

func mergeRequestCookies(headerCookies, jarCookies []*http.Cookie, policy *requestCookiePolicy, cookieURL *url.URL) []*http.Cookie {
	merged := cloneCookies(jarCookies)
	if policy == nil {
		return mergeCookieLayer(merged, headerCookies)
	}

	explicit := deduplicateCookieLayer(cloneCookies(headerCookies))

	if policy.matches(cookieURL) {
		for _, name := range policy.frozenNames {
			if containsCookieName(explicit, name) {
				continue
			}

			merged = replaceCookiesByName(merged, name, policy.frozen[name])
		}
	}

	if policy.allowProtocolDefaults && cookieURL != nil && isProtocolCookieDomain(cookieURL.Hostname()) {
		for _, name := range sortedCookieNames(policy.defaults) {
			if !containsCookieName(merged, name) && !containsCookieName(explicit, name) {
				merged = append(merged, cloneCookie(policy.defaults[name]))
			}
		}
	}

	return mergeCookieLayer(merged, explicit)
}

// mergeCookieLayer 合并两个 Cookie 层，override 按 Name 覆盖 base。
// 只对 override 层做同名去重；未被覆盖的 base 会原样保留，以支持 Jar 中不同 Path 的同名 Cookie。
func mergeCookieLayer(base, override []*http.Cookie) []*http.Cookie {
	if len(override) == 0 {
		if len(base) == 0 {
			return nil
		}

		result := make([]*http.Cookie, 0, len(base))
		for _, cookie := range base {
			if cookie != nil {
				result = append(result, cookie)
			}
		}
		return result
	}

	if len(base) == 0 {
		return deduplicateCookieLayer(override)
	}

	override = deduplicateCookieLayer(override)
	if len(override) == 0 {
		result := make([]*http.Cookie, 0, len(base))
		for _, cookie := range base {
			if cookie != nil {
				result = append(result, cookie)
			}
		}
		return result
	}

	if len(override) == 1 {
		targetName := override[0].Name

		result := make([]*http.Cookie, 0, len(base)+1)
		for _, cookie := range base {
			if cookie != nil && cookie.Name != targetName {
				result = append(result, cookie)
			}
		}
		return append(result, override[0])
	}

	overridden := make(map[string]struct{}, len(override))
	for _, cookie := range override {
		if cookie == nil {
			continue
		}

		overridden[cookie.Name] = struct{}{}
	}

	result := make([]*http.Cookie, 0, len(base)+len(override))
	for _, cookie := range base {
		if cookie == nil {
			continue
		}

		if _, ok := overridden[cookie.Name]; !ok {
			result = append(result, cookie)
		}
	}
	return append(result, override...)
}

// deduplicateCookieLayer 对单一 Cookie 层按 Name 去重，保持最后一次设置生效。
// 该方法不能直接用于完整 Jar Cookie 列表，否则会破坏不同 Path 的合法同名值。
func deduplicateCookieLayer(cookies []*http.Cookie) []*http.Cookie {
	switch len(cookies) {
	case 0:
		return nil
	case 1:
		if cookies[0] == nil {
			return nil
		}
		return []*http.Cookie{cookies[0]}
	}

	last := make(map[string]int, len(cookies))
	for index, cookie := range cookies {
		if cookie != nil {
			last[cookie.Name] = index
		}
	}

	result := make([]*http.Cookie, 0, len(last))
	for index, cookie := range cookies {
		if cookie != nil && last[cookie.Name] == index {
			result = append(result, cookie)
		}
	}
	return result
}

func sortedCookieNames(cookies map[string]*http.Cookie) []string {
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}

	slices.Sort(names)
	return names
}

// replaceCookiesByName 用冻结快照逐项替换同名 Cookie，保留其他 Cookie 的相对位置。
func replaceCookiesByName(cookies []*http.Cookie, name string, replacements []*http.Cookie) []*http.Cookie {
	result := make([]*http.Cookie, 0, len(cookies)+len(replacements))
	replacementIndex := 0

	for _, cookie := range cookies {
		if cookie.Name == name {
			if replacementIndex < len(replacements) {
				result = append(result, replacements[replacementIndex])
				replacementIndex++
			}
			continue
		}

		result = append(result, cookie)
	}

	return append(result, replacements[replacementIndex:]...)
}

// containsCookieName 按区分大小写的 Name 判断 Cookie 是否存在。
func containsCookieName(cookies []*http.Cookie, name string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return true
		}
	}
	return false
}

func (r *requestCookiePolicy) matches(cookieURL *url.URL) bool {
	if r == nil || r.originURL == nil || cookieURL == nil {
		return false
	}
	return strings.EqualFold(r.originURL.Scheme, cookieURL.Scheme) &&
		normalizedCookieHost(r.originURL) == normalizedCookieHost(cookieURL) &&
		cookieURLPort(r.originURL) == cookieURLPort(cookieURL) &&
		cookiePath(r.originURL) == cookiePath(cookieURL)
}

func cookiePath(cookieURL *url.URL) string {
	path := cookieURL.EscapedPath()
	if path == "" {
		return "/"
	}
	return path
}

func normalizedCookieHost(cookieURL *url.URL) string {
	return strings.ToLower(strings.TrimSuffix(cookieURL.Hostname(), "."))
}

func cookieURLPort(cookieURL *url.URL) string {
	if port := cookieURL.Port(); port != "" {
		return port
	}

	switch strings.ToLower(cookieURL.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func cookieURLForHost(source *url.URL, host string) *url.URL {
	if source == nil || host == "" {
		return source
	}

	cookieURL := cloneURL(source)
	cookieURL.Host = host
	return cookieURL
}

func cloneURL(source *url.URL) *url.URL {
	if source == nil {
		return nil
	}

	clone := *source
	return &clone
}

// isProtocolCookieDomain 判断目标主机是否允许注入协议预置 Cookie。
// 范围刻意限制为 music.163.com 及其子域；代理捕获域名、CDN 和对象存储域名不属于该信任边界。
func isProtocolCookieDomain(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "music.163.com" || strings.HasSuffix(host, ".music.163.com")
}

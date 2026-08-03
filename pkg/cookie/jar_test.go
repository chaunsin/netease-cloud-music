// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package cookie

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGo126QuotedCookies(t *testing.T) {
	jar, err := New(nil)
	require.NoError(t, err)

	u := mustCookieURL(t, "http://www.host.test/")
	jar.SetCookies(u, []*http.Cookie{
		{Name: "plain", Value: "quoted", Quoted: true},
		{Name: "spaces", Value: "quoted with spaces", Quoted: true},
		{Name: "unquoted", Value: "value"},
	})

	got := jar.Cookies(u)
	require.Len(t, got, 3)
	assert.Equal(t, `plain="quoted"`, got[0].String())
	assert.True(t, got[0].Quoted)
	assert.Equal(t, `spaces="quoted with spaces"`, got[1].String())
	assert.True(t, got[1].Quoted)
	assert.Equal(t, "unquoted=value", got[2].String())
	assert.False(t, got[2].Quoted)
}

func TestGo126SecureCookiesOnLoopback(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "localhost", url: "http://localhost:8910/", want: true},
		{name: "localhost suffix", url: "http://example.LOCALHOST:8910/", want: true},
		{name: "IPv4 loopback", url: "http://127.0.0.1:8910/", want: true},
		{name: "IPv6 loopback", url: "http://[::1]:8910/", want: true},
		{name: "localhost substring", url: "http://notlocalhost/", want: false},
		{name: "ordinary HTTP host", url: "http://www.host.test/", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jar, err := New(nil)
			require.NoError(t, err)

			u := mustCookieURL(t, tt.url)
			jar.SetCookies(u, []*http.Cookie{{Name: "secure", Value: "value", Secure: true}})

			got := jar.Cookies(u)
			assert.Equal(t, tt.want, len(got) == 1)
		})
	}
}

func TestGo126BasicDomainAndSecureBehavior(t *testing.T) {
	jar, err := New(nil)
	require.NoError(t, err)

	origin := mustCookieURL(t, "https://www.example.com/")
	jar.SetCookies(origin, []*http.Cookie{
		{Name: "host", Value: "1", Path: "/"},
		{Name: "domain", Value: "2", Domain: ".example.com", Path: "/"},
		{Name: "secure", Value: "3", Domain: ".example.com", Path: "/", Secure: true},
		{Name: "invalid", Value: "4", Domain: ".other.example", Path: "/"},
	})

	assert.Equal(t, []string{"host", "domain", "secure"}, cookieNames(jar.Cookies(origin)))
	assert.Equal(t, []string{"host", "domain"}, cookieNames(jar.Cookies(mustCookieURL(t, "http://www.example.com/"))))
	assert.Equal(t, []string{"domain", "secure"}, cookieNames(jar.Cookies(mustCookieURL(t, "https://sub.example.com/"))))
	assert.Nil(t, jar.Cookies(mustCookieURL(t, "https://other.example/")))
}

func TestGo126IPv6ZoneIsIsolatedFromDomainBucket(t *testing.T) {
	jar, err := New(nil)
	require.NoError(t, err)

	jar.SetCookies(
		mustCookieURL(t, "https://example.com/"),
		[]*http.Cookie{{Name: "secret", Value: "value"}},
	)

	got := jar.Cookies(mustCookieURL(t, "https://[::1%25.example.com]:80/"))
	assert.Empty(t, got)
}

func TestGo126IsIP(t *testing.T) {
	tests := map[string]bool{
		"127.0.0.1":            true,
		"1.2.3.4":              true,
		"2001:4860:0:2001::68": true,
		"::1%zone":             true,
		"example.com":          false,
		"1.1.1.300":            false,
		"www.foo.bar.net":      false,
		"123.foo.bar.net":      false,
	}

	for host, want := range tests {
		assert.Equalf(t, want, isIP(host), "host %q", host)
	}
}

func TestCookiePathOrdering(t *testing.T) {
	jar, err := New(nil)
	require.NoError(t, err)

	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	jar.setCookies(mustCookieURL(t, "https://example.com/a/b/item"), []*http.Cookie{
		{Name: "root", Value: "1", Path: "/"},
		{Name: "middle", Value: "2", Path: "/a"},
		{Name: "deep", Value: "3", Path: "/a/b"},
	}, now)

	got := jar.cookies(mustCookieURL(t, "https://example.com/a/b/item"), now)
	require.Len(t, got, 3)
	assert.Equal(t, "deep", got[0].Name)
	assert.Equal(t, "middle", got[1].Name)
	assert.Equal(t, "root", got[2].Name)
}

func TestGo126NoMatchingCookiesReturnNil(t *testing.T) {
	jar, err := New(nil)
	require.NoError(t, err)

	jar.SetCookies(
		mustCookieURL(t, "https://example.com/only"),
		[]*http.Cookie{{Name: "scoped", Value: "value", Path: "/only"}},
	)

	assert.Nil(t, jar.Cookies(mustCookieURL(t, "https://example.com/other")))
}

func cookieNames(cookies []*http.Cookie) []string {
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}

	return names
}

func mustCookieURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	u, err := url.Parse(rawURL)
	require.NoError(t, err)

	return u
}

// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chaunsin/netease-cloud-music/pkg/cookie"
	"github.com/chaunsin/netease-cloud-music/pkg/crypto"
	"github.com/chaunsin/netease-cloud-music/pkg/log"
)

type closeIdleTestTransport struct {
	closeCalls int
}

func (*closeIdleTestTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unexpected RoundTrip")
}

func (t *closeIdleTestTransport) CloseIdleConnections() {
	t.closeCalls++
}

type forwardingTestTransport struct {
	transport http.RoundTripper
}

func (t *forwardingTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return t.transport.RoundTrip(request)
}

func (t *forwardingTestTransport) CloseIdleConnections() {
	if closer, ok := t.transport.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

type trackedRequestBody struct {
	reader     io.Reader
	closeCalls int
	closeErr   error
}

func (b *trackedRequestBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *trackedRequestBody) Close() error {
	b.closeCalls++
	return b.closeErr
}

func TestCookieTransportMergesCookieLayers(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	requestURL := mustParseURL(t, "https://music.163.com/api/test")
	jar.SetCookies(requestURL, []*http.Cookie{
		{Name: "jar", Value: "jar-value"},
		{Name: "shared", Value: "jar-value"},
	})

	policy := mustNewRequestCookiePolicy(t,
		requestURL,
		[]*http.Cookie{
			{Name: "shared", Value: "option-value"},
			{Name: "option", Value: "option-value"},
		},
		jar.Cookies(requestURL),
	)
	require.NoError(t, policy.setDefaultCookie("jar", "default-value"))
	require.NoError(t, policy.setDefaultCookie("default", "default-value"))
	policy.finalize(nil)
	explicit := policy.options

	merged := mergeRequestCookies(explicit, jar.Cookies(requestURL), policy, requestURL)
	assert.Equal(t, map[string]string{
		"jar":     "jar-value",
		"shared":  "option-value",
		"option":  "option-value",
		"default": "default-value",
	}, cookieValues(merged))
}

func TestCookieTransportKeepsExplicitEmptyCookieOverride(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	requestURL := mustParseURL(t, "https://music.163.com/api/test")
	jar.SetCookies(requestURL, []*http.Cookie{{Name: "shared", Value: "jar-value"}})

	policy := mustNewRequestCookiePolicy(t,
		requestURL,
		[]*http.Cookie{{Name: "shared", Value: ""}},
		jar.Cookies(requestURL),
	)
	require.NoError(t, policy.setDefaultCookie("shared", "default-value"))
	policy.finalize(nil)
	explicit := policy.options

	merged := mergeRequestCookies(explicit, jar.Cookies(requestURL), policy, requestURL)
	assert.Equal(t, []string{""}, cookieValuesByName(merged, "shared"))
}

func TestCookieTransportKeepsStableLayerOrder(t *testing.T) {
	requestURL := mustParseURL(t, "https://music.163.com/api/test")
	jarCookies := []*http.Cookie{
		{Name: "jar-first", Value: "first"},
		{Name: "jar-overridden", Value: "jar"},
		{Name: "jar-last", Value: "last"},
	}
	policy := mustNewRequestCookiePolicy(t,
		requestURL,
		[]*http.Cookie{
			{Name: "option-z", Value: "z"},
			{Name: "jar-overridden", Value: "option"},
			{Name: "option-a", Value: "a"},
		},
		jarCookies,
	)
	require.NoError(t, policy.setDefaultCookie("default-z", "z"))
	require.NoError(t, policy.setDefaultCookie("default-a", "a"))

	policy.finalize(nil)
	explicit := policy.options
	merged := mergeRequestCookies(explicit, jarCookies, policy, requestURL)
	assert.Equal(t, []string{
		"jar-first", "jar-last",
		"default-a", "default-z",
		"option-z", "jar-overridden", "option-a",
	}, cookieNameOrder(merged))
}

func TestRequestCookiePolicyPreservesInterleavedFrozenJarOrder(t *testing.T) {
	requestURL := mustParseURL(t, "https://music.163.com/api/test")
	jarCookies := []*http.Cookie{
		{Name: "deviceId", Value: "specific", Path: "/api"},
		{Name: "other", Value: "middle", Path: "/"},
		{Name: "deviceId", Value: "general", Path: "/"},
	}
	policy := mustNewRequestCookiePolicy(t, requestURL, nil, jarCookies)

	policy.finalize([]string{"deviceId"})
	explicit := policy.options
	merged := mergeRequestCookies(explicit, jarCookies, policy, requestURL)

	assert.Equal(t, []string{"deviceId", "other", "deviceId"}, cookieNameOrder(merged))
	assert.Equal(t, []string{"specific", "general"}, cookieValuesByName(merged, "deviceId"))
}

func TestRequestCookiePolicyResolvesSourceBeforeAliasOrder(t *testing.T) {
	requestURL := mustParseURL(t, "https://music.163.com/api/test")
	policy := mustNewRequestCookiePolicy(t,
		requestURL,
		[]*http.Cookie{{Name: "__csrf_token", Value: "option"}},
		[]*http.Cookie{{Name: "__csrf", Value: "jar"}},
	)
	require.NoError(t, policy.setDefaultCookie("__csrf", "default"))

	value, found := policy.cookieScalar("__csrf", "__csrf_token")
	require.True(t, found)
	assert.Equal(t, "option", value)
}

func TestProtocolCookieDomainBoundary(t *testing.T) {
	tests := map[string]bool{
		"music.163.com":           true,
		"interface.music.163.com": true,
		"MUSIC.163.COM.":          true,
		"music.163.com.example":   false,
		"163.com":                 false,
		"127.0.0.1":               false,
	}

	for host, want := range tests {
		assert.Equal(t, want, isProtocolCookieDomain(host), host)
	}
}

func TestRequestCookiePolicyMatchesNormalizedURL(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		current string
		want    bool
	}{
		{
			name:    "default HTTPS port and trailing dot are equivalent",
			origin:  "https://MUSIC.163.COM./api/test",
			current: "https://music.163.com:443/api/test",
			want:    true,
		},
		{
			name:    "query does not identify the physical route",
			origin:  "https://music.163.com/api/test?attempt=1",
			current: "https://music.163.com/api/test?attempt=2",
			want:    true,
		},
		{
			name:    "empty path equals root path",
			origin:  "https://music.163.com",
			current: "https://music.163.com/",
			want:    true,
		},
		{
			name:    "non-default port is different",
			origin:  "https://music.163.com/api/test",
			current: "https://music.163.com:8443/api/test",
			want:    false,
		},
		{
			name:    "escaped path is compared on the wire form",
			origin:  "https://music.163.com/api/%2Ftest",
			current: "https://music.163.com/api//test",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := mustNewRequestCookiePolicy(t, mustParseURL(t, tt.origin), nil, nil)
			assert.Equal(t, tt.want, policy.matches(mustParseURL(t, tt.current)))
		})
	}
}

func TestCookieTransportPreservesSameNameJarCookiesWithoutOverride(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	origin := mustParseURL(t, "https://music.163.com")
	jar.SetCookies(origin, []*http.Cookie{{Name: "same", Value: "root", Path: "/"}})
	jar.SetCookies(mustParseURL(t, "https://music.163.com/api"), []*http.Cookie{{Name: "same", Value: "api", Path: "/api"}})

	requestURL := mustParseURL(t, "https://music.163.com/api/test")
	merged := mergeRequestCookies(nil, jar.Cookies(requestURL), nil, requestURL)
	assert.Equal(t, []string{"api", "root"}, cookieValuesByName(merged, "same"))

	merged = mergeRequestCookies([]*http.Cookie{
		{Name: "same", Value: "first-explicit"},
		{Name: "same", Value: "last-explicit"},
	}, jar.Cookies(requestURL), nil, requestURL)
	assert.Equal(t, []string{"last-explicit"}, cookieValuesByName(merged, "same"))
}

func TestRequestCookiePolicyUsesMostSpecificJarCookieForScalar(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	jar.SetCookies(mustParseURL(t, "https://music.163.com"), []*http.Cookie{{Name: "deviceId", Value: "root", Path: "/"}})
	jar.SetCookies(mustParseURL(t, "https://music.163.com/api"), []*http.Cookie{{Name: "deviceId", Value: "api", Path: "/api"}})

	requestURL := mustParseURL(t, "https://music.163.com/api/test")
	policy := mustNewRequestCookiePolicy(t, requestURL, nil, jar.Cookies(requestURL))

	deviceID, ok := policy.cookieScalar("deviceId")
	require.True(t, ok)
	assert.Equal(t, "api", deviceID)
	assert.Equal(t, []string{"api", "root"}, cookieValuesByName(jar.Cookies(requestURL), "deviceId"))
}

func TestRequestCookiePolicyUsesNormalizedOptionCookie(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	requestURL := mustParseURL(t, "https://music.163.com/api/test")
	policy := mustNewRequestCookiePolicy(t,
		requestURL,
		[]*http.Cookie{{Name: "deviceId", Value: "last"}},
		jar.Cookies(requestURL),
	)

	deviceID, ok := policy.cookieScalar("deviceId")
	require.True(t, ok)
	assert.Equal(t, "last", deviceID)
	assert.Equal(t, []string{"last"}, cookieValuesByName(policy.options, "deviceId"))
}

func TestRequestCookiePolicyRejectsInvalidDefaultCookie(t *testing.T) {
	policy := mustNewRequestCookiePolicy(t, mustParseURL(t, "https://music.163.com/api/test"), nil, nil)
	secret := "anonymous-secret;private"

	err := policy.setDefaultCookie("MUSIC_A", secret)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `default Cookie "MUSIC_A" is invalid`)
	assert.NotContains(t, err.Error(), secret)
	assert.NotContains(t, policy.defaults, "MUSIC_A")
}

func TestCookieTransportScopesDefaultsAndFrozenCookies(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	origin := mustParseURL(t, "https://music.163.com/api/original")
	jar.SetCookies(origin, []*http.Cookie{{Name: "deviceId", Value: "jar-device"}})

	policy := mustNewRequestCookiePolicy(t, origin, nil, jar.Cookies(origin))
	require.NoError(t, policy.setDefaultCookie("default", "default-value"))
	policy.finalize([]string{"deviceId"})
	initial := policy.options

	jar.SetCookies(origin, []*http.Cookie{{Name: "deviceId", Value: "new-device"}, {Name: "session", Value: "new-session"}})
	assert.Equal(t, map[string]string{
		"deviceId": "jar-device",
		"session":  "new-session",
		"default":  "default-value",
	}, cookieValues(mergeRequestCookies(initial, jar.Cookies(origin), policy, origin)))

	redirect := mustParseURL(t, "https://sub.music.163.com/api/redirect")
	jar.SetCookies(redirect, []*http.Cookie{{Name: "deviceId", Value: "redirect-device"}})
	assert.Equal(t, map[string]string{
		"deviceId": "redirect-device",
		"default":  "default-value",
	}, cookieValues(mergeRequestCookies(initial, jar.Cookies(redirect), policy, redirect)))

	outsideProtocolDomain := mustParseURL(t, "https://example.com/api/redirect")
	jar.SetCookies(outsideProtocolDomain, []*http.Cookie{{Name: "jar", Value: "jar-value"}})
	assert.Equal(t, map[string]string{"jar": "jar-value"}, cookieValues(mergeRequestCookies(nil, jar.Cookies(outsideProtocolDomain), policy, outsideProtocolDomain)))
}

func TestCookieTransportUsesEffectiveHostForJarScope(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	target := mustParseURL(t, "https://127.0.0.1/api/test")
	logical := mustParseURL(t, "https://music.163.com/api/test")

	jar.SetCookies(target, []*http.Cookie{{Name: "target", Value: "target-value"}})
	jar.SetCookies(logical, []*http.Cookie{{Name: "logical", Value: "host-value"}})

	transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, "music.163.com", request.Host)
		assert.Empty(t, requestCookieValue(request, "target"))
		assert.Equal(t, "host-value", requestCookieValue(request, "logical"))
		return responseWithCookies(request, http.StatusOK, http.Header{"Set-Cookie": {"scope=logical; Path=/"}}), nil
	}))
	request, err := http.NewRequest(http.MethodGet, target.String(), http.NoBody)
	require.NoError(t, err)

	request.Host = "music.163.com"

	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.Empty(t, cookieValue(jar.Cookies(target), "scope"))
	assert.Equal(t, "logical", cookieValue(jar.Cookies(logical), "scope"))
}

func TestRequestSnapshotUsesRewrittenXEAPIURLForJarScope(t *testing.T) {
	client := newCookieTransportTestClient(t)
	original := mustParseURL(t, "https://music.163.com/eapi/test")
	client.SetCookies(original, []*http.Cookie{
		{Name: "deviceId", Value: "original-device", Path: "/eapi"},
	})

	rewritten := mustParseURL(t, "https://music.163.com/xeapi/test")
	client.SetCookies(rewritten, []*http.Cookie{
		{Name: "deviceId", Value: "rewritten-device", Path: "/xeapi"},
	})

	policy := mustNewRequestCookiePolicy(t, rewritten, nil, client.cookieJar.Cookies(rewritten))
	require.NoError(t, policy.setDefaultCookies(client.defHeader.XEAPI.Cookie))

	val, _ := policy.cookieScalar("deviceId")
	assert.Equal(t, "rewritten-device", val)
}

func TestCookieTransportDoesNotSendDefaultsOutsideProtocolCookieDomain(t *testing.T) {
	client := newCookieTransportTestClient(t)
	client.GetAnonymous().Set("anonymous-token;private")
	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		assert.Empty(t, requestCookieValue(request, "deviceId"))
		assert.Empty(t, requestCookieValue(request, "appver"))
		assert.Empty(t, requestCookieValue(request, "MUSIC_A"))
		return responseWithCookies(request, http.StatusServiceUnavailable, nil), nil
	}))

	var reply eapiTestResponse

	_, err := client.Request(
		context.Background(),
		"https://example.test/eapi/test",
		map[string]string{"deviceId": "caller-device"},
		&reply,
		NewOptions().SetEAPI(),
	)
	require.ErrorContains(t, err, "http status code: 503")
}

func TestClientRequestRejectsInvalidAnonymousCookieBeforeTransport(t *testing.T) {
	client := newCookieTransportTestClient(t)
	secret := "anonymous-secret;private"
	client.GetAnonymous().Set(secret)

	transportCalls := 0

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		transportCalls++
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))

	var reply map[string]any

	_, err := client.Request(
		context.Background(),
		"https://music.163.com/weapi/test",
		map[string]string{"id": "1"},
		&reply,
		NewOptions(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `default Cookie "MUSIC_A" is invalid`)
	assert.NotContains(t, err.Error(), secret)
	assert.Zero(t, transportCalls)
}

func TestClientRequestRejectsInvalidJarCookieBeforeTransport(t *testing.T) {
	client := newCookieTransportTestClient(t)
	target := mustParseURL(t, "https://music.163.com/weapi/test")
	secret := "device-secret;private"
	client.SetCookies(target, []*http.Cookie{{Name: "deviceId", Value: secret}})

	transportCalls := 0

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		transportCalls++
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))

	var reply map[string]any

	_, err := client.Request(
		context.Background(),
		target.String(),
		map[string]string{"id": "1"},
		&reply,
		NewOptions(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `jar Cookie "deviceId" at index 0 is invalid`)
	assert.NotContains(t, err.Error(), secret)
	assert.Zero(t, transportCalls)
}

func TestRequestFinalizesCookiesForEveryCryptoMode(t *testing.T) {
	linuxResponse, err := crypto.LinuxApiEncrypt(map[string]int{"code": http.StatusOK})
	require.NoError(t, err)

	tests := []struct {
		name         string
		newClient    func(*testing.T) *Client
		requestURL   string
		request      any
		options      *Options
		responseBody func(*testing.T) []byte
	}{
		{
			name:       "api",
			newClient:  newCookieTransportTestClient,
			requestURL: "https://music.163.com/api/test",
			request:    map[string]string{"id": "1"},
			options:    NewOptions().SetAPI(),
			responseBody: func(*testing.T) []byte {
				return []byte(`{"code":200}`)
			},
		},
		{
			name:       "weapi",
			newClient:  newCookieTransportTestClient,
			requestURL: "https://music.163.com/weapi/test",
			request:    map[string]string{"id": "1"},
			options:    NewOptions().SetWEAPI(),
			responseBody: func(*testing.T) []byte {
				return []byte(`{"code":200}`)
			},
		},
		{
			name:       "eapi",
			newClient:  newCookieTransportTestClient,
			requestURL: "https://music.163.com/eapi/test",
			request:    map[string]any{"id": "1", "e_r": false},
			options:    NewOptions().SetEAPI(),
			responseBody: func(*testing.T) []byte {
				return []byte(`{"code":200}`)
			},
		},
		{
			name:       "linux",
			newClient:  newCookieTransportTestClient,
			requestURL: "https://music.163.com/api/linux/test",
			request:    map[string]string{"id": "1"},
			options:    NewOptions().SetLinuxAPI(),
			responseBody: func(*testing.T) []byte {
				return []byte(linuxResponse["eparams"])
			},
		},
		{
			name:       "xeapi",
			newClient:  newOfflineXeapiClient,
			requestURL: "https://music.163.com/api/test",
			request:    map[string]string{"id": "1"},
			options:    NewOptions().SetXEAPI(),
			responseBody: func(t *testing.T) []byte {
				t.Helper()

				return encryptLegacyEapiResponse(t, []byte(`{"code":200}`))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.newClient(t)
			cookieURL := mustParseURL(t, tt.requestURL)
			client.SetCookies(cookieURL, []*http.Cookie{{Name: "jar", Value: "jar-value", Path: "/"}})
			tt.options.SetCookies(&http.Cookie{Name: "option", Value: "option-value"})

			client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				assert.Equal(t, []string{"jar-value"}, cookieValuesByName(request.Cookies(), "jar"))
				assert.Equal(t, []string{"option-value"}, cookieValuesByName(request.Cookies(), "option"))
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(tt.responseBody(t))),
					Request:    request,
				}, nil
			}))

			var reply map[string]any

			_, err := client.Request(context.Background(), tt.requestURL, tt.request, &reply, tt.options)
			require.NoError(t, err)
			assert.InDelta(t, http.StatusOK, reply["code"], 0)
		})
	}
}

func TestClientRequestUsesCustomHostForCookieScope(t *testing.T) {
	client := newCookieTransportTestClient(t)
	target := mustParseURL(t, "https://example.test/weapi/test")
	logical := mustParseURL(t, "https://music.163.com/weapi/test")

	client.SetCookies(target, []*http.Cookie{{Name: "target", Value: "target-value"}})
	client.SetCookies(logical, []*http.Cookie{{Name: "logical", Value: "logical-value"}})
	client.GetAnonymous().Set("anonymous-token")

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, "example.test", request.URL.Hostname())
		assert.Equal(t, "music.163.com", request.Host)
		assert.Empty(t, requestCookieValue(request, "target"))
		assert.Equal(t, "logical-value", requestCookieValue(request, "logical"))
		assert.Equal(t, "anonymous-token", requestCookieValue(request, "MUSIC_A"))
		assert.NotEmpty(t, requestCookieValue(request, "appver"))
		return responseWithCookies(request, http.StatusOK, http.Header{"Set-Cookie": {"scope=logical; Path=/"}}), nil
	}))

	var reply map[string]any

	options := NewOptions()
	options.Headers["host"] = []string{"music.163.com"}

	_, err := client.Request(
		context.Background(),
		target.String(),
		map[string]string{"id": "1"},
		&reply,
		options,
	)
	require.NoError(t, err)
	assert.Empty(t, cookieValue(client.GetCookies(target), "scope"))
	assert.Equal(t, "logical", cookieValue(client.GetCookies(logical), "scope"))
}

func TestClientRequestCustomHostCannotUseURLCookieScope(t *testing.T) {
	client := newCookieTransportTestClient(t)
	urlScope := mustParseURL(t, "https://music.163.com/weapi/test")
	hostScope := mustParseURL(t, "https://example.test/weapi/test")

	client.SetCookies(urlScope, []*http.Cookie{{Name: "url", Value: "url-value"}})
	client.SetCookies(hostScope, []*http.Cookie{{Name: "host", Value: "host-value"}})
	client.GetAnonymous().Set("anonymous-token")

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, "music.163.com", request.URL.Hostname())
		assert.Equal(t, "example.test", request.Host)
		assert.Empty(t, requestCookieValue(request, "url"))
		assert.Equal(t, "host-value", requestCookieValue(request, "host"))
		assert.Empty(t, requestCookieValue(request, "MUSIC_A"))
		assert.Empty(t, requestCookieValue(request, "appver"))
		assert.Empty(t, requestCookieValue(request, "deviceId"))
		return responseWithCookies(request, http.StatusOK, http.Header{"Set-Cookie": {"scope=host; Path=/"}}), nil
	}))

	var reply map[string]any

	options := NewOptions()
	options.Headers["HOST"] = []string{"example.test"}

	_, err := client.Request(
		context.Background(),
		urlScope.String(),
		map[string]string{"id": "1"},
		&reply,
		options,
	)
	require.NoError(t, err)
	assert.Empty(t, cookieValue(client.GetCookies(urlScope), "scope"))
	assert.Equal(t, "host", cookieValue(client.GetCookies(hostScope), "scope"))
}

func TestMergeRequestHeadersCanonicalizesExplicitEmptyOverride(t *testing.T) {
	merged := mergeRequestHeaders(
		http.Header{"Host": {"music.163.com"}},
		http.Header{"host": nil},
	)

	values, ok := merged["Host"]
	require.True(t, ok)
	assert.Empty(t, values)
	assert.NotContains(t, merged, "host")
}

func TestRequestCookiePolicyUsesExplicitAndJarIdentityOutsideProtocolCookieDomain(t *testing.T) {
	client := newCookieTransportTestClient(t)
	target := mustParseURL(t, "https://example.test/eapi/test")
	client.SetCookies(target, []*http.Cookie{{Name: "deviceId", Value: "jar-device"}})

	policy := mustNewRequestCookiePolicy(t,
		target,
		[]*http.Cookie{{Name: "appver", Value: "explicit-appver"}},
		client.cookieJar.Cookies(target),
	)
	require.NoError(t, policy.setDefaultCookies(client.defHeader.EAPI.Cookie))

	deviceId, _ := policy.cookieScalar("deviceId")
	appver, _ := policy.cookieScalar("appver")

	assert.False(t, policy.allowProtocolDefaults)
	assert.Equal(t, "jar-device", deviceId)
	assert.Equal(t, "explicit-appver", appver)

	_, found := policy.cookieScalar("os")
	assert.False(t, found)
}

func TestCookieTransportRoundTripUpdatesJarForRedirects(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/first":
			assert.Empty(t, request.Header.Get("Cookie"))
			return responseWithCookies(request, http.StatusFound, http.Header{
				"Location":   {"/second"},
				"Set-Cookie": {"step=one; Path=/"},
			}), nil
		case "/second":
			assert.Equal(t, "one", requestCookieValue(request, "step"))
			return responseWithCookies(request, http.StatusOK, http.Header{"Set-Cookie": {"final=value; Path=/"}}), nil
		default:
			return nil, errors.New("unexpected path")
		}
	}))
	client := &http.Client{Transport: transport}

	response, err := client.Get("https://music.163.com/first")
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	requestURL := mustParseURL(t, "https://music.163.com/second")
	assert.Equal(t, map[string]string{"step": "one", "final": "value"}, cookieValues(jar.Cookies(requestURL)))
}

func TestCookieTransportRequeriesJarForRedirectPath(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	origin := mustParseURL(t, "https://music.163.com/first/start")
	target := mustParseURL(t, "https://music.163.com/second/end")

	jar.SetCookies(origin, []*http.Cookie{{Name: "scoped", Value: "first", Path: "/first"}})
	jar.SetCookies(target, []*http.Cookie{{Name: "scoped", Value: "second", Path: "/second"}})

	policy := mustNewRequestCookiePolicy(t, origin, nil, jar.Cookies(origin))
	policy.finalize(nil)
	initial := policy.options

	transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case origin.Path:
			assert.Equal(t, []string{"first"}, cookieValuesByName(request.Cookies(), "scoped"))

			response := responseWithCookies(request, http.StatusFound, http.Header{"Location": {target.String()}})
			// Custom RoundTrippers may leave Request unset; the wrapper reconstructs the redirect chain.
			response.Request = nil
			return response, nil
		case target.Path:
			assert.Equal(t, []string{"second"}, cookieValuesByName(request.Cookies(), "scoped"))
			return responseWithCookies(request, http.StatusOK, nil), nil
		default:
			return nil, errors.New("unexpected redirect path")
		}
	}))
	client := &http.Client{Transport: transport}

	request, err := http.NewRequestWithContext(
		withRequestCookiePolicy(context.Background(), policy),
		http.MethodGet,
		origin.String(),
		http.NoBody,
	)
	require.NoError(t, err)

	for _, cookie := range initial {
		request.AddCookie(cookie)
	}

	response, err := client.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
}

func TestCookieTransportWritesCookiesForHTTPResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		value  string
	}{
		{name: "success", status: http.StatusOK, value: "success"},
		{name: "redirect", status: http.StatusFound, value: "redirect"},
		{name: "client error", status: http.StatusBadRequest, value: "client-error"},
		{name: "server error", status: http.StatusInternalServerError, value: "server-error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jar := newCookieTransportTestJar(t)
			requestURL := mustParseURL(t, "https://music.163.com/api/test")
			transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				return responseWithCookies(request, tt.status, http.Header{"Set-Cookie": {"status=" + tt.value + "; Path=/"}}), nil
			}))
			request, err := http.NewRequest(http.MethodGet, requestURL.String(), http.NoBody)
			require.NoError(t, err)

			response, err := transport.RoundTrip(request)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			assert.Equal(t, tt.value, cookieValue(jar.Cookies(requestURL), "status"))
		})
	}
}

func TestCookieTransportWritesCookiesBeforeRedirectPolicy(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	origin := mustParseURL(t, "https://music.163.com/first")
	transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return responseWithCookies(request, http.StatusFound, http.Header{
			"Location":   {"/next"},
			"Set-Cookie": {"redirect=written; Path=/"},
		}), nil
	}))
	stop := errors.New("stop redirect")
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return stop
		},
	}

	response, err := client.Get(origin.String())
	require.ErrorIs(t, err, stop)
	require.NotNil(t, response)
	require.NoError(t, response.Body.Close())

	assert.Equal(t, "written", cookieValue(jar.Cookies(origin), "redirect"))
}

func TestCookieTransportDoesNotWriteCookiesWithoutSuccessfulResponse(t *testing.T) {
	tests := []struct {
		name      string
		roundTrip testRoundTripperFunc
	}{
		{
			name: "response and error",
			roundTrip: func(request *http.Request) (*http.Response, error) {
				return responseWithCookies(request, http.StatusInternalServerError, http.Header{"Set-Cookie": {"late=value; Path=/"}}), assert.AnError
			},
		},
		{
			name: "nil response",
			roundTrip: func(*http.Request) (*http.Response, error) {
				//nolint:nilnil // A malformed response is the behavior under test.
				return nil, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jar := newCookieTransportTestJar(t)
			requestURL := mustParseURL(t, "https://music.163.com/api/test")
			transport := newCookieTransport(jar, tt.roundTrip)
			request, err := http.NewRequest(http.MethodGet, requestURL.String(), http.NoBody)
			require.NoError(t, err)

			response, err := transport.RoundTrip(request)
			require.Error(t, err)

			if response != nil {
				require.NoError(t, response.Body.Close())
			}

			assert.Empty(t, jar.Cookies(requestURL))
		})
	}
}

func TestClientRequestKeepsCookiesFromHTTPStatusError(t *testing.T) {
	client := newCookieTransportTestClient(t)
	requestURL := "https://music.163.com/weapi/test"

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return responseWithCookies(request, http.StatusInternalServerError, http.Header{"Set-Cookie": {"server=updated; Path=/"}}), nil
	}))

	var reply map[string]any

	_, err := client.Request(context.Background(), requestURL, map[string]string{"id": "1"}, &reply, NewOptions())
	require.Error(t, err)
	assert.Equal(t, "updated", cookieValue(client.GetCookies(mustParseURL(t, requestURL)), "server"))
}

func TestCookieTransportFollowsStandardExplicitCookieRedirectBehavior(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	origin := mustParseURL(t, "https://music.163.com/first")
	target := mustParseURL(t, "https://example.test/second")
	jar.SetCookies(target, []*http.Cookie{{Name: "target", Value: "jar-value"}})
	policy := mustNewRequestCookiePolicy(t,
		origin,
		[]*http.Cookie{{Name: "explicit", Value: "value"}},
		jar.Cookies(origin),
	)
	policy.finalize(nil)
	initial := policy.options

	transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/first":
			assert.Equal(t, "value", requestCookieValue(request, "explicit"))
			return responseWithCookies(request, http.StatusFound, http.Header{"Location": {target.String()}}), nil
		case "/second":
			assert.Empty(t, requestCookieValue(request, "explicit"))
			assert.Equal(t, "jar-value", requestCookieValue(request, "target"))
			return responseWithCookies(request, http.StatusOK, nil), nil
		default:
			return nil, errors.New("unexpected path")
		}
	}))
	client := &http.Client{Transport: transport}

	request, err := http.NewRequestWithContext(withRequestCookiePolicy(context.Background(), policy), http.MethodGet, origin.String(), http.NoBody)
	require.NoError(t, err)

	for _, cookie := range initial {
		request.AddCookie(cookie)
	}

	response, err := client.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
}

func TestCookieTransportKeepsSensitiveHeaderStrippingAfterReturningToOrigin(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	origin := mustParseURL(t, "https://music.163.com/api/original")
	outside := mustParseURL(t, "https://example.test/outside")

	jar.SetCookies(origin, []*http.Cookie{
		{Name: "session", Value: "old-session", Path: "/"},
		{Name: "deviceId", Value: "frozen-device", Path: "/"},
	})
	jar.SetCookies(outside, []*http.Cookie{{Name: "outside", Value: "outside-jar", Path: "/"}})

	policy := mustNewRequestCookiePolicy(t,
		origin,
		[]*http.Cookie{{Name: "option", Value: "option-value"}},
		jar.Cookies(origin),
	)
	require.NoError(t, policy.setDefaultCookie("default", "default-value"))
	policy.finalize([]string{"deviceId"})
	initial := policy.options
	originRequests := 0

	transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Hostname() {
		case origin.Hostname():
			originRequests++
			if originRequests == 1 {
				assert.Equal(t, "old-session", requestCookieValue(request, "session"))
				assert.Equal(t, "frozen-device", requestCookieValue(request, "deviceId"))
				assert.Equal(t, "default-value", requestCookieValue(request, "default"))
				assert.Equal(t, "option-value", requestCookieValue(request, "option"))
				return responseWithCookies(request, http.StatusFound, http.Header{"Location": {outside.String()}}), nil
			}

			assert.Equal(t, "new-session", requestCookieValue(request, "session"))
			assert.Equal(t, "frozen-device", requestCookieValue(request, "deviceId"))
			assert.Equal(t, "default-value", requestCookieValue(request, "default"))
			assert.Empty(t, requestCookieValue(request, "option"))
			assert.Empty(t, requestCookieValue(request, "outside"))
			return responseWithCookies(request, http.StatusOK, nil), nil
		case outside.Hostname():
			assert.Equal(t, "outside-jar", requestCookieValue(request, "outside"))
			assert.Empty(t, requestCookieValue(request, "session"))
			assert.Empty(t, requestCookieValue(request, "deviceId"))
			assert.Empty(t, requestCookieValue(request, "default"))
			assert.Empty(t, requestCookieValue(request, "option"))

			jar.SetCookies(origin, []*http.Cookie{
				{Name: "session", Value: "new-session", Path: "/"},
				{Name: "deviceId", Value: "new-device", Path: "/"},
			})
			return responseWithCookies(request, http.StatusFound, http.Header{"Location": {origin.String()}}), nil
		default:
			return nil, errors.New("unexpected redirect host")
		}
	}))
	client := &http.Client{Transport: transport}

	request, err := http.NewRequestWithContext(
		withRequestCookiePolicy(context.Background(), policy),
		http.MethodGet,
		origin.String(),
		http.NoBody,
	)
	require.NoError(t, err)

	for _, cookie := range initial {
		request.AddCookie(cookie)
	}

	response, err := client.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.Equal(t, 2, originRequests)
}

func TestCookieTransportPreservesCheckRedirectCookieChanges(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	origin := mustParseURL(t, "https://music.163.com/first/start")
	target := mustParseURL(t, "https://music.163.com/second/end")

	jar.SetCookies(origin, []*http.Cookie{
		{Name: "origin-only", Value: "origin", Path: "/first"},
		{Name: "same-value", Value: "origin", Path: "/first"},
	})
	jar.SetCookies(target, []*http.Cookie{
		{Name: "target-only", Value: "target", Path: "/second"},
		{Name: "remove", Value: "target-jar", Path: "/second"},
		{Name: "replace", Value: "target-jar", Path: "/second"},
		{Name: "same-value", Value: "target-jar", Path: "/second"},
	})

	policy := mustNewRequestCookiePolicy(t,
		origin,
		[]*http.Cookie{
			{Name: "keep", Value: "option"},
			{Name: "remove", Value: "option"},
			{Name: "replace", Value: "option"},
		},
		jar.Cookies(origin),
	)
	policy.finalize(nil)
	initial := policy.options

	transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case origin.Path:
			assert.Equal(t, "origin", requestCookieValue(request, "origin-only"))
			assert.Equal(t, "origin", requestCookieValue(request, "same-value"))
			assert.Equal(t, "option", requestCookieValue(request, "remove"))
			return responseWithCookies(request, http.StatusFound, http.Header{"Location": {target.String()}}), nil
		case target.Path:
			assert.Equal(t, "target", requestCookieValue(request, "target-only"))
			assert.Equal(t, "option", requestCookieValue(request, "keep"))
			assert.Equal(t, "redirect", requestCookieValue(request, "replace"))
			assert.Equal(t, "redirect", requestCookieValue(request, "added"))
			assert.Equal(t, "origin", requestCookieValue(request, "same-value"))
			assert.Equal(t, "target-jar", requestCookieValue(request, "remove"))
			assert.Empty(t, requestCookieValue(request, "origin-only"))
			return responseWithCookies(request, http.StatusOK, nil), nil
		default:
			return nil, errors.New("unexpected redirect path")
		}
	}))
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			request.Header.Del("Cookie")
			request.AddCookie(&http.Cookie{Name: "keep", Value: "option"})
			request.AddCookie(&http.Cookie{Name: "replace", Value: "redirect"})
			request.AddCookie(&http.Cookie{Name: "added", Value: "redirect"})
			request.AddCookie(&http.Cookie{Name: "same-value", Value: "origin"})
			return nil
		},
	}

	request, err := http.NewRequestWithContext(
		withRequestCookiePolicy(context.Background(), policy),
		http.MethodGet,
		origin.String(),
		http.NoBody,
	)
	require.NoError(t, err)

	for _, cookie := range initial {
		request.AddCookie(cookie)
	}

	response, err := client.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
}

func TestCookieTransportFallsBackToJarWhenCheckRedirectDeletesOptionProtocolCookie(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	origin := mustParseURL(t, "https://music.163.com/api/test?step=origin")
	target := mustParseURL(t, "https://music.163.com/api/test?step=target")
	policy := mustNewRequestCookiePolicy(t,
		origin,
		[]*http.Cookie{{Name: "deviceId", Value: "option"}},
		jar.Cookies(origin),
	)
	policy.finalize([]string{"deviceId"})

	transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.RawQuery {
		case origin.RawQuery:
			assert.Equal(t, "option", requestCookieValue(request, "deviceId"))
			return responseWithCookies(request, http.StatusFound, http.Header{
				"Location":   {target.String()},
				"Set-Cookie": {"deviceId=target-jar; Path=/api"},
			}), nil
		case target.RawQuery:
			assert.Equal(t, "target-jar", requestCookieValue(request, "deviceId"))
			return responseWithCookies(request, http.StatusOK, nil), nil
		default:
			return nil, errors.New("unexpected redirect query")
		}
	}))
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			request.Header.Del("Cookie")
			return nil
		},
	}

	request, err := http.NewRequestWithContext(
		withRequestCookiePolicy(context.Background(), policy),
		http.MethodGet,
		origin.String(),
		http.NoBody,
	)
	require.NoError(t, err)

	for _, cookie := range policy.options {
		request.AddCookie(cookie)
	}

	response, err := client.Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.Equal(t, "target-jar", cookieValue(jar.Cookies(target), "deviceId"))
}

func TestCookieTransportWritesRetryCookieBeforeNextAttempt(t *testing.T) {
	client := newCookieTransportTestClient(t)
	client.cli.SetRetryCount(1)
	client.cli.SetRetryWaitTime(time.Millisecond)
	client.cli.SetRetryMaxWaitTime(time.Millisecond)
	client.cli.AddRetryCondition(func(response *resty.Response, err error) bool {
		return err == nil && response.StatusCode() == http.StatusServiceUnavailable
	})

	attempt := 0

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		attempt++
		if attempt == 1 {
			assert.Empty(t, requestCookieValue(request, "retry"))
			return responseWithCookies(request, http.StatusServiceUnavailable, http.Header{"Set-Cookie": {"retry=updated; Path=/"}}), nil
		}

		assert.Equal(t, "updated", requestCookieValue(request, "retry"))
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))

	var reply map[string]any

	_, err := client.Request(context.Background(), "https://music.163.com/weapi/test", map[string]string{"id": "1"}, &reply, NewOptions())
	require.NoError(t, err)
	assert.Equal(t, 2, attempt)
	assert.Equal(t, "updated", cookieValue(client.GetCookies(mustParseURL(t, "https://music.163.com/weapi/test")), "retry"))
}

func TestCookieTransportDoesNotResurrectDeletedCookieOnRetry(t *testing.T) {
	client := newCookieTransportTestClient(t)
	requestURL := mustParseURL(t, "https://music.163.com/weapi/test")
	client.SetCookies(requestURL, []*http.Cookie{{Name: "retry", Value: "old", Path: "/"}})
	client.cli.SetRetryCount(1)
	client.cli.SetRetryWaitTime(time.Millisecond)
	client.cli.SetRetryMaxWaitTime(time.Millisecond)
	client.cli.AddRetryCondition(func(response *resty.Response, err error) bool {
		return err == nil && response.StatusCode() == http.StatusServiceUnavailable
	})

	attempt := 0

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		attempt++
		if attempt == 1 {
			assert.Equal(t, "old", requestCookieValue(request, "retry"))
			return responseWithCookies(request, http.StatusServiceUnavailable, http.Header{
				"Set-Cookie": {"retry=; Max-Age=0; Path=/"},
			}), nil
		}

		assert.Empty(t, requestCookieValue(request, "retry"))
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))

	var reply map[string]any

	_, err := client.Request(context.Background(), requestURL.String(), map[string]string{"id": "1"}, &reply, NewOptions())
	require.NoError(t, err)
	assert.Equal(t, 2, attempt)
	assert.Empty(t, cookieValue(client.GetCookies(requestURL), "retry"))
}

func TestCookieTransportFreezesAbsentProtocolCookieAcrossRetry(t *testing.T) {
	client := newCookieTransportTestClient(t)
	requestURL := mustParseURL(t, "https://music.163.com/weapi/test")

	client.cli.SetRetryCount(1)
	client.cli.SetRetryWaitTime(time.Millisecond)
	client.cli.SetRetryMaxWaitTime(time.Millisecond)
	client.cli.AddRetryCondition(func(response *resty.Response, err error) bool {
		return err == nil && response.StatusCode() == http.StatusServiceUnavailable
	})

	attempt := 0

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		attempt++

		assert.Empty(t, requestCookieValue(request, "MUSIC_U"))

		if attempt == 1 {
			return responseWithCookies(request, http.StatusServiceUnavailable, http.Header{
				"Set-Cookie": {"MUSIC_U=next-request; Path=/"},
			}), nil
		}
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))

	var reply map[string]any

	_, err := client.Request(context.Background(), requestURL.String(), map[string]string{"id": "1"}, &reply, NewOptions())
	require.NoError(t, err)
	assert.Equal(t, 2, attempt)
	assert.Equal(t, "next-request", cookieValue(client.GetCookies(requestURL), "MUSIC_U"))
}

func TestCookieTransportDoesNotMutateRedirectRequest(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))

	request, err := http.NewRequest(http.MethodGet, "https://music.163.com/test", http.NoBody)
	require.NoError(t, err)
	request.Header.Set("Cookie", "explicit=value")
	originalHeader := request.Header.Clone()

	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.Equal(t, originalHeader, request.Header)
}

func TestCookieTransportCloseWaitsForCookieWriteback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jar := newCookieTransportTestJar(t)
		requestURL := mustParseURL(t, "https://music.163.com/weapi/test")
		entered := make(chan struct{})
		release := make(chan struct{})
		transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
			close(entered)
			<-release
			return responseWithCookies(request, http.StatusOK, http.Header{"Set-Cookie": {"late=value; Path=/"}}), nil
		}))

		request, err := http.NewRequest(http.MethodGet, requestURL.String(), http.NoBody)
		require.NoError(t, err)

		requestDone := make(chan error, 1)

		go func() {
			response, roundTripErr := transport.RoundTrip(request)
			if response != nil {
				roundTripErr = errors.Join(roundTripErr, response.Body.Close())
			}

			requestDone <- roundTripErr
		}()

		<-entered

		closeDone := make(chan struct{})

		go func() {
			transport.Close()
			close(closeDone)
		}()

		synctest.Wait()

		select {
		case <-closeDone:
			t.Fatal("Close returned before the in-flight request completed")
		default:
		}

		close(release)
		synctest.Wait()

		require.NoError(t, <-requestDone)
		<-closeDone
		assert.Equal(t, "value", cookieValue(jar.Cookies(requestURL), "late"))

		response, err := transport.RoundTrip(request)
		if response != nil {
			require.NoError(t, response.Body.Close())
		}

		require.ErrorIs(t, err, ErrClientClosed)
	})
}

func TestClientCloseHonorsCanceledContextWhileShutdownContinues(t *testing.T) {
	client := newCookieTransportTestClient(t)
	entered := make(chan struct{})
	release := make(chan struct{})

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return responseWithCookies(request, http.StatusOK, http.Header{"Set-Cookie": {"late=value; Path=/"}}), nil
	}))

	requestDone := make(chan error, 1)

	go func() {
		var reply map[string]any

		_, err := client.Request(context.Background(), "https://music.163.com/weapi/test", map[string]string{"id": "1"}, &reply, NewOptions())
		requestDone <- err
	}()

	<-entered

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	closeDone := make(chan error, 1)
	go func() { closeDone <- client.Close(ctx) }()

	select {
	case err := <-closeDone:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("Close did not return after its context was canceled")
	}

	var reply map[string]any

	_, err := client.Request(context.Background(), "https://music.163.com/weapi/test", map[string]string{"id": "1"}, &reply, NewOptions())
	require.ErrorIs(t, err, ErrClientClosed)

	close(release)
	require.NoError(t, <-requestDone)
	require.NoError(t, client.Close(context.Background()))
	assert.Equal(t, "value", cookieValue(client.GetCookies(mustParseURL(t, "https://music.163.com/weapi/test")), "late"))
}

func TestClientGetClientJarIsNilAndSetTransportKeepsCookieWrapper(t *testing.T) {
	client := newCookieTransportTestClient(t)
	require.Nil(t, client.GetClient().Jar)

	called := false

	client.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		called = true
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))

	request, err := http.NewRequest(http.MethodGet, "https://music.163.com/test", http.NoBody)
	require.NoError(t, err)
	response, err := client.GetClient().Do(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.True(t, called)
}

func TestCookieTransportSetTransportDuringRoundTrip(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	requestURL := mustParseURL(t, "https://music.163.com/test")
	entered := make(chan struct{})
	release := make(chan struct{})
	transport := newCookieTransport(jar, testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))

	request, err := http.NewRequest(http.MethodGet, requestURL.String(), http.NoBody)
	require.NoError(t, err)

	firstDone := make(chan error, 1)

	go func() {
		response, roundTripErr := transport.RoundTrip(request)
		if response != nil {
			roundTripErr = errors.Join(roundTripErr, response.Body.Close())
		}

		firstDone <- roundTripErr
	}()

	<-entered

	newCalls := 0

	transport.SetTransport(testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		newCalls++
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))
	close(release)
	require.NoError(t, <-firstDone)

	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.Equal(t, 1, newCalls)
}

func TestCookieTransportRejectsTransportLoops(t *testing.T) {
	tests := []struct {
		name  string
		build func(http.CookieJar) *cookieTransport
	}{
		{
			name: "direct",
			build: func(jar http.CookieJar) *cookieTransport {
				transport := newCookieTransport(jar, http.DefaultTransport)
				transport.SetTransport(transport)
				return transport
			},
		},
		{
			name: "through wrapper",
			build: func(jar http.CookieJar) *cookieTransport {
				transport := newCookieTransport(jar, http.DefaultTransport)
				transport.SetTransport(&forwardingTestTransport{transport: transport})
				return transport
			},
		},
		{
			name: "between Cookie transports",
			build: func(jar http.CookieJar) *cookieTransport {
				first := newCookieTransport(jar, http.DefaultTransport)
				second := newCookieTransport(jar, first)
				first.SetTransport(second)
				return first
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := tt.build(newCookieTransportTestJar(t))
			body := &trackedRequestBody{reader: bytes.NewBufferString("body")}
			request, err := http.NewRequest(http.MethodPost, "https://music.163.com/test", body)
			require.NoError(t, err)

			response, err := transport.RoundTrip(request)
			if response != nil {
				require.NoError(t, response.Body.Close())
			}

			assert.Nil(t, response)
			require.ErrorIs(t, err, errCookieTransportLoop)
			assert.Equal(t, 1, body.closeCalls)

			// A loop must not recurse through the optional close capability either.
			transport.CloseIdleConnections()
		})
	}
}

func TestCookieTransportClosesBodyWhenClosed(t *testing.T) {
	transport := newCookieTransport(newCookieTransportTestJar(t), http.DefaultTransport)
	transport.Close()

	closeErr := errors.New("close request body")
	body := &trackedRequestBody{reader: bytes.NewBufferString("body"), closeErr: closeErr}
	request, err := http.NewRequest(http.MethodPost, "https://music.163.com/test", body)
	require.NoError(t, err)

	response, err := transport.RoundTrip(request)
	if response != nil {
		require.NoError(t, response.Body.Close())
	}

	assert.Nil(t, response)
	require.ErrorIs(t, err, ErrClientClosed)
	require.ErrorIs(t, err, closeErr)
	assert.Equal(t, 1, body.closeCalls)
}

func TestCookieTransportResponseRequestCanBeReused(t *testing.T) {
	calls := 0
	transport := newCookieTransport(newCookieTransportTestJar(t), testRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return responseWithCookies(request, http.StatusOK, nil), nil
	}))
	request, err := http.NewRequest(http.MethodGet, "https://music.163.com/test", http.NoBody)
	require.NoError(t, err)

	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.Nil(t, response.Request.Context().Value(transport))

	replayed, err := transport.RoundTrip(response.Request)
	require.NoError(t, err)
	require.NoError(t, replayed.Body.Close())
	assert.Equal(t, 2, calls)
}

func TestClientGetClientForwardsCloseIdleConnections(t *testing.T) {
	client := newCookieTransportTestClient(t)
	transport := new(closeIdleTestTransport)
	client.SetTransport(transport)

	client.GetClient().CloseIdleConnections()

	assert.Equal(t, 1, transport.closeCalls)
}

func TestClientCloseClosesIdleConnections(t *testing.T) {
	client := newCookieTransportTestClient(t)
	transport := new(closeIdleTestTransport)
	client.SetTransport(transport)

	require.NoError(t, client.Close(context.Background()))
	assert.Equal(t, 1, transport.closeCalls)
}

func newCookieTransportTestJar(t *testing.T) http.CookieJar {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return jar
}

func newCookieTransportTestClient(t *testing.T) *Client {
	t.Helper()
	home := t.TempDir()
	logger := log.New(&log.Config{Level: "error"})
	client, err := NewClient(&Config{
		Timeout: time.Second,
		HomeDir: home,
		Cookie: cookie.Config{
			Filepath: filepath.Join(home, "cookie.json"),
			Interval: 0,
		},
	}, logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(context.Background()))
		require.NoError(t, logger.Close())
	})
	return client
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)
	return parsed
}

func mustNewRequestCookiePolicy(t *testing.T, cookieURL *url.URL, optionCookies, jarCookies []*http.Cookie) *requestCookiePolicy {
	t.Helper()

	policy, err := newRequestCookiePolicy(cookieURL, optionCookies, jarCookies)
	require.NoError(t, err)
	return policy
}

func cookieValues(cookies []*http.Cookie) map[string]string {
	values := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		values[cookie.Name] = cookie.Value
	}
	return values
}

func cookieValuesByName(cookies []*http.Cookie, name string) []string {
	values := make([]string, 0)

	for _, cookie := range cookies {
		if cookie.Name == name {
			values = append(values, cookie.Value)
		}
	}
	return values
}

func cookieNameOrder(cookies []*http.Cookie) []string {
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}
	return names
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func requestCookieValue(request *http.Request, name string) string {
	for _, cookie := range request.Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func responseWithCookies(request *http.Request, status int, header http.Header) *http.Response {
	if header == nil {
		header = make(http.Header)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"code":200}`))),
		Request:    request,
	}
}

func TestDeduplicateCookieLayerFastPaths(t *testing.T) {
	assert.Nil(t, deduplicateCookieLayer(nil))
	assert.Nil(t, deduplicateCookieLayer([]*http.Cookie{}))
	assert.Nil(t, deduplicateCookieLayer([]*http.Cookie{nil}))

	single := []*http.Cookie{{Name: "a", Value: "1"}}
	assert.Equal(t, single, deduplicateCookieLayer(single))

	multiple := []*http.Cookie{
		{Name: "a", Value: "1"},
		nil,
		{Name: "a", Value: "2"},
		{Name: "b", Value: "3"},
	}
	deduped := deduplicateCookieLayer(multiple)
	assert.Equal(t, []*http.Cookie{
		{Name: "a", Value: "2"},
		{Name: "b", Value: "3"},
	}, deduped)
}

func TestMergeCookieLayerFastPaths(t *testing.T) {
	// base 和 override 均为空
	assert.Nil(t, mergeCookieLayer(nil, nil))
	assert.Nil(t, mergeCookieLayer([]*http.Cookie{}, []*http.Cookie{}))

	base := []*http.Cookie{{Name: "b1", Value: "v1"}, {Name: "b2", Value: "v2"}}
	// override 为空，直接返回 base（去 nil）
	assert.Equal(t, base, mergeCookieLayer(base, nil))

	// base 为空，直接返回去重后的 override
	override := []*http.Cookie{{Name: "o1", Value: "ov1"}}
	assert.Equal(t, override, mergeCookieLayer(nil, override))

	// override 长度为 1 的 fast-path
	overrideSingle := []*http.Cookie{{Name: "b1", Value: "new_v1"}}
	merged := mergeCookieLayer(base, overrideSingle)
	assert.Equal(t, []*http.Cookie{
		{Name: "b2", Value: "v2"},
		{Name: "b1", Value: "new_v1"},
	}, merged)
}

func TestConcurrentCloseIdleConnections(t *testing.T) {
	jar := newCookieTransportTestJar(t)
	transport := new(closeIdleTestTransport)
	ct := newCookieTransport(jar, transport)

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			ct.CloseIdleConnections()
		})
	}

	wg.Wait()

	assert.GreaterOrEqual(t, transport.closeCalls, 1)
}

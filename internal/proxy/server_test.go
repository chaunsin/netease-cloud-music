// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/elazarl/goproxy"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	ncmcrypto "github.com/chaunsin/netease-cloud-music/pkg/crypto"
)

func TestHTTPProxyCapturesAndRedactsWithoutChangingTraffic(t *testing.T) {
	t.Parallel()

	requestBody := []byte(`{"name":"song","access_token":"request-secret"}`)
	responseBody := []byte(`{"code":200,"token":"response-secret"}`)
	received := make(chan []byte, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read origin request body: %v", err)
			return
		}

		received <- body

		if got := req.Header.Get("Authorization"); got != "Bearer auth-secret" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer auth-secret")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "MUSIC_U=cookie-secret")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(responseBody)
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, output, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	client := newProxyClient(t, proxyURL, nil, false)
	req, err := http.NewRequest(http.MethodPost, origin.URL+"/api/test?csrf_token=query-secret", bytes.NewReader(requestBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer auth-secret")
	req.Header.Set("Cookie", "MUSIC_U=cookie-secret")

	resp, err := client.Do(req)
	require.NoError(t, err)
	gotResponse, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, responseBody, gotResponse)
	require.Equal(t, requestBody, <-received)

	requireOutputContains(t, output, "RESPONSE status=201")
	logOutput := output.String()
	require.Contains(t, logOutput, "REQUEST protocol=api")
	require.Contains(t, logOutput, "RESPONSE status=201")
	require.Contains(t, logOutput, redactedValue)

	for _, secret := range []string{"request-secret", "response-secret", "query-secret", "auth-secret", "cookie-secret"} {
		require.NotContains(t, logOutput, secret)
	}
}

func TestHTTPProxyDoesNotCaptureMismatchedAbsoluteTarget(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "served")
	}))
	t.Cleanup(origin.Close)
	originURL, err := url.Parse(origin.URL)
	require.NoError(t, err)

	proxyURL, _, output, upstreamTransport := newTestProxy(t, []string{"music.163.com"}, 1<<20)
	upstreamTransport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, originURL.Host)
	}

	proxyConn, err := net.Dial("tcp", proxyURL.Host)
	require.NoError(t, err)
	_, err = io.WriteString(proxyConn, "GET http://other.example/not-target HTTP/1.1\r\nHost: music.163.com\r\nConnection: close\r\n\r\n")
	require.NoError(t, err)
	response, err := http.ReadResponse(bufio.NewReader(proxyConn), &http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.NoError(t, proxyConn.Close())

	client := newProxyClient(t, proxyURL, nil, false)
	response, err = client.Get("http://music.163.com/capture-marker")
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	requireOutputContains(t, output, "/capture-marker")
	require.NotContains(t, output.String(), "/not-target")
}

func TestCaptureOutputFailureDoesNotChangeForwardedTraffic(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(writer, "forwarded")
	}))
	t.Cleanup(origin.Close)

	dir := filepath.Join(t.TempDir(), "proxy")
	ca, _, err := loadOrCreateCA(filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key"), false)
	require.NoError(t, err)
	matcher, err := newHostMatcher([]string{"127.0.0.1"})
	require.NoError(t, err)

	diagnostics := &lockedBuffer{}
	recorder := newRecorderWithXeapiSessions(errorCaptureWriter{err: errors.New("capture unavailable")}, diagnostics, 1<<20, false, nil)
	t.Cleanup(recorder.Close)

	cfg := Config{MaxBodyBytes: 1 << 20, Out: io.Discard, ErrOut: diagnostics}
	proxyServer, upstreamTransport := newProxyServer(&cfg, matcher, ca, recorder, nil)
	t.Cleanup(upstreamTransport.CloseIdleConnections)

	server := httptest.NewServer(proxyServer)
	t.Cleanup(server.Close)
	proxyURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	client := newProxyClient(t, proxyURL, nil, false)
	response, err := client.Get(origin.URL + "/api/output-error")
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusCreated, response.StatusCode)
	require.Equal(t, "forwarded", string(body))
	require.True(t, flushRecorder(recorder, time.Second))
	require.Contains(t, diagnostics.String(), "CAPTURE_OUTPUT_ERROR")
}

func TestHTTPProxyLearnsXeapiSessionBeforeForwardingResponse(t *testing.T) {
	const (
		sessionID  = "runtime-session"
		sessionKey = "0123456789abcdef"
	)

	responseBody := []byte("opaque-upstream-response")
	receivedBody := make(chan []byte, 1)
	receivedHeader := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/xeapi/bootstrap":
			w.Header().Set("X-Encr-Ssid", sessionID)
			w.Header().Set("X-Encr-Sskey", sessionKey)
			w.Header().Set("X-Bootstrap", "preserved")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "not-an-encrypted-response")
		case "/xeapi/song/detail":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read request: %v", err)
				return
			}

			receivedBody <- body

			receivedHeader <- req.Header.Get("X-Preserve")

			w.Header().Set("X-Response-Preserve", "yes")
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write(responseBody)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(origin.Close)

	proxy := newXeapiTestProxy(t)
	defer proxy.close()

	client := newProxyClient(t, proxy.url, nil, false)

	bootstrap, err := client.Get(origin.URL + "/xeapi/bootstrap")
	require.NoError(t, err)

	learnedKey, source, ok := proxy.sessions.lookup(sessionID)
	require.True(t, ok)
	require.Equal(t, []byte(sessionKey), learnedKey)
	require.Equal(t, xeapiSessionSourceResponseHeader, source)

	bootstrapBody, err := io.ReadAll(bootstrap.Body)
	require.NoError(t, err)
	require.NoError(t, bootstrap.Body.Close())
	require.Equal(t, http.StatusServiceUnavailable, bootstrap.StatusCode)
	require.Equal(t, "preserved", bootstrap.Header.Get("X-Bootstrap"))
	require.Equal(t, sessionID, bootstrap.Header.Get("X-Encr-Ssid"))
	require.Equal(t, sessionKey, bootstrap.Header.Get("X-Encr-Sskey"))
	require.Equal(t, []byte("not-an-encrypted-response"), bootstrapBody)

	params := encryptXEAPIRequestForProxy(t, &ncmcrypto.XeapiEncryptRequest{
		URI:         "/api/song/detail?id=1",
		ContentType: "application/json",
		Body:        []byte(`{"token":"request-secret","id":1}`),
	}, ncmcrypto.XeapiSession{ID: sessionID, Key: sessionKey})
	forwardedBody := []byte(params.Encode())
	request, err := http.NewRequest(http.MethodPost, origin.URL+"/xeapi/song/detail", bytes.NewReader(forwardedBody))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Preserve", "yes")

	response, err := client.Do(request)
	require.NoError(t, err)
	gotResponseBody, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusTeapot, response.StatusCode)
	require.Equal(t, "yes", response.Header.Get("X-Response-Preserve"))
	require.Equal(t, responseBody, gotResponseBody)
	require.Equal(t, forwardedBody, <-receivedBody)
	require.Equal(t, "yes", <-receivedHeader)

	requireOutputContains(t, proxy.output, "REQUEST protocol=xeapi decode=decrypted")
	requireOutputContains(t, proxy.output, "api-path: /api/song/detail")
	require.NotContains(t, proxy.output.String(), sessionKey)
	require.NotContains(t, proxy.output.String(), "request-secret")
	require.NotContains(t, proxy.diagnostics.String(), sessionKey)
}

func TestXeapiSessionLearningDoesNotDependOnRecorderQueue(t *testing.T) {
	const (
		sessionID  = "runtime-session"
		sessionKey = "0123456789abcdef"
	)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Encr-Ssid", sessionID)
		w.Header().Set("X-Encr-Sskey", sessionKey)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "broken")
	}))
	t.Cleanup(origin.Close)

	proxy := newXeapiTestProxy(t)
	proxy.recorder.CloseWithTimeout(time.Second)

	defer proxy.close()

	client := newProxyClient(t, proxy.url, nil, false)
	response, err := client.Get(origin.URL + "/xeapi/bootstrap")
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusBadGateway, response.StatusCode)

	key, source, ok := proxy.sessions.lookup(sessionID)
	require.True(t, ok)
	require.Equal(t, []byte(sessionKey), key)
	require.Equal(t, xeapiSessionSourceResponseHeader, source)
}

func TestHTTPSMITMRequiresAndUsesGeneratedCA(t *testing.T) {
	t.Parallel()

	originProtocol := make(chan string, 1)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		originProtocol <- req.Proto

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":200}`)
	}))
	t.Cleanup(origin.Close)

	proxyURL, ca, output, upstreamTransport := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	originRoots := x509.NewCertPool()
	originRoots.AddCert(origin.Certificate())
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: originRoots, MinVersion: tls.VersionTLS12}

	untrusted := newProxyClient(t, proxyURL, nil, false)

	untrustedResponse, err := untrusted.Get(origin.URL + "/api/test") //nolint:bodyclose // A response is optional on the expected TLS failure and closed below when present.
	if untrustedResponse != nil {
		closeTestResource(t, untrustedResponse.Body)
	}

	require.Error(t, err)

	proxyRoots := x509.NewCertPool()
	proxyRoots.AddCert(ca.Leaf)
	trusted := newProxyClient(t, proxyURL, proxyRoots, false)
	resp, err := trusted.Get(origin.URL + "/api/test")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.JSONEq(t, `{"code":200}`, string(body))
	require.Equal(t, "HTTP/1.1", resp.Proto)
	require.Equal(t, "HTTP/1.1", <-originProtocol)
	requireOutputContains(t, output, "REQUEST protocol=api")
	requireOutputContains(t, output, "RESPONSE status=200")
}

func TestHTTPSMITMNegotiatesHTTP2(t *testing.T) {
	type observedRequest struct {
		path     string
		protocol string
		body     []byte
	}

	observed := make(chan observedRequest, 2)
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read HTTP/2 origin request: %v", err)
			return
		}

		observed <- observedRequest{path: req.URL.Path, protocol: req.Proto, body: body}

		switch req.URL.Path {
		case "/api/h2/body":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"result":"body-response"}`)
		case "/api/h2/empty":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, "empty-response")
		default:
			http.NotFound(w, req)
		}
	}))
	origin.EnableHTTP2 = true
	require.NoError(t, http2.ConfigureServer(origin.Config, &http2.Server{}))
	origin.StartTLS()
	t.Cleanup(origin.Close)

	proxyURL, ca, output, upstreamTransport := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	originRoots := x509.NewCertPool()
	originRoots.AddCert(origin.Certificate())
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: originRoots, MinVersion: tls.VersionTLS12}
	require.True(t, upstreamTransport.ForceAttemptHTTP2)

	proxyRoots := x509.NewCertPool()
	proxyRoots.AddCert(ca.Leaf)
	client := newHTTP2ProxyClient(t, proxyURL, proxyRoots)

	requestBody := []byte(`{"mode":"h2"}`)
	request, err := http.NewRequest(http.MethodPost, origin.URL+"/api/h2/body", bytes.NewReader(requestBody))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusCreated, response.StatusCode)
	require.Equal(t, "HTTP/2.0", response.Proto)
	require.JSONEq(t, `{"result":"body-response"}`, string(body))

	emptyResponse, err := client.Get(origin.URL + "/api/h2/empty")
	require.NoError(t, err)
	emptyBody, err := io.ReadAll(emptyResponse.Body)
	require.NoError(t, err)
	require.NoError(t, emptyResponse.Body.Close())
	require.Equal(t, "HTTP/2.0", emptyResponse.Proto)
	require.Equal(t, "empty-response", string(emptyBody))

	requestObservation := <-observed
	require.Equal(t, "/api/h2/body", requestObservation.path)
	require.Equal(t, "HTTP/2.0", requestObservation.protocol)
	require.Equal(t, requestBody, requestObservation.body)

	emptyObservation := <-observed
	require.Equal(t, "/api/h2/empty", emptyObservation.path)
	require.Equal(t, "HTTP/2.0", emptyObservation.protocol)
	require.Empty(t, emptyObservation.body)

	requireOutputContains(t, output, "POST "+origin.URL+"/api/h2/body")
	requireOutputContains(t, output, "GET "+origin.URL+"/api/h2/empty")
	requireOutputContains(t, output, `"mode": "h2"`)
	requireOutputContains(t, output, `"result": "body-response"`)
	requireOutputContains(t, output, "RESPONSE status=201")
	require.Eventually(t, func() bool {
		block, _, found := captureBlockForPath(output.String(), "REQUEST", "/api/h2/empty")
		return found && strings.Contains(block, "content-length=0 captured=0") && strings.Contains(block, "<empty>")
	}, 2*time.Second, 10*time.Millisecond)
}

func TestHTTPSMITMHTTP2UnknownLengthRequestIsForwardedWithoutPreRead(t *testing.T) {
	firstChunk := bytes.Repeat([]byte("h2-request-stream-"), 32)
	lastChunk := []byte("h2-tail")
	firstReceived := make(chan []byte, 1)
	completeRequest := make(chan []byte, 1)
	protocol := make(chan string, 1)

	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		protocol <- req.Proto

		prefix := make([]byte, len(firstChunk))
		if _, err := io.ReadFull(req.Body, prefix); err != nil {
			firstReceived <- nil
			return
		}

		firstReceived <- prefix

		rest, err := io.ReadAll(req.Body)
		if err != nil {
			completeRequest <- nil
			return
		}

		complete := append(append([]byte(nil), prefix...), rest...)
		completeRequest <- complete

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":200}`)
	}))
	origin.EnableHTTP2 = true
	require.NoError(t, http2.ConfigureServer(origin.Config, &http2.Server{}))
	origin.StartTLS()
	t.Cleanup(origin.Close)

	proxyURL, ca, output, upstreamTransport := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	originRoots := x509.NewCertPool()
	originRoots.AddCert(origin.Certificate())
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: originRoots, MinVersion: tls.VersionTLS12}

	proxyRoots := x509.NewCertPool()
	proxyRoots.AddCert(ca.Leaf)
	client := newHTTP2ProxyClient(t, proxyURL, proxyRoots)
	reader, writer := io.Pipe()

	t.Cleanup(func() { _ = writer.Close() })

	request, err := http.NewRequest(http.MethodPost, origin.URL+"/api/h2/stream", reader)
	require.NoError(t, err)

	request.ContentLength = -1
	request.Header.Set("Content-Type", "application/json")

	result := make(chan struct {
		response *http.Response
		err      error
	}, 1)
	go func() {
		response, requestErr := client.Do(request) //nolint:bodyclose // The receiver owns and closes the response body.
		result <- struct {
			response *http.Response
			err      error
		}{response: response, err: requestErr}
	}()

	_, err = writer.Write(firstChunk)
	require.NoError(t, err)

	select {
	case got := <-firstReceived:
		require.Equal(t, firstChunk, got)
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP/2 origin did not receive the first request chunk before EOF")
	}

	_, err = writer.Write(lastChunk)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	response := <-result
	require.NoError(t, response.err)
	require.Equal(t, "HTTP/2.0", response.response.Proto)
	_, err = io.Copy(io.Discard, response.response.Body)
	require.NoError(t, err)
	require.NoError(t, response.response.Body.Close())
	require.Equal(t, "HTTP/2.0", <-protocol)
	require.Equal(t, append(firstChunk, lastChunk...), <-completeRequest)
	requireOutputContains(t, output, "unknown-length request body omitted")
}

func TestHTTPSMITMHTTP2ClientFallsBackToHTTP1Upstream(t *testing.T) {
	originProtocol := make(chan string, 1)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		originProtocol <- req.Proto

		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "fallback-response")
	}))
	t.Cleanup(origin.Close)

	proxyURL, ca, output, upstreamTransport := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	originRoots := x509.NewCertPool()
	originRoots.AddCert(origin.Certificate())
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: originRoots, MinVersion: tls.VersionTLS12}

	proxyRoots := x509.NewCertPool()
	proxyRoots.AddCert(ca.Leaf)
	client := newHTTP2ProxyClient(t, proxyURL, proxyRoots)

	response, err := client.Get(origin.URL + "/api/h2-fallback")
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, "HTTP/2.0", response.Proto)
	require.Equal(t, "HTTP/1.1", <-originProtocol)
	require.Equal(t, "fallback-response", string(body))
	requireOutputContains(t, output, "GET "+origin.URL+"/api/h2-fallback")
}

func TestHTTPSMITMHTTP1ClientNegotiatesHTTP2Upstream(t *testing.T) {
	originProtocol := make(chan string, 1)
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		originProtocol <- req.Proto

		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "h2-upstream-response")
	}))
	origin.EnableHTTP2 = true
	require.NoError(t, http2.ConfigureServer(origin.Config, &http2.Server{}))
	origin.StartTLS()
	t.Cleanup(origin.Close)

	proxyURL, ca, output, upstreamTransport := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	originRoots := x509.NewCertPool()
	originRoots.AddCert(origin.Certificate())
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: originRoots, MinVersion: tls.VersionTLS12}

	proxyRoots := x509.NewCertPool()
	proxyRoots.AddCert(ca.Leaf)
	client := newHTTP1ProxyClient(t, proxyURL, proxyRoots)

	response, err := client.Get(origin.URL + "/api/h1-to-h2")
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, "HTTP/1.1", response.Proto)
	require.Equal(t, "HTTP/2.0", <-originProtocol)
	require.Equal(t, "h2-upstream-response", string(body))
	requireOutputContains(t, output, "GET "+origin.URL+"/api/h1-to-h2")
}

func TestHTTPSMITMHTTP2ConcurrentStreamsKeepCaptureIsolated(t *testing.T) {
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})

	type observedRequest struct {
		path     string
		protocol string
		body     []byte
	}

	observed := make(chan observedRequest, 2)

	var releaseOnce sync.Once

	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })

	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if req.URL.Path == "/api/h2/warmup" {
			_, _ = io.WriteString(w, `{"stream":"warmup"}`)
			return
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read concurrent HTTP/2 request body: %v", err)
			return
		}

		observed <- observedRequest{path: req.URL.Path, protocol: req.Proto, body: body}

		arrived <- struct{}{}

		<-release

		switch req.URL.Path {
		case "/api/h2/one":
			_, _ = io.WriteString(w, `{"stream":"response-one"}`)
		case "/api/h2/two":
			_, _ = io.WriteString(w, `{"stream":"response-two"}`)
		default:
			http.NotFound(w, req)
		}
	}))
	origin.EnableHTTP2 = true
	require.NoError(t, http2.ConfigureServer(origin.Config, &http2.Server{}))
	origin.StartTLS()
	t.Cleanup(origin.Close)

	proxyURL, ca, output, upstreamTransport := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	originRoots := x509.NewCertPool()
	originRoots.AddCert(origin.Certificate())
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: originRoots, MinVersion: tls.VersionTLS12}

	proxyRoots := x509.NewCertPool()
	proxyRoots.AddCert(ca.Leaf)
	client := newHTTP2ProxyClient(t, proxyURL, proxyRoots)
	clientTransport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	clientTransport.MaxConnsPerHost = 1

	warmup, err := client.Get(origin.URL + "/api/h2/warmup")
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, warmup.Body)
	require.NoError(t, err)
	require.NoError(t, warmup.Body.Close())
	require.Equal(t, "HTTP/2.0", warmup.Proto)

	type streamResult struct {
		name     string
		protocol string
		body     string
		err      error
	}

	results := make(chan streamResult, 2)
	start := make(chan struct{})

	for _, name := range []string{"one", "two"} {
		go func(name string) {
			<-start

			marker := fmt.Appendf(nil, `{"marker":%q}`, name)

			request, requestErr := http.NewRequest(http.MethodPost, origin.URL+"/api/h2/"+name, bytes.NewReader(marker))
			if requestErr == nil {
				request.Header.Set("Content-Type", "application/json")
			}

			response, requestErr := client.Do(request)
			if requestErr != nil {
				results <- streamResult{name: name, err: requestErr}
				return
			}

			body, readErr := io.ReadAll(response.Body)

			closeErr := response.Body.Close()
			results <- streamResult{
				name:     name,
				protocol: response.Proto,
				body:     string(body),
				err:      errors.Join(readErr, closeErr),
			}
		}(name)
	}

	close(start)

	for range 2 {
		select {
		case <-arrived:
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent HTTP/2 streams did not reach the origin together")
		}
	}

	releaseOnce.Do(func() { close(release) })

	for range 2 {
		result := <-results
		require.NoError(t, result.err)
		require.Equal(t, "HTTP/2.0", result.protocol)
		require.JSONEq(t, fmt.Sprintf(`{"stream":%q}`, "response-"+result.name), result.body)
	}

	for range 2 {
		request := <-observed
		require.Equal(t, "HTTP/2.0", request.protocol)
		require.Equal(t, fmt.Sprintf(`{"marker":%q}`, strings.TrimPrefix(request.path, "/api/h2/")), string(request.body))
	}

	for _, name := range []string{"one", "two"} {
		path := "/api/h2/" + name

		require.Eventually(t, func() bool {
			_, requestSession, requestFound := captureBlockForPath(output.String(), "REQUEST", path)
			_, responseSession, responseFound := captureBlockForPath(output.String(), "RESPONSE", path)

			return requestFound && responseFound && requestSession == responseSession
		}, 2*time.Second, 10*time.Millisecond)

		requestBlock, _, requestFound := captureBlockForPath(output.String(), "REQUEST", path)
		responseBlock, _, responseFound := captureBlockForPath(output.String(), "RESPONSE", path)

		require.True(t, requestFound)
		require.True(t, responseFound)
		require.Contains(t, requestBlock, fmt.Sprintf(`"marker": %q`, name))

		otherName := "one"
		if name == "one" {
			otherName = "two"
		}

		require.NotContains(t, requestBlock, fmt.Sprintf(`"marker": %q`, otherName))
		require.Contains(t, responseBlock, fmt.Sprintf(`"stream": "response-%s"`, name))
	}

	_, oneSession, oneFound := captureBlockForPath(output.String(), "REQUEST", "/api/h2/one")
	_, twoSession, twoFound := captureBlockForPath(output.String(), "REQUEST", "/api/h2/two")

	require.True(t, oneFound)
	require.True(t, twoFound)
	require.NotEqual(t, oneSession, twoSession)
}

func TestMITMDebugDiagnostics(t *testing.T) {
	const connectTarget = "music.163.com:443"

	t.Run("matching SNI and certificate", func(t *testing.T) {
		proxyURL, ca, _, diagnostics, _ := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, true)
		roots := x509.NewCertPool()
		roots.AddCert(ca.Leaf)
		client := newMITMTestTLSClient(t, proxyURL, connectTarget, "music.163.com", roots)
		require.NoError(t, client.Handshake())

		requireOutputContains(t, diagnostics, "[TLS_DIAGNOSTIC] phase=client_hello")
		text := diagnostics.String()
		require.Contains(t, text, `[TLS_DIAGNOSTIC] phase=connect`)
		require.Contains(t, text, `connect_target="music.163.com:443"`)
		require.Contains(t, text, `sni="music.163.com" sni_relation=match`)
		require.Contains(t, text, `dns_sans=["music.163.com"]`)
		require.Contains(t, text, `cert_matches_connect=true cert_matches_sni=true`)
	})

	t.Run("SNI and CONNECT mismatch", func(t *testing.T) {
		proxyURL, ca, _, diagnostics, _ := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, true)
		roots := x509.NewCertPool()
		roots.AddCert(ca.Leaf)
		client := newMITMTestTLSClient(t, proxyURL, connectTarget, "interface3.music.163.com", roots)
		require.Error(t, client.Handshake())

		requireOutputContains(t, diagnostics, "cert_matches_connect=true cert_matches_sni=false")
		text := diagnostics.String()
		require.Contains(t, text, `sni="interface3.music.163.com" sni_relation=mismatch`)
	})

	t.Run("disabled", func(t *testing.T) {
		proxyURL, ca, _, diagnostics, _ := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, false)
		roots := x509.NewCertPool()
		roots.AddCert(ca.Leaf)
		client := newMITMTestTLSClient(t, proxyURL, connectTarget, "music.163.com", roots)
		require.NoError(t, client.Handshake())
		require.NotContains(t, diagnostics.String(), "TLS_DIAGNOSTIC")
	})
}

func TestIPConnectUsesTargetSNIForMITM(t *testing.T) {
	const targetHost = "interface.music.163.com"

	received := make(chan struct {
		host string
		path string
		body string
	}, 1)
	proxyURL, ca, output, diagnostics, upstreamTransport := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, true)
	origin := newTLSServerForHost(t, ca, targetHost, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read IP-targeted request body: %v", err)
			http.Error(writer, "read request", http.StatusInternalServerError)

			return
		}

		received <- struct {
			host string
			path string
			body string
		}{host: request.Host, path: request.URL.Path, body: string(body)}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"code":200}`)
	}))

	upstreamRoots := x509.NewCertPool()
	upstreamRoots.AddCert(ca.Leaf)
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: upstreamRoots, MinVersion: tls.VersionTLS12}

	proxyRoots := x509.NewCertPool()
	proxyRoots.AddCert(ca.Leaf)
	client := newMITMTestTLSClient(t, proxyURL, origin.Listener.Addr().String(), targetHost, proxyRoots)
	requestBody := `{"params":"reproduction"}`
	_, err := fmt.Fprintf(
		client,
		"POST /eapi/repro HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		targetHost,
		len(requestBody),
		requestBody,
	)
	require.NoError(t, err)

	response, err := http.ReadResponse(bufio.NewReader(client), &http.Request{Method: http.MethodPost})
	require.NoError(t, err)
	responseBody, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.JSONEq(t, `{"code":200}`, string(responseBody))
	require.Equal(t, struct {
		host string
		path string
		body string
	}{host: targetHost, path: "/eapi/repro", body: requestBody}, <-received)

	requireOutputContains(t, output, "REQUEST protocol=eapi")
	requireOutputContains(t, output, "POST https://"+targetHost+"/eapi/repro")
	requireOutputContains(t, diagnostics, "action=inspect_sni reason=ip_target")
	requireOutputContains(t, diagnostics, `sni="`+targetHost+`" action=mitm reason=sni_target`)
	requireOutputContains(t, diagnostics, "cert_matches_connect=false cert_matches_sni=true")
}

func TestIPConnectTargetSNIUsesHTTP2AndPinnedUpstreamDial(t *testing.T) {
	const targetHost = "interface.music.163.com"

	type observedRequest struct {
		host               string
		path               string
		protocol           string
		negotiatedProtocol string
	}

	observed := make(chan observedRequest, 2)
	proxyURL, ca, output, _, upstreamTransport := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, true)
	origin := newTLSServerForHostWithHTTP2(t, ca, targetHost, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		observed <- observedRequest{
			host:               request.Host,
			path:               request.URL.Path,
			protocol:           request.Proto,
			negotiatedProtocol: request.TLS.NegotiatedProtocol,
		}

		writer.Header().Set("Content-Type", "text/plain")

		switch request.URL.Path {
		case "/api/ip-h2/first":
			_, _ = io.WriteString(writer, "response:/api/ip-h2/first")
		case "/api/ip-h2/second":
			_, _ = io.WriteString(writer, "response:/api/ip-h2/second")
		default:
			http.NotFound(writer, request)
		}
	}))

	upstreamRoots := x509.NewCertPool()
	upstreamRoots.AddCert(ca.Leaf)
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: upstreamRoots, MinVersion: tls.VersionTLS12}

	dialed := make(chan string, 4)
	upstreamTransport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed <- address

		var dialer net.Dialer

		return dialer.DialContext(ctx, network, address)
	}

	connectTarget := origin.Listener.Addr().String()
	proxyRoots := x509.NewCertPool()
	proxyRoots.AddCert(ca.Leaf)
	client := newMITMTestTLSClientWithNextProtos(t, proxyURL, connectTarget, targetHost, proxyRoots, []string{"h2"})
	require.NoError(t, client.Handshake())
	require.Equal(t, "h2", client.ConnectionState().NegotiatedProtocol)

	h2Client, err := (&http2.Transport{}).NewClientConn(client)
	require.NoError(t, err)
	t.Cleanup(func() { _ = h2Client.Close() })

	roundTrip := func(path string) {
		request, requestErr := http.NewRequest(http.MethodGet, "https://"+targetHost+path, http.NoBody)
		require.NoError(t, requestErr)

		response, roundTripErr := h2Client.RoundTrip(request)
		require.NoError(t, roundTripErr)

		body, readErr := io.ReadAll(response.Body)
		require.NoError(t, readErr)
		require.NoError(t, response.Body.Close())
		require.Equal(t, "HTTP/2.0", response.Proto)
		require.Equal(t, "response:"+path, string(body))
	}

	roundTrip("/api/ip-h2/first")
	require.Equal(t, connectTarget, <-dialed)

	// Force the pinned transport to open another upstream connection while the
	// client keeps using the same HTTP/2 MITM connection.
	origin.CloseClientConnections()
	roundTrip("/api/ip-h2/second")
	require.Equal(t, connectTarget, <-dialed)

	for _, path := range []string{"/api/ip-h2/first", "/api/ip-h2/second"} {
		got := <-observed
		require.Equal(t, observedRequest{
			host:               targetHost,
			path:               path,
			protocol:           "HTTP/2.0",
			negotiatedProtocol: "h2",
		}, got)
		requireOutputContains(t, output, "GET https://"+targetHost+path)
	}
}

func TestIPConnectWithNonTargetSNIStaysTunneled(t *testing.T) {
	const serverName = "other.example"

	proxyURL, ca, output, diagnostics, _ := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, true)
	origin := newTLSServerForHost(t, ca, serverName, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "tunneled")
	}))

	roots := x509.NewCertPool()
	roots.AddCert(ca.Leaf)
	client := newMITMTestTLSClient(t, proxyURL, origin.Listener.Addr().String(), serverName, roots)
	_, err := fmt.Fprintf(client, "GET /not-target HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", serverName)
	require.NoError(t, err)

	response, err := http.ReadResponse(bufio.NewReader(client), &http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, "tunneled", string(body))
	require.Empty(t, output.String())
	requireOutputContains(t, diagnostics, `sni="`+serverName+`" action=tunnel reason=sni_not_target`)
	require.NotContains(t, diagnostics.String(), "signing for "+serverName)
}

func TestNonTargetHostnameConnectLogsTunnelDecision(t *testing.T) {
	const (
		connectTarget = "other.example:443"
		serverName    = "other.example"
	)

	proxyURL, ca, output, diagnostics, upstreamTransport := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, true)
	origin := newTLSServerForHost(t, ca, serverName, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "named tunnel")
	}))
	dialed := make(chan string, 1)
	upstreamTransport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed <- address

		var dialer net.Dialer

		return dialer.DialContext(ctx, network, origin.Listener.Addr().String())
	}

	roots := x509.NewCertPool()
	roots.AddCert(ca.Leaf)
	client := newMITMTestTLSClient(t, proxyURL, connectTarget, serverName, roots)
	_, err := fmt.Fprintf(client, "GET /not-target HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", serverName)
	require.NoError(t, err)

	response, err := http.ReadResponse(bufio.NewReader(client), &http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, "named tunnel", string(body))
	require.Equal(t, connectTarget, <-dialed)
	require.Empty(t, output.String())
	requireOutputContains(t, diagnostics, `connect_target="`+connectTarget+`"`)
	requireOutputContains(t, diagnostics, "action=tunnel reason=target_not_matched")
}

func TestNonTargetHTTPSIsTunneledWithoutCapture(t *testing.T) {
	t.Parallel()

	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "tunneled")
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, output, diagnostics, _ := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, true)
	originRoots := x509.NewCertPool()
	originRoots.AddCert(origin.Certificate())
	client := newProxyClient(t, proxyURL, originRoots, false)

	resp, err := client.Get(origin.URL + "/api/test")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "tunneled", string(body))
	require.Empty(t, output.String())
	requireOutputContains(t, diagnostics, "action=inspect_sni reason=ip_target")
	requireOutputContains(t, diagnostics, `sni="" action=tunnel reason=sni_missing`)
}

func TestIPConnectWithNonTLSPayloadStaysTunneled(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "plaintext tunnel")
	}))
	t.Cleanup(origin.Close)

	originURL, err := url.Parse(origin.URL)
	require.NoError(t, err)
	proxyURL, _, output, diagnostics, _ := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, true)
	client, err := net.Dial("tcp", proxyURL.Host)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))

	_, err = fmt.Fprintf(client, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", originURL.Host, originURL.Host)
	require.NoError(t, err)

	reader := bufio.NewReader(client)
	connectResponse, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, connectResponse.StatusCode)
	require.NoError(t, connectResponse.Body.Close())

	_, err = fmt.Fprintf(client, "GET /plain HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", originURL.Host)
	require.NoError(t, err)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, "plaintext tunnel", string(body))
	require.Empty(t, output.String())
	requireOutputContains(t, diagnostics, "action=tunnel reason=not_tls_client_hello")
}

func TestIPConnectForwardsUpstreamFirstPayloadWithoutWaitingForClient(t *testing.T) {
	origin, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = origin.Close() })

	originDone := make(chan error, 1)

	go func() {
		conn, acceptErr := origin.Accept()
		if acceptErr != nil {
			originDone <- acceptErr
			return
		}
		defer func() { _ = conn.Close() }()

		if _, writeErr := io.WriteString(conn, "220 ready\r\n"); writeErr != nil {
			originDone <- writeErr
			return
		}

		line, readErr := bufio.NewReader(conn).ReadString('\n')
		if readErr == nil && line != "QUIT\r\n" {
			readErr = fmt.Errorf("origin received %q, want QUIT", line)
		}

		originDone <- readErr
	}()

	proxyURL, _, output, diagnostics, _ := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, true)
	client, err := net.Dial("tcp", proxyURL.Host)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.SetDeadline(time.Now().Add(2*time.Second)))

	_, err = fmt.Fprintf(client, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", origin.Addr(), origin.Addr())
	require.NoError(t, err)

	reader := bufio.NewReader(client)
	connectResponse, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, connectResponse.StatusCode)
	require.NoError(t, connectResponse.Body.Close())

	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "220 ready\r\n", line)

	_, err = io.WriteString(client, "QUIT\r\n")
	require.NoError(t, err)
	require.NoError(t, <-originDone)
	require.Empty(t, output.String())
	requireOutputContains(t, diagnostics, "action=tunnel reason=upstream_first")
}

func TestIPConnectTargetSNIWebSocketClosesAfterUpstreamExit(t *testing.T) {
	const targetHost = "interface.music.163.com"

	originDone := make(chan error, 1)
	proxyURL, ca, output, _, upstreamTransport := newTestProxyWithDebug(t, []string{"music.163.com"}, 1<<20, true)
	origin := newTLSServerForHost(t, ca, targetHost, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			originDone <- errors.New("origin response writer does not support hijacking")
			return
		}

		conn, readWriter, err := hijacker.Hijack()
		if err != nil {
			originDone <- err
			return
		}
		defer func() { _ = conn.Close() }()

		_, err = fmt.Fprint(readWriter, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: test\r\n\r\n")
		if err == nil {
			err = readWriter.Flush()
		}

		payload := make([]byte, 4)
		if err == nil {
			_, err = io.ReadFull(readWriter, payload)
		}

		if err == nil {
			_, err = readWriter.Write(payload)
		}

		if err == nil {
			err = readWriter.Flush()
		}

		originDone <- err
	}))

	upstreamRoots := x509.NewCertPool()
	upstreamRoots.AddCert(ca.Leaf)
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: upstreamRoots, MinVersion: tls.VersionTLS12}
	proxyRoots := x509.NewCertPool()
	proxyRoots.AddCert(ca.Leaf)
	client := newMITMTestTLSClient(t, proxyURL, origin.Listener.Addr().String(), targetHost, proxyRoots)

	_, err := fmt.Fprintf(
		client,
		"GET /socket HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n",
		targetHost,
	)
	require.NoError(t, err)

	reader := bufio.NewReader(client)
	statusLine, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Contains(t, statusLine, "101")

	for {
		line, readErr := reader.ReadString('\n')
		require.NoError(t, readErr)

		if line == "\r\n" {
			break
		}
	}

	_, err = client.Write([]byte("ping"))
	require.NoError(t, err)

	echo := make([]byte, 4)
	_, err = io.ReadFull(reader, echo)
	require.NoError(t, err)
	require.Equal(t, "ping", string(echo))
	require.NoError(t, <-originDone)

	_, err = reader.ReadByte()
	require.Error(t, err)
	requireOutputContains(t, output, "protocol upgrade body omitted")
}

func TestCaptureLimitDoesNotTruncateForwardedBodies(t *testing.T) {
	t.Parallel()

	requestBody := bytes.Repeat([]byte("request-"), 4096)
	responseBody := bytes.Repeat([]byte("response-"), 4096)
	received := make(chan []byte, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read origin request body: %v", err)
			return
		}

		received <- body

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", stringInt(len(responseBody)))
		_, _ = w.Write(responseBody)
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, output, _ := newTestProxy(t, []string{"127.0.0.1"}, 1024)
	client := newProxyClient(t, proxyURL, nil, true)
	req, err := http.NewRequest(http.MethodPost, origin.URL+"/api/large", bytes.NewReader(requestBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")
	resp, err := client.Do(req)
	require.NoError(t, err)
	gotResponse, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	require.Equal(t, requestBody, <-received)
	require.Equal(t, responseBody, gotResponse)
	require.Equal(t, int64(len(responseBody)), resp.ContentLength)
	require.Eventually(t, func() bool {
		return strings.Count(output.String(), "truncated=true") >= 2
	}, 2*time.Second, 10*time.Millisecond)
}

func TestCompressedResponseIsDecodedOnlyForDisplay(t *testing.T) {
	t.Parallel()

	original := []byte(`{"code":200,"name":"song"}`)

	tests := []struct {
		name     string
		encoding string
		encode   func(*testing.T, []byte) []byte
	}{
		{name: "gzip", encoding: "gzip", encode: gzipTestBody},
		{name: "deflate", encoding: "deflate", encode: deflateTestBody},
		{name: "brotli", encoding: "br", encode: brotliTestBody},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			wireBody := test.encode(t, original)
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Encoding", test.encoding)
				w.Header().Set("Content-Length", stringInt(len(wireBody)))
				_, _ = w.Write(wireBody)
			}))
			t.Cleanup(origin.Close)

			proxyURL, _, output, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
			client := newProxyClient(t, proxyURL, nil, true)
			req, err := http.NewRequest(http.MethodGet, origin.URL+"/api/compressed", http.NoBody)
			require.NoError(t, err)
			req.Header.Set("Accept-Encoding", test.encoding)
			resp, err := client.Do(req)
			require.NoError(t, err)
			gotWireBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			require.Equal(t, wireBody, gotWireBody)
			require.Equal(t, int64(len(wireBody)), resp.ContentLength)
			requireOutputContains(t, output, `"name": "song"`)
		})
	}
}

func TestProxyDoesNotAddAcceptEncoding(t *testing.T) {
	t.Parallel()

	acceptEncoding := make(chan string, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		acceptEncoding <- req.Header.Get("Accept-Encoding")

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":200}`)
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, _, upstreamTransport := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	require.True(t, upstreamTransport.DisableCompression)
	client := newProxyClient(t, proxyURL, nil, true)
	resp, err := client.Get(origin.URL + "/api/plain")
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Empty(t, <-acceptEncoding)
}

func TestRecorderSerializesConcurrentSubmissions(t *testing.T) {
	t.Parallel()

	output := &lockedBuffer{}
	recorder := newRecorder(output, 1<<20, false)
	t.Cleanup(recorder.Close)

	const blocks = 32

	expected := make([]string, blocks)
	accepted := make(chan bool, blocks)

	var group sync.WaitGroup

	for i := range blocks {
		block := fmt.Sprintf("BEGIN-%02d\n%s\nEND-%02d\n", i, strings.Repeat(strconv.Itoa(i%10), 4096), i)
		expected[i] = block + "\n"

		group.Add(1)
		go func(block string) {
			defer group.Done()

			accepted <- recorder.submit(func() { recorder.writeBlock([]byte(block)) })
		}(block)
	}

	group.Wait()

	for range blocks {
		require.True(t, <-accepted)
	}

	require.Eventually(t, func() bool { return flushRecorder(recorder, 100*time.Millisecond) }, time.Second, 10*time.Millisecond)

	text := output.String()
	for _, block := range expected {
		require.Contains(t, text, block)
	}

	require.Equal(t, blocks, strings.Count(text, "BEGIN-"))
	require.Equal(t, blocks, strings.Count(text, "END-"))
}

func TestUnknownLengthRequestIsForwardedWithoutPreRead(t *testing.T) {
	t.Parallel()

	firstChunk := bytes.Repeat([]byte("request-stream-"), 64)
	lastChunk := []byte("tail")
	firstReceived := make(chan []byte, 1)
	completeRequest := make(chan []byte, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		prefix := make([]byte, len(firstChunk))
		if _, err := io.ReadFull(req.Body, prefix); err != nil {
			firstReceived <- nil
			return
		}

		firstReceived <- prefix

		rest, err := io.ReadAll(req.Body)
		if err != nil {
			completeRequest <- nil
			return
		}

		complete := make([]byte, len(prefix)+len(rest))
		copy(complete, prefix)
		copy(complete[len(prefix):], rest)

		completeRequest <- complete

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":200}`)
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, output, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	client := newProxyClient(t, proxyURL, nil, true)
	reader, writer := io.Pipe()
	req, err := http.NewRequest(http.MethodPost, origin.URL+"/api/stream", reader)
	require.NoError(t, err)

	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")

	response := make(chan struct {
		resp *http.Response
		err  error
	}, 1)
	go func() {
		resp, requestErr := client.Do(req) //nolint:bodyclose // The receiver owns and closes the response body.
		response <- struct {
			resp *http.Response
			err  error
		}{resp: resp, err: requestErr}
	}()

	_, err = writer.Write(firstChunk)
	require.NoError(t, err)

	select {
	case got := <-firstReceived:
		require.Equal(t, firstChunk, got)
	case <-time.After(2 * time.Second):
		t.Fatal("origin did not receive the first request chunk before EOF")
	}

	_, err = writer.Write(lastChunk)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	result := <-response
	require.NoError(t, result.err)
	_, err = io.Copy(io.Discard, result.resp.Body)
	require.NoError(t, err)
	require.NoError(t, result.resp.Body.Close())
	require.Equal(t, append(firstChunk, lastChunk...), <-completeRequest)
	requireOutputContains(t, output, "unknown-length request body omitted")
}

func TestKnownLengthExpectContinueIsForwardedBeforeBodyCapture(t *testing.T) {
	t.Parallel()

	firstChunk := []byte(`{"message":"known-length-`)
	lastChunk := []byte(`body"}`)
	headersReceived := make(chan struct{})
	received := make(chan []byte, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		close(headersReceived)

		body, err := io.ReadAll(req.Body)
		if err != nil {
			received <- nil
			return
		}

		received <- body

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":200}`)
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, output, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	client := newProxyClient(t, proxyURL, nil, true)
	reader, writer := io.Pipe()

	t.Cleanup(func() { _ = writer.Close() })

	req, err := http.NewRequest(http.MethodPost, origin.URL+"/api/known-length", reader)
	require.NoError(t, err)

	req.ContentLength = int64(len(firstChunk) + len(lastChunk))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Expect", "100-continue")

	result := make(chan struct {
		response *http.Response
		err      error
	}, 1)
	go func() {
		response, requestErr := client.Do(req) //nolint:bodyclose // The receiver owns and closes the response body.
		result <- struct {
			response *http.Response
			err      error
		}{response: response, err: requestErr}
	}()

	select {
	case <-headersReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("origin did not receive known-length request headers before the body was available")
	}

	_, err = writer.Write(firstChunk)
	require.NoError(t, err)
	_, err = writer.Write(lastChunk)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	response := <-result
	require.NoError(t, response.err)
	_, err = io.Copy(io.Discard, response.response.Body)
	require.NoError(t, err)
	require.NoError(t, response.response.Body.Close())
	require.Equal(t, append(firstChunk, lastChunk...), <-received)
	requireOutputContains(t, output, "known-length-body")
}

func TestChunkedResponseIsCapturedWhileStreaming(t *testing.T) {
	t.Parallel()

	firstChunk := `{"items":[` + strings.Repeat(" ", 64<<10)
	lastChunk := `1,2,3]}`
	firstSent := make(chan struct{})
	releaseTail := make(chan struct{}, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("origin response writer does not support flushing")
			return
		}

		_, _ = io.WriteString(w, firstChunk)

		flusher.Flush()
		close(firstSent)
		<-releaseTail

		_, _ = io.WriteString(w, lastChunk)
	}))
	t.Cleanup(origin.Close)
	t.Cleanup(func() {
		select {
		case releaseTail <- struct{}{}:
		default:
		}
	})

	proxyURL, _, output, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	client := newProxyClient(t, proxyURL, nil, true)

	response := make(chan struct {
		resp *http.Response
		err  error
	}, 1)
	go func() {
		resp, err := client.Get(origin.URL + "/api/chunked") //nolint:bodyclose // The receiver owns and closes the response body.
		response <- struct {
			resp *http.Response
			err  error
		}{resp: resp, err: err}
	}()

	select {
	case <-firstSent:
	case <-time.After(2 * time.Second):
		t.Fatal("origin did not send the first response chunk")
	}

	var result struct {
		resp *http.Response
		err  error
	}
	select {
	case result = <-response:
		require.NoError(t, result.err)
	case <-time.After(2 * time.Second):
		t.Fatal("proxy did not forward response headers while the body was still streaming")
	}

	prefix := make([]byte, len(firstChunk))
	_, err := io.ReadFull(result.resp.Body, prefix)
	require.NoError(t, err)
	require.Equal(t, firstChunk, string(prefix))
	require.NotContains(t, output.String(), "RESPONSE status=200")

	releaseTail <- struct{}{}

	rest, err := io.ReadAll(result.resp.Body)
	require.NoError(t, err)
	require.NoError(t, result.resp.Body.Close())
	require.JSONEq(t, `{"items":[1,2,3]}`, string(prefix)+string(rest))
	requireOutputContains(t, output, `"items":`)
	requireOutputContains(t, output, "RESPONSE status=200")
}

func TestWebSocketUpgradeIsForwardedWithoutWrappingBody(t *testing.T) {
	t.Parallel()

	originErrors := make(chan error, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			originErrors <- errors.New("origin response writer does not support hijacking")
			return
		}

		conn, readWriter, err := hijacker.Hijack()
		if err != nil {
			originErrors <- err
			return
		}
		defer closeTestResource(t, conn)

		_, err = fmt.Fprint(readWriter, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: test\r\n\r\n")
		if err == nil {
			err = readWriter.Flush()
		}

		if err != nil {
			originErrors <- err
			return
		}

		payload := make([]byte, 4)
		if _, err = io.ReadFull(readWriter, payload); err == nil {
			_, err = readWriter.Write(payload)
		}

		if err == nil {
			err = readWriter.Flush()
		}

		originErrors <- err
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, output, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	proxyConn, err := net.Dial("tcp", proxyURL.Host)
	require.NoError(t, err)

	t.Cleanup(func() { closeTestResource(t, proxyConn) })

	require.NoError(t, proxyConn.SetDeadline(time.Now().Add(5*time.Second)))

	originURL, err := url.Parse(origin.URL)
	require.NoError(t, err)
	_, err = fmt.Fprintf(proxyConn,
		"GET %s/socket HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n",
		origin.URL,
		originURL.Host,
	)
	require.NoError(t, err)

	reader := bufio.NewReader(proxyConn)
	statusLine, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Contains(t, statusLine, "101")

	for {
		line, readErr := reader.ReadString('\n')
		require.NoError(t, readErr)

		if line == "\r\n" {
			break
		}
	}

	_, err = proxyConn.Write([]byte("ping"))
	require.NoError(t, err)

	echo := make([]byte, 4)
	_, err = io.ReadFull(reader, echo)
	require.NoError(t, err)
	require.Equal(t, "ping", string(echo))
	require.NoError(t, <-originErrors)
	requireOutputContains(t, output, "protocol upgrade body omitted")
}

func TestUpstreamFailureIsReportedWithoutPanic(t *testing.T) {
	t.Parallel()

	closedAddress := reserveAddress(t)
	proxyURL, _, output, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	client := newProxyClient(t, proxyURL, nil, true)
	resp, err := client.Get("http://" + closedAddress + "/api/test")
	require.NoError(t, err)

	_, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	requireOutputContains(t, output, "RESPONSE_ERROR")
}

func TestHTTPSMITMUpstreamFailureReturnsBadGateway(t *testing.T) {
	t.Parallel()

	closedAddress := reserveAddress(t)
	proxyURL, ca, output, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	roots := x509.NewCertPool()
	roots.AddCert(ca.Leaf)
	client := newProxyClient(t, proxyURL, roots, true)

	response, err := client.Get("https://" + closedAddress + "/api/test")
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusBadGateway, response.StatusCode)
	require.Equal(t, "Bad Gateway\n", string(body))
	requireOutputContains(t, output, "RESPONSE_ERROR")
}

func TestRecorderRedactsResponseErrorDetails(t *testing.T) {
	t.Parallel()

	output := &lockedBuffer{}
	recorder := newRecorder(output, 1<<20, false)
	t.Cleanup(recorder.Close)

	requestURL, err := url.Parse("https://music.163.com/api/test?name=song")
	require.NoError(t, err)
	recorder.recordResponseError(&captureState{
		session:       1,
		started:       time.Now(),
		requestMethod: http.MethodGet,
		requestURL:    requestURL,
	}, errors.New("fetch https://user:password@music.163.com/api?token=error-secret failed"))

	require.True(t, flushRecorder(recorder, time.Second))

	text := output.String()
	require.Contains(t, text, "RESPONSE_ERROR")
	require.Contains(t, text, redactedValue)

	for _, secret := range []string{"user", "password", "error-secret"} {
		require.NotContains(t, text, secret)
	}
}

func TestRunStartsAndStopsWithContext(t *testing.T) {
	address := reserveAddress(t)
	dir := t.TempDir()

	var (
		output      bytes.Buffer
		diagnostics bytes.Buffer
	)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, &Config{
			ListenAddr:      address,
			CACertPath:      filepath.Join(dir, "proxy", "ca.crt"),
			CAKeyPath:       filepath.Join(dir, "proxy", "ca.key"),
			MaxBodyBytes:    1024,
			Domains:         []string{"127.0.0.1"},
			Out:             &output,
			ErrOut:          &diagnostics,
			ShutdownTimeout: time.Second,
		})
	}()

	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", address, 20*time.Millisecond)
		if err != nil {
			return false
		}

		_ = conn.Close()
		return true
	}, 5*time.Second, 20*time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not stop after context cancellation")
	}

	require.Contains(t, diagnostics.String(), "ncmctl proxy listening")
	require.Empty(t, output.String())

	_, err := os.Stat(filepath.Join(dir, "proxy", "ca.crt"))
	require.NoError(t, err)
}

func TestRunReportsListenConflictBeforeCreatingCA(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	dir := filepath.Join(t.TempDir(), "proxy")
	certPath := filepath.Join(dir, "ca.crt")
	err = Run(context.Background(), &Config{
		ListenAddr:   listener.Addr().String(),
		CACertPath:   certPath,
		CAKeyPath:    filepath.Join(dir, "ca.key"),
		MaxBodyBytes: 1024,
		Domains:      []string{"music.163.com"},
		Out:          io.Discard,
		ErrOut:       io.Discard,
	})
	require.ErrorContains(t, err, "listen on")

	_, statErr := os.Stat(certPath)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestPrintStartupWarnsForLANAndSensitiveOutput(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proxy")
	ca, _, err := loadOrCreateCA(filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key"), false)
	require.NoError(t, err)

	var diagnostics bytes.Buffer

	err = printStartup(&Config{
		ListenAddr:    "0.0.0.0:9000",
		CACertPath:    filepath.Join(dir, "ca.crt"),
		ShowSensitive: true,
		ErrOut:        &diagnostics,
	}, &net.TCPAddr{IP: net.IPv4zero, Port: 9000}, ca, true)
	require.NoError(t, err)

	text := diagnostics.String()
	require.Contains(t, text, "unauthenticated open proxy")
	require.Contains(t, text, "trusted network behind a firewall")
	require.Contains(t, text, "terminal or redirected files")
	require.Contains(t, text, "CA SHA-256")
}

func TestDiagnosticWriterReportsShortWrites(t *testing.T) {
	n, err := (&diagnosticWriter{out: shortWriter{}, showSensitive: true}).Write([]byte("diagnostic"))
	require.ErrorIs(t, err, io.ErrShortWrite)
	require.Zero(t, n)
}

func TestRunClosesActiveConnectTunnel(t *testing.T) {
	origin, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = origin.Close() })

	accepted := make(chan net.Conn, 1)

	go func() {
		conn, acceptErr := origin.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
	}()

	address := reserveAddress(t)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, &Config{
			ListenAddr:      address,
			CACertPath:      filepath.Join(dir, "proxy", "ca.crt"),
			CAKeyPath:       filepath.Join(dir, "proxy", "ca.key"),
			MaxBodyBytes:    1024,
			Domains:         []string{"music.163.com"},
			Out:             io.Discard,
			ErrOut:          io.Discard,
			ShutdownTimeout: time.Second,
		})
	}()

	require.Eventually(t, func() bool {
		conn, dialErr := net.DialTimeout("tcp", address, 20*time.Millisecond)
		if dialErr != nil {
			return false
		}

		_ = conn.Close()
		return true
	}, 5*time.Second, 20*time.Millisecond)

	clientConn, err := net.Dial("tcp", address)
	require.NoError(t, err)

	defer closeTestResource(t, clientConn)

	_, err = fmt.Fprintf(clientConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", origin.Addr(), origin.Addr())
	require.NoError(t, err)

	reader := bufio.NewReader(clientConn)
	connectResponse, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, connectResponse.StatusCode)
	require.NoError(t, connectResponse.Body.Close())

	select {
	case originConn := <-accepted:
		defer closeTestResource(t, originConn)
	case <-time.After(2 * time.Second):
		t.Fatal("proxy did not establish the CONNECT tunnel")
	}

	cancel()

	select {
	case runErr := <-done:
		require.NoError(t, runErr)
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not stop with an active CONNECT tunnel")
	}

	require.NoError(t, clientConn.SetReadDeadline(time.Now().Add(time.Second)))

	_, err = reader.ReadByte()
	require.Error(t, err)
}

func TestNonTargetConnectPreservesHalfClose(t *testing.T) {
	origin, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = origin.Close() })

	requestBody := []byte("request after CONNECT")
	responseBody := []byte("response after client half-close")
	originResult := make(chan error, 1)

	go func() {
		conn, acceptErr := origin.Accept()
		if acceptErr != nil {
			originResult <- acceptErr
			return
		}
		defer closeTestResource(t, conn)

		body, readErr := io.ReadAll(conn)
		if readErr != nil {
			originResult <- readErr
			return
		}

		if !bytes.Equal(body, requestBody) {
			originResult <- fmt.Errorf("origin received %q, want %q", body, requestBody)
			return
		}

		_, writeErr := conn.Write(responseBody)
		originResult <- writeErr
	}()

	address := reserveAddress(t)
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	proxyDone := make(chan error, 1)
	go func() {
		proxyDone <- Run(ctx, &Config{
			ListenAddr:      address,
			CACertPath:      filepath.Join(dir, "proxy", "ca.crt"),
			CAKeyPath:       filepath.Join(dir, "proxy", "ca.key"),
			MaxBodyBytes:    1024,
			Domains:         []string{"music.163.com"},
			Out:             io.Discard,
			ErrOut:          io.Discard,
			ShutdownTimeout: time.Second,
		})
	}()

	waitForProxyListener(t, address)

	clientConn, err := net.Dial("tcp", address)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientConn.Close() })

	_, err = fmt.Fprintf(clientConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", origin.Addr(), origin.Addr())
	require.NoError(t, err)

	reader := bufio.NewReader(clientConn)
	connectResponse, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, connectResponse.StatusCode)
	require.NoError(t, connectResponse.Body.Close())

	_, err = clientConn.Write(requestBody)
	require.NoError(t, err)

	halfCloser, ok := clientConn.(interface{ CloseWrite() error })
	require.True(t, ok)
	require.NoError(t, halfCloser.CloseWrite())
	require.NoError(t, clientConn.SetReadDeadline(time.Now().Add(2*time.Second)))

	response, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, responseBody, response)

	select {
	case originErr := <-originResult:
		require.NoError(t, originErr)
	case <-time.After(2 * time.Second):
		t.Fatal("origin did not receive the client half-close")
	}

	cancel()

	select {
	case runErr := <-proxyDone:
		require.NoError(t, runErr)
	case <-time.After(5 * time.Second):
		t.Fatal("proxy did not stop after the half-close test")
	}
}

func TestCopyTunnelClosesBothConnectionsOnError(t *testing.T) {
	copyErr := errors.New("copy failed")
	destination := &halfCloseTestConn{}
	source := &halfCloseTestConn{readErr: copyErr}

	copyTunnel(destination, source, "upstream", nil)

	require.True(t, destination.closed)
	require.True(t, source.closed)
	require.False(t, destination.writeClosed)
	require.False(t, source.readClosed)
}

func TestConnectProbeDecision(t *testing.T) {
	matcher, err := newHostMatcher([]string{"music.163.com"})
	require.NoError(t, err)

	invalidHello := errors.New("invalid client hello")
	for _, test := range []struct {
		name  string
		probe connectProbeResult
		want  connectDecision
	}{
		{
			name: "upstream bytes take precedence",
			probe: connectProbeResult{
				upstreamPrefix: []byte("220 ready\r\n"),
				upstreamErr:    errors.New("upstream unavailable"),
				upstreamFirst:  true,
				serverName:     "interface.music.163.com",
				helloErr:       invalidHello,
			},
			want: connectDecision{route: connectRouteTunnel, reason: "upstream_first"},
		},
		{
			name:  "upstream completes first",
			probe: connectProbeResult{upstreamFirst: true, serverName: "interface.music.163.com", helloErr: invalidHello},
			want:  connectDecision{route: connectRouteTunnel, reason: "upstream_read_failed"},
		},
		{
			name:  "upstream read fails",
			probe: connectProbeResult{upstreamErr: errors.New("upstream unavailable"), serverName: "interface.music.163.com", helloErr: invalidHello},
			want:  connectDecision{route: connectRouteTunnel, reason: "upstream_read_failed"},
		},
		{
			name:  "client hello is invalid",
			probe: connectProbeResult{helloErr: invalidHello, serverName: "interface.music.163.com"},
			want:  connectDecision{route: connectRouteTunnel, reason: "client_hello_invalid"},
		},
		{
			name:  "SNI is missing",
			probe: connectProbeResult{},
			want:  connectDecision{route: connectRouteTunnel, reason: "sni_missing"},
		},
		{
			name:  "SNI is not a target",
			probe: connectProbeResult{serverName: "other.example"},
			want:  connectDecision{route: connectRouteTunnel, reason: "sni_not_target"},
		},
		{
			name:  "SNI is a target",
			probe: connectProbeResult{serverName: "interface.music.163.com"},
			want:  connectDecision{route: connectRouteMITM, reason: "sni_target"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, test.probe.decide(matcher))
		})
	}
}

func TestIPConnectTargetRequiresExplicitPort(t *testing.T) {
	for _, test := range []struct {
		target string
		want   bool
	}{
		{target: "127.0.0.1:443", want: true},
		{target: "127.0.0.1:8443", want: true},
		{target: "[2001:db8::1]:443", want: true},
		{target: "127.0.0.1", want: false},
		{target: "[2001:db8::1]", want: false},
		{target: "music.163.com:443", want: false},
	} {
		t.Run(test.target, func(t *testing.T) {
			require.Equal(t, test.want, isIPConnectTarget(test.target))
		})
	}
}

func TestPinnedDialerReusesOriginalConnectTarget(t *testing.T) {
	initial, initialPeer := net.Pipe()

	t.Cleanup(func() {
		_ = initial.Close()
		_ = initialPeer.Close()
	})

	redial, redialPeer := net.Pipe()

	t.Cleanup(func() {
		_ = redial.Close()
		_ = redialPeer.Close()
	})

	const connectTarget = "192.0.2.10:443"

	dialedAddress := ""
	base := &http.Transport{
		TLSHandshakeTimeout:   7 * time.Second,
		IdleConnTimeout:       8 * time.Second,
		ExpectContinueTimeout: 9 * time.Second,
		ForceAttemptHTTP2:     true,
		DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
			dialedAddress = address
			return redial, nil
		},
	}
	transport, closeUnused := newPinnedTransport(base, connectTarget, "interface.music.163.com", initial)
	t.Cleanup(transport.CloseIdleConnections)
	t.Cleanup(closeUnused)
	require.Equal(t, base.TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	require.Equal(t, base.IdleConnTimeout, transport.IdleConnTimeout)
	require.Equal(t, base.ExpectContinueTimeout, transport.ExpectContinueTimeout)
	require.Equal(t, base.ForceAttemptHTTP2, transport.ForceAttemptHTTP2)

	connection, err := transport.DialContext(context.Background(), "tcp", "interface.music.163.com:443")
	require.NoError(t, err)
	require.Same(t, initial, connection)
	connection, err = transport.DialContext(context.Background(), "tcp", "interface.music.163.com:443")
	require.NoError(t, err)
	require.Same(t, redial, connection)
	require.Equal(t, connectTarget, dialedAddress)
}

func TestSNIConnectDialUsesConnectRequestContext(t *testing.T) {
	requestContext, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodConnect, "http://127.0.0.1:443", http.NoBody).WithContext(requestContext)
	dialStarted := make(chan struct{})
	handler := &sniConnectHandler{
		transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				close(dialStarted)
				<-ctx.Done()
				return nil, ctx.Err()
			},
		},
	}
	client := &halfCloseTestConn{}
	done := make(chan struct{})

	go func() {
		handler.Handle("127.0.0.1:443", request, client, nil)
		close(done)
	}()

	<-dialStarted
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("IP CONNECT dial did not stop when its request context was canceled")
	}

	require.True(t, client.closed)
}

func TestMITMHandshakeTimeoutClosesStalledConnect(t *testing.T) {
	timeout := 100 * time.Millisecond
	proxyURL, _, _ := newTrackedTestProxy(t, []string{"127.0.0.1"}, timeout)

	for _, test := range []struct {
		name        string
		clientHello []byte
	}{
		{name: "before client hello"},
		{name: "incomplete client hello", clientHello: []byte{tlsHandshakeRecordType, 0x03, 0x03, 0x00, 0x20}},
		{name: "non-TLS byte without HTTP headers", clientHello: []byte("X")},
		{name: "incomplete HTTP/2 preface", clientHello: []byte(http2.ClientPreface[:18])},
	} {
		t.Run(test.name, func(t *testing.T) {
			clientConn, err := net.Dial("tcp", proxyURL.Host)
			require.NoError(t, err)

			defer closeTestResource(t, clientConn)

			_, err = fmt.Fprintf(clientConn, "CONNECT 127.0.0.1:443 HTTP/1.1\r\nHost: 127.0.0.1:443\r\n\r\n")
			require.NoError(t, err)

			reader := bufio.NewReader(clientConn)
			connectResponse, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, connectResponse.StatusCode)
			require.NoError(t, connectResponse.Body.Close())

			if len(test.clientHello) > 0 {
				_, err = clientConn.Write(test.clientHello)
				require.NoError(t, err)
			}

			started := time.Now()
			require.NoError(t, clientConn.SetReadDeadline(time.Now().Add(time.Second)))

			_, err = reader.ReadByte()
			require.Error(t, err)
			require.Less(t, time.Since(started), 800*time.Millisecond)
		})
	}
}

func TestMITMHandshakeTimeoutAfterKeepAliveHTTP(t *testing.T) {
	timeout := 100 * time.Millisecond
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, _ := newTrackedTestProxy(t, []string{"target.test"}, timeout)
	clientConn, err := net.Dial("tcp", proxyURL.Host)
	require.NoError(t, err)

	defer closeTestResource(t, clientConn)

	originURL, err := url.Parse(origin.URL)
	require.NoError(t, err)
	_, err = fmt.Fprintf(clientConn, "GET %s/ HTTP/1.1\r\nHost: %s\r\n\r\n", origin.URL, originURL.Host)
	require.NoError(t, err)

	reader := bufio.NewReader(clientConn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	_, err = fmt.Fprint(clientConn, "CONNECT target.test:443 HTTP/1.1\r\nHost: target.test:443\r\n\r\n")
	require.NoError(t, err)
	connectResponse, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, connectResponse.StatusCode)
	require.NoError(t, connectResponse.Body.Close())

	started := time.Now()
	require.NoError(t, clientConn.SetReadDeadline(time.Now().Add(time.Second)))

	_, err = reader.ReadByte()
	require.Error(t, err)
	require.Less(t, time.Since(started), 800*time.Millisecond)
}

func TestMITMHandshakeTimeoutAfterCompleteClientHello(t *testing.T) {
	for _, version := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		for _, target := range []struct {
			name      string
			connect   string
			ipConnect bool
		}{
			{
				name:    "hostname",
				connect: "target.test:443",
			},
			{
				name:      "ip with target SNI",
				ipConnect: true,
			},
		} {
			name := fmt.Sprintf("%s/%s", target.name, tlsVersionName(version))
			t.Run(name, func(t *testing.T) {
				timeout := 100 * time.Millisecond
				proxyURL, _, upstreamTransport := newTrackedTestProxy(t, []string{"target.test"}, timeout)

				connectTarget := target.connect
				if target.ipConnect {
					connectTarget = "127.0.0.1:443"
				}

				upstreamConn, upstreamPeer := net.Pipe()
				upstreamTransport.DialContext = func(context.Context, string, string) (net.Conn, error) {
					return upstreamConn, nil
				}

				t.Cleanup(func() {
					_ = upstreamConn.Close()
					_ = upstreamPeer.Close()
				})

				hello := captureClientHello(t, "target.test", version)

				rawConn, err := net.Dial("tcp", proxyURL.Host)
				require.NoError(t, err)
				t.Cleanup(func() { _ = rawConn.Close() })

				_, err = fmt.Fprintf(rawConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", connectTarget, connectTarget)
				require.NoError(t, err)

				reader := bufio.NewReader(rawConn)
				connectResponse, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, connectResponse.StatusCode)
				require.NoError(t, connectResponse.Body.Close())

				_, err = rawConn.Write(hello)
				require.NoError(t, err)

				started := time.Now()
				_ = rawConn.SetReadDeadline(time.Now().Add(time.Second))
				readDone := make(chan error, 1)

				go func() {
					_, readErr := io.Copy(io.Discard, reader)
					readDone <- readErr
				}()

				select {
				case <-readDone:
					require.Less(t, time.Since(started), 800*time.Millisecond)
				case <-time.After(2 * time.Second):
					t.Fatal("stalled TLS handshake was not closed")
				}
			})
		}
	}
}

func TestMITMHandshakeDeadlineClearsForLongLivedConnection(t *testing.T) {
	timeout := 100 * time.Millisecond
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(3 * timeout)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":200}`)
	}))
	t.Cleanup(origin.Close)

	proxyURL, ca, upstreamTransport := newTrackedTestProxy(t, []string{"127.0.0.1"}, timeout)
	originRoots := x509.NewCertPool()
	originRoots.AddCert(origin.Certificate())
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: originRoots, MinVersion: tls.VersionTLS12}
	proxyRoots := x509.NewCertPool()
	proxyRoots.AddCert(ca.Leaf)
	client := newProxyClient(t, proxyURL, proxyRoots, true)

	response, err := client.Get(origin.URL + "/api/slow")
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.JSONEq(t, `{"code":200}`, string(body))
}

func TestPlaintextTargetConnectClearsDeadlineAfterRequestHeaders(t *testing.T) {
	timeout := 100 * time.Millisecond
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(3 * timeout)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "plaintext response")
	}))
	t.Cleanup(origin.Close)
	originURL, err := url.Parse(origin.URL)
	require.NoError(t, err)

	proxyURL, _, _ := newTrackedTestProxy(t, []string{"127.0.0.1"}, timeout)
	clientConn, err := net.Dial("tcp", proxyURL.Host)
	require.NoError(t, err)

	defer closeTestResource(t, clientConn)

	_, err = fmt.Fprintf(clientConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", originURL.Host, originURL.Host)
	require.NoError(t, err)

	reader := bufio.NewReader(clientConn)
	connectResponse, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, connectResponse.StatusCode)
	require.NoError(t, connectResponse.Body.Close())

	_, err = fmt.Fprintf(clientConn, "GET /api/plain HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", originURL.Host)
	require.NoError(t, err)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "plaintext response", string(body))
}

func TestTrackedConnFindsSplitPlaintextHeaderEnd(t *testing.T) {
	client, server := net.Pipe()

	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	conn := &trackedConn{Conn: server}
	conn.armHandshakeDeadline(time.Second)
	conn.observeRead([]byte("GET / HTTP/1.1\r\nHost: music.163.com\r\n\r"))
	conn.observeRead([]byte("\n"))

	conn.mu.Lock()
	defer conn.mu.Unlock()

	require.False(t, conn.handshakeDeadlineActive)
	require.False(t, conn.plaintextPending)
	require.Nil(t, conn.plaintextHeader)
}

func TestTrackedConnWaitsForCompleteHTTP2Preface(t *testing.T) {
	client, server := net.Pipe()

	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	conn := &trackedConn{Conn: server}
	conn.armHandshakeDeadline(time.Second)
	conn.observeRead([]byte(http2.ClientPreface[:18]))

	conn.mu.Lock()
	require.True(t, conn.handshakeDeadlineActive)
	require.True(t, conn.plaintextPending)
	require.Equal(t, []byte(http2.ClientPreface[:18]), conn.plaintextHeader)
	conn.mu.Unlock()

	conn.observeRead([]byte(http2.ClientPreface[18:]))

	conn.mu.Lock()
	defer conn.mu.Unlock()

	require.False(t, conn.handshakeDeadlineActive)
	require.False(t, conn.plaintextPending)
	require.Nil(t, conn.plaintextHeader)
}

func TestTrackedConnClearsDeadlineWhenHTTP2PrefaceDiverges(t *testing.T) {
	client, server := net.Pipe()

	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	conn := &trackedConn{Conn: server}
	conn.armHandshakeDeadline(time.Second)
	conn.observeRead([]byte(http2.ClientPreface[:18]))
	conn.observeRead([]byte("X"))

	conn.mu.Lock()
	defer conn.mu.Unlock()

	require.False(t, conn.handshakeDeadlineActive)
	require.False(t, conn.plaintextPending)
	require.Nil(t, conn.plaintextHeader)
}

func TestTrackedConnPreservesHandshakeDeadlineAcrossHijackSetters(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func(*trackedConn) error
		last func(*deadlineRecordingConn) time.Time
	}{
		{
			name: "deadline",
			set:  func(conn *trackedConn) error { return conn.SetDeadline(time.Time{}) },
			last: func(conn *deadlineRecordingConn) time.Time { return conn.deadline },
		},
		{
			name: "read deadline",
			set:  func(conn *trackedConn) error { return conn.SetReadDeadline(time.Time{}) },
			last: func(conn *deadlineRecordingConn) time.Time { return conn.readDeadline },
		},
		{
			name: "write deadline",
			set:  func(conn *trackedConn) error { return conn.SetWriteDeadline(time.Time{}) },
			last: func(conn *deadlineRecordingConn) time.Time { return conn.writeDeadline },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			underlying := &deadlineRecordingConn{}
			conn := &trackedConn{Conn: underlying}
			conn.armHandshakeDeadline(time.Second)

			preserved := conn.handshakeDeadline
			require.NoError(t, test.set(conn))
			require.True(t, test.last(underlying).Equal(preserved))

			conn.clearHandshakeDeadline(0)
			require.True(t, underlying.deadline.IsZero())
			require.NoError(t, test.set(conn))
			require.True(t, test.last(underlying).IsZero())
		})
	}
}

func TestTrackedConnIgnoresLateHandshakeDeadlineClear(t *testing.T) {
	underlying := &deadlineRecordingConn{}
	conn := &trackedConn{Conn: underlying}

	firstGeneration := conn.armHandshakeDeadline(time.Second)
	firstDeadline := conn.handshakeDeadline
	secondGeneration := conn.armHandshakeDeadline(2 * time.Second)
	secondDeadline := conn.handshakeDeadline

	require.Greater(t, secondGeneration, firstGeneration)
	require.NotEqual(t, firstDeadline, secondDeadline)

	conn.clearHandshakeDeadline(firstGeneration)
	conn.mu.Lock()
	require.True(t, conn.handshakeDeadlineActive)
	require.Equal(t, secondDeadline, conn.handshakeDeadline)
	conn.mu.Unlock()

	conn.clearHandshakeDeadline(secondGeneration)
	conn.mu.Lock()
	defer conn.mu.Unlock()

	require.False(t, conn.handshakeDeadlineActive)
}

func newTestProxy(t *testing.T, domains []string, maxBodyBytes int64) (*url.URL, *tls.Certificate, *lockedBuffer, *http.Transport) {
	t.Helper()
	proxyURL, ca, output, _, upstreamTransport := newTestProxyWithDebug(t, domains, maxBodyBytes, false)
	return proxyURL, ca, output, upstreamTransport
}

type xeapiTestProxyHarness struct {
	url               *url.URL
	sessions          *xeapiSessionCache
	output            *lockedBuffer
	diagnostics       *lockedBuffer
	recorder          *recorder
	server            *httptest.Server
	upstreamTransport *http.Transport
}

func newXeapiTestProxy(t *testing.T) *xeapiTestProxyHarness {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "proxy")
	ca, _, err := loadOrCreateCA(filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key"), false)
	require.NoError(t, err)
	matcher, err := newHostMatcher([]string{"127.0.0.1"})
	require.NoError(t, err)

	output := &lockedBuffer{}
	diagnostics := &lockedBuffer{}
	sessions := newXeapiSessionCache(nil)
	recorder := newRecorderWithXeapiSessions(output, diagnostics, 1<<20, false, sessions)
	cfg := Config{MaxBodyBytes: 1 << 20, Out: output, ErrOut: diagnostics}
	proxyServer, upstreamTransport := newProxyServer(&cfg, matcher, ca, recorder, nil)
	server := httptest.NewServer(proxyServer)
	proxyURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	return &xeapiTestProxyHarness{
		url:               proxyURL,
		sessions:          sessions,
		output:            output,
		diagnostics:       diagnostics,
		recorder:          recorder,
		server:            server,
		upstreamTransport: upstreamTransport,
	}
}

func (h *xeapiTestProxyHarness) close() {
	h.server.Close()
	h.upstreamTransport.CloseIdleConnections()
	h.recorder.Close()
}

func newTestProxyWithDebug(t *testing.T, domains []string, maxBodyBytes int64, debug bool) (*url.URL, *tls.Certificate, *lockedBuffer, *lockedBuffer, *http.Transport) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "proxy")
	ca, _, err := loadOrCreateCA(filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key"), false)
	require.NoError(t, err)
	matcher, err := newHostMatcher(domains)
	require.NoError(t, err)

	output := &lockedBuffer{}
	diagnostics := &lockedBuffer{}
	cfg := Config{
		MaxBodyBytes:  maxBodyBytes,
		ShowSensitive: false,
		Debug:         debug,
		Out:           output,
		ErrOut:        diagnostics,
	}
	recorder := newRecorder(output, maxBodyBytes, false)
	proxyServer, upstreamTransport := newProxyServer(&cfg, matcher, ca, recorder, nil)
	server := httptest.NewServer(proxyServer)
	t.Cleanup(server.Close)
	t.Cleanup(upstreamTransport.CloseIdleConnections)
	t.Cleanup(recorder.Close)

	proxyURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	return proxyURL, ca, output, diagnostics, upstreamTransport
}

func newMITMTestTLSClient(t *testing.T, proxyURL *url.URL, connectTarget, serverName string, roots *x509.CertPool) *tls.Conn {
	t.Helper()
	return newMITMTestTLSClientWithNextProtos(t, proxyURL, connectTarget, serverName, roots, nil)
}

func newMITMTestTLSClientWithNextProtos(t *testing.T, proxyURL *url.URL, connectTarget, serverName string, roots *x509.CertPool, nextProtos []string) *tls.Conn {
	t.Helper()

	rawConn, err := net.Dial("tcp", proxyURL.Host)
	require.NoError(t, err)
	require.NoError(t, rawConn.SetDeadline(time.Now().Add(5*time.Second)))
	t.Cleanup(func() { _ = rawConn.Close() })

	_, err = fmt.Fprintf(rawConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", connectTarget, connectTarget)
	require.NoError(t, err)

	reader := bufio.NewReader(rawConn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, response.Body.Close())

	client := tls.Client(&bufferedTestConn{Conn: rawConn, reader: reader}, &tls.Config{
		RootCAs:    roots,
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
		NextProtos: append([]string(nil), nextProtos...),
	})

	t.Cleanup(func() { _ = client.Close() })
	return client
}

func newTLSServerForHost(t *testing.T, ca *tls.Certificate, host string, handler http.Handler) *httptest.Server {
	t.Helper()
	return newTLSServerForHostWithOptions(t, ca, host, handler, false)
}

func newTLSServerForHostWithHTTP2(t *testing.T, ca *tls.Certificate, host string, handler http.Handler) *httptest.Server {
	t.Helper()
	return newTLSServerForHostWithOptions(t, ca, host, handler, true)
}

func newTLSServerForHostWithOptions(t *testing.T, ca *tls.Certificate, host string, handler http.Handler, enableHTTP2 bool) *httptest.Server {
	t.Helper()

	certificateProxy := goproxy.NewProxyHttpServer()
	config, err := goproxy.TLSConfigFromCA(ca)(host, &goproxy.ProxyCtx{Proxy: certificateProxy})
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(handler)

	server.EnableHTTP2 = enableHTTP2
	if enableHTTP2 {
		require.NoError(t, http2.ConfigureServer(server.Config, &http2.Server{}))
	}

	server.TLS = &tls.Config{Certificates: config.Certificates, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}

func newTrackedTestProxy(t *testing.T, domains []string, handshakeTimeout time.Duration) (*url.URL, *tls.Certificate, *http.Transport) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "proxy")
	ca, _, err := loadOrCreateCA(filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.key"), false)
	require.NoError(t, err)
	matcher, err := newHostMatcher(domains)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	tracked := newTrackedListener(listener, handshakeTimeout)
	output := &lockedBuffer{}
	diagnostics := &lockedBuffer{}
	cfg := Config{
		MaxBodyBytes:  1 << 20,
		ShowSensitive: false,
		Out:           output,
		ErrOut:        diagnostics,
	}
	recorder := newRecorder(output, cfg.MaxBodyBytes, false)
	proxyServer, upstreamTransport := newProxyServer(&cfg, matcher, ca, recorder, tracked)
	httpServer := &http.Server{
		Handler:           proxyServer,
		ReadHeaderTimeout: time.Second,
		IdleTimeout:       time.Second,
		ErrorLog:          log.New(diagnostics, "proxy server: ", log.LstdFlags),
	}

	serveDone := make(chan error, 1)
	go func() { serveDone <- httpServer.Serve(tracked) }()

	t.Cleanup(func() {
		_ = httpServer.Close()
		_ = tracked.closeAll()

		upstreamTransport.CloseIdleConnections()
		recorder.Close()

		select {
		case serveErr := <-serveDone:
			if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) && !errors.Is(serveErr, net.ErrClosed) {
				t.Errorf("serve tracked test proxy: %v", serveErr)
			}
		case <-time.After(2 * time.Second):
			t.Error("tracked test proxy did not stop")
		}
	})
	return &url.URL{Scheme: "http", Host: tracked.Addr().String()}, ca, upstreamTransport
}

func waitForProxyListener(t *testing.T, address string) {
	t.Helper()
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", address, 20*time.Millisecond)
		if err != nil {
			return false
		}

		_ = conn.Close()
		return true
	}, 5*time.Second, 20*time.Millisecond)
}

func newProxyClient(t *testing.T, proxyURL *url.URL, roots *x509.CertPool, disableCompression bool) *http.Client {
	t.Helper()
	return newProxyClientWithHTTP2(t, proxyURL, roots, disableCompression, false, nil)
}

func newHTTP1ProxyClient(t *testing.T, proxyURL *url.URL, roots *x509.CertPool) *http.Client {
	t.Helper()
	return newProxyClientWithHTTP2(t, proxyURL, roots, false, false, []string{"http/1.1"})
}

func newHTTP2ProxyClient(t *testing.T, proxyURL *url.URL, roots *x509.CertPool) *http.Client {
	t.Helper()
	return newProxyClientWithHTTP2(t, proxyURL, roots, false, true, []string{"h2", "http/1.1"})
}

func newProxyClientWithHTTP2(t *testing.T, proxyURL *url.URL, roots *x509.CertPool, disableCompression, forceHTTP2 bool, nextProtos []string) *http.Client {
	t.Helper()

	transport := &http.Transport{
		Proxy:              http.ProxyURL(proxyURL),
		DisableCompression: disableCompression,
		ForceAttemptHTTP2:  forceHTTP2,
		TLSClientConfig: &tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS12,
			NextProtos: append([]string(nil), nextProtos...),
		},
	}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 5 * time.Second}
}

func reserveAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func closeTestResource(t *testing.T, closer io.Closer) {
	t.Helper()

	if err := closer.Close(); err != nil {
		t.Errorf("close test resource: %v", err)
	}
}

func stringInt(value int) string {
	return strconv.Itoa(value)
}

func gzipTestBody(t *testing.T, body []byte) []byte {
	t.Helper()

	var output bytes.Buffer

	writer := gzip.NewWriter(&output)
	_, err := writer.Write(body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return output.Bytes()
}

func deflateTestBody(t *testing.T, body []byte) []byte {
	t.Helper()

	var output bytes.Buffer

	writer := zlib.NewWriter(&output)
	_, err := writer.Write(body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return output.Bytes()
}

func brotliTestBody(t *testing.T, body []byte) []byte {
	t.Helper()

	var output bytes.Buffer

	writer := brotli.NewWriter(&output)
	_, err := writer.Write(body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return output.Bytes()
}

func requireOutputContains(t *testing.T, output *lockedBuffer, substring string) {
	t.Helper()
	require.Eventually(t, func() bool {
		return strings.Contains(output.String(), substring)
	}, 2*time.Second, 10*time.Millisecond)
}

func captureBlockForPath(output, kind, path string) (string, string, bool) {
	for block := range strings.SplitSeq(output, "\n\n") {
		lines := strings.Split(block, "\n")
		if len(lines) < 2 || !strings.Contains(lines[0], " "+kind+" ") || !strings.Contains(lines[1], path) {
			continue
		}

		for field := range strings.FieldsSeq(lines[0]) {
			if strings.HasPrefix(field, "#") {
				return block, field, true
			}
		}
	}

	return "", "", false
}

type lockedBuffer struct {
	mu     sync.RWMutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.buffer.String()
}

type bufferedTestConn struct {
	net.Conn

	reader *bufio.Reader
}

func (c *bufferedTestConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

type halfCloseTestConn struct {
	readErr     error
	closed      bool
	readClosed  bool
	writeClosed bool
}

func (c *halfCloseTestConn) Read([]byte) (int, error) {
	if c.readErr != nil {
		return 0, c.readErr
	}
	return 0, io.EOF
}

func (*halfCloseTestConn) Write(p []byte) (int, error) { return len(p), nil }

func (c *halfCloseTestConn) Close() error {
	c.closed = true
	return nil
}

func (c *halfCloseTestConn) CloseRead() error {
	c.readClosed = true
	return nil
}

func (c *halfCloseTestConn) CloseWrite() error {
	c.writeClosed = true
	return nil
}

func (*halfCloseTestConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (*halfCloseTestConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (*halfCloseTestConn) SetDeadline(time.Time) error      { return nil }
func (*halfCloseTestConn) SetReadDeadline(time.Time) error  { return nil }
func (*halfCloseTestConn) SetWriteDeadline(time.Time) error { return nil }

type deadlineRecordingConn struct {
	halfCloseTestConn

	deadline      time.Time
	readDeadline  time.Time
	writeDeadline time.Time
}

func tlsVersionName(version uint16) string {
	if version == tls.VersionTLS12 {
		return "tls12"
	}
	return "tls13"
}

func captureClientHello(t *testing.T, serverName string, version uint16) []byte {
	t.Helper()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	go func() {
		config := &tls.Config{ServerName: serverName, MinVersion: version, MaxVersion: version}
		_ = tls.Client(client, config).Handshake()
	}()

	var hello bytes.Buffer

	buf := make([]byte, 4096)
	for hello.Len() < 5 {
		n, err := server.Read(buf)
		require.NoError(t, err)
		hello.Write(buf[:n])
	}

	recordLength := int(hello.Bytes()[3])<<8 | int(hello.Bytes()[4])
	for hello.Len() < 5+recordLength {
		n, err := server.Read(buf)
		require.NoError(t, err)
		hello.Write(buf[:n])
	}
	return hello.Bytes()
}

func (c *deadlineRecordingConn) SetDeadline(deadline time.Time) error {
	c.deadline = deadline
	return nil
}

func (c *deadlineRecordingConn) SetReadDeadline(deadline time.Time) error {
	c.readDeadline = deadline
	return nil
}

func (c *deadlineRecordingConn) SetWriteDeadline(deadline time.Time) error {
	c.writeDeadline = deadline
	return nil
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return max(0, len(p)-1), nil }

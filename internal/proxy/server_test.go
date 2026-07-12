// Copyright (c) 2024-2026 chaunsin
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
	"github.com/stretchr/testify/require"
)

func TestHTTPProxyCapturesAndRedactsWithoutChangingTraffic(t *testing.T) {
	t.Parallel()

	requestBody := []byte(`{"name":"song","access_token":"request-secret"}`)
	responseBody := []byte(`{"code":200,"token":"response-secret"}`)
	received := make(chan []byte, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		received <- body
		require.Equal(t, "Bearer auth-secret", req.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "MUSIC_U=cookie-secret")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(responseBody)
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, output, _, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
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

func TestHTTPSMITMRequiresAndUsesGeneratedCA(t *testing.T) {
	t.Parallel()

	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":200}`)
	}))
	t.Cleanup(origin.Close)

	proxyURL, ca, output, _, upstreamTransport := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	originRoots := x509.NewCertPool()
	originRoots.AddCert(origin.Certificate())
	upstreamTransport.TLSClientConfig = &tls.Config{RootCAs: originRoots, MinVersion: tls.VersionTLS12}

	untrusted := newProxyClient(t, proxyURL, nil, false)
	_, err := untrusted.Get(origin.URL + "/api/test")
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
	requireOutputContains(t, output, "REQUEST protocol=api")
	requireOutputContains(t, output, "RESPONSE status=200")
}

func TestNonTargetHTTPSIsTunneledWithoutCapture(t *testing.T) {
	t.Parallel()

	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "tunneled")
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, output, _, _ := newTestProxy(t, []string{"music.163.com"}, 1<<20)
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
}

func TestCaptureLimitDoesNotTruncateForwardedBodies(t *testing.T) {
	t.Parallel()

	requestBody := bytes.Repeat([]byte("request-"), 4096)
	responseBody := bytes.Repeat([]byte("response-"), 4096)
	received := make(chan []byte, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		received <- body
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", stringInt(len(responseBody)))
		_, _ = w.Write(responseBody)
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, output, _, _ := newTestProxy(t, []string{"127.0.0.1"}, 1024)
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

			proxyURL, _, output, _, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
			client := newProxyClient(t, proxyURL, nil, true)
			req, err := http.NewRequest(http.MethodGet, origin.URL+"/api/compressed", nil)
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

	proxyURL, _, _, _, upstreamTransport := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
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
	for i := 0; i < blocks; i++ {
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
		completeRequest <- append(prefix, rest...)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":200}`)
	}))
	t.Cleanup(origin.Close)

	proxyURL, _, output, _, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
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
		resp, requestErr := client.Do(req)
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

	proxyURL, _, output, _, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
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
		response, requestErr := client.Do(req)
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

	proxyURL, _, output, _, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	client := newProxyClient(t, proxyURL, nil, true)
	response := make(chan struct {
		resp *http.Response
		err  error
	}, 1)
	go func() {
		resp, err := client.Get(origin.URL + "/api/chunked")
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
			originErrors <- fmt.Errorf("origin response writer does not support hijacking")
			return
		}
		conn, readWriter, err := hijacker.Hijack()
		if err != nil {
			originErrors <- err
			return
		}
		defer conn.Close()
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

	proxyURL, _, output, _, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
	proxyConn, err := net.Dial("tcp", proxyURL.Host)
	require.NoError(t, err)
	defer proxyConn.Close()
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
	proxyURL, _, output, _, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
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
	proxyURL, ca, output, _, _ := newTestProxy(t, []string{"127.0.0.1"}, 1<<20)
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
	}, fmt.Errorf("fetch https://user:password@music.163.com/api?token=error-secret failed"))

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
	var output bytes.Buffer
	var diagnostics bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Config{
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
	err = Run(context.Background(), Config{
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
	printStartup(Config{
		ListenAddr:    "0.0.0.0:9000",
		CACertPath:    filepath.Join(dir, "ca.crt"),
		ShowSensitive: true,
		ErrOut:        &diagnostics,
	}, &net.TCPAddr{IP: net.IPv4zero, Port: 9000}, ca, true)

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
		done <- Run(ctx, Config{
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
	defer clientConn.Close()
	_, err = fmt.Fprintf(clientConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", origin.Addr(), origin.Addr())
	require.NoError(t, err)
	reader := bufio.NewReader(clientConn)
	connectResponse, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, connectResponse.StatusCode)
	require.NoError(t, connectResponse.Body.Close())

	select {
	case originConn := <-accepted:
		defer originConn.Close()
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
		defer conn.Close()
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
		proxyDone <- Run(ctx, Config{
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

func TestMITMHandshakeTimeoutClosesStalledConnect(t *testing.T) {
	timeout := 100 * time.Millisecond
	proxyURL, _, _, _, _ := newTrackedTestProxy(t, []string{"127.0.0.1"}, timeout)

	for _, test := range []struct {
		name        string
		clientHello []byte
	}{
		{name: "before client hello"},
		{name: "incomplete client hello", clientHello: []byte{tlsHandshakeRecordType, 0x03, 0x03, 0x00, 0x20}},
		{name: "non-TLS byte without HTTP headers", clientHello: []byte("X")},
	} {
		t.Run(test.name, func(t *testing.T) {
			clientConn, err := net.Dial("tcp", proxyURL.Host)
			require.NoError(t, err)
			defer clientConn.Close()
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

	proxyURL, _, _, _, _ := newTrackedTestProxy(t, []string{"target.test"}, timeout)
	clientConn, err := net.Dial("tcp", proxyURL.Host)
	require.NoError(t, err)
	defer clientConn.Close()
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

func TestMITMHandshakeDeadlineClearsForLongLivedConnection(t *testing.T) {
	timeout := 100 * time.Millisecond
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(3 * timeout)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":200}`)
	}))
	t.Cleanup(origin.Close)

	proxyURL, ca, _, _, upstreamTransport := newTrackedTestProxy(t, []string{"127.0.0.1"}, timeout)
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

	proxyURL, _, _, _, _ := newTrackedTestProxy(t, []string{"127.0.0.1"}, timeout)
	clientConn, err := net.Dial("tcp", proxyURL.Host)
	require.NoError(t, err)
	defer clientConn.Close()
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

func newTestProxy(t *testing.T, domains []string, maxBodyBytes int64) (*url.URL, *tls.Certificate, *lockedBuffer, *lockedBuffer, *http.Transport) {
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
		Out:           output,
		ErrOut:        diagnostics,
	}
	recorder := newRecorder(output, maxBodyBytes, false)
	proxyServer, upstreamTransport := newProxyServer(cfg, matcher, ca, recorder, nil)
	server := httptest.NewServer(proxyServer)
	t.Cleanup(server.Close)
	t.Cleanup(upstreamTransport.CloseIdleConnections)
	t.Cleanup(recorder.Close)
	proxyURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	return proxyURL, ca, output, diagnostics, upstreamTransport
}

func newTrackedTestProxy(t *testing.T, domains []string, handshakeTimeout time.Duration) (*url.URL, *tls.Certificate, *lockedBuffer, *lockedBuffer, *http.Transport) {
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
	proxyServer, upstreamTransport := newProxyServer(cfg, matcher, ca, recorder, tracked)
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
	return &url.URL{Scheme: "http", Host: tracked.Addr().String()}, ca, output, diagnostics, upstreamTransport
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
	transport := &http.Transport{
		Proxy:              http.ProxyURL(proxyURL),
		DisableCompression: disableCompression,
		TLSClientConfig: &tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS12,
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

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return max(0, len(p)-1), nil }

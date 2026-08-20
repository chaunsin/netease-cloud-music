// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elazarl/goproxy"
	"golang.org/x/net/http2"
)

const (
	connectEstablishedResponse = "HTTP/1.0 200 Connection established\r\n\r\n"
	connectBadGatewayResponse  = "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
)

var errClientHelloCaptured = errors.New("client hello captured")

type connectRoute string

const (
	connectRouteTunnel connectRoute = "tunnel"
	connectRouteMITM   connectRoute = "mitm"
)

type connectDecision struct {
	route  connectRoute
	reason string
}

// connectRequestMetadata carries the SNI-selected route into each decrypted
// request without replacing its Host header or upstream destination.
type connectRequestMetadata struct {
	serverName   string
	roundTripper http.RoundTripper
}

type connectRequestMetadataKey struct{}

func requestConnectMetadata(request *http.Request) *connectRequestMetadata {
	if request == nil {
		return nil
	}

	metadata, _ := request.Context().Value(connectRequestMetadataKey{}).(*connectRequestMetadata)
	return metadata
}

// sniConnectHandler inspects IP-targeted CONNECT traffic just far enough to
// select a route. Every non-target prefix is replayed unchanged, while a target
// SNI remains pinned to the original CONNECT address upstream.
type sniConnectHandler struct {
	debug            bool
	matcher          *hostMatcher
	proxy            *goproxy.ProxyHttpServer
	transport        *http.Transport
	errorLog         *log.Logger
	tlsConfig        func(certificateHost, connectTarget string, proxyCtx *goproxy.ProxyCtx) (*tls.Config, error)
	handshakeTimeout time.Duration
}

func (h *sniConnectHandler) Handle(connectTarget string, request *http.Request, client net.Conn, proxyCtx *goproxy.ProxyCtx) {
	dialContext := context.Background()
	if request != nil {
		dialContext = request.Context()
	}

	upstream, err := h.dial(dialContext, connectTarget)
	if err != nil {
		if proxyCtx != nil {
			proxyCtx.Warnf("Cannot connect IP-targeted tunnel to %s: %v", connectTarget, err)
		}

		_, _ = io.WriteString(client, connectBadGatewayResponse)
		_ = client.Close()
		return
	}

	// The hijacked tunnel outlives ServeHTTP; socket deadlines and closure bound it.
	tunnelContext := context.WithoutCancel(dialContext)
	go h.handle(tunnelContext, connectTarget, client, upstream, proxyCtx)
}

func (h *sniConnectHandler) handle(ctx context.Context, connectTarget string, client, upstream net.Conn, proxyCtx *goproxy.ProxyCtx) {
	defer func() { _ = client.Close() }()
	defer func() { _ = upstream.Close() }()

	deadline := time.Now().Add(h.handshakeTimeout)
	_ = client.SetDeadline(deadline)

	if _, err := io.WriteString(client, connectEstablishedResponse); err != nil {
		return
	}

	probe := probeConnect(client, upstream, deadline)
	_ = client.SetDeadline(time.Time{})

	decision := probe.decide(h.matcher)
	h.logDecision(proxyCtx, connectTarget, probe.serverName, decision.route, decision.reason)

	if decision.route == connectRouteTunnel {
		relayTunnel(client, upstream, probe.clientPrefix, probe.upstreamPrefix, proxyCtx)
		return
	}

	config, err := h.tlsConfig(probe.serverName, connectTarget, proxyCtx)
	if err != nil {
		h.logDecision(proxyCtx, connectTarget, probe.serverName, connectRouteTunnel, "certificate_unavailable")
		relayTunnel(client, upstream, probe.clientPrefix, probe.upstreamPrefix, proxyCtx)
		return
	}

	tlsClient := tls.Server(&prefixedConn{Conn: client, reader: io.MultiReader(bytes.NewReader(probe.clientPrefix), client)}, config)
	if err := tlsClient.HandshakeContext(ctx); err != nil {
		if proxyCtx != nil {
			proxyCtx.Warnf("Cannot handshake IP-targeted client %s: %v", connectTarget, err)
		}
		return
	}

	_ = tlsClient.SetDeadline(time.Time{})

	h.serveMITM(tlsClient, upstream, connectTarget, probe.serverName, proxyCtx)
}

func (h *sniConnectHandler) dial(ctx context.Context, connectTarget string) (net.Conn, error) {
	if h.transport != nil && h.transport.DialContext != nil {
		return h.transport.DialContext(ctx, "tcp", connectTarget)
	}

	var dialer net.Dialer
	return dialer.DialContext(ctx, "tcp", connectTarget)
}

func (h *sniConnectHandler) logDecision(proxyCtx *goproxy.ProxyCtx, connectTarget, serverName string, route connectRoute, reason string) {
	if !h.debug || proxyCtx == nil {
		return
	}

	proxyCtx.Logf(
		"[TLS_DIAGNOSTIC] phase=client_hello connect_target=%q connect_host=%q sni=%q action=%s reason=%s",
		connectTarget,
		canonicalHostname(connectTarget),
		serverName,
		route,
		reason,
	)
}

func (h *sniConnectHandler) serveMITM(client *tls.Conn, upstream net.Conn, connectTarget, serverName string, proxyCtx *goproxy.ProxyCtx) {
	roundTripper, closeUnused := newPinnedTransport(h.transport, connectTarget, serverName, upstream)
	defer roundTripper.CloseIdleConnections()
	defer closeUnused()

	metadata := &connectRequestMetadata{serverName: serverName, roundTripper: roundTripper}
	listener := newSingleConnListener(client)
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer listener.closeHijacked()

		if request.URL == nil {
			http.Error(writer, "invalid request URL", http.StatusBadRequest)
			return
		}

		request.URL.Scheme = "https"
		if request.URL.Host == "" {
			request.URL.Host = request.Host
		}

		if request.URL.Host == "" {
			request.URL.Host = serverName
		}
		// x/net/http2 gives bodyless streams a non-nil empty body. Normalize it
		// before handing the request to goproxy so the upstream transport does
		// not infer a phantom request body on the IP-selected path.
		if request.ContentLength == 0 {
			request.Body = http.NoBody
		}

		ctx := context.WithValue(request.Context(), connectRequestMetadataKey{}, metadata)
		h.proxy.ServeHTTP(writer, request.WithContext(ctx))
	})

	server := &http.Server{
		Handler:   handler,
		ErrorLog:  h.errorLog,
		ConnState: listener.connState,
	}
	if err := http2.ConfigureServer(server, &http2.Server{}); err != nil {
		if proxyCtx != nil {
			proxyCtx.Warnf("Cannot configure HTTP/2 for SNI-selected MITM connection to %s: %v", connectTarget, err)
		}
		return
	}

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) && proxyCtx != nil {
		proxyCtx.Warnf("Cannot serve SNI-selected MITM connection to %s: %v", connectTarget, err)
	}
}

func newPinnedTransport(base *http.Transport, connectTarget, serverName string, initial net.Conn) (*http.Transport, func()) {
	transport := base.Clone()

	dial := base.DialContext
	if dial == nil {
		var netDialer net.Dialer

		dial = netDialer.DialContext
	}

	var initialMu sync.Mutex

	takeInitial := func() net.Conn {
		initialMu.Lock()
		defer initialMu.Unlock()

		conn := initial
		initial = nil
		return conn
	}

	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		if conn := takeInitial(); conn != nil {
			return conn, nil
		}
		return dial(ctx, network, connectTarget)
	}
	transport.DialTLSContext = nil

	tlsConfig := &tls.Config{}
	if transport.TLSClientConfig != nil {
		tlsConfig = transport.TLSClientConfig.Clone()
	}

	tlsConfig.ServerName = serverName
	transport.TLSClientConfig = tlsConfig
	closeUnused := func() {
		if conn := takeInitial(); conn != nil {
			_ = conn.Close()
		}
	}
	return transport, closeUnused
}

type prefixedConn struct {
	net.Conn

	reader io.Reader
}

func (c *prefixedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// singleConnListener keeps http.Server.Serve alive until its only connection
// closes. A hijacked WebSocket is signaled explicitly because net/http no
// longer owns it and will not report StateClosed.
type singleConnListener struct {
	conn     net.Conn
	done     chan struct{}
	hijacked atomic.Bool

	mu        sync.Mutex
	accepted  bool
	closeOnce sync.Once
}

func newSingleConnListener(conn net.Conn) *singleConnListener {
	return &singleConnListener{conn: conn, done: make(chan struct{})}
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if !l.accepted {
		l.accepted = true
		l.mu.Unlock()
		return l.conn, nil
	}

	done := l.done
	l.mu.Unlock()

	<-done
	return nil, net.ErrClosed
}

func (l *singleConnListener) Close() error {
	l.signal()
	return l.conn.Close()
}

func (l *singleConnListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

func (l *singleConnListener) signal() {
	l.closeOnce.Do(func() { close(l.done) })
}

func (l *singleConnListener) connState(_ net.Conn, state http.ConnState) {
	switch state {
	case http.StateHijacked:
		l.hijacked.Store(true)
	case http.StateClosed:
		l.signal()
	case http.StateNew, http.StateActive, http.StateIdle:
	}
}

func (l *singleConnListener) closeHijacked() {
	if l.hijacked.Load() {
		_ = l.Close()
	}
}

func relayTunnel(client, upstream net.Conn, clientPrefix, upstreamPrefix []byte, proxyCtx *goproxy.ProxyCtx) {
	if len(upstreamPrefix) > 0 {
		if _, err := io.Copy(client, bytes.NewReader(upstreamPrefix)); err != nil {
			if proxyCtx != nil {
				proxyCtx.Warnf("Cannot replay inspected upstream CONNECT bytes: %v", err)
			}
			return
		}
	}

	if len(clientPrefix) > 0 {
		if _, err := io.Copy(upstream, bytes.NewReader(clientPrefix)); err != nil {
			if proxyCtx != nil {
				proxyCtx.Warnf("Cannot replay inspected CONNECT bytes: %v", err)
			}
			return
		}
	}

	var wait sync.WaitGroup

	wait.Go(func() { copyTunnel(upstream, client, "upstream", proxyCtx) })
	wait.Go(func() { copyTunnel(client, upstream, "client", proxyCtx) })

	wait.Wait()
}

type halfClosableConn interface {
	net.Conn
	CloseRead() error
	CloseWrite() error
}

func copyTunnel(dst, src net.Conn, direction string, proxyCtx *goproxy.ProxyCtx) {
	_, err := io.Copy(dst, src)
	if err != nil {
		_ = dst.Close()
		_ = src.Close()
	} else if dstHalf, dstOK := dst.(halfClosableConn); dstOK {
		if srcHalf, srcOK := src.(halfClosableConn); srcOK {
			_ = dstHalf.CloseWrite()
			_ = srcHalf.CloseRead()
		} else {
			_ = dst.Close()
		}
	} else {
		_ = dst.Close()
	}

	if err != nil && !errors.Is(err, net.ErrClosed) && proxyCtx != nil {
		proxyCtx.Warnf("Cannot copy inspected CONNECT bytes to %s: %v", direction, err)
	}
}

type connectProbeResult struct {
	clientPrefix   []byte
	upstreamPrefix []byte
	serverName     string
	helloErr       error
	upstreamErr    error
	upstreamFirst  bool
}

func (r *connectProbeResult) decide(matcher *hostMatcher) connectDecision {
	switch {
	case len(r.upstreamPrefix) > 0:
		return connectDecision{route: connectRouteTunnel, reason: "upstream_first"}
	case r.upstreamFirst || (r.upstreamErr != nil && !isTimeoutError(r.upstreamErr)):
		return connectDecision{route: connectRouteTunnel, reason: "upstream_read_failed"}
	case r.helloErr != nil:
		return connectDecision{route: connectRouteTunnel, reason: clientHelloFailureReason(r.helloErr)}
	case r.serverName == "":
		return connectDecision{route: connectRouteTunnel, reason: "sni_missing"}
	case !matcher.Match(r.serverName):
		return connectDecision{route: connectRouteTunnel, reason: "sni_not_target"}
	default:
		return connectDecision{route: connectRouteMITM, reason: "sni_target"}
	}
}

type clientHelloProbeResult struct {
	prefix     []byte
	serverName string
	err        error
}

type prefixReadResult struct {
	prefix []byte
	err    error
}

// probeConnect races ClientHello inspection against server-first traffic and
// interrupts the losing read without closing either side of the tunnel.
func probeConnect(client, upstream net.Conn, deadline time.Time) connectProbeResult {
	_ = upstream.SetReadDeadline(deadline)

	clientResult := make(chan clientHelloProbeResult, 1)
	upstreamResult := make(chan prefixReadResult, 1)

	go func() {
		prefix, serverName, err := inspectClientHello(client)
		clientResult <- clientHelloProbeResult{prefix: prefix, serverName: serverName, err: err}
	}()
	go func() {
		var prefix [1]byte

		n, err := upstream.Read(prefix[:])
		upstreamResult <- prefixReadResult{prefix: append([]byte(nil), prefix[:n]...), err: err}
	}()

	var (
		clientProbe   clientHelloProbeResult
		upstreamProbe prefixReadResult
		upstreamFirst bool
	)
	select {
	case clientProbe = <-clientResult:
		_ = upstream.SetReadDeadline(time.Now())
		upstreamProbe = <-upstreamResult
	case upstreamProbe = <-upstreamResult:
		upstreamFirst = true
		_ = client.SetReadDeadline(time.Now())
		clientProbe = <-clientResult
	}

	_ = upstream.SetReadDeadline(time.Time{})

	return connectProbeResult{
		clientPrefix:   clientProbe.prefix,
		upstreamPrefix: upstreamProbe.prefix,
		serverName:     clientProbe.serverName,
		helloErr:       clientProbe.err,
		upstreamErr:    upstreamProbe.err,
		upstreamFirst:  upstreamFirst,
	}
}

// clientHelloProbeConn records bytes consumed by crypto/tls and suppresses any
// probe-generated alert, leaving fallback traffic untouched for replay.
type clientHelloProbeConn struct {
	net.Conn

	prefix []byte
}

func (c *clientHelloProbeConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.prefix = append(c.prefix, p[:n]...)

	return n, err
}

func (c *clientHelloProbeConn) Write(p []byte) (int, error) {
	return len(p), nil
}

func inspectClientHello(conn net.Conn) ([]byte, string, error) {
	probeConn := &clientHelloProbeConn{Conn: conn, prefix: make([]byte, 0, 4096)}
	serverName := ""
	captured := false
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			captured = true
			serverName = canonicalHostname(hello.ServerName)

			return nil, errClientHelloCaptured
		},
	}

	err := tls.Server(probeConn, config).HandshakeContext(context.Background())
	if captured {
		return probeConn.prefix, serverName, nil
	}

	return probeConn.prefix, "", err
}

func isTimeoutError(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func clientHelloFailureReason(err error) string {
	var recordHeaderError tls.RecordHeaderError
	if errors.As(err, &recordHeaderError) {
		return "not_tls_client_hello"
	}

	var networkError net.Error
	if errors.As(err, &networkError) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "client_hello_read_failed"
	}

	return "client_hello_invalid"
}

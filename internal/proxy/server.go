// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/elazarl/goproxy"
)

type captureState struct {
	session        int64
	started        time.Time
	requestMethod  string
	requestURL     *url.URL
	requestHeader  http.Header
	requestBody    bodySnapshot
	requestDecoded decodeResult
	requestRecord  *requestRecord
	requestOnce    sync.Once

	responseBody     bodySnapshot
	responseCaptured bool
	responseDeferred bool
	responseOnce     sync.Once
}

// Run starts the proxy and blocks until the context is canceled or the server fails.
func Run(ctx context.Context, rawConfig *Config) error {
	cfg, err := normalizeConfig(rawConfig)
	if err != nil {
		return fmt.Errorf("validate proxy config: %w", err)
	}

	matcher, err := newHostMatcher(cfg.Domains)
	if err != nil {
		return fmt.Errorf("create host matcher: %w", err)
	}

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.ListenAddr, err)
	}

	ca, generated, err := loadOrCreateCA(cfg.CACertPath, cfg.CAKeyPath, cfg.RequirePrivateCAPath)
	if err != nil {
		_ = listener.Close()
		return fmt.Errorf("load proxy CA: %w", err)
	}

	tracked := newTrackedListener(listener, defaultConnectHandshakeTimeout)

	recorder := newRecorder(cfg.Out, cfg.MaxBodyBytes, cfg.ShowSensitive)
	defer recorder.Close()

	proxyServer, transport := newProxyServer(&cfg, matcher, ca, recorder, tracked)
	httpServer := &http.Server{
		Handler:           proxyServer,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
		ErrorLog:          log.New(cfg.ErrOut, "proxy server: ", log.LstdFlags),
	}

	if err := printStartup(&cfg, tracked.Addr(), ca, generated); err != nil {
		return errors.Join(fmt.Errorf("print proxy startup: %w", err), tracked.Close())
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- httpServer.Serve(tracked)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		shutdownErr := httpServer.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			_ = httpServer.Close()
		}

		closeErr := tracked.closeAll()

		transport.CloseIdleConnections()

		if shutdownErr != nil {
			return fmt.Errorf("shutdown proxy: %w", shutdownErr)
		}

		if closeErr != nil {
			return fmt.Errorf("close proxy connections: %w", closeErr)
		}
		return nil
	case err := <-serveErr:
		_ = httpServer.Close()
		_ = tracked.closeAll()

		transport.CloseIdleConnections()

		if err == nil || errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return fmt.Errorf("serve proxy: %w", err)
	}
}

func newProxyServer(cfg *Config, matcher *hostMatcher, ca *tls.Certificate, recorder *recorder, tracked *trackedListener) (*goproxy.ProxyHttpServer, *http.Transport) {
	target := goproxy.ReqConditionFunc(func(req *http.Request, _ *goproxy.ProxyCtx) bool {
		if req == nil {
			return false
		}

		host := req.Host
		if req.URL != nil {
			switch {
			case req.URL.Host != "":
				host = req.URL.Host
			case req.URL.Opaque != "":
				host = req.URL.Opaque
			}
		}
		return matcher.Match(host)
	})
	tlsConfig := func(host string, proxyCtx *goproxy.ProxyCtx) (*tls.Config, error) {
		handshakeTimeout := defaultConnectHandshakeTimeout
		if tracked != nil && tracked.handshakeTimeout > 0 {
			handshakeTimeout = tracked.handshakeTimeout
		}

		config, err := goproxy.TLSConfigFromCA(ca)(host, proxyCtx)
		if err != nil {
			return nil, err
		}

		var observeClientHello func(*tls.ClientHelloInfo, *tls.Config)
		if cfg.Debug {
			observeClientHello = func(hello *tls.ClientHelloInfo, selected *tls.Config) {
				logMITMClientHello(proxyCtx, host, hello, selected)
			}
		}
		return withMITMHandshakeTimeout(config, handshakeTimeout, observeClientHello), nil
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{}
	}

	transport.Proxy = nil
	transport.TLSClientConfig = nil
	transport.DisableCompression = true

	server := goproxy.NewProxyHttpServer()
	server.Verbose = cfg.Debug
	server.Logger = log.New(&diagnosticWriter{out: cfg.ErrOut, showSensitive: cfg.ShowSensitive}, "goproxy: ", log.LstdFlags)
	server.KeepAcceptEncoding = true
	server.AllowHTTP2 = false
	server.Tr = transport
	server.ConnectDial = nil
	server.ConnectDialWithReq = nil
	server.CertStore = newMemoryCertStore()
	server.OnRequest().HandleConnectFunc(func(host string, proxyCtx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if matcher.Match(host) {
			if cfg.Debug && proxyCtx != nil {
				proxyCtx.Logf("[TLS_DIAGNOSTIC] phase=connect connect_target=%q connect_host=%q action=mitm", host, canonicalHostname(host))
			}

			if tracked != nil && proxyCtx != nil && proxyCtx.Req != nil {
				tracked.armHandshakeDeadline(proxyCtx.Req.RemoteAddr)
			}
			return &goproxy.ConnectAction{
				Action:    goproxy.ConnectMitm,
				TLSConfig: tlsConfig,
			}, host
		}

		if tracked != nil && proxyCtx != nil && proxyCtx.Req != nil {
			tracked.clearHandshakeDeadline(proxyCtx.Req.RemoteAddr)
		}
		return &goproxy.ConnectAction{Action: goproxy.ConnectAccept}, host
	})

	server.OnRequest(target).DoFunc(func(req *http.Request, proxyCtx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		state := &captureState{
			session:       proxyCtx.Session,
			started:       time.Now(),
			requestMethod: req.Method,
			requestURL:    cloneURL(req.URL),
			requestHeader: req.Header.Clone(),
		}
		prepareRequestCapture(state, req, cfg.MaxBodyBytes, recorder)
		proxyCtx.UserData = state

		proxyCtx.RoundTripper = goproxy.RoundTripperFunc(func(outbound *http.Request, _ *goproxy.ProxyCtx) (*http.Response, error) {
			response, roundTripErr := transport.RoundTrip(outbound)
			if roundTripErr != nil {
				if response != nil && response.Body != nil {
					_ = response.Body.Close()
				}

				state.responseOnce.Do(func() {
					recorder.recordResponseError(state, roundTripErr)
				})

				if outbound.URL != nil && strings.EqualFold(outbound.URL.Scheme, "https") {
					return goproxy.NewResponse(outbound, goproxy.ContentTypeText, http.StatusBadGateway, "Bad Gateway\n"), nil
				}
				return nil, roundTripErr
			}

			if response != nil {
				state.responseBody = newBodySnapshot(response.Header, response.ContentLength)
				state.responseCaptured = true
				hasBody := response.Body != nil && response.Body != http.NoBody

				omittedReason := bodyOmissionReason(state.responseBody.contentType, state.requestURL.Path)
				switch {
				case isUpgradeResponse(response):
					state.responseBody.omittedReason = "protocol upgrade body omitted"
				case hasBody && omittedReason != "":
					state.responseBody.omittedReason = omittedReason
				case hasBody:
					state.responseDeferred = true
					responseMetadata := cloneResponseMetadata(response)
					response.Body = newCaptureReadCloser(response.Body, &state.responseBody, cfg.MaxBodyBytes, func(snapshot bodySnapshot) {
						state.responseBody = snapshot
						state.responseOnce.Do(func() {
							recorder.recordResponse(state, responseMetadata)
						})
					})
				}
			}
			return response, nil
		})
		return req, nil
	})

	server.OnResponse().DoFunc(func(response *http.Response, proxyCtx *goproxy.ProxyCtx) *http.Response {
		state, ok := proxyCtx.UserData.(*captureState)
		if !ok || state == nil {
			return response
		}

		if state.responseDeferred {
			return response
		}

		state.responseOnce.Do(func() {
			if response == nil {
				recorder.recordResponseError(state, proxyCtx.Error)
				return
			}

			if !state.responseCaptured {
				state.responseBody = bodySnapshot{
					contentType:   response.Header.Get("Content-Type"),
					contentEncode: response.Header.Get("Content-Encoding"),
					contentLength: response.ContentLength,
					omittedReason: "response body unavailable",
				}
			}

			recorder.recordResponse(state, response)
		})
		return response
	})
	return server, transport
}

func prepareRequestCapture(state *captureState, request *http.Request, limit int64, recorder *recorder) {
	state.requestBody = newBodySnapshot(request.Header, request.ContentLength)
	state.requestRecord, state.requestDecoded = newRequestRecord(state.requestURL)
	finish := func(snapshot bodySnapshot) {
		state.requestBody = snapshot
		state.requestOnce.Do(func() {
			recorder.finishRequest(state.requestRecord, state)
		})
	}

	if request.Body == nil || request.Body == http.NoBody {
		finish(state.requestBody)
		return
	}

	if reason := bodyOmissionReason(state.requestBody.contentType, state.requestURL.Path); reason != "" {
		state.requestBody.omittedReason = reason
		finish(state.requestBody)
		return
	}

	if request.ContentLength < 0 {
		state.requestBody.omittedReason = "unknown-length request body omitted to avoid delaying streaming traffic"
		finish(state.requestBody)
		return
	}

	// Capture while the transport forwards the body; never pre-read client data.
	request.Body = newCaptureReadCloser(request.Body, &state.requestBody, limit, finish)
}

func withMITMHandshakeTimeout(config *tls.Config, timeout time.Duration, observeClientHello func(*tls.ClientHelloInfo, *tls.Config)) *tls.Config {
	if config == nil || timeout <= 0 {
		return config
	}

	base := config.Clone()
	getConfigForClient := config.GetConfigForClient
	base.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		_ = hello.Conn.SetDeadline(time.Now().Add(timeout))

		selected := config

		if getConfigForClient != nil {
			var err error

			selected, err = getConfigForClient(hello)
			if err != nil {
				return nil, err
			}

			if selected == nil {
				selected = config
			}
		}

		if observeClientHello != nil {
			observeClientHello(hello, selected)
		}

		selected = selected.Clone()
		selected.GetConfigForClient = nil
		verifyConnection := selected.VerifyConnection
		selected.VerifyConnection = func(state tls.ConnectionState) error {
			if verifyConnection != nil {
				if err := verifyConnection(state); err != nil {
					return err
				}
			}

			_ = hello.Conn.SetDeadline(time.Time{})
			return nil
		}
		return selected, nil
	}
	return base
}

func logMITMClientHello(proxyCtx *goproxy.ProxyCtx, connectTarget string, hello *tls.ClientHelloInfo, config *tls.Config) {
	if proxyCtx == nil || hello == nil {
		return
	}

	var (
		connectHost = canonicalHostname(connectTarget)
		sniHost     = canonicalHostname(hello.ServerName)
		sniRelation = "mismatch"
	)

	switch {
	case sniHost == "":
		sniRelation = "missing"
	case connectHost == sniHost:
		sniRelation = "match"
	}

	if config == nil || len(config.Certificates) == 0 || config.Certificates[0].Leaf == nil {
		proxyCtx.Logf("[TLS_DIAGNOSTIC] phase=client_hello connect_target=%q connect_host=%q sni=%q sni_relation=%s certificate_status=unavailable",
			connectTarget, connectHost, hello.ServerName, sniRelation)
		return
	}

	var (
		leaf   = config.Certificates[0].Leaf
		ipSANs = make([]string, len(leaf.IPAddresses))
	)
	for i, address := range leaf.IPAddresses {
		ipSANs[i] = address.String()
	}

	proxyCtx.Logf(
		"[TLS_DIAGNOSTIC] phase=client_hello connect_target=%q connect_host=%q sni=%q sni_relation=%s dns_sans=%q ip_sans=%q cert_matches_connect=%t cert_matches_sni=%t",
		connectTarget, connectHost, hello.ServerName, sniRelation, leaf.DNSNames, ipSANs, leaf.VerifyHostname(connectHost) == nil, sniHost != "" && leaf.VerifyHostname(sniHost) == nil,
	)
}

func isUpgradeResponse(response *http.Response) bool {
	if response == nil {
		return false
	}

	if response.StatusCode == http.StatusSwitchingProtocols {
		return true
	}

	for _, value := range response.Header.Values("Connection") {
		for part := range strings.SplitSeq(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), "upgrade") {
				return true
			}
		}
	}

	_, readWriter := response.Body.(io.ReadWriter)
	return readWriter
}

func cloneResponseMetadata(response *http.Response) *http.Response {
	return &http.Response{
		Status:           response.Status,
		StatusCode:       response.StatusCode,
		Proto:            response.Proto,
		ProtoMajor:       response.ProtoMajor,
		ProtoMinor:       response.ProtoMinor,
		Header:           response.Header.Clone(),
		ContentLength:    response.ContentLength,
		TransferEncoding: append([]string(nil), response.TransferEncoding...),
	}
}

func printStartup(cfg *Config, address net.Addr, ca *tls.Certificate, generated bool) error {
	state := "loaded"
	if generated {
		state = "generated"
	}

	lines := []string{
		fmt.Sprintf("ncmctl proxy listening on http://%s\n", address.String()),
		fmt.Sprintf("CA certificate (%s): %s\n", state, cfg.CACertPath),
		fmt.Sprintf("CA SHA-256: %s\n", formatFingerprint(ca.Leaf.Raw)),
		"Trust this CA on the client before capturing HTTPS. Press Ctrl+C to stop.\n",
	}

	if !isLoopbackListenAddress(cfg.ListenAddr) {
		lines = append(lines, "WARNING: this unauthenticated open proxy is reachable beyond this machine; use only on a trusted network behind a firewall.\n")
	}

	if cfg.ShowSensitive {
		lines = append(lines, "WARNING: sensitive output is enabled; credentials may appear in the terminal or redirected files.\n")
	}

	if _, err := io.WriteString(cfg.ErrOut, strings.Join(lines, "")); err != nil {
		return fmt.Errorf("write startup diagnostics: %w", err)
	}
	return nil
}

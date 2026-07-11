// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"errors"
	"net"
	"sync"
	"time"
)

const (
	defaultConnectHandshakeTimeout = 10 * time.Second
	tlsHandshakeRecordType         = byte(22)
	maxPlaintextConnectHeaderBytes = 1 << 20
)

type trackedListener struct {
	net.Listener
	mu               sync.Mutex
	conns            map[*trackedConn]struct{}
	handshakeTimeout time.Duration
}

type trackedConn struct {
	net.Conn
	once    sync.Once
	onClose func()

	mu                      sync.Mutex
	awaitingConnectPayload  bool
	handshakeDeadlineActive bool
	plaintextHeader         []byte
	plaintextPending        bool
}

func newTrackedListener(listener net.Listener) *trackedListener {
	return newTrackedListenerWithHandshakeTimeout(listener, defaultConnectHandshakeTimeout)
}

func newTrackedListenerWithHandshakeTimeout(listener net.Listener, handshakeTimeout time.Duration) *trackedListener {
	return &trackedListener{
		Listener:         listener,
		conns:            make(map[*trackedConn]struct{}),
		handshakeTimeout: handshakeTimeout,
	}
}

func (l *trackedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	tracked := &trackedConn{Conn: conn}
	tracked.onClose = func() {
		l.mu.Lock()
		delete(l.conns, tracked)
		l.mu.Unlock()
	}
	l.mu.Lock()
	l.conns[tracked] = struct{}{}
	l.mu.Unlock()
	return tracked, nil
}

func (l *trackedListener) closeAll() error {
	l.mu.Lock()
	connections := make([]*trackedConn, 0, len(l.conns))
	for conn := range l.conns {
		connections = append(connections, conn)
	}
	l.mu.Unlock()

	var errs []error
	for _, conn := range connections {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// armHandshakeDeadline is invoked after goproxy hijacks a target CONNECT but
// before it acknowledges the tunnel, so it also covers CONNECT following a
// normal request on the same keep-alive socket.
func (l *trackedListener) armHandshakeDeadline(remoteAddr string) {
	for _, conn := range l.connectionsForRemote(remoteAddr) {
		conn.armHandshakeDeadline(l.handshakeTimeout)
	}
}

func (l *trackedListener) clearHandshakeDeadline(remoteAddr string) {
	for _, conn := range l.connectionsForRemote(remoteAddr) {
		conn.clearHandshakeDeadline()
	}
}

func (l *trackedListener) connectionsForRemote(remoteAddr string) []*trackedConn {
	l.mu.Lock()
	connections := make([]*trackedConn, 0, 1)
	for conn := range l.conns {
		if conn.RemoteAddr().String() == remoteAddr {
			connections = append(connections, conn)
		}
	}
	l.mu.Unlock()
	return connections
}

func (c *trackedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.observeRead(p[:n])
	}
	return n, err
}

func (c *trackedConn) observeRead(data []byte) {
	clearDeadline := false
	c.mu.Lock()
	if c.awaitingConnectPayload && len(data) > 0 {
		c.awaitingConnectPayload = false
		if data[0] != tlsHandshakeRecordType {
			c.plaintextPending = true
		}
	}
	if c.plaintextPending {
		remaining := maxPlaintextConnectHeaderBytes - len(c.plaintextHeader)
		if remaining > 0 {
			if len(data) > remaining {
				data = data[:remaining]
			}
			c.plaintextHeader = append(c.plaintextHeader, data...)
			if bytes.Contains(c.plaintextHeader, []byte("\r\n\r\n")) {
				c.plaintextPending = false
				c.handshakeDeadlineActive = false
				c.plaintextHeader = nil
				clearDeadline = true
			}
		}
	}
	c.mu.Unlock()

	if clearDeadline {
		_ = c.Conn.SetDeadline(time.Time{})
	}
}
func (c *trackedConn) armHandshakeDeadline(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	deadline := time.Now().Add(timeout)
	_ = c.Conn.SetDeadline(deadline)
	c.mu.Lock()
	c.awaitingConnectPayload = true
	c.handshakeDeadlineActive = true
	c.plaintextHeader = nil
	c.plaintextPending = false
	c.mu.Unlock()
}

func (c *trackedConn) clearHandshakeDeadline() {
	c.mu.Lock()
	if !c.handshakeDeadlineActive {
		c.mu.Unlock()
		return
	}
	c.awaitingConnectPayload = false
	c.handshakeDeadlineActive = false
	c.plaintextHeader = nil
	c.plaintextPending = false
	c.mu.Unlock()
	_ = c.Conn.SetDeadline(time.Time{})
}

// CloseWrite and CloseRead preserve the half-close capability of accepted TCP
// connections so goproxy can keep forwarding the reverse direction after EOF.
func (c *trackedConn) CloseWrite() error {
	if conn, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return conn.CloseWrite()
	}
	return c.Close()
}

func (c *trackedConn) CloseRead() error {
	if conn, ok := c.Conn.(interface{ CloseRead() error }); ok {
		return conn.CloseRead()
	}
	return c.Close()
}

func (c *trackedConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.onClose)
	return err
}

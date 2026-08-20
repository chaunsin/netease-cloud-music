// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"errors"
	"net"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

const (
	defaultConnectHandshakeTimeout = 10 * time.Second
	tlsHandshakeRecordType         = byte(22)
	maxPlaintextConnectHeaderBytes = 1 << 20
	plaintextHeaderEnd             = "\r\n\r\n"
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
	handshakeDeadline       time.Time
	handshakeDeadlineActive bool
	handshakeGeneration     uint64
	plaintextHeader         []byte
	plaintextPending        bool
}

func newTrackedListener(listener net.Listener, handshakeTimeout time.Duration) *trackedListener {
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

// armHandshakeDeadline is invoked before goproxy acknowledges a target CONNECT,
// so it also covers CONNECT following a normal request on the same keep-alive socket.
func (l *trackedListener) armHandshakeDeadline(remoteAddr string) func() {
	type armedDeadline struct {
		conn       *trackedConn
		generation uint64
	}

	armed := make([]armedDeadline, 0, 1)
	connections := l.connectionsForRemote(remoteAddr)

	for _, conn := range connections {
		if generation := conn.armHandshakeDeadline(l.handshakeTimeout); generation != 0 {
			armed = append(armed, armedDeadline{conn: conn, generation: generation})
		}
	}

	var clearOnce sync.Once
	return func() {
		clearOnce.Do(func() {
			for _, deadline := range armed {
				deadline.conn.clearHandshakeDeadline(deadline.generation)
			}
		})
	}
}

func (l *trackedListener) clearHandshakeDeadline(remoteAddr string) {
	for _, conn := range l.connectionsForRemote(remoteAddr) {
		conn.clearHandshakeDeadline(0)
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

// SetDeadline preserves the CONNECT deadline when net/http clears deadlines
// during hijacking, until the tunnel handshake proves the connection is usable.
func (c *trackedConn) SetDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if deadline.IsZero() && c.handshakeDeadlineActive {
		deadline = c.handshakeDeadline
	}
	return c.Conn.SetDeadline(deadline)
}

func (c *trackedConn) SetReadDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if deadline.IsZero() && c.handshakeDeadlineActive {
		deadline = c.handshakeDeadline
	}
	return c.Conn.SetReadDeadline(deadline)
}

func (c *trackedConn) SetWriteDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if deadline.IsZero() && c.handshakeDeadlineActive {
		deadline = c.handshakeDeadline
	}
	return c.Conn.SetWriteDeadline(deadline)
}

func (c *trackedConn) observeRead(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.awaitingConnectPayload && len(data) > 0 {
		c.awaitingConnectPayload = false
		if data[0] != tlsHandshakeRecordType {
			c.plaintextPending = true
		}
	}

	if c.plaintextPending {
		remaining := maxPlaintextConnectHeaderBytes - len(c.plaintextHeader)
		if remaining > 0 {
			clientPreface := []byte(http2.ClientPreface)

			searchFrom := max(0, len(c.plaintextHeader)-len(plaintextHeaderEnd)+1)
			if len(c.plaintextHeader) < len(clientPreface) && bytes.Equal(c.plaintextHeader, clientPreface[:len(c.plaintextHeader)]) {
				searchFrom = 0
			}

			if len(data) > remaining {
				data = data[:remaining]
			}

			c.plaintextHeader = append(c.plaintextHeader, data...)
			if len(c.plaintextHeader) < len(clientPreface) && bytes.Equal(c.plaintextHeader, clientPreface[:len(c.plaintextHeader)]) {
				return
			}

			if bytes.HasPrefix(c.plaintextHeader, clientPreface) || bytes.Contains(c.plaintextHeader[searchFrom:], []byte(plaintextHeaderEnd)) {
				c.clearHandshakeDeadlineLocked()
			}
		}
	}
}

func (c *trackedConn) armHandshakeDeadline(timeout time.Duration) uint64 {
	if timeout <= 0 {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.handshakeGeneration++
	c.awaitingConnectPayload = true
	c.handshakeDeadline = time.Now().Add(timeout)
	c.handshakeDeadlineActive = true
	c.plaintextHeader = nil
	c.plaintextPending = false
	_ = c.Conn.SetDeadline(c.handshakeDeadline)
	return c.handshakeGeneration
}

func (c *trackedConn) clearHandshakeDeadline(generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.handshakeDeadlineActive || (generation != 0 && generation != c.handshakeGeneration) {
		return
	}

	c.clearHandshakeDeadlineLocked()
}

func (c *trackedConn) clearHandshakeDeadlineLocked() {
	c.awaitingConnectPayload = false
	c.handshakeDeadline = time.Time{}
	c.handshakeDeadlineActive = false
	c.plaintextHeader = nil
	c.plaintextPending = false
	_ = c.Conn.SetDeadline(time.Time{})
}

// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	ncmcrypto "github.com/chaunsin/netease-cloud-music/pkg/crypto"
)

const (
	XeapiSessionSourceStateFile   = "state-file"
	XeapiSessionSourceCommandLine = "command-line"

	xeapiSessionSourceResponseHeader = "response-header"
	xeapiSessionCapacity             = 256
	maxXeapiSessionIDBytes           = 1024
)

// XeapiSessionSeed supplies one startup session without granting the proxy
// permission to persist later sessions.
type XeapiSessionSeed struct {
	ID     string
	Key    string
	Source string
}

// Validate ensures a startup seed has a supported source and raw ASCII AES key.
func (seed XeapiSessionSeed) Validate() error {
	switch seed.Source {
	case XeapiSessionSourceStateFile, XeapiSessionSourceCommandLine:
	default:
		return errors.New("unknown source for XEAPI session")
	}

	if _, err := validatedXeapiSession(seed.ID, seed.Key); err != nil {
		return err
	}
	return nil
}

type xeapiSessionLookup interface {
	lookup(string) ([]byte, string, bool)
}

type xeapiSessionEntry struct {
	id     string
	key    []byte
	source string
}

type xeapiSessionSnapshot struct {
	entries map[string]xeapiSessionEntry
}

func (s *xeapiSessionSnapshot) lookup(id string) ([]byte, string, bool) {
	if s == nil {
		return nil, "", false
	}

	id, err := normalizedXeapiSessionID(id)
	if err != nil {
		return nil, "", false
	}

	entry, ok := s.entries[id]
	if !ok {
		return nil, "", false
	}
	return append([]byte(nil), entry.key...), entry.source, true
}

var emptyXeapiSessionSnapshot = &xeapiSessionSnapshot{}

// xeapiSessionCache evicts by update time, not lookup time. This keeps a burst
// of stale request lookups from displacing sessions learned more recently.
type xeapiSessionCache struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]*list.Element
	updates  *list.List
	view     *xeapiSessionSnapshot
}

func newXeapiSessionCache(seeds []XeapiSessionSeed) *xeapiSessionCache {
	cache := &xeapiSessionCache{
		capacity: xeapiSessionCapacity,
		entries:  make(map[string]*list.Element, min(len(seeds), xeapiSessionCapacity)),
		updates:  list.New(),
		view:     emptyXeapiSessionSnapshot,
	}
	for _, seed := range seeds {
		cache.store(seed.ID, []byte(seed.Key), seed.Source)
	}
	return cache
}

func (c *xeapiSessionCache) lookup(id string) ([]byte, string, bool) {
	if c == nil {
		return nil, "", false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	id, err := normalizedXeapiSessionID(id)
	if err != nil {
		return nil, "", false
	}

	element, ok := c.entries[id]
	if !ok {
		return nil, "", false
	}

	entry, ok := element.Value.(xeapiSessionEntry)
	if !ok {
		return nil, "", false
	}
	return append([]byte(nil), entry.key...), entry.source, true
}

func (c *xeapiSessionCache) snapshot() *xeapiSessionSnapshot {
	if c == nil {
		return emptyXeapiSessionSnapshot
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.view
}

func (c *xeapiSessionCache) learnResponseHeaders(header http.Header) error {
	if c == nil || header == nil {
		return nil
	}

	id := header.Get("X-Encr-Ssid")
	key := header.Get("X-Encr-Sskey")

	if id == "" && key == "" {
		return nil
	}

	if id == "" || key == "" {
		return errors.New("incomplete X-Encr-Ssid/X-Encr-Sskey header pair")
	}

	session, err := validatedXeapiSession(id, strings.TrimSpace(key))
	if err != nil {
		return fmt.Errorf("invalid XEAPI session response headers: %w", err)
	}

	c.store(session.ID, []byte(session.Key), xeapiSessionSourceResponseHeader)
	return nil
}

// store reports whether the published cache contents changed. An identical
// update still refreshes eviction order without rebuilding the snapshot.
func (c *xeapiSessionCache) store(id string, key []byte, source string) bool {
	if c == nil {
		return false
	}

	id, err := normalizedXeapiSessionID(id)
	if err != nil || !validXeapiSessionKeyLength(len(key)) {
		return false
	}

	for _, value := range key {
		if value > 0x7f {
			return false
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.entries[id]; ok {
		current, valid := existing.Value.(xeapiSessionEntry)
		c.updates.MoveToFront(existing)

		if valid && current.source == source && bytes.Equal(current.key, key) {
			return false
		}

		if valid {
			id = current.id
		} else {
			id = strings.Clone(id)
		}

		entry := xeapiSessionEntry{id: id, key: append([]byte(nil), key...), source: strings.Clone(source)}
		existing.Value = entry

		c.publishSnapshotLocked()
		return true
	}

	id = strings.Clone(id)
	entry := xeapiSessionEntry{id: id, key: append([]byte(nil), key...), source: strings.Clone(source)}
	element := c.updates.PushFront(entry)

	c.entries[id] = element
	if c.updates.Len() <= c.capacity {
		c.publishSnapshotLocked()
		return true
	}

	oldest := c.updates.Back()

	oldestEntry, ok := oldest.Value.(xeapiSessionEntry)
	if !ok {
		c.updates.Remove(oldest)
		c.publishSnapshotLocked()
		return true
	}

	delete(c.entries, oldestEntry.id)
	c.updates.Remove(oldest)
	c.publishSnapshotLocked()
	return true
}

func (c *xeapiSessionCache) publishSnapshotLocked() {
	entries := make(map[string]xeapiSessionEntry, len(c.entries))
	for id, element := range c.entries {
		entry, ok := element.Value.(xeapiSessionEntry)
		if !ok {
			continue
		}

		entry.key = append([]byte(nil), entry.key...)
		entries[id] = entry
	}

	c.view = &xeapiSessionSnapshot{entries: entries}
}

func validateXeapiSession(session ncmcrypto.XeapiSession) error {
	if err := session.Validate(); err != nil {
		return err
	}

	for i := range len(session.Key) {
		if session.Key[i] > 0x7f {
			return errors.New("xeapi session key must contain ASCII bytes only")
		}
	}
	return nil
}

func validatedXeapiSession(id, key string) (ncmcrypto.XeapiSession, error) {
	id, err := normalizedXeapiSessionID(id)
	if err != nil {
		return ncmcrypto.XeapiSession{}, err
	}

	session := ncmcrypto.XeapiSession{ID: id, Key: key}
	if err := validateXeapiSession(session); err != nil {
		return ncmcrypto.XeapiSession{}, err
	}
	return session, nil
}

func normalizedXeapiSessionID(id string) (string, error) {
	if len(id) > maxXeapiSessionIDBytes {
		return "", fmt.Errorf("xeapi session id exceeds %d bytes", maxXeapiSessionIDBytes)
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return "", ncmcrypto.ErrSessionIncomplete
	}
	return id, nil
}

func validXeapiSessionKeyLength(length int) bool {
	switch length {
	case 16, 24, 32:
		return true
	default:
		return false
	}
}

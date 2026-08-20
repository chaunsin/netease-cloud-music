// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXeapiSessionCachePrecedenceAndCoexistence(t *testing.T) {
	cache := newXeapiSessionCache([]XeapiSessionSeed{
		{ID: "same", Key: "state-file-key!!", Source: XeapiSessionSourceStateFile},
		{ID: "other", Key: "other-state-key!", Source: XeapiSessionSourceStateFile},
		{ID: "same", Key: "command-line-key", Source: XeapiSessionSourceCommandLine},
	})

	key, source, ok := cache.lookup("same")
	require.True(t, ok)
	assert.Equal(t, []byte("command-line-key"), key)
	assert.Equal(t, XeapiSessionSourceCommandLine, source)

	key[0] = 'X'
	stored, _, ok := cache.lookup("same")
	require.True(t, ok)
	assert.Equal(t, []byte("command-line-key"), stored, "lookup must return a key copy")

	_, source, ok = cache.lookup("other")
	require.True(t, ok)
	assert.Equal(t, XeapiSessionSourceStateFile, source)

	require.NoError(t, cache.learnResponseHeaders(http.Header{
		"X-Encr-Ssid":  {"same"},
		"X-Encr-Sskey": {"runtime-key-1234"},
	}))
	key, source, ok = cache.lookup("same")
	require.True(t, ok)
	assert.Equal(t, []byte("runtime-key-1234"), key)
	assert.Equal(t, xeapiSessionSourceResponseHeader, source)
}

func TestXeapiSessionCacheRejectsInvalidHeadersWithoutOverwrite(t *testing.T) {
	cache := newXeapiSessionCache([]XeapiSessionSeed{{
		ID: "session", Key: "valid-key-123456", Source: XeapiSessionSourceCommandLine,
	}})

	tests := []http.Header{
		{"X-Encr-Ssid": {"session"}},
		{"X-Encr-Sskey": {"valid-key-123456"}},
		{"X-Encr-Ssid": {"session"}, "X-Encr-Sskey": {"short"}},
		{"X-Encr-Ssid": {""}, "X-Encr-Sskey": {"valid-key-123456"}},
	}
	for _, header := range tests {
		err := cache.learnResponseHeaders(header)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "valid-key-123456")

		key, source, ok := cache.lookup("session")
		require.True(t, ok)
		assert.Equal(t, []byte("valid-key-123456"), key)
		assert.Equal(t, XeapiSessionSourceCommandLine, source)
	}

	require.NoError(t, cache.learnResponseHeaders(nil))
	require.NoError(t, cache.learnResponseHeaders(http.Header{"Other": {"value"}}))
}

func TestXeapiSessionCacheSessionIDBoundary(t *testing.T) {
	const key = "0123456789abcdef"

	id := strings.Repeat("s", maxXeapiSessionIDBytes)
	cache := newXeapiSessionCache(nil)
	require.NoError(t, cache.learnResponseHeaders(http.Header{
		"X-Encr-Ssid":  {id},
		"X-Encr-Sskey": {key},
	}))

	storedKey, source, ok := cache.lookup(id)
	require.True(t, ok)
	assert.Equal(t, []byte(key), storedKey)
	assert.Equal(t, xeapiSessionSourceResponseHeader, source)

	view := cache.snapshot()
	oversizedID := id + " "
	err := cache.learnResponseHeaders(http.Header{
		"X-Encr-Ssid":  {oversizedID},
		"X-Encr-Sskey": {"fedcba9876543210"},
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), oversizedID)
	assert.NotContains(t, err.Error(), "fedcba9876543210")
	assert.Same(t, view, cache.snapshot(), "a rejected update must not publish a snapshot")

	storedKey, source, ok = cache.lookup(id)
	require.True(t, ok)
	assert.Equal(t, []byte(key), storedKey)
	assert.Equal(t, xeapiSessionSourceResponseHeader, source)
}

func TestXeapiSessionCacheStoresCanonicalSessionID(t *testing.T) {
	cache := newXeapiSessionCache(nil)
	require.NoError(t, cache.learnResponseHeaders(http.Header{
		"X-Encr-Ssid":  {" \tcanonical-session\n"},
		"X-Encr-Sskey": {"0123456789abcdef"},
	}))

	view := cache.snapshot()
	require.Contains(t, view.entries, "canonical-session")
	assert.NotContains(t, view.entries, " \tcanonical-session\n")
}

func TestXeapiSessionCacheIdenticalUpdateReusesSnapshotAndRefreshesEviction(t *testing.T) {
	cache := newXeapiSessionCache(nil)
	cache.capacity = 2
	key := []byte("0123456789abcdef")

	require.True(t, cache.store("first", key, "test"))
	require.True(t, cache.store("second", key, "test"))
	view := cache.snapshot()

	assert.False(t, cache.store("first", key, "test"))
	assert.Same(t, view, cache.snapshot(), "an identical update must not rebuild the snapshot")
	require.True(t, cache.store("third", key, "test"))

	_, _, ok := cache.lookup("second")
	assert.False(t, ok, "the identical update must still refresh eviction order")
	_, _, ok = cache.lookup("first")
	assert.True(t, ok)

	view = cache.snapshot()
	require.True(t, cache.store("first", key, "updated"))
	assert.NotSame(t, view, cache.snapshot(), "a source change must publish a snapshot")
	_, source, ok := cache.lookup("first")
	require.True(t, ok)
	assert.Equal(t, "updated", source)

	view = cache.snapshot()
	replacementKey := []byte("fedcba9876543210")
	require.True(t, cache.store("first", replacementKey, "updated"))
	assert.NotSame(t, view, cache.snapshot(), "a key change must publish a snapshot")
	storedKey, _, ok := cache.lookup("first")
	require.True(t, ok)
	assert.Equal(t, replacementKey, storedKey)
}

func TestXeapiSessionCacheStoreRejectsInvalidData(t *testing.T) {
	cache := newXeapiSessionCache(nil)
	view := cache.snapshot()

	tests := []struct {
		name string
		id   string
		key  []byte
	}{
		{name: "empty id", key: []byte("0123456789abcdef")},
		{name: "oversized id", id: strings.Repeat("s", maxXeapiSessionIDBytes+1), key: []byte("0123456789abcdef")},
		{name: "invalid key length", id: "session", key: []byte("short")},
		{name: "non ASCII key", id: "session", key: []byte("0123456789abcde\xff")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.False(t, cache.store(test.id, test.key, "test"))
			assert.Same(t, view, cache.snapshot())
		})
	}
}

func TestXeapiSessionCacheEvictsLeastRecentlyUpdated(t *testing.T) {
	cache := newXeapiSessionCache(nil)

	key := []byte("0123456789abcdef")
	for i := range xeapiSessionCapacity {
		cache.store(fmt.Sprintf("session-%03d", i), key, "test")
	}

	cache.store("session-000", key, "updated")
	cache.store("session-new", key, "test")

	_, _, ok := cache.lookup("session-001")
	assert.False(t, ok)
	_, source, ok := cache.lookup("session-000")
	require.True(t, ok)
	assert.Equal(t, "updated", source)

	_, _, ok = cache.lookup("session-new")
	assert.True(t, ok)

	_, _, ok = cache.lookup("session-002")
	require.True(t, ok)
	cache.store("session-newer", key, "test")
	_, _, ok = cache.lookup("session-002")
	assert.False(t, ok, "lookup must not refresh update-based eviction order")
}

func TestXeapiSessionCacheConcurrentAccess(t *testing.T) {
	cache := newXeapiSessionCache(nil)

	const workers = 32

	var wait sync.WaitGroup
	wait.Add(workers)

	for worker := range workers {
		go func(worker int) {
			defer wait.Done()

			for i := range 100 {
				id := fmt.Sprintf("session-%d-%d", worker, i%8)
				cache.store(id, []byte("0123456789abcdef"), "concurrent")
				_, _, _ = cache.lookup(id)
			}
		}(worker)
	}

	wait.Wait()
	assertXeapiSessionCacheBounds(t, cache)
}

func TestXeapiSessionCacheBoundsSessionIDStorage(t *testing.T) {
	cache := newXeapiSessionCache(nil)
	key := []byte("0123456789abcdef")

	for i := range xeapiSessionCapacity + 32 {
		prefix := fmt.Sprintf("%03d-", i)
		id := prefix + strings.Repeat("s", maxXeapiSessionIDBytes-len(prefix))
		require.True(t, cache.store(id, key, "test"))
	}

	view := cache.snapshot()
	require.Len(t, view.entries, xeapiSessionCapacity)

	totalIDBytes := 0

	for id := range view.entries {
		assert.LessOrEqual(t, len(id), maxXeapiSessionIDBytes)
		totalIDBytes += len(id)
	}

	assert.LessOrEqual(t, totalIDBytes, xeapiSessionCapacity*maxXeapiSessionIDBytes)
	assertXeapiSessionCacheBounds(t, cache)
}

func TestXeapiSessionSnapshotDoesNotLearnRetroactively(t *testing.T) {
	cache := newXeapiSessionCache([]XeapiSessionSeed{{
		ID: "existing", Key: "old-key-12345678", Source: XeapiSessionSourceStateFile,
	}})
	snapshot := cache.snapshot()

	require.NoError(t, cache.learnResponseHeaders(http.Header{
		"X-Encr-Ssid":  {"existing"},
		"X-Encr-Sskey": {"new-key-12345678"},
	}))
	require.NoError(t, cache.learnResponseHeaders(http.Header{
		"X-Encr-Ssid":  {"later"},
		"X-Encr-Sskey": {"later-key-123456"},
	}))

	key, source, ok := snapshot.lookup("existing")
	require.True(t, ok)
	assert.Equal(t, []byte("old-key-12345678"), key)
	assert.Equal(t, XeapiSessionSourceStateFile, source)

	_, _, ok = snapshot.lookup("later")
	assert.False(t, ok, "a response-learned key must apply only to later requests")
}

func TestXeapiSessionSeedValidate(t *testing.T) {
	require.NoError(t, (XeapiSessionSeed{
		ID: "session", Key: "0123456789abcdef", Source: XeapiSessionSourceStateFile,
	}).Validate())
	require.ErrorContains(t, (XeapiSessionSeed{
		ID: "session", Key: "short", Source: XeapiSessionSourceStateFile,
	}).Validate(), "length")
	require.ErrorContains(t, (XeapiSessionSeed{
		ID: "session", Key: "0123456789abcdef",
	}).Validate(), "unknown source")
}

func TestXeapiSessionSeedValidateSessionIDBoundary(t *testing.T) {
	const key = "0123456789abcdef"

	id := strings.Repeat("s", maxXeapiSessionIDBytes)
	require.NoError(t, (XeapiSessionSeed{
		ID: id, Key: key, Source: XeapiSessionSourceStateFile,
	}).Validate())

	oversizedID := id + " "
	err := (XeapiSessionSeed{
		ID: oversizedID, Key: key, Source: XeapiSessionSourceStateFile,
	}).Validate()
	require.Error(t, err)
	assert.NotContains(t, err.Error(), oversizedID)
	assert.NotContains(t, err.Error(), key)
}

func assertXeapiSessionCacheBounds(t *testing.T, cache *xeapiSessionCache) {
	t.Helper()

	cache.mu.Lock()
	defer cache.mu.Unlock()

	assert.LessOrEqual(t, cache.updates.Len(), cache.capacity)
	assert.Len(t, cache.entries, cache.updates.Len())
	assert.Len(t, cache.view.entries, len(cache.entries))

	totalIDBytes := 0

	for id, element := range cache.entries {
		assert.LessOrEqual(t, len(id), maxXeapiSessionIDBytes)
		totalIDBytes += len(id)

		entry, ok := element.Value.(xeapiSessionEntry)
		if assert.True(t, ok) {
			assert.Equal(t, id, entry.id)
			assert.True(t, validXeapiSessionKeyLength(len(entry.key)))
		}
	}

	assert.LessOrEqual(t, totalIDBytes, cache.capacity*maxXeapiSessionIDBytes)
}

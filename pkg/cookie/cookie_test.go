// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package cookie

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/publicsuffix"
)

func TestPersistentCookieRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookie.json")
	jar, err := NewCookie(WithSyncInterval(0), WithFilePath(path))
	require.NoError(t, err)

	u := mustCookieURL(t, "https://www.example.com/a/item")
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	jar.SetCookies(u, []*http.Cookie{
		{
			Name:     "session",
			Value:    "quoted",
			Quoted:   true,
			Domain:   ".example.com",
			Path:     "/a",
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
		},
		{
			Name:    "persistent",
			Value:   "value",
			Domain:  ".example.com",
			Path:    "/a",
			Expires: expires,
		},
	})
	require.NoError(t, jar.Close(context.Background()))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"Quoted":true`)
	assert.NotContains(t, string(data), `"Quoted":false`)
	assertCookieFileMode(t, path)
	assertNoTemporaryCookieFiles(t, filepath.Dir(path))

	reloaded, err := NewCookie(WithSyncInterval(0), WithFilePath(path))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reloaded.Close(context.Background()))
	})

	got := reloaded.Cookies(mustCookieURL(t, "https://sub.example.com/a/item"))
	require.Len(t, got, 2)
	assert.Equal(t, "session", got[0].Name)
	assert.True(t, got[0].Quoted)
	assert.Equal(t, "persistent", got[1].Name)
	assert.False(t, got[1].Quoted)

	session := findRuntimeEntry(t, reloaded, "session")
	assert.False(t, session.Persistent)
	assert.Equal(t, "SameSite=None", session.SameSite)
	assert.True(t, session.Secure)
	assert.True(t, session.HttpOnly)

	persistent := findRuntimeEntry(t, reloaded, "persistent")
	assert.True(t, persistent.Persistent)
	assert.WithinDuration(t, expires, persistent.Expires, time.Second)
}

func TestNewCookieUsesPublicSuffixListByDefault(t *testing.T) {
	jar, err := NewCookie(
		WithSyncInterval(0),
		WithFilePath(filepath.Join(t.TempDir(), "default.json")),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, jar.Close(context.Background()))
	})

	u := mustCookieURL(t, "https://foo.co.uk/")
	jar.SetCookies(u, []*http.Cookie{{Name: "public", Value: "value", Domain: "co.uk"}})
	assert.Empty(t, jar.Cookies(u))

	withoutList, err := NewCookie(
		WithSyncInterval(0),
		WithFilePath(filepath.Join(t.TempDir(), "nil.json")),
		WithPublicSuffixList(nil),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, withoutList.Close(context.Background()))
	})

	withoutList.SetCookies(u, []*http.Cookie{{Name: "public", Value: "value", Domain: "co.uk"}})
	assert.Len(t, withoutList.Cookies(mustCookieURL(t, "https://bar.co.uk/")), 1)
}

func TestNewCookieCreatesPrivateParentDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state", "cookie")
	path := filepath.Join(dir, "cookie.json")
	jar, err := NewCookie(WithSyncInterval(0), WithFilePath(path))
	require.NoError(t, err)

	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(dir)
		require.NoError(t, statErr)
		assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	}

	require.NoError(t, jar.Close(context.Background()))
	assertCookieFileMode(t, path)
}

func TestNewCookieRejectsInvalidOptions(t *testing.T) {
	_, err := NewCookie(WithFilePath(""))
	require.ErrorContains(t, err, "cookie filepath is empty")

	_, err = NewCookie(nil)
	require.ErrorContains(t, err, "cookie option is nil")
}

func TestPersistentCookieRejectsInvalidEntriesWithoutChangingFile(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	tests := map[string]func(entry) (string, string, entry){
		"bucket": func(e entry) (string, string, entry) {
			id := e.id()

			return "wrong.example", id, e
		},
		"entry id": func(e entry) (string, string, entry) {
			return jarKey(e.Domain, publicsuffix.List), "wrong-id", e
		},
		"cookie name": func(e entry) (string, string, entry) {
			e.Name = "invalid name"
			id := e.id()

			return jarKey(e.Domain, publicsuffix.List), id, e
		},
		"cookie value": func(e entry) (string, string, entry) {
			e.Value += ";private"
			id := e.id()

			return jarKey(e.Domain, publicsuffix.List), id, e
		},
		"canonical domain": func(e entry) (string, string, entry) {
			e.Domain = "EXAMPLE.COM"
			id := e.id()

			return jarKey(e.Domain, publicsuffix.List), id, e
		},
		"absolute path": func(e entry) (string, string, entry) {
			e.Path = "relative"
			id := e.id()

			return jarKey(e.Domain, publicsuffix.List), id, e
		},
		"IP host-only": func(e entry) (string, string, entry) {
			e.Domain = "127.0.0.1"
			e.HostOnly = false
			id := e.id()

			return jarKey(e.Domain, publicsuffix.List), id, e
		},
		"public suffix": func(e entry) (string, string, entry) {
			e.Domain = "co.uk"
			e.HostOnly = false
			id := e.id()

			return jarKey(e.Domain, publicsuffix.List), id, e
		},
		"session expiration": func(e entry) (string, string, entry) {
			e.Expires = now.Add(time.Hour)
			id := e.id()

			return jarKey(e.Domain, publicsuffix.List), id, e
		},
		"creation time": func(e entry) (string, string, entry) {
			e.Creation = time.Time{}
			id := e.id()

			return jarKey(e.Domain, publicsuffix.List), id, e
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cookie.json")
			e := validRuntimeEntry("token", 0, now)
			e.Value = "top-secret-value"
			bucket, id, e := mutate(e)
			content := map[string]map[string]Entry{
				bucket: {id: e.persistedEntry()},
			}
			original := writePersistedEntries(t, path, content)

			_, err := NewCookie(WithFilePath(path))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), e.Value)

			after, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, original, after)
		})
	}
}

func TestPersistentCookieFiltersExpiryAndRepairsSequenceNumbers(t *testing.T) {
	now := time.Now().UTC()
	first := validRuntimeEntry("first", 0, now.Add(-3*time.Hour))
	second := validRuntimeEntry("second", 0, now.Add(-2*time.Hour))
	expired := validRuntimeEntry("expired", 99, now.Add(-time.Hour))
	expired.Persistent = true
	expired.Expires = now.Add(-time.Minute)

	path := filepath.Join(t.TempDir(), "cookie.json")
	writePersistedEntries(t, path, persistedEntries(first, second, expired))

	jar, err := NewCookie(
		WithSyncInterval(time.Hour),
		WithFilePath(path),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, jar.Close(context.Background()))
	})

	assert.Equal(t, uint64(2), jar.jar.nextSeqNum)
	assert.Equal(t, uint64(0), findRuntimeEntry(t, jar, "first").seqNum)
	assert.Equal(t, uint64(1), findRuntimeEntry(t, jar, "second").seqNum)
	assertRuntimeEntryMissing(t, jar, "expired")

	jar.jar.SetCookies(
		mustCookieURL(t, "https://example.com/"),
		[]*http.Cookie{{Name: "new", Value: "value"}},
	)
	assert.Equal(t, uint64(2), findRuntimeEntry(t, jar, "new").seqNum)
}

func TestPersistentCookieRestoresNextSequenceAfterZero(t *testing.T) {
	now := time.Now().UTC()
	only := validRuntimeEntry("only", 0, now)
	path := filepath.Join(t.TempDir(), "cookie.json")
	writePersistedEntries(t, path, persistedEntries(only))

	jar, err := NewCookie(WithSyncInterval(0), WithFilePath(path))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, jar.Close(context.Background()))
	})

	assert.Equal(t, uint64(1), jar.jar.nextSeqNum)
}

func TestPersistentCookieMigratesLegacyPublicSuffixBucket(t *testing.T) {
	now := time.Now().UTC()
	e := validRuntimeEntry("legacy", 0, now)
	e.Domain = "foo.example.co.uk"
	id := e.id()
	legacyBucket := jarKey(e.Domain, nil)
	targetBucket := jarKey(e.Domain, publicsuffix.List)
	require.NotEqual(t, legacyBucket, targetBucket)

	path := filepath.Join(t.TempDir(), "cookie.json")
	original := writePersistedEntries(t, path, map[string]map[string]Entry{
		legacyBucket: {id: e.persistedEntry()},
	})
	assert.NotContains(t, string(original), `"Quoted"`)

	jar, err := NewCookie(WithSyncInterval(0), WithFilePath(path))
	require.NoError(t, err)

	jar.jar.mu.Lock()
	_, hasLegacyBucket := jar.jar.entries[legacyBucket]
	_, hasTargetBucket := jar.jar.entries[targetBucket]
	jar.jar.mu.Unlock()
	assert.False(t, hasLegacyBucket)
	assert.True(t, hasTargetBucket)

	got := jar.Cookies(mustCookieURL(t, "https://foo.example.co.uk/"))
	require.Len(t, got, 1)
	assert.Equal(t, "legacy", got[0].Name)
	assert.False(t, got[0].Quoted)
	require.NoError(t, jar.Close(context.Background()))

	var exported map[string]map[string]Entry

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &exported))
	assert.NotContains(t, exported, legacyBucket)
	require.Contains(t, exported, targetBucket)
	assert.Contains(t, exported[targetBucket], id)
}

func TestPersistentCookieRejectsLegacyBucketMigrationCollision(t *testing.T) {
	now := time.Now().UTC()
	legacy := validRuntimeEntry("duplicate", 0, now)
	legacy.Domain = "foo.example.co.uk"
	current := legacy
	current.Value = "different-value"

	legacyBucket := jarKey(legacy.Domain, nil)
	targetBucket := jarKey(legacy.Domain, publicsuffix.List)
	id := legacy.id()
	path := filepath.Join(t.TempDir(), "cookie.json")
	original := writePersistedEntries(t, path, map[string]map[string]Entry{
		legacyBucket: {id: legacy.persistedEntry()},
		targetBucket: {id: current.persistedEntry()},
	})

	_, err := NewCookie(WithFilePath(path))
	require.ErrorContains(t, err, "duplicate entry after bucket migration")
	assert.NotContains(t, err.Error(), legacy.Value)
	assert.NotContains(t, err.Error(), current.Value)

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, original, after)
}

func TestWriteCookieFileReplacesAndRestrictsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookie.json")
	// The replacement must repair an existing overly broad mode.
	//nolint:gosec // Deliberately create the insecure mode under test.
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o644))

	require.NoError(t, writeCookieFile(path, []byte("new")))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), got)
	assertCookieFileMode(t, path)
	assertNoTemporaryCookieFiles(t, dir)
}

func TestWriteCookieFileReplaceFailurePreservesTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookie.json")
	require.NoError(t, os.Mkdir(path, 0o700))
	marker := filepath.Join(path, "old")
	require.NoError(t, os.WriteFile(marker, []byte("old"), 0o600))

	err := writeCookieFile(path, []byte("new"))
	require.ErrorContains(t, err, "replace cookie file")

	got, readErr := os.ReadFile(marker)
	require.NoError(t, readErr)
	assert.Equal(t, []byte("old"), got)
	assertNoTemporaryCookieFiles(t, dir)
}

func TestCookieCloseReturnsFinalExportError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "state")
	jar, err := NewCookie(
		WithSyncInterval(0),
		WithFilePath(filepath.Join(dir, "cookie.json")),
	)
	require.NoError(t, err)

	require.NoError(t, os.Remove(dir))
	require.NoError(t, os.WriteFile(dir, []byte("blocks directory creation"), 0o600))

	firstErr := jar.Close(context.Background())
	require.Error(t, firstErr)
	require.ErrorContains(t, firstErr, "create cookie directory")

	secondErr := jar.Close(context.Background())
	assert.EqualError(t, secondErr, firstErr.Error())
}

func TestCookieCloseContinuesAfterContextCancellation(t *testing.T) {
	jar, err := NewCookie(
		WithSyncInterval(0),
		WithFilePath(filepath.Join(t.TempDir(), "cookie.json")),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	closeErr := jar.Close(ctx)
	require.ErrorIs(t, closeErr, context.Canceled)

	require.NoError(t, jar.Close(context.Background()))
	require.ErrorIs(t, jar.Close(ctx), context.Canceled)

	select {
	case <-jar.syncDone:
	default:
		t.Fatal("sync goroutine is still running after Close")
	}
}

func TestCookieCloseWaitsForInFlightSetCookies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookie.json")
	psl := &blockingPublicSuffixList{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	jar, err := NewCookie(
		WithSyncInterval(time.Hour),
		WithFilePath(path),
		WithPublicSuffixList(psl),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		psl.unblock()
		require.NoError(t, jar.Close(context.Background()))
	})

	u := mustCookieURL(t, "https://www.example.com/")
	setDone := make(chan struct{})

	go func() {
		jar.SetCookies(u, []*http.Cookie{{Name: "in-flight", Value: "value"}})
		close(setDone)
	}()

	waitForSignal(t, psl.entered, "SetCookies to reach the public suffix list")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, jar.Close(ctx), context.Canceled)
	waitForSignal(t, jar.done, "shutdown to start")

	select {
	case <-jar.closed:
		t.Fatal("Close completed while SetCookies was still in flight")
	default:
	}

	psl.unblock()
	waitForSignal(t, setDone, "SetCookies to finish")
	require.NoError(t, jar.Close(context.Background()))

	reloaded, err := NewCookie(WithSyncInterval(0), WithFilePath(path))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reloaded.Close(context.Background()))
	})

	got := reloaded.Cookies(u)
	require.Len(t, got, 1)
	assert.Equal(t, "in-flight", got[0].Name)
}

func TestCookieSetCookiesAfterCloseIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookie.json")
	jar, err := NewCookie(WithSyncInterval(time.Hour), WithFilePath(path))
	require.NoError(t, err)
	require.NoError(t, jar.Close(context.Background()))

	u := mustCookieURL(t, "https://example.com/")
	jar.SetCookies(u, []*http.Cookie{{Name: "late", Value: "value"}})
	assert.Empty(t, jar.Cookies(u))
}

func TestCookieConcurrentClose(t *testing.T) {
	jar, err := NewCookie(
		WithSyncInterval(time.Hour),
		WithFilePath(filepath.Join(t.TempDir(), "cookie.json")),
	)
	require.NoError(t, err)

	const callers = 8

	var (
		start = make(chan struct{})
		errs  = make(chan error, callers)
	)

	for range callers {
		go func() {
			<-start

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			errs <- jar.Close(ctx)
		}()
	}

	close(start)

	for range callers {
		require.NoError(t, <-errs)
	}
}

func validRuntimeEntry(name string, sequence uint64, creation time.Time) entry {
	return entry{
		Name:       name,
		Value:      "value",
		Domain:     "example.com",
		Path:       "/",
		HostOnly:   true,
		Expires:    endOfTime,
		Creation:   creation,
		LastAccess: creation,
		seqNum:     sequence,
	}
}

type blockingPublicSuffixList struct {
	enterOnce   sync.Once
	releaseOnce sync.Once
	entered     chan struct{}
	release     chan struct{}
}

func (p *blockingPublicSuffixList) PublicSuffix(string) string {
	p.enterOnce.Do(func() {
		close(p.entered)
	})
	<-p.release

	return "com"
}

func (*blockingPublicSuffixList) String() string {
	return "blocking test list"
}

func (p *blockingPublicSuffixList) unblock() {
	p.releaseOnce.Do(func() {
		close(p.release)
	})
}

func waitForSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-signal:
	case <-timer.C:
		t.Fatalf("timed out waiting for %s", description)
	}
}

func persistedEntries(entries ...entry) map[string]map[string]Entry {
	content := make(map[string]map[string]Entry)

	for i := range entries {
		e := &entries[i]

		bucket := jarKey(e.Domain, publicsuffix.List)
		if content[bucket] == nil {
			content[bucket] = make(map[string]Entry)
		}

		content[bucket][e.id()] = e.persistedEntry()
	}

	return content
}

func writePersistedEntries(t *testing.T, path string, content map[string]map[string]Entry) []byte {
	t.Helper()

	data, err := json.Marshal(content)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))

	return data
}

func findRuntimeEntry(t *testing.T, jar *Cookie, name string) entry {
	t.Helper()

	jar.jar.mu.Lock()
	defer jar.jar.mu.Unlock()

	for _, entries := range jar.jar.entries {
		for id := range entries {
			e := entries[id]
			if e.Name == name {
				return e
			}
		}
	}

	t.Fatalf("cookie entry %q not found", name)

	return entry{}
}

func assertRuntimeEntryMissing(t *testing.T, jar *Cookie, name string) {
	t.Helper()

	jar.jar.mu.Lock()
	defer jar.jar.mu.Unlock()

	for _, entries := range jar.jar.entries {
		for id := range entries {
			e := entries[id]
			assert.NotEqual(t, name, e.Name)
		}
	}
}

func assertCookieFileMode(t *testing.T, path string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		return
	}

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func assertNoTemporaryCookieFiles(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".cookie.json-"), e.Name())
	}
}

func TestEntryUnmarshalRequiresLegacyFields(t *testing.T) {
	required := []string{
		"Name",
		"Value",
		"Domain",
		"Path",
		"SameSite",
		"Secure",
		"HttpOnly",
		"Persistent",
		"HostOnly",
		"Expires",
		"Creation",
		"LastAccess",
		"SeqNum",
	}

	for _, field := range required {
		t.Run(field+" missing", func(t *testing.T) {
			fields := validEntryJSONFields()
			delete(fields, field)

			var entry Entry

			err := json.Unmarshal(marshalEntryJSON(t, fields), &entry)
			require.ErrorContains(t, err, field)
			assert.NotContains(t, err.Error(), "top-secret-value")
		})

		t.Run(field+" null", func(t *testing.T) {
			fields := validEntryJSONFields()
			fields[field] = nil

			var entry Entry

			err := json.Unmarshal(marshalEntryJSON(t, fields), &entry)
			require.ErrorContains(t, err, field)
			assert.NotContains(t, err.Error(), "top-secret-value")
		})
	}
}

func TestEntryUnmarshalRejectsMisspelledScopeFields(t *testing.T) {
	for _, field := range []string{"Secure", "HostOnly"} {
		t.Run(field, func(t *testing.T) {
			fields := validEntryJSONFields()
			delete(fields, field)
			fields[field+"Typo"] = true

			var entry Entry

			err := json.Unmarshal(marshalEntryJSON(t, fields), &entry)
			require.ErrorContains(t, err, field)
			assert.NotContains(t, err.Error(), "top-secret-value")
		})
	}
}

func TestEntryUnmarshalAcceptsExplicitZeroValues(t *testing.T) {
	fields := validEntryJSONFields()
	fields["Name"] = ""
	fields["Value"] = ""
	fields["SameSite"] = ""
	fields["Secure"] = false
	fields["HttpOnly"] = false
	fields["Persistent"] = false
	fields["HostOnly"] = false
	fields["SeqNum"] = uint64(0)

	var entry Entry
	require.NoError(t, json.Unmarshal(marshalEntryJSON(t, fields), &entry))
	assert.Empty(t, entry.Name)
	assert.Empty(t, entry.Value)
	assert.Empty(t, entry.SameSite)
	assert.False(t, entry.Secure)
	assert.False(t, entry.HttpOnly)
	assert.False(t, entry.Persistent)
	assert.False(t, entry.HostOnly)
	assert.Zero(t, entry.SeqNum)
}

func TestEntryUnmarshalAllowsMissingQuotedForOldFormat(t *testing.T) {
	fields := validEntryJSONFields()
	delete(fields, "Quoted")

	var entry Entry
	require.NoError(t, json.Unmarshal(marshalEntryJSON(t, fields), &entry))
	assert.False(t, entry.Quoted)
}

func TestEntryUnmarshalRejectsNullQuoted(t *testing.T) {
	fields := validEntryJSONFields()
	fields["Quoted"] = nil

	var entry Entry

	err := json.Unmarshal(marshalEntryJSON(t, fields), &entry)
	require.ErrorContains(t, err, "Quoted")
	assert.NotContains(t, err.Error(), "top-secret-value")
}

func TestEntryUnmarshalAllowsUnknownFields(t *testing.T) {
	fields := validEntryJSONFields()
	fields["FutureField"] = map[string]any{"enabled": true}

	var entry Entry
	require.NoError(t, json.Unmarshal(marshalEntryJSON(t, fields), &entry))
	assert.Equal(t, "token", entry.Name)
	assert.True(t, entry.Secure)
	assert.True(t, entry.HostOnly)
	assert.True(t, entry.Quoted)
}

func TestNewCookieRejectsMissingScopeFieldsWithoutChangingFile(t *testing.T) {
	for _, field := range []string{"Secure", "HostOnly"} {
		for _, mode := range []string{"missing", "null"} {
			t.Run(field+" "+mode, func(t *testing.T) {
				fields := validEntryJSONFields()
				if mode == "missing" {
					delete(fields, field)
				} else {
					fields[field] = nil
				}

				path, original := writeEntryJSONFile(t, fields)
				_, err := NewCookie(WithFilePath(path))
				require.ErrorContains(t, err, field)
				assert.NotContains(t, err.Error(), "top-secret-value")

				after, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				assert.Equal(t, original, after)
			})
		}
	}
}

func TestNewCookieLoadsExplicitFalseScopeFields(t *testing.T) {
	fields := validEntryJSONFields()
	fields["Secure"] = false
	fields["HostOnly"] = false
	path, _ := writeEntryJSONFile(t, fields)

	jar, err := NewCookie(WithSyncInterval(0), WithFilePath(path))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, jar.Close(context.Background()))
	})

	u, err := url.Parse("http://sub.example.com/")
	require.NoError(t, err)

	cookies := jar.Cookies(u)
	require.Len(t, cookies, 1)
	assert.Equal(t, "token", cookies[0].Name)
}

func validEntryJSONFields() map[string]any {
	return map[string]any{
		"Name":       "token",
		"Value":      "top-secret-value",
		"Quoted":     true,
		"Domain":     "example.com",
		"Path":       "/",
		"SameSite":   "SameSite=None",
		"Secure":     true,
		"HttpOnly":   true,
		"Persistent": false,
		"HostOnly":   true,
		"Expires":    "9999-12-31T23:59:59Z",
		"Creation":   "2026-08-03T12:00:00Z",
		"LastAccess": "2026-08-03T12:00:00Z",
		"SeqNum":     uint64(0),
	}
}

func marshalEntryJSON(t *testing.T, fields map[string]any) []byte {
	t.Helper()

	data, err := json.Marshal(fields)
	require.NoError(t, err)

	return data
}

func writeEntryJSONFile(t *testing.T, fields map[string]any) (string, []byte) {
	t.Helper()

	content := map[string]map[string]json.RawMessage{
		"example.com": {
			"example.com;/;token": marshalEntryJSON(t, fields),
		},
	}
	data, err := json.Marshal(content)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "cookie.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))

	return path, data
}

type emptyStateRestorePSL struct{}

func (emptyStateRestorePSL) PublicSuffix(string) string { return "" }
func (emptyStateRestorePSL) String() string             { return "empty state restore PSL" }

type hostDependentStateRestorePSL map[string]string

func (p hostDependentStateRestorePSL) PublicSuffix(domain string) string {
	if suffix, ok := p[domain]; ok {
		return suffix
	}

	return "uk"
}

func (hostDependentStateRestorePSL) String() string { return "host-dependent state restore PSL" }

func TestPersistentCookiePreservesOriginDerivedBucket(t *testing.T) {
	tests := []struct {
		name       string
		psl        PublicSuffixList
		origin     string
		domain     string
		visibleURL string
		hiddenURLs []string
	}{
		{
			name:       "nil PSL single-label domain",
			origin:     "http://foo.com/",
			domain:     "com",
			visibleURL: "http://sub.foo.com/",
			hiddenURLs: []string{"http://bar.com/", "http://com/"},
		},
		{
			name:       "empty PSL safe fallback",
			psl:        emptyStateRestorePSL{},
			origin:     "http://www.example.com/",
			domain:     "example.com",
			visibleURL: "http://www.example.com/",
			hiddenURLs: []string{"http://api.www.example.com/", "http://example.com/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "cookie.json")
			jar, err := NewCookie(
				WithFilePath(path),
				WithSyncInterval(0),
				WithPublicSuffixList(tt.psl),
			)
			require.NoError(t, err)

			origin := mustCookieURL(t, tt.origin)
			jar.SetCookies(origin, []*http.Cookie{{Name: "scoped", Value: "value", Domain: tt.domain}})
			assertStateRestoreScope(t, jar, origin, tt.visibleURL, tt.hiddenURLs)
			require.NoError(t, jar.Close(context.Background()))

			reloaded, err := NewCookie(
				WithFilePath(path),
				WithSyncInterval(0),
				WithPublicSuffixList(tt.psl),
			)
			require.NoError(t, err)
			assertStateRestoreScope(t, reloaded, origin, tt.visibleURL, tt.hiddenURLs)
			require.NoError(t, reloaded.Close(context.Background()))

			reloadedAgain, err := NewCookie(
				WithFilePath(path),
				WithSyncInterval(0),
				WithPublicSuffixList(tt.psl),
			)
			require.NoError(t, err)
			assertStateRestoreScope(t, reloadedAgain, origin, tt.visibleURL, tt.hiddenURLs)
			require.NoError(t, reloadedAgain.Close(context.Background()))
		})
	}
}

func TestPersistentCookiePreservesAmbiguousCustomPSLBucket(t *testing.T) {
	const (
		originURL = "https://sub.foo.example.co.uk/"
		domainURL = "https://foo.example.co.uk/"
	)

	psl := hostDependentStateRestorePSL{
		"sub.foo.example.co.uk": "uk",
		"foo.example.co.uk":     "example.co.uk",
	}
	path := filepath.Join(t.TempDir(), "cookie.json")
	jar, err := NewCookie(
		WithFilePath(path),
		WithSyncInterval(0),
		WithPublicSuffixList(psl),
	)
	require.NoError(t, err)

	origin := mustCookieURL(t, originURL)
	jar.SetCookies(origin, []*http.Cookie{{
		Name:   "scoped",
		Value:  "value",
		Domain: "foo.example.co.uk",
	}})
	assertAmbiguousCustomPSLScope(t, jar, originURL, domainURL)
	require.NoError(t, jar.Close(context.Background()))

	reloaded, err := NewCookie(
		WithFilePath(path),
		WithSyncInterval(0),
		WithPublicSuffixList(psl),
	)
	require.NoError(t, err)
	assertAmbiguousCustomPSLScope(t, reloaded, originURL, domainURL)
	require.NoError(t, reloaded.Close(context.Background()))
}

func TestPersistentCookieMigratesHostOnlyLegacyBucket(t *testing.T) {
	now := time.Now().UTC()
	e := validRuntimeEntry("host-only", 0, now)
	e.Domain = "sub.foo.example.co.uk"
	e.HostOnly = true
	legacyBucket := jarKey(e.Domain, nil)
	targetBucket := jarKey(e.Domain, publicsuffix.List)
	require.NotEqual(t, legacyBucket, targetBucket)

	path := filepath.Join(t.TempDir(), "cookie.json")
	writePersistedEntries(t, path, map[string]map[string]Entry{
		legacyBucket: {e.id(): e.persistedEntry()},
	})

	jar, err := NewCookie(WithFilePath(path), WithSyncInterval(0))
	require.NoError(t, err)

	jar.jar.mu.Lock()
	_, hasLegacyBucket := jar.jar.entries[legacyBucket]
	_, hasTargetBucket := jar.jar.entries[targetBucket]
	jar.jar.mu.Unlock()
	assert.False(t, hasLegacyBucket)
	assert.True(t, hasTargetBucket)
	assert.Equal(t, []string{"host-only"}, cookieNames(jar.Cookies(mustCookieURL(t, "https://sub.foo.example.co.uk/"))))
	assert.Empty(t, jar.Cookies(mustCookieURL(t, "https://child.sub.foo.example.co.uk/")))
	require.NoError(t, jar.Close(context.Background()))
}

func TestPersistentCookieMigratesDomainLegacyBucketWithDefaultPSL(t *testing.T) {
	now := time.Now().UTC()
	e := validRuntimeEntry("domain", 0, now)
	e.Domain = "foo.example.co.uk"
	e.HostOnly = false
	legacyBucket := jarKey(e.Domain, nil)
	targetBucket := jarKey(e.Domain, publicsuffix.List)
	require.NotEqual(t, legacyBucket, targetBucket)

	path := filepath.Join(t.TempDir(), "cookie.json")
	writePersistedEntries(t, path, map[string]map[string]Entry{
		legacyBucket: {e.id(): e.persistedEntry()},
	})

	jar, err := NewCookie(WithFilePath(path), WithSyncInterval(0))
	require.NoError(t, err)

	jar.jar.mu.Lock()
	_, hasLegacyBucket := jar.jar.entries[legacyBucket]
	_, hasTargetBucket := jar.jar.entries[targetBucket]
	jar.jar.mu.Unlock()
	assert.False(t, hasLegacyBucket)
	assert.True(t, hasTargetBucket)
	assert.Equal(t, []string{"domain"}, cookieNames(jar.Cookies(mustCookieURL(t, "https://child.foo.example.co.uk/"))))
	require.NoError(t, jar.Close(context.Background()))
}

func TestPersistentCookieRejectsImpossibleOriginDerivedBucket(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name      string
		bucket    string
		configure func(*entry)
	}{
		{
			name:   "non-canonical bucket",
			bucket: "FOO.COM",
			configure: func(e *entry) {
				e.Domain = "com"
				e.HostOnly = false
			},
		},
		{
			name:   "bucket is not a jar key",
			bucket: "sub.foo.com",
			configure: func(e *entry) {
				e.Domain = "com"
				e.HostOnly = false
			},
		},
		{
			name:   "bucket cannot produce domain",
			bucket: "foo.net",
			configure: func(e *entry) {
				e.Domain = "com"
				e.HostOnly = false
			},
		},
		{
			name:   "IP-like bucket cannot produce domain",
			bucket: "::1%.com",
			configure: func(e *entry) {
				e.Domain = "com"
				e.HostOnly = false
			},
		},
		{
			name:   "host-only bucket must be exact",
			bucket: "sub.foo.com",
			configure: func(e *entry) {
				e.Domain = "foo.com"
				e.HostOnly = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validRuntimeEntry("scope", 0, now)
			e.Value = "top-secret-value"
			tt.configure(&e)
			id := e.id()
			path := filepath.Join(t.TempDir(), "cookie.json")
			writePersistedEntries(t, path, map[string]map[string]Entry{
				tt.bucket: {id: e.persistedEntry()},
			})

			_, err := NewCookie(
				WithFilePath(path),
				WithSyncInterval(0),
				WithPublicSuffixList(nil),
			)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), e.Value)
		})
	}
}

func TestPersistentCookieDensifiesSequenceNumbers(t *testing.T) {
	now := time.Now().UTC()
	existing := validRuntimeEntry("existing", ^uint64(0)-1, now.Add(-time.Hour))
	path := filepath.Join(t.TempDir(), "cookie.json")
	writePersistedEntries(t, path, persistedEntries(existing))

	jar, err := NewCookie(WithFilePath(path), WithSyncInterval(0))
	require.NoError(t, err)
	assert.Equal(t, uint64(0), findRuntimeEntry(t, jar, "existing").seqNum)
	assert.Equal(t, uint64(1), jar.jar.nextSeqNum)

	u := mustCookieURL(t, "https://example.com/")
	jar.SetCookies(u, []*http.Cookie{
		{Name: "A", Value: "a"},
		{Name: "B", Value: "b"},
	})
	assert.Equal(t, []string{"existing", "A", "B"}, cookieNames(jar.Cookies(u)))
	assert.Equal(t, uint64(1), findRuntimeEntry(t, jar, "A").seqNum)
	assert.Equal(t, uint64(2), findRuntimeEntry(t, jar, "B").seqNum)
	require.NoError(t, jar.Close(context.Background()))

	reloaded, err := NewCookie(WithFilePath(path), WithSyncInterval(0))
	require.NoError(t, err)
	assert.Equal(t, []string{"existing", "A", "B"}, cookieNames(reloaded.Cookies(u)))
	assert.Equal(t, uint64(0), findRuntimeEntry(t, reloaded, "existing").seqNum)
	assert.Equal(t, uint64(1), findRuntimeEntry(t, reloaded, "A").seqNum)
	assert.Equal(t, uint64(2), findRuntimeEntry(t, reloaded, "B").seqNum)
	assert.Equal(t, uint64(3), reloaded.jar.nextSeqNum)
	require.NoError(t, reloaded.Close(context.Background()))
}

func TestPersistentCookieSequenceNormalizationPreservesPathOrdering(t *testing.T) {
	now := time.Now().UTC()
	rootA := validRuntimeEntry("root-a", 7, now)
	rootB := validRuntimeEntry("root-b", 7, now)
	deepA := validRuntimeEntry("deep-a", 7, now)
	deepA.Path = "/deep"
	deepB := validRuntimeEntry("deep-b", 7, now)
	deepB.Path = "/deep"

	path := filepath.Join(t.TempDir(), "cookie.json")
	writePersistedEntries(t, path, persistedEntries(deepB, rootB, deepA, rootA))

	jar, err := NewCookie(WithFilePath(path), WithSyncInterval(0))
	require.NoError(t, err)
	assertNormalizedPathOrder(t, jar)
	require.NoError(t, jar.Close(context.Background()))

	reloaded, err := NewCookie(WithFilePath(path), WithSyncInterval(0))
	require.NoError(t, err)
	assertNormalizedPathOrder(t, reloaded)
	require.NoError(t, reloaded.Close(context.Background()))
}

func assertStateRestoreScope(t *testing.T, jar *Cookie, origin *url.URL, visibleURL string, hiddenURLs []string) {
	t.Helper()

	for _, u := range []*url.URL{origin, mustCookieURL(t, visibleURL)} {
		got := jar.Cookies(u)
		require.Len(t, got, 1, u.String())
		assert.Equal(t, "scoped", got[0].Name, u.String())
	}

	for _, rawURL := range hiddenURLs {
		assert.Empty(t, jar.Cookies(mustCookieURL(t, rawURL)), rawURL)
	}
}

func assertAmbiguousCustomPSLScope(t *testing.T, jar *Cookie, originURL, domainURL string) {
	t.Helper()

	assert.Equal(t, []string{"scoped"}, cookieNames(jar.Cookies(mustCookieURL(t, originURL))))
	assert.Empty(t, jar.Cookies(mustCookieURL(t, domainURL)))
	assert.Empty(t, jar.Cookies(mustCookieURL(t, "https://bar.co.uk/")))
}

func assertNormalizedPathOrder(t *testing.T, jar *Cookie) {
	t.Helper()

	assert.Equal(t, uint64(0), findRuntimeEntry(t, jar, "root-a").seqNum)
	assert.Equal(t, uint64(1), findRuntimeEntry(t, jar, "root-b").seqNum)
	assert.Equal(t, uint64(2), findRuntimeEntry(t, jar, "deep-a").seqNum)
	assert.Equal(t, uint64(3), findRuntimeEntry(t, jar, "deep-b").seqNum)
	assert.Equal(t, uint64(4), jar.jar.nextSeqNum)
	assert.Equal(t,
		[]string{"deep-a", "deep-b", "root-a", "root-b"},
		cookieNames(jar.Cookies(mustCookieURL(t, "https://example.com/deep/item"))),
	)
}

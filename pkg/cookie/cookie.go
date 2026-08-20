// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package cookie

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/publicsuffix"
)

type Config struct {
	*Options `json:"-" yaml:"-"`

	Filepath string        `json:"filepath" yaml:"filepath"`
	Interval time.Duration `json:"interval" yaml:"interval"`
	// crypto
}

func (p Config) Valid() error {
	if p.Filepath == "" {
		return errors.New("cookie filepath is empty")
	}

	return nil
}

type Cookie struct {
	jar *Jar
	cfg *Config

	lifecycleMu sync.RWMutex
	closing     atomic.Bool
	persistMu   sync.Mutex
	async       bool
	done        chan struct{}
	syncDone    chan struct{}
	closed      chan struct{}
	closeOnce   sync.Once
	closeErr    error
}

func NewCookie(opts ...Option) (*Cookie, error) {
	cfg := Config{
		Options: &Options{
			PublicSuffixList: publicsuffix.List,
		},
		Filepath: "./cookie.json",
		Interval: 3 * time.Second,
	}

	for _, opt := range opts {
		if opt == nil {
			return nil, errors.New("cookie option is nil")
		}

		opt.apply(&cfg)
	}

	if err := cfg.Valid(); err != nil {
		return nil, fmt.Errorf("validate cookie config: %w", err)
	}

	jar, err := New(cfg.Options)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	c := &Cookie{
		jar:      jar,
		cfg:      &cfg,
		async:    cfg.Interval > 0,
		done:     make(chan struct{}),
		syncDone: make(chan struct{}),
		closed:   make(chan struct{}),
	}
	if err := c.init(); err != nil {
		return nil, fmt.Errorf("initialize cookie jar: %w", err)
	}

	if c.async {
		go c.sync()
	} else {
		close(c.syncDone)
	}

	return c, nil
}

func (c *Cookie) SetCookies(u *url.URL, cookies []*http.Cookie) {
	c.lifecycleMu.RLock()
	defer c.lifecycleMu.RUnlock()

	if c.closing.Load() {
		return
	}

	c.jar.SetCookies(u, cookies)

	if !c.async {
		if err := c.export(); err != nil {
			log.Printf("cookie: persist SetCookies update: %v", err)
		}
	}
}

func (c *Cookie) Cookies(u *url.URL) []*http.Cookie {
	return c.jar.Cookies(u)
}

func (c *Cookie) Close(ctx context.Context) error {
	if ctx == nil {
		return errors.New("cookie close context is nil")
	}

	c.closeOnce.Do(func() {
		c.closing.Store(true)
		go c.shutdown()
	})

	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case <-c.closed:
		if err := ctx.Err(); err != nil {
			return err
		}

		return c.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Cookie) shutdown() {
	close(c.done)

	// The write-lock barrier waits for SetCookies calls accepted before Close.
	c.lifecycleMu.Lock()
	c.lifecycleMu.Unlock() //nolint:gocritic,staticcheck // An immediate unlock is the intended lifecycle barrier.

	<-c.syncDone

	c.closeErr = c.export()
	close(c.closed)
}

func (c *Cookie) sync() {
	defer close(c.syncDone)

	ticker := time.NewTicker(c.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.export(); err != nil {
				log.Printf("cookie: periodic export: %v", err)
			}
		case <-c.done:
			return
		}
	}
}

func (c *Cookie) init() error {
	data, err := os.ReadFile(c.cfg.Filepath)
	if errors.Is(err, os.ErrNotExist) {
		if mkdirErr := os.MkdirAll(filepath.Dir(c.cfg.Filepath), 0o700); mkdirErr != nil {
			return fmt.Errorf("create cookie directory: %w", mkdirErr)
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("read cookie file: %w", err)
	}

	var content map[string]map[string]Entry
	if err := json.Unmarshal(data, &content); err != nil {
		return fmt.Errorf("decode cookie file: %w", err)
	}

	return c.importEntries(content, time.Now())
}

type loadedEntry struct {
	bucket string
	id     string
	entry  entry
}

func (c *Cookie) importEntries(content map[string]map[string]Entry, now time.Time) error {
	loaded := make([]loadedEntry, 0)
	seenEntries := make(map[struct{ bucket, id string }]struct{})

	for bucket, entries := range content {
		for id := range entries {
			persisted := entries[id]

			e := persisted.runtimeEntry()

			targetBucket, err := c.validateEntry(bucket, id, &e)
			if err != nil {
				return fmt.Errorf("validate cookie bucket %q entry %q: %w", bucket, id, err)
			}

			if e.Persistent && !e.Expires.After(now) {
				continue
			}

			key := struct{ bucket, id string }{bucket: targetBucket, id: id}
			if _, exists := seenEntries[key]; exists {
				return fmt.Errorf("validate cookie bucket %q entry %q: duplicate entry after bucket migration", bucket, id)
			}

			seenEntries[key] = struct{}{}

			loaded = append(loaded, loadedEntry{bucket: targetBucket, id: id, entry: e})
		}
	}

	nextSeqNum := normalizeSequenceNumbers(loaded)

	imported := make(map[string]map[string]entry)

	for i := range loaded {
		item := &loaded[i]
		if imported[item.bucket] == nil {
			imported[item.bucket] = make(map[string]entry)
		}

		imported[item.bucket][item.id] = item.entry
	}

	// Initialization happens before the Jar is published or its sync goroutine starts.
	c.jar.nextSeqNum = nextSeqNum
	c.jar.entries = imported

	return nil
}

func (c *Cookie) validateEntry(bucket, id string, e *entry) (string, error) {
	if err := (&http.Cookie{Name: e.Name, Value: e.Value, Quoted: e.Quoted}).Valid(); err != nil {
		return "", fmt.Errorf("cookie name or value is invalid: %w", err)
	}

	if e.Domain == "" {
		return "", errors.New("domain is empty")
	}

	canonical, err := canonicalHost(e.Domain)
	if err != nil {
		return "", fmt.Errorf("canonicalize domain: %w", err)
	}

	if canonical != e.Domain {
		return "", errors.New("domain is not canonical")
	}

	if e.Path == "" || e.Path[0] != '/' {
		return "", errors.New("path is not absolute")
	}

	if id != e.id() {
		return "", errors.New("entry id does not match cookie fields")
	}

	if !e.HostOnly && isIP(e.Domain) {
		return "", errors.New("IP cookie is not host-only")
	}

	if !e.HostOnly && c.jar.psList != nil {
		suffix := c.jar.psList.PublicSuffix(e.Domain)
		if suffix != "" && !hasDotSuffix(e.Domain, suffix) {
			return "", errors.New("domain cookie targets a public suffix")
		}
	}

	if e.Expires.IsZero() {
		return "", errors.New("expiration time is missing")
	}

	if !e.Persistent && !e.Expires.Equal(endOfTime) {
		return "", errors.New("session cookie expiration is invalid")
	}

	if e.Creation.IsZero() {
		return "", errors.New("creation time is missing")
	}

	if e.LastAccess.IsZero() {
		return "", errors.New("last access time is missing")
	}

	return c.restoreBucket(bucket, e)
}

func (c *Cookie) restoreBucket(bucket string, e *entry) (string, error) {
	targetBucket := jarKey(e.Domain, c.jar.psList)
	if bucket == targetBucket {
		return bucket, nil
	}

	legacyBucket := jarKey(e.Domain, nil)
	if c.canMigrateLegacyBucket(bucket, targetBucket, legacyBucket, e.HostOnly) {
		return targetBucket, nil
	}

	if e.HostOnly {
		return "", errors.New("host-only cookie bucket does not match cookie domain")
	}

	// A broken or empty PSL can key a domain cookie outside jarKey(e.Domain).
	canonical, err := canonicalHost(bucket)
	if err != nil {
		return "", fmt.Errorf("canonicalize bucket: %w", err)
	}

	if canonical != bucket {
		return "", errors.New("bucket is not canonical")
	}

	if isIP(bucket) {
		return "", errors.New("domain cookie bucket is IP-like")
	}

	if jarKey(bucket, c.jar.psList) != bucket {
		return "", errors.New("bucket is not a possible cookie jar key")
	}

	if bucket != e.Domain && !hasDotSuffix(bucket, e.Domain) && !hasDotSuffix(e.Domain, bucket) {
		return "", errors.New("bucket cannot scope cookie domain")
	}

	return bucket, nil
}

func (c *Cookie) canMigrateLegacyBucket(bucket, target, legacy string, hostOnly bool) bool {
	if target == legacy || bucket != legacy {
		return false
	}

	// A host-only cookie's origin is its domain, so its legacy bucket is
	// unambiguous. Domain-cookie buckets from custom lists are not: the list may
	// return a different suffix for the original origin than for the domain.
	return hostOnly || reflect.TypeOf(c.jar.psList) == reflect.TypeOf(publicsuffix.List)
}

// normalizeSequenceNumbers preserves RFC ordering while removing persisted gaps and overflow edges.
func normalizeSequenceNumbers(entries []loadedEntry) uint64 {
	sort.Slice(entries, func(i, j int) bool {
		left := &entries[i]

		right := &entries[j]
		if order := left.entry.Creation.Compare(right.entry.Creation); order != 0 {
			return order < 0
		}

		if left.entry.seqNum != right.entry.seqNum {
			return left.entry.seqNum < right.entry.seqNum
		}

		if left.bucket != right.bucket {
			return left.bucket < right.bucket
		}

		return left.id < right.id
	})

	for i := range entries {
		entries[i].entry.seqNum = uint64(i)
	}

	return uint64(len(entries))
}

func (c *Cookie) export() error {
	c.persistMu.Lock()
	defer c.persistMu.Unlock()

	exported := c.snapshot()

	data, err := json.Marshal(exported)
	if err != nil {
		return fmt.Errorf("encode cookie file: %w", err)
	}

	return writeCookieFile(c.cfg.Filepath, data)
}

func (c *Cookie) snapshot() map[string]map[string]Entry {
	c.jar.mu.Lock()
	defer c.jar.mu.Unlock()

	exported := make(map[string]map[string]Entry, len(c.jar.entries))
	for bucket, entries := range c.jar.entries {
		exported[bucket] = make(map[string]Entry, len(entries))

		for id := range entries {
			e := entries[id]
			exported[bucket][id] = e.persistedEntry()
		}
	}

	return exported
}

func writeCookieFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create cookie directory: %w", err)
	}

	temp, createErr := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if createErr != nil {
		return fmt.Errorf("create temporary cookie file: %w", createErr)
	}

	var (
		tempName   = temp.Name()
		tempClosed bool
	)

	defer func() {
		if !tempClosed {
			_ = temp.Close()
		}

		_ = os.Remove(tempName)
	}()

	if err := temp.Chmod(0o600); err != nil {
		return fmt.Errorf("restrict temporary cookie file: %w", err)
	}

	written, writeErr := temp.Write(data)
	if writeErr != nil {
		return fmt.Errorf("write temporary cookie file: %w", writeErr)
	}

	if written != len(data) {
		return fmt.Errorf("write temporary cookie file: %w", io.ErrShortWrite)
	}

	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary cookie file: %w", err)
	}

	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary cookie file: %w", err)
	}

	tempClosed = true

	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace cookie file: %w", err)
	}

	return nil
}

type Entry struct {
	Name       string    `json:"Name"`
	Value      string    `json:"Value"`
	Quoted     bool      `json:"Quoted,omitempty"`
	Domain     string    `json:"Domain"`
	Path       string    `json:"Path"`
	SameSite   string    `json:"SameSite"`
	Secure     bool      `json:"Secure"`
	HttpOnly   bool      `json:"HttpOnly"`
	Persistent bool      `json:"Persistent"`
	HostOnly   bool      `json:"HostOnly"`
	Expires    time.Time `json:"Expires"`
	Creation   time.Time `json:"Creation"`
	LastAccess time.Time `json:"LastAccess"`
	SeqNum     uint64    `json:"SeqNum"`
}

// UnmarshalJSON distinguishes required fields from their valid zero values.
// Quoted is the only optional field because it was added after the original
// persistence format.
func (e *Entry) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("decode cookie entry: %w", err)
	}

	var decoded Entry
	for _, field := range []struct {
		name string
		dest any
	}{
		{name: "Name", dest: &decoded.Name},
		{name: "Value", dest: &decoded.Value},
		{name: "Domain", dest: &decoded.Domain},
		{name: "Path", dest: &decoded.Path},
		{name: "SameSite", dest: &decoded.SameSite},
		{name: "Secure", dest: &decoded.Secure},
		{name: "HttpOnly", dest: &decoded.HttpOnly},
		{name: "Persistent", dest: &decoded.Persistent},
		{name: "HostOnly", dest: &decoded.HostOnly},
		{name: "Expires", dest: &decoded.Expires},
		{name: "Creation", dest: &decoded.Creation},
		{name: "LastAccess", dest: &decoded.LastAccess},
		{name: "SeqNum", dest: &decoded.SeqNum},
	} {
		if err := decodeRequiredEntryField(fields, field.name, field.dest); err != nil {
			return err
		}
	}

	if raw, ok := fields["Quoted"]; ok {
		if isJSONNull(raw) {
			return nullEntryFieldError("Quoted")
		}

		if err := json.Unmarshal(raw, &decoded.Quoted); err != nil {
			return fmt.Errorf("decode cookie entry field %q: %w", "Quoted", err)
		}
	}

	*e = decoded

	return nil
}

func (e *Entry) runtimeEntry() entry {
	return entry{
		Name:       e.Name,
		Value:      e.Value,
		Quoted:     e.Quoted,
		Domain:     e.Domain,
		Path:       e.Path,
		SameSite:   e.SameSite,
		Secure:     e.Secure,
		HttpOnly:   e.HttpOnly,
		Persistent: e.Persistent,
		HostOnly:   e.HostOnly,
		Expires:    e.Expires,
		Creation:   e.Creation,
		LastAccess: e.LastAccess,
		seqNum:     e.SeqNum,
	}
}

func decodeRequiredEntryField(fields map[string]json.RawMessage, name string, dest any) error {
	raw, ok := fields[name]
	if !ok {
		return fmt.Errorf("cookie entry field %q is missing", name)
	}

	if isJSONNull(raw) {
		return nullEntryFieldError(name)
	}

	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode cookie entry field %q: %w", name, err)
	}

	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func nullEntryFieldError(name string) error {
	return fmt.Errorf("cookie entry field %q is null", name)
}

func (e *entry) persistedEntry() Entry {
	return Entry{
		Name:       e.Name,
		Value:      e.Value,
		Quoted:     e.Quoted,
		Domain:     e.Domain,
		Path:       e.Path,
		SameSite:   e.SameSite,
		Secure:     e.Secure,
		HttpOnly:   e.HttpOnly,
		Persistent: e.Persistent,
		HostOnly:   e.HostOnly,
		Expires:    e.Expires,
		Creation:   e.Creation,
		LastAccess: e.LastAccess,
		SeqNum:     e.seqNum,
	}
}

// Option is an option to configure PersistentJarOptions.
type Option interface {
	apply(p *Config)
}

type optionFunc func(p *Config)

func (f optionFunc) apply(p *Config) {
	f(p)
}

// WithSyncInterval sets sync time interval.
func WithSyncInterval(interval time.Duration) Option {
	return optionFunc(func(p *Config) {
		p.Interval = interval
	})
}

// WithFilePath sets the file path.
func WithFilePath(filePath string) Option {
	return optionFunc(func(p *Config) {
		p.Filepath = filePath
	})
}

// WithPublicSuffixList sets the public suffix list.
func WithPublicSuffixList(list PublicSuffixList) Option {
	return optionFunc(func(p *Config) {
		if p.Options == nil {
			p.Options = &Options{}
		}

		p.PublicSuffixList = list
	})
}

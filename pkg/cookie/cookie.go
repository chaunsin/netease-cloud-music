// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package cookie

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	*Options
	Filepath string        `json:"filepath" yaml:"filepath"`
	Interval time.Duration `yaml:"interval" yaml:"interval"`
	// crypto
}

func (p Config) Valid() error {
	return nil
}

type Cookie struct {
	jar       *Jar
	cfg       *Config
	mu        sync.RWMutex
	async     bool
	done      chan struct{}
	closeOnce sync.Once
}

func NewCookie(opts ...Option) (*Cookie, error) {
	var cfg = Config{
		Options:  nil,
		Filepath: "./cookie.json",
		Interval: time.Second * 3,
	}
	for _, opt := range opts {
		opt.apply(&cfg)
	}
	if err := cfg.Valid(); err != nil {
		return nil, fmt.Errorf("valid: %w", err)
	}

	jar, err := New(cfg.Options)
	if err != nil {
		return nil, fmt.Errorf("new: %w", err)
	}

	p := Cookie{
		jar:   jar,
		cfg:   &cfg,
		done:  make(chan struct{}),
		async: true,
	}
	if cfg.Interval <= 0 {
		p.async = false
	}
	if err := p.init(); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	if p.async {
		go p.sync()
	}
	return &p, nil
}

func (c *Cookie) SetCookies(u *url.URL, cookies []*http.Cookie) {
	c.jar.SetCookies(u, cookies)
	if !c.async {
		if err := c.export(); err != nil {
			log.Printf("cookie warnning set cookies err: %s", err)
		}
	}
}

func (c *Cookie) Cookies(u *url.URL) []*http.Cookie {
	return c.jar.Cookies(u)
}

func (c *Cookie) Close(ctx context.Context) error {
	c.closeOnce.Do(func() {
		close(c.done)
		if err := c.export(); err != nil {
			log.Printf("cookie export err: %s", err)
		}
	})
	return nil
}

func (c *Cookie) sync() {
	tick := time.NewTicker(c.cfg.Interval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := c.export(); err != nil {
				log.Printf("cookie export err: %s", err)
			}
		case <-c.done:
			tick.Stop()
			return
		}
	}
}

func (c *Cookie) init() error {
	// 如果文件存在则读取配置文件
	if !fileExists(c.cfg.Filepath) {
		log.Printf("cookie: warnning %s file not found", c.cfg.Filepath)
		return os.MkdirAll(filepath.Dir(c.cfg.Filepath), os.ModePerm)
	}

	data, err := os.ReadFile(c.cfg.Filepath)
	if err != nil {
		return err
	}

	var (
		content    map[string]map[string]Entry
		imported   = make(map[string]map[string]entry)
		nextSeqNum uint64
	)
	if err := json.Unmarshal(data, &content); err != nil {
		return err
	}
	for domain, cookies := range content {
		imported[domain] = make(map[string]entry)
		for name, cookie := range cookies {
			imported[domain][name] = entry{
				Name:       cookie.Name,
				Value:      cookie.Value,
				Domain:     cookie.Domain,
				Path:       cookie.Path,
				SameSite:   cookie.SameSite,
				Secure:     cookie.Secure,
				HttpOnly:   cookie.HttpOnly,
				Persistent: cookie.Persistent,
				HostOnly:   cookie.HostOnly,
				Expires:    cookie.Expires,
				Creation:   cookie.Creation,
				LastAccess: cookie.LastAccess,
				seqNum:     cookie.SeqNum,
			}
			if cookie.SeqNum > nextSeqNum {
				nextSeqNum = cookie.SeqNum + 1
			}
		}
	}
	c.mu.Lock()
	c.jar.nextSeqNum = nextSeqNum
	c.jar.entries = imported
	c.mu.Unlock()
	return nil
}

func (c *Cookie) export() error {
	c.jar.mu.Lock()
	defer c.jar.mu.Unlock()

	var exported = make(map[string]map[string]Entry)
	for domain, cookies := range c.jar.entries {
		exported[domain] = make(map[string]Entry)
		for name, cookie := range cookies {
			exported[domain][name] = Entry{
				Name:       cookie.Name,
				Value:      cookie.Value,
				Domain:     cookie.Domain,
				Path:       cookie.Path,
				SameSite:   cookie.SameSite,
				Secure:     cookie.Secure,
				HttpOnly:   cookie.HttpOnly,
				Persistent: cookie.Persistent,
				HostOnly:   cookie.HostOnly,
				Expires:    cookie.Expires,
				Creation:   cookie.Creation,
				LastAccess: cookie.LastAccess,
				SeqNum:     cookie.seqNum,
			}
		}
	}
	data, err := json.Marshal(exported)
	if err != nil {
		return err
	}
	if err := os.WriteFile(c.cfg.Filepath, data, os.ModePerm); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(filepath.Dir(c.cfg.Filepath), os.ModePerm); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("WriteFile: %w", err)
	}
	return nil
}

type Entry struct {
	Name       string
	Value      string
	Domain     string
	Path       string
	SameSite   string
	Secure     bool
	HttpOnly   bool
	Persistent bool
	HostOnly   bool
	Expires    time.Time
	Creation   time.Time
	LastAccess time.Time
	SeqNum     uint64
}

func fileExists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

// Option is an option to configure PersistentJarOptions.
type Option interface {
	apply(p *Config)
}

type optionFunc func(p *Config)

func (f optionFunc) apply(p *Config) {
	f(p)
}

// WithSyncInterval sets sync time interval
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
		p.Options.PublicSuffixList = list
	})
}

// // WithLogger sets the logger.
// func WithLogger(logger log.Logger) Option {
// 	return optionFunc(func(p *Config) {
// 		p.logger = logger
// 	})
// }

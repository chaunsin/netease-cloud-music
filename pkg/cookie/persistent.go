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

type PersistentJarConfig struct {
	*Options
	Filepath string        `json:"filepath" yaml:"filepath"`
	Interval time.Duration `yaml:"interval" yaml:"interval"`
	// crypto
}

func (p PersistentJarConfig) Valid() error {
	return nil
}

type PersistentJar struct {
	jar       *Jar
	cfg       *PersistentJarConfig
	mu        sync.RWMutex
	async     bool
	done      chan struct{}
	closeOnce sync.Once
}

func NewPersistentJar(opts ...PersistentJarOption) (*PersistentJar, error) {
	var cfg = PersistentJarConfig{
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

	p := PersistentJar{
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

func (c *PersistentJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	c.jar.SetCookies(u, cookies)
	if !c.async {
		if err := c.export(); err != nil {
			log.Printf("cookie warnning set cookies err: %s", err)
		}
	}
}

func (c *PersistentJar) Cookies(u *url.URL) []*http.Cookie {
	return c.jar.Cookies(u)
}

func (c *PersistentJar) Close(ctx context.Context) error {
	c.closeOnce.Do(func() {
		close(c.done)
	})
	return nil
}

func (c *PersistentJar) sync() {
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
		}
	}
}

func (c *PersistentJar) init() error {
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

func (c *PersistentJar) export() error {
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

// PersistentJarOption is an option to configure PersistentJarOptions.
type PersistentJarOption interface {
	apply(p *PersistentJarConfig)
}

type persistentJarOptionFunc func(p *PersistentJarConfig)

func (f persistentJarOptionFunc) apply(p *PersistentJarConfig) {
	f(p)
}

// WithSyncInterval sets sync time interval
func WithSyncInterval(interval time.Duration) PersistentJarOption {
	return persistentJarOptionFunc(func(p *PersistentJarConfig) {
		p.Interval = interval
	})
}

// WithFilePath sets the file path.
func WithFilePath(filePath string) PersistentJarOption {
	return persistentJarOptionFunc(func(p *PersistentJarConfig) {
		p.Filepath = filePath
	})
}

// WithPublicSuffixList sets the public suffix list.
func WithPublicSuffixList(list PublicSuffixList) PersistentJarOption {
	return persistentJarOptionFunc(func(p *PersistentJarConfig) {
		p.Options.PublicSuffixList = list
	})
}

// // WithLogger sets the logger.
// func WithLogger(logger log.Logger) PersistentJarOption {
// 	return persistentJarOptionFunc(func(p *PersistentJarConfig) {
// 		p.logger = logger
// 	})
// }

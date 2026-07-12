// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/elazarl/goproxy"
)

const (
	caRSAKeyBits          = 2048
	maxCachedCertificates = 256
)

// loadOrCreateCA manages a CA pair. When requirePrivateCAPath is
// true, existing parent directories must remain private as well as the key.
func loadOrCreateCA(certPath, keyPath string, requirePrivateCAPath bool) (*tls.Certificate, bool, error) {
	if certPath == "" || keyPath == "" {
		return nil, false, fmt.Errorf("CA certificate and key paths are required")
	}
	if filepath.Clean(certPath) == filepath.Clean(keyPath) {
		return nil, false, fmt.Errorf("CA certificate and key paths must be different")
	}
	certExists, err := pathExists(certPath)
	if err != nil {
		return nil, false, fmt.Errorf("check CA certificate: %w", err)
	}
	keyExists, err := pathExists(keyPath)
	if err != nil {
		return nil, false, fmt.Errorf("check CA private key: %w", err)
	}
	if certExists != keyExists {
		return nil, false, fmt.Errorf("CA certificate and private key must either both exist or both be absent")
	}
	if certExists {
		if requirePrivateCAPath {
			for _, dir := range uniqueParentDirs(certPath, keyPath) {
				if err := ensurePrivateDir(dir); err != nil {
					return nil, false, err
				}
			}
		}
		if err := secureCAPrivateKey(keyPath); err != nil {
			return nil, false, fmt.Errorf("secure CA private key: %w", err)
		}
		ca, err := loadCA(certPath, keyPath)
		if err != nil {
			return nil, false, err
		}
		return ca, false, nil
	}

	for _, dir := range uniqueParentDirs(certPath, keyPath) {
		if err := ensurePrivateDir(dir); err != nil {
			return nil, false, err
		}
	}

	certPEM, keyPEM, err := generateCA(time.Now())
	if err != nil {
		return nil, false, err
	}
	if err := writeExclusive(keyPath, keyPEM, 0o600); err != nil {
		return nil, false, fmt.Errorf("write CA private key: %w", err)
	}
	if err := secureCAPrivateKey(keyPath); err != nil {
		_ = os.Remove(keyPath)
		return nil, false, fmt.Errorf("secure CA private key: %w", err)
	}
	if err := writeExclusive(certPath, certPEM, 0o644); err != nil {
		_ = os.Remove(keyPath)
		return nil, false, fmt.Errorf("write CA certificate: %w", err)
	}

	ca, err := loadCA(certPath, keyPath)
	if err != nil {
		_ = os.Remove(certPath)
		_ = os.Remove(keyPath)
		return nil, false, fmt.Errorf("validate generated CA: %w", err)
	}
	return ca, true, nil
}

// loadCA validates an existing CA without changing its parent directory. It is
// also used for explicitly supplied certificate and key files.
func loadCA(certPath, keyPath string) (*tls.Certificate, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read CA private key: %w", err)
	}

	ca, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate and private key: %w", err)
	}
	if len(ca.Certificate) == 0 {
		return nil, fmt.Errorf("CA certificate chain is empty")
	}
	leaf, err := x509.ParseCertificate(ca.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse CA leaf certificate: %w", err)
	}
	ca.Leaf = leaf
	if err := validateCA(&ca, time.Now()); err != nil {
		return nil, err
	}
	return &ca, nil
}

func validateCA(ca *tls.Certificate, now time.Time) error {
	if ca == nil || ca.Leaf == nil {
		return fmt.Errorf("CA leaf certificate is missing")
	}
	if !ca.Leaf.BasicConstraintsValid || !ca.Leaf.IsCA {
		return fmt.Errorf("certificate is not a CA")
	}
	if ca.Leaf.KeyUsage&x509.KeyUsageCertSign == 0 {
		return fmt.Errorf("CA certificate cannot sign certificates")
	}
	if now.Before(ca.Leaf.NotBefore) {
		return fmt.Errorf("CA certificate is not valid before %s", ca.Leaf.NotBefore.Format(time.RFC3339))
	}
	if now.After(ca.Leaf.NotAfter) {
		return fmt.Errorf("CA certificate expired at %s", ca.Leaf.NotAfter.Format(time.RFC3339))
	}

	signer, ok := ca.PrivateKey.(crypto.Signer)
	if !ok {
		return fmt.Errorf("CA private key does not implement crypto.Signer")
	}
	certPublicKey, err := x509.MarshalPKIXPublicKey(ca.Leaf.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal CA certificate public key: %w", err)
	}
	privatePublicKey, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return fmt.Errorf("marshal CA private key public key: %w", err)
	}
	if !bytes.Equal(certPublicKey, privatePublicKey) {
		return fmt.Errorf("CA certificate and private key do not match")
	}
	return nil
}

func generateCA(now time.Time) ([]byte, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, caRSAKeyBits)
	if err != nil {
		return nil, nil, fmt.Errorf("generate CA private key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("generate CA serial number: %w", err)
	}
	if serial.Sign() == 0 {
		serial.SetInt64(1)
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal CA public key: %w", err)
	}
	subjectKeyID := sha256.Sum256(publicKeyDER)
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "ncmctl Local Proxy CA",
			Organization: []string{"ncmctl"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
		SubjectKeyId:          append([]byte(nil), subjectKeyID[:]...),
		AuthorityKeyId:        append([]byte(nil), subjectKeyID[:]...),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("create CA certificate: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return certPEM, keyPEM, nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func uniqueParentDirs(paths ...string) []string {
	dirs := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		dir := filepath.Dir(path)
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
	}
	return dirs
}

func ensurePrivateDir(dir string) error {
	info, err := os.Stat(dir)
	if err == nil && !info.IsDir() {
		return fmt.Errorf("CA parent path %q is not a directory", dir)
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect CA directory %q: %w", dir, err)
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create CA directory %q: %w", dir, err)
		}
	}
	return secureCAPrivateDir(dir)
}

func writeExclusive(path string, data []byte, mode os.FileMode) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.Remove(path)
		}
	}()

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	succeeded = true
	return nil
}

var _ goproxy.CertStorage = (*memoryCertStore)(nil)

type memoryCertStore struct {
	mu    sync.Mutex
	certs map[string]*memoryCertEntry
	order []string
}

type memoryCertEntry struct {
	once sync.Once
	cert *tls.Certificate
	err  error
}

func newMemoryCertStore() *memoryCertStore {
	return &memoryCertStore{certs: make(map[string]*memoryCertEntry)}
}

func (s *memoryCertStore) Fetch(hostname string, gen func() (*tls.Certificate, error)) (*tls.Certificate, error) {
	// A trailing dot changes the name goproxy signs. Preserve it in the cache
	// key so a certificate for "music.163.com." cannot be reused for the bare
	// hostname (or vice versa).
	key := strings.ToLower(hostname)
	s.mu.Lock()
	if s.certs == nil {
		s.certs = make(map[string]*memoryCertEntry)
	}
	entry, exists := s.certs[key]
	if !exists {
		if len(s.certs) >= maxCachedCertificates {
			s.evictOldestLocked()
		}
		entry = &memoryCertEntry{}
		s.certs[key] = entry
		s.order = append(s.order, key)
	}
	s.mu.Unlock()

	entry.once.Do(func() {
		entry.cert, entry.err = gen()
	})
	if entry.err != nil {
		s.mu.Lock()
		if s.certs[key] == entry {
			delete(s.certs, key)
			s.removeFromOrderLocked(key)
		}
		s.mu.Unlock()
	}
	return entry.cert, entry.err
}

func (s *memoryCertStore) evictOldestLocked() {
	for len(s.order) > 0 {
		oldest := s.order[0]
		s.order = s.order[1:]
		if _, exists := s.certs[oldest]; exists {
			delete(s.certs, oldest)
			return
		}
	}
}

func (s *memoryCertStore) removeFromOrderLocked(key string) {
	for i, candidate := range s.order {
		if candidate == key {
			s.order = append(s.order[:i], s.order[i+1:]...)
			return
		}
	}
}

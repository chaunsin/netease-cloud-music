// Copyright (c) 2024-2026 chaunsin
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoadOrCreateCAGeneratesAndReuses(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proxy")
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")

	ca, created, err := loadOrCreateCA(certPath, keyPath, false)
	if err != nil {
		t.Fatalf("loadOrCreateCA() error = %v", err)
	}

	if !created {
		t.Fatal("loadOrCreateCA() created = false, want true")
	}

	if ca.Leaf == nil || !ca.Leaf.IsCA {
		t.Fatal("generated certificate is not a parsed CA")
	}

	privateKey, ok := ca.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		t.Fatalf("generated key type = %T, want *rsa.PrivateKey", ca.PrivateKey)
	}

	if bits := privateKey.N.BitLen(); bits != caRSAKeyBits {
		t.Fatalf("generated RSA key bits = %d, want %d", bits, caRSAKeyBits)
	}

	validity := ca.Leaf.NotAfter.Sub(ca.Leaf.NotBefore)
	if validity < 9*365*24*time.Hour || validity > 11*365*24*time.Hour {
		t.Fatalf("generated CA validity = %s, want approximately 10 years", validity)
	}

	if runtime.GOOS != "windows" {
		assertPermissions(t, dir, 0o700)
		assertPermissions(t, keyPath, 0o600)
	}

	firstCertificate := append([]byte(nil), ca.Certificate[0]...)

	ca, created, err = loadOrCreateCA(certPath, keyPath, false)
	if err != nil {
		t.Fatalf("second loadOrCreateCA() error = %v", err)
	}

	if created {
		t.Fatal("second loadOrCreateCA() created = true, want false")
	}

	if !bytes.Equal(ca.Certificate[0], firstCertificate) {
		t.Fatal("second loadOrCreateCA() did not reuse certificate")
	}
}

func TestLoadOrCreateCARepairsKeyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix file modes")
	}

	dir := filepath.Join(t.TempDir(), "proxy")
	certPath := filepath.Join(dir, "ca.crt")

	keyPath := filepath.Join(dir, "ca.key")
	if _, _, err := loadOrCreateCA(certPath, keyPath, false); err != nil {
		t.Fatalf("loadOrCreateCA() error = %v", err)
	}

	if err := os.Chmod(keyPath, 0o644); err != nil { //nolint:gosec // The test deliberately creates an insecure key.
		t.Fatalf("chmod key: %v", err)
	}

	if _, created, err := loadOrCreateCA(certPath, keyPath, false); err != nil {
		t.Fatalf("loadOrCreateCA() error = %v", err)
	} else if created {
		t.Fatal("existing CA reported as newly created")
	}

	assertPermissions(t, keyPath, 0o600)
}

func TestLoadOrCreateCARejectsBroadExistingManagedDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows fails closed before checking Unix permissions")
	}

	dir := filepath.Join(t.TempDir(), "proxy")
	certPath := filepath.Join(dir, "ca.crt")

	keyPath := filepath.Join(dir, "ca.key")
	if _, _, err := loadOrCreateCA(certPath, keyPath, false); err != nil {
		t.Fatalf("create CA: %v", err)
	}

	if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // The test deliberately weakens directory permissions.
		t.Fatalf("chmod CA directory: %v", err)
	}

	_, _, err := loadOrCreateCA(certPath, keyPath, true)
	if err == nil || !strings.Contains(err.Error(), "permissions are too broad") {
		t.Fatalf("loadOrCreateCA() error = %v, want private directory error", err)
	}
}

func TestLoadOrCreateCAAllowsExplicitExistingBroadDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows fails closed before checking Unix permissions")
	}

	dir := filepath.Join(t.TempDir(), "proxy")
	certPath := filepath.Join(dir, "ca.crt")

	keyPath := filepath.Join(dir, "ca.key")
	if _, _, err := loadOrCreateCA(certPath, keyPath, false); err != nil {
		t.Fatalf("create CA: %v", err)
	}

	if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // The test deliberately weakens directory permissions.
		t.Fatalf("chmod CA directory: %v", err)
	}

	if _, created, err := loadOrCreateCA(certPath, keyPath, false); err != nil {
		t.Fatalf("load explicit CA: %v", err)
	} else if created {
		t.Fatal("existing explicit CA reported as newly created")
	}
}

func TestLoadOrCreateCAFailsClosedOnPartialPair(t *testing.T) {
	for _, existing := range []string{"certificate", "key"} {
		t.Run(existing, func(t *testing.T) {
			dir := t.TempDir()
			certPath := filepath.Join(dir, "ca.crt")
			keyPath := filepath.Join(dir, "ca.key")

			path := certPath
			if existing == "key" {
				path = keyPath
			}

			original := []byte("do not overwrite")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			if _, _, err := loadOrCreateCA(certPath, keyPath, false); err == nil {
				t.Fatal("loadOrCreateCA() unexpectedly succeeded")
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read preserved file: %v", err)
			}

			if !bytes.Equal(got, original) {
				t.Fatalf("existing file was overwritten: %q", got)
			}
		})
	}
}

func TestLoadOrCreateCAAcceptsMissingKeyUsage(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	template := testCertificateTemplate(time.Now().Add(-time.Hour), time.Now().Add(time.Hour), true, 0)
	writeTestCertificatePair(t, certPath, keyPath, &template, false)

	ca, created, err := loadOrCreateCA(certPath, keyPath, false)
	if err != nil {
		t.Fatalf("loadOrCreateCA() error = %v", err)
	}

	if created {
		t.Fatal("existing CA reported as newly created")
	}

	if ca.Leaf.KeyUsage != 0 {
		t.Fatalf("KeyUsage = %v, want no extension", ca.Leaf.KeyUsage)
	}
}

func TestLoadOrCreateCARejectsInvalidExistingPair(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		template   x509.Certificate
		mismatched bool
		wantError  string
	}{
		{
			name:      "not a CA",
			template:  testCertificateTemplate(now.Add(-time.Hour), now.Add(time.Hour), false, x509.KeyUsageDigitalSignature),
			wantError: "not a CA",
		},
		{
			name:      "expired",
			template:  testCertificateTemplate(now.Add(-2*time.Hour), now.Add(-time.Hour), true, x509.KeyUsageCertSign),
			wantError: "expired",
		},
		{
			name:      "not yet valid",
			template:  testCertificateTemplate(now.Add(time.Hour), now.Add(2*time.Hour), true, x509.KeyUsageCertSign),
			wantError: "not valid before",
		},
		{
			name:      "cannot sign",
			template:  testCertificateTemplate(now.Add(-time.Hour), now.Add(time.Hour), true, x509.KeyUsageDigitalSignature),
			wantError: "cannot sign",
		},
		{
			name:       "key mismatch",
			template:   testCertificateTemplate(now.Add(-time.Hour), now.Add(time.Hour), true, x509.KeyUsageCertSign),
			mismatched: true,
			wantError:  "private key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			certPath := filepath.Join(dir, "ca.crt")
			keyPath := filepath.Join(dir, "ca.key")
			writeTestCertificatePair(t, certPath, keyPath, &tt.template, tt.mismatched)

			_, _, err := loadOrCreateCA(certPath, keyPath, false)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("loadOrCreateCA() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestLoadOrCreateCARejectsCorruptPair(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")

	if err := os.WriteFile(certPath, []byte("corrupt certificate"), 0o600); err != nil {
		t.Fatalf("write certificate: %v", err)
	}

	if err := os.WriteFile(keyPath, []byte("corrupt key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	if _, _, err := loadOrCreateCA(certPath, keyPath, false); err == nil {
		t.Fatal("loadOrCreateCA() unexpectedly succeeded")
	}
}

func TestLoadOrCreateCARejectsUnsafeInputs(t *testing.T) {
	if _, _, err := loadOrCreateCA("", "key", false); err == nil {
		t.Fatal("empty certificate path unexpectedly succeeded")
	}

	path := filepath.Join(t.TempDir(), "same")
	if _, _, err := loadOrCreateCA(path, path, false); err == nil {
		t.Fatal("identical certificate and key paths unexpectedly succeeded")
	}
}

func TestMemoryCertStoreCachesConcurrentGeneration(t *testing.T) {
	store := &memoryCertStore{}
	want := &tls.Certificate{}

	var calls atomic.Int32

	gen := func() (*tls.Certificate, error) { //nolint:unparam // CertStorage requires an error-returning generator callback.
		calls.Add(1)
		time.Sleep(5 * time.Millisecond)
		return want, nil
	}

	const goroutines = 32

	results := make(chan *tls.Certificate, goroutines)
	errs := make(chan error, goroutines)

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			host := "MUSIC.163.COM"
			if i%2 == 0 {
				host = "music.163.com"
			}

			cert, err := store.Fetch(host, gen)
			results <- cert

			errs <- err
		}(i)
	}

	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
	}

	for cert := range results {
		if cert != want {
			t.Fatalf("Fetch() certificate = %p, want %p", cert, want)
		}
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("generator calls = %d, want 1", got)
	}
}

func TestMemoryCertStoreSeparatesTrailingDotCertificates(t *testing.T) {
	store := newMemoryCertStore()
	bareCertificate := &tls.Certificate{}
	dottedCertificate := &tls.Certificate{}

	var calls atomic.Int32

	bare, err := store.Fetch("music.163.com", func() (*tls.Certificate, error) {
		calls.Add(1)
		return bareCertificate, nil
	})
	if err != nil {
		t.Fatalf("Fetch() bare host error = %v", err)
	}

	dotted, err := store.Fetch("MUSIC.163.COM.", func() (*tls.Certificate, error) {
		calls.Add(1)
		return dottedCertificate, nil
	})
	if err != nil {
		t.Fatalf("Fetch() dotted host error = %v", err)
	}

	if bare != bareCertificate || dotted != dottedCertificate {
		t.Fatalf("certificates were reused across trailing-dot hostnames: bare=%p dotted=%p", bare, dotted)
	}

	if _, err := store.Fetch("MUSIC.163.COM", func() (*tls.Certificate, error) {
		return nil, errors.New("bare certificate should be cached")
	}); err != nil {
		t.Fatalf("Fetch() cached bare host error = %v", err)
	}

	if _, err := store.Fetch("music.163.com.", func() (*tls.Certificate, error) {
		return nil, errors.New("dotted certificate should be cached")
	}); err != nil {
		t.Fatalf("Fetch() cached dotted host error = %v", err)
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("generator calls = %d, want 2", got)
	}
}

func TestMemoryCertStoreDoesNotCacheErrors(t *testing.T) {
	store := &memoryCertStore{}
	want := &tls.Certificate{}

	var calls atomic.Int32

	gen := func() (*tls.Certificate, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("temporary failure")
		}
		return want, nil
	}

	if _, err := store.Fetch("music.163.com", gen); err == nil {
		t.Fatal("first Fetch() unexpectedly succeeded")
	}

	cert, err := store.Fetch("music.163.com", gen)
	if err != nil {
		t.Fatalf("second Fetch() error = %v", err)
	}

	if cert != want {
		t.Fatalf("second Fetch() certificate = %p, want %p", cert, want)
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("generator calls = %d, want 2", got)
	}
}

func TestMemoryCertStoreIsBounded(t *testing.T) {
	store := newMemoryCertStore()

	var calls atomic.Int32

	gen := func() (*tls.Certificate, error) {
		calls.Add(1)
		return &tls.Certificate{}, nil
	}

	for i := range maxCachedCertificates + 20 {
		host := fmt.Sprintf("host-%d.netease.com", i)
		if _, err := store.Fetch(host, gen); err != nil {
			t.Fatal(err)
		}
	}

	store.mu.Lock()
	cached := len(store.certs)
	store.mu.Unlock()

	if cached != maxCachedCertificates {
		t.Fatalf("cached certificates = %d, want %d", cached, maxCachedCertificates)
	}

	if _, err := store.Fetch("host-0.netease.com", gen); err != nil {
		t.Fatal(err)
	}

	if got := calls.Load(); got != maxCachedCertificates+21 {
		t.Fatalf("generator calls = %d, want %d", got, maxCachedCertificates+21)
	}
}

func assertPermissions(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}

	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s permissions = %04o, want %04o", path, got, want)
	}
}

func testCertificateTemplate(notBefore, notAfter time.Time, isCA bool, keyUsage x509.KeyUsage) x509.Certificate {
	return x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
}

func writeTestCertificatePair(t *testing.T, certPath, keyPath string, template *x509.Certificate, mismatched bool) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	keyForFile := privateKey
	if mismatched {
		keyForFile, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("generate mismatched private key: %v", err)
		}
	}

	keyDER, err := x509.MarshalECPrivateKey(keyForFile)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}

	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), 0o600); err != nil {
		t.Fatalf("write certificate: %v", err)
	}

	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
}

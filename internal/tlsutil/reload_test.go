// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// pair writes a self-signed certificate for cn and returns its file paths.
func pair(t *testing.T, dir, cn string) (string, string) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		DNSNames:     []string{cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	if err != nil {
		t.Fatal(err)
	}
	kb, _ := x509.MarshalECPrivateKey(k)
	cf, kf := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	if err := os.WriteFile(cf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kf, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600); err != nil {
		t.Fatal(err)
	}
	return cf, kf
}

func subject(t *testing.T, c *tls.Certificate) string {
	t.Helper()
	x, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return x.Subject.CommonName
}

func TestReloaderPicksUpRotation(t *testing.T) {
	dir := t.TempDir()
	cf, kf := pair(t, dir, "one.test")
	r, err := NewReloader(cf, kf)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Now()
	r.now = func() time.Time { return clock }

	c, _ := r.GetCertificate(nil)
	if subject(t, c) != "one.test" {
		t.Fatalf("subject %s", subject(t, c))
	}

	pair(t, dir, "two.test") // rewrites the same paths
	future := time.Now().Add(time.Minute)
	_ = os.Chtimes(cf, future, future)
	_ = os.Chtimes(kf, future, future)

	// Inside the recheck interval the old certificate still serves.
	if c, _ = r.GetCertificate(nil); subject(t, c) != "one.test" {
		t.Fatalf("reloaded before the recheck interval: %s", subject(t, c))
	}
	clock = clock.Add(recheck + time.Second)
	if c, _ = r.GetCertificate(nil); subject(t, c) != "two.test" {
		t.Fatalf("not reloaded: %s", subject(t, c))
	}
	if r.Err() != nil {
		t.Fatalf("err %v", r.Err())
	}
}

func TestReloaderKeepsOldCertWhenReplacementIsBad(t *testing.T) {
	dir := t.TempDir()
	cf, kf := pair(t, dir, "one.test")
	r, _ := NewReloader(cf, kf)
	clock := time.Now()
	r.now = func() time.Time { return clock }

	if err := os.WriteFile(cf, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Minute)
	_ = os.Chtimes(cf, future, future)
	clock = clock.Add(recheck + time.Second)

	c, err := r.GetCertificate(nil)
	if err != nil || subject(t, c) != "one.test" {
		t.Fatalf("lost the working certificate: %v", err)
	}
	if r.Err() == nil {
		t.Fatal("a failed reload went unreported")
	}
}

func TestNewReloaderNeedsBothFiles(t *testing.T) {
	if _, err := NewReloader("a", ""); err == nil {
		t.Fatal("accepted a certificate without a key")
	}
	if _, err := NewReloader("/nonexistent.crt", "/nonexistent.key"); err == nil {
		t.Fatal("accepted missing files")
	}
}

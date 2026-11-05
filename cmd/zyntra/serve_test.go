// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testPair(t *testing.T) (string, string, *x509.CertPool) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	if err != nil {
		t.Fatal(err)
	}
	kb, _ := x509.MarshalECPrivateKey(k)
	dir := t.TempDir()
	cf, kf := filepath.Join(dir, "c.pem"), filepath.Join(dir, "k.pem")
	crt := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	_ = os.WriteFile(cf, crt, 0o600)
	_ = os.WriteFile(kf, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600)
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(crt)
	return cf, kf, pool
}

func TestConsoleServerPlainWithoutCertificate(t *testing.T) {
	srv, useTLS, err := newConsoleServer(":0", http.NotFoundHandler(), "", "")
	if err != nil || useTLS {
		t.Fatalf("plain: tls=%v err=%v", useTLS, err)
	}
	if srv.ReadTimeout == 0 || srv.WriteTimeout == 0 || srv.IdleTimeout == 0 || srv.ReadHeaderTimeout == 0 {
		t.Fatalf("missing timeouts: %+v", srv)
	}
	if srv.TLSConfig != nil {
		t.Fatal("plain server has a TLS config")
	}
}

func TestConsoleServerNeedsBothOrNeither(t *testing.T) {
	cf, kf, _ := testPair(t)
	if _, _, err := newConsoleServer(":0", nil, cf, ""); err == nil {
		t.Error("accepted a certificate without a key")
	}
	if _, _, err := newConsoleServer(":0", nil, "", kf); err == nil {
		t.Error("accepted a key without a certificate")
	}
	if _, _, err := newConsoleServer(":0", nil, "/no/such.crt", "/no/such.key"); err == nil {
		t.Error("accepted unreadable files")
	}
}

func TestConsoleServerServesTLS(t *testing.T) {
	cf, kf, pool := testPair(t)
	srv, useTLS, err := newConsoleServer("127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			http.Error(w, "plain", 500)
			return
		}
		w.Write([]byte("secure"))
	}), cf, kf)
	if err != nil || !useTLS {
		t.Fatalf("tls=%v err=%v", useTLS, err)
	}
	if srv.TLSConfig.MinVersion < tls.VersionTLS12 {
		t.Fatalf("min version %x", srv.TLSConfig.MinVersion)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.ServeTLS(ln, "", "")
	defer srv.Close()

	c := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
	resp, err := c.Get("https://" + ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}

	// An old protocol version is refused.
	old := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MaxVersion: tls.VersionTLS11}}}
	if r, err := old.Get("https://" + ln.Addr().String()); err == nil {
		r.Body.Close()
		t.Fatal("TLS 1.1 was accepted")
	}
}

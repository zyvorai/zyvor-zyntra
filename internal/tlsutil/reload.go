// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package tlsutil

import (
	"crypto/tls"
	"errors"
	"os"
	"sync"
	"time"
)

// Reloader serves a certificate from files and picks up a replacement when
// either file changes, so rotating a certificate needs no restart. If a
// replacement cannot be read or does not pair, the previous certificate keeps
// serving and the error is available from Err.
type Reloader struct {
	certFile, keyFile string

	mu      sync.Mutex
	cert    *tls.Certificate
	certMod time.Time
	keyMod  time.Time
	checked time.Time
	err     error
	now     func() time.Time
}

// recheck bounds how often the files are stat'ed.
const recheck = 5 * time.Second

// NewReloader loads the pair once and fails if it is unusable.
func NewReloader(certFile, keyFile string) (*Reloader, error) {
	if certFile == "" || keyFile == "" {
		return nil, errors.New("tls: both a certificate and a key file are needed")
	}
	r := &Reloader{certFile: certFile, keyFile: keyFile, now: time.Now}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Reloader) load() error {
	c, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return err
	}
	ci, err := os.Stat(r.certFile)
	if err != nil {
		return err
	}
	ki, err := os.Stat(r.keyFile)
	if err != nil {
		return err
	}
	r.cert, r.certMod, r.keyMod = &c, ci.ModTime(), ki.ModTime()
	r.checked = r.now()
	return nil
}

// GetCertificate is for tls.Config.GetCertificate.
func (r *Reloader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if now := r.now(); now.Sub(r.checked) >= recheck {
		r.checked = now
		ci, cerr := os.Stat(r.certFile)
		ki, kerr := os.Stat(r.keyFile)
		if cerr == nil && kerr == nil && (!ci.ModTime().Equal(r.certMod) || !ki.ModTime().Equal(r.keyMod)) {
			// Keep the old certificate if the new pair is half-written or
			// mismatched; the next check tries again.
			r.err = r.load()
		}
	}
	return r.cert, nil
}

// Err is the last reload failure, nil once a reload succeeds.
func (r *Reloader) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// Config is a server TLS config that serves from the reloader.
func (r *Reloader) Config() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: r.GetCertificate}
}

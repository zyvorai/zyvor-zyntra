// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/zyvorai/zyntra/internal/tlsutil"
)

// Console server limits. WriteTimeout is generous because a few handlers wait
// on an AI engine; the event stream renews its own deadline on every write.
const (
	consoleReadTimeout  = 60 * time.Second
	consoleWriteTimeout = 5 * time.Minute
	consoleIdleTimeout  = 2 * time.Minute
)

// newConsoleServer builds the console listener. With a certificate and key it
// serves HTTPS (TLS 1.2 or later) and re-reads them when they change; with
// neither it serves plain HTTP as before; with only one it is an error.
func newConsoleServer(addr string, h http.Handler, certFile, keyFile string) (*http.Server, bool, error) {
	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       consoleReadTimeout,
		WriteTimeout:      consoleWriteTimeout,
		IdleTimeout:       consoleIdleTimeout,
	}
	switch {
	case certFile == "" && keyFile == "":
		return srv, false, nil
	case certFile == "" || keyFile == "":
		return nil, false, fmt.Errorf("ZYNTRA_TLS_CERT and ZYNTRA_TLS_KEY must be set together")
	}
	rl, err := tlsutil.NewReloader(certFile, keyFile)
	if err != nil {
		return nil, false, fmt.Errorf("console tls: %w", err)
	}
	srv.TLSConfig = rl.Config()
	return srv, true, nil
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package main

import (
	"context"
	"io"
	"strings"
	"testing"
)

// ontologyCmd adds its own flags to the shared flag set. A name that the shared
// set already owns (such as -o) panics with "flag redefined" on every ontology
// subcommand, so run each through the real entry point.
func TestOntologySubcommandsDoNotRedefineSharedFlags(t *testing.T) {
	for _, sub := range []string{"suggest", "validate", "dump", "impact", "migrate"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ontology %s panicked: %v", sub, r)
				}
			}()
			_ = run(context.Background(), "ontology", []string{sub}, io.Discard)
		}()
	}
}

func TestOntologySuggestNeedsDriverAndDSNEnv(t *testing.T) {
	err := run(context.Background(), "ontology", []string{"suggest"}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "-dsn-env") {
		t.Fatalf("want a usage error naming -dsn-env, got %v", err)
	}
}

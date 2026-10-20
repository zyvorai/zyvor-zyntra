// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOverlayMissingIsEmptyMalformedIsError(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadOverlay(dir); err != nil {
		t.Fatalf("missing overlay must be empty, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "executive.yaml"), []byte("decisions: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOverlay(dir); err == nil {
		t.Fatal("malformed executive.yaml must be reported, not silently dropped")
	}
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package docs holds checks on the repository's documentation.
package docs

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const root = "../.."

var (
	fileRef = regexp.MustCompile("`([A-Za-z0-9_./-]+\\.(?:go|yml|yaml|md)|Dockerfile)`")
	status  = map[string]bool{"provided": true, "partial": true, "delegated": true, "gap": true}
)

// TestControlMappingReferencesExist keeps docs/CONTROL_MAPPING.md honest: every
// file it cites must exist, and every table row must carry a known status.
func TestControlMappingReferencesExist(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(root, "docs", "CONTROL_MAPPING.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)

	refs := fileRef.FindAllStringSubmatch(text, -1)
	if len(refs) < 10 {
		t.Fatalf("found only %d file references; the parser or the document broke", len(refs))
	}
	for _, m := range refs {
		p := m[1]
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			// Some references are relative to a sibling directory.
			if _, err2 := os.Stat(filepath.Join(root, "docs", p)); err2 != nil {
				t.Errorf("CONTROL_MAPPING.md cites %s, which does not exist", p)
			}
		}
	}

	rows := 0
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| Control") || strings.HasPrefix(line, "|---") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			continue
		}
		rows++
		if s := strings.TrimSpace(cells[2]); !status[s] {
			t.Errorf("row %q has unknown status %q", strings.TrimSpace(cells[1]), s)
		}
	}
	if rows < 30 {
		t.Errorf("only %d control rows parsed", rows)
	}
}

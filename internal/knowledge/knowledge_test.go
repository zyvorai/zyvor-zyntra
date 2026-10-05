// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package knowledge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var viewer = Reader{Roles: []string{"viewer"}}
var admin = Reader{Admin: true, Roles: []string{"admin"}}

func fixture(t *testing.T) *Store {
	t.Helper()
	s, e := Open("")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func put(t *testing.T, s *Store, id, visibility, tenant, text string) Document {
	t.Helper()
	d, e := s.Put(Document{ID: id, Title: id, Text: text, Visibility: visibility, Tenant: tenant}, 0, "tester")
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestACLBeforeRetrievalAndRanking(t *testing.T) {
	s := fixture(t)
	put(t, s, "shared", "shared", "", "Inventory reorder before stock reaches zero.")
	put(t, s, "provider", "provider", "", "Inventory root password SECRET")
	put(t, s, "alpha", "tenant", "alpha", "Inventory alpha instructions")
	put(t, s, "beta", "tenant", "beta", "Inventory beta SECRET")
	r := Reader{Tenant: "alpha", Roles: []string{"viewer"}}
	hits, e := s.Search("inventory", r, 10)
	if e != nil {
		t.Fatal(e)
	}
	if len(hits) != 2 {
		t.Fatalf("%+v", hits)
	}
	for _, c := range hits {
		if strings.Contains(c.Excerpt, "SECRET") || c.Document == "beta" {
			t.Fatal("hidden text leaked")
		}
	}
	if _, e = s.Get("beta", 0, r); !errors.Is(e, ErrNotFound) {
		t.Fatal("hidden doc is distinguishable")
	}
	// Invisible additions cannot affect the visible corpus's score.
	first := hits[0].Score
	put(t, s, "hidden", "tenant", "beta", strings.Repeat("inventory ", 500))
	hits, _ = s.Search("inventory", r, 10)
	if hits[0].Score != first {
		t.Fatal("hidden document changed rank")
	}
	list, _ := s.List(r)
	for _, d := range list {
		if d.Text != "" {
			t.Fatal("list exposes bodies")
		}
	}
}
func TestRoleRestrictionsAndACLChangesInvalidateOldAccess(t *testing.T) {
	s := fixture(t)
	d := put(t, s, "restricted", "shared", "", "Emergency inventory procedure")
	d.Roles = []string{"approver"}
	d, e := s.Put(d, 1, "admin")
	if e != nil {
		t.Fatal(e)
	}
	hits, _ := s.Search("inventory", viewer, 5)
	if len(hits) != 0 {
		t.Fatal("role denied docs found")
	}
	if _, e = s.Get(d.ID, 1, viewer); !errors.Is(e, ErrNotFound) {
		t.Fatal("old revision bypassed current ACL")
	}
	d.Roles = []string{}
	_, e = s.Put(d, 2, "admin")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(d.ID, 2, viewer); !errors.Is(e, ErrNotFound) {
		t.Fatal("old revision ACL ignored")
	}
}
func TestVersionsConflictDeleteAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docs.sqlite")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	d := put(t, s, "runbook", "provider", "", "First instructions")
	originalHash := d.Hash
	d.Text = "Second instructions"
	d, e = s.Put(d, 1, "another-user")
	if e != nil || d.Version != 2 || d.Hash == originalHash {
		t.Fatalf("%+v %v", d, e)
	}
	if _, e = s.Put(d, 1, "stale"); !errors.Is(e, ErrConflict) {
		t.Fatal("lost update accepted")
	}
	s.Close()
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	old, e := s.Get(d.ID, 1, admin)
	if e != nil || old.Text != "First instructions" || old.Hash != originalHash {
		t.Fatalf("%+v %v", old, e)
	}
	if e = s.Delete(d.ID, 1); !errors.Is(e, ErrConflict) {
		t.Fatal("stale delete accepted")
	}
	if e = s.Delete(d.ID, 2); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(d.ID, 1, admin); !errors.Is(e, ErrNotFound) {
		t.Fatal("deleted evidence still readable")
	}
	var revisions int
	s.db.QueryRow("SELECT count(*) FROM revisions").Scan(&revisions)
	if revisions != 0 {
		t.Fatal("deleted text retained")
	}
	if _, e = s.Put(Document{ID: d.ID, Title: "reuse", Text: "new"}, 0, "x"); !errors.Is(e, ErrConflict) {
		t.Fatal("citation identifier reused")
	}
}
func TestLineChunksAndUnicodeEvidence(t *testing.T) {
	s := fixture(t)
	text := "Heading\n" + strings.Repeat("ordinary line\n", 31) + "Réorder inventory before zero.\n" + strings.Repeat("界", 2100)
	d := put(t, s, "unicode", "shared", "", text)
	hits, e := s.Search("réorder", viewer, 5)
	if e != nil || len(hits) != 1 {
		t.Fatalf("%+v %v", hits, e)
	}
	c := hits[0]
	if c.StartLine > 33 || c.EndLine < 33 || c.Hash != d.Hash {
		t.Fatalf("bad provenance %+v", c)
	}
	cs := chunks(d)
	var rebuilt strings.Builder
	for _, c := range cs {
		if len([]rune(c.text)) > 2000 {
			t.Fatal("chunk cap exceeded")
		}
		rebuilt.WriteString(c.text)
	}
	if rebuilt.String() != text {
		t.Fatal("chunking lost text")
	}
}
func TestValidationNoMatchAndRevisionCap(t *testing.T) {
	s := fixture(t)
	for _, d := range []Document{{ID: "../x", Title: "x", Text: "x"}, {ID: "x", Title: "x", Text: strings.Repeat("x", MaxText+1)}, {ID: "x", Title: "x", Text: "x", Visibility: "tenant"}, {ID: "x", Title: "x", Text: "x", Roles: []string{"exec"}}} {
		if _, e := s.Put(d, 0, "a"); !errors.Is(e, ErrInvalid) {
			t.Fatalf("accepted %+v", d)
		}
	}
	d := put(t, s, "history", "shared", "", "inventory")
	for i := 1; i < 12; i++ {
		var e error
		d, e = s.Put(d, d.Version, "a")
		if e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.Get(d.ID, 1, viewer); !errors.Is(e, ErrNotFound) {
		t.Fatal("old revision retained")
	}
	hits, e := s.Search("unmatched", viewer, 5)
	if e != nil || len(hits) != 0 {
		t.Fatalf("%v %v", hits, e)
	}
	for _, limit := range []int{0, 11} {
		if _, e = s.Search("inventory", viewer, limit); !errors.Is(e, ErrInvalid) {
			t.Fatal("invalid limit")
		}
	}
}
func TestConcurrentVersionedWritesHaveOneWinner(t *testing.T) {
	s := fixture(t)
	d := put(t, s, "concurrent", "shared", "", "inventory")
	var wg sync.WaitGroup
	wins := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.Put(d, 1, "a"); wins <- e == nil }()
	}
	wg.Wait()
	close(wins)
	n := 0
	for ok := range wins {
		if ok {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d writers won", n)
	}
}

func TestKnowledgeFileIsPrivateAndTamperedBodyFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge.sqlite")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private file %v %v", info, e)
	}
	d := put(t, s, "private", "provider", "", "inventory instructions")
	d.Text = "tampered text"
	body, _ := json.Marshal(d)
	s.db.Exec("UPDATE documents SET body=? WHERE id=?", body, d.ID)
	if _, e = s.Get(d.ID, 0, admin); e == nil {
		t.Fatal("tampered hash accepted")
	}
	if _, e = s.Search("tampered", admin, 5); e == nil {
		t.Fatal("tampered evidence retrieved")
	}
}

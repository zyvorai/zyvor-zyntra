package inputs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJournalPersistenceIdempotencyAndLateEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inputs.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	body := map[string]any{"value": 1.0}
	r, err := s.IngestEvent("x", body, "operator", "key", time.Time{}, now, "")
	if err != nil || r.Sequence != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	body["value"] = 9.0
	r, err = s.IngestEvent("x", map[string]any{"value": 1.0}, "operator", "key", time.Time{}, now.Add(time.Second), "")
	if err != nil || !r.Duplicate || r.Sequence != 1 {
		t.Fatalf("retry %+v %v", r, err)
	}
	if _, err = s.IngestEvent("x", body, "operator", "key", time.Time{}, now, ""); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	r, err = s.IngestEvent("x", body, "operator", "late", now.Add(-time.Hour), now, "")
	if err != nil || r.Status != "late" {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = s.IngestEvent("x", body, "operator", "bad", now.Add(time.Second), now, "bad contract")
	if err != nil || r.Status != "quarantined" {
		t.Fatalf("%+v %v", r, err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	latest, _ := s.Webhook("x")
	if latest.Body.(map[string]any)["value"] != 1.0 || !latest.At.Equal(now) {
		t.Fatalf("late/quarantined value replaced live: %+v", latest)
	}
	page := s.Events("x", 0, 2)
	if len(page.Events) != 2 || page.Next != 2 {
		t.Fatal(page)
	}
	page.Events[0].Document.Body.(map[string]any)["value"] = 22.0
	latest.Body.(map[string]any)["value"] = 33.0
	again, _ := s.Webhook("x")
	if again.Body.(map[string]any)["value"] != 1.0 {
		t.Fatal("caller mutated store")
	}
}
func TestFailedJournalCommitDoesNotChangeMemory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent", "inputs.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "parent"), []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.Ingest("x", 1, "test", time.Now()); err == nil {
		t.Fatal("expected persist failure")
	}
	if _, ok := s.Webhook("x"); ok || len(s.Events("x", 0, 100).Events) != 0 {
		t.Fatal("uncommitted data became visible")
	}
}
func TestJournalRetentionAndChannelIsolation(t *testing.T) {
	s, _ := Open("")
	now := time.Now()
	for i := 0; i < MaxEvents+2; i++ {
		if _, err := s.IngestEvent("x", i, "test", fmt.Sprint(i), now.Add(time.Duration(i)*time.Millisecond), now, ""); err != nil {
			t.Fatal(err)
		}
	}
	p := s.Events("x", 1, 200)
	if !p.Truncated || p.Oldest != 3 || len(p.Events) != 200 {
		t.Fatalf("%+v", p)
	}
	if len(s.Events("other", 0, 100).Events) != 0 {
		t.Fatal("channel leak")
	}
}

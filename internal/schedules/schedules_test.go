// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package schedules

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDue(t *testing.T) {
	now := time.Now()
	s := Schedule{EveryMinutes: 15, Enabled: true}
	if !Due(s, now) {
		t.Fatal("never-run schedule should be due")
	}
	s.LastRun = now.Add(-5 * time.Minute)
	if Due(s, now) {
		t.Fatal("recent run should not be due")
	}
	s.LastRun = now.Add(-20 * time.Minute)
	if !Due(s, now) {
		t.Fatal("old run should be due")
	}
}

func TestStore(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "schedules.json"))
	if err != nil {
		t.Fatal(err)
	}
	sc, err := st.Put(Schedule{Name: "nightly", EveryMinutes: 60, Enabled: true})
	if err != nil || sc.ID == "" {
		t.Fatalf("put: %+v %v", sc, err)
	}
	if !Due(sc, time.Now()) {
		t.Fatal("new schedule should be due")
	}
	if err := st.MarkRun(sc.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, ok := st.Get(sc.ID)
	if !ok || got.LastRun.IsZero() {
		t.Fatalf("mark run: %+v", got)
	}
	st2, err := Open(filepath.Join(dir, "schedules.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(st2.List()) != 1 {
		t.Fatalf("reload: %d", len(st2.List()))
	}
}

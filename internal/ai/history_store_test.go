// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package ai

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryMigrationCommitAndRestart(t *testing.T) {
	dir := t.TempDir()
	m := load(t)
	at := time.Now().UTC().Add(-time.Hour)
	b, _ := json.Marshal(map[string][]Point{"cost": {{T: at, V: 100}}})
	if err := os.WriteFile(filepath.Join(dir, "history.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	h, err := OpenHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Series("cost")) != 1 {
		t.Fatal("legacy import missing")
	}
	m.KPIs[2].Value = 120
	h.Record(m, at.Add(time.Minute))
	h.Record(m, at.Add(70*time.Second))
	if h.PersistenceError() != "" || len(h.Series("cost")) != 2 {
		t.Fatalf("%v %s", h.Series("cost"), h.PersistenceError())
	}
	if err = h.Close(); err != nil {
		t.Fatal(err)
	}
	h, err = OpenHistory(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if s := h.Series("cost"); len(s) != 2 || s[1].V != 120 {
		t.Fatalf("restart: %v", s)
	}
}
func TestHistoryRetentionSkipAndFailureAtomicity(t *testing.T) {
	h, err := OpenHistory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	h.max = 3
	m := load(t)
	at := time.Now().UTC()
	for i := 0; i < 5; i++ {
		h.Record(m, at.Add(time.Duration(i)*time.Minute), map[string]bool{"wait": true})
	}
	if len(h.Series("wait")) != 0 || len(h.Series("cost")) != 3 {
		t.Fatal("skip/retention")
	}
	var count int
	if err = h.db.QueryRow("SELECT count(*) FROM samples WHERE kpi='cost'").Scan(&count); err != nil || count != 3 {
		t.Fatalf("disk retention %d %v", count, err)
	}
	h.db.Close()
	h.Record(m, at.Add(6*time.Minute))
	if h.PersistenceError() == "" || len(h.Series("cost")) != 3 {
		t.Fatal("failed commit advanced history")
	}
}
func TestHistoryCleansInvalidOutOfOrderAndDuplicatePoints(t *testing.T) {
	h := NewHistory(3)
	at := time.Now().UTC()
	h.Restore(map[string][]Point{"x": {{at.Add(time.Minute), 2}, {at, 1}, {at, 3}, {at.Add(2 * time.Minute), math.NaN()}}})
	pts := h.Series("x")
	if len(pts) != 2 || pts[0].V != 3 || pts[1].V != 2 {
		t.Fatalf("%+v", pts)
	}
	m := load(t)
	h.Record(m, at)
	h.Record(m, at.Add(-time.Minute))
	h.Record(m, at)
	if len(h.Series("cost")) != 1 {
		t.Fatal("non-increasing samples recorded")
	}
}
func TestCorruptLegacyHistoryIsReported(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "history.json"), []byte("bad json"), 0600)
	if h, err := OpenHistory(dir); err == nil {
		h.Close()
		t.Fatal("silent corrupt migration")
	}
}

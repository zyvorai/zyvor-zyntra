// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package watch

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var start = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func rule() Rule {
	return Rule{ID: "latency", Name: "High latency", Metric: "latency", Tenant: "alpha", Unit: "ms", Operator: "above", Threshold: 100, ForSeconds: 60, ClearSeconds: 30, MaxGapSeconds: 40, Enabled: true}
}
func store(t *testing.T) *Store {
	t.Helper()
	s, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	_, err = s.Put(rule(), 0, "admin", start)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func observe(t *testing.T, s *Store, seconds int, value float64) {
	t.Helper()
	at := start.Add(time.Duration(seconds) * time.Second)
	if err := s.Evaluate(map[string]Observation{"latency": {At: at, Value: value, Valid: true, Tenant: "alpha"}}, at); err != nil {
		t.Fatal(err)
	}
}
func TestSustainedLifecycleAcknowledgementAndRecovery(t *testing.T) {
	s := store(t)
	observe(t, s, 0, 110)
	observe(t, s, 30, 120)
	if len(s.View("").Incidents) != 0 {
		t.Fatal("opened early")
	}
	observe(t, s, 60, 130)
	v := s.View("alpha")
	if len(v.Incidents) != 1 || v.Runtime["latency"].Status != "open" {
		t.Fatal(v)
	}
	in := v.Incidents[0]
	ack, err := s.Acknowledge(in.ID, 1, "alice", "Investigating queue", "alpha", start.Add(time.Minute))
	if err != nil || ack.Status != "acknowledged" || ack.Version != 2 {
		t.Fatal(ack, err)
	}
	if _, err = s.Acknowledge(in.ID, 1, "bob", "Taking over", "alpha", start); !errors.Is(err, ErrConflict) {
		t.Fatal("lost acknowledgement guard", err)
	}
	observe(t, s, 90, 90)
	if s.View("").Incidents[0].Status != "acknowledged" {
		t.Fatal("closed before recovery duration")
	}
	observe(t, s, 120, 90)
	v = s.View("")
	if v.Incidents[0].Status != "resolved" || v.Incidents[0].AckBy != "alice" || v.Incidents[0].ResolveReason != "threshold recovered" || v.Incidents[0].Value != 90 {
		t.Fatal(v)
	}
	observe(t, s, 150, 110)
	observe(t, s, 180, 110)
	observe(t, s, 210, 110)
	v = s.View("")
	if len(v.Incidents) != 2 || v.Incidents[0].ID == in.ID {
		t.Fatal("recurrence not separate", v)
	}
}
func TestMissingGapsDuplicateAndInvalidSamplesResetTimers(t *testing.T) {
	for _, kind := range []string{"missing", "invalid", "wrong-tenant", "gap"} {
		t.Run(kind, func(t *testing.T) {
			s := store(t)
			observe(t, s, 0, 110)
			observe(t, s, 30, 110)
			at := start.Add(40 * time.Second)
			o := map[string]Observation{}
			if kind == "invalid" {
				o["latency"] = Observation{At: at, Value: math.NaN(), Valid: true, Tenant: "alpha"}
			}
			if kind == "wrong-tenant" {
				o["latency"] = Observation{At: at, Value: 110, Valid: true, Tenant: "beta"}
			}
			if kind != "gap" {
				if err := s.Evaluate(o, at); err != nil {
					t.Fatal(err)
				}
			}
			observe(t, s, 90, 110)
			if len(s.View("").Incidents) != 0 {
				t.Fatal("unknown interval counted")
			}
		})
	}
	s := store(t)
	observe(t, s, 0, 110)
	// Replaying the same observation with a later clock cannot satisfy duration.
	if err := s.Evaluate(map[string]Observation{"latency": {At: start, Value: 110, Valid: true, Tenant: "alpha"}}, start.Add(60*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(s.View("").Incidents) != 0 {
		t.Fatal("replayed sample opened incident")
	}
	// Future samples do not count either.
	if err := s.Evaluate(map[string]Observation{"latency": {At: start.Add(time.Hour), Value: 110, Valid: true, Tenant: "alpha"}}, start); err != nil {
		t.Fatal(err)
	}
	if s.View("").Runtime["latency"].Status != "unavailable" {
		t.Fatal("future data accepted")
	}
}
func TestStaleNeverClosesOpenIncidentAndRestartPreservesAck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	r := rule()
	r.ForSeconds = 0
	r.MaxGapSeconds = 120
	if _, err = s.Put(r, 0, "admin", start); err != nil {
		t.Fatal(err)
	}
	observe(t, s, 0, 110)
	in := s.View("").Incidents[0]
	if _, err = s.Acknowledge(in.ID, 1, "alice", "On it", "alpha", start); err != nil {
		t.Fatal(err)
	}
	observe(t, s, 10, 90)
	if err = s.Evaluate(nil, start.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	if s.View("").Incidents[0].Status != "acknowledged" {
		t.Fatal("stale data closed incident")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	v := s.View("")
	if v.Incidents[0].Status != "acknowledged" || v.Runtime[r.ID].Clearing != nil || v.Runtime[r.ID].LastAt != nil {
		t.Fatal("restart reused duration", v)
	}
	observe(t, s, 120, 90)
	if s.View("").Incidents[0].Status == "resolved" {
		t.Fatal("downtime counted as recovery")
	}
	observe(t, s, 150, 90)
	if s.View("").Incidents[0].Status != "resolved" {
		t.Fatal("recovery not recorded")
	}
	mode, _ := os.Stat(path)
	if mode.Mode().Perm() != 0600 {
		t.Fatal("database permissions", mode.Mode())
	}
}
func TestRuleVersionScopeDisableDeleteAndSnapshotIsolation(t *testing.T) {
	s := store(t)
	r := rule()
	r.ForSeconds = 0
	r.Version = 999
	r.UpdatedBy = "forged"
	saved, err := s.Put(r, 1, "admin", start)
	if err != nil || saved.Version != 2 || saved.UpdatedBy != "admin" {
		t.Fatal(saved, err)
	}
	observe(t, s, 0, 110)
	r.Enabled = false
	if _, err = s.Put(r, 2, "admin", start); err != nil {
		t.Fatal(err)
	}
	if s.View("").Incidents[0].ResolveReason != "rule changed" {
		t.Fatal("disabled rule left active")
	}
	if _, err = s.Put(r, 2, "admin", start); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	r.Tenant = "beta"
	if _, err = s.Put(r, 3, "admin", start); !errors.Is(err, ErrInvalid) {
		t.Fatal("scope migrated", err)
	}
	if len(s.View("beta").Rules) != 0 || len(s.View("beta").Incidents) != 0 || len(s.View("beta").Events) != 0 {
		t.Fatal("tenant leak")
	}
	v := s.View("")
	v.Rules[0].Name = "changed"
	v.Runtime["latency"] = Runtime{}
	if s.View("").Rules[0].Name == "changed" {
		t.Fatal("returned mutable state")
	}
	if err = s.Delete("latency", 3, "admin", start); err != nil {
		t.Fatal(err)
	}
	if len(s.View("").Rules) != 0 || len(s.View("").Incidents) != 1 {
		t.Fatal("deletion lost incident history")
	}
}
func TestPersistenceFailureLeavesDurableStateAndResetsContinuity(t *testing.T) {
	s := store(t)
	r := rule()
	r.MaxGapSeconds = 600
	if _, err := s.Put(r, 1, "admin", start); err != nil {
		t.Fatal(err)
	}
	observe(t, s, 0, 110)
	_, err := s.db.Exec(`CREATE TRIGGER deny_write BEFORE INSERT ON watch_state BEGIN SELECT RAISE(FAIL,'test failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Evaluate(nil, start.Add(30*time.Second)); err == nil {
		t.Fatal("write failure hidden")
	}
	if s.View("").Runtime["latency"].Pending == nil || s.View("").PersistenceError == "" {
		t.Fatal("failed write modified durable state")
	}
	if _, err = s.db.Exec(`DROP TRIGGER deny_write`); err != nil {
		t.Fatal(err)
	}
	observe(t, s, 60, 110)
	if len(s.View("").Incidents) != 0 {
		t.Fatal("failed persistence interval counted")
	}
	observe(t, s, 90, 110)
	observe(t, s, 120, 110)
	if len(s.View("").Incidents) != 1 {
		t.Fatal("did not recover")
	}
}
func TestRuleValidationBelowAndEquality(t *testing.T) {
	for _, mutate := range []func(*Rule){func(r *Rule) { r.Operator = "sql" }, func(r *Rule) { r.Threshold = math.Inf(1) }, func(r *Rule) { r.ForSeconds = -1 }, func(r *Rule) { r.ClearSeconds = 86401 }, func(r *Rule) { r.MaxGapSeconds = 0 }} {
		r := rule()
		mutate(&r)
		if r.Validate() == nil {
			t.Fatal("invalid rule accepted")
		}
	}
	s := store(t)
	r := rule()
	r.Operator = "below"
	r.ForSeconds = 0
	r.ClearSeconds = 0
	if _, err := s.Put(r, 1, "admin", start); err != nil {
		t.Fatal(err)
	}
	observe(t, s, 0, 100)
	if len(s.View("").Incidents) != 0 {
		t.Fatal("equality breached")
	}
	observe(t, s, 1, 90)
	observe(t, s, 2, 100)
	if s.View("").Incidents[0].Status != "resolved" {
		t.Fatal("below did not recover at equality")
	}
}

func TestRetentionKeepsActiveIncidentsAndBoundsEvents(t *testing.T) {
	s := store(t)
	r := rule()
	r.ForSeconds = 0
	r.ClearSeconds = 0
	if _, err := s.Put(r, 1, "admin", start); err != nil {
		t.Fatal(err)
	}
	observe(t, s, 0, 110)
	s.mu.Lock()
	active := s.data.Incidents[0]
	s.data.Incidents = nil
	for i := 0; i < MaxIncidents-1; i++ {
		in := active
		in.ID = fmt.Sprintf("archived-%d", i)
		in.Status = "resolved"
		in.ResolvedAt = &start
		s.data.Incidents = append(s.data.Incidents, in)
	}
	s.data.Incidents = append(s.data.Incidents, active)
	s.data.Events = nil
	for i := 0; i < MaxEvents; i++ {
		s.data.Events = append(s.data.Events, Event{Sequence: int64(i + 1), At: start, Rule: r.ID, Tenant: r.Tenant, Kind: "opened", By: "zyntra"})
	}
	s.data.Next = MaxEvents + 10
	s.mu.Unlock()
	observe(t, s, 1, 90)
	observe(t, s, 2, 110)
	v := s.View("")
	if len(v.Incidents) != MaxIncidents || len(v.Events) != MaxEvents || v.Incidents[0].Status != "open" {
		t.Fatal("retention bounds or active incident lost")
	}
	for _, in := range v.Incidents {
		if in.ID == "archived-0" {
			t.Fatal("oldest resolved not evicted")
		}
	}
}
func TestCapacityAndConcurrentCompareAndSwap(t *testing.T) {
	s := store(t)
	result := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			r := rule()
			r.Name = fmt.Sprintf("edit-%d", i)
			_, err := s.Put(r, 1, "admin", start)
			result <- err
		}(i)
	}
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-result
		if err == nil {
			successes++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("non-atomic version guard")
	}
	for i := 1; i < MaxRules; i++ {
		r := rule()
		r.ID = fmt.Sprintf("rule-%d", i)
		if _, err := s.Put(r, 0, "admin", start); err != nil {
			t.Fatal(err)
		}
	}
	r := rule()
	r.ID = "overflow"
	if _, err := s.Put(r, 0, "admin", start); !errors.Is(err, ErrCapacity) {
		t.Fatal("rule cap", err)
	}
}
func TestCorruptStateAndUnsupportedSchemaFailStartup(t *testing.T) {
	for _, body := range []string{`not-json`, `{"schema":99,"rules":{},"runtime":{},"incidents":[],"events":[]}`} {
		path := filepath.Join(t.TempDir(), "watch.sqlite")
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.db.Exec(`INSERT INTO watch_state(id,body) VALUES(1,?)`, body); err != nil {
			t.Fatal(err)
		}
		s.Close()
		if reopened, err := Open(path); err == nil {
			reopened.Close()
			t.Fatal("corrupt state accepted")
		}
	}
}

func TestSecondStoreCannotOverwriteNewerDurableState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch.sqlite")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err = first.Put(rule(), 0, "first", start); err != nil {
		t.Fatal(err)
	}
	r := rule()
	r.Name = "overwrite"
	if _, err = second.Put(r, 0, "second", start); !errors.Is(err, ErrConflict) {
		t.Fatal("stale database state overwritten", err)
	}
	if len(second.View("").Rules) != 0 || second.View("").PersistenceError == "" {
		t.Fatal("failed write updated memory")
	}
	third, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	if third.View("").Rules[0].UpdatedBy != "first" {
		t.Fatal("durable write lost")
	}
}

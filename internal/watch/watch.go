// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
// Package watch records threshold incidents. It never runs actions or sends messages.
package watch

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("watch item not found")
var ErrConflict = errors.New("version changed; reload before editing")
var ErrInvalid = errors.New("invalid watch rule")
var ErrCapacity = errors.New("watch capacity exceeded")

const MaxRules = 100
const MaxIncidents = 500
const MaxEvents = 2000

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,79}$`)

type Rule struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Metric        string    `json:"metric"`
	MetricName    string    `json:"metric_name"`
	Unit          string    `json:"unit"`
	Tenant        string    `json:"tenant,omitempty"`
	Operator      string    `json:"operator"`
	Threshold     float64   `json:"threshold"`
	ForSeconds    int       `json:"for_seconds"`
	ClearSeconds  int       `json:"clear_seconds"`
	MaxGapSeconds int       `json:"max_gap_seconds"`
	Enabled       bool      `json:"enabled"`
	Version       int       `json:"version"`
	UpdatedAt     time.Time `json:"updated_at"`
	UpdatedBy     string    `json:"updated_by"`
}

func (r Rule) Validate() error {
	if !idPattern.MatchString(r.ID) || strings.TrimSpace(r.Name) == "" || len(r.Name) > 120 || r.Metric == "" || !finite(r.Threshold) || r.ForSeconds < 0 || r.ForSeconds > 86400 || r.ClearSeconds < 0 || r.ClearSeconds > 86400 || r.MaxGapSeconds < 1 || r.MaxGapSeconds > 3600 || (r.Operator != "above" && r.Operator != "below") {
		return ErrInvalid
	}
	return nil
}

type Runtime struct {
	Status    string     `json:"status"`
	Active    string     `json:"active_incident,omitempty"`
	LastAt    *time.Time `json:"last_observed_at,omitempty"`
	LastValue *float64   `json:"last_value,omitempty"`
	Pending   *time.Time `json:"pending_since,omitempty"`
	Clearing  *time.Time `json:"clearing_since,omitempty"`
}
type Incident struct {
	ID            string     `json:"id"`
	Rule          Rule       `json:"rule"` // immutable rule snapshot, including scope
	Version       int        `json:"version"`
	Status        string     `json:"status"` // open | acknowledged | resolved
	OpenedAt      time.Time  `json:"opened_at"`
	ObservedAt    time.Time  `json:"observed_at"`
	Value         float64    `json:"value"`
	AckAt         *time.Time `json:"acknowledged_at,omitempty"`
	AckBy         string     `json:"acknowledged_by,omitempty"`
	AckNote       string     `json:"acknowledgement_note,omitempty"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	ResolveReason string     `json:"resolve_reason,omitempty"`
}
type Event struct {
	Sequence int64     `json:"sequence"`
	At       time.Time `json:"at"`
	Rule     string    `json:"rule"`
	Tenant   string    `json:"tenant,omitempty"`
	Incident string    `json:"incident,omitempty"`
	Kind     string    `json:"kind"`
	By       string    `json:"by"`
	Note     string    `json:"note,omitempty"`
}
type View struct {
	Rules            []Rule             `json:"rules"`
	Runtime          map[string]Runtime `json:"runtime"`
	Incidents        []Incident         `json:"incidents"`
	Events           []Event            `json:"events"`
	PersistenceError string             `json:"persistence_error,omitempty"`
}
type state struct {
	Schema    int                `json:"schema"`
	Next      int64              `json:"next"`
	Rules     map[string]Rule    `json:"rules"`
	Runtime   map[string]Runtime `json:"runtime"`
	Incidents []Incident         `json:"incidents"`
	Events    []Event            `json:"events"`
}
type Store struct {
	mu           sync.Mutex
	db           *sql.DB
	data         state
	persistError string
	needsReset   bool
	revision     int64
}

func Open(path string) (*Store, error) {
	dsn := ":memory:"
	if path != "" {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		path = absolute
		if err = os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, err
		}
		file.Close()
		if err = os.Chmod(path, 0600); err != nil {
			return nil, err
		}
		u := url.URL{Scheme: "file", Path: path}
		dsn = u.String() + "?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*Store, error) { db.Close(); return nil, err }
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS watch_state(id INTEGER PRIMARY KEY CHECK(id=1),body BLOB NOT NULL,revision INTEGER NOT NULL DEFAULT 0)`); err != nil {
		return fail(err)
	}
	s := &Store{db: db, data: state{Schema: 1, Rules: map[string]Rule{}, Runtime: map[string]Runtime{}, Incidents: []Incident{}, Events: []Event{}}}
	var body []byte
	err = db.QueryRow(`SELECT body,revision FROM watch_state WHERE id=1`).Scan(&body, &s.revision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fail(err)
	}
	if err == nil {
		if err = json.Unmarshal(body, &s.data); err != nil {
			return fail(fmt.Errorf("corrupt watch state: %w", err))
		}
		if s.revision < 0 || s.data.Schema != 1 || s.data.Rules == nil || s.data.Runtime == nil || len(s.data.Rules) > MaxRules || len(s.data.Incidents) > MaxIncidents || len(s.data.Events) > MaxEvents {
			return fail(fmt.Errorf("invalid watch state"))
		}
		for id, r := range s.data.Rules {
			if r.Validate() != nil || id != r.ID || r.Version < 1 {
				return fail(fmt.Errorf("invalid persisted watch rule"))
			}
		}
		// Unknown downtime never counts towards either duration; active incidents survive.
		for id, r := range s.data.Rules {
			rt := s.data.Runtime[id]
			rt.LastAt = nil
			rt.Pending = nil
			rt.Clearing = nil
			rt.Status = "waiting"
			if rt.Active != "" {
				rt.Status = "unavailable"
			}
			if !r.Enabled {
				rt.Status = "disabled"
			}
			s.data.Runtime[id] = rt
		}
		if err = s.commit(s.data); err != nil {
			return fail(err)
		}
	}
	return s, nil
}
func (s *Store) Close() error { s.mu.Lock(); defer s.mu.Unlock(); return s.db.Close() }
func clone[T any](v T) T      { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }
func (s *Store) commit(next state) error {
	body, err := json.Marshal(next)
	if err == nil {
		var result sql.Result
		result, err = s.db.Exec(`INSERT INTO watch_state(id,body,revision) VALUES(1,?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body,revision=excluded.revision WHERE watch_state.revision=?`, body, s.revision+1, s.revision)
		if err == nil {
			count, countErr := result.RowsAffected()
			if countErr != nil {
				err = countErr
			} else if count != 1 {
				err = ErrConflict
			}
		}
	}
	if err != nil {
		s.persistError = "Watch persistence failed; last durable state retained."
		s.needsReset = true
		return err
	}
	s.data = next
	s.revision++
	s.persistError = ""
	return nil
}
func event(st *state, at time.Time, r Rule, incident, kind, by, note string) {
	st.Next++
	st.Events = append(st.Events, Event{st.Next, at, r.ID, r.Tenant, incident, kind, by, note})
	if len(st.Events) > MaxEvents {
		st.Events = st.Events[len(st.Events)-MaxEvents:]
	}
}
func resolve(st *state, id, reason, by string, now time.Time) {
	for i := range st.Incidents {
		in := &st.Incidents[i]
		if in.ID == id && in.Status != "resolved" {
			in.Status = "resolved"
			in.Version++
			in.ResolvedAt = &now
			in.ResolveReason = reason
			event(st, now, in.Rule, id, "resolved", by, reason)
			return
		}
	}
}
func (s *Store) Put(r Rule, expected int, by string, now time.Time) (Rule, error) {
	r.Name = strings.TrimSpace(r.Name)
	if r.Validate() != nil {
		return Rule{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.data)
	old, exists := next.Rules[r.ID]
	if exists && old.Version != expected || !exists && expected != 0 {
		return Rule{}, ErrConflict
	}
	if exists && r.Tenant != old.Tenant {
		return Rule{}, ErrInvalid
	}
	if !exists && len(next.Rules) >= MaxRules {
		return Rule{}, ErrCapacity
	}
	if rt := next.Runtime[r.ID]; rt.Active != "" {
		resolve(&next, rt.Active, "rule changed", by, now)
	}
	r.Version = old.Version + 1
	r.UpdatedAt = now.UTC()
	r.UpdatedBy = by
	next.Rules[r.ID] = r
	status := "waiting"
	if !r.Enabled {
		status = "disabled"
	}
	next.Runtime[r.ID] = Runtime{Status: status}
	event(&next, now, r, "", "rule-saved", by, "")
	if err := s.commit(next); err != nil {
		return Rule{}, err
	}
	return r, nil
}
func (s *Store) Delete(id string, expected int, by string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.data.Rules[id]
	if !ok {
		return ErrNotFound
	}
	if r.Version != expected {
		return ErrConflict
	}
	next := clone(s.data)
	if rt := next.Runtime[id]; rt.Active != "" {
		resolve(&next, rt.Active, "rule deleted", by, now)
	}
	delete(next.Rules, id)
	delete(next.Runtime, id)
	event(&next, now, r, "", "rule-deleted", by, "")
	return s.commit(next)
}
func (s *Store) Acknowledge(id string, expected int, by, note, tenant string, now time.Time) (Incident, error) {
	note = strings.TrimSpace(note)
	if note == "" || len(note) > 500 {
		return Incident{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.data)
	for i := range next.Incidents {
		in := &next.Incidents[i]
		if in.ID != id || (tenant != "" && in.Rule.Tenant != tenant) {
			continue
		}
		if in.Version != expected || in.Status != "open" {
			return Incident{}, ErrConflict
		}
		in.Status = "acknowledged"
		in.Version++
		t := now.UTC()
		in.AckAt = &t
		in.AckBy = by
		in.AckNote = note
		event(&next, now, in.Rule, id, "acknowledged", by, note)
		result := *in
		if err := s.commit(next); err != nil {
			return Incident{}, err
		}
		return result, nil
	}
	return Incident{}, ErrNotFound
}
func (s *Store) View(tenant string) View {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := View{Rules: []Rule{}, Runtime: map[string]Runtime{}, Incidents: []Incident{}, Events: []Event{}, PersistenceError: s.persistError}
	for id, r := range s.data.Rules {
		if tenant == "" || r.Tenant == tenant {
			out.Rules = append(out.Rules, r)
			out.Runtime[id] = s.data.Runtime[id]
		}
	}
	sort.Slice(out.Rules, func(i, j int) bool { return out.Rules[i].ID < out.Rules[j].ID })
	for i := len(s.data.Incidents) - 1; i >= 0; i-- {
		in := s.data.Incidents[i]
		if tenant == "" || in.Rule.Tenant == tenant {
			out.Incidents = append(out.Incidents, in)
		}
	}
	for i := len(s.data.Events) - 1; i >= 0; i-- {
		e := s.data.Events[i]
		if tenant == "" || e.Tenant == tenant {
			out.Events = append(out.Events, e)
		}
	}
	return clone(out)
}

type Observation struct {
	At     time.Time
	Value  float64
	Valid  bool
	Tenant string
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Evaluate uses only new fresh observations. Missing, invalid, out-of-order or
// widely separated samples never establish duration or falsely resolve an incident.
func (s *Store) Evaluate(observations map[string]Observation, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Rules) == 0 {
		return nil
	}
	next := clone(s.data)
	if s.needsReset {
		for id, rt := range next.Runtime {
			rt.Pending = nil
			rt.Clearing = nil
			rt.LastAt = nil
			next.Runtime[id] = rt
		}
	}
	ids := []string{}
	for id := range next.Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := next.Rules[id]
		rt := next.Runtime[id]
		if !r.Enabled {
			continue
		}
		o, ok := observations[r.Metric]
		valid := ok && o.Valid && o.Tenant == r.Tenant && finite(o.Value) && !o.At.IsZero() && !o.At.After(now) && now.Sub(o.At) <= time.Duration(r.MaxGapSeconds)*time.Second
		if !valid {
			rt.Pending = nil
			rt.Clearing = nil
			rt.Status = "unavailable"
			next.Runtime[id] = rt
			continue
		}
		if rt.LastAt != nil && !o.At.After(*rt.LastAt) {
			continue
		}
		if rt.LastAt != nil && o.At.Sub(*rt.LastAt) > time.Duration(r.MaxGapSeconds)*time.Second {
			rt.Pending = nil
			rt.Clearing = nil
		}
		t := o.At.UTC()
		rt.LastAt = &t
		value := o.Value
		rt.LastValue = &value
		breached := r.Operator == "above" && value > r.Threshold || r.Operator == "below" && value < r.Threshold
		if rt.Active == "" {
			rt.Clearing = nil
			rt.Status = "waiting"
			if !breached {
				rt.Pending = nil
			} else {
				if rt.Pending == nil {
					p := t
					rt.Pending = &p
				}
				rt.Status = "pending"
				if t.Sub(*rt.Pending) >= time.Duration(r.ForSeconds)*time.Second {
					next.Next++
					incidentID := fmt.Sprintf("watch-%d", next.Next)
					next.Incidents = append(next.Incidents, Incident{ID: incidentID, Rule: r, Version: 1, Status: "open", OpenedAt: t, ObservedAt: t, Value: value})
					rt.Active = incidentID
					rt.Pending = nil
					rt.Status = "open"
					event(&next, t, r, incidentID, "opened", "zyntra", "")
				}
			}
		} else {
			for i := range next.Incidents {
				in := &next.Incidents[i]
				if in.ID == rt.Active {
					in.ObservedAt = t
					in.Value = value
				}
			}
			rt.Pending = nil
			rt.Status = "open"
			if breached {
				rt.Clearing = nil
			} else {
				if rt.Clearing == nil {
					p := t
					rt.Clearing = &p
				}
				rt.Status = "recovering"
				if t.Sub(*rt.Clearing) >= time.Duration(r.ClearSeconds)*time.Second {
					resolve(&next, rt.Active, "threshold recovered", "zyntra", t)
					rt.Active = ""
					rt.Clearing = nil
					rt.Status = "waiting"
				}
			}

		}
		next.Runtime[id] = rt
	}
	// Retain active incidents; evict the oldest resolved records first.
	for len(next.Incidents) > MaxIncidents {
		removed := false
		for i, in := range next.Incidents {
			if in.Status == "resolved" {
				next.Incidents = append(next.Incidents[:i], next.Incidents[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			return ErrCapacity
		}
	}
	if reflect.DeepEqual(next, s.data) {
		return nil
	}
	if err := s.commit(next); err != nil {
		return err
	}
	s.needsReset = false
	return nil
}

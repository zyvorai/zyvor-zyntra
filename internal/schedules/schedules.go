// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package schedules provides interval-based recurring plan generation.
// Due schedules create pending proposals that still need human approval;
// they never execute actions themselves.
package schedules

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Schedule describes a recurring plan run.
type Schedule struct {
	ID           string    `json:"id"`
	Name         string    `json:"name,omitempty"`
	Pack         string    `json:"pack,omitempty"`
	EveryMinutes int       `json:"every_minutes"`
	Enabled      bool      `json:"enabled"`
	LastRun      time.Time `json:"last_run,omitempty"`
	Owner        string    `json:"owner,omitempty"`
	// Action, when set, is proposed instead of the top-ranked plan recommendation.
	Action string `json:"action,omitempty"`
}

// Due reports whether the schedule should run now.
func Due(s Schedule, now time.Time) bool {
	if !s.Enabled || s.EveryMinutes < 1 {
		return false
	}
	if s.LastRun.IsZero() {
		return true
	}
	return now.Sub(s.LastRun) >= time.Duration(s.EveryMinutes)*time.Minute
}

// Store keeps schedules in one JSON file.
type Store struct {
	mu   sync.Mutex
	path string
	list map[string]Schedule
}

// Open loads the store; an empty path keeps it in memory.
func Open(path string) (*Store, error) {
	s := &Store{path: path, list: map[string]Schedule{}}
	if path == "" {
		return s, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.list); err != nil {
		return nil, err
	}
	if s.list == nil {
		s.list = map[string]Schedule{}
	}
	return s, nil
}

func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Put saves sc, assigning an id when it has none.
func (s *Store) Put(sc Schedule) (Schedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sc.EveryMinutes < 1 {
		return sc, errors.New("every_minutes must be at least 1")
	}
	if sc.ID == "" {
		var b [6]byte
		_, _ = rand.Read(b[:])
		sc.ID = "sch-" + hex.EncodeToString(b[:])
	}
	s.list[sc.ID] = sc
	return sc, s.save()
}

// Get returns one schedule.
func (s *Store) Get(id string) (Schedule, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.list[id]
	return sc, ok
}

// List returns schedules sorted by id.
func (s *Store) List() []Schedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Schedule, 0, len(s.list))
	for _, sc := range s.list {
		out = append(out, sc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Delete removes a schedule.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.list[id]; !ok {
		return errors.New("no such schedule")
	}
	delete(s.list, id)
	return s.save()
}

// MarkRun records that the schedule just fired.
func (s *Store) MarkRun(id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc, ok := s.list[id]
	if !ok {
		return errors.New("no such schedule")
	}
	sc.LastRun = at.UTC()
	s.list[id] = sc
	return s.save()
}

// DueNow returns enabled schedules that are due at now.
func (s *Store) DueNow(now time.Time) []Schedule {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Schedule
	for _, sc := range s.list {
		if Due(sc, now) {
			out = append(out, sc)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

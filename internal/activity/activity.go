// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package activity tracks last-seen timestamps for principals.
package activity

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Record is a lightweight activity stamp.
type Record struct {
	Principal string    `json:"principal"`
	LastSeen  time.Time `json:"last_seen"`
	Count     int       `json:"count"`
}

// Touch updates or creates a record.
func Touch(r *Record, principal string, now time.Time) {
	if r == nil {
		return
	}
	if r.Principal == "" {
		r.Principal = principal
	}
	r.LastSeen = now
	r.Count++
}

// Store persists activity records.
type Store struct {
	mu   sync.Mutex
	path string
	list map[string]Record
}

// Open loads the store; an empty path keeps it in memory.
func Open(path string) (*Store, error) {
	s := &Store{path: path, list: map[string]Record{}}
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
		s.list = map[string]Record{}
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

// Touch updates the principal's last-seen stamp.
func (s *Store) Touch(principal string, now time.Time) {
	if s == nil || principal == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.list[principal]
	Touch(&r, principal, now.UTC())
	s.list[principal] = r
	_ = s.save()
}

// List returns records newest first.
func (s *Store) List() []Record {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, 0, len(s.list))
	for _, r := range s.list {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}

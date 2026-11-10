// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package inputs keeps values pushed into Zyntra rather than pulled: the
// last document POSTed to each webhook-in channel and the values operators
// type for manual KPIs. Both survive restarts when a path is given.
package inputs

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Doc is the last document received on a webhook-in channel.
type Doc struct {
	Body any       `json:"body"`
	At   time.Time `json:"at"`
	From string    `json:"from,omitempty"`
}

// Manual is an operator-entered KPI value.
type Manual struct {
	Value  float64   `json:"value"`
	At     time.Time `json:"at"`
	By     string    `json:"by"`
	Reason string    `json:"reason,omitempty"`
}

type state struct {
	Webhooks     map[string]Doc    `json:"webhooks"`
	Manual       map[string]Manual `json:"manual"`
	Events       []Event           `json:"events,omitempty"`
	NextSequence uint64            `json:"next_sequence,omitempty"`
}

// Store holds pushed inputs. The zero value is not usable; call Open.
type Store struct {
	mu   sync.RWMutex
	path string
	s    state
}

// Open loads the store from path; an empty path keeps it in memory.
func Open(path string) (*Store, error) {
	st := &Store{path: path, s: state{Webhooks: map[string]Doc{}, Manual: map[string]Manual{}}}
	if path == "" {
		return st, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &st.s); err != nil {
		return nil, err
	}
	if st.s.Webhooks == nil {
		st.s.Webhooks = map[string]Doc{}
	}
	if st.s.Manual == nil {
		st.s.Manual = map[string]Manual{}
	}
	var last uint64
	for _, e := range st.s.Events {
		if e.Sequence <= last || e.Sequence > st.s.NextSequence {
			return nil, errors.New("invalid input event sequence")
		}
		last = e.Sequence
	}
	return st, nil
}

// copyState is called under the write lock. Existing document bodies are
// immutable; IngestEvent and Webhook detach all external references.
func (s *Store) copyState() state {
	return state{Webhooks: maps.Clone(s.s.Webhooks), Manual: maps.Clone(s.s.Manual), Events: append([]Event(nil), s.s.Events...), NextSequence: s.s.NextSequence}
}
func (s *Store) commit(next state) error {
	if s.path == "" {
		s.s = next
		return nil
	}
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, s.path); err != nil {
		return err
	}
	s.s = next
	dir, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

// Ingest preserves the existing API while retaining accepted and late events.
func (s *Store) Ingest(channel string, body any, from string, at time.Time) error {
	_, err := s.IngestEvent(channel, body, from, "", at, at, "")
	return err
}

// Webhook returns the latest document on channel.
func (s *Store) Webhook(channel string) (Doc, bool) {
	if s == nil {
		return Doc{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.s.Webhooks[channel]
	if !ok {
		return d, false
	}
	b, _ := json.Marshal(d)
	var detached Doc
	_ = json.Unmarshal(b, &detached)
	return detached, true
}

// SetManual records an operator-entered value for kpi.
func (s *Store) SetManual(kpi string, v Manual) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v.At = v.At.UTC()
	next := s.copyState()
	next.Manual[kpi] = v
	return s.commit(next)
}

// Manual returns the operator-entered value for kpi.
func (s *Store) Manual(kpi string) (Manual, bool) {
	if s == nil {
		return Manual{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.s.Manual[kpi]
	return v, ok
}

// ManualValues returns every manual entry.
func (s *Store) ManualValues() map[string]Manual {
	out := map[string]Manual{}
	if s == nil {
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	maps.Copy(out, s.s.Manual)
	return out
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package webhooks provides HMAC-signed outbound event delivery to configured
// destinations. Delivery is best-effort and never blocks an approval.
package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Event is a signed outbound notification.
type Event struct {
	Type string         `json:"type"`
	At   time.Time      `json:"at"`
	Data map[string]any `json:"data"`
}

// Destination is a configured outbound webhook URL.
type Destination struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	Secret       string    `json:"secret,omitempty"`
	Events       []string  `json:"events,omitempty"` // empty = all
	Enabled      bool      `json:"enabled"`
	LastStatus   int       `json:"last_status,omitempty"`
	LastDelivery time.Time `json:"last_delivery,omitempty"`
	Failures     int       `json:"failures,omitempty"`
}

// Public returns a copy with the secret redacted.
func (d Destination) Public() Destination {
	out := d
	if out.Secret != "" {
		out.Secret = "••••••••"
	}
	return out
}

// Matches reports whether this destination wants event type.
func (d Destination) Matches(eventType string) bool {
	if !d.Enabled || d.URL == "" {
		return false
	}
	if len(d.Events) == 0 {
		return true
	}
	for _, e := range d.Events {
		if strings.EqualFold(e, eventType) || e == "*" {
			return true
		}
	}
	return false
}

// Sign returns the hex HMAC-SHA256 of the canonical JSON payload.
func Sign(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// Deliver posts a signed event. Fire-and-forget; errors are returned to the caller.
func Deliver(url, secret string, ev Event, client *http.Client) (int, error) {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Zyntra-Signature", "sha256="+Sign(secret, body))
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// Store keeps destinations in one JSON file.
type Store struct {
	mu   sync.Mutex
	path string
	list map[string]Destination
}

// Open loads the store; an empty path keeps it in memory.
func Open(path string) (*Store, error) {
	s := &Store{path: path, list: map[string]Destination{}}
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
		s.list = map[string]Destination{}
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

// Put saves d, assigning an id and secret when missing.
func (s *Store) Put(d Destination) (Destination, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d.URL == "" {
		return d, errors.New("url is required")
	}
	if d.ID == "" {
		var b [6]byte
		_, _ = rand.Read(b[:])
		d.ID = "wh-" + hex.EncodeToString(b[:])
	}
	if d.Secret == "" {
		var b [16]byte
		_, _ = rand.Read(b[:])
		d.Secret = hex.EncodeToString(b[:])
	} else if existing, ok := s.list[d.ID]; ok && (d.Secret == "••••••••" || d.Secret == "") {
		d.Secret = existing.Secret
	}
	s.list[d.ID] = d
	return d, s.save()
}

// Get returns one destination.
func (s *Store) Get(id string) (Destination, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.list[id]
	return d, ok
}

// List returns destinations sorted by id.
func (s *Store) List() []Destination {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Destination, 0, len(s.list))
	for _, d := range s.list {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Delete removes a destination.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.list[id]; !ok {
		return errors.New("no such webhook")
	}
	delete(s.list, id)
	return s.save()
}

// RecordDelivery updates last status for a destination.
func (s *Store) RecordDelivery(id string, status int, at time.Time, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.list[id]
	if !ok {
		return
	}
	d.LastStatus = status
	d.LastDelivery = at.UTC()
	if err != nil || status >= 400 || status == 0 {
		d.Failures++
	} else {
		d.Failures = 0
	}
	s.list[id] = d
	_ = s.save()
}

// Matching returns enabled destinations that want eventType.
func (s *Store) Matching(eventType string) []Destination {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Destination
	for _, d := range s.list {
		if d.Matches(eventType) {
			out = append(out, d)
		}
	}
	return out
}

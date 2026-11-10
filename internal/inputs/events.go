// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package inputs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const MaxEvents = 1000
const MaxEventBytes = 8 << 20

var ErrIdempotencyConflict = errors.New("idempotency key already used with different content")

type Event struct {
	Sequence   uint64    `json:"sequence"`
	Channel    string    `json:"channel"`
	Key        string    `json:"key,omitempty"`
	SHA256     string    `json:"sha256"`
	ReceivedAt time.Time `json:"received_at"`
	Document   Doc       `json:"document"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason,omitempty"`
	Bytes      int       `json:"bytes"`
}
type EventPage struct {
	Events    []Event `json:"events"`
	Next      uint64  `json:"next"`
	Oldest    uint64  `json:"oldest"`
	Truncated bool    `json:"truncated"`
}
type Receipt struct {
	Sequence  uint64 `json:"sequence"`
	Status    string `json:"status"`
	Duplicate bool   `json:"duplicate"`
}

// IngestEvent atomically stores a bounded event history and advances the live
// document only for accepted, non-late observations. Quarantine never changes
// the latest value. Idempotency is bounded by the retained event window.
func (s *Store) IngestEvent(channel string, body any, from, key string, observed, received time.Time, quarantine string) (Receipt, error) {
	var result Receipt
	explicitObservation := !observed.IsZero()
	if !explicitObservation {
		observed = received
	}
	if s == nil {
		return result, fmt.Errorf("input store unavailable")
	}
	if channel == "" || len(key) > 200 || observed.IsZero() || received.IsZero() || observed.After(received.Add(time.Minute)) {
		return result, fmt.Errorf("invalid event channel, key or observation time")
	}
	b, err := json.Marshal(body)
	if err != nil {
		return result, err
	}
	if len(b) > 1<<20 {
		return result, fmt.Errorf("event exceeds 1 MiB")
	}
	// Normalize and detach the caller's maps before retaining the document.
	var detached any
	if err = json.Unmarshal(b, &detached); err != nil {
		return result, err
	}
	sum := sha256.Sum256(b)
	digest := hex.EncodeToString(sum[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.s.Events {
		if key != "" && e.Channel == channel && e.Key == key {
			if e.SHA256 != digest || (explicitObservation && !e.Document.At.Equal(observed)) {
				return result, ErrIdempotencyConflict
			}
			return Receipt{Sequence: e.Sequence, Status: e.Status, Duplicate: true}, nil
		}
	}
	status := "accepted"
	reason := ""
	if quarantine != "" {
		status = "quarantined"
		reason = quarantine
	} else if previous, ok := s.s.Webhooks[channel]; ok && !observed.After(previous.At) {
		status = "late"
		reason = "observation does not follow the current value"
	}
	if s.s.NextSequence == ^uint64(0) {
		return result, fmt.Errorf("event sequence exhausted")
	}
	next := s.copyState()
	next.NextSequence++
	event := Event{Sequence: next.NextSequence, Channel: channel, Key: key, SHA256: digest, ReceivedAt: received.UTC(), Document: Doc{Body: detached, At: observed.UTC(), From: from}, Status: status, Reason: reason, Bytes: len(b)}
	next.Events = append(next.Events, event)
	bytes := 0
	for _, e := range next.Events {
		bytes += e.Bytes
	}
	for len(next.Events) > MaxEvents || bytes > MaxEventBytes {
		bytes -= next.Events[0].Bytes
		next.Events = next.Events[1:]
	}
	if status == "accepted" {
		next.Webhooks[channel] = event.Document
	}
	if err = s.commit(next); err != nil {
		return result, err
	}
	return Receipt{Sequence: event.Sequence, Status: status}, nil
}
func (s *Store) Events(channel string, after uint64, limit int) EventPage {
	out := EventPage{Events: []Event{}, Next: after}
	if s == nil {
		return out
	}
	if limit < 1 || limit > 200 {
		limit = 100
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.s.Events) > 0 {
		out.Oldest = s.s.Events[0].Sequence
		out.Truncated = after > 0 && after < out.Oldest-1
	}
	for _, e := range s.s.Events {
		if e.Channel == channel && e.Sequence > after {
			// Return a detached document, so callers cannot mutate retained evidence.
			b, _ := json.Marshal(e)
			var copy Event
			_ = json.Unmarshal(b, &copy)
			out.Events = append(out.Events, copy)
			out.Next = e.Sequence
			if len(out.Events) >= limit {
				break
			}
		}
	}
	return out
}

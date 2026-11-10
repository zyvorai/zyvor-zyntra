// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package planning

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/zyvorai/zyntra/internal/graph"
)

type Snapshot struct {
	Schema     int             `json:"schema"`
	CapturedAt time.Time       `json:"captured_at"`
	Model      *graph.Model    `json:"model"`
	Config     Config          `json:"config"`
	Unusable   map[string]bool `json:"unusable"`
	SHA256     string          `json:"sha256"`
}

func (s Snapshot) checksum() (string, error) {
	s.SHA256 = ""
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func Capture(m *graph.Model, c Config, unusable map[string]bool, at time.Time) (Snapshot, error) {
	// Round-trip to ensure the snapshot cannot share mutable state with the caller.
	b, err := json.Marshal(Snapshot{Schema: 1, CapturedAt: at, Model: m, Config: c, Unusable: unusable})
	if err != nil {
		return Snapshot{}, err
	}
	var s Snapshot
	if err = json.Unmarshal(b, &s); err != nil {
		return s, err
	}
	s.SHA256, err = s.checksum()
	return s, err
}
func (s Snapshot) Validate() error {
	if s.Schema != 1 || s.CapturedAt.IsZero() || s.Model == nil {
		return fmt.Errorf("invalid snapshot header")
	}
	if len(s.Model.KPIs) > 2000 || len(s.Model.Actions) > 200 || len(s.Model.Edges) > 10000 {
		return fmt.Errorf("snapshot exceeds model limits")
	}
	h, err := s.checksum()
	if err != nil {
		return err
	}
	if h != s.SHA256 {
		return fmt.Errorf("snapshot checksum mismatch")
	}
	if err = s.Model.Validate(); err != nil {
		return err
	}
	for _, k := range s.Model.KPIs {
		if !finite(k.Value) {
			return fmt.Errorf("invalid snapshot KPI value")
		}
	}
	return s.Config.Validate(s.Model)
}
func Replay(ctx context.Context, s Snapshot, o Options) (Result, error) {
	if err := s.Validate(); err != nil {
		return Result{}, err
	}
	o.At = s.CapturedAt
	o.Unusable = s.Unusable
	return Plan(ctx, s.Model, s.Config, o)
}

type EdgeChange struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Weight float64 `json:"weight"`
}
type EdgeDiff struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Before float64 `json:"before"`
	After  float64 `json:"after"`
}
type ReviewRequest struct {
	Snapshot Snapshot     `json:"snapshot"`
	Changes  []EdgeChange `json:"changes"`
	Options  Options      `json:"options"`
	// Actual measurements are optional, after the original observation window.
	Actions []string           `json:"actions,omitempty"`
	Actual  map[string]float64 `json:"actual,omitempty"`
}
type ErrorComparison struct {
	KPI         string  `json:"kpi"`
	Actual      float64 `json:"actual"`
	BeforeError float64 `json:"before_error"`
	AfterError  float64 `json:"after_error"`
}
type Review struct {
	BaseVersion      string            `json:"base_version"`
	CandidateVersion string            `json:"candidate_version"`
	Changes          []EdgeDiff        `json:"changes"`
	Before           Result            `json:"before"`
	After            Result            `json:"after"`
	Errors           []ErrorComparison `json:"errors"`
	Candidate        *graph.Model      `json:"candidate"`
	Note             string            `json:"note"`
}

func Compare(ctx context.Context, req ReviewRequest) (Review, error) {
	out := Review{Changes: []EdgeDiff{}, Errors: []ErrorComparison{}, Note: "Review preview only. No model is installed or approved. Actual outcomes may be confounded; this comparison is not independent validation."}
	s := req.Snapshot
	if err := s.Validate(); err != nil {
		return out, err
	}
	if len(req.Changes) == 0 || len(req.Changes) > 100 {
		return out, fmt.Errorf("provide 1..100 edge changes")
	}
	candidate := s.Model.Clone()
	seen := map[string]bool{}
	for _, ch := range req.Changes {
		key := ch.From + "\x00" + ch.To
		if seen[key] || !finite(ch.Weight) {
			return out, fmt.Errorf("duplicate or invalid edge change")
		}
		seen[key] = true
		found := false
		for i, e := range candidate.Edges {
			if e.From == ch.From && e.To == ch.To {
				out.Changes = append(out.Changes, EdgeDiff{ch.From, ch.To, e.Weight, ch.Weight})
				candidate.Edges[i].Weight = ch.Weight
				found = true
			}
		}
		if !found {
			return out, fmt.Errorf("unknown edge %s -> %s", ch.From, ch.To)
		}
	}
	if err := candidate.Validate(); err != nil {
		return out, err
	}
	o := req.Options
	o.At = s.CapturedAt
	o.Unusable = s.Unusable
	var err error
	out.Before, err = Plan(ctx, s.Model, s.Config, o)
	if err != nil {
		return out, err
	}
	out.After, err = Plan(ctx, candidate, s.Config, o)
	if err != nil {
		return out, err
	}
	out.BaseVersion = s.Model.Version()
	out.CandidateVersion = candidate.Version()
	out.Candidate = candidate
	if len(req.Actual) > 0 {
		before, err := Evaluate(s.Model, req.Actions, s.Config, o)
		if err != nil {
			return out, err
		}
		after, err := Evaluate(candidate, req.Actions, s.Config, o)
		if err != nil {
			return out, err
		}
		for id, v := range req.Actual {
			if _, ok := s.Model.KPI(id); !ok || !finite(v) {
				return out, fmt.Errorf("unknown or invalid actual KPI %s", id)
			}
		}
		for i, k := range before.Simulation.KPIs {
			if v, ok := req.Actual[k.KPI]; ok {
				out.Errors = append(out.Errors, ErrorComparison{k.KPI, v, math.Abs(k.After - v), math.Abs(after.Simulation.KPIs[i].After - v)})
			}
		}
	}
	return out, nil
}

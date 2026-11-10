// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/zyvorai/zyntra/internal/freshness"
	"github.com/zyvorai/zyntra/internal/graph"
	"github.com/zyvorai/zyntra/internal/planning"
	"github.com/zyvorai/zyntra/internal/semantic"
	"github.com/zyvorai/zyntra/internal/sim"
)

func (s *Server) planningConfig() (planning.Config, error) {
	var c planning.Config
	if s.opt.PackDir == "" {
		return c, nil
	}
	b, err := os.ReadFile(filepath.Join(s.opt.PackDir, "planning.json"))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return c, err
	}
	if d.Decode(new(any)) != io.EOF {
		return c, errors.New("planning.json must contain exactly one JSON object")
	}
	return c, nil
}

// Free-text economic assumptions can reference other actions. Until the config
// has property-level markings, omit it for partially visible models.
func (s *Server) planningContext(r *http.Request) (*graph.Model, planning.Config, error) {
	m, _ := s.snapshotFor(r)
	full, _ := s.snapshot()
	if full.Version() != m.Version() {
		return m, planning.Config{}, nil
	}
	c, err := s.planningConfig()
	if err != nil {
		return m, c, errors.New("invalid planning configuration")
	}
	return m, c, nil
}

func planningDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		writeErr(w, 400, "invalid planning request: "+err.Error())
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		writeErr(w, 400, "expected one JSON object")
		return false
	}
	return true
}
func (s *Server) handlePlanningSnapshot(w http.ResponseWriter, r *http.Request) {
	m, c, err := s.planningContext(r)
	if err == nil {
		err = c.Validate(m)
	}
	if err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	snap, err := planning.Capture(m, c, freshness.Set(s.freshness(m)), time.Now().UTC())
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, snap)
}
func (s *Server) handlePlanning(w http.ResponseWriter, r *http.Request) {
	var o planning.Options
	if !planningDecode(w, r, &o) {
		return
	}
	m, c, err := s.planningContext(r)
	if err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	// Live planning always uses the server clock and current freshness.
	o.At = time.Now().UTC()
	o.Unusable = freshness.Set(s.freshness(m))
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := planning.Plan(ctx, m, c, o)
	if err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) handlePlanningReplay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Snapshot planning.Snapshot `json:"snapshot"`
		Options  planning.Options  `json:"options"`
	}
	if !planningDecode(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := planning.Replay(ctx, req.Snapshot, req.Options)
	if err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) handlePlanningReview(w http.ResponseWriter, r *http.Request) {
	var req planning.ReviewRequest
	if !planningDecode(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := planning.Compare(ctx, req)
	if err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) handlePlanningTimeline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actions []string `json:"actions"`
		Horizon string   `json:"horizon"`
		Step    string   `json:"step"`
	}
	if !planningDecode(w, r, &req) {
		return
	}
	horizon, err := time.ParseDuration(req.Horizon)
	if err != nil {
		writeErr(w, 422, "invalid horizon")
		return
	}
	step, err := time.ParseDuration(req.Step)
	if err != nil {
		writeErr(w, 422, "invalid step")
		return
	}
	m, _ := s.snapshotFor(r)
	acts, err := resolveActions(m, req.Actions...)
	if err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	if len(acts) > 6 {
		writeErr(w, 422, "at most six actions")
		return
	}
	seen := map[string]bool{}
	for _, a := range acts {
		if seen[a.ID] {
			writeErr(w, 422, "duplicate action")
			return
		}
		seen[a.ID] = true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := sim.Timeline(ctx, m, acts, horizon, step, freshness.Set(s.freshness(m)))
	if err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) handlePlanningProcess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Events []planning.Event `json:"events"`
	}
	if !planningDecode(w, r, &req) {
		return
	}
	result, err := planning.Mine(req.Events)
	if err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) handlePlanningBenefits(w http.ResponseWriter, r *http.Request) {
	var req planning.Experiment
	if !planningDecode(w, r, &req) {
		return
	}
	result, err := planning.Measure(req)
	if err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) handlePlanningQuality(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Contract      string           `json:"contract"`
		Rows          []map[string]any `json:"rows"`
		ObservedField string           `json:"observed_field"`
	}
	if !planningDecode(w, r, &req) {
		return
	}
	visible, _ := s.snapshotFor(r)
	full, _ := s.snapshot()
	if visible.Version() != full.Version() {
		writeErr(w, 403, "full model clearance required for data contracts")
		return
	}
	contracts, err := s.semanticContracts()
	if err != nil {
		writeErr(w, 422, "data contracts unavailable")
		return
	}
	for _, c := range contracts {
		if c.Name == req.Contract {
			result, err := semantic.CheckBatch(c, req.Rows, req.ObservedField, time.Now().UTC())
			if err != nil {
				writeErr(w, 422, err.Error())
				return
			}
			writeJSON(w, 200, result)
			return
		}
	}
	writeErr(w, 404, "unknown data contract")
}

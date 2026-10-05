// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"encoding/json"
	"errors"
	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/watch"
	"io"
	"net/http"
	"time"
)

func (s *Server) watchReady(w http.ResponseWriter) bool {
	if s.opt.Watches == nil {
		writeErr(w, 503, "watch store unavailable")
		return false
	}
	return true
}
func watchErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, watch.ErrNotFound):
		writeErr(w, 404, "watch item not found")
	case errors.Is(err, watch.ErrConflict), errors.Is(err, watch.ErrCapacity):
		writeErr(w, 409, err.Error())
	case errors.Is(err, watch.ErrInvalid):
		writeErr(w, 400, err.Error())
	default:
		writeErr(w, 500, "watch persistence unavailable")
	}
}
func strictWatchJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		writeErr(w, 400, "invalid watch JSON")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		writeErr(w, 400, "expected one JSON object")
		return false
	}
	return true
}
func (s *Server) handleWatches(w http.ResponseWriter, r *http.Request) {
	if !s.watchReady(w) {
		return
	}
	writeJSON(w, 200, s.opt.Watches.View(auth.FromContext(r.Context()).Tenant))
}
func (s *Server) handleWatchPut(w http.ResponseWriter, r *http.Request) {
	if !s.watchReady(w) {
		return
	}
	var req struct {
		Name            string   `json:"name"`
		Metric          string   `json:"metric"`
		Operator        string   `json:"operator"`
		Threshold       *float64 `json:"threshold"`
		ForSeconds      *int     `json:"for_seconds"`
		ClearSeconds    *int     `json:"clear_seconds"`
		MaxGapSeconds   *int     `json:"max_gap_seconds"`
		Enabled         *bool    `json:"enabled"`
		ExpectedVersion *int     `json:"expected_version"`
	}
	if !strictWatchJSON(w, r, &req) {
		return
	}
	if req.Threshold == nil || req.ForSeconds == nil || req.ClearSeconds == nil || req.MaxGapSeconds == nil || req.Enabled == nil || req.ExpectedVersion == nil {
		writeErr(w, 400, "threshold, durations, enabled and expected_version are required")
		return
	}
	m, _ := s.snapshot()
	k, ok := m.KPI(req.Metric)
	if !ok {
		writeErr(w, 400, "metric unavailable")
		return
	}
	rule := watch.Rule{ID: r.PathValue("id"), Name: req.Name, Metric: k.ID, MetricName: k.Name, Unit: k.DisplayUnit(), Tenant: k.Tenant, Operator: req.Operator, Threshold: *req.Threshold, ForSeconds: *req.ForSeconds, ClearSeconds: *req.ClearSeconds, MaxGapSeconds: *req.MaxGapSeconds, Enabled: *req.Enabled}
	saved, err := s.opt.Watches.Put(rule, *req.ExpectedVersion, auth.FromContext(r.Context()).Subject, time.Now())
	if err != nil {
		watchErr(w, err)
		return
	}
	writeJSON(w, 200, saved)
}
func (s *Server) handleWatchDelete(w http.ResponseWriter, r *http.Request) {
	if !s.watchReady(w) {
		return
	}
	var req struct {
		ExpectedVersion *int `json:"expected_version"`
	}
	if !strictWatchJSON(w, r, &req) {
		return
	}
	if req.ExpectedVersion == nil {
		writeErr(w, 400, "expected_version required")
		return
	}
	if err := s.opt.Watches.Delete(r.PathValue("id"), *req.ExpectedVersion, auth.FromContext(r.Context()).Subject, time.Now()); err != nil {
		watchErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) handleWatchAcknowledge(w http.ResponseWriter, r *http.Request) {
	if !s.watchReady(w) {
		return
	}
	var req struct {
		ExpectedVersion *int   `json:"expected_version"`
		Note            string `json:"note"`
	}
	if !strictWatchJSON(w, r, &req) {
		return
	}
	if req.ExpectedVersion == nil {
		writeErr(w, 400, "expected_version required")
		return
	}
	id := auth.FromContext(r.Context())
	in, err := s.opt.Watches.Acknowledge(r.PathValue("id"), *req.ExpectedVersion, id.Subject, req.Note, id.Tenant, time.Now())
	if err != nil {
		watchErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

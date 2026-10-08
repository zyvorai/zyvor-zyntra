// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package api

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/zyvorai/zyntra/internal/executive"
	"github.com/zyvorai/zyntra/internal/gaps"
	"github.com/zyvorai/zyntra/internal/graph"
)

func (s *Server) executiveInput(r *http.Request) (executive.Input, error) {
	m, _ := s.snapshot()
	plan, st, err := s.plan(m)
	if err != nil {
		return executive.Input{}, err
	}
	now := time.Now()
	in := executive.Input{
		Model: m, Plan: plan, Gaps: gaps.Detect(m), Fresh: st,
		Overlay: s.overlay(m), Proposals: s.opt.Store.List(),
		Now: now, Audience: r.URL.Query().Get("audience"),
	}
	return in, nil
}

func (s *Server) overlay(m *graph.Model) executive.Overlay {
	if s.opt.PackDir == "" {
		return executive.Overlay{}
	}
	b, err := os.ReadFile(filepath.Join(s.opt.PackDir, "executive.yaml"))
	if err != nil {
		return executive.Overlay{}
	}
	o, err := executive.ParseOverlay(b)
	if err != nil {
		return executive.Overlay{}
	}
	return o
}

func (s *Server) handleExecutiveInbox(w http.ResponseWriter, r *http.Request) {
	in, err := s.executiveInput(r)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, executive.BuildInbox(in))
}

func (s *Server) handleExecutiveBrief(w http.ResponseWriter, r *http.Request) {
	in, err := s.executiveInput(r)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, executive.BuildBrief(in, r.URL.Query().Get("decision")))
}

func (s *Server) handleExecutiveConflicts(w http.ResponseWriter, r *http.Request) {
	in, err := s.executiveInput(r)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conflicts": executive.Conflicts(in)})
}

func (s *Server) handleExecutiveReviews(w http.ResponseWriter, r *http.Request) {
	in, err := s.executiveInput(r)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": executive.Reviews(in.Proposals, in.Now)})
}

func (s *Server) handleExecutiveDigest(w http.ResponseWriter, r *http.Request) {
	in, err := s.executiveInput(r)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, executive.BuildDigest(in))
}

func (s *Server) handleExecutiveSensitivity(w http.ResponseWriter, r *http.Request) {
	in, err := s.executiveInput(r)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	action := r.URL.Query().Get("action")
	if action == "" {
		writeErr(w, http.StatusBadRequest, "action is required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": action, "assumptions": executive.Probe(in, action)})
}

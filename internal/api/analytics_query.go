// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"github.com/zyvorai/zyntra/internal/ai"
	"github.com/zyvorai/zyntra/internal/auth"
	"net/http"
	"time"
)

// Build a minimal catalog and history together. Neither source configuration,
// provider metadata nor unauthorized metric names reach an answer or the LLM.
func (s *Server) queryContext(r *http.Request) ([]ai.Metric, map[string][]ai.Point) {
	m, _ := s.snapshot()
	tenant := auth.FromContext(r.Context()).Tenant
	unavailable := s.analyticsUnavailable(m)
	catalog := []ai.Metric{}
	ids := []string{}
	for _, k := range m.KPIs {
		if tenant != "" && k.Tenant != tenant {
			continue
		}
		warning := ""
		if unavailable[k.ID] {
			warning = "Current observations are unavailable or stale; historical samples may not describe current conditions."
		}
		catalog = append(catalog, ai.Metric{ID: k.ID, Name: k.Name, Unit: k.DisplayUnit(), Warning: warning})
		ids = append(ids, k.ID)
	}
	return catalog, s.opt.History.QueryHistory(ids)
}
func (s *Server) handleAnalyticsCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, _ := s.queryContext(r)
	writeJSON(w, 200, map[string]any{"metrics": catalog, "operations": []string{"latest", "mean", "min", "max", "change", "trend"}, "max_window_hours": 720, "max_metrics": 5})
}
func (s *Server) handleAnalyticsQuery(w http.ResponseWriter, r *http.Request) {
	q, err := ai.DecodeQuery(http.MaxBytesReader(w, r.Body, 8192))
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	catalog, history := s.queryContext(r)
	result, err := ai.RunQuery(q, catalog, history, time.Now())
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, result)
}

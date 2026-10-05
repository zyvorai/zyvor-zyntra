// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"github.com/zyvorai/zyntra/internal/ai"
	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/watch"
	"net/http"
	"time"
)

type incidentInvestigation struct {
	Incident    watch.Incident         `json:"incident"`
	GeneratedAt time.Time              `json:"generated_at"`
	Context     string                 `json:"context"`
	Report      ai.InvestigationReport `json:"report"`
}

// Investigations are read-only and reconstructed from retained history. The
// incident's opening time, never the request time, bounds the evidence window.
func (s *Server) handleIncidentInvestigation(w http.ResponseWriter, r *http.Request) {
	if !s.watchReady(w) {
		return
	}
	var incident watch.Incident
	for _, in := range s.opt.Watches.View(auth.FromContext(r.Context()).Tenant).Incidents {
		if in.ID == r.PathValue("id") {
			incident = in
			break
		}
	}
	if incident.ID == "" {
		writeErr(w, 404, "watch item not found")
		return
	}
	m, _ := s.snapshot()
	target, ok := m.KPI(incident.Rule.Metric)
	if !ok || target.Tenant != incident.Rule.Tenant || target.DisplayUnit() != incident.Rule.Unit {
		writeErr(w, 409, "incident metric is unavailable or its tenant or unit has changed")
		return
	}
	// Even deployment-wide readers compare only metrics in this incident's tenant.
	// Do not copy another tenant's history before applying this filter.
	catalog := []ai.Metric{}
	for _, k := range m.KPIs {
		if k.Tenant == incident.Rule.Tenant {
			catalog = append(catalog, ai.Metric{ID: k.ID, Name: k.Name, Unit: k.DisplayUnit()})
		}
	}
	q := ai.Investigation{Metric: target.ID, WindowHours: 168, RecentHours: 6, MaxLagHours: 6}
	ids, err := ai.InvestigationMetrics(q, catalog)
	if err != nil {
		writeErr(w, 409, "incident metric unavailable")
		return
	}
	report, err := ai.Investigate(q, catalog, s.opt.History.QueryHistory(ids), incident.OpenedAt)
	if err != nil {
		writeErr(w, 409, "incident investigation unavailable")
		return
	}
	writeJSON(w, 200, incidentInvestigation{
		Incident: incident, GeneratedAt: time.Now().UTC(), Report: report,
		Context: "Historical context before incident opening: 168 completed UTC hours, most recent 6 hours, candidate leads 0–6 hours, at most 64 metrics from the same tenant. The partial opening hour and later observations are excluded. This is reconstructed from currently retained history and current metric metadata; it is not a frozen opening-time capture. Short threshold breaches may not establish an hourly shift. Missing history remains missing; associations do not establish root cause.",
	})
}

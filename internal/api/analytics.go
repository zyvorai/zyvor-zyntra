// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"github.com/zyvorai/zyntra/internal/freshness"
	"github.com/zyvorai/zyntra/internal/graph"
	"time"
)

func (s *Server) analyticsUnavailable(m *graph.Model) map[string]bool {
	out := map[string]bool{}
	for _, f := range s.freshness(m) {
		if !f.Usable() || f.Status == freshness.Held || f.LastError != "" {
			out[f.KPI] = true
		}
	}

	cutoff := time.Now().Add(-max(3*s.opt.Interval, 2*time.Minute))
	for _, k := range m.KPIs {
		pts := s.opt.History.Series(k.ID)
		if len(pts) > 0 && pts[len(pts)-1].T.Before(cutoff) {
			out[k.ID] = true
		}
	}
	return out
}

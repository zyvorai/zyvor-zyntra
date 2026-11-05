// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package pack

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/zyvorai/zyntra/internal/adapters"
	"github.com/zyvorai/zyntra/internal/planner"
)

// TestDefencePacks also covers the aviation packs, which share its shape.
func TestDefencePacks(t *testing.T) {
	cases := []struct {
		pack string
		want map[string]float64
	}{
		{"defence-sustainment", map[string]float64{
			"mission_capable_rate": 0.7, "nmc_supply_rate": 0.2, "predicted_failure_backlog": 3,
			"stockout_risk_parts": 3, "depot_open_workorders": 4,
		}},
		{"defence-compliance", map[string]float64{
			"controls_implemented_ratio": 0.7, "controls_stale_evidence": 2, "poam_open_items": 3,
			"critical_vuln_age_days": 35, "cui_access_anomaly_rate": 0.2,
		}},
		{"defence-sensors", map[string]float64{
			"sensor_uptime": 0.8, "link_availability": 2.0 / 3, "spares_out_of_stock": 1,
		}},
	}
	for _, c := range cases {
		t.Run(c.pack, func(t *testing.T) {
			dir := filepath.Join(root, c.pack)
			if r := Validate(context.Background(), dir); !r.Valid() {
				t.Fatalf("invalid: %v", r.Errors)
			}
			m, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			// Manual KPIs have no value to fetch; that is expected.
			_, _ = adapters.Refresh(context.Background(), m, adapters.Config{Files: adapters.NewFileCache()})
			for id, want := range c.want {
				k, ok := m.KPI(id)
				if !ok {
					t.Fatalf("missing KPI %s", id)
				}
				if math.Abs(k.Value-want) > 1e-6 {
					t.Errorf("%s = %v, want %v", id, k.Value, want)
				}
			}
			res, err := planner.PlanWith(m, planner.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Recommendations) == 0 {
				t.Errorf("no recommendation proposed for the fixture")
			}
		})
	}
}

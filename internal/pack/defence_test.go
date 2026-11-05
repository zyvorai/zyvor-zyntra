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

// TestDefencePacks also covers the aviation, industrial, UAS and launch-range
// packs, which share its shape.
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
		{"aviation-ops", map[string]float64{
			"on_time_rate": 0.6, "cancellation_rate": 0.1, "long_delays": 2,
			"turnaround_p90_min": 95, "crews_near_duty_limit": 2,
		}},
		{"aviation-mro", map[string]float64{
			"aircraft_availability": 4.0 / 6, "aircraft_on_ground": 1, "open_work_orders": 6,
			"work_order_age_p90_days": 30, "deferred_defects_overdue": 2,
			"unscheduled_share": 0.375, "parts_fill_rate": 23.0 / 28, "aog_critical_stockouts": 2,
		}},
		{"airport-ops", map[string]float64{
			"stand_turnaround_p90_min": 90, "taxi_out_extra_avg_min": 5.625, "stands_blocked": 1,
			"stand_utilisation": 0.5, "ctot_adherence": 0.8, "last_bag_p90_min": 41,
			"late_baggage_flights": 2, "security_queue_p90_min": 22, "security_lanes_open": 4,
		}},
		{"industrial-plant", map[string]float64{
			"machine_availability": 0.7, "performance_rate": 2800.0 / 3300, "first_pass_yield": 2530.0 / 2800,
			"oee": 0.7 * (2800.0 / 3300) * (2530.0 / 2800), "machines_down": 1,
			"mtbf_avg_h": 147.5, "mttr_avg_h": 6.75, "pm_compliance": 0.5, "overdue_pm": 1, "open_work_orders": 3,
		}},
		{"process-safety", map[string]float64{
			"tier1_events": 1, "tier2_events": 3, "overdue_moc": 2, "oldest_overdue_moc_days": 30,
			"overdue_proof_tests": 2, "sis_proof_tests_overdue": 1, "safety_system_demands": 4,
		}},
		{"industrial-ot-security", map[string]float64{
			"monitoring_coverage": 0.6, "critical_assets_unpatched": 2, "patch_age_p90_days": 400,
			"internet_exposed_assets": 1, "unapproved_remote_sessions": 1,
		}},
		{"uas-fleet", map[string]float64{
			"fleet_ready_rate": 0.6, "firmware_out_of_date": 2, "batteries_over_cycle_limit": 2,
			"battery_resistance_max_mohm": 45, "failed_landing_rate": 1.0 / 6, "swap_turnaround_p90_min": 9,
		}},
		{"launch-range", map[string]float64{
			"slot_utilisation": 0.5, "slots_held": 2, "pad_turnaround_p90_h": 150, "equipment_ready_ratio": 0.6,
			"critical_equipment_not_ready": 2, "weather_holds": 2, "technical_holds": 1,
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

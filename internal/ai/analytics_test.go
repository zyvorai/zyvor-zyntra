// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package ai

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/zyntra/internal/graph"
)

func analyticalFixture(t *testing.T, n int, f func(int) float64) (*graph.Model, *History) {
	t.Helper()
	m := load(t)
	h := NewHistory(1000)
	pts := make([]Point, n)
	for i := range pts {
		pts[i] = Point{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour), f(i)}
	}
	h.Restore(map[string][]Point{"cost": pts})
	return m, h
}
func TestSeasonalSelectionAndProjections(t *testing.T) {
	m, h := analyticalFixture(t, 241, func(i int) float64 { return 100 + 30*math.Sin(float64(i%24)*math.Pi/12) })
	r := Analytics(m, h, nil)
	k := r.KPIs[2]
	if k.Status != "ready" || k.Selected != "daily-seasonal" || len(k.Projections) != 4 {
		t.Fatalf("%+v", k)
	}
	for _, b := range k.Backtests {
		if b.Method == "daily-seasonal" && b.MAE > 1e-8 {
			t.Fatalf("seasonal MAE %+v", b)
		}
	}
	if math.Abs(k.Projections[0].Value-100) > 1e-8 {
		t.Fatalf("next hour %+v", k.Projections[0])
	}
	if k.Projections[0].Low > k.Projections[0].Value || k.Projections[0].High < k.Projections[0].Value {
		t.Fatal("bad band")
	}
	// Eight daily phase comparisons mean an ordinary daily peak is not unusual.
	if len(r.Anomalies) != 0 {
		t.Fatalf("daily cycle flagged: %+v", r.Anomalies)
	}
}
func TestWeeklySelection(t *testing.T) {
	m, h := analyticalFixture(t, 505, func(i int) float64 { return float64((i%168)/24)*100 + float64(i%24) })
	k := Analytics(m, h, nil).KPIs[2]
	if k.Selected != "weekly-seasonal" {
		t.Fatalf("%+v", k)
	}
}
func TestLinearSelectionAndNoFutureLeakage(t *testing.T) {
	m, h := analyticalFixture(t, 97, func(i int) float64 { return 100 + float64(i)*2 })
	k := Analytics(m, h, nil).KPIs[2]
	if k.Selected != "linear" || math.Abs(k.Projections[0].Value-292) > 1e-8 {
		t.Fatalf("%+v", k)
	}
	v := []float64{}
	for i := 0; i < 96; i++ {
		v = append(v, float64(i))
	}
	v[95] = 1000
	b := backtest(v, "linear")
	if b.MAE < 30 {
		t.Fatalf("last observation leaked into its prediction: %+v", b)
	}
}
func TestGapsAndUnusableInputs(t *testing.T) {
	m, h := analyticalFixture(t, 100, func(i int) float64 { return float64(i) })
	pts := h.Series("cost")
	pts = append(pts[:50], pts[51:]...)
	h.Restore(map[string][]Point{"cost": pts})
	if k := Analytics(m, h, nil).KPIs[2]; k.Status != "warming" || k.HourlySamples != 48 {
		t.Fatalf("gap was filled: %+v", k)
	}
	k := Analytics(m, h, map[string]bool{"cost": true}).KPIs[2]
	if k.Status != "unavailable" || len(k.Projections) != 0 {
		t.Fatalf("stale projected: %+v", k)
	}
}
func TestRobustAnomalyResistsContaminatedBaseline(t *testing.T) {
	m, h := analyticalFixture(t, 50, func(i int) float64 {
		if i == 10 {
			return 10000
		}
		if i == 48 {
			return 130
		}
		return 100 + float64(i%3)
	})
	r := Analytics(m, h, nil)
	if len(r.Anomalies) != 1 || r.Anomalies[0].KPI != "cost" || r.Anomalies[0].Score < 7 {
		t.Fatalf("%+v", r.Anomalies)
	}
}
func TestFlatSeriesUsesBaselineAndFiniteScores(t *testing.T) {
	m, h := analyticalFixture(t, 100, func(int) float64 { return 100 })
	k := Analytics(m, h, nil).KPIs[2]
	if k.Selected != "last-value" {
		t.Fatalf("%+v", k)
	}
	pts := h.Series("cost")
	pts[len(pts)-2].V = 120
	h.Restore(map[string][]Point{"cost": pts})
	r := Analytics(m, h, nil)
	if len(r.Anomalies) != 1 || math.IsInf(r.Anomalies[0].Score, 0) {
		t.Fatalf("%+v", r.Anomalies)
	}
}

func TestAnalyticsAnswerGroundingAndTenantScope(t *testing.T) {
	m, h := analyticalFixture(t, 100, func(i int) float64 { return float64(i) })
	report := Analytics(m, h, nil)
	s := Snapshot{Model: m, Analytics: &report}
	a := (&Engine{}).Ask(context.Background(), "show seasonal forecast accuracy", s)
	if a.Intent != "analytics" || len(a.Grounding) != 1 || a.Grounding[0] != "analytics:cost" || !strings.Contains(a.Text, "24 historical origins") {
		t.Fatalf("%+v", a)
	}
	s.ObjectsOnly = true
	a = (&Engine{}).Ask(context.Background(), "show seasonal forecasts", s)
	if a.Intent != "scope" || strings.Contains(a.Text, "cost") {
		t.Fatalf("tenant sees analytics: %+v", a)
	}
}

func TestExtremeFiniteValuesDoNotBreakJSON(t *testing.T) {
	m, h := analyticalFixture(t, 100, func(int) float64 { return math.MaxFloat64 })
	report := Analytics(m, h, nil)
	if report.KPIs[2].Status != "unavailable" || len(report.Anomalies) != 0 {
		t.Fatalf("%+v", report)
	}
}

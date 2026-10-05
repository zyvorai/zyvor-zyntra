// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package ai

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/zyvorai/zyntra/internal/graph"
)

type Projection struct {
	At     time.Time `json:"at"`
	Hours  int       `json:"hours"`
	Value  float64   `json:"value"`
	Low    float64   `json:"low"`
	High   float64   `json:"high"`
	Breach bool      `json:"breach"`
}
type Backtest struct {
	Method      string  `json:"method"`
	Samples     int     `json:"samples"`
	MAE         float64 `json:"mae"`
	RMSE        float64 `json:"rmse"`
	BaselineMAE float64 `json:"baseline_mae"`
	ErrorRadius float64 `json:"error_radius"`
}
type AnalyticalKPI struct {
	KPI           string       `json:"kpi"`
	Name          string       `json:"name"`
	Unit          string       `json:"unit"`
	Status        string       `json:"status"`
	Reason        string       `json:"reason,omitempty"`
	Observations  int          `json:"observations"`
	HourlySamples int          `json:"hourly_samples"`
	LastSample    *time.Time   `json:"last_sample,omitempty"`
	Backtests     []Backtest   `json:"backtests"`
	Selected      string       `json:"selected,omitempty"`
	Projections   []Projection `json:"projections"`
}
type RobustAnomaly struct {
	KPI      string  `json:"kpi"`
	Name     string  `json:"name"`
	Method   string  `json:"method"`
	Value    float64 `json:"value"`
	Baseline float64 `json:"baseline"`
	Score    float64 `json:"score"`
	Samples  int     `json:"samples"`
	Severity string  `json:"severity"`
	Text     string  `json:"text"`
}
type AnalyticsReport struct {
	KPIs             []AnalyticalKPI `json:"kpis"`
	Anomalies        []RobustAnomaly `json:"anomalies"`
	Persistent       bool            `json:"persistent"`
	PersistenceError string          `json:"persistence_error,omitempty"`
	IntervalNote     string          `json:"interval_note"`
}

// Analytics never changes the simulator or planner. Every model is evaluated
// at 24 rolling origins using training data strictly before the observation.
// Hourly means prevent dense scrapes from dominating the model. Only the
// contiguous recent suffix is used: gaps are never filled with fabricated data.
func Analytics(m *graph.Model, h *History, unusable map[string]bool) AnalyticsReport {
	out := AnalyticsReport{KPIs: []AnalyticalKPI{}, Anomalies: []RobustAnomaly{}, Persistent: h.Persistent(), PersistenceError: h.PersistenceError(), IntervalNote: "Bands are the 90th percentile of absolute one-hour backtest errors, not calibrated multi-horizon prediction intervals."}
	for _, k := range m.KPIs {
		raw := h.Series(k.ID)

		valid := true
		for _, p := range raw {
			if math.Abs(p.V) > 1e100 {
				valid = false
				break
			}
		}
		pts := []Point{}
		if valid {
			pts = hourly(raw)
		}
		row := AnalyticalKPI{KPI: k.ID, Name: k.Name, Unit: k.DisplayUnit(), Observations: len(raw), HourlySamples: len(pts), Backtests: []Backtest{}, Projections: []Projection{}}
		if len(raw) > 0 {
			at := raw[len(raw)-1].T
			row.LastSample = &at
		}

		if !valid {
			row.Status = "unavailable"
			row.Reason = "Input magnitude exceeds the analytical numeric range."
			out.KPIs = append(out.KPIs, row)
			continue
		}
		if unusable[k.ID] {
			row.Status = "unavailable"
			row.Reason = "Current input is stale, missing, held, or failed."
			out.KPIs = append(out.KPIs, row)
			continue
		}
		if a, ok := robust(k, pts); ok {
			out.Anomalies = append(out.Anomalies, a)
		}
		if len(pts) < 72 {
			row.Status = "warming"
			row.Reason = "Need 72 consecutive hourly buckets for a 24-origin backtest."
			out.KPIs = append(out.KPIs, row)
			continue
		}
		values := make([]float64, len(pts))
		for i, p := range pts {
			values[i] = p.V
		}

		methods := []string{"last-value", "linear", "daily-seasonal"}
		if len(values) >= 360 {
			methods = append(methods, "weekly-seasonal")
		}
		best := math.Inf(1)
		var radius float64
		for _, method := range methods {
			b := backtest(values, method)
			row.Backtests = append(row.Backtests, b)
			// Stable ties prefer the simpler last-value model.
			if b.MAE < best-1e-9 {
				best = b.MAE
				row.Selected = method
				radius = b.ErrorRadius
			}
		}
		row.Status = "ready"
		for _, hours := range []int{1, 6, 24, 168} {
			value := predict(values, row.Selected, hours)
			if math.IsNaN(value) || math.IsInf(value, 0) {
				row.Status = "unavailable"
				row.Reason = "Projection overflowed; input scale needs review."
				row.Projections = []Projection{}
				break
			}
			breach := k.Target != nil && ((k.Direction == graph.LowerIsBetter && value > *k.Target) || (k.Direction == graph.HigherIsBetter && value < *k.Target))
			row.Projections = append(row.Projections, Projection{At: pts[len(pts)-1].T.Add(time.Duration(hours) * time.Hour), Hours: hours, Value: value, Low: value - radius, High: value + radius, Breach: breach})
		}
		out.KPIs = append(out.KPIs, row)
	}
	sort.SliceStable(out.Anomalies, func(i, j int) bool { return math.Abs(out.Anomalies[i].Score) > math.Abs(out.Anomalies[j].Score) })
	return out
}

func hourly(raw []Point) []Point {
	// Exclude the newest, still-open bucket to avoid comparing partial hours
	// with complete hours. Its value remains visible in the existing insights.
	if len(raw) == 0 {
		return nil
	}
	lastHour := raw[len(raw)-1].T.UTC().Truncate(time.Hour)
	var out []Point
	var at time.Time
	sum := 0.0
	n := 0
	flush := func() {
		if n > 0 {
			out = append(out, Point{at, sum / float64(n)})
		}
	}
	for _, p := range raw {
		hour := p.T.UTC().Truncate(time.Hour)
		if !hour.Before(lastHour) {
			break
		}
		if n > 0 && !hour.Equal(at) {
			flush()
			sum = 0
			n = 0
		}
		at = hour
		sum += p.V
		n++
	}
	flush()
	start := 0
	for i := 1; i < len(out); i++ {
		if out[i].T.Sub(out[i-1].T) != time.Hour {
			start = i
		}
	}
	return out[start:]
}
func median(values []float64) float64 {
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	n := len(v)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return v[n/2]
	}
	return v[n/2-1]/2 + v[n/2]/2
}
func robust(k graph.KPI, pts []Point) (RobustAnomaly, bool) {
	if len(pts) < 10 {
		return RobustAnomaly{}, false
	}
	last := pts[len(pts)-1]
	base := []float64{}
	method := "median-mad"
	for i := len(pts) - 25; i >= 0; i -= 24 {
		base = append(base, pts[i].V)
	}
	if len(base) >= 8 {
		method = "daily-phase-mad"
	} else {
		base = base[:0]
		start := max(0, len(pts)-49)
		for _, p := range pts[start : len(pts)-1] {
			base = append(base, p.V)
		}
	}
	center := median(base)
	dev := make([]float64, len(base))
	for i, v := range base {
		dev[i] = math.Abs(v - center)
	}
	scale := 1.4826 * median(dev)
	delta := last.V - center
	if math.Abs(delta) <= 1e-9*math.Max(1, math.Abs(center)) {
		return RobustAnomaly{}, false
	}
	score := clampInf(math.Copysign(math.Inf(1), delta))
	if scale > 1e-12 {
		score = clampInf(delta / scale)
	}
	if math.Abs(score) <= 3.5 {
		return RobustAnomaly{}, false
	}
	severity := "warning"
	if math.Abs(score) > 7 {
		severity = "critical"
	}
	return RobustAnomaly{KPI: k.ID, Name: k.Name, Method: method, Value: last.V, Baseline: center, Score: score, Samples: len(base), Severity: severity, Text: fmt.Sprintf("%s completed-hour mean %.4g differs from the robust baseline %.4g (%s, %d baseline hours).", k.Name, last.V, center, method, len(base))}, true
}
func predict(v []float64, method string, steps int) float64 {
	n := len(v)
	switch method {
	case "linear":
		// Limit the trend fit to the recent two days, including every origin.
		v = v[max(0, n-48):]
		n = len(v)
		var sx, sy, sxx, sxy float64
		for i, y := range v {
			x := float64(i)
			sx += x
			sy += y
			sxx += x * x
			sxy += x * y
		}
		den := float64(n)*sxx - sx*sx
		slope := (float64(n)*sxy - sx*sy) / den
		return sy/float64(n) + slope*(float64(n-1+steps)-sx/float64(n))
	case "daily-seasonal", "weekly-seasonal":
		period := 24
		if method == "weekly-seasonal" {
			period = 168
		}
		return v[n-period+(steps-1)%period]
	default:
		return v[n-1]
	}
}
func backtest(v []float64, method string) Backtest {
	b := Backtest{Method: method, Samples: 24}
	errors := make([]float64, 0, 24)
	for origin := len(v) - 24; origin < len(v); origin++ {
		err := math.Abs(predict(v[:origin], method, 1) - v[origin])
		errors = append(errors, err)
		b.MAE += err
		b.RMSE += err * err
		b.BaselineMAE += math.Abs(v[origin-1] - v[origin])
	}
	b.MAE /= 24
	b.RMSE = math.Sqrt(b.RMSE / 24)
	b.BaselineMAE /= 24
	sort.Float64s(errors)
	b.ErrorRadius = errors[int(math.Ceil(.9*float64(len(errors))))-1]
	return b
}

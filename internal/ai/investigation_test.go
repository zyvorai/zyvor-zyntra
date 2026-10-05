// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package ai

import (
	"encoding/json"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"
)

var investigationNow = time.Date(2026, 10, 5, 12, 30, 0, 0, time.UTC)
var investigationCatalog = []Metric{{ID: "latency", Name: "Latency", Unit: "ms"}, {ID: "queue", Name: "Queue", Unit: "jobs"}, {ID: "inverse", Name: "Inverse", Unit: "percent"}}

func investigationQuery() Investigation {
	return Investigation{Metric: "latency", WindowHours: 72, RecentHours: 6, MaxLagHours: 6}
}
func levelHistory(values []float64, now time.Time) []Point {
	end := now.UTC().Truncate(time.Hour)
	out := []Point{}
	for i, v := range values {
		out = append(out, Point{T: end.Add(time.Duration(i-len(values))*time.Hour + 10*time.Minute), V: v})
	}
	return out
}
func TestInvestigationSustainedShiftAndEvidence(t *testing.T) {
	for _, sign := range []float64{-1, 1} {
		values := make([]float64, 72)
		for i := range values {
			values[i] = 100 + float64(i%3-1)
			if i >= 66 {
				values[i] += sign * 40
			}
		}
		history := map[string][]Point{"latency": levelHistory(values, investigationNow)}
		history["latency"] = append(history["latency"], Point{T: investigationNow, V: 999999}) // open hour ignored
		report, err := Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
		if err != nil {
			t.Fatal(err)
		}
		s := report.Shift
		if s.Status != "shift" || s.BaselineHours != 66 || s.RecentHours != 6 || s.BeyondThreshold != 6 || math.Abs(*s.Delta-sign*40) > 1e-9 || len(report.Hours) != 72 || len(report.SHA256) != 64 {
			t.Fatalf("%+v", report)
		}
		if sign < 0 && s.Direction != "down" || sign > 0 && s.Direction != "up" {
			t.Fatal(s.Direction)
		}
		if !report.End.Equal(investigationNow.Truncate(time.Hour)) || !s.RecentStart.Equal(report.End.Add(-6*time.Hour)) {
			t.Fatal("wrong window")
		}
		repeated, _ := Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
		if report.SHA256 != repeated.SHA256 {
			t.Fatal("unstable evidence")
		}
	}
}
func TestInvestigationOutlierFlatAndMissingHours(t *testing.T) {
	values := make([]float64, 72)
	for i := range values {
		values[i] = 100
	}
	values[71] = 999
	history := map[string][]Point{"latency": levelHistory(values, investigationNow)}
	report, err := Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
	if err != nil || report.Shift.Status != "stable" {
		t.Fatalf("outlier %+v %v", report.Shift, err)
	}
	history["latency"] = history["latency"][:71]
	report, _ = Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
	if report.Shift.Status != "warming" || report.Shift.RecentHours != 5 {
		t.Fatalf("recent gap %+v", report.Shift)
	}
	history["latency"] = levelHistory(values, investigationNow)[60:]
	report, _ = Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
	if report.Shift.Status != "warming" || report.Shift.BaselineHours != 6 {
		t.Fatalf("short baseline %+v", report.Shift)
	}
	// Flat zero baseline remains finite and detects a genuine sustained level change.
	for i := range values {
		values[i] = 0
		if i >= 66 {
			values[i] = 10
		}
	}
	report, _ = Investigate(investigationQuery(), investigationCatalog, map[string][]Point{"latency": levelHistory(values, investigationNow)}, investigationNow)
	if report.Shift.Status != "shift" || !finite(*report.Shift.Threshold) {
		t.Fatalf("flat baseline %+v", report.Shift)
	}
}
func delayedHistory(n, lag int) map[string][]Point {
	random := rand.New(rand.NewSource(731))
	increments := make([]float64, n)
	driver, target, inverse := make([]float64, n), make([]float64, n), make([]float64, n)
	for i := 1; i < n; i++ {
		increments[i] = float64(random.Intn(21) - 10)
		driver[i] = driver[i-1] + increments[i]
		inverse[i] = -driver[i]
		target[i] = target[i-1]
		if i >= lag {
			target[i] += 3 * increments[i-lag]
		}
	}
	return map[string][]Point{"latency": levelHistory(target, investigationNow), "queue": levelHistory(driver, investigationNow), "inverse": levelHistory(inverse, investigationNow)}
}
func TestInvestigationFindsPositiveAndInverseLeadingChanges(t *testing.T) {
	history := delayedHistory(72, 2)
	report, err := Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Comparisons) != 2 {
		t.Fatal(report)
	}
	for _, row := range report.Comparisons {
		if row.Status != "associated" || row.LagHours != 2 || row.Pairs != 69 || row.TestedLags != 7 || math.Abs(*row.Correlation) < .999999 || len(row.SHA256) != 64 {
			t.Fatalf("%+v", row)
		}
		if row.ID == "inverse" && *row.Correlation >= 0 {
			t.Fatal("lost inverse relationship")
		}
	}
	if !strings.Contains(report.Caveat, "not causes") {
		t.Fatal("causal claim")
	}
}
func TestInvestigationAlignsRealTimesAndNeverBridgesGaps(t *testing.T) {
	history := delayedHistory(72, 2)
	// Remove three hours from one candidate, not three array positions from both.
	series := history["queue"]
	history["queue"] = append(series[:35:35], series[38:]...)
	report, _ := Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
	for _, row := range report.Comparisons {
		if row.ID == "queue" && (row.LagHours != 2 || row.Pairs != 65 || math.Abs(*row.Correlation) < .999999) {
			t.Fatalf("%+v", row)
		}
	}
	// Shift a candidate's timestamps far outside the target window: no index zip.
	for i := range history["queue"] {
		history["queue"][i].T = history["queue"][i].T.Add(-1000 * time.Hour)
	}
	report, _ = Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
	for _, row := range report.Comparisons {
		if row.ID == "queue" && (row.Correlation != nil || row.Status != "warming" || row.Pairs != 0) {
			t.Fatal(row)
		}
	}
}
func TestInvestigationRejectsConstantDeltasAndSparsePairs(t *testing.T) {
	values := make([]float64, 72)
	for i := range values {
		values[i] = float64(i) * 10
	}
	history := map[string][]Point{"latency": levelHistory(values, investigationNow), "queue": levelHistory(values, investigationNow)}
	report, _ := Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
	for _, row := range report.Comparisons {
		if row.Correlation != nil {
			t.Fatal("shared linear trends interpreted as associated changes")
		}
	}
	history = delayedHistory(24, 2)
	report, _ = Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
	for _, row := range report.Comparisons {
		if row.Status != "warming" || row.Correlation != nil {
			t.Fatal("too few aligned pairs accepted")
		}
	}
}
func TestInvestigationValidationCapAndNumericBounds(t *testing.T) {
	for _, body := range []string{`{"metric":"secret","window_hours":72,"recent_hours":6,"max_lag_hours":6}`, `{"metric":"latency","window_hours":721,"recent_hours":6,"max_lag_hours":6}`, `{"metric":"latency","window_hours":72,"recent_hours":2,"max_lag_hours":6}`, `{"metric":"latency","window_hours":24,"recent_hours":24,"max_lag_hours":6}`, `{"metric":"latency","window_hours":72,"recent_hours":6,"max_lag_hours":13}`, `{"metric":"latency","window_hours":72,"recent_hours":6,"max_lag_hours":6,"candidates":["queue","queue"]}`, `{"metric":"latency","window_hours":72,"recent_hours":6,"max_lag_hours":6,"candidates":["latency"]}`, `{"metric":"latency","window_hours":72,"recent_hours":6,"max_lag_hours":6,"candidates":["secret"]}`, `{"metric":"latency","window_hours":72,"recent_hours":6,"max_lag_hours":6,"sql":"DROP TABLE x"}`, `{} {}`, `null`} {
		q, err := DecodeInvestigation(strings.NewReader(body))
		if err == nil {
			err = q.Validate(investigationCatalog)
		}
		if err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	catalog := append([]Metric(nil), investigationCatalog...)
	for i := 0; i < 70; i++ {
		catalog = append(catalog, Metric{ID: fmtID(i)})
	}
	report, err := Investigate(investigationQuery(), catalog, nil, investigationNow)
	if err != nil || !report.CandidatesTruncated || len(report.Comparisons) != 64 || report.CandidateTotal != 72 {
		t.Fatalf("cap %+v %v", report, err)
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), 1e101} {
		report, err = Investigate(investigationQuery(), investigationCatalog, map[string][]Point{"latency": {{T: investigationNow.Add(-time.Hour), V: bad}}}, investigationNow)
		if err != nil || report.Shift.Status != "unavailable" {
			t.Fatal("unsafe target accepted")
		}
		if _, err := json.Marshal(report); err != nil {
			t.Fatal("non-finite JSON", err)
		}
	}
	history := delayedHistory(72, 2)
	for id, pts := range history {
		for i := range pts {
			pts[i].V *= 1e97
		}
		history[id] = pts
	}
	report, err = Investigate(investigationQuery(), investigationCatalog, history, investigationNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := json.Marshal(report); err != nil {
		t.Fatal(err)
	}
}
func fmtID(i int) string { return "candidate-" + strconv.Itoa(i) }
func TestInvestigationAnswerNamesOneTargetAndNeverCallsGateway(t *testing.T) {
	a := InvestigationAnswer("investigate latency", investigationCatalog, delayedHistory(72, 2), investigationNow)
	if a.Investigation == nil || a.Intent != "investigation" || a.Mode != "heuristic" || !strings.Contains(a.Text, "not causes") {
		t.Fatalf("%+v", a)
	}
	for _, question := range []string{"investigate secret", "compare latency and queue"} {
		a = InvestigationAnswer(question, investigationCatalog, nil, investigationNow)
		if a.Investigation != nil {
			t.Fatalf("ambiguous target %s", question)
		}
	}
}

func TestInvestigationEvidenceAndHistorySelectionAreBounded(t *testing.T) {
	base := delayedHistory(72, 2)
	catalog := []Metric{{ID: "latency"}}
	history := map[string][]Point{"latency": base["latency"]}
	for i := 0; i < 70; i++ {
		id := fmtID(i)
		catalog = append(catalog, Metric{ID: id})
		history[id] = base["queue"]
	}
	ids, err := InvestigationMetrics(investigationQuery(), catalog)
	if err != nil || len(ids) != 65 || ids[0] != "latency" {
		t.Fatalf("bounded copy %v %d", err, len(ids))
	}
	report, err := Investigate(investigationQuery(), catalog, history, investigationNow)
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range report.Comparisons {
		if i < 5 {
			if len(row.Evidence) != row.Pairs || investigationHash(row.Evidence) != row.SHA256 {
				t.Fatal("incomplete pair evidence")
			}
			for _, p := range row.Evidence {
				if p.Target != 3*p.Candidate {
					t.Fatal("evidence did not align target with candidate")
				}
			}
		} else if len(row.Evidence) != 0 {
			t.Fatal("unbounded pair response")
		}
	}
	q := investigationQuery()
	q.Candidates = []string{"candidate-3"}
	ids, err = InvestigationMetrics(q, catalog)
	if err != nil || len(ids) != 2 || ids[1] != "candidate-3" {
		t.Fatal("explicit selection not honored")
	}
	q.Candidates = []string{"secret"}
	if _, err = InvestigationMetrics(q, catalog); err == nil {
		t.Fatal("unauthorized history selected")
	}
}
func TestLatestPointReturnsAValueCopy(t *testing.T) {
	h := NewHistory(100)
	if _, ok := h.LatestPoint("empty"); ok {
		t.Fatal("empty series had a point")
	}
	h.Restore(map[string][]Point{"x": {{T: investigationNow, V: 3}}})
	last, ok := h.LatestPoint("x")
	if !ok || last.V != 3 {
		t.Fatal("missing latest point")
	}
	last.V = 999
	again, _ := h.LatestPoint("x")
	if again.V != 3 {
		t.Fatal("caller changed stored history")
	}
}

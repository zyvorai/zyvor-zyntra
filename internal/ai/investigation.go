// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package ai

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"
)

// Investigation selects one target and bounded, authorized candidate metrics.
// Lag is nonnegative: candidate change at t-lag is compared with target at t.
type Investigation struct {
	Metric      string   `json:"metric"`
	WindowHours int      `json:"window_hours"`
	RecentHours int      `json:"recent_hours"`
	MaxLagHours int      `json:"max_lag_hours"`
	Candidates  []string `json:"candidates,omitempty"`
}

const maxInvestigationCandidates = 64
const minRelationshipPairs = 24

func DecodeInvestigation(r io.Reader) (Investigation, error) {
	var q Investigation
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return q, fmt.Errorf("invalid investigation JSON")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return q, fmt.Errorf("investigation must contain one JSON object")
	}
	return q, nil
}
func (q Investigation) Validate(catalog []Metric) error {
	allowed := map[string]bool{}
	for _, m := range catalog {
		allowed[m.ID] = true
	}
	if !allowed[q.Metric] {
		return fmt.Errorf("metric unavailable")
	}
	if q.WindowHours < 24 || q.WindowHours > 720 {
		return fmt.Errorf("window_hours must be 24-720")
	}
	if q.RecentHours < 3 || q.RecentHours > 24 || q.RecentHours+12 > q.WindowHours {
		return fmt.Errorf("recent_hours must be 3-24 with at least 12 baseline hours in the window")
	}
	if q.MaxLagHours < 0 || q.MaxLagHours > 12 {
		return fmt.Errorf("max_lag_hours must be 0-12")
	}
	if len(q.Candidates) > maxInvestigationCandidates {
		return fmt.Errorf("select at most 64 candidate metrics")
	}
	seen := map[string]bool{}
	for _, id := range q.Candidates {
		if !allowed[id] || id == q.Metric || seen[id] {
			return fmt.Errorf("candidate unavailable, repeated or equal to target")
		}
		seen[id] = true
	}
	return nil
}

type ObservedHour struct {
	Start   time.Time `json:"start"`
	Mean    float64   `json:"mean"`
	Samples int       `json:"samples"`
}
type ShiftEvidence struct {
	Status          string     `json:"status"` // shift | stable | warming | unavailable
	Reason          string     `json:"reason,omitempty"`
	BaselineHours   int        `json:"baseline_hours"`
	RecentHours     int        `json:"recent_hours"`
	BaselineStart   *time.Time `json:"baseline_start,omitempty"`
	RecentStart     time.Time  `json:"recent_start"`
	BaselineMedian  *float64   `json:"baseline_median,omitempty"`
	RecentMedian    *float64   `json:"recent_median,omitempty"`
	Delta           *float64   `json:"delta,omitempty"`
	Threshold       *float64   `json:"threshold,omitempty"`
	BeyondThreshold int        `json:"beyond_threshold"`
	RequiredHours   int        `json:"required_hours"`
	Direction       string     `json:"direction,omitempty"`
}
type Relationship struct {
	Metric
	Status      string       `json:"status"` // associated | weak | warming | unavailable
	Reason      string       `json:"reason,omitempty"`
	Correlation *float64     `json:"correlation,omitempty"`
	LagHours    int          `json:"lag_hours"`
	Pairs       int          `json:"pairs"`
	TestedLags  int          `json:"tested_lags"`
	Start       *time.Time   `json:"start,omitempty"`
	End         *time.Time   `json:"end,omitempty"`
	SHA256      string       `json:"sha256,omitempty"`
	Evidence    []ChangePair `json:"evidence,omitempty"`
}
type InvestigationReport struct {
	Query               Investigation  `json:"query"`
	Target              Metric         `json:"target"`
	Start               time.Time      `json:"start"`
	End                 time.Time      `json:"end"` // exclusive, never includes the current partial hour
	Shift               ShiftEvidence  `json:"shift"`
	Hours               []ObservedHour `json:"hours"`
	SHA256              string         `json:"sha256"`
	Comparisons         []Relationship `json:"comparisons"`
	CandidateTotal      int            `json:"candidate_total"`
	CandidatesTruncated bool           `json:"candidates_truncated"`
	Method              string         `json:"method"`
	Caveat              string         `json:"caveat"`
}

const investigationMethod = "Completed UTC hourly sample means only. Shift compares the recent-hour median with the preceding consecutive baseline median; threshold = max(3 × 1.4826 × baseline MAD, 0.000001 × absolute baseline median, 0.000000000001). At least 80% of recent hours must exceed the threshold in the same direction. Relationships use Pearson correlation of consecutive-hour changes, with at least 24 exactly time-aligned pairs at each eligible lag. Missing hours are never filled."
const investigationCaveat = "Exploratory associations, not causes or statistical significance. The strongest eligible lag is selected after testing multiple lags and metrics; seasonal cycles, shared inputs, outliers and chance can produce high correlations. Positive lag means the candidate change preceded the target change; it does not prove prediction or causation. No actions or model edges are changed."

func Investigate(q Investigation, catalog []Metric, history map[string][]Point, now time.Time) (InvestigationReport, error) {
	if err := q.Validate(catalog); err != nil {
		return InvestigationReport{}, err
	}
	end := now.UTC().Truncate(time.Hour)
	start := end.Add(-time.Duration(q.WindowHours) * time.Hour)
	byID := map[string]Metric{}
	for _, m := range catalog {
		byID[m.ID] = m
	}
	report := InvestigationReport{Query: q, Target: byID[q.Metric], Start: start, End: end, Hours: []ObservedHour{}, Comparisons: []Relationship{}, Method: investigationMethod, Caveat: investigationCaveat}
	hours, err := investigationHours(history[q.Metric], start, end)
	if err != nil {
		report.Shift = ShiftEvidence{Status: "unavailable", Reason: err.Error(), RecentStart: end.Add(-time.Duration(q.RecentHours) * time.Hour)}
		return report, nil
	}
	report.Hours = hours
	report.SHA256 = investigationHash(hours)
	report.Shift = detectShift(hours, end, q.RecentHours)
	candidates, total := investigationCandidates(q, catalog)
	report.CandidateTotal = total
	report.CandidatesTruncated = total > maxInvestigationCandidates
	// Retain the actual considered candidates for review and repeatability.
	report.Query.Candidates = candidates
	targetDeltas := hourChanges(hours)
	for _, id := range candidates {
		row := Relationship{Metric: byID[id], Status: "warming", Reason: "Need at least 24 aligned consecutive-hour change pairs with variance in both metrics."}
		candidateHours, err := investigationHours(history[id], start, end)
		if err != nil {
			row.Status = "unavailable"
			row.Reason = err.Error()
			report.Comparisons = append(report.Comparisons, row)
			continue
		}
		changes := hourChanges(candidateHours)
		var bestPairs []ChangePair
		maxPairs := 0
		for lag := 0; lag <= q.MaxLagHours; lag++ {
			pairs := alignedChanges(targetDeltas, changes, lag)
			maxPairs = max(maxPairs, len(pairs))
			if len(pairs) < minRelationshipPairs {
				continue
			}
			r, ok := changeCorrelation(pairs)
			if !ok {
				continue
			}
			row.TestedLags++
			if row.Correlation == nil || math.Abs(r) > math.Abs(*row.Correlation)+1e-12 {
				row.Correlation = &r
				row.LagHours = lag
				bestPairs = pairs
			}
		}
		row.Pairs = maxPairs
		if row.Correlation != nil {
			row.Pairs = len(bestPairs)
			row.Start = &bestPairs[0].At
			row.End = &bestPairs[len(bestPairs)-1].At
			row.SHA256 = investigationHash(bestPairs)
			row.Evidence = bestPairs
			row.Status = "weak"
			row.Reason = "No eligible lag reached the exploratory absolute-correlation threshold of 0.6."
			if math.Abs(*row.Correlation) >= 0.6 {
				row.Status = "associated"
				row.Reason = "Observed change association; verify shared inputs and operational events before interpreting."
			}
		}
		report.Comparisons = append(report.Comparisons, row)
	}
	sort.SliceStable(report.Comparisons, func(i, j int) bool {
		a, b := report.Comparisons[i], report.Comparisons[j]
		if a.Correlation == nil && b.Correlation == nil {
			return a.ID < b.ID
		}
		if a.Correlation == nil {
			return false
		}
		if b.Correlation == nil {
			return true
		}
		if math.Abs(*a.Correlation) == math.Abs(*b.Correlation) {
			return a.ID < b.ID
		}
		return math.Abs(*a.Correlation) > math.Abs(*b.Correlation)
	})
	// Return full aligned-pair evidence for at most five scored comparisons.
	// Remaining rows retain counts, lag, bounds and the complete pair fingerprint.
	for i := range report.Comparisons {
		if i >= 5 {
			report.Comparisons[i].Evidence = nil
		}
	}
	return report, nil
}
func investigationHash(v any) string {
	b, _ := json.Marshal(v)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func investigationHours(raw []Point, start, end time.Time) ([]ObservedHour, error) {
	buckets := map[time.Time][]Point{}
	for _, p := range raw {
		if p.T.Before(start) || !p.T.Before(end) {
			continue
		}
		if !finite(p.V) || math.Abs(p.V) > 1e100 {
			return nil, fmt.Errorf("Observed values exceed the finite analytical range (absolute value at most 1e100).")
		}
		at := p.T.UTC().Truncate(time.Hour)
		buckets[at] = append(buckets[at], p)
	}
	out := []ObservedHour{}
	for at, p := range buckets {
		out = append(out, ObservedHour{at, sampleMean(p), len(p)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}
func detectShift(hours []ObservedHour, end time.Time, recent int) ShiftEvidence {
	result := ShiftEvidence{Status: "warming", RecentStart: end.Add(-time.Duration(recent) * time.Hour), RequiredHours: int(math.Ceil(.8 * float64(recent)))}
	byTime := map[time.Time]float64{}
	for _, h := range hours {
		byTime[h.Start] = h.Mean
	}
	recentValues := []float64{}
	for at := result.RecentStart; at.Before(end); at = at.Add(time.Hour) {
		if v, ok := byTime[at]; ok {
			recentValues = append(recentValues, v)
		}
	}
	result.RecentHours = len(recentValues)
	if len(recentValues) != recent {
		result.Reason = "Every recent completed hour must be observed; missing recent hours cannot establish a sustained shift."
		return result
	}
	baseline := []float64{}
	for at := result.RecentStart.Add(-time.Hour); ; at = at.Add(-time.Hour) {
		v, ok := byTime[at]
		if !ok {
			break
		}
		baseline = append(baseline, v)
		t := at
		result.BaselineStart = &t
	}
	result.BaselineHours = len(baseline)
	if len(baseline) < 12 {
		result.Reason = "Need at least 12 consecutive baseline hours immediately before the recent period."
		return result
	}
	center := median(baseline)
	recentMedian := median(recentValues)
	deviations := make([]float64, len(baseline))
	for i, v := range baseline {
		deviations[i] = math.Abs(v - center)
	}
	threshold := math.Max(3*1.4826*median(deviations), math.Max(math.Abs(center)*1e-6, 1e-12))
	delta := recentMedian - center
	result.BaselineMedian = &center
	result.RecentMedian = &recentMedian
	result.Delta = &delta
	result.Threshold = &threshold
	for _, v := range recentValues {
		if delta > 0 && v-center > threshold || delta < 0 && center-v > threshold {
			result.BeyondThreshold++
		}
	}
	result.Status = "stable"
	result.Reason = "No sustained level shift under the configured recent-hour rule."
	if math.Abs(delta) > threshold && result.BeyondThreshold >= result.RequiredHours {
		result.Status = "shift"
		result.Direction = "up"
		if delta < 0 {
			result.Direction = "down"
		}
		result.Reason = "Recent-hour level moved beyond the robust baseline threshold in a sustained direction."
	}
	return result
}

type ChangePair struct {
	At        time.Time `json:"at"`
	Candidate float64   `json:"candidate_change"`
	Target    float64   `json:"target_change"`
}

func hourChanges(hours []ObservedHour) map[time.Time]float64 {
	out := map[time.Time]float64{}
	for i := 1; i < len(hours); i++ {
		if hours[i].Start.Sub(hours[i-1].Start) == time.Hour {
			out[hours[i].Start] = hours[i].Mean - hours[i-1].Mean
		}
	}
	return out
}
func alignedChanges(target, candidate map[time.Time]float64, lag int) []ChangePair {
	out := []ChangePair{}
	for at, v := range target {
		if c, ok := candidate[at.Add(-time.Duration(lag)*time.Hour)]; ok {
			out = append(out, ChangePair{at, c, v})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}
func changeCorrelation(pairs []ChangePair) (float64, bool) {
	// Normalize each vector independently before centering; this avoids overflow
	// without allowing constant or effectively constant deltas to look correlated.
	var sx, sy float64
	for _, p := range pairs {
		sx = math.Max(sx, math.Abs(p.Candidate))
		sy = math.Max(sy, math.Abs(p.Target))
	}
	if sx == 0 || sy == 0 {
		return 0, false
	}
	var mx, my float64
	for _, p := range pairs {
		mx += p.Candidate / sx
		my += p.Target / sy
	}
	mx /= float64(len(pairs))
	my /= float64(len(pairs))
	var xx, yy, xy float64
	for _, p := range pairs {
		x, y := p.Candidate/sx-mx, p.Target/sy-my
		xx += x * x
		yy += y * y
		xy += x * y
	}
	if xx < 1e-12 || yy < 1e-12 {
		return 0, false
	}
	r := xy / math.Sqrt(xx*yy)
	return math.Max(-1, math.Min(1, r)), finite(r)
}

// InvestigationAnswer never sends measurements to an LLM or rewrites results.
// Explicitly selecting this scope prevents a speculative "why" answer from
// being presented as a root cause or an instruction to execute an action.
func InvestigationAnswer(question string, catalog []Metric, history map[string][]Point, now time.Time) Answer {
	a := Answer{Intent: "investigation", Mode: "heuristic", Grounding: []string{}}
	q, questionErr := InvestigationQuestion(question, catalog)
	if questionErr != nil {
		a.Text = "Name exactly one permitted target metric to investigate, for example: investigate <metric ID>. The investigation uses the last 168 completed UTC hours and compares the most recent 6 hours with the preceding baseline."
		return a
	}
	report, err := Investigate(q, catalog, history, now)
	if err != nil {
		a.Text = "The investigation could not be formed: " + err.Error()
		return a
	}
	a.Investigation = &report
	lines := []string{fmt.Sprintf("%s: %s. %s", report.Target.ID, report.Shift.Status, report.Shift.Reason)}
	if report.Shift.Delta != nil {
		lines = append(lines, fmt.Sprintf("Baseline median %.6g %s; recent median %.6g %s; change %.6g %s.", *report.Shift.BaselineMedian, report.Target.Unit, *report.Shift.RecentMedian, report.Target.Unit, *report.Shift.Delta, report.Target.Unit))
	}
	count := 0
	for _, row := range report.Comparisons {
		if row.Status == "associated" && count < 5 {
			lines = append(lines, fmt.Sprintf("%s: change correlation %.3f at %dh candidate lead (%d aligned pairs).", row.ID, *row.Correlation, row.LagHours, row.Pairs))
			count++
		}
	}
	if count == 0 {
		lines = append(lines, "No eligible candidate reached the exploratory association threshold.")
	}
	if report.Target.Warning != "" {
		lines = append(lines, report.Target.Warning)
	}
	lines = append(lines, report.Caveat)
	a.Text = strings.Join(lines, "\n")
	a.Grounding = append(a.Grounding, "kpi:"+report.Target.ID+"#hourly-sha256:"+report.SHA256)
	return a
}

func investigationCandidates(q Investigation, catalog []Metric) ([]string, int) {
	candidates := append([]string(nil), q.Candidates...)
	if len(candidates) == 0 {
		for _, m := range catalog {
			if m.ID != q.Metric {
				candidates = append(candidates, m.ID)
			}
		}
		sort.Strings(candidates)
	}
	total := len(candidates)
	if total > maxInvestigationCandidates {
		candidates = candidates[:maxInvestigationCandidates]
	}
	return candidates, total
}

// InvestigationMetrics bounds the history copy to target plus 64 authorized
// candidates. Validate before reading any data, including caller-supplied IDs.
func InvestigationMetrics(q Investigation, catalog []Metric) ([]string, error) {
	if err := q.Validate(catalog); err != nil {
		return nil, err
	}
	ids, _ := investigationCandidates(q, catalog)
	return append([]string{q.Metric}, ids...), nil
}

func InvestigationQuestion(question string, catalog []Metric) (Investigation, error) {
	matches := []string{}
	for _, m := range catalog {
		if containsMetric(strings.ToLower(question), m.ID) || containsMetric(strings.ToLower(question), m.Name) {
			matches = append(matches, m.ID)
		}
	}
	if len(matches) != 1 {
		return Investigation{}, fmt.Errorf("name exactly one permitted target metric")
	}
	return Investigation{Metric: matches[0], WindowHours: 168, RecentHours: 6, MaxLagHours: 6}, nil
}

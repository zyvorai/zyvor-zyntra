// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package planning

import (
	"fmt"
	"sort"
	"time"
)

// Event is an observed process transition, not an instruction to execute.
type Event struct {
	ID       string    `json:"id"`
	Case     string    `json:"case"`
	Activity string    `json:"activity"`
	At       time.Time `json:"at"`
}
type Transition struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	Count       int     `json:"count"`
	MeanSeconds float64 `json:"mean_seconds"`
	P95Seconds  float64 `json:"p95_seconds"`
}
type ProcessReport struct {
	Cases                   int          `json:"cases"`
	Events                  int          `json:"events"`
	ReworkCases             int          `json:"rework_cases"`
	MeanObservedSpanSeconds float64      `json:"mean_observed_span_seconds"`
	Transitions             []Transition `json:"transitions"`
	Note                    string       `json:"note"`
}

func Mine(events []Event) (ProcessReport, error) {
	out := ProcessReport{Transitions: []Transition{}, Note: "Observed event-to-event time includes waiting and work; it is not measured activity duration. Case spans may be incomplete."}
	if len(events) == 0 || len(events) > 10000 {
		return out, fmt.Errorf("provide 1..10000 events")
	}
	cases := map[string][]Event{}
	ids := map[string]bool{}
	for _, e := range events {
		if e.ID == "" || ids[e.ID] || e.Case == "" || e.Activity == "" || e.At.IsZero() {
			return out, fmt.Errorf("events require unique IDs, case, activity and timestamp")
		}
		ids[e.ID] = true
		cases[e.Case] = append(cases[e.Case], e)
	}
	type pair struct{ from, to string }
	durations := map[pair][]float64{}
	for _, rows := range cases {
		sort.Slice(rows, func(i, j int) bool { return rows[i].At.Before(rows[j].At) })
		seen := map[string]bool{}
		rework := false
		for i, e := range rows {
			if seen[e.Activity] {
				rework = true
			}
			seen[e.Activity] = true
			if i > 0 {
				prev := rows[i-1]
				if e.At.Equal(prev.At) {
					return out, fmt.Errorf("case %s has ambiguous simultaneous events", e.Case)
				}
				key := pair{prev.Activity, e.Activity}
				durations[key] = append(durations[key], e.At.Sub(prev.At).Seconds())
			}
		}
		if rework {
			out.ReworkCases++
		}
		out.MeanObservedSpanSeconds += rows[len(rows)-1].At.Sub(rows[0].At).Seconds()
	}
	out.Cases = len(cases)
	out.Events = len(events)
	out.MeanObservedSpanSeconds /= float64(out.Cases)
	for key, ds := range durations {
		sort.Float64s(ds)
		sum := 0.0
		for _, v := range ds {
			sum += v
		}
		idx := (95*len(ds)+99)/100 - 1
		out.Transitions = append(out.Transitions, Transition{key.from, key.to, len(ds), sum / float64(len(ds)), ds[idx]})
	}
	sort.Slice(out.Transitions, func(i, j int) bool {
		a, b := out.Transitions[i], out.Transitions[j]
		if a.MeanSeconds != b.MeanSeconds {
			return a.MeanSeconds > b.MeanSeconds
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.To < b.To
	})
	return out, nil
}

type Experiment struct {
	Metric          string    `json:"metric"`
	Unit            string    `json:"unit"`
	HigherIsBetter  bool      `json:"higher_is_better"`
	TreatmentBefore []float64 `json:"treatment_before"`
	TreatmentAfter  []float64 `json:"treatment_after"`
	ControlBefore   []float64 `json:"control_before,omitempty"`
	ControlAfter    []float64 `json:"control_after,omitempty"`
	Confounders     []string  `json:"confounders"`
}
type BenefitReport struct {
	Metric         string   `json:"metric"`
	Unit           string   `json:"unit"`
	ObservedChange float64  `json:"observed_change"`
	ControlChange  *float64 `json:"control_change,omitempty"`
	AdjustedChange *float64 `json:"adjusted_change,omitempty"`
	Improvement    float64  `json:"improvement"`
	Confounders    []string `json:"confounders"`
	Note           string   `json:"note"`
}

func Measure(e Experiment) (BenefitReport, error) {
	out := BenefitReport{Metric: e.Metric, Unit: e.Unit, Confounders: e.Confounders, Note: "Descriptive comparison only. Control adjustment assumes comparable groups and parallel trends; neither causality nor statistical significance is established."}
	mean := func(values []float64) (float64, error) {
		if len(values) < 2 || len(values) > 10000 {
			return 0, fmt.Errorf("each supplied period requires 2..10000 measurements")
		}
		sum := 0.0
		for _, v := range values {
			if !finite(v) {
				return 0, fmt.Errorf("non-finite or excessive measurement")
			}
			sum += v
		}
		return sum / float64(len(values)), nil
	}
	if e.Metric == "" || e.Unit == "" {
		return out, fmt.Errorf("metric and unit required")
	}
	before, err := mean(e.TreatmentBefore)
	if err != nil {
		return out, err
	}
	after, err := mean(e.TreatmentAfter)
	if err != nil {
		return out, err
	}
	out.ObservedChange = after - before
	out.Improvement = out.ObservedChange
	if len(e.ControlBefore) > 0 || len(e.ControlAfter) > 0 {
		cb, err := mean(e.ControlBefore)
		if err != nil {
			return out, err
		}
		ca, err := mean(e.ControlAfter)
		if err != nil {
			return out, err
		}
		change := ca - cb
		adjusted := out.ObservedChange - change
		out.ControlChange = &change
		out.AdjustedChange = &adjusted
		out.Improvement = adjusted
	}
	if !e.HigherIsBetter {
		out.Improvement = -out.Improvement
	}
	return out, nil
}

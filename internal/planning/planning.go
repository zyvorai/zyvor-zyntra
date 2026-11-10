// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package planning evaluates bounded portfolios without executing actions.
package planning

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/zyvorai/zyntra/internal/graph"
	"github.com/zyvorai/zyntra/internal/planner"
	"github.com/zyvorai/zyntra/internal/sim"
)

// Benefit IDs identify the same economic benefit across actions. A portfolio
// counts only the largest estimate for an ID, never the sum of duplicates.
type Benefit struct {
	ID       string  `json:"id"`
	Monthly  float64 `json:"monthly"`
	Evidence string  `json:"evidence"`
}
type Economics struct {
	Cost        float64   `json:"cost"`
	MonthlyCost float64   `json:"monthly_cost"`
	Benefits    []Benefit `json:"benefits"`
	Assumptions []string  `json:"assumptions"`
}
type ActionSpec struct {
	Resources map[string]float64 `json:"resources,omitempty"`
	Requires  []string           `json:"requires,omitempty"`
	Excludes  []string           `json:"excludes,omitempty"`
	Economics *Economics         `json:"economics,omitempty"`
}
type Config struct {
	Currency string                `json:"currency"`
	Actions  map[string]ActionSpec `json:"actions"`
}
type Options struct {
	MaxActions    int                `json:"max_actions"`
	MaxCandidates int                `json:"max_candidates"`
	Budget        *float64           `json:"budget,omitempty"`
	Capacity      map[string]float64 `json:"capacity,omitempty"`
	At            time.Time          `json:"at"`
	// Populated by the server, never trusted from the caller.
	Unusable map[string]bool `json:"-"`
}
type Finance struct {
	Currency          string    `json:"currency"`
	Complete          bool      `json:"complete"`
	Missing           []string  `json:"missing,omitempty"`
	Cost              float64   `json:"cost"`
	MonthlyCost       float64   `json:"monthly_cost"`
	MonthlyBenefit    float64   `json:"monthly_benefit"`
	MonthlyNet        float64   `json:"monthly_net"`
	Net12Months       float64   `json:"net_12_months"`
	PaybackMonths     *float64  `json:"payback_months,omitempty"`
	CostOfDelay30Days float64   `json:"cost_of_delay_30_days"`
	Benefits          []Benefit `json:"benefits"`
	Assumptions       []string  `json:"assumptions"`
}
type Candidate struct {
	Actions    []string           `json:"actions"`
	Status     string             `json:"status"`
	Reasons    []string           `json:"reasons"`
	Score      float64            `json:"score"`
	Resources  map[string]float64 `json:"resources"`
	Finance    Finance            `json:"finance"`
	Simulation sim.Result         `json:"simulation"`
}
type Result struct {
	Candidates   []Candidate `json:"candidates"`
	Evaluated    int         `json:"evaluated"`
	Truncated    bool        `json:"truncated"`
	ModelVersion string      `json:"model_version"`
	At           time.Time   `json:"at"`
	Note         string      `json:"note"`
}

func finite(v float64) bool      { return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= 1e12 }
func nonnegative(v float64) bool { return finite(v) && v >= 0 }

func (c Config) Validate(m *graph.Model) error {
	if c.Currency != "" && (len(c.Currency) != 3 || c.Currency[0] < 'A' || c.Currency[0] > 'Z' || c.Currency[1] < 'A' || c.Currency[1] > 'Z' || c.Currency[2] < 'A' || c.Currency[2] > 'Z') {
		return fmt.Errorf("currency must be a three-letter uppercase code")
	}
	for id, s := range c.Actions {
		if _, ok := m.Action(id); !ok {
			return fmt.Errorf("unknown action %q in planning configuration", id)
		}
		for k, v := range s.Resources {
			if k == "" || !nonnegative(v) {
				return fmt.Errorf("invalid resource for %s", id)
			}
		}
		for _, ref := range append(append([]string{}, s.Requires...), s.Excludes...) {
			if _, ok := m.Action(ref); !ok || ref == id {
				return fmt.Errorf("invalid dependency/exclusion %s -> %s", id, ref)
			}
		}
		if e := s.Economics; e != nil {
			if c.Currency == "" || !nonnegative(e.Cost) || !nonnegative(e.MonthlyCost) {
				return fmt.Errorf("%s: currency and nonnegative finite costs required", id)
			}
			seen := map[string]bool{}
			for _, b := range e.Benefits {
				if b.ID == "" || seen[b.ID] || b.Evidence == "" || !nonnegative(b.Monthly) {
					return fmt.Errorf("%s: unique benefit IDs, evidence and nonnegative amounts required", id)
				}
				seen[b.ID] = true
			}
		}
	}
	// Prerequisites are a DAG; cyclic plans cannot be scheduled.
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("cyclic dependency at %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, dep := range c.Actions[id].Requires {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range c.Actions {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
func Estimate(ids []string, c Config) Finance {
	f := Finance{Currency: c.Currency, Complete: true, Benefits: []Benefit{}, Assumptions: []string{}}
	benefits := map[string]Benefit{}
	for _, id := range ids {
		e := c.Actions[id].Economics
		if e == nil {
			f.Complete = false
			f.Missing = append(f.Missing, id)
			continue
		}
		f.Cost += e.Cost
		f.MonthlyCost += e.MonthlyCost
		f.Assumptions = append(f.Assumptions, e.Assumptions...)
		for _, b := range e.Benefits {
			if old, ok := benefits[b.ID]; !ok || b.Monthly > old.Monthly {
				benefits[b.ID] = b
			}
		}
	}
	for _, b := range benefits {
		f.Benefits = append(f.Benefits, b)
		f.MonthlyBenefit += b.Monthly
	}
	sort.Slice(f.Benefits, func(i, j int) bool { return f.Benefits[i].ID < f.Benefits[j].ID })
	f.MonthlyNet = f.MonthlyBenefit - f.MonthlyCost
	f.Net12Months = 12*f.MonthlyNet - f.Cost
	f.CostOfDelay30Days = math.Max(0, f.MonthlyNet)
	if f.Complete && f.MonthlyNet > 0 {
		v := f.Cost / f.MonthlyNet
		f.PaybackMonths = &v
	}
	return f
}
func Evaluate(m *graph.Model, ids []string, c Config, o Options) (Candidate, error) {
	out := Candidate{Actions: ids, Status: "pending-approval", Reasons: []string{}, Resources: map[string]float64{}}
	selected := map[string]bool{}
	acts := []graph.Action{}
	risk := 0.0
	for _, id := range ids {
		a, ok := m.Action(id)
		if !ok || selected[id] {
			return out, fmt.Errorf("unknown or duplicate action %q", id)
		}
		selected[id] = true
		acts = append(acts, *a)
	}
	out.Finance = Estimate(ids, c)
	for _, a := range acts {
		s := c.Actions[a.ID]
		for _, id := range s.Requires {
			if !selected[id] {
				out.Reasons = append(out.Reasons, a.ID+" requires "+id)
			}
		}
		for _, id := range s.Excludes {
			if selected[id] {
				out.Reasons = append(out.Reasons, a.ID+" excludes "+id)
			}
		}
		for k, v := range s.Resources {
			out.Resources[k] += v
		}
		if a.Window != "" {
			w, ok := m.Calendars[a.Window]
			if !ok || !w.Contains(o.At) {
				out.Reasons = append(out.Reasons, a.ID+": outside change window")
			}
		}
		switch a.Risk {
		case graph.RiskMedium:
			risk += 0.05
		case graph.RiskHigh:
			risk += 0.15
		}
	}
	keys := []string{}
	for k := range o.Capacity {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if out.Resources[k] > o.Capacity[k] {
			out.Reasons = append(out.Reasons, fmt.Sprintf("%s: %.4g exceeds capacity %.4g", k, out.Resources[k], o.Capacity[k]))
		}
	}
	if o.Budget != nil {
		if !out.Finance.Complete {
			out.Reasons = append(out.Reasons, "budget cannot be checked: missing economics")
		} else if out.Finance.Cost > *o.Budget {
			out.Reasons = append(out.Reasons, "implementation cost exceeds budget")
		}
	}
	r, err := sim.ApplyPlan(m, acts, sim.Options{Unusable: o.Unusable})
	if err != nil {
		return out, err
	}
	out.Simulation = r
	out.Score = r.WeightedBefore - r.WeightedAfterWorst - risk - planner.StalePenalty*float64(len(r.StaleInputs))
	for _, v := range r.Violations {
		out.Reasons = append(out.Reasons, v.Text)
	}
	out.Reasons = append(out.Reasons, r.PreconditionFailures...)
	for _, id := range r.StaleInputs {
		out.Reasons = append(out.Reasons, "unusable input: "+id)
	}
	if len(out.Reasons) > 0 {
		out.Status = "blocked"
	}
	return out, nil
}
func Plan(ctx context.Context, m *graph.Model, c Config, o Options) (Result, error) {
	out := Result{Candidates: []Candidate{}, ModelVersion: m.Version(), At: o.At, Note: "Bounded exhaustive search; financial estimates are declared assumptions, not measured savings. Approval is still required."}
	if err := c.Validate(m); err != nil {
		return out, err
	}
	if o.MaxActions == 0 {
		o.MaxActions = 3
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 2000
	}
	if o.At.IsZero() {
		o.At = time.Now()
		out.At = o.At
	}
	if o.MaxActions < 1 || o.MaxActions > 6 || o.MaxCandidates < 1 || o.MaxCandidates > 10000 {
		return out, fmt.Errorf("max_actions must be 1..6 and max_candidates 1..10000")
	}
	if len(m.Actions) > 200 {
		return out, fmt.Errorf("planning supports at most 200 actions per model")
	}
	if o.Budget != nil && !nonnegative(*o.Budget) {
		return out, fmt.Errorf("invalid budget")
	}
	knownResources := map[string]bool{}
	for _, spec := range c.Actions {
		for name := range spec.Resources {
			knownResources[name] = true
		}
	}
	for k, v := range o.Capacity {
		if k == "" || !nonnegative(v) || !knownResources[k] {
			return out, fmt.Errorf("invalid or undeclared capacity %q", k)
		}
	}
	// Bound retained simulation cells as well as the number of combinations.
	// A large graph must not multiply a small request into an unbounded response.
	if len(m.KPIs) > 0 {
		o.MaxCandidates = min(o.MaxCandidates, max(1, 250000/len(m.KPIs)))
	}
	acts := append([]graph.Action(nil), m.Actions...)
	sort.Slice(acts, func(i, j int) bool { return acts[i].ID < acts[j].ID })
	var walk func(int, int, []string) error
	walk = func(start, left int, ids []string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if left == 0 {
			if out.Evaluated >= o.MaxCandidates {
				out.Truncated = true
				return nil
			}
			r, err := Evaluate(m, append([]string{}, ids...), c, o)
			if err != nil {
				return err
			}
			out.Candidates = append(out.Candidates, r)
			out.Evaluated++
			return nil
		}
		for i := start; i <= len(acts)-left; i++ {
			if err := walk(i+1, left-1, append(ids, acts[i].ID)); err != nil {
				return err
			}
			if out.Truncated {
				return nil
			}
		}
		return nil
	}
	for n := 1; n <= o.MaxActions && n <= len(acts); n++ {
		if err := walk(0, n, nil); err != nil {
			return out, err
		}
		if out.Truncated {
			break
		}
	}
	sort.SliceStable(out.Candidates, func(i, j int) bool {
		a, b := out.Candidates[i], out.Candidates[j]
		if (a.Status == "blocked") != (b.Status == "blocked") {
			return a.Status != "blocked"
		}
		return a.Score > b.Score
	})
	return out, nil
}

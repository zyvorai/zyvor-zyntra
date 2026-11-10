package planning

import (
	"context"
	"encoding/json"
	"github.com/zyvorai/zyntra/internal/graph"
	"math"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) *graph.Model {
	t.Helper()
	m, err := graph.Parse([]byte(`
name: portfolio
kpis:
 - {id: a, value: 100, target: 50, direction: lower}
 - {id: b, value: 100, target: 50, direction: lower}
 - {id: c, value: 100, target: 50, direction: lower}
edges:
 - {from: a, to: b, weight: 0.1}
actions:
 - {id: x, effects: [{kpi: a, change: -0.2}]}
 - {id: y, effects: [{kpi: b, change: -0.2}]}
 - {id: z, effects: [{kpi: c, change: -0.2}]}
`))
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func config() Config {
	return Config{Currency: "INR", Actions: map[string]ActionSpec{
		"x": {Resources: map[string]float64{"staff": 1}, Economics: &Economics{Cost: 100, MonthlyCost: 10, Benefits: []Benefit{{"shared", 100, "fixture"}}}},
		"y": {Resources: map[string]float64{"staff": 1}, Economics: &Economics{Cost: 100, Benefits: []Benefit{{"shared", 80, "fixture"}}}},
		"z": {Economics: &Economics{Cost: 100, Benefits: []Benefit{{"other", 50, "fixture"}}}},
	}}
}
func TestPortfoliosAndFinancialDedup(t *testing.T) {
	m := fixture(t)
	c := config()
	r, err := Plan(context.Background(), m, c, Options{MaxActions: 3})
	if err != nil {
		t.Fatal(err)
	}
	if r.Evaluated != 7 || len(r.Candidates[0].Actions) != 3 {
		t.Fatalf("unexpected search %+v", r)
	}
	f := r.Candidates[0].Finance
	if f.Cost != 300 || f.MonthlyBenefit != 150 || f.MonthlyNet != 140 || f.PaybackMonths == nil {
		t.Fatalf("double counted: %+v", f)
	}
	if m.KPIs[0].Value != 100 {
		t.Fatal("mutated model")
	}
}
func TestConstraintsAndMissingEconomics(t *testing.T) {
	m := fixture(t)
	c := config()
	spec := c.Actions["y"]
	spec.Requires = []string{"x"}
	c.Actions["y"] = spec
	spec = c.Actions["z"]
	spec.Excludes = []string{"x"}
	c.Actions["z"] = spec
	budget := 150.0
	for _, tc := range []struct {
		ids    []string
		opt    Options
		reason string
	}{
		{[]string{"y"}, Options{}, "requires"}, {[]string{"x", "z"}, Options{}, "excludes"}, {[]string{"x", "y"}, Options{Budget: &budget}, "budget"}, {[]string{"x", "y"}, Options{Capacity: map[string]float64{"staff": 1}}, "capacity"}, {[]string{"x"}, Options{Unusable: map[string]bool{"a": true}}, "unusable"},
	} {
		r, err := Evaluate(m, tc.ids, c, tc.opt)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "blocked" || !strings.Contains(strings.Join(r.Reasons, " "), tc.reason) {
			t.Fatalf("missing reason %s: %+v", tc.reason, r)
		}
	}
	delete(c.Actions, "x")
	r, err := Evaluate(m, []string{"x"}, c, Options{Budget: &budget})
	if err != nil || r.Status != "blocked" || r.Finance.Complete {
		t.Fatalf("missing cost treated as free: %+v %v", r, err)
	}
}
func TestLimitsValidationAndCancellation(t *testing.T) {
	m := fixture(t)
	r, err := Plan(context.Background(), m, config(), Options{MaxActions: 3, MaxCandidates: 2})
	if err != nil || !r.Truncated || r.Evaluated != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Plan(ctx, m, config(), Options{}); err == nil {
		t.Fatal("ignored cancellation")
	}
	n := math.NaN()
	for _, o := range []Options{{MaxActions: 7}, {MaxCandidates: -1}, {Budget: &n}, {Capacity: map[string]float64{"staff": -1}}} {
		if _, err = Plan(context.Background(), m, config(), o); err == nil {
			t.Fatalf("accepted %+v", o)
		}
	}
	c := config()
	x := c.Actions["x"]
	x.Requires = []string{"y"}
	c.Actions["x"] = x
	y := c.Actions["y"]
	y.Requires = []string{"x"}
	c.Actions["y"] = y
	if c.Validate(m) == nil {
		t.Fatal("accepted cyclic prerequisites")
	}
}
func TestSnapshotRoundTripAndReview(t *testing.T) {
	m := fixture(t)
	at := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	s, err := Capture(m, config(), nil, at)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(s)
	var imported Snapshot
	if err = json.Unmarshal(b, &imported); err != nil {
		t.Fatal(err)
	}
	m.KPIs[0].Value = 500
	r, err := Replay(context.Background(), imported, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.At.Equal(at) || r.Candidates[0].Simulation.KPIs[0].Before != 100 {
		t.Fatal("replay used live data")
	}
	review, err := Compare(context.Background(), ReviewRequest{Snapshot: imported, Changes: []EdgeChange{{"a", "b", 0.5}}, Actions: []string{"x"}, Actual: map[string]float64{"b": 90}})
	if err != nil {
		t.Fatal(err)
	}
	if review.BaseVersion == review.CandidateVersion || len(review.Errors) != 1 || review.Errors[0].AfterError >= review.Errors[0].BeforeError {
		t.Fatalf("review failed %+v", review)
	}
	if imported.Model.Edges[0].Weight != 0.1 {
		t.Fatal("review mutated original")
	}
	imported.Model.KPIs[0].Value = 101
	if _, err = Replay(context.Background(), imported, Options{}); err == nil {
		t.Fatal("accepted edited snapshot")
	}
}
func TestProcessAndBenefitEvidence(t *testing.T) {
	at := time.Now()
	events := []Event{{"3", "case", "A", at.Add(3 * time.Hour)}, {"1", "case", "A", at}, {"2", "case", "B", at.Add(time.Hour)}}
	p, err := Mine(events)
	if err != nil || p.ReworkCases != 1 || p.Transitions[0].MeanSeconds != 7200 {
		t.Fatalf("%+v %v", p, err)
	}
	events[1].ID = "3"
	if _, err = Mine(events); err == nil {
		t.Fatal("accepted duplicate")
	}
	r, err := Measure(Experiment{Metric: "wait", Unit: "min", TreatmentBefore: []float64{10, 12}, TreatmentAfter: []float64{6, 8}, ControlBefore: []float64{10, 12}, ControlAfter: []float64{8, 10}})
	if err != nil || r.ObservedChange != -4 || r.AdjustedChange == nil || *r.AdjustedChange != -2 || r.Improvement != 2 {
		t.Fatalf("%+v %v", r, err)
	}
}

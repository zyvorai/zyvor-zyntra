// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package graph

import (
	"math"
	"strings"
	"testing"
)

func TestDeriveEval(t *testing.T) {
	for _, c := range []struct {
		name string
		d    Derive
		in   []float64
		want float64
	}{
		{"product", Derive{Op: OpProduct, Of: []string{"a", "b", "c"}}, []float64{0.7, 0.8, 0.9}, 0.504},
		{"sum", Derive{Op: OpSum, Of: []string{"a", "b"}}, []float64{2, 3}, 5},
		{"weighted", Derive{Op: OpWeightedSum, Of: []string{"a", "b"}, Weights: []float64{12, -2}}, []float64{2, 3}, 18},
		{"ratio", Derive{Op: OpRatio, Of: []string{"a", "b"}}, []float64{9, 4}, 2.25},
		{"min", Derive{Op: OpMin, Of: []string{"a", "b"}}, []float64{9, 4}, 4},
		{"max", Derive{Op: OpMax, Of: []string{"a", "b"}}, []float64{9, 4}, 9},
		{"avg", Derive{Op: OpAvg, Of: []string{"a", "b"}}, []float64{9, 4}, 6.5},
	} {
		got, err := c.d.Eval(c.in)
		if err != nil || math.Abs(got-c.want) > 1e-12 {
			t.Errorf("%s = %v, %v; want %v", c.name, got, err, c.want)
		}
	}
	if _, err := (Derive{Op: OpRatio, Of: []string{"a", "b"}}).Eval([]float64{1, 0}); err == nil {
		t.Error("a zero denominator must be an error, not Inf")
	}
	if _, err := (Derive{Op: OpProduct, Of: []string{"a"}}).Eval([]float64{math.NaN()}); err == nil {
		t.Error("a NaN input must be an error")
	}
	if _, err := (Derive{Op: OpSum, Of: []string{"a", "b"}}).Eval([]float64{1}); err == nil {
		t.Error("a wrong number of values must be an error")
	}
}

func TestDeriveEvalIntervalIsExact(t *testing.T) {
	// A weighted sum with a negative weight: the largest value uses the low
	// end of the second input.
	d := Derive{Op: OpWeightedSum, Of: []string{"a", "b"}, Weights: []float64{2, -3}}
	lo, hi, ok := d.EvalInterval([]float64{1, 1}, []float64{2, 4})
	if !ok || lo != 2*1-3*4 || hi != 2*2-3*1 {
		t.Errorf("weighted = [%v, %v] ok=%v, want [-10, 1]", lo, hi, ok)
	}
	p := Derive{Op: OpProduct, Of: []string{"a", "b"}}
	lo, hi, ok = p.EvalInterval([]float64{-2, 3}, []float64{4, 5})
	if !ok || lo != -10 || hi != 20 {
		t.Errorf("product = [%v, %v] ok=%v, want [-10, 20]", lo, hi, ok)
	}
	r := Derive{Op: OpRatio, Of: []string{"a", "b"}}
	if lo, hi, ok = r.EvalInterval([]float64{1, 2}, []float64{3, 4}); !ok || lo != 0.25 || hi != 1.5 {
		t.Errorf("ratio = [%v, %v] ok=%v, want [0.25, 1.5]", lo, hi, ok)
	}
	if _, _, ok = r.EvalInterval([]float64{1, -1}, []float64{3, 1}); ok {
		t.Error("a denominator range that includes zero cannot be bounded")
	}
	if _, _, ok = p.EvalInterval([]float64{1}, []float64{2}); ok {
		t.Error("a wrong number of bounds cannot be evaluated")
	}
	// A point interval is the plain value.
	if lo, hi, ok = p.EvalInterval([]float64{2, 3}, []float64{2, 3}); !ok || lo != 6 || hi != 6 {
		t.Errorf("point = [%v, %v]", lo, hi)
	}
}

func TestDeriveValidation(t *testing.T) {
	base := `kpis: [{id: a, value: 1}, {id: b, value: 2}, `
	for name, c := range map[string]struct {
		yaml string
		want string
	}{
		"derive plus source":     {base + `{id: d, value: 0, derive: {op: sum, of: [a, b]}, source: {kind: manual}}]`, "cannot also have a source"},
		"unknown input":          {base + `{id: d, value: 0, derive: {op: sum, of: [a, z]}}]`, "unknown kpi"},
		"self reference":         {base + `{id: d, value: 0, derive: {op: sum, of: [d]}}]`, "refers to itself"},
		"no op":                  {base + `{id: d, value: 0, derive: {of: [a]}}]`, "needs an op"},
		"bad op":                 {base + `{id: d, value: 0, derive: {op: power, of: [a]}}]`, "must be product"},
		"no inputs":              {base + `{id: d, value: 0, derive: {op: sum, of: []}}]`, "at least one input"},
		"ratio with three":       {base + `{id: d, value: 0, derive: {op: ratio, of: [a, b, a]}}]`, "exactly two"},
		"weights on product":     {base + `{id: d, value: 0, derive: {op: product, of: [a, b], weights: [1, 2]}}]`, "takes no weights"},
		"weight count":           {base + `{id: d, value: 0, derive: {op: weighted_sum, of: [a, b], weights: [1]}}]`, "needs 2 weights"},
		"too many inputs":        {base + `{id: d, value: 0, derive: {op: sum, of: [a, a, a, a, a, a, a]}}]`, "at most 6"},
		"edge into derived":      {base + `{id: d, value: 0, derive: {op: sum, of: [a, b]}}]` + "\nedges: [{from: a, to: d, weight: 1}]", "derived from other kpis"},
		"effect on derived":      {base + `{id: d, value: 0, derive: {op: sum, of: [a, b]}}]` + "\nactions: [{id: x, effects: [{kpi: d, change: 1}]}]", "put the effect on one of its inputs"},
		"cycle through a derive": {`kpis: [{id: a, value: 1}, {id: d, value: 0, derive: {op: sum, of: [a]}}]` + "\nedges: [{from: d, to: a, weight: 1}]", "cycle"},
		"derive of derive cycle": {`kpis: [{id: d, value: 0, derive: {op: sum, of: [e]}}, {id: e, value: 0, derive: {op: sum, of: [d]}}]`, "cycle"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(c.yaml))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestDeriveOrdersInputsFirstAndClonesDeep(t *testing.T) {
	// d is listed first but depends on a and b.
	m, err := Parse([]byte(`kpis:
  - {id: d, value: 0, derive: {op: product, of: [a, b]}}
  - {id: a, value: 2}
  - {id: b, value: 3}
  - {id: c, value: 1}
edges: [{from: d, to: c, weight: 1}]`))
	if err != nil {
		t.Fatal(err)
	}
	order, err := m.TopoOrder()
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	if !(pos["a"] < pos["d"] && pos["b"] < pos["d"] && pos["d"] < pos["c"]) {
		t.Errorf("order = %v, want a and b before d before c", order)
	}
	c := m.Clone()
	c.KPIs[m.index["d"]].Derive.Of[0] = "changed"
	if m.KPIs[m.index["d"]].Derive.Of[0] != "a" {
		t.Error("Clone shares the derive inputs with the original")
	}
}

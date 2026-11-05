// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package graph

import "testing"

func TestRestrictRemovesEverythingThatTouchesAHiddenKPI(t *testing.T) {
	m, err := Parse([]byte(`
name: r
kpis:
  - {id: open, value: 1, target: 2, direction: higher, owner: o}
  - {id: hidden, value: 1, target: 2, direction: higher, owner: o}
  - {id: other, value: 1, target: 2, direction: higher, owner: o}
edges:
  - {from: open, to: hidden, weight: 0.1}
  - {from: open, to: other, weight: 0.1}
constraints:
  - {kpi: hidden, floor: 0}
  - {kpi: other, floor: 0}
actions:
  - {id: fine, effects: [{kpi: open, change: 0.1}]}
  - {id: by_effect, effects: [{kpi: hidden, change: 0.1}]}
  - {id: by_precondition, effects: [{kpi: open, change: 0.1}], preconditions: [{kpi: hidden, worse_than: 5}]}
  - {id: by_invariant, effects: [{kpi: open, change: 0.1}], invariants: [{kpi: hidden, max_worsen: 0.1}]}
  - {id: by_outcome, effects: [{kpi: open, change: 0.1}], outcome: {guardrails: [hidden]}}
  - {id: by_marking, effects: [{kpi: open, change: 0.1}]}
`))
	if err != nil {
		t.Fatal(err)
	}
	c := m.Clone()
	c.Restrict(
		func(k KPI) bool { return k.ID == "hidden" },
		func(a Action) bool { return a.ID == "by_marking" },
	)
	if _, ok := c.KPI("hidden"); ok {
		t.Error("hidden KPI survived")
	}
	if k, ok := c.KPI("other"); !ok || k.ID != "other" {
		t.Error("the index must be rebuilt: other KPI not found")
	}
	if len(c.Edges) != 1 || c.Edges[0].To != "other" {
		t.Errorf("edges = %+v", c.Edges)
	}
	if len(c.Constraints) != 1 || c.Constraints[0].KPI != "other" {
		t.Errorf("constraints = %+v", c.Constraints)
	}
	if len(c.Actions) != 1 || c.Actions[0].ID != "fine" {
		var ids []string
		for _, a := range c.Actions {
			ids = append(ids, a.ID)
		}
		t.Errorf("actions = %v, want only fine", ids)
	}
	// The original is untouched.
	if _, ok := m.KPI("hidden"); !ok || len(m.Actions) != 6 {
		t.Error("Restrict changed the source model")
	}
}

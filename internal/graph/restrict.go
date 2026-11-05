// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package graph

// Restrict removes what a viewer may not see from the model, in place. Call it
// on a clone (Clone, or a snapshot), never on the live model.
//
// KPIs for which hideKPI is true are removed with every edge and constraint
// that touches them. An action is removed when hideAction is true or when it
// refers to a removed KPI in an effect, precondition, invariant, success
// criterion or guardrail: a plan built from what is left can then never name,
// price or depend on something the viewer cannot see.
func (m *Model) Restrict(hideKPI func(KPI) bool, hideAction func(Action) bool) {
	gone := map[string]bool{}
	kept := m.KPIs[:0:0]
	for _, k := range m.KPIs {
		if hideKPI != nil && hideKPI(k) {
			gone[k.ID] = true
			continue
		}
		kept = append(kept, k)
	}
	m.KPIs = kept

	edges := m.Edges[:0:0]
	for _, e := range m.Edges {
		if !gone[e.From] && !gone[e.To] {
			edges = append(edges, e)
		}
	}
	m.Edges = edges

	cons := m.Constraints[:0:0]
	for _, c := range m.Constraints {
		if !gone[c.KPI] {
			cons = append(cons, c)
		}
	}
	m.Constraints = cons

	acts := m.Actions[:0:0]
	for _, a := range m.Actions {
		if (hideAction != nil && hideAction(a)) || touches(a, gone) {
			continue
		}
		acts = append(acts, a)
	}
	m.Actions = acts

	m.index = make(map[string]int, len(m.KPIs))
	for i, k := range m.KPIs {
		m.index[k.ID] = i
	}
}

func touches(a Action, gone map[string]bool) bool {
	if len(gone) == 0 {
		return false
	}
	for _, e := range a.Effects {
		if gone[e.KPI] {
			return true
		}
	}
	for _, p := range a.Preconditions {
		if gone[p.KPI] {
			return true
		}
	}
	for _, i := range a.Invariants {
		if gone[i.KPI] {
			return true
		}
	}
	if o := a.Outcome; o != nil {
		for _, c := range o.Success {
			if gone[c.KPI] {
				return true
			}
		}
		for _, g := range o.Guardrails {
			if gone[g] {
				return true
			}
		}
	}
	return false
}

// ActionTouches reports whether the action refers to any KPI in ids.

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package policyguard evaluates simple policy rules against a decision context.
package policyguard

// Rule is a declarative constraint.
type Rule struct {
	ID        string  `json:"id"`
	KPI       string  `json:"kpi"`
	MaxWorsen float64 `json:"max_worsen,omitempty"` // relative fraction
	MinValue  float64 `json:"min_value,omitempty"`
	Blocked   bool    `json:"blocked"`
}

// Context holds current KPI values.
type Context map[string]float64

// Check returns the first rule that fails, or nil.
func Check(rules []Rule, before, after Context) *Rule {
	for i := range rules {
		r := &rules[i]
		if r.Blocked {
			return r
		}
		b, a := before[r.KPI], after[r.KPI]
		if r.MaxWorsen > 0 && b != 0 {
			worsen := (a - b) / b
			if worsen > r.MaxWorsen {
				return r
			}
		}
		if r.MinValue != 0 && a < r.MinValue {
			return r
		}
	}
	return nil
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package outcomes records predicted vs actual for learning.
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package outcomes

// Record is one decision outcome.
type Record struct {
	DecisionID string             `json:"decision_id"`
	Predicted  map[string]float64 `json:"predicted"`
	Actual     map[string]float64 `json:"actual"`
	Verdict    string             `json:"verdict"` // verified|regressed|missed|inconclusive
}

// AbsError returns mean absolute error across shared KPIs.
func AbsError(r Record) float64 {
	var sum, n float64
	for k, pred := range r.Predicted {
		if act, ok := r.Actual[k]; ok {
			diff := act - pred
			if diff < 0 {
				diff = -diff
			}
			sum += diff
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

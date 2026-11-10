// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package constraints checks hard limits before ranking.
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package constraints

// Limit is a hard bound.
type Limit struct {
	KPI string  `json:"kpi"`
	Min float64 `json:"min,omitempty"`
	Max float64 `json:"max,omitempty"`
}

// Violates returns true if any limit is broken.
func Violates(limits []Limit, values map[string]float64) bool {
	for _, l := range limits {
		v, ok := values[l.KPI]
		if !ok {
			continue
		}
		if l.Min != 0 && v < l.Min {
			return true
		}
		if l.Max != 0 && v > l.Max {
			return true
		}
	}
	return false
}

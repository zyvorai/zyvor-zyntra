// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package scenarios stores and compares what-if results.
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package scenarios

// Result is one simulated outcome set.
type Result struct {
	ID     string             `json:"id"`
	Label  string             `json:"label"`
	Scores map[string]float64 `json:"scores"`
}

// Compare ranks results by a named KPI (higher better) and returns ordered IDs.
func Compare(results []Result, kpi string) []string {
	type pair struct {
		id    string
		score float64
	}
	var ps []pair
	for _, r := range results {
		ps = append(ps, pair{r.ID, r.Scores[kpi]})
	}
	// simple insertion sort for small sets
	for i := 1; i < len(ps); i++ {
		j := i
		for j > 0 && ps[j].score > ps[j-1].score {
			ps[j], ps[j-1] = ps[j-1], ps[j]
			j--
		}
	}
	ids := make([]string, len(ps))
	for i, p := range ps {
		ids[i] = p.id
	}
	return ids
}

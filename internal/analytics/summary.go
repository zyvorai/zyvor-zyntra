// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package analytics provides lightweight outcome and gap summaries.
// It is a helper library; KPI query routes live under internal/api.
package analytics

// Summary is a compact decision-quality snapshot.
type Summary struct {
	TotalDecisions int     `json:"total_decisions"`
	HitRate        float64 `json:"hit_rate"`
	AvgAbsError    float64 `json:"avg_abs_error"`
	OpenGaps       int     `json:"open_gaps"`
	WorstGapID     string  `json:"worst_gap_id,omitempty"`
}

// HitRate computes verified / total from simple counters.
func HitRate(verified, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(verified) / float64(total)
}

// Build assembles a summary from counters.
func Build(total, verified, openGaps int, avgAbsError float64, worstGapID string) Summary {
	return Summary{
		TotalDecisions: total,
		HitRate:        HitRate(verified, total),
		AvgAbsError:    avgAbsError,
		OpenGaps:       openGaps,
		WorstGapID:     worstGapID,
	}
}

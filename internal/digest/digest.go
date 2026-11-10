// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package digest builds a short executive summary.
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package digest

import "fmt"

// Summary is a one-paragraph brief.
func Summary(gaps int, topAction, topKPI string, hitRate float64) string {
	return fmt.Sprintf("%d KPI(s) off target. Top recommended action: %s (closes %s). Historical hit rate on similar decisions: %.0f%%.",
		gaps, topAction, topKPI, hitRate*100)
}

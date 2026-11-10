// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package forecast provides a linear time-to-breach estimate.
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package forecast

// TimeToBreach returns estimated steps until a KPI crosses target (0 if already breached or flat).
func TimeToBreach(current, target, slope float64) int {
	if slope == 0 || (slope > 0 && current >= target) || (slope < 0 && current <= target) {
		return 0
	}
	diff := target - current
	steps := diff / slope
	if steps < 0 {
		return 0
	}
	return int(steps + 0.5)
}

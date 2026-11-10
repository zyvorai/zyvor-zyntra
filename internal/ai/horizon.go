// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package ai

import (
	"math"
	"sort"
)

// HorizonBacktest uses disjoint calibration and evaluation periods. Every
// calibration target precedes the first evaluation origin, including for long
// horizons. Coverage is descriptive: overlapping origins are not independent.
type HorizonBacktest struct {
	Hours              int     `json:"hours"`
	Status             string  `json:"status"`
	Reason             string  `json:"reason,omitempty"`
	Method             string  `json:"method,omitempty"`
	CalibrationSamples int     `json:"calibration_samples"`
	Samples            int     `json:"samples"`
	MAE                float64 `json:"mae"`
	RMSE               float64 `json:"rmse"`
	BaselineMAE        float64 `json:"baseline_mae"`
	ErrorRadius        float64 `json:"error_radius"`
	Coverage           float64 `json:"coverage"`
}

func horizonBacktest(v []float64, hours int) HorizonBacktest {
	b := HorizonBacktest{Hours: hours, Status: "warming", Reason: "Need two disjoint 24-origin windows and at least 48 training hours for this horizon."}
	if hours < 1 {
		return b
	}
	// Training at an origin excludes that observation. The target is h-1
	// observations after the origin, matching predict(..., steps=h).
	evalStart := len(v) - hours - 23
	calibrationStart := evalStart - hours - 23
	if calibrationStart < 48 {
		return b
	}
	methods := []string{"last-value", "linear", "daily-seasonal"}
	if calibrationStart >= 336 {
		methods = append(methods, "weekly-seasonal")
	}
	best := math.Inf(1)
	for _, method := range methods {
		errors := make([]float64, 24)
		mae := 0.0
		for i := range errors {
			origin := calibrationStart + i
			errors[i] = math.Abs(predict(v[:origin], method, hours) - v[origin+hours-1])
			mae += errors[i]
		}
		mae /= 24
		if mae < best-1e-9 {
			best = mae
			b.Method = method
			sort.Float64s(errors)
			b.ErrorRadius = errors[21]
		}
	}
	b.Status = "ready"
	b.Reason = ""
	b.CalibrationSamples = 24
	b.Samples = 24
	for origin := evalStart; origin < evalStart+24; origin++ {
		actual := v[origin+hours-1]
		err := math.Abs(predict(v[:origin], b.Method, hours) - actual)
		b.MAE += err
		b.RMSE += err * err
		b.BaselineMAE += math.Abs(v[origin-1] - actual)
		if err <= b.ErrorRadius+1e-9 {
			b.Coverage++
		}
	}
	b.MAE /= 24
	b.RMSE = math.Sqrt(b.RMSE / 24)
	b.BaselineMAE /= 24
	b.Coverage /= 24
	return b
}

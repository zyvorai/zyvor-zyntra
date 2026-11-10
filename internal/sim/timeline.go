// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package sim

import (
	"context"
	"fmt"
	"github.com/zyvorai/zyntra/internal/graph"
	"time"
)

type TimelinePoint struct {
	Elapsed graph.Duration `json:"elapsed"`
	Result  Result         `json:"result"`
}
type TimelineResult struct {
	Points []TimelinePoint `json:"points"`
	Note   string          `json:"note"`
}

// Timeline models one-shot persistent effects and delayed propagation on a
// fixed grid. Values are always relative to the original baseline; an effect
// is never compounded each tick. Delays round up to the next sample.
func Timeline(ctx context.Context, m *graph.Model, actions []graph.Action, horizon, step time.Duration, unusable map[string]bool) (TimelineResult, error) {
	out := TimelineResult{Points: []TimelinePoint{}, Note: "One-shot persistent effects on a fixed baseline. Delays round up to the sample grid. No exogenous demand, recovery curve or resource scheduling is inferred."}
	if step <= 0 || horizon < 0 || horizon > 365*24*time.Hour || horizon/step > 240 || horizon%step != 0 {
		return out, fmt.Errorf("horizon must be 0..365 days, divisible by positive step, with at most 240 intervals")
	}
	history := []Result{}
	for elapsed := time.Duration(0); elapsed <= horizon; elapsed += step {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		r, err := ApplyPlan(m, actions, Options{Unusable: unusable, elapsed: &elapsed, history: history, step: step})
		if err != nil {
			return out, err
		}
		history = append(history, r)
		out.Points = append(out.Points, TimelinePoint{graph.Duration(elapsed), r})
	}
	return out, nil
}

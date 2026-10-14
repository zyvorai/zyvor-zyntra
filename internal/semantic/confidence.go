// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package semantic

import (
	"fmt"
	"math"
	"time"
)

var defaultWeights = map[string]float64{
	"freshness": 0.18, "data_quality": 0.18, "ontology_confidence": 0.14,
	"calibration": 0.18, "edge_confidence": 0.14, "prediction_accuracy": 0.18,
}

// EvaluateConfidence returns an explainable evidence score. It does not rank,
// approve or execute an action; Zyntra's deterministic planner and human
// approval lane remain authoritative.
//
// A weighted geometric mean is used so one weak evidence dimension cannot be
// hidden by several strong ones.
func EvaluateConfidence(f Factors, now time.Time) (Confidence, error) {
	values := []struct {
		name string
		v    float64
	}{
		{"freshness", f.Freshness},
		{"data_quality", f.DataQuality},
		{"ontology_confidence", f.OntologyConfidence},
		{"calibration", f.Calibration},
		{"edge_confidence", f.EdgeConfidence},
		{"prediction_accuracy", f.PredictionAccuracy},
	}
	sum := 0.0
	reasons := []string{}
	for _, x := range values {
		if x.v < 0 || x.v > 1 || math.IsNaN(x.v) || math.IsInf(x.v, 0) {
			return Confidence{}, fmt.Errorf("%s must be between 0 and 1", x.name)
		}
		// Clamp zero for logarithm while preserving a near-zero aggregate.
		v := math.Max(x.v, 1e-6)
		sum += defaultWeights[x.name] * math.Log(v)
		if x.v < 0.75 {
			reasons = append(reasons, fmt.Sprintf("%s is %.0f%%", x.name, x.v*100))
		}
	}
	score := math.Exp(sum)
	percent := int(math.Round(score * 100))
	band := "low"
	switch {
	case percent >= 85:
		band = "high"
	case percent >= 70:
		band = "medium"
	}
	weights := make(map[string]float64, len(defaultWeights))
	for k, v := range defaultWeights {
		weights[k] = v
	}
	return Confidence{
		Score: score, Percent: percent, Band: band, Factors: f,
		Weights: weights, Reasons: reasons, Calculated: now.UTC(),
	}, nil
}

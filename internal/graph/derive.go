// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package graph

import (
	"fmt"
	"math"
)

// Derive makes a KPI a function of other KPIs instead of a value read from a
// source. The value is computed after the inputs are, so it follows them on
// every refresh and inside every simulation.
//
//	derive: {op: product, of: [availability, performance, quality]}
//	derive: {op: weighted_sum, of: [scrap, downtime_h], weights: [12, 340]}
//	derive: {op: ratio, of: [filled, requested]}        # numerator, denominator
type Derive struct {
	Op      string    `yaml:"op" json:"op"`
	Of      []string  `yaml:"of" json:"of"`
	Weights []float64 `yaml:"weights,omitempty" json:"weights,omitempty"`
}

// Derive operations.
const (
	OpProduct     = "product"
	OpSum         = "sum"
	OpWeightedSum = "weighted_sum"
	OpRatio       = "ratio"
	OpMin         = "min"
	OpMax         = "max"
	OpAvg         = "avg"
)

// MaxDeriveInputs bounds a formula. Interval evaluation tries every corner of
// the inputs' ranges, which is 2^n evaluations.
const MaxDeriveInputs = 6

// Derived reports whether the KPI is computed from other KPIs.
func (k KPI) Derived() bool { return k.Derive != nil }

// check validates the shape of a formula. The references themselves are
// checked by Model.Validate, which knows the KPI ids.
func (d Derive) check() error {
	n := len(d.Of)
	if n == 0 {
		return fmt.Errorf("derive needs at least one input in of")
	}
	if n > MaxDeriveInputs {
		return fmt.Errorf("derive takes at most %d inputs, got %d", MaxDeriveInputs, n)
	}
	switch d.Op {
	case OpProduct, OpSum, OpMin, OpMax, OpAvg:
		if len(d.Weights) != 0 {
			return fmt.Errorf("derive op %s takes no weights (use weighted_sum)", d.Op)
		}
	case OpWeightedSum:
		if len(d.Weights) != n {
			return fmt.Errorf("derive weighted_sum needs %d weights, one per input, got %d", n, len(d.Weights))
		}
		for _, w := range d.Weights {
			if math.IsNaN(w) || math.IsInf(w, 0) {
				return fmt.Errorf("derive weights must be finite numbers")
			}
		}
	case OpRatio:
		if n != 2 {
			return fmt.Errorf("derive ratio needs exactly two inputs (numerator, denominator), got %d", n)
		}
		if len(d.Weights) != 0 {
			return fmt.Errorf("derive op ratio takes no weights")
		}
	case "":
		return fmt.Errorf("derive needs an op: product, sum, weighted_sum, ratio, min, max or avg")
	default:
		return fmt.Errorf("derive op must be product, sum, weighted_sum, ratio, min, max or avg, got %q", d.Op)
	}
	return nil
}

// Eval applies the formula to one value per input, in the order of Of. It
// reports an error rather than a made-up number when the result is not finite
// or a ratio's denominator is zero.
func (d Derive) Eval(v []float64) (float64, error) {
	if len(v) != len(d.Of) {
		return 0, fmt.Errorf("derive %s: got %d values for %d inputs", d.Op, len(v), len(d.Of))
	}
	var r float64
	switch d.Op {
	case OpProduct:
		r = 1
		for _, x := range v {
			r *= x
		}
	case OpSum:
		for _, x := range v {
			r += x
		}
	case OpWeightedSum:
		for i, x := range v {
			r += x * d.Weights[i]
		}
	case OpAvg:
		for _, x := range v {
			r += x
		}
		r /= float64(len(v))
	case OpMin:
		r = math.Inf(1)
		for _, x := range v {
			r = math.Min(r, x)
		}
	case OpMax:
		r = math.Inf(-1)
		for _, x := range v {
			r = math.Max(r, x)
		}
	case OpRatio:
		if v[1] == 0 {
			return 0, fmt.Errorf("derive ratio: denominator %s is zero", d.Of[1])
		}
		r = v[0] / v[1]
	default:
		return 0, fmt.Errorf("derive: unknown op %q", d.Op)
	}
	if math.IsNaN(r) || math.IsInf(r, 0) {
		return 0, fmt.Errorf("derive %s: result is not a finite number", d.Op)
	}
	return r, nil
}

// EvalInterval returns the smallest and largest value the formula takes while
// each input stays within [lo[i], hi[i]]. Every op is linear or monotone in
// each input, so the extremes are at the corners of the box, and trying them
// all is exact. ok is false when the range cannot be bounded: a ratio whose
// denominator range includes zero, or any non-finite corner.
func (d Derive) EvalInterval(lo, hi []float64) (min, max float64, ok bool) {
	n := len(d.Of)
	if len(lo) != n || len(hi) != n {
		return 0, 0, false
	}
	if d.Op == OpRatio && lo[1] <= 0 && hi[1] >= 0 {
		return 0, 0, false
	}
	min, max = math.Inf(1), math.Inf(-1)
	v := make([]float64, n)
	for mask := 0; mask < 1<<n; mask++ {
		for i := range v {
			if mask&(1<<i) != 0 {
				v[i] = hi[i]
			} else {
				v[i] = lo[i]
			}
		}
		r, err := d.Eval(v)
		if err != nil {
			return 0, 0, false
		}
		min, max = math.Min(min, r), math.Max(max, r)
	}
	return min, max, true
}

func cloneDerive(d *Derive) *Derive {
	if d == nil {
		return nil
	}
	c := *d
	c.Of = append([]string(nil), d.Of...)
	c.Weights = append([]float64(nil), d.Weights...)
	return &c
}

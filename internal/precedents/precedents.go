// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package precedents finds similar past decisions by KPI gap signature.
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package precedents

import "math"

// Signature is a vector of gap magnitudes.
type Signature map[string]float64

// Similarity is cosine similarity (0–1).
func Similarity(a, b Signature) float64 {
	var dot, na, nb float64
	for k, va := range a {
		if vb, ok := b[k]; ok {
			dot += va * vb
			na += va * va
			nb += vb * vb
		}
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

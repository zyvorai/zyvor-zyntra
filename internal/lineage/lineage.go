// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package lineage builds simple source → KPI → decision trails.
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package lineage

// Node is a provenance step.
type Node struct {
	Kind string `json:"kind"` // source|kpi|action|decision
	ID   string `json:"id"`
}

// Trail is an ordered path.
type Trail []Node

// Build returns a minimal trail.
func Build(source, kpi, action, decision string) Trail {
	var t Trail
	if source != "" {
		t = append(t, Node{"source", source})
	}
	if kpi != "" {
		t = append(t, Node{"kpi", kpi})
	}
	if action != "" {
		t = append(t, Node{"action", action})
	}
	if decision != "" {
		t = append(t, Node{"decision", decision})
	}
	return t
}

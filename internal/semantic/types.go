// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package semantic adds governed data contracts, lineage and decision confidence
// to Zyntra without changing the deterministic decision engine.
package semantic

import "time"

// Contract describes one governed source feeding Zyntra's ontology/KPI graph.
type Contract struct {
	Name       string            `json:"name"`
	Owner      string            `json:"owner,omitempty"`
	Source     Source            `json:"source"`
	ObjectType string            `json:"object_type"`
	Schema     []Field           `json:"schema"`
	Quality    Quality           `json:"quality,omitempty"`
	KPIs       []string          `json:"kpis,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// Source identifies a source without carrying credentials.
type Source struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
}

// Field is one field in the contract.
type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
}

// Quality is the minimum acceptable quality envelope for a source.
type Quality struct {
	Freshness    string   `json:"freshness,omitempty"`
	Completeness float64  `json:"completeness,omitempty"`
	Unique       []string `json:"unique,omitempty"`
}

// CatalogEntry is the permission-safe catalog representation.
type CatalogEntry struct {
	Name       string            `json:"name"`
	Owner      string            `json:"owner,omitempty"`
	SourceKind string            `json:"source_kind"`
	Reference  string            `json:"reference"`
	ObjectType string            `json:"object_type"`
	Fields     []Field           `json:"fields"`
	Quality    Quality           `json:"quality,omitempty"`
	KPIs       []string          `json:"kpis,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// Node is a semantic lineage node.
type Node struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

// Edge is a directional lineage edge.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// Lineage is a stable graph usable by the console, API and agents.
type Lineage struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Factors are evidence inputs for a decision-confidence calculation.
// Values are normalized to [0,1].
type Factors struct {
	Freshness          float64 `json:"freshness"`
	DataQuality        float64 `json:"data_quality"`
	OntologyConfidence float64 `json:"ontology_confidence"`
	Calibration        float64 `json:"calibration"`
	EdgeConfidence     float64 `json:"edge_confidence"`
	PredictionAccuracy float64 `json:"prediction_accuracy"`
}

// Confidence is an explainable score, never an approval or execution decision.
type Confidence struct {
	Score      float64            `json:"score"`
	Percent    int                `json:"percent"`
	Band       string             `json:"band"`
	Factors    Factors            `json:"factors"`
	Weights    map[string]float64 `json:"weights"`
	Reasons    []string           `json:"reasons,omitempty"`
	Calculated time.Time          `json:"calculated_at"`
}

// AgentResource is a stable semantic resource advertised to agents/MCP bridges.
type AgentResource struct {
	URI         string `json:"uri"`
	Description string `json:"description"`
}

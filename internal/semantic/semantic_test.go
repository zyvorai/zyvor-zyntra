// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package semantic

import (
	"testing"
	"time"
)

func sampleContract() Contract {
	return Contract{
		Name: "orders", Owner: "commerce",
		Source:     Source{Kind: "sql", Reference: "orders"},
		ObjectType: "Order",
		Schema: []Field{
			{Name: "order_id", Type: "string", Required: true},
			{Name: "amount", Type: "number"},
		},
		Quality: Quality{Freshness: "5m", Completeness: .999, Unique: []string{"order_id"}},
		KPIs:    []string{"fulfilment_latency", "revenue"},
	}
}

func TestValidateContract(t *testing.T) {
	if err := ValidateContract(sampleContract()); err != nil {
		t.Fatal(err)
	}
	c := sampleContract()
	c.Quality.Unique = []string{"missing"}
	if err := ValidateContract(c); err == nil {
		t.Fatal("expected unknown unique field to fail")
	}
}

func TestLineageDeterministic(t *testing.T) {
	g := BuildLineage([]Contract{sampleContract()})
	if len(g.Nodes) != 5 {
		t.Fatalf("nodes=%d want 5", len(g.Nodes))
	}
	if len(g.Edges) != 4 {
		t.Fatalf("edges=%d want 4", len(g.Edges))
	}
	if g.Edges[0].From > g.Edges[len(g.Edges)-1].From {
		t.Fatal("edges are not sorted")
	}
}

func TestConfidencePenalizesWeakEvidence(t *testing.T) {
	now := time.Unix(42, 0)
	strong, err := EvaluateConfidence(Factors{.98, .99, .95, .95, .92, .94}, now)
	if err != nil {
		t.Fatal(err)
	}
	weak, err := EvaluateConfidence(Factors{.40, .99, .95, .95, .92, .94}, now)
	if err != nil {
		t.Fatal(err)
	}
	if weak.Score >= strong.Score {
		t.Fatalf("weak=%f strong=%f", weak.Score, strong.Score)
	}
	if len(weak.Reasons) == 0 {
		t.Fatal("weak evidence should explain the penalty")
	}
}

func TestAgentResourcesStable(t *testing.T) {
	rs := AgentResources()
	if len(rs) != 8 || rs[0].URI != "zyntra://objects" || rs[7].URI != "zyntra://decisions" {
		t.Fatalf("unexpected resources: %#v", rs)
	}
}

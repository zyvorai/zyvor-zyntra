// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package semantic

import (
	"sort"
	"strings"
)

// BuildLineage maps source -> contract -> ontology object -> KPI. The graph is
// deliberately declarative: it records claimed lineage and does not pretend
// that correlation proves causality.
func BuildLineage(cs []Contract) Lineage {
	nodes := map[string]Node{}
	edges := map[string]Edge{}

	addNode := func(n Node) { nodes[n.ID] = n }
	addEdge := func(e Edge) { edges[e.From+"\x00"+e.To+"\x00"+e.Kind] = e }

	for _, c := range cs {
		sourceID := "source:" + slug(c.Source.Kind+":"+c.Source.Reference)
		contractID := "contract:" + slug(c.Name)
		objectID := "object-type:" + slug(c.ObjectType)
		addNode(Node{ID: sourceID, Kind: "source", Label: c.Source.Reference})
		addNode(Node{ID: contractID, Kind: "contract", Label: c.Name})
		addNode(Node{ID: objectID, Kind: "object_type", Label: c.ObjectType})
		addEdge(Edge{From: sourceID, To: contractID, Kind: "governed_by"})
		addEdge(Edge{From: contractID, To: objectID, Kind: "materializes"})

		for _, kpi := range c.KPIs {
			kpiID := "kpi:" + slug(kpi)
			addNode(Node{ID: kpiID, Kind: "kpi", Label: kpi})
			addEdge(Edge{From: objectID, To: kpiID, Kind: "measured_by"})
		}
	}

	out := Lineage{}
	for _, n := range nodes {
		out.Nodes = append(out.Nodes, n)
	}
	for _, e := range edges {
		out.Edges = append(out.Edges, e)
	}
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].ID < out.Nodes[j].ID })
	sort.Slice(out.Edges, func(i, j int) bool {
		if out.Edges[i].From != out.Edges[j].From {
			return out.Edges[i].From < out.Edges[j].From
		}
		if out.Edges[i].To != out.Edges[j].To {
			return out.Edges[i].To < out.Edges[j].To
		}
		return out.Edges[i].Kind < out.Edges[j].Kind
	})
	return out
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// AgentResources advertises stable semantic URIs. They are protocol-neutral
// and can be exposed directly by a future MCP transport.
func AgentResources() []AgentResource {
	return []AgentResource{
		{URI: "zyntra://objects", Description: "Permission-scoped business objects and types"},
		{URI: "zyntra://relationships", Description: "Declared ontology relationships"},
		{URI: "zyntra://kpis", Description: "KPI graph, targets and current values"},
		{URI: "zyntra://lineage", Description: "Source-to-contract-to-object-to-KPI lineage"},
		{URI: "zyntra://incidents", Description: "Sustained KPI watch incidents"},
		{URI: "zyntra://plans", Description: "Ranked deterministic action plans"},
		{URI: "zyntra://simulations", Description: "What-if results and assumptions"},
		{URI: "zyntra://decisions", Description: "Approved, signed and verified decision records"},
	}
}

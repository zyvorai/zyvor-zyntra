// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package ontology

import (
	"strings"
	"testing"
)

func sampleTables() []DBTable {
	return []DBTable{
		{Name: "orders", PK: []string{"id"}, FKs: map[string]string{"customer_id": "customers"},
			Columns: []DBColumn{{"id", "integer"}, {"customer_id", "integer"}, {"total", "numeric"}, {"placed_at", "timestamp with time zone"}, {"paid", "boolean"}}},
		{Name: "customers", PK: []string{"id"}, FKs: map[string]string{},
			Columns: []DBColumn{{"id", "integer"}, {"Full Name", "text"}}},
		{Name: "order_tags", PK: []string{"order_id", "tag_id"}, FKs: map[string]string{"order_id": "orders", "tag_id": "tags"},
			Columns: []DBColumn{{"order_id", "integer"}, {"tag_id", "integer"}}},
		{Name: "audit_log", Columns: []DBColumn{{"msg", "text"}}},
	}
}

func TestSuggestDraft(t *testing.T) {
	d, notes, err := Suggest(sampleTables(), SuggestOptions{Driver: "sqlite", DSNEnv: "ZYNTRA_X_DSN", Namespace: "shop"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Objects) != 2 || d.Objects[0].Name != "Customer" || d.Objects[1].Name != "Order" {
		t.Fatalf("objects: %+v", d.Objects)
	}
	if len(d.Links) != 1 || d.Links[0].From != "Order" || d.Links[0].To != "Customer" || d.Links[0].Name != "order_customer" {
		t.Fatalf("links: %+v", d.Links)
	}
	var order ConnectorSpec
	for _, c := range d.Connectors {
		if c.Mapping.Type == "Order" {
			order = c
		}
	}
	want := `SELECT "id", "customer_id", "total", "placed_at", "paid" FROM "orders"`
	if order.Query != want || !order.Prune || order.DSNEnv != "ZYNTRA_X_DSN" {
		t.Fatalf("connector: %+v", order)
	}
	types := map[string]string{}
	for _, p := range d.Objects[1].Properties {
		types[p.Name] = p.Type
	}
	if types["total"] != "number" || types["placed_at"] != "time" || types["paid"] != "bool" || len(types) != 3 {
		t.Fatalf("property types: %v", types)
	}
	if got := d.Connectors[0].Mapping.Props["full_name"]; got != "Full Name" {
		t.Fatalf("column name must be kept for the query: %q", got)
	}
	joined := strings.Join(notes, "\n")
	for _, w := range []string{"order_tags: join table", "audit_log: no primary key"} {
		if !strings.Contains(joined, w) {
			t.Errorf("notes missing %q:\n%s", w, joined)
		}
	}
}

func TestSuggestRejects(t *testing.T) {
	if _, _, err := Suggest(nil, SuggestOptions{Driver: "mysql", DSNEnv: "X"}); err == nil {
		t.Error("unknown driver accepted")
	}
	if _, _, err := Suggest(nil, SuggestOptions{Driver: "sqlite"}); err == nil {
		t.Error("missing dsn_env accepted")
	}
	if _, _, err := Suggest([]DBTable{{Name: "x"}}, SuggestOptions{Driver: "sqlite", DSNEnv: "X"}); err == nil {
		t.Error("a database with nothing mappable must be an error")
	}
}

func TestSuggestFKToUnmappedTable(t *testing.T) {
	tables := []DBTable{{Name: "a", PK: []string{"id"}, FKs: map[string]string{"b_id": "b"},
		Columns: []DBColumn{{"id", "int"}, {"b_id", "int"}}}}
	d, notes, err := Suggest(tables, SuggestOptions{Driver: "postgres", DSNEnv: "X"})
	if err != nil || len(d.Links) != 0 || len(d.Objects[0].Properties) != 1 || len(notes) != 1 {
		t.Fatalf("%v %+v %v", err, d, notes)
	}
}

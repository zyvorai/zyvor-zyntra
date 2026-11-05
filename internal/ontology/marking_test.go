// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package ontology

import (
	"strings"
	"testing"

	"github.com/zyvorai/zyntra/internal/marking"
)

func markedSchema() *Schema {
	return &Schema{Objects: []ObjectType{
		{Name: "Asset", Properties: []Property{
			{Name: "name", Type: "string"},
			{Name: "readiness_detail", Type: "string", Marking: "cui"},
			{Name: "location", Type: "string", Marking: "cui-specified"},
		}},
		{Name: "Plan", Marking: "cui", Properties: []Property{{Name: "name", Type: "string"}}},
	}}
}

func markedScheme(t *testing.T) *marking.Scheme {
	t.Helper()
	s := &marking.Scheme{Levels: []string{"unclassified", "cui", "cui-specified"}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMarkingsHideTypesAndPropertiesAboveClearance(t *testing.T) {
	st, err := Open("", markedSchema())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(obj("Asset", "a1", map[string]any{"name": "A1", "readiness_detail": "x", "location": "grid"})); err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(obj("Plan", "p1", map[string]any{"name": "P"})); err != nil {
		t.Fatal(err)
	}
	ac := &Access{Schema: st.Schema(), Scheme: markedScheme(t)}
	asset, plan := MakeID("Asset", "lab", "a1"), MakeID("Plan", "lab", "p1")

	cases := []struct {
		clearance string
		wantProps []string
		hidden    []string
		seesPlan  bool
	}{
		{"", []string{"name"}, []string{"readiness_detail", "location"}, false},
		{"unclassified", []string{"name"}, []string{"readiness_detail", "location"}, false},
		{"cui", []string{"name", "readiness_detail"}, []string{"location"}, true},
		{"cui-specified", []string{"name", "readiness_detail", "location"}, nil, true},
		{"unknown-level", nil, []string{"name"}, false}, // an unknown clearance sees nothing
	}
	for _, c := range cases {
		// Admins are not exempt: markings follow clearance, not role.
		r := st.As(ac, Principal{Subject: "u", Roles: []string{"admin"}, Clearance: c.clearance})
		o, ok := r.Get(asset)
		for _, p := range c.wantProps {
			if _, has := o.Props[p]; !has {
				t.Errorf("clearance %q lost property %s", c.clearance, p)
			}
		}
		for _, p := range c.hidden {
			if _, has := o.Props[p]; has {
				t.Errorf("clearance %q saw property %s", c.clearance, p)
			}
		}
		if c.clearance == "unknown-level" && ok && len(o.Props) > 0 {
			t.Errorf("an unknown clearance saw %v", o.Props)
		}
		if _, ok := r.Get(plan); ok != c.seesPlan {
			t.Errorf("clearance %q: sees Plan = %v, want %v", c.clearance, ok, c.seesPlan)
		}
		if got := len(r.List("Plan")); (got > 0) != c.seesPlan {
			t.Errorf("clearance %q: List(Plan) returned %d", c.clearance, got)
		}
	}
}

func TestNoSchemeMeansNoMarkingFilter(t *testing.T) {
	st, _ := Open("", markedSchema())
	_ = st.Upsert(obj("Asset", "a1", map[string]any{"name": "A1", "location": "grid"}))
	r := st.As(&Access{Schema: st.Schema()}, Principal{Subject: "u", Roles: []string{"viewer"}})
	o, ok := r.Get(MakeID("Asset", "lab", "a1"))
	if !ok || o.Props["location"].V != "grid" {
		t.Fatal("with markings off nothing may be hidden")
	}
}

func TestCheckMarkings(t *testing.T) {
	s := markedSchema()
	if err := s.CheckMarkings(markedScheme(t)); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckMarkings(nil); err == nil || !strings.Contains(err.Error(), "no markings") {
		t.Errorf("markings in the schema without a scheme must be an error, got %v", err)
	}
	s.Objects[0].Properties[1].Marking = "secret"
	if err := s.CheckMarkings(markedScheme(t)); err == nil || !strings.Contains(err.Error(), "unknown marking") {
		t.Errorf("an unknown marking must be an error, got %v", err)
	}
}

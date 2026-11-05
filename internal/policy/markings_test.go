// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package policy

import (
	"strings"
	"testing"

	"github.com/zyvorai/zyntra/internal/graph"
)

const markingsPolicy = `
markings: {levels: [unclassified, cui, cui-specified]}
users:
  - {name: ana, passwordHash: x, roles: [viewer], clearance: cui}
  - {name: bob, passwordHash: y, roles: [viewer]}
`

func TestMarkingsPolicyLoads(t *testing.T) {
	p, err := Parse(markingsPolicy)
	if err != nil {
		t.Fatal(err)
	}
	sc := p.Scheme()
	if sc == nil || sc.Default != "unclassified" {
		t.Fatalf("scheme = %+v", sc)
	}
	if u, _ := p.User("ana"); u.Clearance != "cui" {
		t.Errorf("ana's clearance = %q", u.Clearance)
	}
	var none *Policy
	if none.Scheme() != nil {
		t.Error("a nil policy has no markings")
	}
}

func TestBadMarkingsConfigFailsLoad(t *testing.T) {
	cases := map[string]string{
		"one level":           "markings: {levels: [only]}",
		"duplicate":           "markings: {levels: [a, a]}",
		"uppercase":           "markings: {levels: [a, B]}",
		"clearance no scheme": "users: [{name: ana, passwordHash: x, roles: [viewer], clearance: cui}]",
		"unknown clearance":   "markings: {levels: [a, b]}\nusers: [{name: ana, passwordHash: x, roles: [viewer], clearance: z}]",
	}
	for name, y := range cases {
		if _, err := Parse(y); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

func TestCheckModelValidatesMarkings(t *testing.T) {
	withScheme, err := Parse("markings: {levels: [unclassified, cui]}")
	if err != nil {
		t.Fatal(err)
	}
	off, _ := Parse("rules: []")

	good := &graph.Model{KPIs: []graph.KPI{{ID: "k", Marking: "cui"}}, Actions: []graph.Action{{ID: "a", Marking: "cui"}}}
	if err := withScheme.CheckModel(good); err != nil {
		t.Fatalf("known markings rejected: %v", err)
	}
	bad := &graph.Model{KPIs: []graph.KPI{{ID: "k", Marking: "secret"}}}
	if err := withScheme.CheckModel(bad); err == nil || !strings.Contains(err.Error(), "unknown marking") {
		t.Errorf("unknown KPI marking: %v", err)
	}
	badAction := &graph.Model{Actions: []graph.Action{{ID: "a", Marking: "secret"}}}
	if err := withScheme.CheckModel(badAction); err == nil || !strings.Contains(err.Error(), "unknown marking") {
		t.Errorf("unknown action marking: %v", err)
	}
	// A marking that would be silently ignored is an error, not a pass.
	if err := off.CheckModel(good); err == nil || !strings.Contains(err.Error(), "needs a markings block") {
		t.Errorf("marking with no scheme: %v", err)
	}
	if err := off.CheckModel(&graph.Model{KPIs: []graph.KPI{{ID: "k"}}}); err != nil {
		t.Errorf("an unmarked model must pass with markings off: %v", err)
	}
}

func TestCheckModelRequiresADerivedKPIToBeMarkedAtLeastAsHighAsItsInputs(t *testing.T) {
	p, err := Parse("markings: {levels: [unclassified, cui, cui-specified]}")
	if err != nil {
		t.Fatal(err)
	}
	build := func(derived string) *graph.Model {
		m, err := graph.Parse([]byte(`
kpis:
  - {id: open, value: 1}
  - {id: secret, value: 2, marking: cui}
  - {id: d, value: 0, ` + derived + `, derive: {op: sum, of: [open, secret]}}
`))
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	if err := p.CheckModel(build("marking: cui")); err != nil {
		t.Errorf("a derived KPI at its inputs' highest marking should pass: %v", err)
	}
	if err := p.CheckModel(build("marking: cui-specified")); err != nil {
		t.Errorf("a higher marking should pass: %v", err)
	}
	if err := p.CheckModel(build("owner: o")); err == nil || !strings.Contains(err.Error(), "derived from secret") {
		t.Errorf("an unmarked KPI derived from a cui one must be refused: %v", err)
	}
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"context"
	"encoding/json"
	"github.com/zyvorai/zyntra/internal/adapters"
	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/graph"
	"github.com/zyvorai/zyntra/internal/watch"
	"strings"
	"testing"
	"time"
)

func watchSetup(t *testing.T) *fixture {
	t.Helper()
	s, err := watch.Open("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	f := tenantSetup(t)
	f.s.opt.Watches = s
	return f
}
func watchBody(metric string) string {
	return `{"name":"Latency watch","metric":"` + metric + `","operator":"above","threshold":200,"for_seconds":0,"clear_seconds":0,"max_gap_seconds":120,"enabled":true,"expected_version":0}`
}
func TestWatchRulesAdminGuardTenantReadAndAcknowledgement(t *testing.T) {
	f := watchSetup(t)
	if c := f.do(t, "GET", "/api/v1/watches", "", "", nil); c != 401 {
		t.Fatal(c)
	}
	if c := f.asTenant(t, "PUT", "/api/v1/watches/alpha", "alice", "alpha", []auth.Role{auth.RoleApprover}, watchBody("alpha_latency"), nil); c != 403 {
		t.Fatal(c)
	}
	var r watch.Rule
	if c := f.do(t, "PUT", "/api/v1/watches/alpha", "k3y", watchBody("alpha_latency"), &r); c != 200 || r.Tenant != "alpha" || r.Version != 1 {
		t.Fatalf("%d %+v", c, r)
	}
	if c := f.do(t, "PUT", "/api/v1/watches/beta", "k3y", watchBody("beta_latency"), nil); c != 200 {
		t.Fatal(c)
	}
	if c := f.do(t, "PUT", "/api/v1/watches/alpha", "k3y", watchBody("alpha_latency"), nil); c != 409 {
		t.Fatal("lost update", c)
	}
	for _, body := range []string{strings.TrimSuffix(watchBody("alpha_latency"), "}") + `,"tenant":"beta"}`, watchBody("unknown"), watchBody("alpha_latency") + ` {}`, `null`} {
		if c := f.do(t, "PUT", "/api/v1/watches/bad", "k3y", body, nil); c != 400 {
			t.Fatal("invalid", c, body)
		}
	}
	now := time.Now()
	if err := f.s.opt.Watches.Evaluate(map[string]watch.Observation{"alpha_latency": {At: now, Value: 250, Valid: true, Tenant: "alpha"}, "beta_latency": {At: now, Value: 987654321, Valid: true, Tenant: "beta"}}, now); err != nil {
		t.Fatal(err)
	}
	var view watch.View
	if c := f.asTenant(t, "GET", "/api/v1/watches?tenant=beta", "alice", "alpha", []auth.Role{auth.RoleViewer}, "", &view); c != 200 || len(view.Rules) != 1 || len(view.Incidents) != 1 {
		t.Fatalf("%d %+v", c, view)
	}
	b, _ := json.Marshal(view)
	if strings.Contains(string(b), "beta_latency") || strings.Contains(string(b), "987654321") {
		t.Fatal("tenant leak")
	}
	alphaID := view.Incidents[0].ID
	all := f.s.opt.Watches.View("")
	betaID := ""
	for _, in := range all.Incidents {
		if in.Rule.Tenant == "beta" {
			betaID = in.ID
		}
	}
	body := `{"expected_version":1,"note":"Investigating"}`
	if c := f.asTenant(t, "POST", "/api/v1/watch-incidents/"+alphaID+"/acknowledge", "alice", "alpha", []auth.Role{auth.RoleViewer}, body, nil); c != 403 {
		t.Fatal("viewer ack", c)
	}
	if c := f.asTenant(t, "POST", "/api/v1/watch-incidents/"+betaID+"/acknowledge", "alice", "alpha", []auth.Role{auth.RoleProposer}, body, nil); c != 404 {
		t.Fatal("hidden ack", c)
	}
	var incident watch.Incident
	if c := f.asTenant(t, "POST", "/api/v1/watch-incidents/"+alphaID+"/acknowledge", "alice", "alpha", []auth.Role{auth.RoleProposer}, body, &incident); c != 200 || incident.AckBy != "alice" {
		t.Fatalf("%d %+v", c, incident)
	}
	if c := f.asTenant(t, "POST", "/api/v1/watch-incidents/"+alphaID+"/acknowledge", "alice", "alpha", []auth.Role{auth.RoleProposer}, body, nil); c != 409 {
		t.Fatal("repeat ack", c)
	}
}
func TestWatchRefreshUsesFreshLiveDataAndNeverStaticDefaults(t *testing.T) {
	f := watchSetup(t)
	r := watch.Rule{ID: "live", Name: "Live", Metric: "alpha_latency", Tenant: "alpha", Operator: "above", Threshold: 200, ForSeconds: 0, ClearSeconds: 0, MaxGapSeconds: 120, Enabled: true}
	if _, err := f.s.opt.Watches.Put(r, 0, "admin", time.Now()); err != nil {
		t.Fatal(err)
	}
	f.s.RefreshOnce(context.Background())
	if len(f.s.opt.Watches.View("").Incidents) != 0 {
		t.Fatal("static model triggered")
	}
	f.s.opt.Refresh = func(_ context.Context, m *graph.Model) (adapters.Report, error) {
		return adapters.Report{KPIs: map[string]adapters.KPIUpdate{"alpha_latency": {OK: true}}}, nil
	}
	f.s.RefreshOnce(context.Background())
	if len(f.s.opt.Watches.View("").Incidents) != 1 {
		t.Fatal("fresh live observation missing")
	}
	f.s.opt.Refresh = func(_ context.Context, m *graph.Model) (adapters.Report, error) {
		for i := range m.KPIs {
			if m.KPIs[i].ID == "alpha_latency" {
				m.KPIs[i].Value = 100
			}
		}
		return adapters.Report{KPIs: map[string]adapters.KPIUpdate{"alpha_latency": {Error: "source unavailable"}}}, nil
	}
	f.s.RefreshOnce(context.Background())
	v := f.s.opt.Watches.View("")
	if v.Incidents[0].Status != "open" || v.Runtime["live"].Status != "unavailable" {
		t.Fatal("failed data resolved incident", v)
	}
}

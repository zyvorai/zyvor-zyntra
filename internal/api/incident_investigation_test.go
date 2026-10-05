package api

import (
	"encoding/json"
	"github.com/zyvorai/zyntra/internal/ai"
	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/graph"
	"github.com/zyvorai/zyntra/internal/watch"
	"strings"
	"testing"
	"time"
)

func TestIncidentInvestigationOpeningWindowAndTenantIsolation(t *testing.T) {
	f := watchSetup(t)
	f.s.opt.History = ai.NewHistory(1000)
	f.s.model.KPIs = append(f.s.model.KPIs, graph.KPI{ID: "alpha_queue", Name: "Alpha queue", Unit: "jobs", Tenant: "alpha"})
	for _, tenant := range []string{"alpha", "beta"} {
		if c := f.do(t, "PUT", "/api/v1/watches/"+tenant, "k3y", watchBody(tenant+"_latency"), nil); c != 200 {
			t.Fatal(c)
		}
	}
	opened := time.Now().UTC().Truncate(time.Hour).Add(-25 * time.Hour).Add(30 * time.Minute)
	if err := f.s.opt.Watches.Evaluate(map[string]watch.Observation{
		"alpha_latency": {At: opened, Value: 250, Valid: true, Tenant: "alpha"},
		"beta_latency":  {At: opened, Value: 987654321, Valid: true, Tenant: "beta"},
	}, opened); err != nil {
		t.Fatal(err)
	}
	m, _ := f.s.snapshot()
	end := opened.Truncate(time.Hour)
	for h := -48; h < 24; h++ {
		for j := range m.KPIs {
			if m.KPIs[j].Tenant == "alpha" {
				m.KPIs[j].Value = float64(100 + h*h)
				if h >= 0 {
					m.KPIs[j].Value = 999999999
				}
			}
			if m.KPIs[j].Tenant == "beta" {
				m.KPIs[j].Value = 987654321
			}
		}
		f.s.opt.History.Record(m, end.Add(time.Duration(h)*time.Hour))
	}
	all := f.s.opt.Watches.View("")
	var alpha, beta string
	for _, in := range all.Incidents {
		if in.Rule.Tenant == "alpha" {
			alpha = in.ID
		} else {
			beta = in.ID
		}
	}
	path := "/api/v1/watch-incidents/" + alpha + "/investigation"
	if c := f.do(t, "GET", path, "", "", nil); c != 401 {
		t.Fatal(c)
	}
	var got incidentInvestigation
	if c := f.asTenant(t, "GET", path+"?tenant=beta", "alice", "alpha", []auth.Role{auth.RoleViewer}, "", &got); c != 200 {
		t.Fatal(c)
	}
	if !got.Report.End.Equal(end) || len(got.Report.Hours) != 48 || got.Incident.ID != alpha || got.GeneratedAt.Before(opened) {
		t.Fatalf("wrong context: %+v", got)
	}
	for _, hour := range got.Report.Hours {
		if !hour.Start.Before(end) || hour.Mean == 999999999 {
			t.Fatal("lookahead", hour)
		}
	}
	b, _ := json.Marshal(got)
	if strings.Contains(string(b), "beta_latency") || strings.Contains(string(b), "987654321") {
		t.Fatal("tenant leak")
	}
	for _, id := range []string{beta, "missing"} {
		if c := f.asTenant(t, "GET", "/api/v1/watch-incidents/"+id+"/investigation", "alice", "alpha", []auth.Role{auth.RoleViewer}, "", nil); c != 404 {
			t.Fatal(c)
		}
	}
	// Global readers also stay inside the incident's tenant for comparisons.
	if c := f.do(t, "GET", path, "k3y", "", &got); c != 200 {
		t.Fatal(c)
	}
	if len(got.Report.Comparisons) != 1 || got.Report.Comparisons[0].ID != "alpha_queue" || got.Report.Comparisons[0].Pairs < 24 {
		t.Fatal("missing eligible comparison", got.Report.Comparisons)
	}
	for _, row := range got.Report.Comparisons {
		if strings.HasPrefix(row.ID, "beta_") {
			t.Fatal("cross tenant candidate")
		}
	}
	// Deleting a rule does not erase or change the retained opening context.
	if err := f.s.opt.Watches.Delete("alpha", 1, "admin", time.Now()); err != nil {
		t.Fatal(err)
	}
	var resolved incidentInvestigation
	if c := f.do(t, "GET", path, "k3y", "", &resolved); c != 200 {
		t.Fatal(c)
	}
	if resolved.Incident.Status != "resolved" || resolved.Report.SHA256 != got.Report.SHA256 {
		t.Fatal("deleted rule lost evidence")
	}
	// A changed display unit must not reinterpret the incident threshold/history.
	f.s.mu.Lock()
	for j := range f.s.model.KPIs {
		if f.s.model.KPIs[j].ID == "alpha_latency" {
			f.s.model.KPIs[j].Unit = "seconds"
		}
	}
	f.s.mu.Unlock()
	if c := f.do(t, "GET", path, "k3y", "", nil); c != 409 {
		t.Fatal("unit changed", c)
	}
	// Ownership/unit drift cannot reinterpret the old incident using new metadata.
	f.s.mu.Lock()
	for j := range f.s.model.KPIs {
		if f.s.model.KPIs[j].ID == "alpha_latency" {
			f.s.model.KPIs[j].Unit = "ms"
			f.s.model.KPIs[j].Tenant = "beta"
		}
	}
	f.s.mu.Unlock()
	if c := f.do(t, "GET", path, "k3y", "", nil); c != 409 {
		t.Fatal("ownership changed", c)
	}
}

func TestIncidentInvestigationUnavailableStore(t *testing.T) {
	f := tenantSetup(t)
	if c := f.do(t, "GET", "/api/v1/watch-incidents/missing/investigation", "k3y", "", nil); c != 503 {
		t.Fatal(c)
	}
}

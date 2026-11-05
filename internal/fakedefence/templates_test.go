// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package fakedefence

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/zyntra/internal/adapters"
	"github.com/zyvorai/zyntra/internal/connector"
	"github.com/zyvorai/zyntra/internal/graph"
	"github.com/zyvorai/zyntra/internal/ontology"
)

const templates = "../../examples/defence/"

// server mounts the fake exports under /defence, as `zyntra fake-sources` does.
func server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/defence/", http.StripPrefix("/defence", Handler()))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestKPITemplateReadsPagedExports(t *testing.T) {
	srv := server(t)
	t.Setenv("ZYNTRA_MAINT_URL", srv.URL+"/defence/maintenance")
	t.Setenv("ZYNTRA_SUPPLY_URL", srv.URL+"/defence/supply")
	t.Setenv("ZYNTRA_GRC_URL", srv.URL+"/defence/grc")
	for _, v := range []string{"ZYNTRA_MAINT_TOKEN", "ZYNTRA_SUPPLY_TOKEN", "ZYNTRA_GRC_TOKEN"} {
		t.Setenv(v, Token)
	}
	b, err := os.ReadFile(templates + "kpis-http.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := graph.Parse(b)
	if err != nil {
		t.Fatalf("the template does not parse: %v", err)
	}
	rep, err := adapters.Refresh(context.Background(), m, adapters.Config{})
	if err != nil {
		t.Logf("refresh: %v", err)
	}
	for _, s := range rep.Sources {
		if s.State != adapters.StateOK {
			t.Errorf("source %s is %s: %s", s.Name, s.State, s.Error)
		}
	}
	want := map[string]float64{
		"open_work_orders":           4,          // 5 orders over 3 pages, 4 open
		"open_work_order_age_days":   8,          // (9+14+3+6)/4
		"backordered_requisitions":   2,          // 6 over 3 pages (OData nextLink)
		"parts_fill_rate":            9.0 / 14.0, // sum(filled)/sum(requested)
		"controls_implemented_ratio": 0.75,       // 6 of 8, next in meta.next
	}
	for id, w := range want {
		k, ok := m.KPI(id)
		if !ok {
			t.Fatalf("template lost KPI %s", id)
		}
		if math.Abs(k.Value-w) > 1e-9 {
			t.Errorf("%s = %v, want %v", id, k.Value, w)
		}
	}
}

func TestKPITemplateWithAWrongTokenKeepsFallbacksAndLeaksNothing(t *testing.T) {
	srv := server(t)
	t.Setenv("ZYNTRA_MAINT_URL", srv.URL+"/defence/maintenance")
	t.Setenv("ZYNTRA_SUPPLY_URL", srv.URL+"/defence/supply")
	t.Setenv("ZYNTRA_GRC_URL", srv.URL+"/defence/grc")
	for _, v := range []string{"ZYNTRA_MAINT_TOKEN", "ZYNTRA_SUPPLY_TOKEN", "ZYNTRA_GRC_TOKEN"} {
		t.Setenv(v, "wrong-token-value")
	}
	b, _ := os.ReadFile(templates + "kpis-http.yaml")
	m, err := graph.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	rep, _ := adapters.Refresh(context.Background(), m, adapters.Config{})
	if k, _ := m.KPI("open_work_orders"); k.Value != 0 {
		t.Errorf("a rejected token must leave the fallback, got %v", k.Value)
	}
	if len(rep.Sources) == 0 {
		t.Fatal("no source status reported")
	}
	for _, s := range rep.Sources {
		if s.State == adapters.StateOK {
			t.Errorf("source %s reports ok with a wrong token", s.Name)
		}
		if strings.Contains(s.Error, "wrong-token-value") {
			t.Errorf("the token leaked into a status: %s", s.Error)
		}
	}
}

func TestOntologyTemplatePullsWorkOrders(t *testing.T) {
	srv := server(t)
	t.Setenv("ZYNTRA_MAINT_TOKEN", Token)
	b, err := os.ReadFile(templates + "ontology-rest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(b), "https://maintenance.example.internal/api", srv.URL+"/defence/maintenance")
	d, err := ontology.ParseDefinition([]byte(text))
	if err != nil {
		t.Fatalf("the template does not parse: %v", err)
	}
	if len(d.Connectors) != 1 {
		t.Fatalf("connectors = %d", len(d.Connectors))
	}
	recs, err := connector.REST{Spec: d.Connectors[0], Schema: d.Schema()}.Pull(context.Background(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 5 {
		t.Fatalf("pulled %d work orders, want 5 over 3 pages", len(recs))
	}
	if recs[0].Type != "WorkOrder" || recs[0].Key != "w-1" || recs[0].Props["state"] != "open" {
		t.Errorf("mapping: %+v", recs[0])
	}
}

func TestExportsRequireTheToken(t *testing.T) {
	srv := server(t)
	for _, p := range []string{"/defence/maintenance/work-orders", "/defence/supply/requisitions", "/defence/grc/controls"} {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s without a token = %d, want 401", p, resp.StatusCode)
		}
	}
}

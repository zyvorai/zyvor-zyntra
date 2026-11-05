// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package fakeindustrial

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/zyvorai/zyntra/internal/adapters"
	"github.com/zyvorai/zyntra/internal/graph"
)

const template = "../../examples/industrial/kpis-http.yaml"

// server mounts the fake exports under /industrial, as `zyntra fake-sources` does.
func server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/industrial/", http.StripPrefix("/industrial", Handler()))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func bind(t *testing.T, srv *httptest.Server, token string) {
	t.Setenv("ZYNTRA_MES_URL", srv.URL+"/industrial/mes")
	t.Setenv("ZYNTRA_CMMS_URL", srv.URL+"/industrial/cmms")
	t.Setenv("ZYNTRA_SAFETY_URL", srv.URL+"/industrial/safety")
	for _, v := range []string{"ZYNTRA_MES_TOKEN", "ZYNTRA_CMMS_TOKEN", "ZYNTRA_SAFETY_TOKEN"} {
		t.Setenv(v, token)
	}
}

func load(t *testing.T) *graph.Model {
	t.Helper()
	b, err := os.ReadFile(template)
	if err != nil {
		t.Fatal(err)
	}
	m, err := graph.Parse(b)
	if err != nil {
		t.Fatalf("the template does not parse: %v", err)
	}
	return m
}

func TestTemplateReadsPagedExportsAndDerivesOEE(t *testing.T) {
	srv := server(t)
	bind(t, srv, Token)
	m := load(t)
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
		"machine_availability":    0.7,                                     // 280 of 400 h over 2 pages (links.next)
		"performance_rate":        2800.0 / 3300,                           // total over ideal units
		"first_pass_yield":        2530.0 / 2800,                           // good over total units
		"oee":                     0.7 * (2800.0 / 3300) * (2530.0 / 2800), // derived from the three above
		"machines_down":           1,                                       // m3
		"mttr_avg_h":              6.75,                                    // (4+6+12+5)/4
		"pm_compliance":           0.5,                                     // 2 of 4 preventive done, OData nextLink
		"overdue_pm":              1,                                       // w2
		"overdue_proof_tests":     2,                                       // meta.next: d1 and d3
		"sis_proof_tests_overdue": 1,                                       // d1
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
	if !rep.KPIs["oee"].OK {
		t.Errorf("oee update = %+v, want ok", rep.KPIs["oee"])
	}
}

func TestOEEKeepsItsValueWhenAnInputSourceIsRejected(t *testing.T) {
	srv := server(t)
	bind(t, srv, "wrong-token-value")
	m := load(t)
	rep, _ := adapters.Refresh(context.Background(), m, adapters.Config{})
	if k, _ := m.KPI("oee"); k.Value != 0 {
		t.Errorf("oee = %v: a rejected source must leave the fallback, not a computed number", k.Value)
	}
	if u := rep.KPIs["oee"]; u.OK || !strings.Contains(u.Error, "input") {
		t.Errorf("oee update = %+v, want a fallback that names an input", u)
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

func TestExportsRequireTheToken(t *testing.T) {
	srv := server(t)
	for _, p := range []string{"/industrial/mes/machines", "/industrial/cmms/work-orders", "/industrial/safety/devices"} {
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

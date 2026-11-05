// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package fakeaviation

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

const template = "../../examples/aviation/kpis-http.yaml"

// server mounts the fake exports under /aviation, as `zyntra fake-sources` does.
func server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/aviation/", http.StripPrefix("/aviation", Handler()))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func bind(t *testing.T, srv *httptest.Server, token string) {
	t.Setenv("ZYNTRA_OPS_URL", srv.URL+"/aviation/ops")
	t.Setenv("ZYNTRA_MRO_URL", srv.URL+"/aviation/mro")
	t.Setenv("ZYNTRA_OPS_TOKEN", token)
	t.Setenv("ZYNTRA_MRO_TOKEN", token)
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

func TestTemplateReadsPagedExports(t *testing.T) {
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
		"on_time_rate":             0.4,        // 2 of 5 flights over 3 pages (links.next)
		"long_delays":              2,          // dep_delay_min over 60: 70 and 130
		"turnaround_p90_min":       95,         // nearest rank of 40,45,55,80,95
		"departures_last_24h":      3,          // 2, 5 and 20 hours ago; 30 and 50 are outside
		"deferred_defects_overdue": 2,          // OData nextLink: w-2 and w-3
		"work_order_age_p90_days":  30,         // open ages 4,9,12,30
		"parts_fill_rate":          23.0 / 28., // meta.next
		"aog_critical_stockouts":   2,          // p-2 and p-4
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

func TestTemplateWithAWrongTokenKeepsFallbacksAndLeaksNothing(t *testing.T) {
	srv := server(t)
	bind(t, srv, "wrong-token-value")
	m := load(t)
	rep, _ := adapters.Refresh(context.Background(), m, adapters.Config{})
	if k, _ := m.KPI("long_delays"); k.Value != 0 {
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

func TestExportsRequireTheToken(t *testing.T) {
	srv := server(t)
	for _, p := range []string{"/aviation/ops/flights", "/aviation/mro/work-orders", "/aviation/mro/parts"} {
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

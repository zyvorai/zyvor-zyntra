// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package adapters

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zyvorai/zyntra/internal/graph"
)

func jsonOut(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func fastRetries(t *testing.T) {
	t.Helper()
	old := retryBase
	retryBase = time.Millisecond
	t.Cleanup(func() { retryBase = old })
}

// pagedServer serves 3 pages of work orders: ids 1-2, 3-4, 5.
func pagedServer(t *testing.T, token string, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		switch r.URL.Query().Get("page") {
		case "", "1":
			jsonOut(w, `{"data":{"orders":[{"id":1,"state":"open"},{"id":2,"state":"closed"}]},"links":{"next":"/orders?page=2"}}`)
		case "2":
			jsonOut(w, `{"data":{"orders":[{"id":3,"state":"open"},{"id":4,"state":"open"}]},"links":{"next":"/orders?page=3"}}`)
		default:
			jsonOut(w, `{"data":{"orders":[{"id":5,"state":"open"}]},"links":{"next":null}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPagedHTTPSourceJoinsEveryPage(t *testing.T) {
	var hits atomic.Int64
	srv := pagedServer(t, "tok", &hits)
	t.Setenv("ZYNTRA_TEST_API", srv.URL)
	t.Setenv("ZYNTRA_TEST_TOKEN", "tok")
	m := model(t, `
name: t
kpis:
  - id: open_orders
    value: 0
    source:
      kind: http
      url: "${ZYNTRA_TEST_API}/orders"
      headers: {Authorization: "Bearer ${ZYNTRA_TEST_TOKEN}"}
      next: links.next
      items: data.orders
      field: "#(state=open)"
  - id: total_orders
    value: 0
    source:
      kind: http
      url: "${ZYNTRA_TEST_API}/orders"
      headers: {Authorization: "Bearer ${ZYNTRA_TEST_TOKEN}"}
      next: links.next
      items: data.orders
      field: "#"
edges: []
actions: []
`)
	rep, err := Refresh(context.Background(), m, Config{})
	_ = err
	if k, _ := m.KPI("open_orders"); k.Value != 4 {
		t.Errorf("open_orders = %v, want 4 (pages joined)", k.Value)
	}
	if k, _ := m.KPI("total_orders"); k.Value != 5 {
		t.Errorf("total_orders = %v, want 5", k.Value)
	}
	if hits.Load() != 3 {
		t.Errorf("server saw %d requests, want 3: both KPIs share one fetch", hits.Load())
	}
	for _, s := range rep.Sources {
		if s.State != StateOK {
			t.Errorf("source %s is %s: %s", s.Name, s.State, s.Error)
		}
	}
}

func TestNextLinkToAnotherHostIsRefused(t *testing.T) {
	var other atomic.Int64
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		other.Add(1)
		jsonOut(w, `{"orders":[],"next":""}`)
	}))
	defer elsewhere.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, fmt.Sprintf(`{"orders":[{"id":1}],"next":%q}`, elsewhere.URL+"/steal"))
	}))
	defer srv.Close()
	t.Setenv("ZYNTRA_TEST_API", srv.URL)
	m := model(t, `
name: t
kpis:
  - {id: n, value: 7, source: {kind: http, url: "${ZYNTRA_TEST_API}/x", next: next, items: orders, field: "#", headers: {Authorization: "Bearer s3cret"}}}
edges: []
actions: []
`)
	rep, _ := Refresh(context.Background(), m, Config{})
	if k, _ := m.KPI("n"); k.Value != 7 {
		t.Errorf("a refused page must keep the fallback value, got %v", k.Value)
	}
	if other.Load() != 0 {
		t.Fatal("the next link to another host was fetched, with the credentials")
	}
	found := false
	for _, s := range rep.Sources {
		found = found || strings.Contains(s.Error, "another host")
		if strings.Contains(s.Error, "s3cret") {
			t.Errorf("secret in status: %s", s.Error)
		}
	}
	if !found {
		t.Errorf("status should explain the refusal: %+v", rep.Sources)
	}
}

func TestPagingStopsAtMaxPagesAndOnLoops(t *testing.T) {
	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := n.Add(1)
		jsonOut(w, fmt.Sprintf(`{"items":[{"v":1}],"next":"/p%d"}`, p))
	}))
	defer srv.Close()
	t.Setenv("ZYNTRA_TEST_API", srv.URL)
	m := model(t, `
name: t
kpis:
  - {id: n, value: 1, source: {kind: http, url: "${ZYNTRA_TEST_API}/p0", next: next, items: items, maxPages: 3, field: "#"}}
edges: []
actions: []
`)
	rep, _ := Refresh(context.Background(), m, Config{})
	if k, _ := m.KPI("n"); k.Value != 1 {
		t.Errorf("an endless feed must not produce a partial value, got %v", k.Value)
	}
	if n.Load() != 3 {
		t.Errorf("fetched %d pages, want exactly the cap of 3", n.Load())
	}
	if len(rep.Sources) == 0 || !strings.Contains(rep.Sources[0].Error, "more than 3 pages") {
		t.Errorf("want a page-cap error, got %+v", rep.Sources)
	}

	loop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, `{"items":[{"v":1}],"next":"/loop"}`)
	}))
	defer loop.Close()
	t.Setenv("ZYNTRA_TEST_API", loop.URL)
	m2 := model(t, `
name: t
kpis:
  - {id: n, value: 1, source: {kind: http, url: "${ZYNTRA_TEST_API}/loop", next: next, items: items, field: "#"}}
edges: []
actions: []
`)
	rep, _ = Refresh(context.Background(), m2, Config{})
	if len(rep.Sources) == 0 || !strings.Contains(rep.Sources[0].Error, "repeats") {
		t.Errorf("want a loop error, got %+v", rep.Sources)
	}
}

func TestRetriesOnlyOnThrottleAndServerErrors(t *testing.T) {
	fastRetries(t)
	var calls atomic.Int64
	flaky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		jsonOut(w, `{"v":42}`)
	}))
	defer flaky.Close()
	t.Setenv("ZYNTRA_TEST_API", flaky.URL)
	m := model(t, `
name: t
kpis:
  - {id: a, value: 0, source: {kind: http, url: "${ZYNTRA_TEST_API}/v", field: v, retries: 2}}
edges: []
actions: []
`)
	_, _ = Refresh(context.Background(), m, Config{})
	if k, _ := m.KPI("a"); k.Value != 42 || calls.Load() != 3 {
		t.Errorf("a = %v after %d calls, want 42 after 3", k.Value, calls.Load())
	}

	// Not enough retries: it gives up and keeps the fallback.
	calls.Store(-10)
	m2 := model(t, `
name: t
kpis:
  - {id: a, value: 5, source: {kind: http, url: "${ZYNTRA_TEST_API}/v", field: v, retries: 1}}
edges: []
actions: []
`)
	_, _ = Refresh(context.Background(), m2, Config{})
	if k, _ := m2.KPI("a"); k.Value != 5 {
		t.Errorf("a = %v, want the fallback 5", k.Value)
	}

	// A 404 is not retried.
	var nf atomic.Int64
	gone := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nf.Add(1)
		http.NotFound(w, r)
	}))
	defer gone.Close()
	t.Setenv("ZYNTRA_TEST_API", gone.URL)
	m3 := model(t, `
name: t
kpis:
  - {id: a, value: 5, source: {kind: http, url: "${ZYNTRA_TEST_API}/v", field: v, retries: 3}}
edges: []
actions: []
`)
	_, _ = Refresh(context.Background(), m3, Config{})
	if nf.Load() != 1 {
		t.Errorf("a 404 was requested %d times, want 1", nf.Load())
	}
}

func TestPagingAndRetryOptionsAreValidated(t *testing.T) {
	bad := map[string]string{
		"retries too high": `{kind: http, url: "http://x/", retries: 9, field: a}`,
		"maxPages too big": `{kind: http, url: "http://x/", next: n, maxPages: 500, field: a}`,
		"next on sheet":    `{kind: sheet, url: "http://x/", next: n, field: a}`,
		"items needs next": `{kind: http, url: "http://x/", items: i, field: a}`,
	}
	for name, src := range bad {
		if _, err := graph.Parse([]byte("name: t\nkpis:\n  - {id: a, value: 0, source: " + src + "}\nedges: []\nactions: []\n")); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

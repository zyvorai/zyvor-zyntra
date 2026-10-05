// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zyvorai/zyntra/internal/ai"
	"github.com/zyvorai/zyntra/internal/auth"
)

func TestAnalyticsQueryAuthStrictnessAndTenantIsolation(t *testing.T) {
	f := tenantSetup(t)
	now := time.Now()
	f.s.opt.History.Restore(map[string][]ai.Point{"alpha_latency": {{T: now.Add(-time.Hour), V: 100}, {T: now, V: 200}}, "beta_latency": {{T: now, V: 987654321}}})
	if code := f.do(t, "GET", "/api/v1/analytics/catalog", "", "", nil); code != 401 {
		t.Fatal(code)
	}
	if code := f.do(t, "POST", "/api/v1/analytics/query", "", `{}`, nil); code != 401 {
		t.Fatal(code)
	}
	var catalog struct{ Metrics []ai.Metric }
	if code := f.asTenant(t, "GET", "/api/v1/analytics/catalog?tenant=beta", "alice", "alpha", []auth.Role{auth.RoleViewer}, "", &catalog); code != 200 || len(catalog.Metrics) != 1 || catalog.Metrics[0].ID != "alpha_latency" {
		t.Fatalf("%d %+v", code, catalog)
	}
	for _, body := range []string{`{"metrics":["beta_latency"],"operation":"mean","window_hours":24}`, `{"metrics":["alpha_latency"],"operation":"mean","window_hours":24,"sql":"select *"}`, `{"metrics":["alpha_latency"],"operation":"mean","window_hours":24} {}`, `{"metrics":["alpha_latency"],"operation":"mean","window_hours":0}`} {
		if code := f.asTenant(t, "POST", "/api/v1/analytics/query", "alice", "alpha", []auth.Role{auth.RoleViewer}, body, nil); code != 400 {
			t.Fatalf("%d %s", code, body)
		}
	}
	var result ai.QueryResult
	if code := f.asTenant(t, "POST", "/api/v1/analytics/query?tenant=beta", "alice", "alpha", []auth.Role{auth.RoleViewer}, `{"metrics":["alpha_latency"],"operation":"mean","window_hours":24}`, &result); code != 200 || len(result.Rows) != 1 || *result.Rows[0].Value != 150 {
		t.Fatalf("%d %+v", code, result)
	}
	var a ai.Answer
	if code := f.asTenant(t, "POST", "/api/v1/ai/ask", "alice", "alpha", []auth.Role{auth.RoleViewer}, `{"question":"average alpha_latency last 24 hours","scope":"analytics"}`, &a); code != 200 || a.AnalyticsQuery == nil {
		t.Fatalf("%d %+v", code, a)
	}
	data, _ := json.Marshal(a)
	for _, secret := range []string{"beta_latency", "987654321", "source", "ops"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("leaked %s", secret)
		}
	}
	a = ai.Answer{}
	if code := f.asTenant(t, "POST", "/api/v1/ai/ask", "alice", "alpha", []auth.Role{auth.RoleViewer}, `{"question":"mean beta_latency","scope":"analytics"}`, &a); code != 200 || a.AnalyticsQuery != nil {
		t.Fatalf("hidden metric answer %d %+v", code, a)
	}
}
func TestAnalyticsQueryNoDataAndStaleWarnings(t *testing.T) {
	f := setup(t, nil)
	f.s.opt.History = ai.NewHistory(720)
	var result ai.QueryResult
	if code := f.do(t, "POST", "/api/v1/analytics/query", "k3y", `{"metrics":["lat"],"operation":"mean","window_hours":1}`, &result); code != 200 || result.Rows[0].Value != nil {
		t.Fatalf("%d %+v", code, result)
	}
	f.s.opt.History.Restore(map[string][]ai.Point{"lat": {{T: time.Now().Add(-10 * time.Minute), V: 42}}})
	result = ai.QueryResult{}
	if code := f.do(t, "POST", "/api/v1/analytics/query", "k3y", `{"metrics":["lat"],"operation":"latest","window_hours":1}`, &result); code != 200 || result.Rows[0].Warning == "" || result.Rows[0].Value == nil || *result.Rows[0].Value != 42 {
		t.Fatalf("stale historical query %d %+v", code, result)
	}

}

func TestTenantAnalyticsLLMReceivesOnlyOwnCatalog(t *testing.T) {
	f := tenantSetup(t)
	var leaked atomic.Bool
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		encoded, _ := json.Marshal(body)
		for _, secret := range []string{"beta_latency", "Beta API latency", "provider infrastructure", "owner"} {
			if strings.Contains(string(encoded), secret) {
				leaked.Store(true)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": `{"metrics":["beta_latency"],"operation":"mean","window_hours":24}`}}}})
	}))
	defer gateway.Close()
	f.s.opt.AI = &ai.Engine{LLM: ai.NewProvider(gateway.URL, "key", "test", "test", false)}
	var answer ai.Answer
	code := f.asTenant(t, "POST", "/api/v1/ai/ask", "alice", "alpha", []auth.Role{auth.RoleViewer}, `{"question":"compare service levels","scope":"analytics"}`, &answer)
	if code != 200 || leaked.Load() || answer.AnalyticsQuery != nil || answer.LLMError == "" {
		t.Fatalf("%d leaked=%v %+v", code, leaked.Load(), answer)
	}
	// No authorized metric catalog is also a deterministic refusal.
	answer = f.s.opt.AI.QueryAnswer(context.Background(), "mean alpha_latency", nil, nil, time.Now())
	if answer.AnalyticsQuery != nil {
		t.Fatal("empty catalog accepted")
	}
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"encoding/json"
	"github.com/zyvorai/zyntra/internal/ai"
	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/graph"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInvestigationAPIAuthStrictnessAndTenantScope(t *testing.T) {
	f := tenantSetup(t)
	f.s.model.KPIs = append(f.s.model.KPIs, graph.KPI{ID: "alpha_queue", Name: "Alpha queue", Tenant: "alpha", Unit: "jobs"})
	now := time.Now().UTC().Truncate(time.Hour)
	data := map[string][]ai.Point{}
	for _, id := range []string{"alpha_latency", "alpha_queue", "beta_latency"} {
		for i := 0; i < 72; i++ {
			value := float64(i % 5)
			if id == "beta_latency" {
				value = 987654321
			}
			data[id] = append(data[id], ai.Point{T: now.Add(time.Duration(i-72)*time.Hour + time.Minute), V: value})
		}
	}
	f.s.opt.History.Restore(data)
	body := `{"metric":"alpha_latency","window_hours":72,"recent_hours":6,"max_lag_hours":6}`
	if code := f.do(t, "POST", "/api/v1/analytics/investigate", "", body, nil); code != 401 {
		t.Fatal(code)
	}
	if code := f.do(t, "POST", "/api/v1/analytics/investigate", "ex3c", body, nil); code != 403 {
		t.Fatalf("machine role %d", code)
	}
	var report ai.InvestigationReport
	if code := f.asTenant(t, "POST", "/api/v1/analytics/investigate?tenant=beta", "alice", "alpha", []auth.Role{auth.RoleViewer}, body, &report); code != 200 || report.Target.ID != "alpha_latency" || len(report.Comparisons) != 1 || report.Comparisons[0].ID != "alpha_queue" {
		t.Fatalf("%d %+v", code, report)
	}
	encoded, _ := json.Marshal(report)
	for _, secret := range []string{"beta_latency", "Beta API latency", "987654321", "owner", "source", "ops"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("tenant leak %s", secret)
		}
	}
	for _, bad := range []string{strings.Replace(body, "alpha_latency", "beta_latency", 1), strings.TrimSuffix(body, "}") + `,"candidates":["beta_latency"]}`, strings.TrimSuffix(body, "}") + `,"sql":"select *"}`, body + ` {}`, strings.Repeat(" ", 8200) + body} {
		if code := f.asTenant(t, "POST", "/api/v1/analytics/investigate", "alice", "alpha", []auth.Role{auth.RoleViewer}, bad, nil); code != 400 {
			t.Fatalf("accepted invalid request %d", code)
		}
	}
	var answer ai.Answer
	if code := f.asTenant(t, "POST", "/api/v1/ai/ask", "alice", "alpha", []auth.Role{auth.RoleViewer}, `{"question":"investigate alpha_latency","scope":"investigation"}`, &answer); code != 200 || answer.Investigation == nil || answer.Mode != "heuristic" {
		t.Fatalf("%d %+v", code, answer)
	}
	encoded, _ = json.Marshal(answer)
	if strings.Contains(string(encoded), "beta_latency") {
		t.Fatal("Ask leaked other tenant")
	}
	answer = ai.Answer{}
	if code := f.asTenant(t, "POST", "/api/v1/ai/ask", "alice", "alpha", []auth.Role{auth.RoleViewer}, `{"question":"investigate beta_latency","scope":"investigation"}`, &answer); code != 200 || answer.Investigation != nil {
		t.Fatalf("hidden target %d %+v", code, answer)
	}
}
func TestInvestigationDoesNotCallGatewayOrChangeModel(t *testing.T) {
	f := setup(t, nil)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("investigation sent data to gateway")
		w.WriteHeader(500)
	}))
	defer gateway.Close()
	f.s.opt.AI = &ai.Engine{LLM: ai.NewProvider(gateway.URL, "key", "test", "test", false)}
	before, _ := json.Marshal(f.s.model)
	var answer ai.Answer
	if code := f.do(t, "POST", "/api/v1/ai/ask", "k3y", `{"question":"investigate lat","scope":"investigation"}`, &answer); code != 200 || answer.Investigation == nil || answer.Investigation.Shift.Status != "warming" {
		t.Fatalf("%d %+v", code, answer)
	}
	after, _ := json.Marshal(f.s.model)
	if string(before) != string(after) {
		t.Fatal("investigation changed model")
	}
}

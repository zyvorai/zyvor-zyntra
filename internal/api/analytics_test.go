// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zyvorai/zyntra/internal/adapters"
	"github.com/zyvorai/zyntra/internal/ai"
	"github.com/zyvorai/zyntra/internal/graph"
)

func TestAnalyticsAPICompatibilityAndAuthorization(t *testing.T) {
	f := setup(t, nil)
	request := func(key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/ai/insights", nil)
		if key != "" {
			r.Header.Set("Authorization", "Bearer "+key)
		}
		w := httptest.NewRecorder()
		f.s.Handler().ServeHTTP(w, r)
		return w
	}
	if w := request(""); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth %d", w.Code)
	}
	w := request("k3y")
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var body map[string]json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &body)
	for _, field := range []string{"anomalies", "forecasts", "analytics"} {
		if body[field] == nil {
			t.Fatalf("missing %s", field)
		}
	}
	if tenantAllowed("GET", "/api/v1/ai/insights") {
		t.Fatal("provider analytics exposed to tenant")
	}
}
func TestFailedAndUnreportedLiveInputsDoNotEnterHistory(t *testing.T) {
	m, err := graph.Parse([]byte(`kpis:
 - {id: a, value: 10, source: {kind: manual}}
 - {id: b, value: 20, source: {kind: manual}}
`))
	if err != nil {
		t.Fatal(err)
	}
	h := ai.NewHistory(100)
	s := New(Options{Model: m, History: h, Refresh: func(context.Context, *graph.Model) (adapters.Report, error) {
		return adapters.Report{KPIs: map[string]adapters.KPIUpdate{"a": {Error: "down"}}}, nil
	}})
	s.RefreshOnce(context.Background())
	if len(h.Series("a")) != 0 || len(h.Series("b")) != 0 {
		t.Fatal("failed or unreported source recorded")
	}
}
func TestAnalyticsBlocksOldHistory(t *testing.T) {
	f := setup(t, nil)
	f.s.opt.History.Restore(map[string][]ai.Point{"lat": {{T: time.Now().Add(-time.Hour), V: 300}}})
	if !f.s.analyticsUnavailable(f.s.model)["lat"] {
		t.Fatal("old history is available")
	}
}

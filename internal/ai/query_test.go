// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package ai

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var queryCatalog = []Metric{{ID: "latency", Name: "API latency", Unit: "ms"}, {ID: "revenue", Name: "Revenue", Unit: "USD"}}

func TestRunQueryArithmeticWindowAndEvidence(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	points := []Point{{now.Add(time.Minute), 999}, {now.Add(-3 * time.Hour), 1000}, {now.Add(-time.Hour), 20}, {now.Add(-2 * time.Hour), 10}, {now, 40}}
	for op, want := range map[string]float64{"latest": 40, "mean": 70.0 / 3, "min": 10, "max": 40, "change": 30} {
		result, err := RunQuery(Query{Metrics: []string{"latency"}, Operation: op, WindowHours: 2}, queryCatalog, map[string][]Point{"latency": points}, now)
		if err != nil {
			t.Fatal(err)
		}
		row := result.Rows[0]
		if row.Value == nil || math.Abs(*row.Value-want) > 1e-9 || row.Samples != 3 || row.First.V != 10 || row.Last.V != 40 || len(row.SHA256) != 64 {
			t.Fatalf("%s %+v", op, row)
		}
		second, _ := RunQuery(result.Query, queryCatalog, map[string][]Point{"latency": points}, now)
		if row.SHA256 != second.Rows[0].SHA256 {
			t.Fatal("unstable evidence")
		}
	}
	// Units stay in separate rows, and missing history is null, never zero.
	result, err := RunQuery(Query{Metrics: []string{"latency", "revenue"}, Operation: "mean", WindowHours: 2}, queryCatalog, map[string][]Point{"latency": points}, now)
	if err != nil || result.Rows[0].Unit != "ms" || result.Rows[1].Value != nil || result.Rows[1].Samples != 0 {
		t.Fatalf("%+v %v", result, err)
	}
}
func TestQueryTrendMissingBucketsAndEndpoints(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	history := map[string][]Point{"latency": {{now.Add(-4 * time.Hour), 10}, {now.Add(-3*time.Hour - time.Minute), 30}, {now, 50}}}
	result, err := RunQuery(Query{Metrics: []string{"latency"}, Operation: "trend", WindowHours: 4, BucketHours: 1}, queryCatalog, history, now)
	if err != nil {
		t.Fatal(err)
	}
	row := result.Rows[0]
	if row.Value != nil || len(row.Buckets) != 2 || row.Buckets[0].Value != 20 || row.Buckets[0].Samples != 2 || row.Buckets[1].Value != 50 || !row.Buckets[1].End.Equal(now) {
		t.Fatalf("%+v", row)
	}
	history["latency"] = history["latency"][:1]
	result, err = RunQuery(Query{Metrics: []string{"latency"}, Operation: "change", WindowHours: 4}, queryCatalog, history, now)
	if err != nil || result.Rows[0].Value != nil || result.Rows[0].Note == "" {
		t.Fatal("single sample change must be unavailable")
	}
}
func TestQueryValidationAndNumericBounds(t *testing.T) {
	for _, body := range []string{`{"metrics":["secret"],"operation":"mean","window_hours":24}`, `{"metrics":["latency","latency"],"operation":"mean","window_hours":24}`, `{"metrics":["latency"],"operation":"sql","window_hours":24}`, `{"metrics":["latency"],"operation":"mean","window_hours":721}`, `{"metrics":["latency"],"operation":"trend","window_hours":24}`, `{"metrics":["latency"],"operation":"mean","window_hours":24,"bucket_hours":1}`, `{"metrics":["latency"],"operation":"mean","window_hours":24,"sql":"DROP TABLE x"}`, `{} {}`, `null`} {
		q, err := DecodeQuery(strings.NewReader(body))
		if err == nil {
			err = q.Validate(queryCatalog)
		}
		if err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	now := time.Now()
	result, err := RunQuery(Query{Metrics: []string{"latency"}, Operation: "mean", WindowHours: 1}, queryCatalog, map[string][]Point{"latency": {{now.Add(-time.Minute), math.MaxFloat64}, {now.Add(-time.Second), math.MaxFloat64}, {now, math.MaxFloat64}}}, now)
	if err != nil || !finite(*result.Rows[0].Value) {
		t.Fatalf("finite mean %v", err)
	}
	_, err = RunQuery(Query{Metrics: []string{"latency"}, Operation: "change", WindowHours: 1}, queryCatalog, map[string][]Point{"latency": {{now.Add(-time.Minute), -math.MaxFloat64}, {now.Add(-time.Second), math.MaxFloat64}, {now, math.MaxFloat64}}}, now)
	if err == nil {
		t.Fatal("overflow accepted")
	}
}
func TestLocalQueryGrammarAndAmbiguity(t *testing.T) {
	for _, tc := range []struct {
		question, op  string
		hours, bucket int
	}{{"average API latency over the last 24 hours", "mean", 24, 0}, {"daily trend latency over the past 7 days", "trend", 168, 24}, {"latest revenue", "latest", 24, 0}, {"maximum latency", "max", 24, 0}, {"change latency last 2 hours", "change", 2, 0}} {
		q, err := LocalQuery(tc.question, queryCatalog)
		if err != nil || q.Operation != tc.op || q.WindowHours != tc.hours || q.BucketHours != tc.bucket {
			t.Fatalf("%s: %+v %v", tc.question, q, err)
		}
	}
	for _, s := range []string{"average hidden", "average latency and maximum revenue", "average latency yesterday", "mean latency last 2 weeks", "trend latency past 100 days", "show latency", "mean latency last 2 hours past 3 days", "mean latency last 0 hours", "mean latency last 999999999999999999999999 days", "daily trend latency last 2 hours", "mean latency last 2 hours since Monday", "mean latency 30 days", "median latency"} {
		if q, err := LocalQuery(s, queryCatalog); err == nil {
			t.Fatalf("accepted ambiguous %q: %+v", s, q)
		}
	}
}
func TestQueryLLMValidationAndDeterministicAnswer(t *testing.T) {
	for _, tc := range []struct {
		reply string
		valid bool
	}{{`{"metrics":["latency"],"operation":"mean","window_hours":24}`, true}, {`{"metrics":["secret"],"operation":"mean","window_hours":24}`, false}, {`{"metrics":["latency"],"operation":"mean","window_hours":24,"sql":"select *"}`, false}, {`{} trailing`, false}} {
		t.Run(tc.reply, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req map[string]any
				json.NewDecoder(r.Body).Decode(&req)
				data, _ := json.Marshal(req)
				if strings.Contains(string(data), "SECRET_SOURCE") {
					t.Error("unfiltered source reached LLM")
				}
				json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": tc.reply}}}})
			}))
			defer server.Close()
			e := Engine{LLM: &Provider{BaseURL: server.URL, Model: "test", APIKey: "key", HTTP: server.Client()}}
			now := time.Now()
			a := e.QueryAnswer(context.Background(), "average latency", queryCatalog, map[string][]Point{"latency": {{now, 42}}, "SECRET_SOURCE": {{now, 999}}}, now)
			if a.AnalyticsQuery == nil || *a.AnalyticsQuery.Rows[0].Value != 42 || !strings.Contains(a.Text, "42 ms") {
				t.Fatalf("%+v", a)
			}
			if tc.valid && a.Mode != "llm" {
				t.Fatalf("valid translation %+v", a)
			}
			if !tc.valid && (a.Mode != "heuristic" || a.LLMError == "") {
				t.Fatalf("unsafe translation %+v", a)
			}
		})
	}
}

func TestQueryMetricNameCannotChangeOperation(t *testing.T) {
	catalog := []Metric{{ID: "daily_revenue", Name: "Daily mean revenue", Unit: "USD"}}
	for _, question := range []string{"average daily_revenue last 24 hours", "maximum Daily mean revenue last 24 hours"} {
		q, err := LocalQuery(question, catalog)
		if err != nil || q.Operation == "trend" {
			t.Fatalf("%s %+v %v", question, q, err)
		}
	}
}

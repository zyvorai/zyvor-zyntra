package api

import (
	"encoding/json"
	"github.com/zyvorai/zyntra/internal/inputs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIngestJournalQualityGate(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"data-contracts.json":   `[{"name":"gateway","object_type":"Reading","source":{"kind":"webhook","reference":"gateway"},"schema":[{"name":"value","type":"number","required":true}]}]`,
		"ingest-contracts.json": `{"gateway":{"contract":"gateway"}}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	f := setupWith(t, `kpis:
 - {id: load, value: 0, source: {kind: webhook-in, name: gateway, field: value}}
actions: []
`, func(o *Options) { o.PackDir = dir; o.Inputs, _ = inputs.Open("") })
	post := func(body, key string) int {
		req, _ := http.NewRequest("POST", f.ts.URL+"/api/v1/ingest/gateway", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer k3y")
		req.Header.Set("Idempotency-Key", key)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if c := post(`{"value":5}`, "good"); c != 202 {
		t.Fatal(c)
	}
	if c := post(`{"value":5}`, "good"); c != 202 {
		t.Fatalf("retry %d", c)
	}
	if c := post(`{"value":6}`, "good"); c != 409 {
		t.Fatalf("conflict %d", c)
	}
	if c := post(`{"value":"bad"}`, "bad"); c != 422 {
		t.Fatalf("quality %d", c)
	}
	d, _ := f.s.opt.Inputs.Webhook("gateway")
	if d.Body.(map[string]any)["value"] != 5.0 {
		t.Fatal("quarantine changed latest value")
	}
	var page inputs.EventPage
	if c := f.do(t, "GET", "/api/v1/inputs/gateway/events", "k3y", "", &page); c != 200 || len(page.Events) != 2 {
		b, _ := json.Marshal(page)
		t.Fatalf("%d %s", c, b)
	}
	if tenantAllowed("GET", "/api/v1/inputs/gateway/events") {
		t.Fatal("tenant can read provider events")
	}
}

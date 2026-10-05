// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zyvorai/zyntra/internal/ai"
	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/knowledge"
)

func knowledgeSetup(t *testing.T) *fixture {
	t.Helper()
	st, err := knowledge.Open("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return setup(t, func(o *Options) { o.Knowledge = st })
}
func TestKnowledgeCRUDVersionAndAdminGate(t *testing.T) {
	f := knowledgeSetup(t)
	var doc knowledge.Document
	body := `{"document":{"title":"Inventory SOP","text":"Reorder inventory before zero.","visibility":"shared"},"expected_version":0}`
	if code := f.do(t, "PUT", "/api/v1/knowledge/documents/inventory", "", body, nil); code != 401 {
		t.Fatalf("unauth: %d", code)
	}
	if code := f.asTenant(t, "PUT", "/api/v1/knowledge/documents/inventory", "alice", "alpha", []auth.Role{auth.RoleApprover}, body, nil); code != 403 {
		t.Fatalf("tenant write: %d", code)
	}
	if code := f.do(t, "PUT", "/api/v1/knowledge/documents/inventory", "k3y", body, &doc); code != 200 || doc.Version != 1 || doc.Hash == "" {
		t.Fatalf("%d %+v", code, doc)
	}
	if code := f.do(t, "PUT", "/api/v1/knowledge/documents/inventory", "k3y", body, nil); code != 409 {
		t.Fatalf("lost update: %d", code)
	}
	if code := f.do(t, "GET", "/api/v1/knowledge/documents/inventory?version=bad", "k3y", "", nil); code != 400 {
		t.Fatalf("version: %d", code)
	}
	if code := f.do(t, "DELETE", "/api/v1/knowledge/documents/inventory", "k3y", `{"expected_version":1}`, nil); code != 200 {
		t.Fatalf("delete %d", code)
	}
	if code := f.do(t, "GET", "/api/v1/knowledge/documents/inventory", "k3y", "", nil); code != 404 {
		t.Fatalf("deleted %d", code)
	}
}
func TestKnowledgeTenantSearchAskAndNoProviderLeak(t *testing.T) {
	f := knowledgeSetup(t)
	for _, d := range []knowledge.Document{
		{ID: "public", Title: "Stock SOP", Text: "Inventory reorder at ten units.", Visibility: "shared"},
		{ID: "alpha", Title: "Alpha SOP", Text: "Alpha inventory lead time is two days.", Visibility: "tenant", Tenant: "alpha"},
		{ID: "beta", Title: "BETA SECRET", Text: "Inventory secret token BETA_SECRET.", Visibility: "tenant", Tenant: "beta"},
		{ID: "provider", Title: "PROVIDER SECRET", Text: "Inventory secret token PROVIDER_SECRET.", Visibility: "provider"},
	} {
		if _, err := f.s.opt.Knowledge.Put(d, 0, "admin"); err != nil {
			t.Fatal(err)
		}
	}
	var answer ai.Answer
	code := f.asTenant(t, "POST", "/api/v1/ai/ask", "alice", "alpha", []auth.Role{auth.RoleViewer}, `{"question":"Inventory procedure","scope":"documents"}`, &answer)
	if code != 200 || answer.Intent != "documents" || len(answer.DocumentCitations) != 2 {
		t.Fatalf("%d %+v", code, answer)
	}
	data, _ := json.Marshal(answer)
	if strings.Contains(string(data), "SECRET") || strings.Contains(string(data), "lat") {
		t.Fatal("provider state leaked to tenant document answer")
	}
	if code = f.asTenant(t, "GET", "/api/v1/knowledge/documents/beta", "alice", "alpha", []auth.Role{auth.RoleViewer}, "", nil); code != 404 {
		t.Fatalf("hidden document %d", code)
	}
	if code = f.asTenant(t, "GET", "/api/v1/knowledge/documents/public?version=1", "alice", "alpha", []auth.Role{auth.RoleViewer}, "", nil); code != 200 {
		t.Fatalf("shared revision %d", code)
	}
	var hits struct{ Citations []knowledge.Citation }
	code = f.asTenant(t, "GET", "/api/v1/knowledge/search?q=inventory&limit=5", "alice", "alpha", []auth.Role{auth.RoleViewer}, "", &hits)
	if code != 200 || len(hits.Citations) != 2 {
		t.Fatalf("%d %+v", code, hits)
	}
	code = f.do(t, "POST", "/api/v1/ai/ask", "k3y", `{"question":"unmatched","scope":"documents"}`, &answer)
	if code != 200 || !strings.Contains(answer.Text, "No matching passage") {
		t.Fatalf("%d %+v", code, answer)
	}
	if len(f.s.opt.Store.List()) != 0 || len(f.run.calls) != 0 {
		t.Fatal("knowledge caused execution or proposals")
	}
}
func TestKnowledgeAutoRoutingDoesNotAlterOperationalAsk(t *testing.T) {
	f := knowledgeSetup(t)
	var a ai.Answer
	f.do(t, "POST", "/api/v1/ai/ask", "k3y", `{"question":"What should we do?"}`, &a)
	if a.Intent != "plan" {
		t.Fatalf("%+v", a)
	}
	f.do(t, "POST", "/api/v1/ai/ask", "k3y", `{"question":"According to the runbook, what should we do?"}`, &a)
	if a.Intent != "documents" {
		t.Fatalf("%+v", a)
	}
	if code := f.do(t, "POST", "/api/v1/ai/ask", "k3y", `{"question":"hello","scope":"unknown"}`, nil); code != http.StatusBadRequest {
		t.Fatalf("%d", code)
	}
	// Running the model/AI engine directly still has no knowledge or tool access.
	answer := f.s.opt.AI.Ask(context.Background(), "what should we do?", f.s.aiSnapshot())
	if answer.Intent != "plan" {
		t.Fatalf("%+v", answer)
	}
}

func TestKnowledgePackSelectionKeepsIndependentCorpus(t *testing.T) {
	a := knowledgeSetup(t)
	b := knowledgeSetup(t)
	for _, f := range []*fixture{a, b} {
		title := "First"
		if f == b {
			title = "Second"
		}
		if _, err := f.s.opt.Knowledge.Put(knowledge.Document{ID: "stock", Title: title, Text: title + " inventory instructions", Visibility: "shared"}, 0, "admin"); err != nil {
			t.Fatal(err)
		}
	}
	mux := PackMux(map[string]*Server{"a": a.s, "b": b.s}, "a")
	req := httptest.NewRequest("GET", "/api/v1/knowledge/search?q=inventory&pack=b", nil)
	req.Header.Set("Authorization", "Bearer k3y")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Second") || strings.Contains(w.Body.String(), "First") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

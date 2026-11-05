// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/policy"
)

const markedModel = `
name: marked
kpis:
  - {id: queue, name: Queue depth, value: 20, target: 10, direction: lower, owner: ops}
  - {id: secret_latency, name: Secret latency, value: 900, target: 200, direction: lower, owner: ops, marking: cui}
edges:
  - {from: queue, to: secret_latency, weight: 0.1}
actions:
  - id: a_public
    name: Public action
    effects: [{kpi: queue, change: -0.5}]
  - id: a_cui
    name: Cui action
    effects: [{kpi: secret_latency, change: -0.5}]
  - id: a_marked
    name: Marked action
    marking: cui
    effects: [{kpi: queue, change: -0.2}]
`

// secrets are strings that must never reach a caller without the clearance.
var secrets = []string{"secret_latency", "Secret latency", "a_cui", "Cui action", "a_marked", "Marked action"}

func markedSetup(t *testing.T) *fixture {
	t.Helper()
	pol, err := policy.Parse("markings: {levels: [unclassified, cui]}")
	if err != nil {
		t.Fatal(err)
	}
	return setupWith(t, markedModel, func(o *Options) { o.Policy = pol })
}

// asClr performs a request as an identity holding a clearance.
func (f *fixture) asClr(t *testing.T, method, path, user string, roles []auth.Role, clearance, body string, out any) (int, string) {
	t.Helper()
	id, err := auth.Identity{Subject: user, Roles: roles, Method: "password"}.WithClearance(clearance)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := f.s.opt.Auth.Mint(time.Hour, id)
	req, _ := http.NewRequest(method, f.ts.URL+path, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: v})
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if out != nil && resp.StatusCode < 300 {
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s %s: %v: %s", method, path, err, b)
		}
	}
	return resp.StatusCode, string(b)
}

func TestMarkedKPIsAndActionsAreInvisibleBelowClearance(t *testing.T) {
	f := markedSetup(t)
	admin := []auth.Role{auth.RoleAdmin}

	// Clearance, not role: an admin without the clearance sees the public part only.
	_, low := f.asClr(t, "GET", "/api/v1/graph", "root", admin, "", "", nil)
	for _, s := range secrets {
		if strings.Contains(low, s) {
			t.Errorf("/graph leaked %q to a caller without clearance", s)
		}
	}
	if !strings.Contains(low, "queue") {
		t.Error("the public KPI should still be visible")
	}
	_, high := f.asClr(t, "GET", "/api/v1/graph", "root", admin, "cui", "", nil)
	for _, s := range []string{"secret_latency", "a_cui", "a_marked"} {
		if !strings.Contains(high, s) {
			t.Errorf("/graph hid %q from a cleared caller", s)
		}
	}

	// History of a hidden KPI looks like a missing one.
	if c, _ := f.asClr(t, "GET", "/api/v1/kpis/secret_latency/history", "v", viewer, "", "", nil); c != 404 {
		t.Errorf("hidden KPI history = %d, want 404", c)
	}
	if c, _ := f.asClr(t, "GET", "/api/v1/kpis/secret_latency/history", "v", viewer, "cui", "", nil); c != 200 {
		t.Errorf("cleared KPI history = %d, want 200", c)
	}
}

func TestPlansNeverNameHiddenActions(t *testing.T) {
	f := markedSetup(t)
	var low, high struct {
		Recommendations []struct {
			Action string `json:"action"`
		} `json:"recommendations"`
	}
	f.asClr(t, "GET", "/api/v1/plan", "v", viewer, "", "", &low)
	f.asClr(t, "GET", "/api/v1/plan", "v", viewer, "cui", "", &high)
	for _, r := range low.Recommendations {
		if r.Action != "a_public" {
			t.Errorf("low clearance was offered %q", r.Action)
		}
	}
	seen := map[string]bool{}
	for _, r := range high.Recommendations {
		seen[r.Action] = true
	}
	if !seen["a_cui"] || !seen["a_marked"] {
		t.Errorf("cleared caller should see every action, got %v", seen)
	}
}

func TestHiddenProposalsAreHiddenEverywhere(t *testing.T) {
	f := markedSetup(t)
	var made struct {
		ID string `json:"id"`
	}
	if c, b := f.asClr(t, "POST", "/api/v1/proposals", "p", []auth.Role{auth.RoleProposer}, "cui", `{"action":"a_marked"}`, &made); c >= 300 || made.ID == "" {
		t.Fatalf("cleared proposer could not propose: %d %s", c, b)
	}
	// A caller without clearance cannot even propose it.
	if c, _ := f.asClr(t, "POST", "/api/v1/proposals", "p", []auth.Role{auth.RoleProposer}, "", `{"action":"a_marked"}`, nil); c < 400 {
		t.Errorf("proposing a hidden action = %d, want an error", c)
	}

	id := made.ID
	for _, path := range []string{
		"/api/v1/proposals/" + id, "/api/v1/decisions/" + id, "/api/v1/decisions/" + id + "/export",
		"/api/v1/proposals/" + id + "/explanation", "/api/v1/proposals/" + id + "/similar",
	} {
		if c, _ := f.asClr(t, "GET", path, "v", viewer, "", "", nil); c != 404 {
			t.Errorf("%s = %d for an uncleared viewer, want 404", path, c)
		}
		// A cleared viewer finds it. (An explanation exists only after a verdict.)
		if c, _ := f.asClr(t, "GET", path, "v", viewer, "cui", "", nil); c == 404 {
			t.Errorf("%s = 404 for a cleared viewer", path)
		}
	}
	for _, path := range []string{"/api/v1/proposals", "/api/v1/decisions", "/api/v1/audit"} {
		_, body := f.asClr(t, "GET", path, "v", viewer, "", "", nil)
		if strings.Contains(body, id) || strings.Contains(body, "a_marked") {
			t.Errorf("%s listed a hidden proposal: %s", path, body)
		}
	}
	if c, _ := f.asClr(t, "POST", "/api/v1/proposals/"+id+"/approve", "boss", approver, "", "", nil); c != 404 {
		t.Errorf("approving a hidden proposal = %d, want 404", c)
	}
}

// routeRE finds the parameterless GET routes registered in api.go, so a route
// added later is swept too.
var routeRE = regexp.MustCompile(`"GET (/api/v1/[^"{]*)"`)

func TestNoGETRouteLeaksAMarkedNameBelowClearance(t *testing.T) {
	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatal(err)
	}
	routes := routeRE.FindAllStringSubmatch(string(src), -1)
	if len(routes) < 30 {
		t.Fatalf("found only %d routes; the pattern or api.go changed", len(routes))
	}
	f := markedSetup(t)
	for _, m := range routes {
		path := m[1]
		if path == "/api/v1/events" { // a stream; covered by its snapshot
			continue
		}
		for _, role := range [][]auth.Role{viewer, {auth.RoleAdmin}} {
			_, body := f.asClr(t, "GET", path, "u", role, "", "", nil)
			for _, s := range secrets {
				if strings.Contains(body, s) {
					t.Errorf("GET %s leaked %q to a caller without clearance (roles %v)", path, s, role)
				}
			}
		}
	}
}

func TestMarkingsOffChangesNothing(t *testing.T) {
	f := setupWith(t, markedModel, nil) // a marked model but no markings in the policy
	_, body := f.asClr(t, "GET", "/api/v1/graph", "v", viewer, "", "", nil)
	if !strings.Contains(body, "secret_latency") {
		t.Error("with no markings block everything must stay visible")
	}
}

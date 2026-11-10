package api

import (
	"encoding/json"
	"github.com/zyvorai/zyntra/internal/planning"
	"strings"
	"testing"
)

func TestPlanningRoutes(t *testing.T) {
	f := setup(t, nil)
	var result planning.Result
	if c := f.do(t, "POST", "/api/v1/planning/portfolios", "", "{}", &result); c != 401 {
		t.Fatalf("unauthenticated %d", c)
	}
	if c := f.do(t, "POST", "/api/v1/planning/portfolios", "k3y", `{"max_actions":3}`, &result); c != 200 || result.Evaluated != 7 {
		t.Fatalf("%d %+v", c, result)
	}
	if c := f.do(t, "POST", "/api/v1/planning/portfolios", "k3y", `{"unusable":{}}`, nil); c != 400 {
		t.Fatalf("caller freshness accepted %d", c)
	}
	if c := f.do(t, "POST", "/api/v1/planning/portfolios", "k3y", `{} {}`, nil); c != 400 {
		t.Fatalf("trailing JSON accepted %d", c)
	}
	var snap planning.Snapshot
	if c := f.do(t, "GET", "/api/v1/planning/snapshot", "k3y", "", &snap); c != 200 {
		t.Fatal(c)
	}
	b, _ := json.Marshal(map[string]any{"snapshot": snap, "options": map[string]any{"max_actions": 3}})
	if c := f.do(t, "POST", "/api/v1/planning/replay", "k3y", string(b), &result); c != 200 {
		t.Fatal(c)
	}
	snap.SHA256 = "bad"
	b, _ = json.Marshal(map[string]any{"snapshot": snap})
	if c := f.do(t, "POST", "/api/v1/planning/replay", "k3y", string(b), nil); c != 422 {
		t.Fatal(c)
	}
	if len(f.s.opt.Store.List()) != 0 || len(f.run.calls) != 0 {
		t.Fatal("planning executed or proposed an action")
	}
	for _, path := range []string{"snapshot", "portfolios", "replay", "review", "timeline", "process", "benefits", "quality"} {
		if tenantAllowed("POST", "/api/v1/planning/"+path) || tenantAllowed("GET", "/api/v1/planning/"+path) {
			t.Fatalf("tenant allowed %s", path)
		}
	}
}
func TestPlanningSnapshotHonorsMarkings(t *testing.T) {
	f := markedSetup(t)
	_, body := f.asClr(t, "GET", "/api/v1/planning/snapshot", "u", viewer, "", "", nil)
	for _, secret := range secrets {
		if strings.Contains(body, secret) {
			t.Fatalf("snapshot leaked %s", secret)
		}
	}
}

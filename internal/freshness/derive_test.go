// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package freshness

import (
	"testing"
	"time"

	"github.com/zyvorai/zyntra/internal/graph"
)

func TestDerivedKPIInheritsTheWorstInputState(t *testing.T) {
	m, err := graph.Parse([]byte(`
kpis:
  - {id: d, value: 0, derive: {op: product, of: [fresh, old]}}
  - {id: fresh, value: 1, source: {kind: file, file: f.csv, field: "#"}}
  - {id: old, value: 1, source: {kind: file, file: g.csv, field: "#"}}
  - {id: s, value: 1}
  - {id: onlystatic, value: 0, derive: {op: sum, of: [s]}}
  - {id: chain, value: 0, derive: {op: sum, of: [d, s]}}
  - {id: neverseen, value: 1, source: {kind: file, file: h.csv, field: "#"}}
  - {id: m, value: 0, derive: {op: sum, of: [fresh, neverseen]}}
`))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	tr := New(time.Minute)
	tr.now = func() time.Time { return now }
	tr.Success("fresh", now)
	tr.Success("old", now.Add(-time.Hour))

	got := Index(tr.States(m))
	if got["d"].Status != Stale {
		t.Errorf("d = %s, want stale because one input is", got["d"].Status)
	}
	if got["d"].Usable() {
		t.Error("a stale derived KPI must be out of planning")
	}
	if ls := got["d"].LastSuccess; ls == nil || !ls.Equal(now.Add(-time.Hour)) {
		t.Errorf("d last success = %v, want the oldest input's", ls)
	}
	if got["onlystatic"].Status != Static {
		t.Errorf("onlystatic = %s, want static", got["onlystatic"].Status)
	}
	if got["chain"].Status != Stale {
		t.Errorf("chain = %s: staleness must pass through a derived input", got["chain"].Status)
	}
	if got["m"].Status != Missing {
		t.Errorf("m = %s, want missing when an input never refreshed", got["m"].Status)
	}
}

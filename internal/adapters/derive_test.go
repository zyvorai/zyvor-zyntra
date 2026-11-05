// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package adapters

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zyvorai/zyntra/internal/graph"
)

// derivedModel has a file-backed input (a = 3 of 4 up = 0.75), a static input
// (b = 0.8) and a chain: d = a*b, e = d/b. The derived KPIs are listed first,
// so the test also checks that inputs are computed before what depends on them.
func derivedModel(t *testing.T, csv string) *graph.Model {
	t.Helper()
	dir := t.TempDir()
	if csv != "" {
		if err := os.WriteFile(filepath.Join(dir, "units.csv"), []byte(csv), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := graph.Parse([]byte(`
kpis:
  - {id: e, value: 0.123, derive: {op: ratio, of: [d, b]}}
  - {id: d, value: 0.123, derive: {op: product, of: [a, b]}}
  - {id: a, value: 0.1, source: {kind: file, file: units.csv, field: "#(state=up)", denominator: "#"}}
  - {id: b, value: 0.8}
`))
	if err != nil {
		t.Fatal(err)
	}
	m.Dir = dir
	return m
}

const units = "id,state\n1,up\n2,up\n3,down\n4,up\n"

func TestDerivedKPIsFollowTheirInputs(t *testing.T) {
	m := derivedModel(t, units)
	rep, err := Refresh(context.Background(), m, Config{Files: NewFileCache()})
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]float64{"a": 0.75, "d": 0.6, "e": 0.75} {
		k, _ := m.KPI(id)
		if math.Abs(k.Value-want) > 1e-12 {
			t.Errorf("%s = %v, want %v", id, k.Value, want)
		}
	}
	for _, id := range []string{"d", "e"} {
		if !rep.KPIs[id].OK {
			t.Errorf("%s should report an update: %+v", id, rep.KPIs[id])
		}
	}
}

func TestDerivedKPIKeepsItsValueWhenAnInputFails(t *testing.T) {
	m := derivedModel(t, "") // no units.csv: the input a cannot be read
	rep, err := Refresh(context.Background(), m, Config{Files: NewFileCache()})
	if err == nil {
		t.Fatal("the failed input should be reported")
	}
	for _, id := range []string{"d", "e"} {
		k, _ := m.KPI(id)
		if k.Value != 0.123 {
			t.Errorf("%s = %v: a derived value must not be recomputed from a failed input", id, k.Value)
		}
		u := rep.KPIs[id]
		if u.OK || !u.Fallback || u.Error == "" {
			t.Errorf("%s update = %+v, want a fallback with a reason", id, u)
		}
	}
	if !strings.Contains(rep.KPIs["d"].Error, "input a") {
		t.Errorf("the reason should name the input: %q", rep.KPIs["d"].Error)
	}
}

func TestDerivedRatioWithAZeroDenominatorIsAnError(t *testing.T) {
	m, err := graph.Parse([]byte(`
kpis:
  - {id: x, value: 5}
  - {id: y, value: 0}
  - {id: r, value: 7, derive: {op: ratio, of: [x, y]}}
`))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := Refresh(context.Background(), m, Config{})
	if err == nil || !strings.Contains(err.Error(), "zero") {
		t.Fatalf("err = %v, want a zero-denominator error", err)
	}
	if k, _ := m.KPI("r"); k.Value != 7 || rep.KPIs["r"].OK {
		t.Errorf("r = %v update = %+v: no number should be invented", k.Value, rep.KPIs["r"])
	}
}

func TestGenericCoversADerivedOnlyModel(t *testing.T) {
	m, err := graph.Parse([]byte("kpis:\n  - {id: x, value: 2}\n  - {id: y, value: 3, derive: {op: sum, of: [x]}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !Generic(m) {
		t.Error("a model with a derived KPI needs the refresh pass that computes it")
	}
}

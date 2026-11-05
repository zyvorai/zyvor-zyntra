// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package adapters

import (
	"math"
	"testing"
	"time"

	"github.com/zyvorai/zyntra/internal/graph"
)

func rows() any {
	b := []byte("id,status,age,min,at\n" +
		"a,open,9,10,2026-11-04T10:00:00Z\n" +
		"b,open,14,20,2026-10-20T10:00:00Z\n" +
		"c,closed,3,30,2026-11-05T09:00:00Z\n" +
		"d,Open,6,40,1762330000\n" + // epoch: 2025-11-05, a year old
		"e,hold,21,50,2026-11-01\n")
	d, err := parseCSV(b)
	if err != nil {
		panic(err)
	}
	return d
}

var now = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

func TestWhereOperators(t *testing.T) {
	for _, c := range []struct {
		name  string
		where map[string]string
		want  float64
	}{
		{"equality unchanged", map[string]string{"status": "open"}, 3},
		{"greater", map[string]string{"age": ">9"}, 2},
		{"greater or equal", map[string]string{"age": ">=9"}, 3},
		{"less", map[string]string{"age": "<6"}, 1},
		{"less or equal", map[string]string{"age": "<=6"}, 2},
		{"not equal", map[string]string{"status": "!=open"}, 2},
		{"in", map[string]string{"status": "in:open|hold"}, 4},
		{"two keys are ANDed", map[string]string{"status": "open", "age": ">=9"}, 2},
		{"explicit equality", map[string]string{"status": "=open"}, 3},
		{"non-numeric cell fails a comparison", map[string]string{"status": ">1"}, 0},
		{"bad limit matches nothing", map[string]string{"age": ">abc"}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExtractAt(rows(), graph.Source{Field: "#", Where: c.where}, now)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestPercentileAgg(t *testing.T) {
	for _, c := range []struct {
		agg  string
		want float64
	}{{"p50", 30}, {"p90", 50}, {"p99", 50}, {"p1", 10}} {
		got, err := ExtractAt(rows(), graph.Source{Field: "*.min", Agg: c.agg}, now)
		if err != nil || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s = %v, %v; want %v", c.agg, got, err, c.want)
		}
	}
	one := []any{map[string]any{"v": 7.0}}
	if got, err := ExtractAt(one, graph.Source{Field: "*.v", Agg: "p90"}, now); err != nil || got != 7 {
		t.Errorf("single row p90 = %v, %v", got, err)
	}
	if _, err := ExtractAt([]any{}, graph.Source{Field: "*.v", Agg: "p90"}, now); err == nil {
		t.Error("empty set should error, as avg does")
	}
	for _, bad := range []string{"p0", "p100", "p9x", "p090", "pp5"} {
		if _, err := ExtractAt(rows(), graph.Source{Field: "*.min", Agg: bad}, now); err == nil {
			t.Errorf("agg %q should be rejected", bad)
		}
	}
}

func TestTrailingWindow(t *testing.T) {
	src := func(w time.Duration) graph.Source {
		return graph.Source{Field: "#", TimeField: "at", Within: graph.Duration(w)}
	}
	// Window of 24h before 2026-11-05T12:00: a (11-04 10:00) and c are in;
	// b, the year-old epoch row d, and e (11-01 midnight) are out.
	got, err := ExtractAt(rows(), src(26*time.Hour), now)
	if err != nil || got != 2 {
		t.Errorf("26h = %v, %v; want 2", got, err)
	}
	// The boundary is inclusive: exactly 26h before now keeps a row stamped then.
	edge := []any{map[string]any{"at": "2026-11-04T10:00:00Z"}}
	if got, _ := ExtractAt(edge, src(26*time.Hour), now); got != 1 {
		t.Errorf("row exactly on the boundary = %v, want kept", got)
	}
	// Window runs before where.
	s := src(26 * time.Hour)
	s.Where = map[string]string{"status": "open"}
	if got, _ := ExtractAt(rows(), s, now); got != 1 {
		t.Errorf("window then where = %v, want 1", got)
	}
	// 30 days keeps a, b, c, e; the epoch row is far older.
	if got, _ := ExtractAt(rows(), src(30*24*time.Hour), now); got != 4 {
		t.Errorf("30d = %v, want 4", got)
	}
	bad := []any{map[string]any{"at": "yesterday"}}
	if _, err := ExtractAt(bad, src(time.Hour), now); err == nil {
		t.Error("an unparseable timestamp must be an error, not a silent drop")
	}
	if _, err := ExtractAt([]any{map[string]any{"x": 1.0}}, src(time.Hour), now); err == nil {
		t.Error("a row without the time field must be an error")
	}
	// No timeField: unchanged.
	if got, _ := ExtractAt(rows(), graph.Source{Field: "#"}, now); got != 5 {
		t.Errorf("no window = %v, want 5", got)
	}
}

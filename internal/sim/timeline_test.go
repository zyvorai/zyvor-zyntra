package sim

import (
	"context"
	"github.com/zyvorai/zyntra/internal/graph"
	"testing"
	"time"
)

func TestTimelineDelayedPropagationDoesNotCompound(t *testing.T) {
	m, err := graph.Parse([]byte(`
kpis:
 - {id: a, value: 100}
 - {id: b, value: 100}
edges:
 - {from: a, to: b, weight: 1, delay: 2h}
actions:
 - {id: change, effects: [{kpi: a, change: -0.5, delay: 1h}]}
`))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Timeline(context.Background(), m, m.Actions, 4*time.Hour, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{100, 100, 100, 50, 50} {
		if r.Points[i].Result.KPIs[1].After != want {
			t.Fatalf("time %d: %+v", i, r.Points[i])
		}
	}
	if r.Points[4].Result.KPIs[0].After != 50 || m.KPIs[0].Value != 100 {
		t.Fatal("compounded effect or mutated baseline")
	}
	if _, err = Timeline(context.Background(), m, m.Actions, time.Hour, 0, nil); err == nil {
		t.Fatal("accepted zero step")
	}
}

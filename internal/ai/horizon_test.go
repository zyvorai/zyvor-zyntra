package ai

import (
	"math"
	"testing"
)

func TestHorizonSplitAndWarming(t *testing.T) {
	v := make([]float64, 600)
	for i := range v {
		v[i] = float64(i)
	}
	for _, h := range []int{1, 6, 24, 168} {
		b := horizonBacktest(v, h)
		if b.Status != "ready" || b.Method != "linear" || b.MAE > 1e-8 || b.Coverage != 1 {
			t.Fatalf("h=%d %+v", h, b)
		}
	}
	if b := horizonBacktest(v[:100], 168); b.Status != "warming" {
		t.Fatal(b)
	}
}
func TestHorizonCalibrationNeverReadsEvaluationTargets(t *testing.T) {
	v := make([]float64, 300)
	for i := range v {
		v[i] = 100 + float64(i)
	}
	before := horizonBacktest(v, 24)
	// Modify all evaluation targets. Method and radius must remain unchanged.
	for i := len(v) - 24; i < len(v); i++ {
		v[i] += 1000
	}
	after := horizonBacktest(v, 24)
	if before.Method != after.Method || math.Abs(before.ErrorRadius-after.ErrorRadius) > 1e-9 || after.MAE < 500 || after.Coverage != 0 {
		t.Fatalf("before=%+v after=%+v", before, after)
	}
}
func TestWarmingHorizonDoesNotBorrowOneHourBand(t *testing.T) {
	m, h := analyticalFixture(t, 100, func(i int) float64 { return 100 + float64(i) })
	row := Analytics(m, h, nil).KPIs[2]
	if len(row.Projections) != 4 || row.Projections[3].Low != nil || row.Projections[3].High != nil || row.Projections[3].IntervalStatus != "warming" {
		t.Fatalf("%+v", row)
	}
}

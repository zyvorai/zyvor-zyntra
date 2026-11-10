package semantic

import (
	"testing"
	"time"
)

func TestCheckBatchQuality(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	c := Contract{Name: "source", ObjectType: "Order", Source: Source{Kind: "csv", Reference: "fixture"}, Schema: []Field{{Name: "id", Type: "string", Required: true}, {Name: "at", Type: "timestamp", Required: true}}, Quality: Quality{Freshness: "1h", Unique: []string{"id"}, Completeness: 1}, KPIs: []string{"orders"}}
	rows := []map[string]any{{"id": "one", "at": now.Format(time.RFC3339)}, {"id": "one", "at": now.Add(-2 * time.Hour).Format(time.RFC3339), "extra": true}}
	r, err := CheckBatch(c, rows, "at", now)
	if err != nil || r.Valid || len(r.Issues) != 3 || len(r.AffectedKPIs) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = CheckBatch(c, rows[:1], "at", now)
	if err != nil || !r.Valid {
		t.Fatalf("%+v %v", r, err)
	}
}

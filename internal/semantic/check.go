// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package semantic

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type BatchCheck struct {
	Valid        bool         `json:"valid"`
	Rows         int          `json:"rows"`
	Completeness float64      `json:"completeness"`
	Issues       []BatchIssue `json:"issues"`
	AffectedKPIs []string     `json:"affected_kpis"`
	Note         string       `json:"note"`
}
type BatchIssue struct {
	Row    int    `json:"row"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// CheckBatch evaluates supplied records without ingesting them. Issues identify
// rows/fields, never copy potentially sensitive cell values into diagnostics.
func CheckBatch(c Contract, rows []map[string]any, observedField string, now time.Time) (BatchCheck, error) {
	out := BatchCheck{Valid: true, Rows: len(rows), Issues: []BatchIssue{}, AffectedKPIs: []string{}, Note: "Validation preview only. No records are ingested or quarantined and live freshness is unchanged."}
	if err := ValidateContract(c); err != nil {
		return out, err
	}
	if len(rows) == 0 || len(rows) > 10000 {
		return out, fmt.Errorf("provide 1..10000 rows")
	}
	if math.IsNaN(c.Quality.Completeness) || math.IsInf(c.Quality.Completeness, 0) {
		return out, fmt.Errorf("completeness must be finite")
	}
	schema := map[string]Field{}
	for _, f := range c.Schema {
		schema[f.Name] = f
	}
	maxAge := time.Duration(0)
	if c.Quality.Freshness != "" {
		var err error
		maxAge, err = time.ParseDuration(c.Quality.Freshness)
		if err != nil {
			return out, err
		}
		f, ok := schema[observedField]
		if !ok || strings.ToLower(f.Type) != "timestamp" {
			return out, fmt.Errorf("freshness requires an observed_field of type timestamp")
		}
	}
	add := func(i int, field, reason string) {
		out.Valid = false
		out.Issues = append(out.Issues, BatchIssue{i, field, reason})
	}
	unique := map[string]bool{}
	present, total := 0, 0
	for i, row := range rows {
		for _, f := range c.Schema {
			value, ok := row[f.Name]
			missing := !ok || value == nil || value == ""
			if f.Required {
				total++
				if !missing {
					present++
				} else {
					add(i, f.Name, "required value missing")
				}
			}
			if missing {
				continue
			}
			valid := true
			switch strings.ToLower(f.Type) {
			case "string", "enum":
				_, valid = value.(string)
			case "number", "integer":
				n, ok := value.(float64)
				valid = ok && !math.IsNaN(n) && !math.IsInf(n, 0)
				if strings.ToLower(f.Type) == "integer" {
					valid = valid && n == math.Trunc(n)
				}
			case "boolean":
				_, valid = value.(bool)
			case "object":
				_, valid = value.(map[string]any)
			case "duration":
				v, ok := value.(string)
				_, err := time.ParseDuration(v)
				valid = ok && err == nil
			case "timestamp":
				v, ok := value.(string)
				_, err := time.Parse(time.RFC3339Nano, v)
				valid = ok && err == nil
			}
			if !valid {
				add(i, f.Name, "type mismatch")
			}
		}
		// Unknown fields indicate a source schema change, rather than silently disappearing.
		for name := range row {
			if _, ok := schema[name]; !ok {
				add(i, name, "undeclared field")
			}
		}
		if len(c.Quality.Unique) > 0 {
			vals := []any{}
			complete := true
			for _, name := range c.Quality.Unique {
				v, ok := row[name]
				if !ok || v == nil || v == "" {
					complete = false
					add(i, name, "unique key missing")
				}
				vals = append(vals, v)
			}
			if complete {
				b, err := json.Marshal(vals)
				if err != nil {
					return out, err
				}
				key := string(b)
				if unique[key] {
					add(i, "", "duplicate composite key")
				}
				unique[key] = true
			}
		}
		if maxAge > 0 {
			value, ok := row[observedField].(string)
			at, err := time.Parse(time.RFC3339Nano, value)
			if !ok || err != nil {
				add(i, observedField, "missing or invalid observation time")
			} else if at.After(now.Add(time.Minute)) {
				add(i, observedField, "observation is in the future")
			} else if now.Sub(at) > maxAge {
				add(i, observedField, "observation is stale")
			}
		}
	}
	out.Completeness = 1
	if total > 0 {
		out.Completeness = float64(present) / float64(total)
	}
	if out.Completeness < c.Quality.Completeness {
		add(-1, "", "batch completeness below contract")
	}
	if !out.Valid {
		out.AffectedKPIs = append(out.AffectedKPIs, c.KPIs...)
	}
	sort.Slice(out.Issues, func(i, j int) bool {
		a, b := out.Issues[i], out.Issues[j]
		if a.Row != b.Row {
			return a.Row < b.Row
		}
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		return a.Reason < b.Reason
	})
	return out, nil
}

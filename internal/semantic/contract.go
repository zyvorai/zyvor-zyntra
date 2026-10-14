// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package semantic

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

var sourceKinds = map[string]bool{
	"csv": true, "file": true, "prometheus": true, "sql": true,
	"rest": true, "http": true, "kubernetes": true, "webhook": true,
	"netra": true, "gryvia": true,
}

var fieldTypes = map[string]bool{
	"string": true, "integer": true, "number": true, "boolean": true,
	"timestamp": true, "duration": true, "enum": true, "object": true,
}

// ValidateContract rejects ambiguous or unsafe contract definitions.
func ValidateContract(c Contract) error {
	var errs []error
	if strings.TrimSpace(c.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if strings.TrimSpace(c.ObjectType) == "" {
		errs = append(errs, errors.New("object_type is required"))
	}
	if !sourceKinds[strings.ToLower(c.Source.Kind)] {
		errs = append(errs, fmt.Errorf("unsupported source kind %q", c.Source.Kind))
	}
	if strings.TrimSpace(c.Source.Reference) == "" {
		errs = append(errs, errors.New("source.reference is required"))
	}
	if len(c.Schema) == 0 {
		errs = append(errs, errors.New("schema must contain at least one field"))
	}

	seen := map[string]bool{}
	for i, f := range c.Schema {
		if strings.TrimSpace(f.Name) == "" {
			errs = append(errs, fmt.Errorf("schema[%d].name is required", i))
			continue
		}
		if seen[f.Name] {
			errs = append(errs, fmt.Errorf("schema field %q is duplicated", f.Name))
		}
		seen[f.Name] = true
		if !fieldTypes[strings.ToLower(f.Type)] {
			errs = append(errs, fmt.Errorf("schema field %q has unsupported type %q", f.Name, f.Type))
		}
	}
	if c.Quality.Freshness != "" {
		d, err := time.ParseDuration(c.Quality.Freshness)
		if err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("quality.freshness %q must be a positive duration", c.Quality.Freshness))
		}
	}
	if c.Quality.Completeness < 0 || c.Quality.Completeness > 1 {
		errs = append(errs, errors.New("quality.completeness must be between 0 and 1"))
	}
	for _, key := range c.Quality.Unique {
		if !seen[key] {
			errs = append(errs, fmt.Errorf("quality.unique names unknown field %q", key))
		}
	}
	for _, kpi := range c.KPIs {
		if strings.TrimSpace(kpi) == "" {
			errs = append(errs, errors.New("kpis may not contain an empty id"))
		}
	}
	return errors.Join(errs...)
}

// Validate validates all contracts and enforces globally unique names.
func Validate(cs []Contract) error {
	var errs []error
	seen := map[string]bool{}
	for i, c := range cs {
		if seen[c.Name] && c.Name != "" {
			errs = append(errs, fmt.Errorf("contract %q is duplicated", c.Name))
		}
		seen[c.Name] = true
		if err := ValidateContract(c); err != nil {
			errs = append(errs, fmt.Errorf("contract[%d] %q: %w", i, c.Name, err))
		}
	}
	return errors.Join(errs...)
}

// LoadFile reads a JSON contract array. Credentials must remain in the
// existing connector/environment configuration, never in this file.
func LoadFile(path string) ([]Contract, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cs []Contract
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cs); err != nil {
		return nil, err
	}
	if err := Validate(cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// Catalog produces a stable, permission-safe catalog. Secrets are not part of
// the Contract type, so they cannot accidentally be returned.
func Catalog(cs []Contract) []CatalogEntry {
	out := make([]CatalogEntry, 0, len(cs))
	for _, c := range cs {
		out = append(out, CatalogEntry{
			Name: c.Name, Owner: c.Owner, SourceKind: c.Source.Kind,
			Reference: c.Source.Reference, ObjectType: c.ObjectType,
			Fields: append([]Field(nil), c.Schema...), Quality: c.Quality,
			KPIs: append([]string(nil), c.KPIs...), Labels: cloneLabels(c.Labels),
		})
	}
	return out
}

func cloneLabels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

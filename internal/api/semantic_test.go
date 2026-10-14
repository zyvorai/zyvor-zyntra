// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package api

import "testing"

func TestSemanticEndpoints(t *testing.T) {
	f := setup(t, nil)

	var cat struct {
		Configured bool  `json:"configured"`
		Contracts  []any `json:"contracts"`
	}
	if c := f.do(t, "GET", "/api/v1/semantic/catalog", "k3y", "", &cat); c != 200 || len(cat.Contracts) != 0 {
		t.Fatalf("catalog status %d %+v", c, cat)
	}

	var res struct {
		Resources []struct {
			URI string `json:"uri"`
		} `json:"resources"`
	}
	if c := f.do(t, "GET", "/api/v1/semantic/resources", "k3y", "", &res); c != 200 || len(res.Resources) == 0 {
		t.Fatalf("resources status %d %+v", c, res)
	}

	var v struct {
		Valid bool `json:"valid"`
	}
	good := `[{"name":"orders","source":{"kind":"sql","reference":"orders"},"object_type":"Order","schema":[{"name":"id","type":"string"}]}]`
	if c := f.do(t, "POST", "/api/v1/semantic/contracts/validate", "k3y", good, &v); c != 200 || !v.Valid {
		t.Fatalf("validate good: %d %+v", c, v)
	}
	if c := f.do(t, "POST", "/api/v1/semantic/contracts/validate", "k3y", `[{"name":"x","password":"p"}]`, nil); c != 400 {
		t.Fatalf("unknown field status %d", c)
	}
	if c := f.do(t, "POST", "/api/v1/semantic/contracts/validate", "k3y", `[{"name":""}]`, nil); c != 422 {
		t.Fatalf("invalid status %d", c)
	}

	var conf struct {
		Percent int    `json:"percent"`
		Band    string `json:"band"`
	}
	body := `{"freshness":1,"data_quality":1,"ontology_confidence":1,"calibration":1,"edge_confidence":1,"prediction_accuracy":1}`
	if c := f.do(t, "POST", "/api/v1/semantic/confidence", "k3y", body, &conf); c != 200 || conf.Percent != 100 || conf.Band != "high" {
		t.Fatalf("confidence %d %+v", c, conf)
	}
	if c := f.do(t, "POST", "/api/v1/semantic/confidence", "k3y", `{"freshness":2}`, nil); c != 422 {
		t.Fatalf("out of range status %d", c)
	}
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package graph

import (
	"testing"
	"time"
)

func TestWindowValidationAndDays(t *testing.T) {
	var d Duration
	if err := d.parse("30d"); err != nil || d.D() != 30*24*time.Hour {
		t.Errorf("30d = %v, %v", d.D(), err)
	}
	for _, bad := range []string{"xd", "-1d", "9999d"} {
		if err := d.parse(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	for _, c := range []struct {
		name string
		src  Source
		ok   bool
	}{
		{"both", Source{Kind: SourceFile, File: "f.csv", TimeField: "at", Within: Duration(time.Hour)}, true},
		{"neither", Source{Kind: SourceFile, File: "f.csv"}, true},
		{"field only", Source{Kind: SourceFile, File: "f.csv", TimeField: "at"}, false},
		{"within only", Source{Kind: SourceFile, File: "f.csv", Within: Duration(time.Hour)}, false},
		{"webhook-in", Source{Kind: SourceWebhookIn, Name: "x", TimeField: "at", Within: Duration(time.Hour)}, false},
	} {
		s := c.src
		err := validSource(&KPI{ID: "k", Source: &s})
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}

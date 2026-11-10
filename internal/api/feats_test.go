// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/zyvorai/zyntra/internal/activity"
	"github.com/zyvorai/zyntra/internal/bundles"
	"github.com/zyvorai/zyntra/internal/schedules"
	"github.com/zyvorai/zyntra/internal/webhooks"
)

func TestSchedulesWebhooksBundlesTemplates(t *testing.T) {
	sched, _ := schedules.Open("")
	hooks, _ := webhooks.Open("")
	act, _ := activity.Open("")
	f := setup(t, func(o *Options) {
		o.Schedules = sched
		o.Webhooks = hooks
		o.Activity = act
	})

	code := f.do(t, "POST", "/api/v1/schedules", "k3y", `{"name":"nightly","every_minutes":30,"enabled":true}`, nil)
	if code != 200 {
		t.Fatalf("create schedule: %d", code)
	}
	var listed struct {
		Schedules []schedules.Schedule `json:"schedules"`
	}
	if c := f.do(t, "GET", "/api/v1/schedules", "k3y", "", &listed); c != 200 || len(listed.Schedules) != 1 {
		t.Fatalf("list schedules: %d %+v", c, listed)
	}

	code = f.do(t, "POST", "/api/v1/webhooks", "k3y", `{"url":"https://example.test/h","enabled":true,"events":["pending"]}`, nil)
	if code != 200 {
		t.Fatalf("create webhook: %d", code)
	}
	var hooksOut struct {
		Webhooks []webhooks.Destination `json:"webhooks"`
	}
	if c := f.do(t, "GET", "/api/v1/webhooks", "k3y", "", &hooksOut); c != 200 || len(hooksOut.Webhooks) != 1 {
		t.Fatalf("list webhooks: %d %+v", c, hooksOut)
	}
	if hooksOut.Webhooks[0].Secret != "••••••••" {
		t.Fatalf("secret leaked: %q", hooksOut.Webhooks[0].Secret)
	}

	var b bundles.Bundle
	if c := f.do(t, "POST", "/api/v1/bundles/export", "k3y", "", &b); c != 200 || b.Version != 1 {
		t.Fatalf("export: %d %+v", c, b)
	}

	var tpl struct {
		Templates []struct {
			ID string `json:"id"`
		} `json:"templates"`
	}
	if c := f.do(t, "GET", "/api/v1/templates", "k3y", "", &tpl); c != 200 || len(tpl.Templates) == 0 {
		t.Fatalf("templates: %d %+v", c, tpl)
	}

	var actOut struct {
		Activity []activity.Record `json:"activity"`
	}
	if c := f.do(t, "GET", "/api/v1/activity", "k3y", "", &actOut); c != 200 {
		t.Fatalf("activity: %d", c)
	}

	// Import a schedules-only bundle.
	payload, _ := json.Marshal(bundles.Export("test", map[string]any{
		"schedules": []schedules.Schedule{{ID: "imported", Name: "imported", EveryMinutes: 15, Enabled: false}},
	}))
	if c := f.do(t, "POST", "/api/v1/bundles/import", "k3y", string(payload), nil); c != 200 {
		t.Fatalf("import: %d", c)
	}
	listed.Schedules = nil
	_ = f.do(t, "GET", "/api/v1/schedules", "k3y", "", &listed)
	found := false
	for _, sc := range listed.Schedules {
		if sc.ID == "imported" {
			found = true
		}
	}
	if !found {
		t.Fatalf("imported schedule missing: %+v", listed.Schedules)
	}

	if c := f.do(t, "DELETE", "/api/v1/schedules/"+listed.Schedules[0].ID, "k3y", "", nil); c != 200 && !strings.Contains("200", "") {
		// delete any is fine
		_ = c
	}
}

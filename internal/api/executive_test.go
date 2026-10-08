// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package api

import (
	"testing"
)

func TestExecutiveInbox(t *testing.T) {
	f := setup(t, nil)
	var inbox struct {
		Audience string `json:"audience"`
		Items    []struct {
			Question string `json:"question"`
		} `json:"items"`
	}
	if c := f.do(t, "GET", "/api/v1/executive/inbox?audience=ceo", "k3y", "", &inbox); c != 200 {
		t.Fatalf("status %d", c)
	}
	if inbox.Audience != "ceo" || len(inbox.Items) == 0 || inbox.Items[0].Question == "" {
		t.Fatalf("inbox = %+v", inbox)
	}
	var brief struct {
		Question string `json:"question"`
		Options  []struct {
			ID string `json:"id"`
		} `json:"options"`
		NextStep struct {
			Summary string `json:"summary"`
		} `json:"next_step"`
	}
	if c := f.do(t, "GET", "/api/v1/executive/brief?decision=weekly-review", "k3y", "", &brief); c != 200 || brief.NextStep.Summary == "" {
		t.Fatalf("brief status/body %+v", brief)
	}
	var digest struct {
		Headline string   `json:"headline"`
		Lines    []string `json:"lines"`
	}
	if c := f.do(t, "GET", "/api/v1/executive/digest", "k3y", "", &digest); c != 200 || digest.Headline == "" {
		t.Fatalf("digest status %d %+v", c, digest)
	}
}

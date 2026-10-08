// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package executive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/zyntra/internal/approvals"
	"github.com/zyvorai/zyntra/internal/freshness"
	"github.com/zyvorai/zyntra/internal/gaps"
	"github.com/zyvorai/zyntra/internal/outcome"
	"github.com/zyvorai/zyntra/internal/pack"
	"github.com/zyvorai/zyntra/internal/planner"
)

func leadership(t *testing.T) Input {
	t.Helper()
	dir := filepath.Join("..", "..", "packs", "leadership")
	m, err := pack.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "executive.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := ParseOverlay(b)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planner.PlanWith(m, planner.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return Input{
		Model: m, Plan: plan, Gaps: gaps.Detect(m), Overlay: overlay,
		Now: time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC), Audience: AudienceCEO,
	}
}

func TestLeadershipBriefShowsCapacityConstraint(t *testing.T) {
	in := leadership(t)
	inbox := BuildInbox(in)
	if len(inbox.Items) < 2 {
		t.Fatalf("inbox = %+v", inbox.Items)
	}
	if inbox.Items[0].Owner == "" || inbox.Items[0].Question == "" {
		t.Fatalf("inbox item missing question: %+v", inbox.Items[0])
	}
	brief := BuildBrief(in, "spend-or-hire")
	if !strings.Contains(brief.Question, "20 lakh") {
		t.Fatalf("question = %q", brief.Question)
	}
	var spend, hire, nothing Option
	for _, o := range brief.Options {
		switch o.ID {
		case "spend_20l_marketing":
			spend = o
		case "hire_two_engineers":
			hire = o
		case "do_nothing":
			nothing = o
		}
	}
	if spend.ID == "" || hire.ID == "" || nothing.ID == "" {
		t.Fatalf("missing options: %+v", brief.Options)
	}
	if len(spend.Blocked) == 0 {
		t.Fatalf("marketing spend should be blocked by onboarding capacity, got %+v", spend)
	}
	if strings.Join(spend.Blocked, " ") == "" || !strings.Contains(strings.ToLower(strings.Join(spend.Blocked, " ")), "onboarding") && !strings.Contains(strings.ToLower(strings.Join(spend.Blocked, " ")), "capacity") {
		t.Fatalf("block reason should name the constraint: %v", spend.Blocked)
	}
	if brief.NextStep.Summary == "" || brief.Narrative == "" {
		t.Fatalf("brief has no next step: %+v", brief.NextStep)
	}
	conf := Conflicts(in)
	if len(conf) == 0 {
		t.Fatal("expected a capacity conflict")
	}
}

func TestAssumptionFlipAndStaleGate(t *testing.T) {
	in := leadership(t)
	rows := Probe(in, "hire_two_engineers")
	if len(rows) != 3 {
		t.Fatalf("sensitivity rows = %d", len(rows))
	}
	in.Fresh = []freshness.State{{KPI: "qualified_pipeline", Status: freshness.Stale}}
	brief := BuildBrief(in, "spend-or-hire")
	if brief.NextStep.Kind != "missing-evidence" || brief.NextStep.Ready {
		t.Fatalf("stale input should close approval, got %+v", brief.NextStep)
	}
}

func TestReviewsAtWindows(t *testing.T) {
	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	executed := now.AddDate(0, 0, -61)
	reviews := Reviews([]approvals.Proposal{{
		ID: "p1", Action: "hire_two_engineers", ActionName: "Hire two engineers",
		CreatedBy: "ceo", ExecutedAt: &executed, Phase: approvals.PhaseVerified,
		Outcome: &outcome.Record{Accuracy: []outcome.Accuracy{{KPI: "delivery_days", Hit: false}, {KPI: "onboarding_capacity", Hit: true}}},
	}}, now)
	if len(reviews) != 1 || reviews[0].Window != "60" || !reviews[0].Due {
		t.Fatalf("review = %+v", reviews)
	}
	if reviews[0].HitRate != 0.5 {
		t.Fatalf("hit rate = %v", reviews[0].HitRate)
	}
	if !strings.Contains(reviews[0].FollowUp, "missed") {
		t.Fatalf("follow-up = %q", reviews[0].FollowUp)
	}
}

func TestDigestIsGrounded(t *testing.T) {
	in := leadership(t)
	d := BuildDigest(in)
	if d.Headline == "" || len(d.Lines) == 0 {
		t.Fatalf("digest = %+v", d)
	}
	if strings.Contains(strings.ToLower(d.Headline), "i recommend") {
		t.Fatalf("digest must not pick: %s", d.Headline)
	}
}

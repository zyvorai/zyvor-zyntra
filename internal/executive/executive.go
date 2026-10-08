// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package executive builds a leadership view over a Zyntra decision loop.
// It does not rank, pick, or run an action. Every number comes from a KPI,
// an edge, an effect, or a recorded outcome. A model may narrate the brief
// later; this package will not.
package executive

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/zyvorai/zyntra/internal/approvals"
	"github.com/zyvorai/zyntra/internal/freshness"
	"github.com/zyvorai/zyntra/internal/gaps"
	"github.com/zyvorai/zyntra/internal/graph"
	"github.com/zyvorai/zyntra/internal/planner"
	"github.com/zyvorai/zyntra/internal/sim"
)

const (
	AudienceAll       = "leadership"
	AudienceCEO       = "ceo"
	AudienceCTO       = "cto"
	AudienceMarketing = "marketing"
)

// Overlay names the decisions a pack wants a leadership review to open.
// It is optional. Without it the inbox is built from gaps and the plan.
type Overlay struct {
	Decisions   []DecisionSpec `yaml:"decisions" json:"decisions"`
	Assumptions []Assumption   `yaml:"assumptions" json:"assumptions"`
}

// DecisionSpec is a named question. Options are action ids, including
// postpone and do-nothing when the pack defines them.
type DecisionSpec struct {
	ID         string   `yaml:"id" json:"id"`
	Question   string   `yaml:"question" json:"question"`
	Audience   []string `yaml:"audience" json:"audience"`
	Owner      string   `yaml:"owner" json:"owner"`
	Due        string   `yaml:"due,omitempty" json:"due,omitempty"`
	Options    []string `yaml:"options" json:"options"`
	Experiment string   `yaml:"experiment,omitempty" json:"experiment,omitempty"`
	Why        string   `yaml:"why,omitempty" json:"why,omitempty"`
}

// Assumption is a named edge a reviewer can turn off. Scale is the weight
// multiplier used when the assumption is disabled (0 removes the edge).
type Assumption struct {
	ID        string  `yaml:"id" json:"id"`
	Statement string  `yaml:"statement" json:"statement"`
	From      string  `yaml:"from" json:"from"`
	To        string  `yaml:"to" json:"to"`
	Scale     float64 `yaml:"scale" json:"scale"`
}

// ParseOverlay reads an executive.yaml. An empty file is a valid overlay.
func ParseOverlay(b []byte) (Overlay, error) {
	var o Overlay
	if len(strings.TrimSpace(string(b))) == 0 {
		return o, nil
	}
	if err := yaml.Unmarshal(b, &o); err != nil {
		return Overlay{}, fmt.Errorf("parse executive overlay: %w", err)
	}
	return o, nil
}

// Inbox is the weekly list of decisions that need a person.
type Inbox struct {
	Audience string      `json:"audience"`
	AsOf     time.Time   `json:"as_of"`
	Items    []InboxItem `json:"items"`
	Blocked  int         `json:"blocked"`
	Stale    int         `json:"stale_inputs"`
}

// InboxItem is one decision, not a dashboard tile.
type InboxItem struct {
	ID         string   `json:"id"`
	Question   string   `json:"question"`
	Owner      string   `json:"owner"`
	Audience   []string `json:"audience"`
	Due        string   `json:"due,omitempty"`
	Rank       int      `json:"rank"`
	Severity   float64  `json:"severity"`
	Why        string   `json:"why"`
	Evidence   []string `json:"evidence"`
	Stale      []string `json:"stale,omitempty"`
	Blocked    bool     `json:"blocked,omitempty"`
	NextStep   string   `json:"next_step"`
}

// Brief is what an executive opens. Options include postpone and do nothing
// when those actions exist.
type Brief struct {
	ID          string       `json:"id"`
	Question    string       `json:"question"`
	Owner       string       `json:"owner"`
	Due         string       `json:"due,omitempty"`
	Audience    []string     `json:"audience"`
	Why         string       `json:"why,omitempty"`
	Evidence    []Evidence   `json:"evidence"`
	Options     []Option     `json:"options"`
	Uncertainty []string     `json:"uncertainty"`
	Conflicts   []Conflict   `json:"conflicts,omitempty"`
	NextStep    NextStep     `json:"next_step"`
	Narrative   string       `json:"narrative"`
}

// Evidence is a KPI or a stated constraint, with source and freshness.
type Evidence struct {
	KPI      string  `json:"kpi"`
	Name     string  `json:"name"`
	Owner    string  `json:"owner,omitempty"`
	Value    float64 `json:"value"`
	Target   float64 `json:"target,omitempty"`
	Unit     string  `json:"unit,omitempty"`
	Severity float64 `json:"severity"`
	Fresh    string  `json:"fresh"`
	Source   string  `json:"source"`
	Note     string  `json:"note,omitempty"`
}

// Option is one candidate, including do nothing. Rank comes from the planner.
// Kind is action, experiment, postpone, or nothing.
type Option struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Rank        int      `json:"rank,omitempty"`
	Status      string   `json:"status"`
	Risk        string   `json:"risk,omitempty"`
	Improvement float64  `json:"improvement"`
	Pessimistic float64  `json:"pessimistic"`
	Blocked     []string `json:"blocked,omitempty"`
	Moves       []string `json:"moves,omitempty"`
	Note        string   `json:"note,omitempty"`
}

// NextStep is an approval, an experiment, or a request for missing evidence.
type NextStep struct {
	Kind    string `json:"kind"`
	Action  string `json:"action,omitempty"`
	Owner   string `json:"owner,omitempty"`
	Summary string `json:"summary"`
	Ready   bool   `json:"ready"`
}

// Conflict is two plans that push the same KPI in opposite directions, or a
// demand action whose downstream capacity KPI is already off target.
type Conflict struct {
	KPI      string   `json:"kpi"`
	Name     string   `json:"name"`
	Actions  []string `json:"actions"`
	Owners   []string `json:"owners"`
	Kind     string   `json:"kind"`
	Summary  string   `json:"summary"`
}

// Sensitivity is what happens to one action when an assumption is turned off.
type Sensitivity struct {
	Assumption  string  `json:"assumption"`
	Statement   string  `json:"statement"`
	Before      float64 `json:"improvement_before"`
	After       float64 `json:"improvement_after"`
	Flips       bool    `json:"flips"`
	Was         string  `json:"top_before,omitempty"`
	Now         string  `json:"top_after,omitempty"`
}

// Review is a 30, 60, or 90 day look at a recorded decision.
type Review struct {
	ProposalID string  `json:"proposal_id"`
	Action     string  `json:"action"`
	Name       string  `json:"name"`
	Owner      string  `json:"owner,omitempty"`
	AgeDays    int     `json:"age_days"`
	Window     string  `json:"window"`
	Due        bool    `json:"due"`
	Verdict    string  `json:"verdict"`
	HitRate    float64 `json:"hit_rate"`
	FollowUp   string  `json:"follow_up"`
}

// Digest is a grounded weekly note. It quotes the inbox; it does not invent.
type Digest struct {
	Audience string   `json:"audience"`
	Headline string   `json:"headline"`
	Lines    []string `json:"lines"`
}

// Input is everything the brief is allowed to see.
type Input struct {
	Model    *graph.Model
	Plan     planner.Result
	Gaps     []gaps.Gap
	Fresh    []freshness.State
	Overlay  Overlay
	Proposals []approvals.Proposal
	Now      time.Time
	Audience string
}

// BuildInbox returns the decisions for an audience, highest severity first.
func BuildInbox(in Input) Inbox {
	if in.Now.IsZero() {
		in.Now = time.Now()
	}
	aud := normAudience(in.Audience)
	out := Inbox{Audience: aud, AsOf: in.Now}
	stale := staleSet(in.Fresh)
	out.Stale = len(stale)
	items := decisionsFor(in, aud, stale)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Blocked != items[j].Blocked {
			return !items[i].Blocked
		}
		return items[i].Severity > items[j].Severity
	})
	for i := range items {
		items[i].Rank = i + 1
		if items[i].Blocked {
			out.Blocked++
		}
	}
	out.Items = items
	return out
}

// BuildBrief opens one decision. An unknown id still returns a brief over
// the top of the inbox so the console has something to render.
func BuildBrief(in Input, id string) Brief {
	inbox := BuildInbox(in)
	spec, ok := findSpec(in.Overlay, id)
	if !ok && id == "" && len(inbox.Items) > 0 {
		id = inbox.Items[0].ID
		spec, ok = findSpec(in.Overlay, id)
	}
	if !ok {
		spec = DecisionSpec{ID: id, Question: questionFrom(in), Owner: ownerFrom(in), Options: actionIDs(in.Model)}
	}
	if spec.ID == "" {
		spec.ID = "weekly-review"
	}
	b := Brief{
		ID: spec.ID, Question: spec.Question, Owner: spec.Owner, Due: spec.Due,
		Audience: spec.Audience, Why: spec.Why,
		Evidence: evidence(in),
		Options:  options(in, spec),
		Conflicts: Conflicts(in),
	}
	if len(b.Audience) == 0 {
		b.Audience = []string{AudienceAll}
	}
	b.Uncertainty = uncertainty(in, b.Options)
	b.NextStep = nextStep(b)
	b.Narrative = narrate(b)
	return b
}

// Conflicts finds opposing moves and capacity constraints across owners.
func Conflicts(in Input) []Conflict {
	if in.Model == nil {
		return nil
	}
	var out []Conflict
	byKPI := map[string][]signed{}
	for _, rec := range append(append([]planner.Recommendation{}, in.Plan.Recommendations...), in.Plan.Blocked...) {
		for _, k := range rec.Result.KPIs {
			if k.After == k.Before {
				continue
			}
			byKPI[k.KPI] = append(byKPI[k.KPI], signed{action: rec.Action, up: k.After > k.Before, owner: ownerOf(in.Model, rec.Action)})
		}
	}
	for id, moves := range byKPI {
		up, down := false, false
		owners := map[string]bool{}
		var acts []string
		for _, mv := range moves {
			up = up || mv.up
			down = down || !mv.up
			if mv.owner != "" {
				owners[mv.owner] = true
			}
			acts = append(acts, mv.action)
		}
		if !(up && down) || len(owners) < 2 {
			continue
		}
		k, _ := in.Model.KPI(id)
		out = append(out, Conflict{
			KPI: id, Name: k.Name, Actions: unique(acts), Owners: keys(owners), Kind: "opposing",
			Summary: fmt.Sprintf("%s is pushed both ways by %s.", k.Name, strings.Join(unique(acts), " and ")),
		})
	}
	// A demand action that consumes a capacity KPI already off target.
	// The planner omits actions that do not improve the score, so simulate
	// any spec option that never reached the blocked list.
	seen := map[string]bool{}
	for _, rec := range append(append([]planner.Recommendation{}, in.Plan.Recommendations...), in.Plan.Blocked...) {
		seen[rec.Action] = true
	}
	for _, spec := range in.Overlay.Decisions {
		for _, id := range spec.Options {
			if seen[id] {
				continue
			}
			a, ok := in.Model.Action(id)
			if !ok {
				continue
			}
			r, err := sim.ApplyPlan(in.Model, []graph.Action{*a}, sim.Options{})
			if err != nil || len(r.Violations) == 0 {
				continue
			}
			in.Plan.Blocked = append(in.Plan.Blocked, planner.Recommendation{
				Action: a.ID, Name: a.Name, Status: planner.StatusBlocked, BlockedReasons: []string{r.Violations[0].Text},
			})
		}
	}
	for _, g := range in.Gaps {
		if g.Owner != "cto" && !strings.Contains(g.KPI, "capacity") && g.KPI != "engineering_free" {
			continue
		}
		for _, rec := range in.Plan.Blocked {
			for _, reason := range rec.BlockedReasons {
				if strings.Contains(reason, g.KPI) || strings.Contains(strings.ToLower(reason), "capacity") {
					k, _ := in.Model.KPI(g.KPI)
					out = append(out, Conflict{
						KPI: g.KPI, Name: k.Name, Actions: []string{rec.Action},
						Owners: unique([]string{g.Owner, ownerOf(in.Model, rec.Action)}),
						Kind: "capacity",
						Summary: fmt.Sprintf("%s is blocked: %s", rec.Name, reason),
					})
				}
			}
		}
	}
	seenConflict := map[string]bool{}
	var deduped []Conflict
	for _, c := range out {
		key := c.Kind + "|" + strings.Join(c.Actions, ",") + "|" + c.Summary
		if seenConflict[key] {
			continue
		}
		seenConflict[key] = true
		deduped = append(deduped, c)
	}
	sort.SliceStable(deduped, func(i, j int) bool { return deduped[i].KPI < deduped[j].KPI })
	return deduped
}

// Sensitivity re-simulates one action with each assumption turned off, and
// re-ranks to see whether the top recommendation flips. The model is cloned.
func Probe(in Input, actionID string) []Sensitivity {
	if in.Model == nil || len(in.Overlay.Assumptions) == 0 {
		return nil
	}
	base, err := sim.ApplyPlan(in.Model, mustActions(in.Model, actionID), sim.Options{})
	if err != nil {
		return nil
	}
	topBefore := topAction(in.Plan)
	var out []Sensitivity
	for _, a := range in.Overlay.Assumptions {
		clone := in.Model.Clone()
		scaleEdge(clone, a.From, a.To, a.Scale)
		after, err := sim.ApplyPlan(clone, mustActions(clone, actionID), sim.Options{})
		row := Sensitivity{Assumption: a.ID, Statement: a.Statement, Before: improvement(base), After: math.NaN()}
		if err == nil {
			row.After = improvement(after)
		}
		ranked, err := planner.PlanWith(clone, planner.Options{})
		if err == nil {
			row.Now = topAction(ranked)
			row.Was = topBefore
			row.Flips = row.Now != "" && row.Was != "" && row.Now != row.Was
		}
		out = append(out, row)
	}
	return out
}

// Reviews marks 30, 60, and 90 day follow-ups from recorded proposals.
func Reviews(proposals []approvals.Proposal, now time.Time) []Review {
	if now.IsZero() {
		now = time.Now()
	}
	var out []Review
	for _, p := range proposals {
		at := p.CreatedAt
		if p.ExecutedAt != nil {
			at = *p.ExecutedAt
		} else if p.DecidedAt != nil {
			at = *p.DecidedAt
		}
		if at.IsZero() {
			continue
		}
		age := int(now.Sub(at).Hours() / 24)
		window, due := reviewWindow(age)
		verdict := string(p.Phase)
		if verdict == "" {
			verdict = string(p.Status)
		}
		hit := 0.0
		if p.Outcome != nil && len(p.Outcome.Accuracy) > 0 {
			n := 0
			for _, a := range p.Outcome.Accuracy {
				if a.Hit {
					n++
				}
			}
			hit = float64(n) / float64(len(p.Outcome.Accuracy))
		}
		follow := "Wait for the observation window."
		switch {
		case p.Outcome != nil && hit <= 0.5 && len(p.Outcome.Accuracy) > 0:
			follow = "Forecast missed. Open a follow-up decision before repeating the action."
		case string(p.Phase) == string(approvals.PhaseRegressed):
			follow = "Outcome regressed. The compensating action is the follow-up."
		case due:
			follow = "Review is due. Compare predicted and actual, then record the next decision."
		}
		out = append(out, Review{
			ProposalID: p.ID, Action: p.Action, Name: p.ActionName, Owner: p.CreatedBy,
			AgeDays: age, Window: window, Due: due, Verdict: verdict, HitRate: hit, FollowUp: follow,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].AgeDays > out[j].AgeDays })
	return out
}

// BuildDigest writes the weekly note from the inbox, conflicts, and reviews.
func BuildDigest(in Input) Digest {
	inbox := BuildInbox(in)
	d := Digest{Audience: inbox.Audience}
	if len(inbox.Items) == 0 {
		d.Headline = "No decision needs this audience today."
		d.Lines = []string{"Gaps are inside target, or no action is ranked."}
		return d
	}
	d.Headline = fmt.Sprintf("%d decision(s) need %s.", len(inbox.Items), inbox.Audience)
	for _, item := range inbox.Items {
		if item.Rank > 3 {
			break
		}
		d.Lines = append(d.Lines, fmt.Sprintf("%d. %s — owner %s. %s", item.Rank, item.Question, item.Owner, item.NextStep))
	}
	for _, c := range Conflicts(in) {
		d.Lines = append(d.Lines, "Conflict: "+c.Summary)
	}
	due := 0
	for _, r := range Reviews(in.Proposals, in.Now) {
		if r.Due {
			due++
		}
	}
	if due > 0 {
		d.Lines = append(d.Lines, fmt.Sprintf("%d recorded decision(s) are due for a 30, 60, or 90 day review.", due))
	}
	if inbox.Stale > 0 {
		d.Lines = append(d.Lines, fmt.Sprintf("%d input(s) are stale. Approval stays closed until they refresh.", inbox.Stale))
	}
	return d
}

func decisionsFor(in Input, aud string, stale map[string]bool) []InboxItem {
	if len(in.Overlay.Decisions) > 0 {
		var items []InboxItem
		for _, spec := range in.Overlay.Decisions {
			if !forAudience(spec.Audience, aud) {
				continue
			}
			sev, ev := specSeverity(in, spec)
			item := InboxItem{
				ID: spec.ID, Question: spec.Question, Owner: spec.Owner, Audience: spec.Audience,
				Due: spec.Due, Severity: sev, Why: spec.Why, Evidence: ev,
			}
			item.Stale = staleIn(ev, stale)
			item.Blocked = specBlocked(in, spec)
			item.NextStep = nextLabel(item.Blocked, len(item.Stale) > 0, spec.Experiment)
			items = append(items, item)
		}
		if len(items) > 0 {
			return items
		}
	}
	return []InboxItem{{
		ID: "weekly-review", Question: questionFrom(in), Owner: ownerFrom(in),
		Audience: []string{aud}, Severity: totalSeverity(in.Gaps),
		Why: "Ranked from the KPI gaps and the current plan.",
		Evidence: gapIDs(in.Gaps), NextStep: "Open the brief and compare options, including do nothing.",
	}}
}

func evidence(in Input) []Evidence {
	fresh := map[string]string{}
	for _, st := range in.Fresh {
		fresh[st.KPI] = st.Status
	}
	var out []Evidence
	for _, k := range in.Model.KPIs {
		sev := 0.0
		target := 0.0
		if k.Target != nil {
			target = *k.Target
			sev = gaps.Severity(k, k.Value)
		}
		src := "model value"
		if k.Source != nil && k.Source.Kind != "" {
			src = string(k.Source.Kind)
			if k.Source.File != "" {
				src = k.Source.File
			}
		}
		fr := fresh[k.ID]
		if fr == "" {
			fr = freshness.Static
		}
		note := ""
		if sev > 0 {
			note = "off target"
		}
		out = append(out, Evidence{
			KPI: k.ID, Name: k.Name, Owner: k.Owner, Value: k.Value, Target: target,
			Unit: k.DisplayUnit(), Severity: sev, Fresh: fr, Source: src, Note: note,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Severity > out[j].Severity })
	return out
}

func options(in Input, spec DecisionSpec) []Option {
	want := map[string]bool{}
	for _, id := range spec.Options {
		want[id] = true
	}
	if spec.Experiment != "" {
		want[spec.Experiment] = true
	}
	ranked := map[string]planner.Recommendation{}
	for _, rec := range in.Plan.Recommendations {
		ranked[rec.Action] = rec
	}
	for _, rec := range in.Plan.Blocked {
		ranked[rec.Action] = rec
	}
	ids := spec.Options
	if len(ids) == 0 {
		ids = actionIDs(in.Model)
	}
	if spec.Experiment != "" && !contains(ids, spec.Experiment) {
		ids = append(ids, spec.Experiment)
	}
	var out []Option
	for _, id := range ids {
		a, ok := in.Model.Action(id)
		if !ok {
			continue
		}
		opt := Option{ID: a.ID, Name: a.Name, Kind: kindOf(a), Risk: string(a.Risk), Status: "unranked", Note: a.Description}
		if rec, ok := ranked[a.ID]; ok {
			opt.Rank = rec.Rank
			opt.Status = rec.Status
			opt.Improvement = rec.Improvement
			opt.Pessimistic = rec.PessimisticImprovement
			opt.Blocked = append(rec.BlockedReasons, rec.PreconditionFailures...)
			opt.Moves = moves(rec)
		} else if in.Model != nil {
			if r, err := sim.ApplyPlan(in.Model, []graph.Action{*a}, sim.Options{}); err == nil {
				opt.Improvement = r.WeightedImprovement()
				opt.Pessimistic = r.WeightedBefore - r.WeightedAfterWorst
				for _, v := range r.Violations {
					opt.Blocked = append(opt.Blocked, v.Text)
				}
				opt.Blocked = append(opt.Blocked, r.PreconditionFailures...)
				if len(opt.Blocked) > 0 {
					opt.Status = planner.StatusBlocked
				} else if opt.Improvement <= 0 {
					opt.Status = "not-an-improvement"
					opt.Note = strings.TrimSpace(opt.Note + " Not ranked: it does not reduce weighted severity.")
				}
			}
		}
		out = append(out, opt)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Rank == 0 {
			return false
		}
		if out[j].Rank == 0 {
			return true
		}
		return out[i].Rank < out[j].Rank
	})
	return out
}

func nextStep(b Brief) NextStep {
	stale := 0
	for _, e := range b.Evidence {
		if e.Fresh == freshness.Stale || e.Fresh == freshness.Missing {
			stale++
		}
	}
	if stale > 0 {
		return NextStep{Kind: "missing-evidence", Owner: b.Owner, Ready: false,
			Summary: fmt.Sprintf("%d input(s) are stale or missing. Refresh them before approval.", stale)}
	}
	for _, o := range b.Options {
		if o.Kind == "experiment" && o.Status != "blocked" {
			return NextStep{Kind: "experiment", Action: o.ID, Owner: b.Owner, Ready: true,
				Summary: o.Name + " is the experiment if the full spend is blocked or the evidence is thin."}
		}
	}
	for _, o := range b.Options {
		if len(o.Blocked) == 0 && o.Kind == "action" && o.Rank > 0 {
			return NextStep{Kind: "approval", Action: o.ID, Owner: b.Owner, Ready: true,
				Summary: "Ask " + b.Owner + " to approve or reject " + o.Name + ". Do nothing stays on the brief."}
		}
	}
	return NextStep{Kind: "approval", Owner: b.Owner, Ready: false,
		Summary: "No unblocked action is ranked. Postpone or do nothing, and record the assumption."}
}

func narrate(b Brief) string {
	var blocked []string
	for _, o := range b.Options {
		if len(o.Blocked) > 0 {
			blocked = append(blocked, o.Name+": "+o.Blocked[0])
		}
	}
	s := b.Question
	if b.Why != "" {
		s += " " + b.Why
	}
	if len(blocked) > 0 {
		s += " Blocked: " + strings.Join(blocked, "; ") + "."
	}
	s += " " + b.NextStep.Summary
	return s
}

func uncertainty(in Input, opts []Option) []string {
	var lines []string
	for _, a := range in.Overlay.Assumptions {
		lines = append(lines, a.Statement+" Turn it off to see if the recommendation flips.")
	}
	for _, o := range opts {
		if o.Pessimistic < 0 && o.Improvement > 0 {
			lines = append(lines, o.Name+" improves only in the optimistic band.")
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "Bands come from declared edge and effect uncertainty, not from calibrated history.")
	}
	return lines
}

type signed struct {
	action string
	up     bool
	owner  string
}

func scaleEdge(m *graph.Model, from, to string, scale float64) {
	for i := range m.Edges {
		if m.Edges[i].From == from && m.Edges[i].To == to {
			m.Edges[i].Weight *= scale
		}
	}
}

func mustActions(m *graph.Model, id string) []graph.Action {
	a, ok := m.Action(id)
	if !ok {
		return nil
	}
	return []graph.Action{*a}
}

func improvement(r sim.Result) float64 { return r.WeightedBefore - r.WeightedAfter }

func topAction(r planner.Result) string {
	if len(r.Recommendations) == 0 {
		return ""
	}
	return r.Recommendations[0].Action
}

func reviewWindow(age int) (string, bool) {
	switch {
	case age >= 90:
		return "90", true
	case age >= 60:
		return "60", true
	case age >= 30:
		return "30", true
	default:
		return "pending", false
	}
}

func normAudience(a string) string {
	switch strings.ToLower(strings.TrimSpace(a)) {
	case AudienceCEO, AudienceCTO, AudienceMarketing:
		return strings.ToLower(a)
	default:
		return AudienceAll
	}
}

func forAudience(spec []string, aud string) bool {
	if aud == AudienceAll || len(spec) == 0 {
		return true
	}
	for _, s := range spec {
		if strings.EqualFold(s, aud) || strings.EqualFold(s, AudienceAll) {
			return true
		}
	}
	return false
}

func findSpec(o Overlay, id string) (DecisionSpec, bool) {
	for _, d := range o.Decisions {
		if d.ID == id {
			return d, true
		}
	}
	return DecisionSpec{}, false
}

func specSeverity(in Input, spec DecisionSpec) (float64, []string) {
	owners := map[string]bool{spec.Owner: true}
	for _, a := range spec.Audience {
		owners[a] = true
	}
	var sev float64
	var ev []string
	for _, g := range in.Gaps {
		if owners[g.Owner] || contains(spec.Audience, g.Owner) {
			sev += g.Severity
			ev = append(ev, g.KPI)
		}
	}
	if sev == 0 {
		sev = totalSeverity(in.Gaps)
		ev = gapIDs(in.Gaps)
	}
	return sev, ev
}

func specBlocked(in Input, spec DecisionSpec) bool {
	blocked := map[string]bool{}
	for _, rec := range in.Plan.Blocked {
		blocked[rec.Action] = true
	}
	anyOpen := false
	for _, id := range spec.Options {
		if id == "do_nothing" || id == "postpone_30d" {
			continue
		}
		if !blocked[id] {
			anyOpen = true
		}
	}
	return !anyOpen && len(spec.Options) > 0
}

func nextLabel(blocked, stale bool, experiment string) string {
	switch {
	case stale:
		return "Request the missing evidence before approval."
	case blocked && experiment != "":
		return "Full option is blocked. Approve the experiment, or postpone."
	case blocked:
		return "Options are blocked. Postpone or do nothing, and record why."
	default:
		return "Compare the options, including do nothing, then approve or reject."
	}
}

func questionFrom(in Input) string {
	if len(in.Gaps) == 0 {
		return "Which decision, if any, needs this audience?"
	}
	return "What should we do about " + in.Gaps[0].Name + "?"
}

func ownerFrom(in Input) string {
	if len(in.Gaps) > 0 && in.Gaps[0].Owner != "" {
		return in.Gaps[0].Owner
	}
	return "ceo"
}

func actionIDs(m *graph.Model) []string {
	if m == nil {
		return nil
	}
	var ids []string
	for _, a := range m.Actions {
		ids = append(ids, a.ID)
	}
	return ids
}

func ownerOf(m *graph.Model, actionID string) string {
	a, ok := m.Action(strings.Split(actionID, "+")[0])
	if !ok {
		return ""
	}
	if len(a.Effects) == 0 {
		return ""
	}
	k, ok := m.KPI(a.Effects[0].KPI)
	if !ok {
		return ""
	}
	return k.Owner
}

func kindOf(a *graph.Action) string {
	switch {
	case strings.Contains(a.ID, "experiment"):
		return "experiment"
	case strings.Contains(a.ID, "postpone"):
		return "postpone"
	case strings.Contains(a.ID, "nothing"):
		return "nothing"
	default:
		return "action"
	}
}

func moves(rec planner.Recommendation) []string {
	var out []string
	for _, k := range rec.Result.KPIs {
		if k.After == k.Before {
			continue
		}
		dir := "up"
		if k.After < k.Before {
			dir = "down"
		}
		out = append(out, fmt.Sprintf("%s %s", k.KPI, dir))
	}
	return out
}

func staleSet(st []freshness.State) map[string]bool {
	out := map[string]bool{}
	for _, s := range st {
		if !s.Usable() {
			out[s.KPI] = true
		}
	}
	return out
}

func staleIn(ids []string, stale map[string]bool) []string {
	var out []string
	for _, id := range ids {
		if stale[id] {
			out = append(out, id)
		}
	}
	return out
}

func gapIDs(gs []gaps.Gap) []string {
	var ids []string
	for _, g := range gs {
		ids = append(ids, g.KPI)
	}
	return ids
}

func totalSeverity(gs []gaps.Gap) float64 {
	var s float64
	for _, g := range gs {
		s += g.Severity
	}
	return s
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

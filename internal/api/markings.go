// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package api

import (
	"net/http"
	"time"

	"github.com/zyvorai/zyntra/internal/adapters"
	"github.com/zyvorai/zyntra/internal/approvals"
	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/graph"
	"github.com/zyvorai/zyntra/internal/marking"
	"github.com/zyvorai/zyntra/internal/watch"
)

// scheme is the data-marking scheme, or nil when markings are off.
func (s *Server) scheme() *marking.Scheme { return s.opt.Policy.Scheme() }

// snapshotFor is snapshot as the caller may see it. With markings off it is
// the same model; with markings on, KPIs and actions above the caller's
// clearance are gone, along with everything that depends on them, so nothing
// computed from it (gaps, plans, forecasts, briefs, the AI snapshot) can name
// or price them.
func (s *Server) snapshotFor(r *http.Request) (*graph.Model, time.Time) {
	m, at := s.snapshot()
	return s.restrict(m, auth.FromContext(r.Context()).Clearance), at
}

// restrict removes from m (a clone) what clearance may not see.
func (s *Server) restrict(m *graph.Model, clearance string) *graph.Model {
	sc := s.scheme()
	if sc == nil {
		return m
	}
	m.Restrict(
		func(k graph.KPI) bool { return !sc.Dominates(clearance, k.Marking) },
		func(a graph.Action) bool { return !sc.Dominates(clearance, a.Marking) },
	)
	return m
}

// mayRead reports whether the caller's clearance covers a marking.
func (s *Server) mayRead(r *http.Request, mark string) bool {
	return s.scheme().Dominates(auth.FromContext(r.Context()).Clearance, mark)
}

// sourcesFor is the source status list restricted to the KPIs in m (a model
// already restricted for the caller). A source that feeds only hidden KPIs is
// dropped; one that also feeds visible KPIs stays, listing only those.
func (s *Server) sourcesFor(m *graph.Model) []adapters.Status {
	all := s.sourceStatus()
	if s.scheme() == nil {
		return all
	}
	var out []adapters.Status
	for _, st := range all {
		var kept []string
		for _, id := range st.KPIs {
			if _, ok := m.KPI(id); ok {
				kept = append(kept, id)
			}
		}
		if len(st.KPIs) > 0 && len(kept) == 0 {
			continue
		}
		st.KPIs = kept
		out = append(out, st)
	}
	return out
}

// proposalVisible reports whether the caller may see a proposal. With
// markings on, every action in it must be in the caller's restricted model: an
// action above the clearance, one that touches a hidden KPI, or one no longer
// in the pack is not visible, so a proposal never reveals what its action
// would have.
func (s *Server) proposalVisible(r *http.Request, p approvals.Proposal) bool {
	if s.scheme() == nil {
		return true
	}
	vm, _ := s.snapshotFor(r)
	return proposalIn(vm, p)
}

func proposalIn(vm *graph.Model, p approvals.Proposal) bool {
	ids := p.Actions
	if len(ids) == 0 {
		ids = []string{p.Action}
	}
	for _, id := range ids {
		if _, ok := vm.Action(id); !ok {
			return false
		}
	}
	return true
}

// markFilter keeps the proposals the caller's clearance covers.
func (s *Server) markFilter(r *http.Request, in []approvals.Proposal) []approvals.Proposal {
	if s.scheme() == nil {
		return in
	}
	vm, _ := s.snapshotFor(r)
	out := make([]approvals.Proposal, 0, len(in))
	for _, p := range in {
		if proposalIn(vm, p) {
			out = append(out, p)
		}
	}
	return out
}

// markFilterEvents keeps audit events of visible proposals, and events that
// belong to no proposal.
func (s *Server) markFilterEvents(r *http.Request, in []approvals.Event) []approvals.Event {
	if s.scheme() == nil {
		return in
	}
	vm, _ := s.snapshotFor(r)
	hidden := map[string]bool{}
	for _, p := range s.opt.Store.List() {
		if !proposalIn(vm, p) {
			hidden[p.ID] = true
		}
	}
	out := make([]approvals.Event, 0, len(in))
	for _, e := range in {
		if e.Proposal == "" || !hidden[e.Proposal] {
			out = append(out, e)
		}
	}
	return out
}

// shiftProposals is the proposal list for a summary built from m, a model
// already restricted for the reader.
func (s *Server) shiftProposals(m *graph.Model) []approvals.Proposal {
	all := s.opt.Store.List()
	if s.scheme() == nil {
		return all
	}
	out := make([]approvals.Proposal, 0, len(all))
	for _, p := range all {
		if proposalIn(m, p) {
			out = append(out, p)
		}
	}
	return out
}

// watchView is the watch rules, incidents and events the caller may see:
// those on a KPI above the caller's clearance are removed.
func (s *Server) watchView(r *http.Request) watch.View {
	v := s.opt.Watches.View(auth.FromContext(r.Context()).Tenant)
	if s.scheme() == nil {
		return v
	}
	full, _ := s.snapshot()
	vm, _ := s.snapshotFor(r)
	hidden := func(metric string) bool {
		_, inFull := full.KPI(metric)
		_, inView := vm.KPI(metric)
		return inFull && !inView
	}
	gone := map[string]bool{}
	var rules []watch.Rule
	for _, ru := range v.Rules {
		if hidden(ru.Metric) {
			gone[ru.ID] = true
			continue
		}
		rules = append(rules, ru)
	}
	v.Rules = rules
	var incidents []watch.Incident
	for _, in := range v.Incidents {
		if !hidden(in.Rule.Metric) {
			incidents = append(incidents, in)
		}
	}
	v.Incidents = incidents
	var events []watch.Event
	for _, e := range v.Events {
		if !gone[e.Rule] {
			events = append(events, e)
		}
	}
	v.Events = events
	rt := make(map[string]watch.Runtime, len(v.Runtime))
	for id, x := range v.Runtime {
		if !gone[id] {
			rt[id] = x
		}
	}
	v.Runtime = rt
	return v
}

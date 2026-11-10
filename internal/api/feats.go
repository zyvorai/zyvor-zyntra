// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zyvorai/zyntra/internal/activity"
	"github.com/zyvorai/zyntra/internal/approvals"
	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/bundles"
	"github.com/zyvorai/zyntra/internal/egress"
	"github.com/zyvorai/zyntra/internal/planner"
	"github.com/zyvorai/zyntra/internal/schedules"
	"github.com/zyvorai/zyntra/internal/sim"
	"github.com/zyvorai/zyntra/internal/templates"
	"github.com/zyvorai/zyntra/internal/webhooks"
)

// --- schedules ---

func (s *Server) handleSchedules(w http.ResponseWriter, r *http.Request) {
	if s.opt.Schedules == nil {
		writeErr(w, http.StatusNotFound, "schedules not available")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": s.opt.Schedules.List()})
}

func (s *Server) handleSchedulePut(w http.ResponseWriter, r *http.Request) {
	if s.opt.Schedules == nil {
		writeErr(w, http.StatusNotFound, "schedules not available")
		return
	}
	var sc schedules.Schedule
	if !decode(w, r, &sc) {
		return
	}
	if id := r.PathValue("id"); id != "" && id != "_" {
		sc.ID = id
	}
	if m, _ := s.snapshot(); m.Pack != nil && sc.Pack == "" {
		sc.Pack = m.Pack.ID
	}
	out, err := s.opt.Schedules.Put(sc)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleScheduleDelete(w http.ResponseWriter, r *http.Request) {
	if s.opt.Schedules == nil {
		writeErr(w, http.StatusNotFound, "schedules not available")
		return
	}
	if err := s.opt.Schedules.Delete(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// tickSchedules creates pending proposals for due schedules. It never executes.
func (s *Server) tickSchedules(now time.Time) {
	if s.opt.Schedules == nil {
		return
	}
	m, _ := s.snapshot()
	for _, sc := range s.opt.Schedules.DueNow(now) {
		if m.Pack != nil && sc.Pack != "" && sc.Pack != m.Pack.ID {
			continue
		}
		action := sc.Action
		if action == "" {
			plan, err := planner.Plan(m)
			if err != nil || len(plan) == 0 {
				_ = s.opt.Schedules.MarkRun(sc.ID, now)
				continue
			}
			recs := plan
			if sc.Owner != "" {
				res := planner.ForOwner(m, planner.Result{Recommendations: plan}, sc.Owner)
				recs = res.Recommendations
			}
			if len(recs) == 0 {
				_ = s.opt.Schedules.MarkRun(sc.ID, now)
				continue
			}
			action = recs[0].Action
		}
		if s.hasPendingScheduleProposal(sc.ID, action) {
			_ = s.opt.Schedules.MarkRun(sc.ID, now)
			continue
		}
		if err := s.proposeScheduled(sc, action, now); err != nil {
			log.Printf("schedule %s: %v", sc.ID, err)
			continue
		}
		_ = s.opt.Schedules.MarkRun(sc.ID, now)
	}
}

func (s *Server) hasPendingScheduleProposal(scheduleID, action string) bool {
	by := "schedule:" + scheduleID
	for _, p := range s.opt.Store.List() {
		if (p.Status == approvals.Pending || p.Status == approvals.Approved) && p.Action == action && p.CreatedBy == by {
			return true
		}
	}
	return false
}

func (s *Server) proposeScheduled(sc schedules.Schedule, action string, _ time.Time) error {
	m, _ := s.snapshot()
	acts, err := resolveActions(m, action)
	if err != nil {
		return err
	}
	st := s.freshness(m)
	res, err := sim.ApplyPlan(m, acts, sim.Options{})
	if err != nil {
		return err
	}
	if len(res.Violations) > 0 || len(res.PreconditionFailures) > 0 {
		return nil // skip silently; will retry next interval
	}
	eff := s.opt.Policy.For(acts...)
	risk, adapter := describe(acts)
	ids := make([]string, len(acts))
	for i, a := range acts {
		ids[i] = a.ID
	}
	p := approvals.Proposal{
		Action: res.Action, Actions: ids, ActionName: res.ActionName, Risk: string(risk), Adapter: adapter,
		ModelVersion: m.Version(), Inputs: s.inputs(m, st), Simulation: &res, Policy: &eff,
		RequiredApprovals: eff.Approvals, Compensate: compensations(acts),
		Predicted: approvals.Prediction{SeverityBefore: res.SeverityBefore, SeverityAfter: res.SeverityAfter,
			Closes: res.GapsClosed, Opens: res.GapsOpened, KPIs: map[string]float64{}},
		Baseline: map[string]float64{},
	}
	for _, k := range res.KPIs {
		if k.Change != 0 {
			p.Predicted.KPIs[k.KPI] = k.After
			p.Baseline[k.KPI] = k.Before
		}
	}
	if m.Pack != nil {
		p.PackID = m.Pack.ID
	}
	rd, rerr := s.render(context.Background(), m, acts)
	p.Template, p.Kinds, p.Evidence = strings.Join(rd.templates, "+"), rd.kinds, rd.evidence
	if rerr != nil {
		p.RenderErr = rerr.Error()
	} else {
		p.Render = rd.display
	}
	_, _, err = s.opt.Store.Create(p, "schedule:"+sc.ID)
	return err
}

// --- webhooks ---

func (s *Server) handleWebhooks(w http.ResponseWriter, r *http.Request) {
	if s.opt.Webhooks == nil {
		writeErr(w, http.StatusNotFound, "webhooks not available")
		return
	}
	list := s.opt.Webhooks.List()
	out := make([]webhooks.Destination, len(list))
	for i, d := range list {
		out[i] = d.Public()
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": out})
}

func (s *Server) handleWebhookPut(w http.ResponseWriter, r *http.Request) {
	if s.opt.Webhooks == nil {
		writeErr(w, http.StatusNotFound, "webhooks not available")
		return
	}
	var d webhooks.Destination
	if !decode(w, r, &d) {
		return
	}
	if id := r.PathValue("id"); id != "" && id != "_" {
		d.ID = id
	}
	out, err := s.opt.Webhooks.Put(d)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out.Public())
}

func (s *Server) handleWebhookDelete(w http.ResponseWriter, r *http.Request) {
	if s.opt.Webhooks == nil {
		writeErr(w, http.StatusNotFound, "webhooks not available")
		return
	}
	if err := s.opt.Webhooks.Delete(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleWebhookTest(w http.ResponseWriter, r *http.Request) {
	if s.opt.Webhooks == nil {
		writeErr(w, http.StatusNotFound, "webhooks not available")
		return
	}
	d, ok := s.opt.Webhooks.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "no such webhook")
		return
	}
	ev := webhooks.Event{
		Type: "test",
		At:   time.Now().UTC(),
		Data: map[string]any{"test": true},
	}
	client := s.webhookClient()
	status, err := webhooks.Deliver(d.URL, d.Secret, ev, client)
	s.opt.Webhooks.RecordDelivery(d.ID, status, time.Now(), err)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status})
}

func (s *Server) webhookClient() *http.Client {
	if s.opt.Policy != nil {
		return s.opt.Policy.EgressGuard().Client(&http.Client{Timeout: 5 * time.Second}, egress.Notify)
	}
	return &http.Client{Timeout: 5 * time.Second}
}

// DeliverWebhooks fans approval events out to matching destinations.
func (s *Server) DeliverWebhooks(e approvals.Event) {
	if s.opt.Webhooks == nil {
		return
	}
	typ := string(e.To)
	ev := webhooks.Event{
		Type: typ,
		At:   e.At,
		Data: map[string]any{
			"proposal": e.Proposal,
			"action":   e.Action,
			"from":     string(e.From),
			"to":       string(e.To),
			"by":       e.By,
		},
	}
	client := s.webhookClient()
	for _, d := range s.opt.Webhooks.Matching(typ) {
		status, err := webhooks.Deliver(d.URL, d.Secret, ev, client)
		s.opt.Webhooks.RecordDelivery(d.ID, status, time.Now(), err)
	}
}

// --- bundles ---

func (s *Server) handleBundleExport(w http.ResponseWriter, r *http.Request) {
	packID := ""
	if m, _ := s.snapshot(); m.Pack != nil {
		packID = m.Pack.ID
	}
	items := map[string]any{}
	if s.opt.Schedules != nil {
		items["schedules"] = s.opt.Schedules.List()
	}
	if s.opt.Webhooks != nil {
		list := s.opt.Webhooks.List()
		pubs := make([]webhooks.Destination, len(list))
		for i, d := range list {
			pubs[i] = d.Public()
			pubs[i].Secret = "" // never export secrets
		}
		items["webhooks"] = pubs
	}
	items["templates"] = templates.List()
	b := bundles.Export(packID, items)
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleBundleImport(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	b, err := bundles.Import(body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	imported := map[string]int{}
	if raw, ok := b.Items["schedules"]; ok && s.opt.Schedules != nil {
		var list []schedules.Schedule
		if jb, err := json.Marshal(raw); err == nil && json.Unmarshal(jb, &list) == nil {
			for _, sc := range list {
				if _, err := s.opt.Schedules.Put(sc); err == nil {
					imported["schedules"]++
				}
			}
		}
	}
	if raw, ok := b.Items["webhooks"]; ok && s.opt.Webhooks != nil {
		var list []webhooks.Destination
		if jb, err := json.Marshal(raw); err == nil && json.Unmarshal(jb, &list) == nil {
			for _, d := range list {
				d.Secret = "" // force new secrets on import
				if _, err := s.opt.Webhooks.Put(d); err == nil {
					imported["webhooks"]++
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported, "pack": b.Pack, "version": b.Version})
}

// --- templates ---

func (s *Server) handleTemplates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"templates": templates.List()})
}

func (s *Server) handleTemplatePropose(w http.ResponseWriter, r *http.Request) {
	t, ok := templates.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "no such template")
		return
	}
	m, _ := s.snapshotFor(r)
	if m.Pack != nil && t.Pack != "" && t.Pack != m.Pack.ID {
		writeErr(w, http.StatusUnprocessableEntity, "template is for pack "+t.Pack)
		return
	}
	// Reuse propose body shape.
	body, _ := json.Marshal(map[string]any{"actions": t.Actions})
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	r.ContentLength = int64(len(body))
	s.handlePropose(w, r)
}

// --- activity ---

func (s *Server) handleActivity(w http.ResponseWriter, _ *http.Request) {
	var list []activity.Record
	if s.opt.Activity != nil {
		list = s.opt.Activity.List()
	}
	if list == nil {
		list = []activity.Record{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"activity": list})
}

func (s *Server) touchActivity(r *http.Request) {
	if s.opt.Activity == nil {
		return
	}
	id := auth.FromContext(r.Context())
	if id.Subject != "" {
		s.opt.Activity.Touch(id.Subject, time.Now())
	}
}

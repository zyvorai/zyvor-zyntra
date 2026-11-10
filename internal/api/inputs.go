// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package api

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/graph"
	"github.com/zyvorai/zyntra/internal/inputs"
)

// channelKPIs returns the KPIs fed by a webhook-in channel.
func channelKPIs(m *graph.Model, name string) []string {
	var out []string
	for _, k := range m.KPIs {
		if k.Source != nil && k.Source.Kind == graph.SourceWebhookIn && k.Source.Name == name {
			out = append(out, k.ID)
		}
	}
	return out
}

// handleIngest stores a document POSTed by a gateway; the last one wins.
func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	m, _ := s.snapshotFor(r)
	kpis := channelKPIs(m, name)
	if len(kpis) == 0 {
		writeErr(w, http.StatusNotFound, "no KPI reads webhook-in channel "+name)
		return
	}
	var body any
	if !decode(w, r, &body) {
		return
	}

	if len(r.Header.Get("Idempotency-Key")) > 200 {
		writeErr(w, 400, "idempotency key exceeds 200 characters")
		return
	}
	who := auth.FromContext(r.Context())
	received := time.Now().UTC()
	observed := time.Time{}
	if raw := r.Header.Get("X-Zyntra-Observed-At"); raw != "" {
		var err error
		observed, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil || observed.After(received.Add(time.Minute)) {
			writeErr(w, 400, "invalid observation timestamp")
			return
		}
	}
	quality, qualityErr := s.ingestQuality(name, body, received)
	quarantine := ""
	if qualityErr != nil {
		quarantine = "ingest contract validation failed"
	} else if quality != nil && !quality.Valid {
		quarantine = "batch violates data contract"
	}
	receipt, err := s.opt.Inputs.IngestEvent(name, body, who.Subject, r.Header.Get("Idempotency-Key"), observed, received, quarantine)
	if err != nil {
		if errors.Is(err, inputs.ErrIdempotencyConflict) {
			writeErr(w, 409, err.Error())
		} else {
			writeErr(w, 500, "event could not be persisted")
		}
		return
	}
	if receipt.Status == "quarantined" {
		writeJSON(w, 422, map[string]any{"ok": false, "channel": name, "receipt": receipt, "quality": quality, "error": "event quarantined; latest input unchanged"})
		return
	}
	if receipt.Status == "accepted" && !receipt.Duplicate {
		s.RefreshOnce(r.Context())
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "channel": name, "kpis": kpis, "receipt": receipt})
}

// handleManualValue records an operator-entered value for a manual KPI.
// Every entry is written to the audit chain.
func (s *Server) handleManualValue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, _ := s.snapshotFor(r)
	k, ok := m.KPI(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "unknown kpi")
		return
	}
	if k.Source == nil || k.Source.Kind != graph.SourceManual {
		writeErr(w, http.StatusBadRequest, id+" is not a manual KPI")
		return
	}
	var req struct {
		Value  *float64 `json:"value"`
		Reason string   `json:"reason"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Value == nil || math.IsNaN(*req.Value) || math.IsInf(*req.Value, 0) {
		writeErr(w, http.StatusBadRequest, "value must be a number")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if len(req.Reason) > 500 {
		writeErr(w, http.StatusBadRequest, "reason must be at most 500 characters")
		return
	}
	who := auth.FromContext(r.Context())
	entry := inputs.Manual{Value: *req.Value, At: time.Now(), By: who.Subject, Reason: req.Reason}
	if err := s.opt.Inputs.SetManual(id, entry); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	note := fmt.Sprintf("manual value %s = %g %s", id, *req.Value, k.DisplayUnit())
	if req.Reason != "" {
		note += ": " + req.Reason
	}
	if err := s.opt.Store.Note("manual:"+id, who.Subject, strings.TrimSpace(note)); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.RefreshOnce(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"kpi": id, "entry": entry})
}

type manualView struct {
	KPI   string         `json:"kpi"`
	Name  string         `json:"name"`
	Unit  string         `json:"unit,omitempty"`
	Owner string         `json:"owner,omitempty"`
	Value float64        `json:"value"`
	Entry *inputs.Manual `json:"entry,omitempty"`
}

type channelView struct {
	Name     string     `json:"name"`
	KPIs     []string   `json:"kpis"`
	Received *time.Time `json:"received_at,omitempty"`
	From     string     `json:"from,omitempty"`
}

// handleInputs lists manual KPIs and webhook-in channels.
func (s *Server) handleInputs(w http.ResponseWriter, r *http.Request) {
	m, _ := s.snapshotFor(r)
	manual := []manualView{}
	channels := []channelView{}
	seen := map[string]bool{}
	for _, k := range m.KPIs {
		if k.Source == nil {
			continue
		}
		switch k.Source.Kind {
		case graph.SourceManual:
			v := manualView{KPI: k.ID, Name: k.Name, Unit: k.DisplayUnit(), Owner: k.Owner, Value: k.Value}
			if e, ok := s.opt.Inputs.Manual(k.ID); ok {
				v.Entry = &e
			}
			manual = append(manual, v)
		case graph.SourceWebhookIn:
			if seen[k.Source.Name] {
				continue
			}
			seen[k.Source.Name] = true
			c := channelView{Name: k.Source.Name, KPIs: channelKPIs(m, k.Source.Name)}
			if d, ok := s.opt.Inputs.Webhook(k.Source.Name); ok {
				at := d.At
				c.Received, c.From = &at, d.From
			}
			channels = append(channels, c)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"manual": manual, "webhooks": channels})
}

// Event bodies can include fields beyond visible KPIs. Require full visibility
// of every KPI using this channel before exposing retained payloads.
func (s *Server) handleInputEvents(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	visible, _ := s.snapshotFor(r)
	full, _ := s.snapshot()
	if len(channelKPIs(visible, name)) == 0 || len(channelKPIs(visible, name)) != len(channelKPIs(full, name)) {
		writeErr(w, 404, "unknown input channel")
		return
	}
	after := uint64(0)
	limit := 100
	if raw := r.URL.Query().Get("after"); raw != "" {
		var err error
		after, err = strconv.ParseUint(raw, 10, 64)
		if err != nil {
			writeErr(w, 400, "invalid after cursor")
			return
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 200 {
			writeErr(w, 400, "limit must be 1..200")
			return
		}
	}
	writeJSON(w, 200, s.opt.Inputs.Events(name, after, limit))
}

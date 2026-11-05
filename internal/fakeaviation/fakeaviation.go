// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package fakeaviation serves invented operations and maintenance exports in
// the paged JSON shapes the templates in examples/aviation read, so they can be
// run and tested without any real system. Nothing here resembles a particular
// product: the shapes are common REST patterns (a nested list with a links.next
// URL, OData's value and @odata.nextLink, and a results list with meta.next).
package fakeaviation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Token is the bearer token every endpoint requires.
const Token = "fake-aviation-token"

// PageSize is how many rows each page holds.
const PageSize = 2

type row = map[string]any

// flights are stamped relative to now, so a trailing-window KPI has rows on both
// sides of its edge: three within 24 hours (2, 5 and 20 hours ago) and two
// outside it (30 and 50 hours ago).
func flights(now time.Time) []row {
	at := func(h int) string { return now.Add(-time.Duration(h) * time.Hour).UTC().Format(time.RFC3339) }
	return []row{
		{"id": "f-1", "status": "departed", "dep_delay_min": 0, "turnaround_min": 40, "on_time": "yes", "departed_at": at(2)},
		{"id": "f-2", "status": "departed", "dep_delay_min": 12, "turnaround_min": 45, "on_time": "yes", "departed_at": at(5)},
		{"id": "f-3", "status": "departed", "dep_delay_min": 25, "turnaround_min": 55, "on_time": "no", "departed_at": at(20)},
		{"id": "f-4", "status": "departed", "dep_delay_min": 70, "turnaround_min": 80, "on_time": "no", "departed_at": at(30)},
		{"id": "f-5", "status": "departed", "dep_delay_min": 130, "turnaround_min": 95, "on_time": "no", "departed_at": at(50)},
	}
}

var workOrders = []row{
	{"id": "w-1", "tail": "T1", "kind": "scheduled", "state": "open", "age_days": 4, "due_in_days": 10},
	{"id": "w-2", "tail": "T3", "kind": "deferred", "state": "open", "age_days": 30, "due_in_days": -5},
	{"id": "w-3", "tail": "T4", "kind": "deferred", "state": "open", "age_days": 9, "due_in_days": -1},
	{"id": "w-4", "tail": "T2", "kind": "scheduled", "state": "closed", "age_days": 2, "due_in_days": 0},
	{"id": "w-5", "tail": "T5", "kind": "unscheduled", "state": "open", "age_days": 12, "due_in_days": 2},
}

var parts = []row{
	{"id": "p-1", "aog_critical": "yes", "requested": 10, "filled": 9, "on_hand": 3},
	{"id": "p-2", "aog_critical": "yes", "requested": 4, "filled": 2, "on_hand": 0},
	{"id": "p-3", "aog_critical": "no", "requested": 6, "filled": 6, "on_hand": 12},
	{"id": "p-4", "aog_critical": "yes", "requested": 5, "filled": 3, "on_hand": 0},
	{"id": "p-5", "aog_critical": "no", "requested": 3, "filled": 3, "on_hand": 1},
}

// Handler serves /ops/flights, /mro/work-orders and /mro/parts. Add ?page=N to
// page through them. Next links are relative, so the handler works mounted under
// a prefix (http.StripPrefix).
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ops/flights", func(w http.ResponseWriter, r *http.Request) {
		page, items, more := slice(r, flights(time.Now()))
		body := row{"data": row{"flights": items}, "links": row{"next": nil}}
		if more {
			body["links"] = row{"next": fmt.Sprintf("flights?page=%d", page+1)}
		}
		reply(w, r, body)
	})
	mux.HandleFunc("GET /mro/work-orders", func(w http.ResponseWriter, r *http.Request) {
		page, items, more := slice(r, workOrders)
		body := row{"value": items}
		if more {
			body["@odata.nextLink"] = fmt.Sprintf("work-orders?page=%d", page+1)
		}
		reply(w, r, body)
	})
	mux.HandleFunc("GET /mro/parts", func(w http.ResponseWriter, r *http.Request) {
		page, items, more := slice(r, parts)
		body := row{"results": items, "meta": row{"next": nil}}
		if more {
			body["meta"] = row{"next": fmt.Sprintf("parts?page=%d", page+1)}
		}
		reply(w, r, body)
	})
	return mux
}

func slice(r *http.Request, all []row) (page int, items []row, more bool) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	lo := (page - 1) * PageSize
	if lo >= len(all) {
		return page, []row{}, false
	}
	hi := min(lo+PageSize, len(all))
	return page, all[lo:hi], hi < len(all)
}

func reply(w http.ResponseWriter, r *http.Request, body any) {
	if r.Header.Get("Authorization") != "Bearer "+Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

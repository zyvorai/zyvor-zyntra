// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package fakeindustrial serves invented plant and process-safety exports in
// the paged JSON shapes the template in examples/industrial reads, so it can
// be run and tested without any real system. Nothing here resembles a
// particular product: the shapes are common REST patterns (a nested list with a
// links.next URL, OData's value and @odata.nextLink, and a results list with
// meta.next). The rows match the CSV fixtures of packs/industrial-plant and
// packs/process-safety.
package fakeindustrial

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Token is the bearer token every endpoint requires.
const Token = "fake-industrial-token"

// PageSize is how many rows each page holds.
const PageSize = 2

type row = map[string]any

var machines = []row{
	{"id": "m1", "state": "running", "planned_h": 100, "run_h": 90, "ideal_units": 1000, "good_units": 880, "total_units": 900, "mttr_h": 4},
	{"id": "m2", "state": "running", "planned_h": 100, "run_h": 80, "ideal_units": 1000, "good_units": 700, "total_units": 800, "mttr_h": 6},
	{"id": "m3", "state": "down", "planned_h": 100, "run_h": 40, "ideal_units": 500, "good_units": 300, "total_units": 400, "mttr_h": 12},
	{"id": "m4", "state": "idle", "planned_h": 100, "run_h": 70, "ideal_units": 800, "good_units": 650, "total_units": 700, "mttr_h": 5},
}

var workOrders = []row{
	{"id": "w1", "type": "preventive", "state": "done", "overdue_days": 0},
	{"id": "w2", "type": "preventive", "state": "open", "overdue_days": 5},
	{"id": "w3", "type": "corrective", "state": "open", "overdue_days": 0},
	{"id": "w4", "type": "preventive", "state": "done", "overdue_days": 0},
	{"id": "w5", "type": "preventive", "state": "open", "overdue_days": 0},
	{"id": "w6", "type": "corrective", "state": "done", "overdue_days": 0},
}

var devices = []row{
	{"id": "d1", "class": "sis", "due_in_days": -3},
	{"id": "d2", "class": "sis", "due_in_days": 20},
	{"id": "d3", "class": "relief", "due_in_days": -10},
	{"id": "d4", "class": "alarm", "due_in_days": 5},
	{"id": "d5", "class": "sis", "due_in_days": 40},
}

// Handler serves /mes/machines, /cmms/work-orders and /safety/devices. Add
// ?page=N to page through them. Next links are relative, so the handler works
// mounted under a prefix (http.StripPrefix).
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /mes/machines", func(w http.ResponseWriter, r *http.Request) {
		page, items, more := slice(r, machines)
		body := row{"data": row{"machines": items}, "links": row{"next": nil}}
		if more {
			body["links"] = row{"next": fmt.Sprintf("machines?page=%d", page+1)}
		}
		reply(w, r, body)
	})
	mux.HandleFunc("GET /cmms/work-orders", func(w http.ResponseWriter, r *http.Request) {
		page, items, more := slice(r, workOrders)
		body := row{"value": items}
		if more {
			body["@odata.nextLink"] = fmt.Sprintf("work-orders?page=%d", page+1)
		}
		reply(w, r, body)
	})
	mux.HandleFunc("GET /safety/devices", func(w http.ResponseWriter, r *http.Request) {
		page, items, more := slice(r, devices)
		body := row{"results": items, "meta": row{"next": nil}}
		if more {
			body["meta"] = row{"next": fmt.Sprintf("devices?page=%d", page+1)}
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

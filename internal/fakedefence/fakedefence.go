// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package fakedefence serves invented maintenance, supply and GRC exports in
// the paged JSON shapes the templates in examples/defence read. It exists so
// the templates can be run and tested without any real system. Nothing here
// resembles a particular product: the shapes are common REST patterns
// (a nested list with a links.next URL, OData's value and @odata.nextLink,
// and a results list with meta.next).
package fakedefence

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Token is the bearer token every endpoint requires.
const Token = "fake-defence-token"

// PageSize is how many rows each page holds.
const PageSize = 2

type row = map[string]any

var workOrders = []row{
	{"id": "w-1", "asset_id": "a-03", "state": "open", "age_days": 9},
	{"id": "w-2", "asset_id": "a-05", "state": "open", "age_days": 14},
	{"id": "w-3", "asset_id": "a-08", "state": "open", "age_days": 3},
	{"id": "w-4", "asset_id": "a-09", "state": "open", "age_days": 6},
	{"id": "w-5", "asset_id": "a-01", "state": "closed", "age_days": 4},
}

var requisitions = []row{
	{"id": "r-1", "part": "p-1", "status": "backordered", "requested": 4, "filled": 1},
	{"id": "r-2", "part": "p-2", "status": "backordered", "requested": 2, "filled": 1},
	{"id": "r-3", "part": "p-3", "status": "filled", "requested": 3, "filled": 2},
	{"id": "r-4", "part": "p-4", "status": "filled", "requested": 2, "filled": 2},
	{"id": "r-5", "part": "p-5", "status": "filled", "requested": 2, "filled": 2},
	{"id": "r-6", "part": "p-6", "status": "filled", "requested": 1, "filled": 1},
}

var controls = []row{
	{"id": "3.1.1", "family": "AC", "status": "implemented", "evidence_age_days": 30},
	{"id": "3.1.2", "family": "AC", "status": "implemented", "evidence_age_days": 45},
	{"id": "3.3.1", "family": "AU", "status": "implemented", "evidence_age_days": 20},
	{"id": "3.4.1", "family": "CM", "status": "partial", "evidence_age_days": 200},
	{"id": "3.5.3", "family": "IA", "status": "implemented", "evidence_age_days": 15},
	{"id": "3.6.1", "family": "IR", "status": "partial", "evidence_age_days": 150},
	{"id": "3.13.11", "family": "SC", "status": "implemented", "evidence_age_days": 60},
	{"id": "3.14.1", "family": "SI", "status": "implemented", "evidence_age_days": 25},
}

// Handler serves /maintenance/work-orders, /supply/requisitions and
// /grc/controls. Add ?page=N to page through them. Next links are relative, so
// the handler works mounted under a prefix (http.StripPrefix).
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /maintenance/work-orders", func(w http.ResponseWriter, r *http.Request) {
		page, items, more := slice(r, workOrders)
		body := row{"data": row{"orders": items}, "links": row{"next": nil}}
		if more {
			body["links"] = row{"next": fmt.Sprintf("work-orders?page=%d", page+1)}
		}
		reply(w, r, body)
	})
	mux.HandleFunc("GET /supply/requisitions", func(w http.ResponseWriter, r *http.Request) {
		page, items, more := slice(r, requisitions)
		body := row{"value": items}
		if more {
			body["@odata.nextLink"] = fmt.Sprintf("requisitions?page=%d", page+1)
		}
		reply(w, r, body)
	})
	mux.HandleFunc("GET /grc/controls", func(w http.ResponseWriter, r *http.Request) {
		page, items, more := slice(r, controls)
		body := row{"results": items, "meta": row{"next": nil}}
		if more {
			body["meta"] = row{"next": fmt.Sprintf("controls?page=%d", page+1)}
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

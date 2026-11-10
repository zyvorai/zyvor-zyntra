// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package templates holds reusable decision skeletons. These are starting
// points for proposals; they never execute actions.
package templates

// Template is a named starting point.
type Template struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Pack        string   `json:"pack"`
	Actions     []string `json:"actions"`
	Description string   `json:"description"`
}

// List returns built-in starters.
func List() []Template {
	return []Template{
		{ID: "capacity-scale", Title: "Scale capacity", Pack: "gpu", Actions: []string{"scale_node_pool"}, Description: "Add nodes when queue latency exceeds target"},
		{ID: "stock-reorder", Title: "Reorder stock", Pack: "shop", Actions: []string{"reorder_fast_movers"}, Description: "Raise PO when stockout rate is high"},
		{ID: "load-shed", Title: "Shed load", Pack: "industrial-plant", Actions: []string{"shed_load"}, Description: "Reduce load in a tariff window"},
	}
}

// Get returns a template by id.
func Get(id string) (Template, bool) {
	for _, t := range List() {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package bundles supports export/import of pack configuration subsets.
package bundles

import "encoding/json"

// Bundle is a versioned export of selected pack resources.
type Bundle struct {
	Version int            `json:"version"`
	Pack    string         `json:"pack"`
	Items   map[string]any `json:"items"`
}

// Export creates a minimal bundle.
func Export(packID string, items map[string]any) Bundle {
	if items == nil {
		items = map[string]any{}
	}
	return Bundle{Version: 1, Pack: packID, Items: items}
}

// Import validates version and returns the items.
func Import(data []byte) (Bundle, error) {
	var b Bundle
	if err := json.Unmarshal(data, &b); err != nil {
		return b, err
	}
	if b.Version != 1 {
		return b, ErrBadVersion
	}
	if b.Items == nil {
		b.Items = map[string]any{}
	}
	return b, nil
}

// ErrBadVersion is returned for unsupported bundle versions.
var ErrBadVersion = errString("bundle version 1 required")

type errString string

func (e errString) Error() string { return string(e) }

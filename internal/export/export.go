// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package export formats decision records for offline verification.
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package export

import "encoding/json"

// Decision is a minimal signed-record shape.
type Decision struct {
	ID         string                 `json:"id"`
	ModelHash  string                 `json:"model_hash"`
	Prediction map[string]interface{} `json:"prediction"`
	Approver   string                 `json:"approver,omitempty"`
}

// ToJSON returns canonical JSON.
func ToJSON(d Decision) ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

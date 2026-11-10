// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zyvorai/zyntra/internal/semantic"
	"os"
	"path/filepath"
	"time"
)

type ingestRule struct {
	Contract      string `json:"contract"`
	RecordsField  string `json:"records_field,omitempty"`
	ObservedField string `json:"observed_field,omitempty"`
}

// Rules explicitly bind a channel to a semantic contract. Missing configuration
// preserves old ingestion behavior; invalid configuration fails closed.
func (s *Server) ingestQuality(channel string, body any, now time.Time) (*semantic.BatchCheck, error) {
	if s.opt.PackDir == "" {
		return nil, nil
	}
	b, err := os.ReadFile(filepath.Join(s.opt.PackDir, "ingest-contracts.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rules := map[string]ingestRule{}
	if err = json.Unmarshal(b, &rules); err != nil {
		return nil, err
	}
	rule, ok := rules[channel]
	if !ok {
		return nil, nil
	}
	contracts, err := s.semanticContracts()
	if err != nil {
		return nil, err
	}
	var contract *semantic.Contract
	for i := range contracts {
		if contracts[i].Name == rule.Contract {
			contract = &contracts[i]
			break
		}
	}
	if contract == nil {
		return nil, fmt.Errorf("unknown ingest contract")
	}
	raw := body
	if rule.RecordsField != "" {
		obj, ok := body.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object with records field")
		}
		raw = obj[rule.RecordsField]
	}
	rows := []map[string]any{}
	switch value := raw.(type) {
	case map[string]any:
		rows = append(rows, value)
	case []any:
		for _, v := range value {
			obj, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("each record must be an object")
			}
			rows = append(rows, obj)
		}
	default:
		return nil, fmt.Errorf("records must be an object or array of objects")
	}
	report, err := semantic.CheckBatch(*contract, rows, rule.ObservedField, now)
	return &report, err
}

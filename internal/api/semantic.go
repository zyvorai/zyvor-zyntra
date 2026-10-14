// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/semantic"
)

// Contracts carry no tenant, so tenant-bound callers must not see them.
func tenantBound(r *http.Request) bool {
	return auth.FromContext(r.Context()).Tenant != ""
}

func (s *Server) semanticContracts() ([]semantic.Contract, error) {
	if s.opt.PackDir == "" {
		return nil, os.ErrNotExist
	}
	return semantic.LoadFile(filepath.Join(s.opt.PackDir, "data-contracts.json"))
}

func (s *Server) handleSemanticCatalog(w http.ResponseWriter, r *http.Request) {
	if tenantBound(r) {
		writeJSON(w, http.StatusOK, map[string]any{"contracts": []semantic.CatalogEntry{}, "configured": false})
		return
	}
	cs, err := s.semanticContracts()
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusOK, map[string]any{"contracts": []semantic.CatalogEntry{}, "configured": false})
		return
	}
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contracts": semantic.Catalog(cs), "configured": true})
}

func (s *Server) handleSemanticLineage(w http.ResponseWriter, r *http.Request) {
	if tenantBound(r) {
		writeJSON(w, http.StatusOK, semantic.Lineage{})
		return
	}
	cs, err := s.semanticContracts()
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusOK, semantic.Lineage{})
		return
	}
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, semantic.BuildLineage(cs))
}

func (s *Server) handleSemanticResources(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"resources": semantic.AgentResources()})
}

func (s *Server) handleSemanticContractValidate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	defer r.Body.Close()
	var cs []semantic.Contract
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cs); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := semantic.Validate(cs); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "count": len(cs)})
}

func (s *Server) handleSemanticConfidence(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	defer r.Body.Close()
	var f semantic.Factors
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	c, err := semantic.EvaluateConfidence(f, time.Now())
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

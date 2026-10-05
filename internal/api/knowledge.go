// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/zyvorai/zyntra/internal/auth"
	"github.com/zyvorai/zyntra/internal/knowledge"
)

func documentReader(r *http.Request) knowledge.Reader {
	id := auth.FromContext(r.Context())
	roles := []string{}
	for _, role := range id.Roles {
		roles = append(roles, string(role))
	}
	if len(roles) == 0 && id.Role != "" {
		roles = append(roles, string(id.Role))
	}
	return knowledge.Reader{Tenant: id.Tenant, Roles: roles, Admin: id.Has(auth.RoleAdmin)}
}
func knowledgeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, knowledge.ErrNotFound):
		writeErr(w, 404, "document not found")
	case errors.Is(err, knowledge.ErrConflict):
		writeErr(w, 409, err.Error())
	case errors.Is(err, knowledge.ErrInvalid):
		writeErr(w, 400, err.Error())
	case errors.Is(err, knowledge.ErrCapacity):
		writeErr(w, 409, err.Error())
	default:
		writeErr(w, 500, "document store unavailable")
	}
}
func (s *Server) knowledgeReady(w http.ResponseWriter) bool {
	if s.opt.Knowledge == nil {
		writeErr(w, 503, "document store unavailable")
		return false
	}
	return true
}
func (s *Server) handleDocuments(w http.ResponseWriter, r *http.Request) {
	if !s.knowledgeReady(w) {
		return
	}
	docs, err := s.opt.Knowledge.List(documentReader(r))
	if err != nil {
		knowledgeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"documents": docs})
}
func (s *Server) handleDocument(w http.ResponseWriter, r *http.Request) {
	if !s.knowledgeReady(w) {
		return
	}
	version := 0
	if v := r.URL.Query().Get("version"); v != "" {
		var err error
		version, err = strconv.Atoi(v)
		if err != nil || version < 1 {
			writeErr(w, 400, "version must be a positive integer")
			return
		}
	}
	doc, err := s.opt.Knowledge.Get(r.PathValue("id"), version, documentReader(r))
	if err != nil {
		knowledgeErr(w, err)
		return
	}
	writeJSON(w, 200, doc)
}
func (s *Server) handleDocumentPut(w http.ResponseWriter, r *http.Request) {
	if !s.knowledgeReady(w) {
		return
	}
	var req struct {
		Document        knowledge.Document `json:"document"`
		ExpectedVersion *int               `json:"expected_version"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.ExpectedVersion == nil {
		writeErr(w, 400, "expected_version is required (zero for creation)")
		return
	}
	req.Document.ID = r.PathValue("id")
	d, err := s.opt.Knowledge.Put(req.Document, *req.ExpectedVersion, auth.FromContext(r.Context()).Subject)
	if err != nil {
		knowledgeErr(w, err)
		return
	}
	writeJSON(w, 200, d)
}
func (s *Server) handleDocumentDelete(w http.ResponseWriter, r *http.Request) {
	if !s.knowledgeReady(w) {
		return
	}
	var req struct {
		ExpectedVersion *int `json:"expected_version"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.ExpectedVersion == nil || *req.ExpectedVersion < 1 {
		writeErr(w, 400, "expected_version must be positive")
		return
	}
	if err := s.opt.Knowledge.Delete(r.PathValue("id"), *req.ExpectedVersion); err != nil {
		knowledgeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) handleDocumentSearch(w http.ResponseWriter, r *http.Request) {
	if !s.knowledgeReady(w) {
		return
	}
	limit := 5
	if v := r.URL.Query().Get("limit"); v != "" {
		var err error
		limit, err = strconv.Atoi(v)
		if err != nil {
			writeErr(w, 400, "invalid limit")
			return
		}
	}
	hits, err := s.opt.Knowledge.Search(strings.TrimSpace(r.URL.Query().Get("q")), documentReader(r), limit)
	if err != nil {
		knowledgeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"citations": hits})
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package auth

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zyvorai/zyntra/internal/marking"
)

func TestSessionCookieCarriesAndProtectsClearance(t *testing.T) {
	a := New("k", "")
	id, err := newIdentity("ann", "password", RoleViewer).WithClearance("cui")
	if err != nil {
		t.Fatal(err)
	}
	cookie, _ := a.Mint(time.Hour, id)
	got, ok := a.Valid(cookie)
	if !ok || got.Clearance != "cui" || got.Subject != "ann" {
		t.Fatalf("round trip: %+v %v", got, ok)
	}

	// Raising, changing or dropping the clearance breaks the signature.
	parts := strings.Split(cookie, ".")
	for _, swap := range []string{"cui-specified", "unclassified", "-"} {
		forged := append([]string(nil), parts...)
		forged[4] = swap
		if _, ok := a.Valid(strings.Join(forged, ".")); ok {
			t.Errorf("a cookie with clearance %q was accepted", swap)
		}
	}

	// No clearance round-trips as none.
	plain, _ := a.Mint(time.Hour, newIdentity("root", "key", RoleAdmin))
	if got, ok := a.Valid(plain); !ok || got.Clearance != "" {
		t.Errorf("plain identity: %+v %v", got, ok)
	}

	// Tenant and clearance together, and a dotted operator.
	both, _ := newIdentity("ann.smith@x.io", "password", RoleViewer).WithTenant("alpha")
	both, _ = both.WithClearance("cui")
	c2, _ := a.Mint(time.Hour, both)
	if got, ok := a.Valid(c2); !ok || got.Subject != "ann.smith@x.io" || got.Tenant != "alpha" || got.Clearance != "cui" {
		t.Errorf("tenant + clearance: %+v %v", got, ok)
	}
}

func TestOldSessionCookiesNoLongerValidate(t *testing.T) {
	a := New("k", "")
	// A v3 cookie: exp.method.roles.tenant.operator.sig, signed under the old version.
	exp := time.Now().Add(time.Hour).Unix()
	old := a.mac("zyntra-session.v3", strconv.FormatInt(exp, 10), "password", "viewer", "-", "ann")
	cookie := strings.Join([]string{strconv.FormatInt(exp, 10), "password", "viewer", "-", "ann", old}, ".")
	if _, ok := a.Valid(cookie); ok {
		t.Fatal("a pre-clearance cookie must ask for sign-in again")
	}
}

func TestWithClearanceRejectsBadNames(t *testing.T) {
	for _, bad := range []string{"CUI", "a.b", "a b", "-x", strings.Repeat("a", 41)} {
		if _, err := newIdentity("u", "password", RoleViewer).WithClearance(bad); err == nil {
			t.Errorf("clearance %q accepted", bad)
		}
	}
	if _, err := newIdentity("u", "password", RoleViewer).WithClearance(""); err != nil {
		t.Error("an empty clearance means none")
	}
}

func TestParseClearanceMapAndHighest(t *testing.T) {
	m, err := ParseClearanceMap("analysts=cui; leads=cui-specified")
	if err != nil || m["analysts"] != "cui" || m["leads"] != "cui-specified" {
		t.Fatalf("%v %v", m, err)
	}
	for _, bad := range []string{"analysts", "=cui", "a=B"} {
		if _, err := ParseClearanceMap(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	s := &marking.Scheme{Levels: []string{"unclassified", "cui", "cui-specified"}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := s.Highest(m["analysts"], m["leads"]); got != "cui-specified" {
		t.Errorf("a user in two groups gets the highest level, got %q", got)
	}
}

func TestOIDCClearanceMapNeedsAValidScheme(t *testing.T) {
	a := New("k", "")
	base := OIDCConfig{Issuer: "http://127.0.0.1:1", ClientID: "c", ClearanceMap: map[string]string{"g": "cui"}}
	if err := a.EnableOIDC(t.Context(), base); err == nil || !strings.Contains(err.Error(), "markings") {
		t.Errorf("a clearance map without a scheme: %v", err)
	}
	s := &marking.Scheme{Levels: []string{"unclassified", "cui"}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	base.Scheme = s
	base.ClearanceMap = map[string]string{"g": "secret"}
	if err := a.EnableOIDC(t.Context(), base); err == nil || !strings.Contains(err.Error(), "unknown level") {
		t.Errorf("a clearance map naming an unknown level: %v", err)
	}
}

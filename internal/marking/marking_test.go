// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package marking

import "testing"

func scheme(t *testing.T) *Scheme {
	t.Helper()
	s := &Scheme{Levels: []string{"unclassified", "cui", "cui-specified"}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDominates(t *testing.T) {
	s := scheme(t)
	cases := []struct {
		clearance, marking string
		want               bool
	}{
		{"cui", "unclassified", true},
		{"cui", "cui", true},
		{"cui", "cui-specified", false},
		{"", "", true},              // default clearance sees unmarked data
		{"", "cui", false},          // no clearance sees only the default marking
		{"cui-specified", "", true}, // unmarked is the default marking
		{"cui", "secret", false},    // an unknown marking is hidden, not shown
		{"secret", "unclassified", false},
	}
	for _, c := range cases {
		if got := s.Dominates(c.clearance, c.marking); got != c.want {
			t.Errorf("Dominates(%q, %q) = %v, want %v", c.clearance, c.marking, got, c.want)
		}
	}
}

func TestNilSchemeShowsEverything(t *testing.T) {
	var s *Scheme
	if !s.Dominates("", "anything") || !s.Known("whatever") {
		t.Fatal("markings are off with no scheme")
	}
}

func TestValidate(t *testing.T) {
	bad := []Scheme{
		{},
		{Levels: []string{"only"}},
		{Levels: []string{"a", "a"}},
		{Levels: []string{"a", "B"}},
		{Levels: []string{"a", "b.c"}},
		{Levels: []string{"a", "b"}, Default: "z"},
	}
	for i, s := range bad {
		if err := s.Validate(); err == nil {
			t.Errorf("scheme %d should be rejected", i)
		}
	}
	good := Scheme{Levels: []string{"low", "high"}}
	if err := good.Validate(); err != nil || good.Default != "low" {
		t.Fatalf("default = %q err=%v", good.Default, err)
	}
}

func TestHighestAndMax(t *testing.T) {
	s := scheme(t)
	if got := s.Highest("unclassified", "cui", "nope"); got != "cui" {
		t.Errorf("Highest = %q", got)
	}
	if got := s.Highest("nope"); got != "" {
		t.Errorf("Highest of unknowns = %q", got)
	}
	if got := s.Max("", "cui"); got != "cui" {
		t.Errorf("Max = %q", got)
	}
	if got := s.Max("cui-specified", "cui"); got != "cui-specified" {
		t.Errorf("Max = %q", got)
	}
	if got := s.Max("cui", "mystery"); got != "mystery" {
		t.Errorf("an unknown marking must win so it stays hidden, got %q", got)
	}
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package marking defines ordered data markings (for example unclassified,
// cui, cui-specified) and the "may this clearance see that marking" test.
//
// It enforces nothing by itself: callers hold a *Scheme and ask Dominates. A
// nil scheme means markings are off and everything is visible, which keeps
// every existing deployment unchanged. When a scheme is set, anything that
// cannot be decided (an unknown marking or clearance) is not visible.
package marking

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
)

// Pattern is what a level name looks like. It has no dots, so a level can sit
// inside the dotted session cookie.
var Pattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

// Scheme is the markings block of the policy file.
type Scheme struct {
	// Levels run from least to most restricted.
	Levels []string `yaml:"levels" json:"levels"`
	// Default is the marking of anything unmarked, and the clearance of an
	// identity with none. Empty means the first (lowest) level.
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
}

// Validate checks the scheme and fills in the default.
func (s *Scheme) Validate() error {
	if s == nil {
		return nil
	}
	if len(s.Levels) < 2 {
		return errors.New("markings: levels needs at least two entries, least restricted first")
	}
	seen := map[string]bool{}
	for _, l := range s.Levels {
		if !Pattern.MatchString(l) {
			return fmt.Errorf("markings: level %q must be lowercase letters, digits, - or _", l)
		}
		if seen[l] {
			return fmt.Errorf("markings: level %q is listed twice", l)
		}
		seen[l] = true
	}
	if s.Default == "" {
		s.Default = s.Levels[0]
	}
	if !seen[s.Default] {
		return fmt.Errorf("markings: default %q is not one of the levels", s.Default)
	}
	return nil
}

// Rank returns a level's position, 0 being the least restricted.
func (s *Scheme) Rank(level string) (int, bool) {
	if s == nil {
		return 0, false
	}
	i := slices.Index(s.Levels, level)
	return i, i >= 0
}

// Known reports whether level is part of the scheme. Empty counts as known
// (it means the default).
func (s *Scheme) Known(level string) bool {
	if s == nil || level == "" {
		return true
	}
	_, ok := s.Rank(level)
	return ok
}

func (s *Scheme) orDefault(level string) string {
	if level == "" {
		return s.Default
	}
	return level
}

// Dominates reports whether clearance may see data carrying marking. With no
// scheme it is always true. An empty marking is the default marking and an
// empty clearance the default clearance. Unknown names are never visible.
func (s *Scheme) Dominates(clearance, marking string) bool {
	if s == nil {
		return true
	}
	c, ok := s.Rank(s.orDefault(clearance))
	if !ok {
		return false
	}
	m, ok := s.Rank(s.orDefault(marking))
	if !ok {
		return false
	}
	return c >= m
}

// Highest returns the most restricted of the given levels; unknown ones are
// ignored. It returns "" if none are known.
func (s *Scheme) Highest(levels ...string) string {
	best, bestRank := "", -1
	for _, l := range levels {
		if r, ok := s.Rank(l); ok && r > bestRank {
			best, bestRank = l, r
		}
	}
	return best
}

// Max returns the more restricted of two markings, treating empty as the
// default. An unknown marking wins, so it stays hidden.
func (s *Scheme) Max(a, b string) string {
	if s == nil {
		return ""
	}
	a, b = s.orDefault(a), s.orDefault(b)
	ra, oka := s.Rank(a)
	rb, okb := s.Rank(b)
	switch {
	case !oka:
		return a
	case !okb:
		return b
	case rb > ra:
		return b
	}
	return a
}

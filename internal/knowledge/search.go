// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package knowledge

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

type Citation struct {
	Document  string  `json:"document"`
	Title     string  `json:"title"`
	Source    string  `json:"source"`
	Version   int     `json:"version"`
	Hash      string  `json:"sha256"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Excerpt   string  `json:"excerpt"`
	Score     float64 `json:"score"`
}
type chunk struct {
	doc        Document
	text       string
	start, end int
	terms      map[string]int
	n          int
}

var stopwords = map[string]bool{"a": true, "an": true, "the": true, "is": true, "are": true, "what": true, "how": true, "do": true, "does": true, "to": true, "in": true, "of": true, "for": true, "and": true, "or": true, "according": true, "document": true, "documents": true, "sop": false, "runbook": true, "policy": true, "please": true, "tell": true, "me": true, "about": true, "our": true, "we": true, "should": true}

func terms(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	out := []string{}
	for _, w := range words {
		if len(w) > 1 && !stopwords[w] {
			out = append(out, w)
		}
	}
	return out
}

// Chunks preserve source line numbers, with a 2,000-rune cap. Long lines
// become multiple chunks with the same line reference; no source text is lost.
func chunks(d Document) []chunk {
	out := []chunk{}
	lines := strings.Split(d.Text, "\n")
	buf := []rune{}
	start, end := 1, 1
	flush := func() {
		if len(buf) > 0 {
			text := string(buf)
			freq := map[string]int{}
			ws := terms(text)
			for _, w := range ws {
				freq[w]++
			}
			out = append(out, chunk{d, text, start, end, freq, len(ws)})
			buf = buf[:0]
		}
	}
	for i, line := range lines {
		piece := line
		if i < len(lines)-1 {
			piece += "\n"
		}
		for _, r := range []rune(piece) {
			if len(buf) == 0 {
				start = i + 1
			}
			end = i + 1
			buf = append(buf, r)
			if len(buf) >= 2000 {
				flush()
			}
		}
		if i+1-start >= 29 {
			flush()
		}
	}
	flush()
	return out
}

// Search computes BM25 over visible chunks only. Hidden documents do not
// influence term frequencies, ranking, totals or snippets. Queries are terms,
// not SQL, regexes, model prompts or tool instructions.
func (s *Store) Search(q string, r Reader, limit int) ([]Citation, error) {
	if len(q) == 0 || len(q) > 2000 {
		return nil, fmt.Errorf("%w: query must be 1-2000 bytes", ErrInvalid)
	}
	if limit < 1 || limit > 10 {
		return nil, fmt.Errorf("%w: limit must be 1-10", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	docs, err := s.visible(r)
	if err != nil {
		return nil, err
	}
	cs := []chunk{}
	for _, d := range docs {
		cs = append(cs, chunks(d)...)
	}
	query := map[string]bool{}
	for _, w := range terms(q) {
		query[w] = true
	}
	df := map[string]int{}
	total := 0
	for _, c := range cs {
		total += c.n
		for w := range query {
			if c.terms[w] > 0 {
				df[w]++
			}
		}
	}
	out := []Citation{}
	if len(cs) == 0 || total == 0 {
		return out, nil
	}
	avg := float64(total) / float64(len(cs))
	for _, c := range cs {
		score := 0.0
		for w := range query {
			tf := float64(c.terms[w])
			if tf == 0 {
				continue
			}
			idf := math.Log(1 + (float64(len(cs)-df[w])+.5)/(float64(df[w])+.5))
			score += idf * tf * 2.2 / (tf + 1.2*(.25+.75*float64(c.n)/avg))
		}
		if score > 0 {
			out = append(out, Citation{c.doc.ID, c.doc.Title, c.doc.Source, c.doc.Version, c.doc.Hash, c.start, c.end, c.text, score})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Document != out[j].Document {
			return out[i].Document < out[j].Document
		}
		if out[i].StartLine != out[j].StartLine {
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].Excerpt < out[j].Excerpt
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

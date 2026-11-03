// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package ontology

import (
	"fmt"
	"sort"
	"strings"
)

// DBColumn is one column of a table found by introspection.
type DBColumn struct {
	Name string
	Type string // the database's own type name
}

// DBTable is the shape of one table: its columns, its primary key columns in
// order, and its single-column foreign keys (column -> referenced table).
type DBTable struct {
	Name    string
	Columns []DBColumn
	PK      []string
	FKs     map[string]string
}

// SuggestOptions says how a draft is built.
type SuggestOptions struct {
	Driver    string // postgres or sqlite
	DSNEnv    string // environment variable the connectors read the DSN from
	Namespace string // object namespace, default "db"
	Interval  string // connector interval, default "5m"
}

// Suggest drafts an ontology from a database's tables: one object type per
// table with a single-column primary key, one link type per foreign key, and
// one full-listing sql connector per table. It is a starting point for a
// person to edit, never applied by itself. The notes name everything it left
// out and why. The result has passed Definition.Validate.
func Suggest(tables []DBTable, o SuggestOptions) (*Definition, []string, error) {
	if o.Namespace == "" {
		o.Namespace = "db"
	}
	if o.Interval == "" {
		o.Interval = "5m"
	}
	if o.Driver != "postgres" && o.Driver != "sqlite" {
		return nil, nil, fmt.Errorf("suggest: driver must be postgres or sqlite")
	}
	if o.DSNEnv == "" {
		return nil, nil, fmt.Errorf("suggest: dsn_env is required")
	}
	ts := append([]DBTable(nil), tables...)
	sort.Slice(ts, func(i, j int) bool { return ts[i].Name < ts[j].Name })

	var notes []string
	typeOf := map[string]string{} // table -> object type
	used := map[string]bool{}
	var mapped []DBTable
	for _, t := range ts {
		switch {
		case len(t.PK) == 0:
			notes = append(notes, fmt.Sprintf("skipped table %s: no primary key", t.Name))
			continue
		case len(t.PK) > 1 && isJoinTable(t):
			notes = append(notes, fmt.Sprintf("skipped table %s: join table of %s (a many-to-many link; add it by hand)", t.Name, strings.Join(joinTargets(t), " and ")))
			continue
		case len(t.PK) > 1:
			notes = append(notes, fmt.Sprintf("skipped table %s: composite primary key (%s)", t.Name, strings.Join(t.PK, ", ")))
			continue
		}
		name := pascal(singular(t.Name))
		if name == "" || used[name] {
			name = pascal(t.Name)
		}
		for n := 2; used[name]; n++ {
			name = fmt.Sprintf("%s%d", pascal(t.Name), n)
		}
		used[name] = true
		typeOf[t.Name] = name
		mapped = append(mapped, t)
	}

	d := &Definition{}
	linkNames := map[string]bool{}
	for _, t := range mapped {
		ot := ObjectType{Name: typeOf[t.Name]}
		props := map[string]string{}
		taken := map[string]bool{}
		sel := []string{t.PK[0]}
		var links []LinkMap
		for _, c := range t.Columns {
			if c.Name == t.PK[0] {
				continue
			}
			if ref, ok := t.FKs[c.Name]; ok {
				to, mappedRef := typeOf[ref]
				if !mappedRef {
					notes = append(notes, fmt.Sprintf("table %s: foreign key %s points at %s, which is not mapped; kept as a plain property", t.Name, c.Name, ref))
				} else {
					ln := uniqueLink(linkNames, snake(singular(t.Name))+"_"+strings.TrimSuffix(strings.TrimSuffix(snake(c.Name), "_id"), "id"))
					d.Links = append(d.Links, LinkType{Name: ln, From: ot.Name, To: to})
					links = append(links, LinkMap{Type: ln, Column: c.Name, To: to, Namespace: o.Namespace})
					sel = append(sel, c.Name)
					continue
				}
			}
			p := snake(c.Name)
			for n := 2; p == "" || taken[p]; n++ {
				p = fmt.Sprintf("%s_%d", snake(c.Name), n)
			}
			taken[p] = true
			ot.Properties = append(ot.Properties, Property{Name: p, Type: propType(c.Type)})
			props[p] = c.Name
			sel = append(sel, c.Name)
		}
		d.Objects = append(d.Objects, ot)
		d.Connectors = append(d.Connectors, ConnectorSpec{
			Name:     o.Namespace + "-" + t.Name,
			Kind:     "sql",
			Driver:   o.Driver,
			DSNEnv:   o.DSNEnv,
			Query:    "SELECT " + quoteList(sel) + " FROM " + quoteIdent(t.Name),
			Interval: o.Interval,
			Prune:    true,
			Mapping:  &Mapping{Type: ot.Name, Namespace: o.Namespace, Key: t.PK[0], Props: props, Links: links},
		})
	}
	if len(d.Objects) == 0 {
		return nil, notes, fmt.Errorf("suggest: no table has a single-column primary key, nothing to map")
	}
	// A link must name an existing type; Validate also catches the rest.
	if err := d.Validate(); err != nil {
		return nil, notes, fmt.Errorf("suggest: the draft is not valid (this is a bug): %w", err)
	}
	return d, notes, nil
}

// isJoinTable reports a table whose every column is a foreign key.
func isJoinTable(t DBTable) bool {
	for _, c := range t.Columns {
		if _, ok := t.FKs[c.Name]; !ok {
			return false
		}
	}
	return len(t.Columns) > 0
}

func joinTargets(t DBTable) []string {
	var r []string
	for _, c := range t.Columns {
		r = append(r, t.FKs[c.Name])
	}
	return r
}

func propType(sqlType string) string {
	t := strings.ToLower(sqlType)
	switch {
	case strings.Contains(t, "bool"):
		return "bool"
	case strings.Contains(t, "timestamp"), strings.Contains(t, "date"), t == "time", strings.HasPrefix(t, "time "):
		return "time"
	case strings.Contains(t, "int"), strings.Contains(t, "serial"), strings.Contains(t, "numeric"),
		strings.Contains(t, "decimal"), strings.Contains(t, "real"), strings.Contains(t, "float"),
		strings.Contains(t, "double"), strings.Contains(t, "money"), t == "number":
		return "number"
	}
	return "string"
}

func uniqueLink(seen map[string]bool, base string) string {
	name := base
	for n := 2; seen[name]; n++ {
		name = fmt.Sprintf("%s_%d", base, n)
	}
	seen[name] = true
	return name
}

// singular drops a plural ending: "orders" -> "order", "categories" ->
// "category". Words it is unsure of are left alone.
func singular(s string) string {
	l := strings.ToLower(s)
	switch {
	case strings.HasSuffix(l, "ies") && len(s) > 3:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(l, "sses"), strings.HasSuffix(l, "ss"), strings.HasSuffix(l, "us"), strings.HasSuffix(l, "is"):
		return s
	case strings.HasSuffix(l, "ses"), strings.HasSuffix(l, "xes"), strings.HasSuffix(l, "ches"), strings.HasSuffix(l, "shes"):
		return s[:len(s)-2]
	case strings.HasSuffix(l, "s") && len(s) > 1:
		return s[:len(s)-1]
	}
	return s
}

// words splits an identifier on anything that is not a letter or digit and
// on lower-to-upper case changes.
func words(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = nil
		}
	}
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			cur = append(cur, r)
		case r >= 'A' && r <= 'Z':
			if i > 0 && rs[i-1] >= 'a' && rs[i-1] <= 'z' {
				flush()
			}
			cur = append(cur, r)
		default:
			flush()
		}
	}
	flush()
	return out
}

func pascal(s string) string {
	var b strings.Builder
	for _, w := range words(s) {
		b.WriteString(strings.ToUpper(w[:1]))
		b.WriteString(w[1:])
	}
	return b.String()
}

func snake(s string) string { return strings.Join(words(s), "_") }

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func quoteList(cols []string) string {
	q := make([]string, len(cols))
	for i, c := range cols {
		q[i] = quoteIdent(c)
	}
	return strings.Join(q, ", ")
}

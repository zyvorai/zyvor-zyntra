// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/zyvorai/zyntra/internal/ontology"
)

// Introspect reads table, primary key and single-column foreign key metadata
// from a postgres or sqlite database, in a read-only transaction. schema is
// the postgres schema (default "public") and is ignored for sqlite.
func Introspect(ctx context.Context, db *sql.DB, driver, schema string) ([]ontology.DBTable, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	switch driver {
	case "sqlite":
		return introspectSQLite(ctx, tx)
	case "postgres":
		if schema == "" {
			schema = "public"
		}
		return introspectPostgres(ctx, tx, schema)
	}
	return nil, fmt.Errorf("introspect: driver must be postgres or sqlite")
}

func introspectSQLite(ctx context.Context, tx *sql.Tx) ([]ontology.DBTable, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []ontology.DBTable
	for _, n := range names {
		q := `"` + strings.ReplaceAll(n, `"`, `""`) + `"`
		t := ontology.DBTable{Name: n, FKs: map[string]string{}}
		cr, err := tx.QueryContext(ctx, "PRAGMA table_info("+q+")")
		if err != nil {
			return nil, err
		}
		pk := map[int]string{}
		for cr.Next() {
			var cid, notnull, pkpos int
			var name, typ string
			var dflt sql.NullString
			if err := cr.Scan(&cid, &name, &typ, &notnull, &dflt, &pkpos); err != nil {
				cr.Close()
				return nil, err
			}
			t.Columns = append(t.Columns, ontology.DBColumn{Name: name, Type: typ})
			if pkpos > 0 {
				pk[pkpos] = name
			}
		}
		cr.Close()
		if err := cr.Err(); err != nil {
			return nil, err
		}
		for i := 1; i <= len(pk); i++ {
			t.PK = append(t.PK, pk[i])
		}
		fr, err := tx.QueryContext(ctx, "PRAGMA foreign_key_list("+q+")")
		if err != nil {
			return nil, err
		}
		count := map[int]int{}
		type fk struct{ from, table string }
		byID := map[int]fk{}
		for fr.Next() {
			var id, seq int
			var table, from string
			var to, onUpdate, onDelete, match sql.NullString
			if err := fr.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
				fr.Close()
				return nil, err
			}
			count[id]++
			byID[id] = fk{from, table}
		}
		fr.Close()
		if err := fr.Err(); err != nil {
			return nil, err
		}
		for id, f := range byID {
			if count[id] == 1 { // composite keys are left for a person
				t.FKs[f.from] = f.table
			}
		}
		out = append(out, t)
	}
	return out, nil
}

func introspectPostgres(ctx context.Context, tx *sql.Tx, schema string) ([]ontology.DBTable, error) {
	byName := map[string]*ontology.DBTable{}
	var order []string
	cr, err := tx.QueryContext(ctx, `
SELECT c.table_name, c.column_name, c.data_type
FROM information_schema.columns c
JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
WHERE c.table_schema = $1 AND t.table_type = 'BASE TABLE'
ORDER BY c.table_name, c.ordinal_position`, schema)
	if err != nil {
		return nil, err
	}
	for cr.Next() {
		var tn, cn, ty string
		if err := cr.Scan(&tn, &cn, &ty); err != nil {
			cr.Close()
			return nil, err
		}
		t, ok := byName[tn]
		if !ok {
			t = &ontology.DBTable{Name: tn, FKs: map[string]string{}}
			byName[tn] = t
			order = append(order, tn)
		}
		t.Columns = append(t.Columns, ontology.DBColumn{Name: cn, Type: ty})
	}
	cr.Close()
	if err := cr.Err(); err != nil {
		return nil, err
	}

	pr, err := tx.QueryContext(ctx, `
SELECT c.relname, a.attname
FROM pg_constraint k
JOIN pg_class c ON c.oid = k.conrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN LATERAL unnest(k.conkey) WITH ORDINALITY u(attnum, ord) ON true
JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = u.attnum
WHERE k.contype = 'p' AND n.nspname = $1
ORDER BY c.relname, u.ord`, schema)
	if err != nil {
		return nil, err
	}
	for pr.Next() {
		var tn, cn string
		if err := pr.Scan(&tn, &cn); err != nil {
			pr.Close()
			return nil, err
		}
		if t := byName[tn]; t != nil {
			t.PK = append(t.PK, cn)
		}
	}
	pr.Close()
	if err := pr.Err(); err != nil {
		return nil, err
	}

	fr, err := tx.QueryContext(ctx, `
SELECT c.relname, a.attname, rc.relname
FROM pg_constraint k
JOIN pg_class c ON c.oid = k.conrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_class rc ON rc.oid = k.confrelid
JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = k.conkey[1]
WHERE k.contype = 'f' AND array_length(k.conkey, 1) = 1 AND n.nspname = $1`, schema)
	if err != nil {
		return nil, err
	}
	for fr.Next() {
		var tn, cn, ref string
		if err := fr.Scan(&tn, &cn, &ref); err != nil {
			fr.Close()
			return nil, err
		}
		if t := byName[tn]; t != nil {
			t.FKs[cn] = ref
		}
	}
	fr.Close()
	if err := fr.Err(); err != nil {
		return nil, err
	}

	out := make([]ontology.DBTable, 0, len(order))
	for _, n := range order {
		out = append(out, *byName[n])
	}
	return out, nil
}

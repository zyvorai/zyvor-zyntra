// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package connector

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/zyvorai/zyntra/internal/ontology"
)

func TestIntrospectSQLiteFeedsSuggest(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, customer_id INTEGER REFERENCES customers(id), total REAL)`,
		`CREATE TABLE pairs (a INTEGER, b INTEGER, PRIMARY KEY (a, b))`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	tables, err := Introspect(context.Background(), db, "sqlite", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 3 {
		t.Fatalf("tables: %+v", tables)
	}
	d, notes, err := ontology.Suggest(tables, ontology.SuggestOptions{Driver: "sqlite", DSNEnv: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Objects) != 2 || len(d.Links) != 1 || d.Links[0].To != "Customer" || len(notes) != 1 {
		t.Fatalf("%+v %v", d, notes)
	}
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package knowledge stores versioned text documents and retrieves evidence
// only after applying the asking principal's visibility and role restrictions.
package knowledge

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound = errors.New("document not found")
	ErrConflict = errors.New("document version changed; reload before editing")
	ErrInvalid  = errors.New("invalid document")
	ErrCapacity = errors.New("document capacity exceeded")
)

const MaxText = 128 << 10

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,79}$`)
var tenantPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

type Document struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Source     string    `json:"source"`
	Text       string    `json:"text,omitempty"`
	Visibility string    `json:"visibility"` // provider | shared | tenant
	Tenant     string    `json:"tenant,omitempty"`
	Roles      []string  `json:"roles"`
	Version    int       `json:"version"`
	Hash       string    `json:"sha256"`
	UpdatedAt  time.Time `json:"updated_at"`
	UpdatedBy  string    `json:"updated_by"`
}
type Reader struct {
	Tenant string
	Roles  []string
	Admin  bool
}

func (r Reader) CanRead(d Document) bool {
	if !r.Admin && len(r.Roles) == 0 {
		return false
	}
	switch d.Visibility {
	case "tenant":
		if r.Tenant != d.Tenant && !(r.Admin && r.Tenant == "") {
			return false
		}
	case "provider":
		if r.Tenant != "" {
			return false
		}
	case "shared":
	default:
		return false
	}
	if r.Admin {
		return true
	}
	if len(d.Roles) == 0 {
		return true
	}
	for _, have := range r.Roles {
		for _, want := range d.Roles {
			if have == want {
				return true
			}
		}
	}
	return false
}

type Store struct {
	mu sync.Mutex
	db *sql.DB
}

func Open(path string) (*Store, error) {
	dsn := ":memory:"
	if path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return nil, err
		}

		f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if e != nil {
			return nil, e
		}
		f.Close()
		if e = os.Chmod(path, 0600); e != nil {
			return nil, e
		}
		u := url.URL{Scheme: "file", Path: path}
		dsn = u.String() + "?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS documents(id TEXT PRIMARY KEY,version INTEGER NOT NULL,body BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS tombstones(id TEXT PRIMARY KEY);
 CREATE TABLE IF NOT EXISTS revisions(id TEXT NOT NULL,version INTEGER NOT NULL,body BLOB NOT NULL,PRIMARY KEY(id,version));`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func validate(d *Document) error {
	d.Title = strings.TrimSpace(d.Title)
	d.Source = strings.TrimSpace(d.Source)
	if !idPattern.MatchString(d.ID) || len(d.Title) == 0 || len(d.Title) > 200 || len(d.Source) > 1024 || len(d.Text) == 0 || len(d.Text) > MaxText || !utf8.ValidString(d.Text) || strings.IndexByte(d.Text, 0) >= 0 {
		return fmt.Errorf("%w: id/title/source/text limits", ErrInvalid)
	}
	if d.Visibility == "" {
		d.Visibility = "provider"
	}
	if d.Visibility != "provider" && d.Visibility != "shared" && d.Visibility != "tenant" {
		return fmt.Errorf("%w: visibility must be provider, shared or tenant", ErrInvalid)
	}
	if (d.Visibility == "tenant" && !tenantPattern.MatchString(d.Tenant)) || (d.Visibility != "tenant" && d.Tenant != "") {
		return fmt.Errorf("%w: only tenant documents must name a tenant", ErrInvalid)
	}
	if len(d.Roles) > 5 {
		return fmt.Errorf("%w: too many roles", ErrInvalid)
	}
	seen := map[string]bool{}
	for _, r := range d.Roles {
		if seen[r] || !(r == "viewer" || r == "proposer" || r == "approver" || r == "executor" || r == "admin") {
			return fmt.Errorf("%w: unknown or duplicate reader role", ErrInvalid)
		}
		seen[r] = true
	}
	return nil
}

// Put requires the current version (zero for creation). ACL edits are versioned
// alongside content so a citation always identifies an immutable source body.
func (s *Store) Put(d Document, expected int, by string) (Document, error) {
	if err := validate(&d); err != nil {
		return Document{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return Document{}, err
	}
	defer tx.Rollback()
	var current int
	err = tx.QueryRow("SELECT version FROM documents WHERE id=?", d.ID).Scan(&current)
	if err != nil && err != sql.ErrNoRows {
		return Document{}, err
	}
	// IDs remain reserved after deletion, avoiding reuse of citation versions.
	if err == sql.ErrNoRows {
		var archived int
		if e := tx.QueryRow("SELECT count(*) FROM tombstones WHERE id=?", d.ID).Scan(&archived); e != nil {
			return Document{}, e
		}
		if archived > 0 {
			return Document{}, ErrConflict
		}
	}
	if current != expected || expected < 0 {
		return Document{}, ErrConflict
	}
	var count, bytes int
	if err = tx.QueryRow("SELECT count(*),coalesce(sum(length(body)),0) FROM documents WHERE id<>?", d.ID).Scan(&count, &bytes); err != nil {
		return Document{}, err
	}
	if count >= 500 {
		return Document{}, ErrCapacity
	}
	d.Version = current + 1
	d.UpdatedAt = time.Now().UTC()
	d.UpdatedBy = by
	hash := sha256.Sum256([]byte(d.Text))
	d.Hash = hex.EncodeToString(hash[:])
	if d.Roles == nil {
		d.Roles = []string{}
	}
	body, err := json.Marshal(d)
	if err != nil {
		return Document{}, err
	}
	if bytes+len(body) > 10<<20 {
		return Document{}, ErrCapacity
	}
	if _, err = tx.Exec("INSERT OR REPLACE INTO documents(id,version,body) VALUES(?,?,?)", d.ID, d.Version, body); err != nil {
		return Document{}, err
	}
	if _, err = tx.Exec("INSERT INTO revisions(id,version,body) VALUES(?,?,?)", d.ID, d.Version, body); err != nil {
		return Document{}, err
	}
	if _, err = tx.Exec("DELETE FROM revisions WHERE id=? AND version<=?", d.ID, d.Version-10); err != nil {
		return Document{}, err
	}
	if err = tx.Commit(); err != nil {
		return Document{}, err
	}
	return d, nil
}
func (s *Store) visible(r Reader) ([]Document, error) {
	rows, err := s.db.Query("SELECT body FROM documents ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		d, e := decodeDocument(b)
		if e != nil {
			err = e
			return nil, err
		}
		if r.CanRead(d) {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}
func (s *Store) List(r Reader) ([]Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	docs, err := s.visible(r)
	for i := range docs {
		docs[i].Text = ""
	}
	return docs, err
}
func (s *Store) Get(id string, version int, r Reader) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b []byte
	if err := s.db.QueryRow("SELECT body FROM documents WHERE id=?", id).Scan(&b); err == sql.ErrNoRows {
		return Document{}, ErrNotFound
	} else if err != nil {
		return Document{}, err
	}
	current, err := decodeDocument(b)
	if err != nil {
		return Document{}, err
	}
	if !r.CanRead(current) {
		return Document{}, ErrNotFound
	}
	if version == 0 || version == current.Version {
		return current, nil
	}
	if err := s.db.QueryRow("SELECT body FROM revisions WHERE id=? AND version=?", id, version).Scan(&b); err == sql.ErrNoRows {
		return Document{}, ErrNotFound
	} else if err != nil {
		return Document{}, err
	}
	old, err := decodeDocument(b)
	if err != nil {
		return Document{}, err
	}
	if !r.CanRead(old) {
		return Document{}, ErrNotFound
	}
	return old, nil
}
func (s *Store) Delete(id string, expected int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec("DELETE FROM documents WHERE id=? AND version=?", id, expected)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrConflict
	}
	if _, err = tx.Exec("DELETE FROM revisions WHERE id=?", id); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO tombstones(id) VALUES(?)", id); err != nil {
		return err
	}
	return tx.Commit()
}

func decodeDocument(body []byte) (Document, error) {
	var d Document
	if err := json.Unmarshal(body, &d); err != nil {
		return Document{}, err
	}
	hash := sha256.Sum256([]byte(d.Text))
	if d.Version < 1 || d.Hash != hex.EncodeToString(hash[:]) {
		return Document{}, errors.New("document integrity check failed")
	}
	return d, nil
}

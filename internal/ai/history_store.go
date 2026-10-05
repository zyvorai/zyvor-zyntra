// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package ai

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

// OpenHistory keeps up to 43,200 observations per KPI, at most one per minute.
// Each refresh commits atomically before updating the bounded in-memory view.
// A pack gets a separate database under its existing state directory.
func OpenHistory(dir string) (*History, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: filepath.Join(dir, "analytics.sqlite")}
	db, err := sql.Open("sqlite", u.String()+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(FULL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*History, error) { db.Close(); return nil, fmt.Errorf("analytics history: %w", err) }
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS samples(kpi TEXT NOT NULL, at INTEGER NOT NULL, value REAL NOT NULL, PRIMARY KEY(kpi,at));`); err != nil {
		return fail(err)
	}
	h := NewHistory(43200)
	h.db, h.interval = db, time.Minute
	rows, err := db.Query(`SELECT kpi,at,value FROM samples ORDER BY kpi,at`)
	if err != nil {
		return fail(err)
	}
	for rows.Next() {
		var id string
		var at int64
		var v float64
		if err = rows.Scan(&id, &at, &v); err != nil {
			rows.Close()
			return fail(err)
		}
		h.data[id] = append(h.data[id], Point{time.Unix(0, at).UTC(), v})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return fail(err)
	}
	for id, pts := range h.data {
		if len(pts) > h.max {
			h.data[id] = pts[len(pts)-h.max:]
		}
	}
	// Import the old JSON only into an empty database. Keep it as a backup.
	if len(h.data) == 0 {
		b, e := os.ReadFile(filepath.Join(dir, "history.json"))
		if e != nil && !os.IsNotExist(e) {
			return fail(e)
		}
		if e == nil {
			var legacy map[string][]Point
			if e = json.Unmarshal(b, &legacy); e != nil {
				return fail(fmt.Errorf("legacy history: %w", e))
			}
			h.Restore(legacy)
			tx, e := db.Begin()
			if e != nil {
				return fail(e)
			}
			for id, pts := range h.data {
				for _, pt := range pts {
					if _, e = tx.Exec("INSERT OR IGNORE INTO samples(kpi,at,value) VALUES(?,?,?)", id, pt.T.UnixNano(), pt.V); e != nil {
						tx.Rollback()
						return fail(e)
					}
				}
			}
			if e = tx.Commit(); e != nil {
				return fail(e)
			}
		}
	}
	return h, nil
}

func cleanPoints(pts []Point) []Point {
	out := make([]Point, 0, len(pts))
	for _, pt := range pts {
		if !pt.T.IsZero() && !math.IsNaN(pt.V) && !math.IsInf(pt.V, 0) {
			out = append(out, pt)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T.Before(out[j].T) })
	result := out[:0]
	for _, pt := range out {
		if len(result) > 0 && result[len(result)-1].T.Equal(pt.T) {
			result[len(result)-1] = pt
		} else {
			result = append(result, pt)
		}
	}
	return result
}

func (h *History) persist(batch map[string]Point) error {
	if h.db == nil || len(batch) == 0 {
		return nil
	}
	tx, err := h.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for id, pt := range batch {
		if _, err = tx.Exec("INSERT OR REPLACE INTO samples(kpi,at,value) VALUES(?,?,?)", id, pt.T.UnixNano(), pt.V); err != nil {
			return err
		}
		// The primary-key index bounds retention without scanning other KPIs.
		if _, err = tx.Exec(`DELETE FROM samples WHERE kpi=? AND at < COALESCE((SELECT at FROM samples WHERE kpi=? ORDER BY at DESC LIMIT 1 OFFSET ?),-9223372036854775808)`, id, id, h.max-1); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (h *History) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.db != nil {
		return h.db.Close()
	}
	return nil
}
func (h *History) Persistent() bool { h.mu.RLock(); defer h.mu.RUnlock(); return h.db != nil }
func (h *History) PersistenceError() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.persistError
}

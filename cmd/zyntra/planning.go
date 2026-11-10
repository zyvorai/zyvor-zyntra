// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/zyvorai/zyntra/internal/freshness"
	"github.com/zyvorai/zyntra/internal/planning"
)

func readPlanningJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 4<<20))
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("expected exactly one JSON object")
	}
	return nil
}
func runPlanning(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("planning: plan|capture|replay|review|process|benefits")
	}
	fs := flag.NewFlagSet("planning "+args[0], flag.ContinueOnError)
	var c common
	c.register(fs)
	request := fs.String("request", "", "JSON request file")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	switch args[0] {
	case "replay":
		var req struct {
			Snapshot planning.Snapshot `json:"snapshot"`
			Options  planning.Options  `json:"options"`
		}
		if err := readPlanningJSON(*request, &req); err != nil {
			return err
		}
		r, err := planning.Replay(ctx, req.Snapshot, req.Options)
		if err != nil {
			return err
		}
		return emit(out, r)
	case "review":
		var req planning.ReviewRequest
		if err := readPlanningJSON(*request, &req); err != nil {
			return err
		}
		r, err := planning.Compare(ctx, req)
		if err != nil {
			return err
		}
		return emit(out, r)
	case "process":
		var req struct {
			Events []planning.Event `json:"events"`
		}
		if err := readPlanningJSON(*request, &req); err != nil {
			return err
		}
		r, err := planning.Mine(req.Events)
		if err != nil {
			return err
		}
		return emit(out, r)
	case "benefits":
		var req planning.Experiment
		if err := readPlanningJSON(*request, &req); err != nil {
			return err
		}
		r, err := planning.Measure(req)
		if err != nil {
			return err
		}
		return emit(out, r)
	case "plan", "capture":
		m, err := c.load(ctx)
		if err != nil {
			return err
		}
		dir := c.file
		if st, err := os.Stat(dir); err == nil && !st.IsDir() {
			dir = filepath.Dir(dir)
		}
		var cfg planning.Config
		if err := readPlanningJSON(filepath.Join(dir, "planning.json"), &cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err = cfg.Validate(m); err != nil {
			return err
		}
		var o planning.Options
		if *request != "" {
			if err := readPlanningJSON(*request, &o); err != nil {
				return err
			}
		}
		o.At = time.Now().UTC()
		o.Unusable = freshness.Set(freshness.New(5 * time.Minute).States(m))
		if args[0] == "capture" {
			snap, err := planning.Capture(m, cfg, o.Unusable, o.At)
			if err != nil {
				return err
			}
			return emit(out, snap)
		}
		r, err := planning.Plan(ctx, m, cfg, o)
		if err != nil {
			return err
		}
		return emit(out, r)
	default:
		return fmt.Errorf("unknown planning command %q", args[0])
	}
}

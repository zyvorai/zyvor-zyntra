// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/zyvorai/zyntra/internal/executive"
	"github.com/zyvorai/zyntra/internal/gaps"
	"github.com/zyvorai/zyntra/internal/graph"
	"github.com/zyvorai/zyntra/internal/planner"
)

func runExecutive(out io.Writer, m *graph.Model, file, audience, decision string, asJSON bool) error {
	res, err := planner.PlanWith(m, planner.Options{})
	if err != nil {
		return err
	}
	in := executive.Input{
		Model: m, Plan: res, Gaps: gaps.Detect(m), Overlay: loadOverlay(file),
		Now: time.Now(), Audience: audience,
	}
	inbox := executive.BuildInbox(in)
	brief := executive.BuildBrief(in, decision)
	digest := executive.BuildDigest(in)
	if asJSON {
		return emit(out, map[string]any{"inbox": inbox, "brief": brief, "digest": digest, "conflicts": executive.Conflicts(in)})
	}
	fmt.Fprintf(out, "%s\n", digest.Headline)
	for _, line := range digest.Lines {
		fmt.Fprintf(out, "  %s\n", line)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "\nRANK\tOWNER\tDECISION\tNEXT")
	for _, item := range inbox.Items {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", item.Rank, item.Owner, item.Question, item.NextStep)
	}
	tw.Flush()
	fmt.Fprintf(out, "\nBrief: %s\n%s\n", brief.Question, brief.Narrative)
	fmt.Fprintln(out, "\nOPTION\tSTATUS\tIMPROVEMENT\tPESSIMISTIC")
	tw = tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, o := range brief.Options {
		fmt.Fprintf(tw, "%s\t%s\t%.3f\t%.3f\n", o.Name, o.Status, o.Improvement, o.Pessimistic)
	}
	return tw.Flush()
}

func loadOverlay(file string) executive.Overlay {
	dir := file
	if fi, err := os.Stat(file); err == nil && !fi.IsDir() {
		dir = filepath.Dir(file)
	}
	b, err := os.ReadFile(filepath.Join(dir, "executive.yaml"))
	if err != nil {
		return executive.Overlay{}
	}
	o, err := executive.ParseOverlay(b)
	if err != nil {
		return executive.Overlay{}
	}
	return o
}

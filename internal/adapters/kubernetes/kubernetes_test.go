// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package kubernetes

import "testing"

func TestParseNoReadyNodesReportsZero(t *testing.T) {
	inv, err := Parse([]byte(`{"items":[{"status":{"conditions":[{"type":"Ready","status":"False"}]}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := inv[MetricNodesReady]; !ok || v != 0 {
		t.Fatalf("nodes_ready = %v, %v; want 0, true", v, ok)
	}
	if inv[MetricNodesTotal] != 1 {
		t.Fatalf("nodes_total = %v", inv[MetricNodesTotal])
	}
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package policy

import (
	"strings"
	"testing"

	"github.com/zyvorai/zyntra/internal/egress"
	"github.com/zyvorai/zyntra/internal/graph"
)

const egressPolicy = `
egress:
  default: deny
  destinations:
    onprem: ["*.internal"]
  rules:
    - {allow: [onprem]}
`

func modelWith(webhookURL, sourceURL string) *graph.Model {
	m := &graph.Model{}
	m.Actions = append(m.Actions, graph.Action{ID: "notify", Webhook: &graph.Webhook{Method: "POST", URL: webhookURL}})
	m.KPIs = append(m.KPIs, graph.KPI{ID: "k", Source: &graph.Source{Kind: graph.SourceHTTP, URL: sourceURL}})
	return m
}

func TestEgressBlocksConfiguredURLs(t *testing.T) {
	p, err := Parse(egressPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if p.EgressGuard() == nil {
		t.Fatal("no guard built")
	}
	if err := p.CheckModel(modelWith("https://hooks.internal/x", "http://svc.internal/metrics")); err != nil {
		t.Fatalf("allowed URLs rejected: %v", err)
	}
	err = p.CheckModel(modelWith("https://hooks.example.com/x?token=s3cret", "http://svc.internal/m"))
	if err == nil || !strings.Contains(err.Error(), "action notify") || !strings.Contains(err.Error(), "hooks.example.com") {
		t.Fatalf("want a blocked webhook error naming the action and host, got %v", err)
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatal("the error leaked the URL query")
	}
	err = p.CheckModel(modelWith("https://hooks.internal/x", "https://public.example.com/data"))
	if err == nil || !strings.Contains(err.Error(), "kpi k") {
		t.Fatalf("want a blocked source error, got %v", err)
	}
}

// A destination group with networks cannot decide a host name before the
// connection is made, so the start-up check leaves names to the runtime guard.
func TestEgressNetworkGroupsDeferNamesToRuntime(t *testing.T) {
	p, err := Parse("egress: {default: deny, destinations: {n: [\"10.0.0.0/8\"]}, rules: [{allow: [n]}]}")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.CheckModel(modelWith("https://unknown.example/x", "http://10.1.1.1/m")); err != nil {
		t.Fatalf("a name must pass the static check: %v", err)
	}
	if err := p.CheckModel(modelWith("https://203.0.113.9/x", "http://10.1.1.1/m")); err == nil {
		t.Fatal("an IP outside the network must fail the static check")
	}
}

func TestEgressSkipsURLsWithUnsetVariables(t *testing.T) {
	p, err := Parse(egressPolicy)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZYNTRA_NOT_SET_FOR_TEST", "")
	if err := p.CheckModel(modelWith("${ZYNTRA_NOT_SET_FOR_TEST}/x", "${ZYNTRA_NOT_SET_FOR_TEST}/y")); err != nil {
		t.Fatalf("an unset variable cannot be checked yet: %v", err)
	}
}

func TestNoEgressBlockMeansNoGuard(t *testing.T) {
	p, err := Parse("rules: []")
	if err != nil {
		t.Fatal(err)
	}
	if p.EgressGuard() != nil {
		t.Fatal("a policy without egress must not restrict anything")
	}
	if err := p.CheckModel(modelWith("https://anywhere.example/x", "https://anywhere.example/y")); err != nil {
		t.Fatal(err)
	}
	var nilp *Policy
	if nilp.EgressGuard() != nil {
		t.Fatal("nil policy")
	}
}

func TestBadEgressConfigFailsLoad(t *testing.T) {
	for _, y := range []string{
		"egress: {default: maybe}",
		"egress: {rules: [{allow: [nope]}]}",
		"egress: {destinations: {x: [\"10.0.0.0/99\"]}}",
	} {
		if _, err := Parse(y); err == nil || !strings.Contains(err.Error(), "egress") {
			t.Errorf("%q: want an egress error, got %v", y, err)
		}
	}
	_ = egress.Webhook
}

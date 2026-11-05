// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package egress

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func mustNew(t *testing.T, c Config) *Guard {
	t.Helper()
	g, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestNilGuardAllowsEverything(t *testing.T) {
	var g *Guard
	if err := g.Check(context.Background(), Webhook, "anywhere.example"); err != nil {
		t.Fatal(err)
	}
	c := &http.Client{}
	if g.Client(c, Webhook) != c {
		t.Fatal("a nil guard must return the client unchanged")
	}
}

func TestRulesAndDefaults(t *testing.T) {
	g := mustNew(t, Config{
		Destinations: map[string][]string{
			"onprem": {"hooks.mil.example", "*.int.example"},
		},
		Rules: []Rule{{Purposes: []string{Webhook, AI}, Allow: []string{"onprem"}}},
	})
	cases := []struct {
		purpose, host string
		ok            bool
	}{
		{Webhook, "hooks.mil.example", true},
		{Webhook, "HOOKS.mil.example:8443", true},
		{Webhook, "a.int.example", true},
		{Webhook, "int.example", false}, // a wildcard covers subdomains only
		{Webhook, "evil.example", false},
		{Webhook, "192.0.2.1", false},
		{AI, "evil.example", false},
		{Source, "evil.example", true}, // no rule covers sources and the default is allow
	}
	for _, c := range cases {
		err := g.Static(c.purpose, c.host)
		if (err == nil) != c.ok {
			t.Errorf("%s -> %s: err=%v, want ok=%v", c.purpose, c.host, err, c.ok)
		}
	}

	deny := mustNew(t, Config{
		Default:      "deny",
		Destinations: map[string][]string{"x": {"a.example"}},
		Rules:        []Rule{{Purposes: []string{Webhook}, Allow: []string{"x"}}},
	})
	if deny.Static(Webhook, "a.example") != nil {
		t.Error("a.example should be allowed for webhooks")
	}
	if deny.Static(Source, "a.example") == nil {
		t.Error("the deny default should block a purpose no rule covers")
	}
}

func TestIPsAndNetworks(t *testing.T) {
	g := mustNew(t, Config{
		Destinations: map[string][]string{"n": {"10.1.2.3", "172.16.0.0/12"}},
		Rules:        []Rule{{Allow: []string{"n"}}},
	})
	for host, ok := range map[string]bool{"10.1.2.3": true, "10.1.2.4": false, "172.20.1.1": true, "[::1]:80": false} {
		if err := g.Static(Webhook, host); (err == nil) != ok {
			t.Errorf("%s: err=%v, want ok=%v", host, err, ok)
		}
	}
	// A name is decided when the connection is made, so Static lets it pass
	// and Check resolves it.
	g.lookup = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("203.0.113.5")}, nil }
	if g.Static(Webhook, "svc.example") != nil || g.Check(context.Background(), Webhook, "svc.example") == nil {
		t.Error("a name outside the allowed networks must pass Static and fail Check")
	}
}

func TestErrorsNameThePurposeAndHostOnly(t *testing.T) {
	g := mustNew(t, Config{Default: "deny"})
	err := g.Static(Webhook, "evil.example")
	if err == nil || !strings.Contains(err.Error(), "webhook") || !strings.Contains(err.Error(), "evil.example") {
		t.Fatalf("unhelpful error: %v", err)
	}
}

func TestValidation(t *testing.T) {
	bad := []Config{
		{Default: "maybe"},
		{Destinations: map[string][]string{"x": {"10.0.0.0/99"}}},
		{Destinations: map[string][]string{"x": {"bad host"}}},
		{Destinations: map[string][]string{"x": {"*."}}},
		{Rules: []Rule{{Allow: []string{"missing"}}}},
		{Destinations: map[string][]string{"x": {"a.example"}}, Rules: []Rule{{Purposes: []string{"carrier-pigeon"}, Allow: []string{"x"}}}},
		{Destinations: map[string][]string{"x": {"a.example"}}, Rules: []Rule{{}}},
	}
	for i, c := range bad {
		if _, err := New(c); err == nil {
			t.Errorf("config %d should be rejected", i)
		}
	}
}

func counting() (*httptest.Server, *atomic.Int64) {
	var n atomic.Int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(204)
	}))
	return s, &n
}

func TestBlockedRequestNeverReachesTheServer(t *testing.T) {
	srv, hits := counting()
	defer srv.Close()

	g := mustNew(t, Config{
		Destinations: map[string][]string{"other": {"10.0.0.0/8"}},
		Rules:        []Rule{{Allow: []string{"other"}}},
	})
	c := g.Client(nil, Webhook)
	if _, err := c.Get(srv.URL); err == nil || !strings.Contains(err.Error(), "egress blocked") {
		t.Fatalf("want an egress error, got %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("the blocked request reached the server")
	}

	allowed := mustNew(t, Config{
		Destinations: map[string][]string{"local": {"127.0.0.0/8"}},
		Rules:        []Rule{{Allow: []string{"local"}}},
	})
	resp, err := allowed.Client(nil, Webhook).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if hits.Load() != 1 {
		t.Fatalf("allowed request: server saw %d hits", hits.Load())
	}
}

func TestRedirectToAnotherHostIsRefused(t *testing.T) {
	target, hits := counting()
	defer target.Close()
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(target.URL, "127.0.0.1", "localhost", 1), http.StatusFound)
	}))
	defer redir.Close()

	g := mustNew(t, Config{
		Destinations: map[string][]string{"all": {"*"}},
		Rules:        []Rule{{Allow: []string{"all"}}},
	})
	if _, err := g.Client(nil, Source).Get(redir.URL); err == nil || !strings.Contains(err.Error(), "redirected to another host") {
		t.Fatalf("want a redirect refusal, got %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("the redirect target was contacted")
	}
}

func TestResolvedAddressMustBeInsideAllowedNetworks(t *testing.T) {
	srv, hits := counting()
	defer srv.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))

	g := mustNew(t, Config{
		Destinations: map[string][]string{"net": {"127.0.0.0/8"}},
		Rules:        []Rule{{Allow: []string{"net"}}},
	})

	// A name that resolves outside the allowed network is refused, with no hit.
	g.lookup = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("192.0.2.9")}, nil }
	if _, err := g.Client(nil, Webhook).Get("http://svc.example:" + port); err == nil {
		t.Fatal("a name resolving outside the allowed network was reached")
	}

	// One that resolves inside is dialled at that address, not re-resolved.
	g.lookup = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("127.0.0.1")}, nil }
	resp, err := g.Client(nil, Webhook).Get("http://svc.example:" + port)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if hits.Load() != 1 {
		t.Fatalf("server saw %d hits, want 1", hits.Load())
	}
}

func TestAMixedAnswerOnlyDialsAllowedAddresses(t *testing.T) {
	srv, hits := counting()
	defer srv.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	g := mustNew(t, Config{
		Destinations: map[string][]string{"net": {"127.0.0.0/8"}},
		Rules:        []Rule{{Allow: []string{"net"}}},
	})
	g.lookup = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.0.2.9"), net.ParseIP("127.0.0.1")}, nil
	}
	resp, err := g.Client(nil, Source).Get("http://svc.example:" + port)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if hits.Load() != 1 {
		t.Fatalf("hits=%d", hits.Load())
	}
}

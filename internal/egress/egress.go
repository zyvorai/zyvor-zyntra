// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

// Package egress decides which hosts Zyntra may send data to or fetch from.
//
// A policy names destination groups (hosts, wildcard suffixes and CIDRs) and
// rules that say which purposes may reach which groups. With no policy the
// guard is nil and everything is allowed, which is today's behaviour. A guard
// wraps an *http.Client so the same check covers webhooks, source fetches, the
// AI endpoint, Keep and notifications, including redirects and DNS answers.
package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Purposes a request can have. A rule lists the purposes it covers.
const (
	Webhook = "webhook" // an approved action's HTTP call
	Source  = "source"  // an http or sheet KPI source
	AI      = "ai"      // the model endpoint
	Keep    = "keep"    // Fabric Keep
	Notify  = "notify"  // outbound notifications
)

var purposes = map[string]bool{Webhook: true, Source: true, AI: true, Keep: true, Notify: true}

// Config is the egress block of the policy file.
type Config struct {
	// Destinations are named groups. An entry is a host name, a "*.suffix"
	// wildcard (subdomains only), an IP or CIDR, or "*" for anywhere.
	Destinations map[string][]string `yaml:"destinations,omitempty" json:"destinations,omitempty"`
	// Default is allow (the default) or deny, for a purpose no rule covers.
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
	Rules   []Rule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// Rule lets some purposes reach some destination groups. A purpose covered by
// any rule may reach only the union of those rules' groups.
type Rule struct {
	Name     string   `yaml:"name,omitempty" json:"name,omitempty"`
	Purposes []string `yaml:"purposes,omitempty" json:"purposes,omitempty"` // empty means all
	Allow    []string `yaml:"allow" json:"allow"`
}

type group struct {
	any      bool
	hosts    map[string]bool
	suffixes []string
	nets     []*net.IPNet
}

// Guard enforces a Config. A nil *Guard allows everything.
type Guard struct {
	deny   bool
	groups map[string]*group
	rules  []Rule
	lookup func(ctx context.Context, host string) ([]net.IP, error)
}

// New validates cfg and returns a guard.
func New(cfg Config) (*Guard, error) {
	g := &Guard{groups: map[string]*group{}, rules: cfg.Rules, lookup: lookupIP}
	switch cfg.Default {
	case "", "allow":
	case "deny":
		g.deny = true
	default:
		return nil, errors.New(`default must be "allow" or "deny"`)
	}
	for name, entries := range cfg.Destinations {
		gr := &group{hosts: map[string]bool{}}
		for _, e := range entries {
			if err := gr.add(strings.TrimSpace(e)); err != nil {
				return nil, fmt.Errorf("destination %s: %w", name, err)
			}
		}
		g.groups[name] = gr
	}
	for i, r := range cfg.Rules {
		if len(r.Allow) == 0 {
			return nil, fmt.Errorf("rule %d: allow needs at least one destination group", i)
		}
		for _, p := range r.Purposes {
			if !purposes[p] {
				return nil, fmt.Errorf("rule %d: unknown purpose %q", i, p)
			}
		}
		for _, a := range r.Allow {
			if _, ok := g.groups[a]; !ok {
				return nil, fmt.Errorf("rule %d: unknown destination group %q", i, a)
			}
		}
	}
	return g, nil
}

func (gr *group) add(e string) error {
	switch {
	case e == "":
		return errors.New("empty entry")
	case e == "*":
		gr.any = true
	case strings.Contains(e, "/"):
		_, n, err := net.ParseCIDR(e)
		if err != nil {
			return fmt.Errorf("bad CIDR %q", e)
		}
		gr.nets = append(gr.nets, n)
	case net.ParseIP(e) != nil:
		ip := net.ParseIP(e)
		bits := 128
		if ip.To4() != nil {
			bits = 32
		}
		gr.nets = append(gr.nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	case strings.HasPrefix(e, "*."):
		s := strings.ToLower(e[1:]) // ".example.com"
		if len(s) < 3 || strings.Contains(s[1:], "*") {
			return fmt.Errorf("bad wildcard %q", e)
		}
		gr.suffixes = append(gr.suffixes, s)
	case strings.ContainsAny(e, "*:/ "):
		return fmt.Errorf("bad host %q", e)
	default:
		gr.hosts[strings.ToLower(e)] = true
	}
	return nil
}

func (gr *group) matchesName(host string) bool {
	if gr.any || gr.hosts[host] {
		return true
	}
	for _, s := range gr.suffixes {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	return false
}

func (gr *group) containsIP(ip net.IP) bool {
	if gr.any {
		return true
	}
	for _, n := range gr.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// groupsFor returns the groups a purpose may reach. restricted is false when
// no rule covers it, in which case the default applies.
func (g *Guard) groupsFor(purpose string) (gs []*group, restricted bool) {
	for _, r := range g.rules {
		if len(r.Purposes) > 0 && !slices.Contains(r.Purposes, purpose) {
			continue
		}
		restricted = true
		for _, a := range r.Allow {
			gs = append(gs, g.groups[a])
		}
	}
	return gs, restricted
}

func clean(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(strings.Trim(host, "[]"))
}

func blocked(purpose, host string) error {
	return fmt.Errorf("egress blocked: %s to %s is not allowed by policy", purpose, host)
}

// targets checks host for purpose and returns the addresses to dial. For a
// host allowed by name it returns host itself; for one allowed by network it
// returns only the resolved addresses inside the allowed networks, so a DNS
// answer cannot send the connection elsewhere.
func (g *Guard) targets(ctx context.Context, purpose, host string, resolve bool) ([]string, error) {
	host = clean(host)
	gs, restricted := g.groupsFor(purpose)
	if !restricted {
		if g.deny {
			return nil, blocked(purpose, host)
		}
		return []string{host}, nil
	}
	if ip := net.ParseIP(host); ip != nil {
		for _, gr := range gs {
			if gr.containsIP(ip) {
				return []string{host}, nil
			}
		}
		return nil, blocked(purpose, host)
	}
	for _, gr := range gs {
		if gr.matchesName(host) {
			return []string{host}, nil
		}
	}
	hasNets := false
	for _, gr := range gs {
		if len(gr.nets) > 0 {
			hasNets = true
		}
	}
	if !hasNets {
		return nil, blocked(purpose, host)
	}
	if !resolve {
		return []string{host}, nil // decided when the connection is made
	}
	ips, err := g.lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("egress blocked: %s to %s: cannot resolve", purpose, host)
	}
	var ok []string
	for _, ip := range ips {
		for _, gr := range gs {
			if gr.containsIP(ip) {
				ok = append(ok, ip.String())
				break
			}
		}
	}
	if len(ok) == 0 {
		return nil, blocked(purpose, host)
	}
	return ok, nil
}

// Check reports whether purpose may reach host (a name, an IP or host:port).
// It resolves names when only networks allow the destination.
func (g *Guard) Check(ctx context.Context, purpose, host string) error {
	if g == nil {
		return nil
	}
	_, err := g.targets(ctx, purpose, host, true)
	return err
}

// Static is Check without DNS: a name that only a network could allow passes,
// because it can only be decided when the connection is made. Use it to
// vet configured URLs at start.
func (g *Guard) Static(purpose, host string) error {
	if g == nil {
		return nil
	}
	_, err := g.targets(context.Background(), purpose, host, false)
	return err
}

func lookupIP(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// Client returns a copy of c that enforces the guard for purpose. A nil guard
// returns c unchanged. A nil c is replaced with a client with a 15 second
// timeout. Redirects to another host are refused unless c sets its own
// CheckRedirect. A guarded client connects directly and ignores proxy
// environment variables, because the guard has to see the real destination.
func (g *Guard) Client(c *http.Client, purpose string) *http.Client {
	if g == nil {
		return c
	}
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	out := *c
	rt := &transport{g: g, purpose: purpose}
	switch t := c.Transport.(type) {
	case nil:
		rt.base, rt.pinned = g.pin(http.DefaultTransport.(*http.Transport).Clone(), purpose), true
	case *http.Transport:
		rt.base, rt.pinned = g.pin(t.Clone(), purpose), true
	default:
		rt.base = t // cannot pin the dial, so RoundTrip resolves and checks
	}
	out.Transport = rt
	if out.CheckRedirect == nil {
		out.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) > 0 && !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
				return fmt.Errorf("egress blocked: %s redirected to another host", purpose)
			}
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		}
	}
	return &out
}

// pin makes the transport connect only to addresses the guard allows.
func (g *Guard) pin(t *http.Transport, purpose string) *http.Transport {
	d := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	t.Proxy = nil
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ts, err := g.targets(ctx, purpose, host, true)
		if err != nil {
			return nil, err
		}
		var last error
		for _, a := range ts {
			c, err := d.DialContext(ctx, network, net.JoinHostPort(a, port))
			if err == nil {
				return c, nil
			}
			last = err
		}
		return nil, last
	}
	return t
}

type transport struct {
	g       *Guard
	purpose string
	base    http.RoundTripper
	pinned  bool // the dial itself enforces the guard
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	check := t.g.Check
	if t.pinned {
		check = func(ctx context.Context, purpose, host string) error { return t.g.Static(purpose, host) }
	}
	if err := check(req.Context(), t.purpose, req.URL.Host); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

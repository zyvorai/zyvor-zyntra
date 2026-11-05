// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package auth

import (
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ThrottleConfig limits failed sign-ins. MaxFailures is how many wrong
// credentials one account may present inside Window before it is locked for
// Lockout; a client address gets IPFactor times as many, so one shared
// address (an office, a proxy) does not lock itself out. A zero MaxFailures
// turns the throttle off.
type ThrottleConfig struct {
	MaxFailures int
	Window      time.Duration
	Lockout     time.Duration
	IPFactor    int
	// TrustedProxies are the networks whose X-Forwarded-For header is believed.
	// Without them the header is ignored, because anyone can send it.
	TrustedProxies []netip.Prefix
}

// DefaultThrottle is five wrong passwords in fifteen minutes, then fifteen
// minutes locked.
func DefaultThrottle() ThrottleConfig {
	return ThrottleConfig{MaxFailures: 5, Window: 15 * time.Minute, Lockout: 15 * time.Minute, IPFactor: 4}
}

// maxTracked bounds the failure tables so a flood of made-up account names
// cannot grow them without limit.
const maxTracked = 10000

type failure struct {
	count int
	first time.Time
	until time.Time
}

// throttle counts failed sign-ins per account name and per client address.
// Every name a caller submits is counted, whether or not the account exists,
// so a locked response says nothing about which accounts are real. State is in
// memory and resets on restart.
type throttle struct {
	cfg ThrottleConfig
	now func() time.Time
	// notify is told when a lock starts, as event "locked", with the kind
	// ("account" or "address") and the key (the account name or address).
	notify func(event, kind, key string)

	mu      sync.Mutex
	account map[string]*failure
	address map[string]*failure
}

func newThrottle(cfg ThrottleConfig, now func() time.Time) *throttle {
	if cfg.Window <= 0 {
		cfg.Window = 15 * time.Minute
	}
	if cfg.Lockout <= 0 {
		cfg.Lockout = 15 * time.Minute
	}
	if cfg.IPFactor <= 0 {
		cfg.IPFactor = 4
	}
	return &throttle{cfg: cfg, now: now, account: map[string]*failure{}, address: map[string]*failure{}}
}

func (t *throttle) enabled() bool { return t != nil && t.cfg.MaxFailures > 0 }

// locked reports how long the account or the address stays locked, or zero.
func (t *throttle) locked(account, addr string) time.Duration {
	if !t.enabled() {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	var wait time.Duration
	if f := t.address[addr]; f != nil && now.Before(f.until) {
		wait = f.until.Sub(now)
	}
	if account != "" {
		if f := t.account[account]; f != nil && now.Before(f.until) && f.until.Sub(now) > wait {
			wait = f.until.Sub(now)
		}
	}
	return wait
}

// fail records a wrong credential. It does not extend a lock that is already
// running, so guessing against a locked account gains nothing and costs
// nothing. An empty account name counts against the address only.
func (t *throttle) fail(account, addr string) {
	if !t.enabled() {
		return
	}
	t.mu.Lock()
	now := t.now()
	var events [][3]string
	if account != "" {
		if t.bump(t.account, account, t.cfg.MaxFailures, now) {
			events = append(events, [3]string{"locked", "account", account})
		}
	}
	if t.bump(t.address, addr, t.cfg.MaxFailures*t.cfg.IPFactor, now) {
		events = append(events, [3]string{"locked", "address", addr})
	}
	t.mu.Unlock()
	if t.notify != nil {
		for _, e := range events {
			t.notify(e[0], e[1], e[2])
		}
	}
}

// bump adds one failure and reports whether it started a lock.
func (t *throttle) bump(m map[string]*failure, key string, limit int, now time.Time) bool {
	f := m[key]
	if f == nil {
		if len(m) >= maxTracked {
			t.prune(m, now)
			if len(m) >= maxTracked {
				return false
			}
		}
		f = &failure{first: now}
		m[key] = f
	}
	if now.Before(f.until) {
		return false
	}
	if now.Sub(f.first) > t.cfg.Window {
		*f = failure{first: now}
	}
	f.count++
	if f.count >= limit {
		f.until = now.Add(t.cfg.Lockout)
		f.count, f.first = 0, now
		return true
	}
	return false
}

func (t *throttle) prune(m map[string]*failure, now time.Time) {
	for k, f := range m {
		if now.After(f.until) && now.Sub(f.first) > t.cfg.Window {
			delete(m, k)
		}
	}
}

// succeed clears the account's failures. The address's count is left to age
// out, so one valid login cannot be used to reset guessing from the same place.
func (t *throttle) succeed(account string) {
	if !t.enabled() || account == "" {
		return
	}
	t.mu.Lock()
	delete(t.account, account)
	t.mu.Unlock()
}

// clientAddr is the address a request came from. X-Forwarded-For is used only
// when the direct peer is a trusted proxy, and then the right-most entry that
// is not itself a trusted proxy wins, since that is the one a trusted proxy
// appended.
func (t *throttle) clientAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if t == nil || len(t.cfg.TrustedProxies) == 0 || !t.trusted(host) {
		return host
	}
	parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p == "" {
			continue
		}
		if _, err := netip.ParseAddr(p); err != nil {
			return host
		}
		if !t.trusted(p) {
			return p
		}
	}
	return host
}

func (t *throttle) trusted(host string) bool {
	a, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	a = a.Unmap()
	for _, p := range t.cfg.TrustedProxies {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// retryAfter formats a wait for the Retry-After header, rounded up.
func retryAfter(d time.Duration) string {
	return strconv.Itoa(int((d + time.Second - 1) / time.Second))
}

// ParseTrustedProxies reads a comma-separated list of addresses and CIDRs.
func ParseTrustedProxies(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if p, err := netip.ParsePrefix(part); err == nil {
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, &net.ParseError{Type: "proxy address or CIDR", Text: part}
		}
		out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
	}
	return out, nil
}

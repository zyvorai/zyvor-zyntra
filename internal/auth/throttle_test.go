// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type rig struct {
	a   *Auth
	mux *http.ServeMux
	now time.Time
}

func newRig(t *testing.T, cfg ThrottleConfig) *rig {
	t.Helper()
	r := &rig{a: New("k3y", ""), now: time.Unix(1_700_000_000, 0)}
	r.a.now = func() time.Time { return r.now }
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	r.a.SetUsers([]LocalUser{{Name: "vic", Hash: string(hash), Roles: []Role{RoleViewer}}})
	r.a.SetThrottle(cfg)
	r.mux = http.NewServeMux()
	r.a.Routes(r.mux)
	return r
}

// post signs in from addr and returns the status, Retry-After and body.
func (r *rig) post(body, addr string, hdr ...string) (int, string, string) {
	req := httptest.NewRequest("POST", "/api/v1/session", strings.NewReader(body))
	req.RemoteAddr = addr
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	w := httptest.NewRecorder()
	r.mux.ServeHTTP(w, req)
	return w.Code, w.Header().Get("Retry-After"), w.Body.String()
}

func login(user, pw string) string {
	return `{"username":"` + user + `","password":"` + pw + `"}`
}

func cfg3() ThrottleConfig {
	return ThrottleConfig{MaxFailures: 3, Window: time.Minute, Lockout: 10 * time.Minute, IPFactor: 4}
}

func TestAccountLocksAfterRepeatedFailuresAndBlocksTheRightPassword(t *testing.T) {
	r := newRig(t, cfg3())
	for i := 0; i < 3; i++ {
		if code, _, _ := r.post(login("vic", "wrong"), "10.0.0.1:1"); code != 401 {
			t.Fatalf("attempt %d = %d", i, code)
		}
	}
	// Locked: even the right password is refused, with a Retry-After.
	code, retry, body := r.post(login("vic", "s3cret"), "10.0.0.9:1")
	if code != 401 || retry == "" || !strings.Contains(body, "invalid credentials") {
		t.Fatalf("locked login = %d retry=%q body=%s", code, retry, body)
	}
	// The lock expires.
	r.now = r.now.Add(11 * time.Minute)
	if code, _, _ := r.post(login("vic", "s3cret"), "10.0.0.9:1"); code != 200 {
		t.Fatalf("after the lockout = %d, want 200", code)
	}
}

func TestLockedResponseIsTheSameForRealAndMadeUpAccounts(t *testing.T) {
	r := newRig(t, cfg3())
	for _, name := range []string{"vic", "ghost"} {
		for i := 0; i < 3; i++ {
			r.post(login(name, "wrong"), "10.0.0.1:1")
		}
	}
	c1, ra1, b1 := r.post(login("vic", "wrong"), "10.0.0.2:1")
	c2, ra2, b2 := r.post(login("ghost", "wrong"), "10.0.0.2:1")
	if c1 != c2 || b1 != b2 || (ra1 == "") != (ra2 == "") {
		t.Errorf("an unknown account must lock exactly like a real one: %d %q %s vs %d %q %s", c1, ra1, b1, c2, ra2, b2)
	}
}

func TestSuccessClearsTheAccountButNotTheAddress(t *testing.T) {
	r := newRig(t, cfg3())
	r.post(login("vic", "wrong"), "10.0.0.1:1")
	r.post(login("vic", "wrong"), "10.0.0.1:1")
	if code, _, _ := r.post(login("vic", "s3cret"), "10.0.0.1:1"); code != 200 {
		t.Fatalf("login = %d", code)
	}
	// Two failures were forgotten: two more do not lock.
	r.post(login("vic", "wrong"), "10.0.0.1:1")
	r.post(login("vic", "wrong"), "10.0.0.1:1")
	if code, _, _ := r.post(login("vic", "s3cret"), "10.0.0.1:1"); code != 200 {
		t.Fatalf("an account that succeeded in between must not be locked: %d", code)
	}
	// The address count was not reset: 12 failures across many names lock it.
	r2 := newRig(t, cfg3())
	for i := 0; i < 12; i++ {
		r2.post(login("name"+string(rune('a'+i)), "wrong"), "10.0.0.7:1")
	}
	if code, retry, _ := r2.post(login("vic", "s3cret"), "10.0.0.7:1"); code != 401 || retry == "" {
		t.Errorf("a spraying address must be locked even for a real account: %d %q", code, retry)
	}
	if code, _, _ := r2.post(login("vic", "s3cret"), "10.0.0.8:1"); code != 200 {
		t.Errorf("another address is unaffected: %d", code)
	}
}

func TestAccessKeyIsThrottledByAddressOnly(t *testing.T) {
	r := newRig(t, cfg3())
	// Guessing the key never locks any account, so the break-glass path stays
	// open to the operator's own address.
	for i := 0; i < 5; i++ {
		r.post(`{"token":"nope"}`, "10.0.0.3:1")
	}
	if code, _, _ := r.post(login("vic", "s3cret"), "10.0.0.4:1"); code != 200 {
		t.Errorf("key guessing locked an account: %d", code)
	}
	if code, _, _ := r.post(`{"token":"k3y"}`, "10.0.0.4:1"); code != 200 {
		t.Errorf("the right key from another address = %d", code)
	}
	for i := 0; i < 12; i++ {
		r.post(`{"token":"nope"}`, "10.0.0.5:1")
	}
	if code, retry, _ := r.post(`{"token":"k3y"}`, "10.0.0.5:1"); code != 401 || retry == "" {
		t.Errorf("a guessing address is locked for the key too: %d %q", code, retry)
	}
}

func TestThrottleCanBeTurnedOff(t *testing.T) {
	r := newRig(t, ThrottleConfig{})
	for i := 0; i < 20; i++ {
		r.post(login("vic", "wrong"), "10.0.0.1:1")
	}
	if code, _, _ := r.post(login("vic", "s3cret"), "10.0.0.1:1"); code != 200 {
		t.Errorf("with MaxFailures 0 nothing locks: %d", code)
	}
	// And an Auth that never had a throttle set behaves the same.
	a := New("k3y", "")
	mux := http.NewServeMux()
	a.Routes(mux)
	req := httptest.NewRequest("POST", "/api/v1/session", strings.NewReader(`{"token":"k3y"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("default Auth = %d", w.Code)
	}
}

func TestFailuresAgeOutOfTheWindow(t *testing.T) {
	r := newRig(t, cfg3())
	r.post(login("vic", "wrong"), "10.0.0.1:1")
	r.post(login("vic", "wrong"), "10.0.0.1:1")
	r.now = r.now.Add(2 * time.Minute) // past the one-minute window
	r.post(login("vic", "wrong"), "10.0.0.1:1")
	if code, _, _ := r.post(login("vic", "s3cret"), "10.0.0.1:1"); code != 200 {
		t.Errorf("old failures must not count towards a lock: %d", code)
	}
}

func TestLockNotifiesOnce(t *testing.T) {
	r := newRig(t, cfg3())
	var got []string
	r.a.OnLock(func(kind, key string) { got = append(got, kind+":"+key) })
	for i := 0; i < 8; i++ {
		r.post(login("vic", "wrong"), "10.0.0.1:1")
	}
	if len(got) != 1 || got[0] != "account:vic" {
		t.Errorf("notifications = %v, want one account lock (guessing while locked adds nothing)", got)
	}
}

func TestClientAddrTrustsForwardedForOnlyFromTrustedProxies(t *testing.T) {
	proxies, err := ParseTrustedProxies("10.1.0.0/16, 192.168.1.5")
	if err != nil {
		t.Fatal(err)
	}
	th := newThrottle(ThrottleConfig{MaxFailures: 1, TrustedProxies: proxies}, time.Now)
	get := func(remote, xff string) string {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = remote
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		return th.clientAddr(req)
	}
	if got := get("203.0.113.9:5", "1.2.3.4"); got != "203.0.113.9" {
		t.Errorf("an untrusted peer's X-Forwarded-For must be ignored: %q", got)
	}
	if got := get("10.1.2.3:5", "1.2.3.4"); got != "1.2.3.4" {
		t.Errorf("a trusted proxy's header is believed: %q", got)
	}
	// The client may have put its own entries first; the proxy appended the last.
	if got := get("10.1.2.3:5", "6.6.6.6, 1.2.3.4, 10.1.9.9"); got != "1.2.3.4" {
		t.Errorf("right-most untrusted entry wins, got %q", got)
	}
	if got := get("192.168.1.5:5", "not-an-ip"); got != "192.168.1.5" {
		t.Errorf("a garbled header falls back to the peer: %q", got)
	}
	if got := get("10.1.2.3:5", ""); got != "10.1.2.3" {
		t.Errorf("no header: %q", got)
	}
	if _, err := ParseTrustedProxies("10.1.0.0/16, banana"); err == nil {
		t.Error("a bad proxy entry must be an error")
	}
}

func TestTrackingTableIsBounded(t *testing.T) {
	th := newThrottle(ThrottleConfig{MaxFailures: 100, Window: time.Hour}, time.Now)
	for i := 0; i < maxTracked+500; i++ {
		th.fail("user"+time.Duration(i).String(), "addr")
	}
	th.mu.Lock()
	n := len(th.account)
	th.mu.Unlock()
	if n > maxTracked {
		t.Errorf("tracked %d accounts, want at most %d", n, maxTracked)
	}
}

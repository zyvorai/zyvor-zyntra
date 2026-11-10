// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package webhooks

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestSignAndDeliver(t *testing.T) {
	var gotBody []byte
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotSig = r.Header.Get("X-Zyntra-Signature")
		w.WriteHeader(204)
	}))
	defer srv.Close()

	ev := Event{Type: "pending", At: time.Now().UTC(), Data: map[string]any{"proposal": "p1"}}
	status, err := Deliver(srv.URL, "s3cret", ev, srv.Client())
	if err != nil || status != 204 {
		t.Fatalf("deliver: %d %v", status, err)
	}
	want := "sha256=" + Sign("s3cret", gotBody)
	if gotSig != want {
		t.Fatalf("sig = %q want %q", gotSig, want)
	}
	if len(gotBody) == 0 {
		t.Fatal("empty body")
	}
}

func TestStore(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "webhooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := st.Put(Destination{URL: "https://example.test/hook", Enabled: true, Events: []string{"pending"}})
	if err != nil || d.ID == "" || d.Secret == "" {
		t.Fatalf("put: %+v %v", d, err)
	}
	if !d.Matches("pending") || d.Matches("executed") {
		t.Fatal("match filter")
	}
	if pub := d.Public(); pub.Secret != "••••••••" {
		t.Fatalf("public secret = %q", pub.Secret)
	}
}

// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package prometheus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryRejectsNonFinite(t *testing.T) {
	for _, v := range []string{"NaN", "+Inf", "-Inf"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(`{"status":"success","data":{"resultType":"scalar","result":[1,"` + v + `"]}}`))
		}))
		_, err := New(srv.URL).Query(context.Background(), "x")
		srv.Close()
		if err == nil {
			t.Errorf("%s: want error", v)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"success","data":{"resultType":"scalar","result":[1,"2.5"]}}`))
	}))
	defer srv.Close()
	if v, err := New(srv.URL).Query(context.Background(), "x"); err != nil || v != 2.5 {
		t.Fatalf("got %v, %v", v, err)
	}
}

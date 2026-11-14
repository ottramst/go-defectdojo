package defectdojo_test

import (
	"context"
	"net/http"
	"testing"
)

func TestClient_ServerVersion(t *testing.T) {
	mux, client := setup(t)
	calls := 0
	mux.HandleFunc("GET /api/v2/oa3/schema/", func(w http.ResponseWriter, r *http.Request) {
		calls++
		testQuery(t, r, "format=json")
		writeJSON(t, w, http.StatusOK, []byte(`{"openapi":"3.0.3","info":{"title":"DefectDojo API v2","version":"3.5.103"},"paths":{}}`))
	})

	for range 2 {
		v, err := client.ServerVersion(context.Background())
		if err != nil {
			t.Fatalf("ServerVersion() error = %v", err)
		}
		if v != "3.5.103" {
			t.Errorf("ServerVersion() = %q, want 3.5.103", v)
		}
	}
	if calls != 1 {
		t.Errorf("schema fetched %d times, want 1 (cached)", calls)
	}
}

func TestClient_ServerVersion_errors(t *testing.T) {
	mux, client := setup(t)
	fail := true
	mux.HandleFunc("GET /api/v2/oa3/schema/", func(w http.ResponseWriter, _ *http.Request) {
		if fail {
			writeJSON(t, w, http.StatusServiceUnavailable, nil)
			return
		}
		writeJSON(t, w, http.StatusOK, []byte(`{"info":{}}`))
	})

	if _, err := client.ServerVersion(context.Background()); err == nil {
		t.Error("ServerVersion(503) error = nil, want error")
	}
	// Failures are not cached.
	fail = false
	if _, err := client.ServerVersion(context.Background()); err == nil {
		t.Error("ServerVersion(no version) error = nil, want error")
	}
}

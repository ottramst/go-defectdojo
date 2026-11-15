package defectdojo_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestTestTypesService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/test_types/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=3")
		writeJSON(t, w, http.StatusOK, fixture(t, "test_types/list.json"))
	})

	list, _, err := client.TestTypes.List(context.Background(), &defectdojo.ListTestTypesOptions{ListOptions: defectdojo.ListOptions{Limit: 3}})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 394 || len(list.Results) != 3 {
		t.Fatalf("List() = %d of %d, want 3 of 394", len(list.Results), list.Count)
	}
	if want := (defectdojo.TestType{ID: 3, Name: "Pen Test", Active: true}); *list.Results[2] != want {
		t.Errorf("third test type = %+v, want %+v", list.Results[2], want)
	}
}

func TestTestTypesService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/test_types/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=1&name=a")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":"x","results":[{"id":1,"name":"a"}]}`))
		case "1":
			testQuery(t, r, "limit=1&name=a&offset=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":null,"results":[{"id":2,"name":"a"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})
	var ids []int
	for tt, err := range client.TestTypes.ListAll(context.Background(), &defectdojo.ListTestTypesOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1}, Name: "a",
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		ids = append(ids, tt.ID)
	}
	if fmt.Sprint(ids) != "[1 2]" {
		t.Errorf("ListAll() = %v, want [1 2]", ids)
	}
}

func TestTestTypesService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/test_types/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "1" {
			writeJSON(t, w, http.StatusNotFound, fixture(t, "errors/not_found.json"))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "test_types/get.json"))
	})
	got, _, err := client.TestTypes.Get(context.Background(), 1, nil)
	if err != nil || *got != (defectdojo.TestType{ID: 1, Name: "API Test", Active: true}) {
		t.Errorf("Get() = %+v, %v; want API Test", got, err)
	}
	if _, _, err := client.TestTypes.Get(context.Background(), 2, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestTestTypesService_GetByName(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/test_types/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("name") {
		case "Trivy Scan":
			// A filter that matched loosely is checked again on the client side.
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":5,"name":"trivy scan"},{"id":6,"name":"Trivy Scan"}]}`))
		default:
			writeJSON(t, w, http.StatusOK, []byte(`{"count":0,"results":[]}`))
		}
	})
	tt, _, err := client.TestTypes.GetByName(context.Background(), "Trivy Scan", nil)
	if err != nil || tt.ID != 6 {
		t.Errorf("GetByName() = %v, %v; want ID 6", tt, err)
	}
	if _, _, err := client.TestTypes.GetByName(context.Background(), "Nope", nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(missing) error = %v, want ErrNotFound", err)
	}
}

func TestTestTypesService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	if _, _, err := client.TestTypes.Get(context.Background(), 0, nil); err == nil {
		t.Error("Get(0) error = nil, want an argument error")
	}
	if _, _, err := client.TestTypes.GetByName(context.Background(), "", nil); err == nil {
		t.Error("GetByName(\"\") error = nil, want an argument error")
	}
}

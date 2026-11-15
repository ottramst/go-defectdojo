package defectdojo_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestDevelopmentEnvironmentsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/development_environments/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=3")
		writeJSON(t, w, http.StatusOK, fixture(t, "development_environments/list.json"))
	})

	list, _, err := client.DevelopmentEnvironments.List(context.Background(), &defectdojo.ListDevelopmentEnvironmentsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 3},
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 8 || len(list.DevelopmentEnvironments) != 3 {
		t.Fatalf("List() = %d of %d, want 3 of 8", len(list.DevelopmentEnvironments), list.Count)
	}
	if prod := list.DevelopmentEnvironments[2]; *prod != (defectdojo.DevelopmentEnvironment{ID: 3, Name: "Production"}) {
		t.Errorf("third development environment = %+v, want Production", prod)
	}
}

func TestDevelopmentEnvironmentsService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/development_environments/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":"x","results":[{"id":1,"name":"a"}]}`))
		case "1":
			testQuery(t, r, "limit=1&offset=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":null,"results":[{"id":2,"name":"b"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})

	var names []string
	for env, err := range client.DevelopmentEnvironments.ListAll(context.Background(), &defectdojo.ListDevelopmentEnvironmentsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1},
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, env.Name)
	}
	if fmt.Sprint(names) != "[a b]" {
		t.Errorf("ListAll() = %v, want [a b]", names)
	}
}

func TestDevelopmentEnvironmentsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/development_environments/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "12" {
			writeJSON(t, w, http.StatusNotFound, fixture(t, "errors/not_found.json"))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "development_environments/get.json"))
	})

	got, _, err := client.DevelopmentEnvironments.Get(context.Background(), 12)
	if err != nil || *got != (defectdojo.DevelopmentEnvironment{ID: 12, Name: "QA"}) {
		t.Errorf("Get() = %+v, %v; want QA", got, err)
	}
	if _, _, err := client.DevelopmentEnvironments.Get(context.Background(), 13); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestDevelopmentEnvironmentsService_GetByName(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/development_environments/", func(w http.ResponseWriter, r *http.Request) {
		// There is no name filter: GetByName pages through all of them.
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		wantQuery := "limit=100"
		if offset > 0 {
			wantQuery += "&offset=" + strconv.Itoa(offset)
		}
		testQuery(t, r, wantQuery)
		results := ""
		for i := offset; i < min(offset+100, 150); i++ {
			if results != "" {
				results += ","
			}
			name := fmt.Sprintf("env-%d", i)
			if i == 10 || i == 140 {
				name = "twice"
			}
			results += fmt.Sprintf(`{"id":%d,"name":%q}`, i+1, name)
		}
		writeJSON(t, w, http.StatusOK, []byte(`{"count":150,"results":[`+results+`]}`))
	})

	env, _, err := client.DevelopmentEnvironments.GetByName(context.Background(), "env-120")
	if err != nil || env.ID != 121 {
		t.Errorf("GetByName() = %v, %v; want ID 121 from the second page", env, err)
	}
	for _, name := range []string{"ENV-120", "env-1200"} {
		if _, _, err := client.DevelopmentEnvironments.GetByName(context.Background(), name); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("GetByName(%q) error = %v, want ErrNotFound", name, err)
		}
	}
	// Names aren't unique: several matches, on any page, are an error.
	_, _, err = client.DevelopmentEnvironments.GetByName(context.Background(), "twice")
	if err == nil || errors.Is(err, defectdojo.ErrNotFound) || !strings.Contains(err.Error(), "[11 141]") {
		t.Errorf("GetByName(twice) error = %v, want an error naming IDs 11 and 141", err)
	}
}

func TestDevelopmentEnvironmentsService_GetByName_error(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/development_environments/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, fixture(t, "errors/invalid_token.json"))
	})
	if _, _, err := client.DevelopmentEnvironments.GetByName(context.Background(), "x"); !errors.Is(err, defectdojo.ErrForbidden) {
		t.Errorf("GetByName() error = %v, want ErrForbidden", err)
	}
}

func TestDevelopmentEnvironmentsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/development_environments/", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "Content-Type", "application/json")
		testJSONBody(t, r, `{"name":"QA"}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "development_environments/get.json"))
	})

	in := &defectdojo.DevelopmentEnvironment{ID: 99, Name: "QA"}
	env, resp, err := client.DevelopmentEnvironments.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || env.ID != 12 {
		t.Errorf("Create() = %d %+v", resp.StatusCode, env)
	}
	if in.ID != 99 {
		t.Errorf("Create() changed the caller's development environment: %+v", in)
	}
}

func TestDevelopmentEnvironmentsService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/development_environments/12/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"name":"Quality"}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "development_environments/get.json"))
	})
	if _, _, err := client.DevelopmentEnvironments.Update(context.Background(), 12, &defectdojo.DevelopmentEnvironment{Name: "Quality"}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestDevelopmentEnvironmentsService_Patch(t *testing.T) {
	for name, tt := range map[string]struct {
		patch *defectdojo.DevelopmentEnvironmentPatch
		want  string
	}{
		"empty": {patch: &defectdojo.DevelopmentEnvironmentPatch{}, want: `{}`},
		"name":  {patch: &defectdojo.DevelopmentEnvironmentPatch{Name: new("Quality")}, want: `{"name":"Quality"}`},
	} {
		t.Run(name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/development_environments/12/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "development_environments/get.json"))
			})
			if _, _, err := client.DevelopmentEnvironments.Patch(context.Background(), 12, tt.patch); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestDevelopmentEnvironmentsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/development_environments/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "1" {
			writeJSON(t, w, http.StatusConflict, fixture(t, "errors/development_environment_in_use.json"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.DevelopmentEnvironments.Delete(context.Background(), 12)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("StatusCode = %d, want 204", resp.StatusCode)
	}
	// An environment that tests use can't be deleted.
	_, err = client.DevelopmentEnvironments.Delete(context.Background(), 1)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrConflict) || !strings.Contains(errResp.Message, "Records that refer to it: test.") {
		t.Errorf("Delete(in use) error = %v, want the 409", err)
	}
}

func TestDevelopmentEnvironmentsService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/development_environments/12/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=10")
		writeJSON(t, w, http.StatusOK, fixture(t, "development_environments/delete_preview.json"))
	})
	preview, _, err := client.DevelopmentEnvironments.DeletePreview(context.Background(), 12, &defectdojo.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 1 || *preview.Objects[0] != (defectdojo.DeletedObject{Model: "Development_Environment", ID: 12, Name: "QA"}) {
		t.Errorf("DeletePreview() = %d %+v", preview.Count, preview.Objects)
	}
}

func TestDevelopmentEnvironmentsService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	svc := client.DevelopmentEnvironments
	calls := map[string]func() error{
		"Get zero ID":       func() error { _, _, err := svc.Get(ctx, 0); return err },
		"GetByName empty":   func() error { _, _, err := svc.GetByName(ctx, ""); return err },
		"Create nil":        func() error { _, _, err := svc.Create(ctx, nil); return err },
		"Create empty name": func() error { _, _, err := svc.Create(ctx, &defectdojo.DevelopmentEnvironment{}); return err },
		"Update negative ID": func() error {
			_, _, err := svc.Update(ctx, -1, &defectdojo.DevelopmentEnvironment{Name: "x"})
			return err
		},
		"Update empty name": func() error { _, _, err := svc.Update(ctx, 1, &defectdojo.DevelopmentEnvironment{}); return err },
		"Patch zero ID": func() error {
			_, _, err := svc.Patch(ctx, 0, &defectdojo.DevelopmentEnvironmentPatch{})
			return err
		},
		"Patch nil":             func() error { _, _, err := svc.Patch(ctx, 1, nil); return err },
		"Delete zero ID":        func() error { _, err := svc.Delete(ctx, 0); return err },
		"DeletePreview zero ID": func() error { _, _, err := svc.DeletePreview(ctx, 0, nil); return err },
	}
	for name, call := range calls {
		if err := call(); err == nil {
			t.Errorf("%s: error = nil, want an argument error", name)
		}
	}
}

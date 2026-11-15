package defectdojo_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestCICDInfrastructureService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/cicd_infrastructure/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "infrastructure_type=build_server&limit=5&name=Jenkins")
		writeJSON(t, w, http.StatusOK, fixture(t, "cicd_infrastructure/list.json"))
	})

	list, _, err := client.CICDInfrastructure.List(context.Background(), &defectdojo.ListCICDInfrastructureOptions{
		ListOptions:        defectdojo.ListOptions{Limit: 5},
		Name:               "Jenkins",
		InfrastructureType: defectdojo.CICDInfrastructureTypeBuildServer,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 2 || len(list.CICDInfrastructure) != 2 {
		t.Fatalf("List() = %d of %d, want 2 of 2", len(list.CICDInfrastructure), list.Count)
	}
	if scm := list.CICDInfrastructure[0]; scm.Name != "GitHub" || scm.InfrastructureType != "scm_server" || scm.Description != "" || scm.URL != "" {
		t.Errorf("first object = %+v, want the GitHub fixture", scm)
	}
}

func TestCICDInfrastructureService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/cicd_infrastructure/", func(w http.ResponseWriter, r *http.Request) {
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
	for ci, err := range client.CICDInfrastructure.ListAll(context.Background(), &defectdojo.ListCICDInfrastructureOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1},
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, ci.Name)
	}
	if fmt.Sprint(names) != "[a b]" {
		t.Errorf("ListAll() = %v, want [a b]", names)
	}
}

func TestCICDInfrastructureService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/cicd_infrastructure/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "8" {
			writeJSON(t, w, http.StatusNotFound, fixture(t, "errors/not_found.json"))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "cicd_infrastructure/get.json"))
	})

	got, _, err := client.CICDInfrastructure.Get(context.Background(), 8)
	want := defectdojo.CICDInfrastructure{
		ID: 8, Name: "Jenkins", InfrastructureType: defectdojo.CICDInfrastructureTypeBuildServer,
		Description: "The main build server.", URL: "https://jenkins.example.com",
	}
	if err != nil || *got != want {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, want)
	}
	if _, _, err := client.CICDInfrastructure.Get(context.Background(), 9); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestCICDInfrastructureService_GetByName(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/cicd_infrastructure/", func(w http.ResponseWriter, r *http.Request) {
		// Two types share the name; a loose match is checked again.
		all := []string{
			`{"id":1,"name":"Shared","infrastructure_type":"scm_server"}`,
			`{"id":2,"name":"Shared","infrastructure_type":"build_server"}`,
			`{"id":3,"name":"shared","infrastructure_type":"orchestration"}`,
		}
		var results []string
		for _, o := range all {
			if typ := r.URL.Query().Get("infrastructure_type"); typ == "" || strings.Contains(o, `"`+typ+`"`) {
				results = append(results, o)
			}
		}
		writeJSON(t, w, http.StatusOK, []byte(fmt.Sprintf(`{"count":%d,"results":[%s]}`, len(results), strings.Join(results, ","))))
	})

	ci, _, err := client.CICDInfrastructure.GetByName(context.Background(), "Shared", defectdojo.CICDInfrastructureTypeBuildServer)
	if err != nil || ci.ID != 2 {
		t.Errorf("GetByName(build server) = %v, %v; want ID 2", ci, err)
	}
	_, _, err = client.CICDInfrastructure.GetByName(context.Background(), "Shared", "")
	if err == nil || errors.Is(err, defectdojo.ErrNotFound) || !strings.Contains(err.Error(), "[1 2]") {
		t.Errorf("GetByName(any type) error = %v, want an error naming IDs 1 and 2", err)
	}
	if _, _, err := client.CICDInfrastructure.GetByName(context.Background(), "Shared", defectdojo.CICDInfrastructureTypeOrchestration); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(other case) error = %v, want ErrNotFound", err)
	}
}

func TestCICDInfrastructureService_GetByName_query(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/cicd_infrastructure/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "infrastructure_type=scm_server&limit=100&name=GitHub")
		writeJSON(t, w, http.StatusOK, []byte(`{"count":1,"results":[{"id":7,"name":"GitHub","infrastructure_type":"scm_server"}]}`))
	})
	if ci, _, err := client.CICDInfrastructure.GetByName(context.Background(), "GitHub", "scm_server"); err != nil || ci.ID != 7 {
		t.Errorf("GetByName() = %v, %v; want ID 7", ci, err)
	}
}

func TestCICDInfrastructureService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/cicd_infrastructure/", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "Content-Type", "application/json")
		// The ID is not sent, and the optional fields are sent as "".
		testJSONBody(t, r, `{"name":"Jenkins","infrastructure_type":"build_server","description":"","url":""}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "cicd_infrastructure/get.json"))
	})

	in := &defectdojo.CICDInfrastructure{ID: 99, Name: "Jenkins", InfrastructureType: "build_server"}
	ci, resp, err := client.CICDInfrastructure.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || ci.ID != 8 {
		t.Errorf("Create() = %d %+v", resp.StatusCode, ci)
	}
	if in.ID != 99 {
		t.Errorf("Create() changed the caller's object: %+v", in)
	}
}

func TestCICDInfrastructureService_Create_duplicate(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/cicd_infrastructure/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/cicd_infrastructure_duplicate.json"))
	})
	_, _, err := client.CICDInfrastructure.Create(context.Background(), &defectdojo.CICDInfrastructure{Name: "Spike SCM", InfrastructureType: "scm_server"})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["non_field_errors"]) != 1 {
		t.Errorf("Create() error = %v, want a 400 with a non_field_errors error", err)
	}
}

func TestCICDInfrastructureService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/cicd_infrastructure/8/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"name":"N","infrastructure_type":"build_server","description":"d","url":"https://ci.example.com"}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "cicd_infrastructure/get.json"))
	})
	_, _, err := client.CICDInfrastructure.Update(context.Background(), 8, &defectdojo.CICDInfrastructure{
		Name: "N", InfrastructureType: "build_server", Description: "d", URL: "https://ci.example.com",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestCICDInfrastructureService_Patch(t *testing.T) {
	tests := []struct {
		name  string
		patch *defectdojo.CICDInfrastructurePatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.CICDInfrastructurePatch{}, want: `{}`},
		{name: "clear", patch: &defectdojo.CICDInfrastructurePatch{Description: new(""), URL: new("")}, want: `{"description":"","url":""}`},
		{name: "rename", patch: &defectdojo.CICDInfrastructurePatch{Name: new("N")}, want: `{"name":"N"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/cicd_infrastructure/8/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "cicd_infrastructure/get.json"))
			})
			if _, _, err := client.CICDInfrastructure.Patch(context.Background(), 8, tt.patch); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestCICDInfrastructureService_Patch_conflict(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PATCH /api/v2/cicd_infrastructure/8/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusConflict, fixture(t, "errors/cicd_infrastructure_conflict.json"))
	})
	_, _, err := client.CICDInfrastructure.Patch(context.Background(), 8, &defectdojo.CICDInfrastructurePatch{Name: new("Taken")})
	if !errors.Is(err, defectdojo.ErrConflict) {
		t.Errorf("Patch() error = %v, want ErrConflict", err)
	}
}

func TestCICDInfrastructureService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/cicd_infrastructure/8/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.CICDInfrastructure.Delete(context.Background(), 8)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Errorf("Delete() = %v, %v; want 204", resp, err)
	}
}

func TestCICDInfrastructureService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/cicd_infrastructure/8/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=10")
		writeJSON(t, w, http.StatusOK, fixture(t, "cicd_infrastructure/delete_preview.json"))
	})
	preview, _, err := client.CICDInfrastructure.DeletePreview(context.Background(), 8, &defectdojo.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 1 || *preview.Objects[0] != (defectdojo.DeletedObject{Model: "CICDInfrastructure", ID: 8, Name: "Jenkins"}) {
		t.Errorf("DeletePreview() = %d %+v", preview.Count, preview.Objects)
	}
}

func TestCICDInfrastructureService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	svc := client.CICDInfrastructure
	valid := defectdojo.CICDInfrastructure{Name: "N", InfrastructureType: "scm_server"}
	calls := map[string]func() error{
		"Get zero ID":     func() error { _, _, err := svc.Get(ctx, 0); return err },
		"GetByName empty": func() error { _, _, err := svc.GetByName(ctx, "", "scm_server"); return err },
		"Create nil":      func() error { _, _, err := svc.Create(ctx, nil); return err },
		"Create empty name": func() error {
			_, _, err := svc.Create(ctx, &defectdojo.CICDInfrastructure{InfrastructureType: "scm_server"})
			return err
		},
		"Create empty type": func() error { _, _, err := svc.Create(ctx, &defectdojo.CICDInfrastructure{Name: "N"}); return err },
		"Update zero ID":    func() error { _, _, err := svc.Update(ctx, 0, &valid); return err },
		"Update empty type": func() error {
			_, _, err := svc.Update(ctx, 1, &defectdojo.CICDInfrastructure{Name: "N"})
			return err
		},
		"Patch zero ID":         func() error { _, _, err := svc.Patch(ctx, 0, &defectdojo.CICDInfrastructurePatch{}); return err },
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

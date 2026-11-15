package defectdojo_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestRegulationsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/regulations/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=2&name=Payment+Card+Industry+Data+Security+Standard")
		writeJSON(t, w, http.StatusOK, fixture(t, "regulations/list.json"))
	})

	list, _, err := client.Regulations.List(context.Background(), &defectdojo.ListRegulationsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 2},
		Name:        "Payment Card Industry Data Security Standard",
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 26 || len(list.Regulations) != 2 {
		t.Fatalf("List() = %d of %d, want 2 of 26", len(list.Regulations), list.Count)
	}
	if pci := list.Regulations[0]; pci.ID != 1 || pci.Acronym != "PCI DSS" || pci.Category != defectdojo.RegulationCategoryFinance ||
		pci.Jurisdiction != "United States" || pci.Reference == "" {
		t.Errorf("first regulation = %+v, want the PCI DSS fixture", pci)
	}
}

func TestRegulationsService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/regulations/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":"x","results":[{"id":1,"acronym":"A"}]}`))
		case "1":
			testQuery(t, r, "limit=1&offset=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":null,"results":[{"id":2,"acronym":"B"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})

	var acronyms []string
	for r, err := range client.Regulations.ListAll(context.Background(), &defectdojo.ListRegulationsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1},
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		acronyms = append(acronyms, r.Acronym)
	}
	if fmt.Sprint(acronyms) != "[A B]" {
		t.Errorf("ListAll() = %v, want [A B]", acronyms)
	}
}

func TestRegulationsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/regulations/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "29" {
			writeJSON(t, w, http.StatusNotFound, fixture(t, "errors/not_found.json"))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "regulations/get.json"))
	})

	got, _, err := client.Regulations.Get(context.Background(), 29)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := defectdojo.Regulation{
		ID: 29, Name: "GDPR Test", Acronym: "GDPR-T", Category: defectdojo.RegulationCategoryPrivacy,
		Jurisdiction: "European Union", Description: "A test regulation.", Reference: "https://example.com/gdpr",
	}
	if *got != want {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
	if _, _, err := client.Regulations.Get(context.Background(), 30); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestRegulationsService_GetByName(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/regulations/", func(w http.ResponseWriter, r *http.Request) {
		// A filter that matched loosely is checked again on the client side.
		writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":1,"name":"gdpr"},{"id":2,"name":"GDPR"}]}`))
	})

	r, _, err := client.Regulations.GetByName(context.Background(), "GDPR")
	if err != nil || r.ID != 2 {
		t.Errorf("GetByName() = %v, %v; want ID 2", r, err)
	}
	if _, _, err := client.Regulations.GetByName(context.Background(), "Gdpr"); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(other case) error = %v, want ErrNotFound", err)
	}
}

func TestRegulationsService_GetByAcronym(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/regulations/", func(w http.ResponseWriter, r *http.Request) {
		// There is no acronym filter: GetByAcronym pages through all of them.
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
			results += fmt.Sprintf(`{"id":%d,"acronym":"R-%d"}`, i+1, i)
		}
		writeJSON(t, w, http.StatusOK, []byte(`{"count":150,"results":[`+results+`]}`))
	})

	r, _, err := client.Regulations.GetByAcronym(context.Background(), "R-120")
	if err != nil || r.ID != 121 {
		t.Errorf("GetByAcronym() = %v, %v; want ID 121 from the second page", r, err)
	}
	for _, acronym := range []string{"r-120", "R-1200"} {
		if _, _, err := client.Regulations.GetByAcronym(context.Background(), acronym); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("GetByAcronym(%q) error = %v, want ErrNotFound", acronym, err)
		}
	}
}

func TestRegulationsService_GetByAcronym_error(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/regulations/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, fixture(t, "errors/invalid_token.json"))
	})
	if _, _, err := client.Regulations.GetByAcronym(context.Background(), "x"); !errors.Is(err, defectdojo.ErrForbidden) {
		t.Errorf("GetByAcronym() error = %v, want ErrForbidden", err)
	}
}

func TestRegulationsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/regulations/", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "Content-Type", "application/json")
		// The ID is not sent, and the optional fields are sent as "".
		testJSONBody(t, r, `{"name":"GDPR Test","acronym":"GDPR-T","category":"privacy","jurisdiction":"European Union","description":"","reference":""}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "regulations/get.json"))
	})

	in := &defectdojo.Regulation{ID: 99, Name: "GDPR Test", Acronym: "GDPR-T", Category: "privacy", Jurisdiction: "European Union"}
	r, resp, err := client.Regulations.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || r.ID != 29 {
		t.Errorf("Create() = %d %+v", resp.StatusCode, r)
	}
	if in.ID != 99 {
		t.Errorf("Create() changed the caller's regulation: %+v", in)
	}
}

func TestRegulationsService_Create_duplicate(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/regulations/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/regulation_duplicate_name.json"))
	})
	_, _, err := client.Regulations.Create(context.Background(), &defectdojo.Regulation{
		Name: "GDPR Test", Acronym: "X", Category: "privacy", Jurisdiction: "EU",
	})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["name"]) != 1 {
		t.Errorf("Create() error = %v, want a 400 with a name field error", err)
	}
}

func TestRegulationsService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/regulations/29/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"name":"N","acronym":"A","category":"other","jurisdiction":"J","description":"d","reference":"https://example.com"}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "regulations/get.json"))
	})

	_, _, err := client.Regulations.Update(context.Background(), 29, &defectdojo.Regulation{
		Name: "N", Acronym: "A", Category: defectdojo.RegulationCategoryOther, Jurisdiction: "J", Description: "d", Reference: "https://example.com",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestRegulationsService_Patch(t *testing.T) {
	tests := []struct {
		name  string
		patch *defectdojo.RegulationPatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.RegulationPatch{}, want: `{}`},
		{name: "clear", patch: &defectdojo.RegulationPatch{Description: new(""), Reference: new("")}, want: `{"description":"","reference":""}`},
		{
			name:  "set fields",
			patch: &defectdojo.RegulationPatch{Acronym: new("A"), Category: new("medical"), Jurisdiction: new("J")},
			want:  `{"acronym":"A","category":"medical","jurisdiction":"J"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/regulations/29/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "regulations/get.json"))
			})
			if _, _, err := client.Regulations.Patch(context.Background(), 29, tt.patch); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestRegulationsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/regulations/29/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.Regulations.Delete(context.Background(), 29)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("StatusCode = %d, want 204", resp.StatusCode)
	}
}

func TestRegulationsService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/regulations/29/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=10")
		writeJSON(t, w, http.StatusOK, fixture(t, "regulations/delete_preview.json"))
	})
	preview, _, err := client.Regulations.DeletePreview(context.Background(), 29, &defectdojo.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 1 || *preview.Objects[0] != (defectdojo.DeletedObject{Model: "Regulation", ID: 29, Name: "GDPR-T (European Union)"}) {
		t.Errorf("DeletePreview() = %d %+v", preview.Count, preview.Objects)
	}
}

func TestRegulationsService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	valid := defectdojo.Regulation{Name: "N", Acronym: "A", Category: "privacy", Jurisdiction: "J"}
	with := func(f func(*defectdojo.Regulation)) *defectdojo.Regulation {
		r := valid
		f(&r)
		return &r
	}
	calls := map[string]func() error{
		"Get zero ID":        func() error { _, _, err := client.Regulations.Get(ctx, 0); return err },
		"GetByName empty":    func() error { _, _, err := client.Regulations.GetByName(ctx, ""); return err },
		"GetByAcronym empty": func() error { _, _, err := client.Regulations.GetByAcronym(ctx, ""); return err },
		"Create nil":         func() error { _, _, err := client.Regulations.Create(ctx, nil); return err },
		"Create empty name": func() error {
			_, _, err := client.Regulations.Create(ctx, with(func(r *defectdojo.Regulation) { r.Name = "" }))
			return err
		},
		"Create empty acronym": func() error {
			_, _, err := client.Regulations.Create(ctx, with(func(r *defectdojo.Regulation) { r.Acronym = "" }))
			return err
		},
		"Create empty jurisdiction": func() error {
			_, _, err := client.Regulations.Create(ctx, with(func(r *defectdojo.Regulation) { r.Jurisdiction = "" }))
			return err
		},
		"Create unknown category": func() error {
			_, _, err := client.Regulations.Create(ctx, with(func(r *defectdojo.Regulation) { r.Category = "Privacy" }))
			return err
		},
		"Update negative ID":    func() error { _, _, err := client.Regulations.Update(ctx, -1, &valid); return err },
		"Patch zero ID":         func() error { _, _, err := client.Regulations.Patch(ctx, 0, &defectdojo.RegulationPatch{}); return err },
		"Patch nil":             func() error { _, _, err := client.Regulations.Patch(ctx, 1, nil); return err },
		"Delete zero ID":        func() error { _, err := client.Regulations.Delete(ctx, 0); return err },
		"DeletePreview zero ID": func() error { _, _, err := client.Regulations.DeletePreview(ctx, 0, nil); return err },
	}
	for name, call := range calls {
		if err := call(); err == nil {
			t.Errorf("%s: error = nil, want an argument error", name)
		}
	}
}

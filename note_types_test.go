package defectdojo_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestNoteTypesService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/note_type/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "is_active=false&is_mandatory=true&limit=5")
		writeJSON(t, w, http.StatusOK, fixture(t, "note_types/list.json"))
	})

	list, _, err := client.NoteTypes.List(context.Background(), &defectdojo.ListNoteTypesOptions{
		ListOptions: defectdojo.ListOptions{Limit: 5},
		IsActive:    new(false),
		IsMandatory: new(true),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 2 || len(list.NoteTypes) != 2 {
		t.Fatalf("List() = %d of %d, want 2 of 2", len(list.NoteTypes), list.Count)
	}
	if review := list.NoteTypes[0]; review.Name != "Code Review" || review.IsSingle || review.IsActive || review.IsMandatory {
		t.Errorf("first note type = %+v, want the Code Review fixture", review)
	}
}

func TestNoteTypesService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/note_type/", func(w http.ResponseWriter, r *http.Request) {
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
	for nt, err := range client.NoteTypes.ListAll(context.Background(), &defectdojo.ListNoteTypesOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1},
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, nt.Name)
	}
	if fmt.Sprint(names) != "[a b]" {
		t.Errorf("ListAll() = %v, want [a b]", names)
	}
}

func TestNoteTypesService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/note_type/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "6" {
			writeJSON(t, w, http.StatusNotFound, fixture(t, "errors/not_found.json"))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "note_types/get.json"))
	})

	got, _, err := client.NoteTypes.Get(context.Background(), 6)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := defectdojo.NoteType{ID: 6, Name: "Sign-off", Description: "The release sign-off.", IsSingle: true, IsMandatory: true}
	if *got != want {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
	if _, _, err := client.NoteTypes.Get(context.Background(), 7); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestNoteTypesService_GetByName(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/note_type/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "name=Sign-off")
		// A filter that matched loosely is checked again on the client side.
		writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":1,"name":"sign-off"},{"id":6,"name":"Sign-off"}]}`))
	})

	nt, _, err := client.NoteTypes.GetByName(context.Background(), "Sign-off")
	if err != nil || nt.ID != 6 {
		t.Errorf("GetByName() = %v, %v; want ID 6", nt, err)
	}
}

func TestNoteTypesService_GetByName_notFound(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/note_type/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"count":0,"results":[]}`))
	})
	if _, _, err := client.NoteTypes.GetByName(context.Background(), "x"); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName() error = %v, want ErrNotFound", err)
	}
}

func TestNoteTypesService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/note_type/", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "Content-Type", "application/json")
		// The ID is not sent, and every boolean is: unset ones are false.
		testJSONBody(t, r, `{"name":"Sign-off","description":"The release sign-off.","is_single":true,"is_active":false,"is_mandatory":false}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "note_types/get.json"))
	})

	in := &defectdojo.NoteType{ID: 99, Name: "Sign-off", Description: "The release sign-off.", IsSingle: true}
	nt, resp, err := client.NoteTypes.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || nt.ID != 6 {
		t.Errorf("Create() = %d %+v", resp.StatusCode, nt)
	}
	if in.ID != 99 {
		t.Errorf("Create() changed the caller's note type: %+v", in)
	}
}

func TestNoteTypesService_Create_duplicate(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/note_type/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/note_type_duplicate.json"))
	})
	_, _, err := client.NoteTypes.Create(context.Background(), &defectdojo.NoteType{Name: "Spike NT", Description: "d"})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["name"]) != 1 {
		t.Errorf("Create() error = %v, want a 400 with a name field error", err)
	}
}

func TestNoteTypesService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/note_type/6/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"name":"N","description":"D","is_single":false,"is_active":true,"is_mandatory":true}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "note_types/get.json"))
	})
	_, _, err := client.NoteTypes.Update(context.Background(), 6, &defectdojo.NoteType{Name: "N", Description: "D", IsActive: true, IsMandatory: true})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestNoteTypesService_Patch(t *testing.T) {
	tests := []struct {
		name  string
		patch *defectdojo.NoteTypePatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.NoteTypePatch{}, want: `{}`},
		{name: "flags", patch: &defectdojo.NoteTypePatch{IsActive: new(false), IsMandatory: new(true)}, want: `{"is_active":false,"is_mandatory":true}`},
		{name: "text", patch: &defectdojo.NoteTypePatch{Name: new("N"), Description: new("D")}, want: `{"name":"N","description":"D"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/note_type/6/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "note_types/get.json"))
			})
			if _, _, err := client.NoteTypes.Patch(context.Background(), 6, tt.patch); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestNoteTypesService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/note_type/6/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.NoteTypes.Delete(context.Background(), 6)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("StatusCode = %d, want 204", resp.StatusCode)
	}
}

func TestNoteTypesService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/note_type/5/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=10")
		writeJSON(t, w, http.StatusOK, fixture(t, "note_types/delete_preview.json"))
	})
	preview, _, err := client.NoteTypes.DeletePreview(context.Background(), 5, &defectdojo.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 7 || *preview.Objects[0] != (defectdojo.DeletedObject{Model: "Note_Type", ID: 5, Name: "Code Review"}) {
		t.Errorf("DeletePreview() = %d %+v", preview.Count, preview.Objects[0])
	}
}

func TestNoteTypesService_CountNotes(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		mux, client := setup(t)
		mux.HandleFunc("GET /api/v2/note_type/5/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
			testQuery(t, r, "limit=250")
			writeJSON(t, w, http.StatusOK, fixture(t, "note_types/delete_preview.json"))
		})
		// Two notes, with the note type, their history and their test links.
		if n, _, err := client.NoteTypes.CountNotes(context.Background(), 5); err != nil || n != 2 {
			t.Errorf("CountNotes() = %d, %v; want 2", n, err)
		}
	})
	t.Run("pages", func(t *testing.T) {
		mux, client := setup(t)
		mux.HandleFunc("GET /api/v2/note_type/5/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Query().Get("offset") {
			case "":
				testQuery(t, r, "limit=250")
				results := `{"model":"Note_Type","id":5,"name":"x"}`
				for i := range 99 {
					results += fmt.Sprintf(`,{"model":"Notes","id":%d,"name":"n"}`, i+1)
				}
				writeJSON(t, w, http.StatusOK, []byte(`{"count":102,"results":[`+results+`]}`))
			case "100":
				// A server that caps the page size: the next page starts
				// after the entries it returned.
				testQuery(t, r, "limit=250&offset=100")
				writeJSON(t, w, http.StatusOK, []byte(`{"count":102,"results":[{"model":"Notes","id":100,"name":"n"},{"model":"Test_notes","id":1,"name":"t"}]}`))
			default:
				t.Errorf("unexpected query %q", r.URL.RawQuery)
			}
		})
		if n, _, err := client.NoteTypes.CountNotes(context.Background(), 5); err != nil || n != 100 {
			t.Errorf("CountNotes() = %d, %v; want 100", n, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		mux, client := setup(t)
		mux.HandleFunc("GET /api/v2/note_type/5/delete_preview/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusNotFound, fixture(t, "errors/not_found.json"))
		})
		if _, _, err := client.NoteTypes.CountNotes(context.Background(), 5); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("CountNotes() error = %v, want ErrNotFound", err)
		}
	})
}

func TestNoteTypesService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	valid := defectdojo.NoteType{Name: "N", Description: "D"}
	calls := map[string]func() error{
		"Get zero ID":     func() error { _, _, err := client.NoteTypes.Get(ctx, 0); return err },
		"GetByName empty": func() error { _, _, err := client.NoteTypes.GetByName(ctx, ""); return err },
		"Create nil":      func() error { _, _, err := client.NoteTypes.Create(ctx, nil); return err },
		"Create empty name": func() error {
			_, _, err := client.NoteTypes.Create(ctx, &defectdojo.NoteType{Description: "D"})
			return err
		},
		"Create empty description": func() error {
			_, _, err := client.NoteTypes.Create(ctx, &defectdojo.NoteType{Name: "N"})
			return err
		},
		"Update negative ID":    func() error { _, _, err := client.NoteTypes.Update(ctx, -1, &valid); return err },
		"Patch zero ID":         func() error { _, _, err := client.NoteTypes.Patch(ctx, 0, &defectdojo.NoteTypePatch{}); return err },
		"Patch nil":             func() error { _, _, err := client.NoteTypes.Patch(ctx, 1, nil); return err },
		"Delete zero ID":        func() error { _, err := client.NoteTypes.Delete(ctx, 0); return err },
		"DeletePreview zero ID": func() error { _, _, err := client.NoteTypes.DeletePreview(ctx, 0, nil); return err },
		"CountNotes zero ID":    func() error { _, _, err := client.NoteTypes.CountNotes(ctx, 0); return err },
	}
	for name, call := range calls {
		if err := call(); err == nil {
			t.Errorf("%s: error = nil, want an argument error", name)
		}
	}
}

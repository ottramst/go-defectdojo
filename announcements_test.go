package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestAnnouncementsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/announcements/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=5")
		writeJSON(t, w, http.StatusOK, fixture(t, "announcements/list.json"))
	})

	list, _, err := client.Announcements.List(context.Background(), &defectdojo.ListAnnouncementsOptions{ListOptions: defectdojo.ListOptions{Limit: 5}})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := defectdojo.Announcement{ID: 1, Message: "Maintenance on <b>Saturday</b>.", Style: defectdojo.AnnouncementStyleWarning, Dismissable: true}
	if list.Count != 1 || len(list.Results) != 1 || list.Results[0] != want {
		t.Errorf("List() = %d %+v, want %+v", list.Count, list.Results, want)
	}
}

func TestAnnouncementsService_List_filters(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/announcements/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "dismissable=false&message=Maintenance+on+Saturday&style=warning")
		writeJSON(t, w, http.StatusOK, []byte(`{"count":0,"results":[]}`))
	})
	_, _, err := client.Announcements.List(context.Background(), &defectdojo.ListAnnouncementsOptions{
		Message:     "Maintenance on Saturday",
		Style:       defectdojo.AnnouncementStyleWarning,
		Dismissable: new(false),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
}

func TestAnnouncementsService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/announcements/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=100")
		writeJSON(t, w, http.StatusOK, fixture(t, "announcements/list.json"))
	})
	n := 0
	for a, err := range client.Announcements.ListAll(context.Background(), nil) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		if a.ID != 1 {
			t.Errorf("ListAll() yielded %+v", a)
		}
		n++
	}
	if n != 1 {
		t.Errorf("ListAll() yielded %d announcements, want 1", n)
	}
}

func TestAnnouncementsService_Current(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		mux, client := setup(t)
		mux.HandleFunc("GET /api/v2/announcements/", func(w http.ResponseWriter, r *http.Request) {
			testQuery(t, r, "")
			writeJSON(t, w, http.StatusOK, fixture(t, "announcements/list.json"))
		})
		a, _, err := client.Announcements.Current(context.Background(), nil)
		if err != nil || a.ID != 1 || a.Style != "warning" {
			t.Errorf("Current() = %+v, %v; want the fixture", a, err)
		}
	})
	t.Run("none", func(t *testing.T) {
		mux, client := setup(t)
		mux.HandleFunc("GET /api/v2/announcements/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusOK, []byte(`{"count":0,"next":null,"previous":null,"results":[]}`))
		})
		if _, _, err := client.Announcements.Current(context.Background(), nil); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("Current() error = %v, want ErrNotFound", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mux, client := setup(t)
		mux.HandleFunc("GET /api/v2/announcements/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusForbidden, fixture(t, "errors/invalid_token.json"))
		})
		if _, _, err := client.Announcements.Current(context.Background(), nil); !errors.Is(err, defectdojo.ErrForbidden) {
			t.Errorf("Current() error = %v, want ErrForbidden", err)
		}
	})
}

func TestAnnouncementsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/announcements/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "1" {
			writeJSON(t, w, http.StatusNotFound, fixture(t, "errors/not_found.json"))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "announcements/get.json"))
	})
	a, _, err := client.Announcements.Get(context.Background(), 1, nil)
	if err != nil || a.Message != "Maintenance on <b>Saturday</b>." || !a.Dismissable {
		t.Errorf("Get() = %+v, %v; want the fixture", a, err)
	}
	if _, _, err := client.Announcements.Get(context.Background(), 2, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(2) error = %v, want ErrNotFound", err)
	}
}

func TestAnnouncementsService_Create(t *testing.T) {
	tests := []struct {
		name string
		in   *defectdojo.Announcement
		want string
	}{
		// The ID is not sent.
		{name: "every field", in: &defectdojo.Announcement{ID: 9, Message: "m", Style: "danger", Dismissable: true}, want: `{"message":"m","style":"danger","dismissable":true}`},
		// An empty style is left out, for DefectDojo's default; "" is a 400.
		{name: "default style", in: &defectdojo.Announcement{Message: "m"}, want: `{"message":"m","dismissable":false}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("POST /api/v2/announcements/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusCreated, fixture(t, "announcements/get.json"))
			})
			a, resp, err := client.Announcements.Create(context.Background(), tt.in, nil)
			if err != nil || resp.StatusCode != http.StatusCreated || a.ID != 1 {
				t.Errorf("Create() = %+v, %v", a, err)
			}
		})
	}
}

func TestAnnouncementsService_Create_exists(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/announcements/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/announcement_exists.json"))
	})
	_, _, err := client.Announcements.Create(context.Background(), &defectdojo.Announcement{Message: "second"}, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || errResp.Message != "No more than one Announcement is allowed" {
		t.Errorf("Create() error = %v, want the 400 for a second announcement", err)
	}
}

func TestAnnouncementsService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/announcements/1/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"message":"m","style":"success","dismissable":false}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "announcements/get.json"))
	})
	if _, _, err := client.Announcements.Update(context.Background(), 1, &defectdojo.Announcement{Message: "m", Style: "success"}, nil); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestAnnouncementsService_Patch(t *testing.T) {
	tests := []struct {
		name  string
		patch *defectdojo.AnnouncementPatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.AnnouncementPatch{}, want: `{}`},
		{name: "dismissable", patch: &defectdojo.AnnouncementPatch{Dismissable: new(false)}, want: `{"dismissable":false}`},
		{name: "text", patch: &defectdojo.AnnouncementPatch{Message: new("m"), Style: new(defectdojo.AnnouncementStyle("info"))}, want: `{"message":"m","style":"info"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/announcements/1/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "announcements/get.json"))
			})
			if _, _, err := client.Announcements.Patch(context.Background(), 1, tt.patch, nil); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestAnnouncementsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/announcements/1/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.Announcements.Delete(context.Background(), 1, nil)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Errorf("Delete() = %v, %v; want 204", resp, err)
	}
}

func TestAnnouncementsService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/announcements/1/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=10")
		writeJSON(t, w, http.StatusOK, fixture(t, "announcements/delete_preview.json"))
	})
	preview, _, err := client.Announcements.DeletePreview(context.Background(), 1, &defectdojo.DeletePreviewAnnouncementOptions{ListOptions: defectdojo.ListOptions{Limit: 10}})
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 2 || preview.Results[0].Model != "Announcement" || preview.Results[1].Model != "UserAnnouncement" {
		t.Errorf("DeletePreview() = %d %+v", preview.Count, preview.Results)
	}
}

func TestAnnouncementsService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	svc := client.Announcements
	calls := map[string]func() error{
		"Get zero ID":           func() error { _, _, err := svc.Get(ctx, 0, nil); return err },
		"Create nil":            func() error { _, _, err := svc.Create(ctx, nil, nil); return err },
		"Create empty message":  func() error { _, _, err := svc.Create(ctx, &defectdojo.Announcement{Style: "info"}, nil); return err },
		"Update zero ID":        func() error { _, _, err := svc.Update(ctx, 0, &defectdojo.Announcement{Message: "m"}, nil); return err },
		"Update empty message":  func() error { _, _, err := svc.Update(ctx, 1, &defectdojo.Announcement{}, nil); return err },
		"Patch zero ID":         func() error { _, _, err := svc.Patch(ctx, 0, &defectdojo.AnnouncementPatch{}, nil); return err },
		"Patch nil":             func() error { _, _, err := svc.Patch(ctx, 1, nil, nil); return err },
		"Delete zero ID":        func() error { _, err := svc.Delete(ctx, 0, nil); return err },
		"DeletePreview zero ID": func() error { _, _, err := svc.DeletePreview(ctx, 0, nil); return err },
	}
	for name, call := range calls {
		if err := call(); err == nil {
			t.Errorf("%s: error = nil, want an argument error", name)
		}
	}
}

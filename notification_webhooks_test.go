package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestNotificationWebhooksService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notification_webhooks/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=5&name=Chat+relay&owner=3&status=active")
		writeJSON(t, w, http.StatusOK, fixture(t, "notification_webhooks/list.json"))
	})

	list, _, err := client.NotificationWebhooks.List(context.Background(), &defectdojo.ListNotificationWebhooksOptions{
		ListOptions: defectdojo.ListOptions{Limit: 5},
		Name:        "Chat relay",
		Status:      defectdojo.NotificationWebhookStatusActive,
		Owner:       3,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 1 || list.Results[0].Name != "Chat relay" {
		t.Errorf("List() = %d %+v", list.Count, list.Results)
	}
}

func TestNotificationWebhooksService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notification_webhooks/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":1,"name":"a"}]}`))
		case "1":
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":2,"name":"b"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})
	var names []string
	for wh, err := range client.NotificationWebhooks.ListAll(context.Background(), &defectdojo.ListNotificationWebhooksOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1},
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, wh.Name)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestNotificationWebhooksService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notification_webhooks/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "4" {
			writeJSON(t, w, http.StatusNotFound, []byte(notFound))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "notification_webhooks/get.json"))
	})

	wh, _, err := client.NotificationWebhooks.Get(context.Background(), 4, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := &defectdojo.NotificationWebhook{
		ID:          4,
		Name:        "Chat relay",
		URL:         "https://hooks.example.com/defectdojo",
		HeaderName:  "Authorization",
		HeaderValue: "Bearer fake-token",
		Status:      defectdojo.NotificationWebhookStatusActive,
	}
	if !reflect.DeepEqual(wh, want) {
		t.Errorf("Get() = %+v, want %+v", wh, want)
	}
	if _, _, err := client.NotificationWebhooks.Get(context.Background(), 5, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestNotificationWebhooksService_GetByName(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notification_webhooks/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("name") {
		case "Chat relay", "Chat":
			// A server whose filter matches partially must not match.
			writeJSON(t, w, http.StatusOK, fixture(t, "notification_webhooks/list.json"))
		default:
			writeJSON(t, w, http.StatusOK, []byte(`{"count":0,"results":[]}`))
		}
	})
	wh, _, err := client.NotificationWebhooks.GetByName(context.Background(), "Chat relay", nil)
	if err != nil || wh.ID != 4 {
		t.Errorf("GetByName() = %+v, %v; want ID 4", wh, err)
	}
	for _, name := range []string{"Chat", "missing"} {
		if _, _, err := client.NotificationWebhooks.GetByName(context.Background(), name, nil); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("GetByName(%q) error = %v, want ErrNotFound", name, err)
		}
	}
}

func TestNotificationWebhooksService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/notification_webhooks/", func(w http.ResponseWriter, r *http.Request) {
		// Read-only fields are not sent; empty headers and owner are null.
		testJSONBody(t, r, `{"name":"Chat relay","url":"https://hooks.example.com/defectdojo","header_name":null,"header_value":null,"owner":null}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "notification_webhooks/get.json"))
	})
	in := &defectdojo.NotificationWebhook{
		ID: 99, Name: "Chat relay", URL: "https://hooks.example.com/defectdojo", Status: defectdojo.NotificationWebhookStatusInactivePermanent, Note: "x",
	}
	wh, resp, err := client.NotificationWebhooks.Create(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || wh.ID != 4 || in.ID != 99 {
		t.Errorf("Create() = %d %+v, caller's %+v", resp.StatusCode, wh, in)
	}
}

func TestNotificationWebhooksService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/notification_webhooks/4/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"name":"n","url":"https://x.example","header_name":"Authorization","header_value":"Bearer t","owner":3}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "notification_webhooks/get.json"))
	})
	_, _, err := client.NotificationWebhooks.Update(context.Background(), 4, &defectdojo.NotificationWebhook{
		Name: "n", URL: "https://x.example", HeaderName: "Authorization", HeaderValue: "Bearer t", Owner: 3,
	}, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestNotificationWebhooksService_Patch(t *testing.T) {
	tests := []struct {
		name  string
		patch *defectdojo.NotificationWebhookPatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.NotificationWebhookPatch{}, want: `{}`},
		{
			name:  "clear header",
			patch: &defectdojo.NotificationWebhookPatch{HeaderName: defectdojo.Null[string](), HeaderValue: defectdojo.Null[string]()},
			want:  `{"header_name":null,"header_value":null}`,
		},
		{
			name:  "set fields",
			patch: &defectdojo.NotificationWebhookPatch{URL: new("https://y.example"), HeaderValue: defectdojo.Value("Bearer new"), Owner: defectdojo.Value(3)},
			want:  `{"url":"https://y.example","header_value":"Bearer new","owner":3}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/notification_webhooks/4/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "notification_webhooks/get.json"))
			})
			if _, _, err := client.NotificationWebhooks.Patch(context.Background(), 4, tt.patch, nil); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestNotificationWebhooksService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/notification_webhooks/4/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if resp, err := client.NotificationWebhooks.Delete(context.Background(), 4, nil); err != nil || resp.StatusCode != http.StatusNoContent {
		t.Errorf("Delete() = %v, %v; want 204", resp, err)
	}
}

func TestNotificationWebhooksService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notification_webhooks/4/delete_preview/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "notification_webhooks/delete_preview.json"))
	})
	preview, _, err := client.NotificationWebhooks.DeletePreview(context.Background(), 4, nil)
	if err != nil || preview.Count != 1 || preview.Results[0].Model != "Notification_Webhooks" {
		t.Errorf("DeletePreview() = %+v, %v", preview, err)
	}
}

func TestNotificationWebhooksService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	wh := &defectdojo.NotificationWebhook{Name: "x", URL: "https://x.example"}
	calls := map[string]func() error{
		"Get zero ID":     func() error { _, _, err := client.NotificationWebhooks.Get(ctx, 0, nil); return err },
		"GetByName empty": func() error { _, _, err := client.NotificationWebhooks.GetByName(ctx, "", nil); return err },
		"Create nil":      func() error { _, _, err := client.NotificationWebhooks.Create(ctx, nil, nil); return err },
		"Create empty name": func() error {
			_, _, err := client.NotificationWebhooks.Create(ctx, &defectdojo.NotificationWebhook{URL: "https://x.example"}, nil)
			return err
		},
		"Create empty URL": func() error {
			_, _, err := client.NotificationWebhooks.Create(ctx, &defectdojo.NotificationWebhook{Name: "x"}, nil)
			return err
		},
		"Update zero ID": func() error { _, _, err := client.NotificationWebhooks.Update(ctx, 0, wh, nil); return err },
		"Update nil":     func() error { _, _, err := client.NotificationWebhooks.Update(ctx, 1, nil, nil); return err },
		"Patch zero ID": func() error {
			_, _, err := client.NotificationWebhooks.Patch(ctx, 0, &defectdojo.NotificationWebhookPatch{}, nil)
			return err
		},
		"Patch nil":             func() error { _, _, err := client.NotificationWebhooks.Patch(ctx, 1, nil, nil); return err },
		"Delete zero ID":        func() error { _, err := client.NotificationWebhooks.Delete(ctx, 0, nil); return err },
		"DeletePreview zero ID": func() error { _, _, err := client.NotificationWebhooks.DeletePreview(ctx, 0, nil); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("error = nil, want error")
			}
		})
	}
}

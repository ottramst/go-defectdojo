package defectdojo_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestNotificationChannels_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		in   string
		want defectdojo.NotificationChannels
	}{
		{in: `["mail","alert"]`, want: defectdojo.NotificationChannels{"mail", "alert"}},
		{in: `[]`, want: defectdojo.NotificationChannels{}},
		{in: `"mail"`, want: defectdojo.NotificationChannels{"mail"}},
		{in: `""`, want: defectdojo.NotificationChannels{}},
		{in: `null`, want: nil},
	}
	for _, tt := range tests {
		var got defectdojo.NotificationChannels
		if err := json.Unmarshal([]byte(tt.in), &got); err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Unmarshal(%s) = %#v, %v; want %#v", tt.in, got, err, tt.want)
		}
	}
	var got defectdojo.NotificationChannels
	if err := json.Unmarshal([]byte(`1`), &got); err == nil {
		t.Error("Unmarshal(1) error = nil, want error")
	}
}

func TestNotificationsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notifications/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=2&product=284&template=false&user=1")
		writeJSON(t, w, http.StatusOK, fixture(t, "notifications/list.json"))
	})
	list, _, err := client.Notifications.List(context.Background(), &defectdojo.ListNotificationsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 2}, User: 1, Product: 284, Template: new(false),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 2 || list.Results[0].Product != 0 || list.Results[1].Product != 284 {
		t.Errorf("List() = %d %+v", list.Count, list.Results)
	}
	if got := list.Results[1].ScanAddedEmpty; !reflect.DeepEqual(got, defectdojo.NotificationChannels{"mail"}) {
		t.Errorf("ScanAddedEmpty = %#v, want [mail]", got)
	}
}

func TestNotificationsService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notifications/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=1&template=true")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":1}]}`))
		case "1":
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":2}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})
	var ids []int
	for n, err := range client.Notifications.ListAll(context.Background(), &defectdojo.ListNotificationsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1}, Template: new(true),
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		ids = append(ids, n.ID)
	}
	if want := []int{1, 2}; !reflect.DeepEqual(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestNotificationsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notifications/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "2" {
			writeJSON(t, w, http.StatusNotFound, []byte(notFound))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "notifications/get.json"))
	})
	n, _, err := client.Notifications.Get(context.Background(), 2, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	// The system-wide settings: no user or product, DefectDojo's defaults.
	alert := []string{"alert"}
	if n.ID != 2 || n.User != 0 || n.Product != 0 || n.Template ||
		!reflect.DeepEqual(n.ProductTypeAdded, alert) || !reflect.DeepEqual(n.RiskAcceptanceExpiration, alert) ||
		!reflect.DeepEqual(n.ScanAddedEmpty, defectdojo.NotificationChannels{}) {
		t.Errorf("Get() = %+v", n)
	}
	if _, _, err := client.Notifications.Get(context.Background(), 5, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestNotificationsService_GetByScope(t *testing.T) {
	rows := []string{
		`{"id":1,"user":1,"product":null,"template":false}`,
		`{"id":2,"user":null,"product":null,"template":false}`,
		`{"id":3,"user":2,"product":null,"template":false}`,
		`{"id":4,"user":1,"product":7,"template":false}`,
	}
	for i := 5; i < 105; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":%d,"user":%d,"product":null,"template":false}`, i, i))
	}
	rows = append(rows, `{"id":200,"user":null,"product":null,"template":true}`)
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notifications/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("user") == "99" {
			writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/invalid_filter.json"))
			return
		}
		// A server that ignores the filters: GetByScope matches itself.
		offset, _ := strconv.Atoi(q.Get("offset"))
		end := min(offset+100, len(rows))
		page := "["
		for i, row := range rows[offset:end] {
			if i > 0 {
				page += ","
			}
			page += row
		}
		writeJSON(t, w, http.StatusOK, []byte(`{"count":`+strconv.Itoa(len(rows))+`,"results":`+page+`]}`))
	})
	ctx := context.Background()
	tests := []struct {
		user, product int
		template      bool
		want          int
	}{
		{user: 0, product: 0, want: 2},
		{user: 1, product: 0, want: 1},
		{user: 1, product: 7, want: 4},
		{template: true, want: 200},
	}
	for _, tt := range tests {
		n, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: tt.user, Product: tt.product, Template: tt.template}, nil)
		if err != nil || n.ID != tt.want {
			t.Errorf("GetByScope(%d, %d, %t) = %+v, %v; want ID %d", tt.user, tt.product, tt.template, n, err, tt.want)
		}
	}
	for _, scope := range [][2]int{{2, 7}, {99, 0}} {
		if _, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: scope[0], Product: scope[1], Template: false}, nil); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("GetByScope(%v) error = %v, want ErrNotFound", scope, err)
		}
	}
}

func TestNotificationsService_GetByScope_several(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notifications/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":2,"user":null},{"id":9,"user":null}]}`))
	})
	_, _, err := client.Notifications.GetByScope(context.Background(), defectdojo.NotificationScope{User: 0, Product: 0, Template: false}, nil)
	if !errors.Is(err, defectdojo.ErrAmbiguous) || errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByScope() error = %v, want an error naming the rows", err)
	}
}

func TestNotificationsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/notifications/", func(w http.ResponseWriter, r *http.Request) {
		// Nil events are left out; empty ones are sent; ScanAddedEmpty is
		// one string.
		testJSONBody(t, r, `{"user":1,"product":284,"template":false,"product_added":["mail","alert"],"sla_breach":[],"scan_added_empty":"mail"}`)
		// The response has scan_added_empty as a string.
		writeJSON(t, w, http.StatusCreated, fixture(t, "notifications/create.json"))
	})
	n, _, err := client.Notifications.Create(context.Background(), &defectdojo.NotificationSettings{
		ID: 99, User: 1, Product: 284,
		ProductAdded:   []string{"mail", "alert"},
		SLABreach:      []string{},
		ScanAddedEmpty: defectdojo.NotificationChannels{"mail"},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if n.ID != 35 || !reflect.DeepEqual(n.ScanAddedEmpty, defectdojo.NotificationChannels{"mail"}) || len(n.SLABreach) != 0 {
		t.Errorf("Create() = %+v", n)
	}
}

func TestNotificationsService_Update(t *testing.T) {
	tests := []struct {
		name string
		in   *defectdojo.NotificationSettings
		want string
	}{
		{
			// The scope is always sent, as null when it is zero.
			name: "system",
			in:   &defectdojo.NotificationSettings{Other: []string{"mail"}, ScanAddedEmpty: defectdojo.NotificationChannels{}},
			want: `{"user":null,"product":null,"template":false,"other":["mail"],"scan_added_empty":""}`,
		},
		{
			name: "template",
			in:   &defectdojo.NotificationSettings{Template: true},
			want: `{"user":null,"product":null,"template":true,"scan_added_empty":""}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PUT /api/v2/notifications/2/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "notifications/get.json"))
			})
			if _, _, err := client.Notifications.Update(context.Background(), 2, tt.in, nil); err != nil {
				t.Fatalf("Update() error = %v", err)
			}
		})
	}
}

func TestNotificationsService_Patch(t *testing.T) {
	var none []string
	tests := []struct {
		name  string
		patch *defectdojo.NotificationSettingsPatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.NotificationSettingsPatch{}, want: `{}`},
		{
			name: "set events",
			patch: &defectdojo.NotificationSettingsPatch{
				ProductTypeAdded: new([]string{"slack"}), RiskAcceptanceExpiration: new([]string{}), ScanAddedEmpty: new(""),
			},
			want: `{"product_type_added":["slack"],"risk_acceptance_expiration":[],"scan_added_empty":""}`,
		},
		{name: "nil slice", patch: &defectdojo.NotificationSettingsPatch{Other: &none}, want: `{"other":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/notifications/2/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "notifications/get.json"))
			})
			if _, _, err := client.Notifications.Patch(context.Background(), 2, tt.patch, nil); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
	if none != nil {
		t.Error("Patch() changed the caller's patch")
	}
}

func TestNotificationsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/notifications/35/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if resp, err := client.Notifications.Delete(context.Background(), 35, nil); err != nil || resp.StatusCode != http.StatusNoContent {
		t.Errorf("Delete() = %v, %v; want 204", resp, err)
	}
}

func TestNotificationsService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/notifications/35/delete_preview/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "notifications/delete_preview.json"))
	})
	preview, _, err := client.Notifications.DeletePreview(context.Background(), 35, nil)
	if err != nil || preview.Count != 1 || preview.Results[0].Model != "Notifications" {
		t.Errorf("DeletePreview() = %+v, %v", preview, err)
	}
}

func TestNotificationsService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	n := &defectdojo.NotificationSettings{}
	calls := map[string]func() error{
		"Get zero ID": func() error { _, _, err := client.Notifications.Get(ctx, 0, nil); return err },
		"GetByScope negative user": func() error {
			_, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: -1, Product: 0, Template: false}, nil)
			return err
		},
		"GetByScope template with user": func() error {
			_, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: 1, Product: 0, Template: true}, nil)
			return err
		},
		"Create nil": func() error { _, _, err := client.Notifications.Create(ctx, nil, nil); return err },
		"Create template with product": func() error {
			_, _, err := client.Notifications.Create(ctx, &defectdojo.NotificationSettings{Template: true, Product: 1}, nil)
			return err
		},
		"Create two scan_added_empty channels": func() error {
			_, _, err := client.Notifications.Create(ctx, &defectdojo.NotificationSettings{ScanAddedEmpty: defectdojo.NotificationChannels{"mail", "alert"}}, nil)
			return err
		},
		"Update zero ID": func() error { _, _, err := client.Notifications.Update(ctx, 0, n, nil); return err },
		"Update nil":     func() error { _, _, err := client.Notifications.Update(ctx, 1, nil, nil); return err },
		"Patch zero ID": func() error {
			_, _, err := client.Notifications.Patch(ctx, 0, &defectdojo.NotificationSettingsPatch{}, nil)
			return err
		},
		"Patch nil":             func() error { _, _, err := client.Notifications.Patch(ctx, 1, nil, nil); return err },
		"Delete zero ID":        func() error { _, err := client.Notifications.Delete(ctx, 0, nil); return err },
		"DeletePreview zero ID": func() error { _, _, err := client.Notifications.DeletePreview(ctx, 0, nil); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("error = nil, want error")
			}
		})
	}
}

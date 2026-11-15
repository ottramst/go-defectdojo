package defectdojo_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/ottramst/go-defectdojo"
)

func TestUsersService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "email=example.com&is_active=true&limit=2&username=a")
		writeJSON(t, w, http.StatusOK, fixture(t, "users/list.json"))
	})

	list, _, err := client.Users.List(context.Background(), &defectdojo.ListUsersOptions{
		ListOptions: defectdojo.ListOptions{Limit: 2},
		Username:    "a",
		Email:       "example.com",
		IsActive:    new(true),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list.Results) != 2 || list.Results[0].Username != "admin" || !list.Results[0].IsSuperuser {
		t.Errorf("List() = %d users, first %+v; want admin first", len(list.Results), list.Results[0])
	}
}

func TestUsersService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "is_superuser=false&limit=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":"x","results":[{"id":2,"username":"a"}]}`))
		case "1":
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":null,"results":[{"id":3,"username":"b"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})

	var names []string
	for u, err := range client.Users.ListAll(context.Background(), &defectdojo.ListUsersOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1},
		IsSuperuser: new(false),
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, u.Username)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestUsersService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/users/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "7" {
			writeJSON(t, w, http.StatusNotFound, []byte(notFound))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "users/get.json"))
	})

	u, _, err := client.Users.Get(context.Background(), 7, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := &defectdojo.User{
		ID:                       7,
		Username:                 "dependency-track",
		FirstName:                "Dependency",
		LastName:                 "Track",
		Email:                    "dt@example.com",
		IsActive:                 true,
		ConfigurationPermissions: []int{9, 389},
		DateJoined:               time.Date(2026, time.November, 14, 14, 18, 16, 110685000, time.UTC),
		TokenLastReset:           time.Date(2026, time.November, 14, 14, 18, 16, 193997000, time.UTC),
	}
	if !reflect.DeepEqual(u, want) {
		t.Errorf("Get() = %+v, want %+v", u, want)
	}

	if _, _, err := client.Users.Get(context.Background(), 5, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestUsersService_GetByUsername(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		// The filter is a case-insensitive contains match: GetByUsername
		// pages through what it matches.
		if got := r.URL.Query().Get("username"); got != "ci" {
			t.Errorf("username filter = %q, want ci", got)
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		results := ""
		for i := offset; i < min(offset+100, 120); i++ {
			if results != "" {
				results += ","
			}
			results += fmt.Sprintf(`{"id":%d,"username":"ci-%d"}`, i+1, i)
		}
		if offset == 100 {
			results += `,{"id":500,"username":"CI"},{"id":501,"username":"ci"}`
		}
		writeJSON(t, w, http.StatusOK, []byte(`{"count":122,"results":[`+results+`]}`))
	})

	u, _, err := client.Users.GetByUsername(context.Background(), "ci", nil)
	if err != nil || u.ID != 501 {
		t.Errorf("GetByUsername() = %v, %v; want ID 501, not the case variant", u, err)
	}
}

func TestUsersService_GetByUsername_notFound(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/users/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"count":1,"results":[{"id":2,"username":"Alice"}]}`))
	})
	for _, name := range []string{"alice", "Ali"} {
		if _, _, err := client.Users.GetByUsername(context.Background(), name, nil); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("GetByUsername(%q) error = %v, want ErrNotFound", name, err)
		}
	}
}

func TestUsersService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		// Read-only fields and nil permissions are not sent; the password is.
		testJSONBody(t, r, `{"username":"dependency-track","first_name":"","last_name":"","email":"dt@example.com",`+
			`"is_active":true,"is_staff":false,"is_superuser":false,"password":"Secret-Passw0rd!"}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "users/get.json"))
	})

	in := &defectdojo.CreateUserRequest{
		User:     defectdojo.User{ID: 99, Username: "dependency-track", Email: "dt@example.com", IsActive: true, DateJoined: time.Now()},
		Password: "Secret-Passw0rd!",
	}
	u, resp, err := client.Users.Create(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || u.ID != 7 {
		t.Errorf("Create() = %d %+v", resp.StatusCode, u)
	}
	if in.ID != 99 || in.ConfigurationPermissions != nil {
		t.Errorf("Create() changed the caller's user: %+v", in)
	}
}

func TestUsersService_Create_withoutPassword(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		// No password is left out (single sign-on users), and so are empty
		// permissions.
		testJSONBody(t, r, `{"username":"sso","first_name":"","last_name":"","email":"sso@example.com",`+
			`"is_active":false,"is_staff":false,"is_superuser":false}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "users/get.json"))
	})
	_, _, err := client.Users.Create(context.Background(), &defectdojo.CreateUserRequest{
		User: defectdojo.User{Username: "sso", Email: "sso@example.com", ConfigurationPermissions: []int{}},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestUsersService_Update(t *testing.T) {
	tests := []struct {
		name string
		user *defectdojo.User
		want string
	}{
		{
			// nil permissions keep the user's permissions: [] would remove
			// every Django permission the user has.
			name: "nil permissions",
			user: &defectdojo.User{Username: "dt", Email: "dt@example.com", FirstName: "D", IsActive: true, IsStaff: true},
			want: `{"username":"dt","first_name":"D","last_name":"","email":"dt@example.com","is_active":true,"is_staff":true,"is_superuser":false}`,
		},
		{
			// A user read with Get has an empty, not a nil, list: sending it
			// would remove every Django permission the user has.
			name: "empty permissions",
			user: &defectdojo.User{Username: "dt", Email: "dt@example.com", ConfigurationPermissions: []int{}},
			want: `{"username":"dt","first_name":"","last_name":"","email":"dt@example.com","is_active":false,"is_staff":false,"is_superuser":false}`,
		},
		{
			name: "permissions",
			user: &defectdojo.User{Username: "dt", Email: "dt@example.com", ConfigurationPermissions: []int{9}},
			want: `{"username":"dt","first_name":"","last_name":"","email":"dt@example.com","is_active":false,"is_staff":false,"is_superuser":false,"configuration_permissions":[9]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PUT /api/v2/users/7/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "users/get.json"))
			})
			if _, _, err := client.Users.Update(context.Background(), 7, tt.user, nil); err != nil {
				t.Fatalf("Update() error = %v", err)
			}
		})
	}
}

func TestUsersService_Patch(t *testing.T) {
	var nilPermissions []int
	tests := []struct {
		name  string
		patch *defectdojo.UserPatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.UserPatch{}, want: `{}`},
		{name: "clear name", patch: &defectdojo.UserPatch{FirstName: new("")}, want: `{"first_name":""}`},
		{
			name:  "set fields",
			patch: &defectdojo.UserPatch{Email: new("x@example.com"), IsActive: new(false), ConfigurationPermissions: new([]int{9, 10})},
			want:  `{"email":"x@example.com","is_active":false,"configuration_permissions":[9,10]}`,
		},
		{name: "nil permissions", patch: &defectdojo.UserPatch{ConfigurationPermissions: &nilPermissions}, want: `{"configuration_permissions":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/users/7/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "users/get.json"))
			})
			if _, _, err := client.Users.Patch(context.Background(), 7, tt.patch, nil); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
	if nilPermissions != nil {
		t.Error("Patch() changed the caller's patch")
	}
}

func TestUsersService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/users/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "1" {
			writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/delete_self.json"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if _, err := client.Users.Delete(context.Background(), 7, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err := client.Users.Delete(context.Background(), 1, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || errResp.Message != "Users may not delete themselves" {
		t.Errorf("Delete(self) error = %v, want a 400 with the message", err)
	}
}

func TestUsersService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/users/7/delete_preview/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "users/delete_preview.json"))
	})
	preview, _, err := client.Users.DeletePreview(context.Background(), 7, nil)
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 7 || *preview.Results[4] != (defectdojo.DeletedObject{Model: "Notifications", ID: 11, Name: "Notifications about all projects for Dependency Track (dependency-track)"}) {
		t.Errorf("DeletePreview() = %d %+v", preview.Count, preview.Results)
	}
}

func TestUsersService_ResetAPIToken(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/users/7/reset_api_token/", func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > 0 {
			t.Errorf("request has a body of %d bytes, want none", r.ContentLength)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.Users.ResetAPIToken(context.Background(), 7, nil)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Errorf("ResetAPIToken() = %v, %v; want 204", resp, err)
	}
}

func TestUsersService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	u := &defectdojo.User{Username: "x", Email: "x@example.com"}
	calls := map[string]func() error{
		"Get zero ID":         func() error { _, _, err := client.Users.Get(ctx, 0, nil); return err },
		"GetByUsername empty": func() error { _, _, err := client.Users.GetByUsername(ctx, "", nil); return err },
		"Create nil":          func() error { _, _, err := client.Users.Create(ctx, nil, nil); return err },
		"Create empty username": func() error {
			_, _, err := client.Users.Create(ctx, &defectdojo.CreateUserRequest{User: defectdojo.User{Email: "x@example.com"}}, nil)
			return err
		},
		"Create empty email": func() error {
			_, _, err := client.Users.Create(ctx, &defectdojo.CreateUserRequest{User: defectdojo.User{Username: "x"}}, nil)
			return err
		},
		"Update negative ID":    func() error { _, _, err := client.Users.Update(ctx, -1, u, nil); return err },
		"Update nil":            func() error { _, _, err := client.Users.Update(ctx, 1, nil, nil); return err },
		"Patch zero ID":         func() error { _, _, err := client.Users.Patch(ctx, 0, &defectdojo.UserPatch{}, nil); return err },
		"Patch nil":             func() error { _, _, err := client.Users.Patch(ctx, 1, nil, nil); return err },
		"Delete zero ID":        func() error { _, err := client.Users.Delete(ctx, 0, nil); return err },
		"DeletePreview zero ID": func() error { _, _, err := client.Users.DeletePreview(ctx, 0, nil); return err },
		"ResetAPIToken zero ID": func() error { _, err := client.Users.ResetAPIToken(ctx, 0, nil); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("error = nil, want error")
			}
		})
	}
}

func TestConfigurationPermissionsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/configuration_permissions/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=3&offset=1")
		writeJSON(t, w, http.StatusOK, fixture(t, "configuration_permissions/list.json"))
	})
	list, _, err := client.ConfigurationPermissions.List(context.Background(), &defectdojo.ListConfigurationPermissionsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 3, Offset: 1},
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 52 || len(list.Results) != 3 ||
		*list.Results[1] != (defectdojo.ConfigurationPermission{ID: 9, Name: "Can add user", Codename: "add_user"}) {
		t.Errorf("List() = %d %+v", list.Count, list.Results)
	}
}

func TestConfigurationPermissionsService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/configuration_permissions/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=1&name=x")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":9,"codename":"add_user"}]}`))
		case "1":
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":10,"codename":"change_user"}]}`))
		}
	})
	var codenames []string
	for p, err := range client.ConfigurationPermissions.ListAll(context.Background(), &defectdojo.ListConfigurationPermissionsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1}, Name: "x",
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		codenames = append(codenames, p.Codename)
	}
	if want := []string{"add_user", "change_user"}; !reflect.DeepEqual(codenames, want) {
		t.Errorf("codenames = %v, want %v", codenames, want)
	}
}

func TestConfigurationPermissionsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/configuration_permissions/389/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "configuration_permissions/get.json"))
	})
	p, _, err := client.ConfigurationPermissions.Get(context.Background(), 389, nil)
	if err != nil || *p != (defectdojo.ConfigurationPermission{ID: 389, Name: "Can add sl a_ configuration", Codename: "add_sla_configuration"}) {
		t.Errorf("Get() = %+v, %v", p, err)
	}
}

func TestConfigurationPermissionsService_GetByCodename(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/configuration_permissions/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("codename") {
		case "view_user":
			testQuery(t, r, "codename=view_user")
			writeJSON(t, w, http.StatusOK, fixture(t, "configuration_permissions/codename.json"))
		case "view":
			// A server whose filter matches partially must not match.
			writeJSON(t, w, http.StatusOK, fixture(t, "configuration_permissions/codename.json"))
		default:
			writeJSON(t, w, http.StatusOK, []byte(`{"count":0,"results":[]}`))
		}
	})
	p, _, err := client.ConfigurationPermissions.GetByCodename(context.Background(), "view_user", nil)
	if err != nil || p.ID != 12 {
		t.Errorf("GetByCodename() = %+v, %v; want ID 12", p, err)
	}
	for _, codename := range []string{"view", "change_jira_instance"} {
		if _, _, err := client.ConfigurationPermissions.GetByCodename(context.Background(), codename, nil); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("GetByCodename(%q) error = %v, want ErrNotFound", codename, err)
		}
	}
}

func TestConfigurationPermissionsService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	if _, _, err := client.ConfigurationPermissions.Get(context.Background(), 0, nil); err == nil {
		t.Error("Get(0) error = nil, want error")
	}
	if _, _, err := client.ConfigurationPermissions.GetByCodename(context.Background(), "", nil); err == nil {
		t.Error("GetByCodename(\"\") error = nil, want error")
	}
}

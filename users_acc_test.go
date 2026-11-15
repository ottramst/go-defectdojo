package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

// accPassword meets DefectDojo's default password rules.
const accPassword = "Acc-Test-Passw0rd!"

func TestAccUsers(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	username := randName(t, "acc")

	created, resp, err := client.Users.Create(ctx, &defectdojo.CreateUserRequest{
		User:     defectdojo.User{Username: username, Email: username + "@example.com", FirstName: " Acc ", IsActive: true},
		Password: accPassword,
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanupUser(t, client, created.ID)
	if resp.StatusCode != http.StatusCreated || created.ID == 0 || created.Username != username || created.FirstName != "Acc" ||
		!created.IsActive || created.IsStaff || created.DateJoined.IsZero() || len(created.ConfigurationPermissions) != 0 {
		t.Errorf("Create() = %+v (%d), want the new user with a trimmed first name", created, resp.StatusCode)
	}

	// The password works.
	if _, _, err := client.Auth.Token(ctx, username, accPassword, nil); err != nil {
		t.Errorf("Auth.Token() with the new user's password error = %v", err)
	}

	// Usernames are unique, but only case-sensitively, and a password is
	// required by default.
	_, _, err = client.Users.Create(ctx, &defectdojo.CreateUserRequest{
		User: defectdojo.User{Username: username, Email: "x@example.com"}, Password: accPassword,
	}, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["username"]) == 0 {
		t.Errorf("Create(same username) error = %v, want a 400 with a username field error", err)
	}
	if _, _, err := client.Users.Create(ctx, &defectdojo.CreateUserRequest{
		User: defectdojo.User{Username: randName(t, "acc"), Email: "x@example.com"},
	}, nil); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Create(without password) error = %v, want ErrBadRequest", err)
	}
	upper, _, err := client.Users.Create(ctx, &defectdojo.CreateUserRequest{
		User: defectdojo.User{Username: strings.ToUpper(username), Email: username + "@example.com"}, Password: accPassword,
	}, nil)
	if err != nil {
		t.Fatalf("Create(case variant) error = %v", err)
	}
	cleanupUser(t, client, upper.ID)

	got, _, err := client.Users.Get(ctx, created.ID, nil)
	if err != nil || got.Username != username || got.Email != username+"@example.com" {
		t.Errorf("Get() = %+v, %v", got, err)
	}
	for name, want := range map[string]int{username: created.ID, strings.ToUpper(username): upper.ID} {
		if u, _, err := client.Users.GetByUsername(ctx, name, nil); err != nil || u.ID != want {
			t.Errorf("GetByUsername(%q) = %+v, %v; want ID %d", name, u, err, want)
		}
	}
	if _, _, err := client.Users.GetByUsername(ctx, username[:len(username)-1], nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByUsername(prefix) error = %v, want ErrNotFound", err)
	}

	// Configuration permissions are set by ID, looked up by codename.
	var permissions []int
	for _, codename := range []string{"add_user", "change_sla_configuration"} {
		p, _, err := client.ConfigurationPermissions.GetByCodename(ctx, codename, nil)
		if err != nil {
			t.Fatalf("GetByCodename(%q) error = %v", codename, err)
		}
		permissions = append(permissions, p.ID)
	}

	// Read, modify, write.
	got.LastName = "Test"
	got.IsStaff = true
	got.ConfigurationPermissions = permissions
	updated, _, err := client.Users.Update(ctx, got.ID, got, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.LastName != "Test" || updated.FirstName != "Acc" || !updated.IsStaff || !sameInts(updated.ConfigurationPermissions, permissions) {
		t.Errorf("Update() = %+v, want the new last name, staff and permissions %v", updated, permissions)
	}

	// Empty permissions keep them; names are cleared with "".
	updated.ConfigurationPermissions = []int{}
	if kept, _, err := client.Users.Update(ctx, got.ID, updated, nil); err != nil || !sameInts(kept.ConfigurationPermissions, permissions) {
		t.Errorf("Update(empty permissions) = %+v, %v; want the permissions kept", kept, err)
	}
	patched, _, err := client.Users.Patch(ctx, got.ID, &defectdojo.UserPatch{FirstName: new(""), ConfigurationPermissions: new([]int{})}, nil)
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patched.FirstName != "" || len(patched.ConfigurationPermissions) != 0 || patched.LastName != "Test" {
		t.Errorf("Patch() = %+v, want no first name and no permissions", patched)
	}

	if _, err := client.Users.ResetAPIToken(ctx, created.ID, nil); err != nil {
		t.Errorf("ResetAPIToken() error = %v", err)
	}
	if reset, _, err := client.Users.Get(ctx, created.ID, nil); err != nil || reset.TokenLastReset.IsZero() {
		t.Errorf("Get() after ResetAPIToken() = %+v, %v; want TokenLastReset", reset, err)
	}

	found := false
	for u, err := range client.Users.ListAll(ctx, &defectdojo.ListUsersOptions{Username: username, ListOptions: defectdojo.ListOptions{Limit: 1}}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		found = found || u.ID == created.ID
	}
	if !found {
		t.Error("ListAll() did not return the user")
	}

	preview, _, err := client.Users.DeletePreview(ctx, created.ID, nil)
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if !slices.ContainsFunc(preview.Results, func(o *defectdojo.DeletedObject) bool { return o.Model == "Notifications" }) {
		t.Errorf("DeletePreview() = %+v, want the user's notification settings", preview.Results)
	}

	if _, err := client.Users.Delete(ctx, created.ID, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Users.Get(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestAccUsers_refused(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	profile, _, err := client.UserProfile.Get(ctx, nil)
	if err != nil {
		t.Fatalf("UserProfile.Get() error = %v", err)
	}
	self := profile.User.ID

	// The user the client authenticates as can't delete itself, nor reset
	// its own token here; the bodies are bare JSON.
	_, err = client.Users.Delete(ctx, self, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || errResp.Message != "Users may not delete themselves" {
		t.Errorf("Delete(self) error = %v, want a 400 with the message", err)
	}
	if _, err := client.Users.ResetAPIToken(ctx, self, nil); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("ResetAPIToken(self) error = %v, want ErrBadRequest", err)
	}

	// A user that a product refers to can't be deleted.
	u, _, err := client.Users.Create(ctx, &defectdojo.CreateUserRequest{
		User: defectdojo.User{Username: randName(t, "acc"), Email: "acc@example.com", IsActive: true}, Password: accPassword,
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanupUser(t, client, u.ID)
	_, _, err = client.Products.Create(ctx, &defectdojo.Product{
		Name: randName(t, "acc"), Description: "acceptance test", ProdType: createProductType(t, client), ProductManager: u.ID,
	}, nil)
	if err != nil {
		t.Fatalf("creating product: %v", err)
	}
	if _, err := client.Users.Delete(ctx, u.ID, nil); !errors.Is(err, defectdojo.ErrConflict) {
		t.Errorf("Delete(product manager) error = %v, want ErrConflict", err)
	}
	// A password in an update is refused.
	req, err := client.NewRequest(ctx, http.MethodPatch, "/api/v2/users/"+strconv.Itoa(u.ID)+"/", map[string]string{"password": accPassword + "2"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(req, nil); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("PATCH password error = %v, want ErrBadRequest", err)
	}
}

func TestAccConfigurationPermissions(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	var codenames []string
	for p, err := range client.ConfigurationPermissions.ListAll(ctx, nil) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		codenames = append(codenames, p.Codename)
	}
	for _, want := range []string{"add_user", "change_sla_configuration", "add_product_type"} {
		if !slices.Contains(codenames, want) {
			t.Errorf("ListAll() = %v, want %s", codenames, want)
		}
	}

	p, _, err := client.ConfigurationPermissions.GetByCodename(ctx, "add_product_type", nil)
	if err != nil {
		t.Fatalf("GetByCodename() error = %v", err)
	}
	if got, _, err := client.ConfigurationPermissions.Get(ctx, p.ID, nil); err != nil || *got != *p {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, p)
	}
	// Codenames are exact, and permissions that aren't configuration
	// permissions are not listed.
	for _, codename := range []string{"add_product", "ADD_PRODUCT_TYPE"} {
		if _, _, err := client.ConfigurationPermissions.GetByCodename(ctx, codename, nil); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("GetByCodename(%q) error = %v, want ErrNotFound", codename, err)
		}
	}
}

// cleanupUser deletes the user when the test ends, after the objects
// registered later, which may refer to it.
func cleanupUser(t *testing.T, client *defectdojo.Client, id int) {
	t.Helper()
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Users.Delete(ctx, id, nil)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
}

func sameInts(a, b []int) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

package defectdojo_test

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

// The tests don't run in parallel (no t.Parallel): they change the
// system-wide settings and the template, which other tests' users get.

func TestAccNotifications_systemAndPersonal(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	// DefectDojo creates the system-wide settings and admin's personal ones.
	system, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: 0, Product: 0, Template: false}, nil)
	if err != nil {
		t.Fatalf("GetByScope(system) error = %v", err)
	}
	original := *system
	cleanup(t, func(ctx context.Context) error {
		_, _, err := client.Notifications.Update(ctx, original.ID, &original, nil)
		return err
	})
	patched, _, err := client.Notifications.Patch(ctx, system.ID, &defectdojo.NotificationSettingsPatch{
		ProductAdded: new([]defectdojo.NotificationChannel{defectdojo.NotificationChannelMail, defectdojo.NotificationChannelAlert}),
		SLABreach:    new([]defectdojo.NotificationChannel{}),
		// A single channel, and the response holds it as a string.
		ScanAddedEmpty: new(defectdojo.NotificationChannelAlert),
	}, nil)
	if err != nil {
		t.Fatalf("Patch(system) error = %v", err)
	}
	if !sameStrings(patched.ProductAdded, []defectdojo.NotificationChannel{"mail", "alert"}) || len(patched.SLABreach) != 0 ||
		!reflect.DeepEqual(patched.ScanAddedEmpty, []defectdojo.NotificationChannel{"alert"}) || !reflect.DeepEqual(patched.Other, []defectdojo.NotificationChannel{"alert"}) {
		t.Errorf("Patch(system) = %+v", patched)
	}
	got, _, err := client.Notifications.Get(ctx, system.ID, nil)
	if err != nil || !reflect.DeepEqual(got.ScanAddedEmpty, []defectdojo.NotificationChannel{"alert"}) {
		t.Errorf("Get(system) = %+v, %v; want scan_added_empty [alert]", got, err)
	}

	// A PUT resets the events it leaves out, and keeps the scope.
	updated, _, err := client.Notifications.Update(ctx, system.ID, &defectdojo.NotificationSettings{Other: []defectdojo.NotificationChannel{"mail"}}, nil)
	if err != nil {
		t.Fatalf("Update(system) error = %v", err)
	}
	if updated.User != 0 || updated.Product != 0 || updated.Template || !reflect.DeepEqual(updated.Other, []defectdojo.NotificationChannel{"mail"}) ||
		!reflect.DeepEqual(updated.ProductAdded, []defectdojo.NotificationChannel{"alert"}) {
		t.Errorf("Update(system) = %+v, want other [mail] and the rest reset to alert", updated)
	}
	// scan_added_empty, which DefectDojo would keep, is reset too.
	if got, _, err := client.Notifications.Get(ctx, system.ID, nil); err != nil || len(got.ScanAddedEmpty) != 0 {
		t.Errorf("Get(system) after Update = %+v, %v; want no scan_added_empty channel", got, err)
	}

	// A new user gets personal settings.
	u, _, err := client.Users.Create(ctx, &defectdojo.CreateUserRequest{
		User: defectdojo.User{Username: randName(t, "acc"), Email: "acc@example.com", IsActive: true}, Password: accPassword,
	}, nil)
	if err != nil {
		t.Fatalf("Users.Create() error = %v", err)
	}
	cleanupUser(t, client, u.ID)
	personal, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: u.ID, Product: 0, Template: false}, nil)
	if err != nil || personal.User != u.ID || personal.Product != 0 {
		t.Fatalf("GetByScope(personal) = %+v, %v", personal, err)
	}
	// A second row for the scope is refused.
	if _, _, err := client.Notifications.Create(ctx, &defectdojo.NotificationSettings{User: u.ID}, nil); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Create(existing scope) error = %v, want ErrBadRequest", err)
	}
	// Deleting the user deletes them, and the user filter then answers 400.
	if _, err := client.Users.Delete(ctx, u.ID, nil); err != nil {
		t.Fatalf("Users.Delete() error = %v", err)
	}
	if _, _, err := client.Notifications.Get(ctx, personal.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(personal of deleted user) error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: u.ID, Product: 0, Template: false}, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByScope(deleted user) error = %v, want ErrNotFound", err)
	}
}

func TestAccNotifications_product(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	profile, _, err := client.UserProfile.Get(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	product, _, err := client.Products.Create(ctx, &defectdojo.Product{
		Name: randName(t, "acc"), Description: "acceptance test", ProdType: createProductType(t, client),
	}, nil)
	if err != nil {
		t.Fatalf("creating product: %v", err)
	}

	created, resp, err := client.Notifications.Create(ctx, &defectdojo.NotificationSettings{
		User: profile.User.ID, Product: product.ID,
		EngagementAdded: []defectdojo.NotificationChannel{defectdojo.NotificationChannelMail},
		ScanAddedEmpty:  []defectdojo.NotificationChannel{defectdojo.NotificationChannelMail},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Notifications.Delete(ctx, created.ID, nil)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	// Events that are left out get DefectDojo's default.
	if resp.StatusCode != http.StatusCreated || created.User != profile.User.ID || created.Product != product.ID ||
		!reflect.DeepEqual(created.EngagementAdded, []defectdojo.NotificationChannel{"mail"}) || !reflect.DeepEqual(created.TestAdded, []defectdojo.NotificationChannel{"alert"}) ||
		!reflect.DeepEqual(created.ScanAddedEmpty, []defectdojo.NotificationChannel{"mail"}) {
		t.Errorf("Create() = %+v (%d)", created, resp.StatusCode)
	}

	byScope, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: profile.User.ID, Product: product.ID, Template: false}, nil)
	if err != nil || byScope.ID != created.ID {
		t.Errorf("GetByScope(product) = %+v, %v; want ID %d", byScope, err, created.ID)
	}

	found := false
	for n, err := range client.Notifications.ListAll(ctx, &defectdojo.ListNotificationSettingsOptions{Product: product.ID}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		found = found || n.ID == created.ID
	}
	if !found {
		t.Error("ListAll(product) did not return the settings")
	}

	preview, _, err := client.Notifications.DeletePreview(ctx, created.ID, nil)
	if err != nil || !slices.ContainsFunc(preview.Results, func(o defectdojo.DeletedObject) bool { return o.Model == "Notifications" }) {
		t.Errorf("DeletePreview() = %+v, %v", preview, err)
	}
	if _, err := client.Notifications.Delete(ctx, created.ID, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Notifications.Get(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestAccNotifications_template(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	if existing, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: 0, Product: 0, Template: true}, nil); err == nil {
		t.Fatalf("a template exists already (ID %d); this test needs a server without one", existing.ID)
	}
	template, _, err := client.Notifications.Create(ctx, &defectdojo.NotificationSettings{
		Template: true, ProductAdded: []defectdojo.NotificationChannel{defectdojo.NotificationChannelSlack},
	}, nil)
	if err != nil {
		t.Fatalf("Create(template) error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Notifications.Delete(ctx, template.ID, nil)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	if !template.Template || template.User != 0 {
		t.Errorf("Create(template) = %+v", template)
	}

	// A second template, and a PUT of the template, are refused; a PATCH
	// works.
	if _, _, err := client.Notifications.Create(ctx, &defectdojo.NotificationSettings{Template: true}, nil); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Create(second template) error = %v, want ErrBadRequest", err)
	}
	if _, _, err := client.Notifications.Update(ctx, template.ID, &defectdojo.NotificationSettings{Template: true}, nil); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Update(template) error = %v, want ErrBadRequest", err)
	}
	patched, _, err := client.Notifications.Patch(ctx, template.ID, &defectdojo.NotificationSettingsPatch{
		ProductAdded: new([]defectdojo.NotificationChannel{defectdojo.NotificationChannelMail}),
	}, nil)
	if err != nil || !patched.Template || !reflect.DeepEqual(patched.ProductAdded, []defectdojo.NotificationChannel{"mail"}) {
		t.Errorf("Patch(template) = %+v, %v", patched, err)
	}
	if byScope, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: 0, Product: 0, Template: true}, nil); err != nil || byScope.ID != template.ID {
		t.Errorf("GetByScope(template) = %+v, %v; want ID %d", byScope, err, template.ID)
	}

	// A new user's personal settings are copied from the template.
	u, _, err := client.Users.Create(ctx, &defectdojo.CreateUserRequest{
		User: defectdojo.User{Username: randName(t, "acc"), Email: "acc@example.com", IsActive: true}, Password: accPassword,
	}, nil)
	if err != nil {
		t.Fatalf("Users.Create() error = %v", err)
	}
	cleanupUser(t, client, u.ID)
	personal, _, err := client.Notifications.GetByScope(ctx, defectdojo.NotificationScope{User: u.ID, Product: 0, Template: false}, nil)
	if err != nil || !reflect.DeepEqual(personal.ProductAdded, []defectdojo.NotificationChannel{"mail"}) || personal.Template {
		t.Errorf("GetByScope(new user) = %+v, %v; want the template's events", personal, err)
	}

	if _, err := client.Notifications.Delete(ctx, template.ID, nil); err != nil {
		t.Fatalf("Delete(template) error = %v", err)
	}
}

func sameStrings[T cmp.Ordered](a, b []T) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

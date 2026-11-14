package defectdojo_test

import (
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
	system, _, err := client.Notifications.GetByScope(ctx, 0, 0, false)
	if err != nil {
		t.Fatalf("GetByScope(system) error = %v", err)
	}
	original := *system
	cleanup(t, func(ctx context.Context) error {
		_, _, err := client.Notifications.Update(ctx, original.ID, &original)
		return err
	})
	patched, _, err := client.Notifications.Patch(ctx, system.ID, &defectdojo.NotificationSettingsPatch{
		ProductAdded: new([]string{defectdojo.NotificationChannelMail, defectdojo.NotificationChannelAlert}),
		SLABreach:    new([]string{}),
		// A single channel, and the response holds it as a string.
		ScanAddedEmpty: new(defectdojo.NotificationChannelAlert),
	})
	if err != nil {
		t.Fatalf("Patch(system) error = %v", err)
	}
	if !sameStrings(patched.ProductAdded, []string{"mail", "alert"}) || len(patched.SLABreach) != 0 ||
		!reflect.DeepEqual(patched.ScanAddedEmpty, defectdojo.NotificationChannels{"alert"}) || !reflect.DeepEqual(patched.Other, []string{"alert"}) {
		t.Errorf("Patch(system) = %+v", patched)
	}
	got, _, err := client.Notifications.Get(ctx, system.ID)
	if err != nil || !reflect.DeepEqual(got.ScanAddedEmpty, defectdojo.NotificationChannels{"alert"}) {
		t.Errorf("Get(system) = %+v, %v; want scan_added_empty [alert]", got, err)
	}

	// A PUT resets the events it leaves out, and keeps the scope.
	updated, _, err := client.Notifications.Update(ctx, system.ID, &defectdojo.NotificationSettings{Other: []string{"mail"}})
	if err != nil {
		t.Fatalf("Update(system) error = %v", err)
	}
	if updated.User != 0 || updated.Product != 0 || updated.Template || !reflect.DeepEqual(updated.Other, []string{"mail"}) ||
		!reflect.DeepEqual(updated.ProductAdded, []string{"alert"}) {
		t.Errorf("Update(system) = %+v, want other [mail] and the rest reset to alert", updated)
	}

	// A new user gets personal settings.
	u, _, err := client.Users.Create(ctx, &defectdojo.UserCreate{
		User: defectdojo.User{Username: randName(t, "acc"), Email: "acc@example.com", IsActive: true}, Password: accPassword,
	})
	if err != nil {
		t.Fatalf("Users.Create() error = %v", err)
	}
	cleanupUser(t, client, u.ID)
	personal, _, err := client.Notifications.GetByScope(ctx, u.ID, 0, false)
	if err != nil || personal.User != u.ID || personal.Product != 0 {
		t.Fatalf("GetByScope(personal) = %+v, %v", personal, err)
	}
	// A second row for the scope is refused.
	if _, _, err := client.Notifications.Create(ctx, &defectdojo.NotificationSettings{User: u.ID}); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Create(existing scope) error = %v, want ErrBadRequest", err)
	}
	// Deleting the user deletes them, and the user filter then answers 400.
	if _, err := client.Users.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Users.Delete() error = %v", err)
	}
	if _, _, err := client.Notifications.Get(ctx, personal.ID); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(personal of deleted user) error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.Notifications.GetByScope(ctx, u.ID, 0, false); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByScope(deleted user) error = %v, want ErrNotFound", err)
	}
}

func TestAccNotifications_product(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	profile, _, err := client.UserProfile.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	product, _, err := client.Products.Create(ctx, &defectdojo.Product{
		Name: randName(t, "acc"), Description: "acceptance test", ProdType: createProductType(t, client),
	})
	if err != nil {
		t.Fatalf("creating product: %v", err)
	}

	created, resp, err := client.Notifications.Create(ctx, &defectdojo.NotificationSettings{
		User: profile.User.ID, Product: product.ID,
		EngagementAdded: []string{defectdojo.NotificationChannelMail},
		ScanAddedEmpty:  defectdojo.NotificationChannels{defectdojo.NotificationChannelMail},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Notifications.Delete(ctx, created.ID)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	// Events that are left out get DefectDojo's default.
	if resp.StatusCode != http.StatusCreated || created.User != profile.User.ID || created.Product != product.ID ||
		!reflect.DeepEqual(created.EngagementAdded, []string{"mail"}) || !reflect.DeepEqual(created.TestAdded, []string{"alert"}) ||
		!reflect.DeepEqual(created.ScanAddedEmpty, defectdojo.NotificationChannels{"mail"}) {
		t.Errorf("Create() = %+v (%d)", created, resp.StatusCode)
	}

	byScope, _, err := client.Notifications.GetByScope(ctx, profile.User.ID, product.ID, false)
	if err != nil || byScope.ID != created.ID {
		t.Errorf("GetByScope(product) = %+v, %v; want ID %d", byScope, err, created.ID)
	}

	found := false
	for n, err := range client.Notifications.ListAll(ctx, &defectdojo.ListNotificationsOptions{Product: product.ID}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		found = found || n.ID == created.ID
	}
	if !found {
		t.Error("ListAll(product) did not return the settings")
	}

	preview, _, err := client.Notifications.DeletePreview(ctx, created.ID, nil)
	if err != nil || !slices.ContainsFunc(preview.Objects, func(o *defectdojo.DeletedObject) bool { return o.Model == "Notifications" }) {
		t.Errorf("DeletePreview() = %+v, %v", preview, err)
	}
	if _, err := client.Notifications.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Notifications.Get(ctx, created.ID); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestAccNotifications_template(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	if existing, _, err := client.Notifications.GetByScope(ctx, 0, 0, true); err == nil {
		t.Fatalf("a template exists already (ID %d); this test needs a server without one", existing.ID)
	}
	template, _, err := client.Notifications.Create(ctx, &defectdojo.NotificationSettings{
		Template: true, ProductAdded: []string{defectdojo.NotificationChannelSlack},
	})
	if err != nil {
		t.Fatalf("Create(template) error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Notifications.Delete(ctx, template.ID)
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
	if _, _, err := client.Notifications.Create(ctx, &defectdojo.NotificationSettings{Template: true}); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Create(second template) error = %v, want ErrBadRequest", err)
	}
	if _, _, err := client.Notifications.Update(ctx, template.ID, &defectdojo.NotificationSettings{Template: true}); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Update(template) error = %v, want ErrBadRequest", err)
	}
	patched, _, err := client.Notifications.Patch(ctx, template.ID, &defectdojo.NotificationSettingsPatch{
		ProductAdded: new([]string{defectdojo.NotificationChannelMail}),
	})
	if err != nil || !patched.Template || !reflect.DeepEqual(patched.ProductAdded, []string{"mail"}) {
		t.Errorf("Patch(template) = %+v, %v", patched, err)
	}
	if byScope, _, err := client.Notifications.GetByScope(ctx, 0, 0, true); err != nil || byScope.ID != template.ID {
		t.Errorf("GetByScope(template) = %+v, %v; want ID %d", byScope, err, template.ID)
	}

	// A new user's personal settings are copied from the template.
	u, _, err := client.Users.Create(ctx, &defectdojo.UserCreate{
		User: defectdojo.User{Username: randName(t, "acc"), Email: "acc@example.com", IsActive: true}, Password: accPassword,
	})
	if err != nil {
		t.Fatalf("Users.Create() error = %v", err)
	}
	cleanupUser(t, client, u.ID)
	personal, _, err := client.Notifications.GetByScope(ctx, u.ID, 0, false)
	if err != nil || !reflect.DeepEqual(personal.ProductAdded, []string{"mail"}) || personal.Template {
		t.Errorf("GetByScope(new user) = %+v, %v; want the template's events", personal, err)
	}

	if _, err := client.Notifications.Delete(ctx, template.ID); err != nil {
		t.Fatalf("Delete(template) error = %v", err)
	}
}

func sameStrings(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

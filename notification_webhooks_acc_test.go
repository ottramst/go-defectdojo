package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestAccNotificationWebhooks(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	name := randName(t, "acc")

	// DefectDojo doesn't test the URL, so one that can't resolve is fine.
	created, resp, err := client.NotificationWebhooks.Create(ctx, &defectdojo.NotificationWebhook{
		Name:        name,
		URL:         "https://hooks.invalid/defectdojo",
		HeaderName:  "Authorization",
		HeaderValue: "Bearer acc-test",
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.NotificationWebhooks.Delete(ctx, created.ID, nil)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	want := defectdojo.NotificationWebhook{
		ID: created.ID, Name: name, URL: "https://hooks.invalid/defectdojo",
		HeaderName: "Authorization", HeaderValue: "Bearer acc-test", Status: defectdojo.NotificationWebhookStatusActive,
	}
	if resp.StatusCode != http.StatusCreated || *created != want {
		t.Errorf("Create() = %+v (%d), want %+v", created, resp.StatusCode, want)
	}

	// Names are unique: a duplicate is a 400 with a field error.
	_, _, err = client.NotificationWebhooks.Create(ctx, &defectdojo.NotificationWebhook{Name: name, URL: "https://hooks.invalid/x"}, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["name"]) == 0 {
		t.Errorf("Create(same name) error = %v, want a 400 with a name field error", err)
	}

	got, _, err := client.NotificationWebhooks.Get(ctx, created.ID, nil)
	if err != nil || *got != want {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, want)
	}
	if byName, _, err := client.NotificationWebhooks.GetByName(ctx, name, nil); err != nil || byName.ID != created.ID {
		t.Errorf("GetByName() = %+v, %v; want ID %d", byName, err, created.ID)
	}

	// Read, modify, write: an empty header is cleared.
	got.Name = name + "-renamed"
	got.HeaderName, got.HeaderValue = "", ""
	updated, _, err := client.NotificationWebhooks.Update(ctx, got.ID, got, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != got.Name || updated.HeaderName != "" || updated.HeaderValue != "" || updated.URL != want.URL {
		t.Errorf("Update() = %+v, want the renamed webhook without a header", updated)
	}

	// The status can't be changed through the API: DefectDojo ignores it.
	req, err := client.NewRequest(ctx, http.MethodPatch, "/api/v2/notification_webhooks/"+strconv.Itoa(created.ID)+"/",
		map[string]string{"status": string(defectdojo.NotificationWebhookStatusInactivePermanent), "header_value": "Bearer rotated"})
	if err != nil {
		t.Fatal(err)
	}
	var patched defectdojo.NotificationWebhook
	if _, err := client.Do(req, &patched); err != nil {
		t.Fatalf("PATCH status error = %v", err)
	}
	if patched.Status != defectdojo.NotificationWebhookStatusActive || patched.HeaderValue != "Bearer rotated" {
		t.Errorf("PATCH status = %+v, want the status unchanged and the header value set", patched)
	}

	p, _, err := client.NotificationWebhooks.Patch(ctx, got.ID, &defectdojo.NotificationWebhookPatch{
		URL: new("https://hooks.invalid/other"), HeaderValue: defectdojo.Null[string](),
	}, nil)
	if err != nil || p.URL != "https://hooks.invalid/other" || p.HeaderValue != "" || p.Name != got.Name {
		t.Errorf("Patch() = %+v, %v", p, err)
	}

	found := false
	for wh, err := range client.NotificationWebhooks.ListAll(ctx, &defectdojo.ListNotificationWebhooksOptions{Name: got.Name}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		found = found || wh.ID == created.ID
	}
	if !found {
		t.Error("ListAll() did not return the webhook")
	}

	preview, _, err := client.NotificationWebhooks.DeletePreview(ctx, created.ID, nil)
	if err != nil || !slices.ContainsFunc(preview.Results, func(o defectdojo.DeletedObject) bool { return o.Model == "Notification_Webhooks" }) {
		t.Errorf("DeletePreview() = %+v, %v; want the webhook", preview, err)
	}

	if _, err := client.NotificationWebhooks.Delete(ctx, created.ID, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.NotificationWebhooks.Get(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}

// A user's webhooks are deleted with the user.
func TestAccNotificationWebhooks_owner(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	u, _, err := client.Users.Create(ctx, &defectdojo.CreateUserRequest{
		User: defectdojo.User{Username: randName(t, "acc"), Email: "acc@example.com", IsActive: true}, Password: accPassword,
	}, nil)
	if err != nil {
		t.Fatalf("Users.Create() error = %v", err)
	}
	cleanupUser(t, client, u.ID)
	wh, _, err := client.NotificationWebhooks.Create(ctx, &defectdojo.NotificationWebhook{
		Name: randName(t, "acc"), URL: "https://hooks.invalid/user", Owner: u.ID,
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.NotificationWebhooks.Delete(ctx, wh.ID, nil)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	if wh.Owner != u.ID {
		t.Errorf("Create() Owner = %d, want %d", wh.Owner, u.ID)
	}
	if list, _, err := client.NotificationWebhooks.List(ctx, &defectdojo.ListNotificationWebhooksOptions{Owner: u.ID}); err != nil || list.Count != 1 {
		t.Errorf("List(owner) = %+v, %v; want the webhook", list, err)
	}

	if _, err := client.Users.Delete(ctx, u.ID, nil); err != nil {
		t.Fatalf("Users.Delete() error = %v", err)
	}
	if _, _, err := client.NotificationWebhooks.Get(ctx, wh.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get() after deleting the owner error = %v, want ErrNotFound", err)
	}
}

package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

// DefectDojo keeps one announcement, shared by the whole stack, so a single
// test covers its lifecycle and leaves none behind.
func TestAccAnnouncements(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	if a, _, err := client.Announcements.Current(ctx, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Fatalf("Current() = %+v, %v; want no announcement before the test", a, err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Announcements.Delete(ctx, 1, nil)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})

	// The style is left out, for DefectDojo's default.
	created, resp, err := client.Announcements.Create(ctx, &defectdojo.Announcement{Message: "Maintenance on <b>Saturday</b>."}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	want := defectdojo.Announcement{ID: 1, Message: "Maintenance on <b>Saturday</b>.", Style: defectdojo.AnnouncementStyleInfo}
	if resp.StatusCode != http.StatusCreated || !reflect.DeepEqual(*created, want) {
		t.Errorf("Create() = %+v (%d), want %+v", created, resp.StatusCode, want)
	}

	// A second announcement is a 400.
	_, _, err = client.Announcements.Create(ctx, &defectdojo.Announcement{Message: "second"}, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || errResp.Message != "No more than one Announcement is allowed" {
		t.Errorf("Create(second) error = %v, want the 400 for a second announcement", err)
	}

	current, _, err := client.Announcements.Current(ctx, nil)
	if err != nil || !reflect.DeepEqual(*current, want) {
		t.Errorf("Current() = %+v, %v; want %+v", current, err, want)
	}
	got, _, err := client.Announcements.Get(ctx, 1, nil)
	if err != nil || !reflect.DeepEqual(*got, want) {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, want)
	}

	updated, _, err := client.Announcements.Update(ctx, 1, &defectdojo.Announcement{Message: "Updated", Style: defectdojo.AnnouncementStyleDanger, Dismissable: true}, nil)
	if err != nil || !reflect.DeepEqual(*updated, defectdojo.Announcement{ID: 1, Message: "Updated", Style: "danger", Dismissable: true}) {
		t.Errorf("Update() = %+v, %v", updated, err)
	}
	// An empty style keeps the current one.
	updated, _, err = client.Announcements.Update(ctx, 1, &defectdojo.Announcement{Message: "Updated again"}, nil)
	if err != nil || updated.Style != "danger" || updated.Dismissable {
		t.Errorf("Update(no style) = %+v, %v; want the style kept", updated, err)
	}
	patched, _, err := client.Announcements.Patch(ctx, 1, &defectdojo.AnnouncementPatch{Style: new(defectdojo.AnnouncementStyle("success"))}, nil)
	if err != nil || patched.Style != "success" || patched.Message != "Updated again" {
		t.Errorf("Patch() = %+v, %v; want the new style and the message kept", patched, err)
	}
	for name, patch := range map[string]*defectdojo.AnnouncementPatch{
		"empty message": {Message: new("")},
		"unknown style": {Style: new(defectdojo.AnnouncementStyle("Info"))},
	} {
		if _, _, err := client.Announcements.Patch(ctx, 1, patch, nil); !errors.Is(err, defectdojo.ErrBadRequest) {
			t.Errorf("Patch(%s) error = %v, want ErrBadRequest", name, err)
		}
	}

	preview, _, err := client.Announcements.DeletePreview(ctx, 1, nil)
	if err != nil || preview.Count < 1 || preview.Results[0].Model != "Announcement" {
		t.Errorf("DeletePreview() = %+v, %v; want the announcement first", preview, err)
	}
	if _, err := client.Announcements.Delete(ctx, 1, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Announcements.Current(ctx, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Current(deleted) error = %v, want ErrNotFound", err)
	}

	// The next announcement gets the ID 1 again.
	again, _, err := client.Announcements.Create(ctx, &defectdojo.Announcement{Message: "Again", Style: "warning"}, nil)
	if err != nil || again.ID != 1 {
		t.Errorf("Create(again) = %+v, %v; want ID 1", again, err)
	}
	if _, err := client.Announcements.Delete(ctx, 1, nil); err != nil {
		t.Fatalf("Delete(again) error = %v", err)
	}
	if _, _, err := client.Announcements.Get(ctx, 1, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}

package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/ottramst/go-defectdojo"
)

func TestAccNoteTypes(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	name := randName(t, "acc")

	// Created inactive: an active mandatory note type would keep the UI from
	// closing findings on the shared stack.
	created, resp, err := client.NoteTypes.Create(ctx, &defectdojo.NoteType{Name: name, Description: "acceptance test", IsSingle: true}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.NoteTypes.Delete(ctx, created.ID, nil)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	want := defectdojo.NoteType{ID: created.ID, Name: name, Description: "acceptance test", IsSingle: true}
	if resp.StatusCode != http.StatusCreated || created.ID == 0 || !reflect.DeepEqual(*created, want) {
		t.Errorf("Create() = %+v (%d), want %+v", created, resp.StatusCode, want)
	}

	// Names are unique: a duplicate is a 400 with a field error.
	_, _, err = client.NoteTypes.Create(ctx, &defectdojo.NoteType{Name: name, Description: "d"}, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["name"]) == 0 {
		t.Errorf("Create(same name) error = %v, want a 400 with a name field error", err)
	}

	got, _, err := client.NoteTypes.Get(ctx, created.ID, nil)
	if err != nil || !reflect.DeepEqual(*got, want) {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, want)
	}
	byName, _, err := client.NoteTypes.GetByName(ctx, name, nil)
	if err != nil || byName.ID != created.ID {
		t.Errorf("GetByName() = %+v, %v; want ID %d", byName, err, created.ID)
	}
	list, _, err := client.NoteTypes.List(ctx, &defectdojo.ListNoteTypesOptions{Name: name, IsSingle: new(true)})
	if err != nil || list.Count != 1 {
		t.Errorf("List(name, is_single) = %+v, %v; want the note type", list, err)
	}

	got.Name = name + "-renamed"
	got.IsSingle = false
	got.IsMandatory = true
	updated, _, err := client.NoteTypes.Update(ctx, got.ID, got, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !reflect.DeepEqual(*updated, *got) {
		t.Errorf("Update() = %+v, want %+v", updated, got)
	}
	patched, _, err := client.NoteTypes.Patch(ctx, got.ID, &defectdojo.NoteTypePatch{Description: new("patched"), IsMandatory: new(false)}, nil)
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patched.Description != "patched" || patched.IsMandatory || patched.Name != got.Name || patched.IsActive {
		t.Errorf("Patch() = %+v, want the new description, not mandatory, and the other fields kept", patched)
	}

	// Notes of the type, on a test; the notes API takes an inactive type.
	if n, _, err := client.NoteTypes.CountNotes(ctx, created.ID, nil); err != nil || n != 0 {
		t.Errorf("CountNotes(unused) = %d, %v; want 0", n, err)
	}
	productType := createProductType(t, client)
	product := createProductIn(t, client, productType, false)
	day := defectdojo.Date{Year: 2026, Month: time.January, Day: 1}
	engagement, _, err := client.Engagements.Create(ctx, &defectdojo.Engagement{Product: product, TargetStart: day, TargetEnd: day}, nil)
	if err != nil {
		t.Fatalf("creating engagement: %v", err)
	}
	test := createTest(t, client, engagement.ID, nil)
	for _, entry := range []string{"first", "second"} {
		rawRequest(t, client, http.MethodPost, "/api/v2/tests/"+strconv.Itoa(test)+"/notes/",
			map[string]any{"entry": entry, "note_type": created.ID}, nil)
	}
	if n, _, err := client.NoteTypes.CountNotes(ctx, created.ID, nil); err != nil || n != 2 {
		t.Errorf("CountNotes() = %d, %v; want 2", n, err)
	}
	preview, _, err := client.NoteTypes.DeletePreview(ctx, created.ID, nil)
	if err != nil || preview.Count <= 3 || preview.Results[0].Model != "Note_Type" {
		t.Errorf("DeletePreview() = %+v, %v; want the note type, its notes and their links", preview, err)
	}

	// Deleting the note type deletes its notes.
	if _, err := client.NoteTypes.Delete(ctx, created.ID, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.NoteTypes.Get(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
	var notes struct {
		Count int `json:"count"`
	}
	rawRequest(t, client, http.MethodGet, "/api/v2/tests/"+strconv.Itoa(test)+"/notes/", nil, &notes)
	if notes.Count != 0 {
		t.Errorf("notes of the test after the delete = %d, want 0", notes.Count)
	}
	if _, _, err := client.NoteTypes.CountNotes(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("CountNotes(deleted) error = %v, want ErrNotFound", err)
	}
}

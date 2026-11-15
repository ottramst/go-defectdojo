package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ottramst/go-defectdojo"
)

func TestAccDevelopmentEnvironments(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	name := randName(t, "acc")

	created, resp, err := client.DevelopmentEnvironments.Create(ctx, &defectdojo.DevelopmentEnvironment{Name: name}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	deleteEnv := func(id int) {
		cleanup(t, func(ctx context.Context) error {
			_, err := client.DevelopmentEnvironments.Delete(ctx, id, nil)
			if errors.Is(err, defectdojo.ErrNotFound) {
				return nil
			}
			return err
		})
	}
	deleteEnv(created.ID)
	if resp.StatusCode != http.StatusCreated || created.ID == 0 || created.Name != name {
		t.Errorf("Create() = %+v (%d), want %q", created, resp.StatusCode, name)
	}

	got, _, err := client.DevelopmentEnvironments.Get(ctx, created.ID, nil)
	if err != nil || *got != *created {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, created)
	}
	byName, _, err := client.DevelopmentEnvironments.GetByName(ctx, name, nil)
	if err != nil || byName.ID != created.ID {
		t.Errorf("GetByName() = %+v, %v; want ID %d", byName, err, created.ID)
	}
	// A seeded environment, read only.
	if prod, _, err := client.DevelopmentEnvironments.GetByName(ctx, "Production", nil); err != nil || prod.ID == 0 {
		t.Errorf("GetByName(Production) = %+v, %v; want the seeded environment", prod, err)
	}

	// Names aren't unique: a second environment with the name is accepted,
	// and GetByName then fails.
	twin, _, err := client.DevelopmentEnvironments.Create(ctx, &defectdojo.DevelopmentEnvironment{Name: name}, nil)
	if err != nil {
		t.Fatalf("Create(same name) error = %v", err)
	}
	deleteEnv(twin.ID)
	if _, _, err := client.DevelopmentEnvironments.GetByName(ctx, name, nil); err == nil || errors.Is(err, defectdojo.ErrNotFound) ||
		!strings.Contains(err.Error(), strconv.Itoa(twin.ID)) {
		t.Errorf("GetByName(duplicate) error = %v, want an error naming both IDs", err)
	}
	if _, err := client.DevelopmentEnvironments.Delete(ctx, twin.ID, nil); err != nil {
		t.Fatalf("Delete(twin) error = %v", err)
	}

	updated, _, err := client.DevelopmentEnvironments.Update(ctx, created.ID, &defectdojo.DevelopmentEnvironment{Name: name + "-renamed"}, nil)
	if err != nil || updated.Name != name+"-renamed" {
		t.Errorf("Update() = %+v, %v; want the new name", updated, err)
	}
	patched, _, err := client.DevelopmentEnvironments.Patch(ctx, created.ID, &defectdojo.DevelopmentEnvironmentPatch{Name: new(name)}, nil)
	if err != nil || patched.Name != name {
		t.Errorf("Patch() = %+v, %v; want the original name", patched, err)
	}

	// An environment that a test uses can't be deleted.
	product := createProductIn(t, client, createProductType(t, client), false)
	day := defectdojo.Date{Year: 2026, Month: time.January, Day: 1}
	engagement, _, err := client.Engagements.Create(ctx, &defectdojo.Engagement{Product: product, TargetStart: day, TargetEnd: day}, nil)
	if err != nil {
		t.Fatalf("creating engagement: %v", err)
	}
	test := createTest(t, client, engagement.ID, map[string]any{"environment": created.ID})
	if _, err := client.DevelopmentEnvironments.Delete(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrConflict) {
		t.Errorf("Delete(in use) error = %v, want ErrConflict", err)
	}
	preview, _, err := client.DevelopmentEnvironments.DeletePreview(ctx, created.ID, nil)
	if err != nil || preview.Count < 1 || preview.Results[0].Model != "Development_Environment" {
		t.Errorf("DeletePreview() = %+v, %v; want the environment first", preview, err)
	}
	rawRequest(t, client, http.MethodDelete, "/api/v2/tests/"+strconv.Itoa(test)+"/", nil, nil)

	if _, err := client.DevelopmentEnvironments.Delete(ctx, created.ID, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.DevelopmentEnvironments.Get(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}

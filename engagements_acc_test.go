package defectdojo_test

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/ottramst/go-defectdojo"
)

func TestAccEngagements(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	ptID := createProductType(t, client)
	productID := createProductIn(t, client, ptID, false)
	otherID := createProductIn(t, client, ptID, false)
	name := randName(t, "acc")
	start := defectdojo.Date{Year: 2026, Month: time.January, Day: 5}
	end := defectdojo.Date{Year: 2026, Month: time.December, Day: 31}

	created, resp, err := client.Engagements.Create(ctx, &defectdojo.Engagement{
		Product:        productID,
		Name:           "  " + name + "  ",
		Description:    "acceptance test\n",
		TargetStart:    start,
		TargetEnd:      end,
		EngagementType: defectdojo.EngagementTypeCICD,
		BuildID:        "42",
		CommitHash:     "abc123",
		BranchTag:      "main",
		Tracker:        "https://tracker.example/T-1",
		ThreatModel:    true,
		Lead:           1,
		Tags:           []string{"ci"},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Engagements.Delete(ctx, created.ID, nil)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	// Text is trimmed; an empty status uses DefectDojo's default; the
	// booleans that default to true are false unless they are sent.
	if resp.StatusCode != http.StatusCreated || created.Name != name || created.Description != "acceptance test" ||
		created.TargetStart != start || created.TargetEnd != end || !created.FirstContacted.IsZero() ||
		created.Status != defectdojo.EngagementStatusNotStarted || created.EngagementType != defectdojo.EngagementTypeCICD ||
		!created.Active || !created.ThreatModel || created.APITest || created.Lead != 1 || created.Version != "" ||
		!slices.Equal(created.Tags, []string{"ci"}) || created.Created.IsZero() {
		t.Errorf("Create() = %+v (%d), want the new engagement", created, resp.StatusCode)
	}

	// An engagement without a name is named after its target start.
	unnamed, _, err := client.Engagements.Create(ctx, &defectdojo.Engagement{Product: productID, TargetStart: start, TargetEnd: end}, nil)
	if err != nil {
		t.Fatalf("Create(no name) error = %v", err)
	}
	if unnamed.Name != "2026-01-05" || unnamed.EngagementType != defectdojo.EngagementTypeInteractive {
		t.Errorf("Create(no name) = %+v, want the name 2026-01-05 and an interactive engagement", unnamed)
	}
	if _, err := client.Engagements.Delete(ctx, unnamed.ID, nil); err != nil {
		t.Errorf("Delete(unnamed) error = %v", err)
	}

	got, _, err := client.Engagements.Get(ctx, created.ID, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != name || got.BuildID != "42" || got.Tracker != "https://tracker.example/T-1" {
		t.Errorf("Get() = %+v, want %+v", got, created)
	}

	// The name filter is exact.
	list, _, err := client.Engagements.List(ctx, &defectdojo.ListEngagementsOptions{Product: productID, Name: name})
	if err != nil || list.Count != 1 || list.Results[0].ID != created.ID {
		t.Errorf("List(product, name) = %+v, %v; want the engagement", list, err)
	}
	if list, _, err := client.Engagements.List(ctx, &defectdojo.ListEngagementsOptions{Product: productID, Name: name[:len(name)-1]}); err != nil || list.Count != 0 {
		t.Errorf("List(name prefix) = %+v, %v; want none", list, err)
	}

	// Read, modify, write: empty nullable fields are cleared, and an empty
	// status is kept.
	got.Description = ""
	got.BuildID = ""
	got.Lead = 0
	got.Status = ""
	got.Version = "1.2.3"
	updated, _, err := client.Engagements.Update(ctx, got.ID, got, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Description != "" || updated.BuildID != "" || updated.Lead != 0 || updated.Version != "1.2.3" ||
		updated.Status != defectdojo.EngagementStatusNotStarted || updated.CommitHash != "abc123" {
		t.Errorf("Update() = %+v, want cleared fields and the status kept", updated)
	}

	// DefectDojo checks that the target start is not after the target end
	// only on create.
	late := defectdojo.Date{Year: 2027, Month: time.June, Day: 1}
	patched, _, err := client.Engagements.Patch(ctx, got.ID, &defectdojo.EngagementPatch{
		TargetStart:    &late,
		FirstContacted: defectdojo.Value(start),
		Status:         new(defectdojo.EngagementStatusCompleted),
		Description:    defectdojo.Value("patched"),
	}, nil)
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	// Setting the status doesn't close the engagement.
	if patched.TargetStart != late || patched.FirstContacted != start || patched.Description != "patched" ||
		patched.Status != defectdojo.EngagementStatusCompleted || !patched.Active {
		t.Errorf("Patch() = %+v, want the late start, a completed but active engagement", patched)
	}
	if _, _, err := client.Engagements.Create(ctx, &defectdojo.Engagement{Product: productID, TargetStart: late, TargetEnd: end}, nil); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Create(start after end) error = %v, want ErrBadRequest", err)
	}

	if _, err := client.Engagements.Close(ctx, got.ID, nil); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if e, _, err := client.Engagements.Get(ctx, got.ID, nil); err != nil || e.Active || e.Status != defectdojo.EngagementStatusCompleted {
		t.Errorf("Get(closed) = %+v, %v; want an inactive, completed engagement", e, err)
	}
	if _, err := client.Engagements.Reopen(ctx, got.ID, nil); err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	if e, _, err := client.Engagements.Get(ctx, got.ID, nil); err != nil || !e.Active || e.Status != defectdojo.EngagementStatusInProgress {
		t.Errorf("Get(reopened) = %+v, %v; want an active engagement in progress", e, err)
	}

	// Moving the engagement to another product.
	moved, _, err := client.Engagements.Patch(ctx, got.ID, &defectdojo.EngagementPatch{Product: &otherID}, nil)
	if err != nil || moved.Product != otherID {
		t.Fatalf("Patch(product) = %+v, %v; want product %d", moved, err, otherID)
	}

	found := false
	for e, err := range client.Engagements.ListAll(ctx, &defectdojo.ListEngagementsOptions{ListOptions: defectdojo.ListOptions{Limit: 1}, Product: otherID}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		if e.Product != otherID {
			t.Errorf("ListAll(product %d) returned engagement %d of product %d", otherID, e.ID, e.Product)
		}
		found = found || e.ID == created.ID
	}
	if !found {
		t.Error("ListAll() did not return the engagement")
	}

	// A test makes the engagement's delete cascade.
	if n, _, err := client.Engagements.CountTests(ctx, created.ID, nil); err != nil || n != 0 {
		t.Errorf("CountTests() = %d, %v; want 0", n, err)
	}
	createTest(t, client, created.ID, nil)
	if n, _, err := client.Engagements.CountTests(ctx, created.ID, nil); err != nil || n != 1 {
		t.Errorf("CountTests() = %d, %v; want 1", n, err)
	}
	preview, _, err := client.Engagements.DeletePreview(ctx, created.ID, nil)
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if !slices.ContainsFunc(preview.Results, func(o *defectdojo.DeletedObject) bool { return o.Model == "Test" }) {
		t.Errorf("DeletePreview() = %+v, want the test", preview.Results)
	}

	if _, err := client.Engagements.Delete(ctx, created.ID, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Engagements.Get(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
	// The tests filter rejects engagements that don't exist.
	if _, _, err := client.Engagements.CountTests(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("CountTests(deleted) error = %v, want ErrNotFound", err)
	}
}

// Engagements of a product that passes its tags on get the product's tags,
// and keep them when a write removes them.
func TestAccEngagements_tagInheritance(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	productID := createProductIn(t, client, createProductType(t, client), true)
	start := defectdojo.Date{Year: 2026, Month: time.January, Day: 1}

	e, _, err := client.Engagements.Create(ctx, &defectdojo.Engagement{
		Product: productID, TargetStart: start, TargetEnd: start, Tags: []string{"own"},
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	got, _, err := client.Engagements.Get(ctx, e.ID, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if want := []string{"inherited", "own"}; !slices.Equal(sorted(got.Tags), want) {
		t.Errorf("Tags = %v, want %v", got.Tags, want)
	}
	// Without tags, an engagement inherits the product's tags; with an empty
	// list, it doesn't.
	for _, tt := range []struct {
		tags, want []string
	}{
		{tags: nil, want: []string{"inherited"}},
		{tags: []string{}, want: []string{}},
	} {
		e := &defectdojo.Engagement{Product: productID, TargetStart: start, TargetEnd: start, Tags: tt.tags}
		created, _, err := client.Engagements.Create(ctx, e, nil)
		if err != nil {
			t.Fatalf("Create(tags %#v) error = %v", tt.tags, err)
		}
		if got, _, err := client.Engagements.Get(ctx, created.ID, nil); err != nil || !slices.Equal(got.Tags, tt.want) {
			t.Errorf("Create(tags %#v): Get() tags = %v, %v; want %v", tt.tags, got.Tags, err, tt.want)
		}
	}

	// A write without the inherited tags gets them back.
	if _, _, err := client.Engagements.Patch(ctx, e.ID, &defectdojo.EngagementPatch{Tags: &[]string{"other"}}, nil); err != nil {
		t.Fatalf("Patch(other tags) error = %v", err)
	}
	if got, _, err := client.Engagements.Get(ctx, e.ID, nil); err != nil || !slices.Equal(sorted(got.Tags), []string{"inherited", "other"}) {
		t.Errorf("Get() tags = %v, %v; want [inherited other]", got.Tags, err)
	}
	// But an empty list removes them too.
	if _, _, err := client.Engagements.Patch(ctx, e.ID, &defectdojo.EngagementPatch{Tags: &[]string{}}, nil); err != nil {
		t.Fatalf("Patch(no tags) error = %v", err)
	}
	if got, _, err := client.Engagements.Get(ctx, e.ID, nil); err != nil || len(got.Tags) != 0 {
		t.Errorf("Get() tags = %v, %v; want none", got.Tags, err)
	}
}

// createProductIn creates a product in the product type; it is deleted with
// the product type. With inherit, the product has the tag "inherited" and
// passes its tags on.
func createProductIn(t *testing.T, client *defectdojo.Client, productTypeID int, inherit bool) int {
	t.Helper()
	p := &defectdojo.Product{Name: randName(t, "acc"), Description: "acceptance test", ProdType: productTypeID}
	if inherit {
		p.EnableProductTagInheritance = true
		p.Tags = []string{"inherited"}
	}
	created, _, err := client.Products.Create(t.Context(), p, nil)
	if err != nil {
		t.Fatalf("creating product: %v", err)
	}
	return created.ID
}

// createTest creates a test in the engagement through the raw API; the
// client has no tests service. extra adds fields, such as "environment". It
// returns the test's ID; the test is deleted with the engagement.
func createTest(t *testing.T, client *defectdojo.Client, engagementID int, extra map[string]any) int {
	t.Helper()
	body := map[string]any{
		"engagement": engagementID, "test_type": 1, "title": "acceptance test",
		"target_start": "2026-01-01T00:00:00Z", "target_end": "2026-01-02T00:00:00Z",
	}
	maps.Copy(body, extra)
	var created struct {
		ID int `json:"id"`
	}
	rawRequest(t, client, http.MethodPost, "/api/v2/tests/", body, &created)
	return created.ID
}

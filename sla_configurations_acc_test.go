package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestAccSLAConfigurations(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	name := randName(t, "acc")

	created, resp, err := client.SLAConfigurations.Create(ctx, &defectdojo.SLAConfiguration{
		Name:            name,
		Description:     "acceptance test",
		Critical:        3,
		EnforceCritical: true,
		High:            14,
		EnforceHigh:     true,
		Medium:          60,
		Low:             90,
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.SLAConfigurations.Delete(ctx, created.ID, nil)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	want := defectdojo.SLAConfiguration{
		ID: created.ID, Name: name, Description: "acceptance test",
		Critical: 3, EnforceCritical: true, High: 14, EnforceHigh: true, Medium: 60, Low: 90,
	}
	if resp.StatusCode != http.StatusCreated || created.ID == 0 || *created != want {
		t.Errorf("Create() = %+v (%d), want %+v", created, resp.StatusCode, want)
	}

	// Names are unique: a duplicate is a 400 with a field error.
	_, _, err = client.SLAConfigurations.Create(ctx, &defectdojo.SLAConfiguration{Name: name, Critical: 1, High: 1, Medium: 1, Low: 1}, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["name"]) == 0 {
		t.Errorf("Create(same name) error = %v, want a 400 with a name field error", err)
	}

	got, _, err := client.SLAConfigurations.Get(ctx, created.ID, nil)
	if err != nil || *got != want {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, want)
	}

	byName, _, err := client.SLAConfigurations.GetByName(ctx, name, nil)
	if err != nil || byName.ID != created.ID {
		t.Errorf("GetByName() = %+v, %v; want ID %d", byName, err, created.ID)
	}
	if _, _, err := client.SLAConfigurations.GetByName(ctx, name[:len(name)-1], nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(prefix) error = %v, want ErrNotFound", err)
	}

	// Read, modify, write the fields that the SLA lock doesn't cover.
	got.Name = name + "-renamed"
	got.Description = ""
	got.RestartSLAOnReactivation = true
	updated, _, err := client.SLAConfigurations.Update(ctx, got.ID, got, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != got.Name || updated.Description != "" || !updated.RestartSLAOnReactivation || updated.Critical != 3 {
		t.Errorf("Update() = %+v, want the renamed SLA configuration without a description", updated)
	}

	// A change of the days locks them until Celery recalculated the SLA
	// dates; the test stack has no Celery worker, so the lock stays.
	patched, _, err := client.SLAConfigurations.Patch(ctx, got.ID, &defectdojo.SLAConfigurationPatch{
		Critical:    new(2),
		Description: defectdojo.Value("patched"),
	}, nil)
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patched.Critical != 2 || patched.Description != "patched" || patched.Name != got.Name {
		t.Errorf("Patch() = %+v, want critical 2 and the new description", patched)
	}
	_, _, err = client.SLAConfigurations.Patch(ctx, got.ID, &defectdojo.SLAConfigurationPatch{Critical: new(1)}, nil)
	if !defectdojo.IsSLARecalculating(err) {
		t.Errorf("Patch(days again) error = %v, want IsSLARecalculating", err)
	}
	// The current days and the fields the lock doesn't cover can still change.
	if _, _, err := client.SLAConfigurations.Patch(ctx, got.ID, &defectdojo.SLAConfigurationPatch{
		Critical: new(2), RestartSLAOnReactivation: new(false),
	}, nil); err != nil {
		t.Errorf("Patch(unlocked fields) error = %v", err)
	}

	found := false
	for sla, err := range client.SLAConfigurations.ListAll(ctx, &defectdojo.ListSLAConfigurationsOptions{ListOptions: defectdojo.ListOptions{Limit: 1}}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		found = found || sla.ID == created.ID
	}
	if !found {
		t.Error("ListAll() did not return the SLA configuration")
	}

	preview, _, err := client.SLAConfigurations.DeletePreview(ctx, created.ID, nil)
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if !slices.ContainsFunc(preview.Results, func(o defectdojo.DeletedObject) bool { return o.Model == "SLA_Configuration" }) {
		t.Errorf("DeletePreview() = %+v, want the SLA configuration", preview.Results)
	}

	// The lock doesn't keep the configuration from being deleted.
	if _, err := client.SLAConfigurations.Delete(ctx, created.ID, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.SLAConfigurations.Get(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestAccSLAConfigurations_deleteRefused(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	// "Default" can't be deleted.
	_, err := client.SLAConfigurations.Delete(ctx, 1, nil)
	if !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Delete(Default) error = %v, want ErrBadRequest", err)
	}
	if _, _, err := client.SLAConfigurations.Get(ctx, 1, nil); err != nil {
		t.Errorf("Get(Default) error = %v, want it still there", err)
	}

	// A configuration that a product uses can't be deleted. Creating the
	// product with it doesn't lock the configuration.
	slaID := createSLAConfiguration(t, client)
	product, _, err := client.Products.Create(ctx, &defectdojo.Product{
		Name: randName(t, "acc"), Description: "acceptance test", ProdType: createProductType(t, client), SLAConfiguration: slaID,
	}, nil)
	if err != nil {
		t.Fatalf("creating product: %v", err)
	}
	if _, err := client.SLAConfigurations.Delete(ctx, slaID, nil); !errors.Is(err, defectdojo.ErrConflict) {
		t.Errorf("Delete(in use) error = %v, want ErrConflict", err)
	}
	if _, _, err := client.SLAConfigurations.Patch(ctx, slaID, &defectdojo.SLAConfigurationPatch{High: new(10)}, nil); err != nil {
		t.Errorf("Patch(days of a configuration a new product uses) error = %v, want no lock", err)
	}
	// That change locked the product's SLA configuration too.
	_, _, err = client.Products.Patch(ctx, product.ID, &defectdojo.ProductPatch{SLAConfiguration: new(createSLAConfiguration(t, client))}, nil)
	if !defectdojo.IsSLARecalculating(err) {
		t.Errorf("Products.Patch(SLA configuration) error = %v, want IsSLARecalculating", err)
	}
}

// createSLAConfiguration creates an SLA configuration with DefectDojo's
// default days. It is deleted when the test ends, after the products
// registered later, which refer to it.
func createSLAConfiguration(t *testing.T, client *defectdojo.Client) int {
	t.Helper()
	sla, _, err := client.SLAConfigurations.Create(t.Context(), &defectdojo.SLAConfiguration{
		Name: randName(t, "acc"), Critical: 7, High: 30, Medium: 90, Low: 120,
		EnforceCritical: true, EnforceHigh: true, EnforceMedium: true, EnforceLow: true,
	}, nil)
	if err != nil {
		t.Fatalf("creating SLA configuration: %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.SLAConfigurations.Delete(ctx, sla.ID, nil)
		return err
	})
	return sla.ID
}

package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestAccProductTypes(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	name := randName(t, "acc")

	created, resp, err := client.ProductTypes.Create(ctx, &defectdojo.ProductType{
		Name:            name,
		Description:     "acceptance test",
		CriticalProduct: true,
		AuthorizedUsers: []int{1},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.ProductTypes.Delete(ctx, created.ID)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	if resp.StatusCode != http.StatusCreated || created.ID == 0 || created.Name != name || !created.CriticalProduct ||
		created.Created.IsZero() || !slices.Equal(created.AuthorizedUsers, []int{1}) {
		t.Errorf("Create() = %+v (%d), want the new product type", created, resp.StatusCode)
	}

	// Names are unique: a duplicate is a 400 with a field error, not a 409.
	_, _, err = client.ProductTypes.Create(ctx, &defectdojo.ProductType{Name: name})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["name"]) == 0 {
		t.Errorf("Create(same name) error = %v, want a 400 with a name field error", err)
	}

	got, _, err := client.ProductTypes.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != name || got.Description != "acceptance test" {
		t.Errorf("Get() = %+v, want %+v", got, created)
	}

	byName, _, err := client.ProductTypes.GetByName(ctx, name)
	if err != nil || byName.ID != created.ID {
		t.Errorf("GetByName() = %+v, %v; want ID %d", byName, err, created.ID)
	}
	if _, _, err := client.ProductTypes.GetByName(ctx, name[:len(name)-1]); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(prefix) error = %v, want ErrNotFound", err)
	}

	// Read, modify, write: the other fields are kept.
	got.Name = name + "-renamed"
	got.AuthorizedUsers = nil
	updated, _, err := client.ProductTypes.Update(ctx, got.ID, got)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != got.Name || updated.Description != "acceptance test" || !updated.CriticalProduct || len(updated.AuthorizedUsers) != 0 {
		t.Errorf("Update() = %+v, want the renamed product type without authorized users", updated)
	}

	patched, _, err := client.ProductTypes.Patch(ctx, got.ID, &defectdojo.ProductTypePatch{
		Description: defectdojo.Null[string](),
		KeyProduct:  new(true),
	})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patched.Description != "" || !patched.KeyProduct || patched.Name != got.Name {
		t.Errorf("Patch() = %+v, want a cleared description and key_product set", patched)
	}

	found := false
	for pt, err := range client.ProductTypes.ListAll(ctx, &defectdojo.ListProductTypesOptions{ListOptions: defectdojo.ListOptions{Limit: 1}}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		found = found || pt.ID == created.ID
	}
	if !found {
		t.Error("ListAll() did not return the product type")
	}

	// A product makes the product type's delete cascade.
	if n, _, err := client.ProductTypes.CountProducts(ctx, created.ID); err != nil || n != 0 {
		t.Errorf("CountProducts() = %d, %v; want 0", n, err)
	}
	createProduct(t, client, created.ID)
	if n, _, err := client.ProductTypes.CountProducts(ctx, created.ID); err != nil || n != 1 {
		t.Errorf("CountProducts() = %d, %v; want 1", n, err)
	}
	preview, _, err := client.ProductTypes.DeletePreview(ctx, created.ID, nil)
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if !slices.ContainsFunc(preview.Objects, func(o *defectdojo.DeletedObject) bool { return o.Model == "Product" }) {
		t.Errorf("DeletePreview() = %+v, want the product", preview.Objects)
	}

	if _, err := client.ProductTypes.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.ProductTypes.Get(ctx, created.ID); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
	if n, _, err := client.ProductTypes.CountProducts(ctx, created.ID); err != nil || n != 0 {
		t.Errorf("CountProducts(deleted) = %d, %v; want 0 (the product was deleted too)", n, err)
	}
}

// createProduct creates a product in the product type through the raw API,
// until the client has a Products service. It is deleted with the product
// type.
func createProduct(t *testing.T, client *defectdojo.Client, productTypeID int) {
	t.Helper()
	body := map[string]any{"name": randName(t, "acc"), "description": "acceptance test", "prod_type": productTypeID}
	req, err := client.NewRequest(t.Context(), http.MethodPost, "/api/v2/products/", body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(req, nil); err != nil {
		t.Fatalf("creating product: %v", err)
	}
}

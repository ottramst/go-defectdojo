package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestAccProductAttributes(t *testing.T) {
	client := accClient(t)
	tables := []struct {
		name    string
		svc     *defectdojo.ProductAttributesService
		model   string
		seeded  string
		product func(value string) *defectdojo.ProductPatch
	}{
		{"platforms", client.ProductPlatforms, "Product_Platform", "web service",
			func(v string) *defectdojo.ProductPatch { return &defectdojo.ProductPatch{Platform: nullableValue(v)} }},
		{"lifecycles", client.ProductLifecycles, "Product_Lifecycle", "production",
			func(v string) *defectdojo.ProductPatch { return &defectdojo.ProductPatch{Lifecycle: nullableValue(v)} }},
		{"origins", client.ProductOrigins, "Product_Origin", "open source",
			func(v string) *defectdojo.ProductPatch { return &defectdojo.ProductPatch{Origin: nullableValue(v)} }},
	}
	productType := createProductType(t, client)
	product := createProductIn(t, client, productType, false)

	for _, tt := range tables {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			value := randName(t, "acc")

			created, resp, err := tt.svc.Create(ctx, &defectdojo.ProductAttribute{Value: value, Name: "Acceptance", Icon: "flask", DisplayOrder: 99}, nil)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			cleanup(t, func(ctx context.Context) error {
				_, err := tt.svc.Delete(ctx, created.ID, nil)
				if errors.Is(err, defectdojo.ErrNotFound) {
					return nil
				}
				return err
			})
			want := defectdojo.ProductAttribute{ID: created.ID, Value: value, Name: "Acceptance", Icon: "flask", DisplayOrder: 99}
			if resp.StatusCode != http.StatusCreated || !reflect.DeepEqual(*created, want) {
				t.Errorf("Create() = %+v (%d), want %+v", created, resp.StatusCode, want)
			}

			// Values are unique: a duplicate is a 400 with a field error.
			_, _, err = tt.svc.Create(ctx, &defectdojo.ProductAttribute{Value: value, Name: "Duplicate"}, nil)
			var errResp *defectdojo.ErrorResponse
			if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["value"]) == 0 {
				t.Errorf("Create(same value) error = %v, want a 400 with a value field error", err)
			}

			got, _, err := tt.svc.Get(ctx, created.ID, nil)
			if err != nil || !reflect.DeepEqual(*got, want) {
				t.Errorf("Get() = %+v, %v; want %+v", got, err, want)
			}
			byValue, _, err := tt.svc.GetByValue(ctx, value, nil)
			if err != nil || byValue.ID != created.ID {
				t.Errorf("GetByValue() = %+v, %v; want ID %d", byValue, err, created.ID)
			}
			if seeded, _, err := tt.svc.GetByValue(ctx, tt.seeded, nil); err != nil || seeded.Value != tt.seeded {
				t.Errorf("GetByValue(%q) = %+v, %v; want the seeded entry", tt.seeded, seeded, err)
			}

			// Update ignores the value, which can't change, and clears the icon.
			updated, _, err := tt.svc.Update(ctx, created.ID, &defectdojo.ProductAttribute{Value: value + "-changed", Name: "Renamed", DisplayOrder: -1}, nil)
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			if !reflect.DeepEqual(*updated, defectdojo.ProductAttribute{ID: created.ID, Value: value, Name: "Renamed", DisplayOrder: -1}) {
				t.Errorf("Update() = %+v, want the value kept, the new name and order, and no icon", updated)
			}
			patched, _, err := tt.svc.Patch(ctx, created.ID, &defectdojo.ProductAttributePatch{Icon: new("star")}, nil)
			if err != nil || patched.Icon != "star" || patched.Name != "Renamed" {
				t.Errorf("Patch() = %+v, %v; want the icon and the name kept", patched, err)
			}

			// An entry that a product uses can't be deleted.
			if _, _, err := client.Products.Patch(ctx, product, tt.product(value), nil); err != nil {
				t.Fatalf("setting the product's value: %v", err)
			}
			if _, err := tt.svc.Delete(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrConflict) {
				t.Errorf("Delete(in use) error = %v, want ErrConflict", err)
			}
			preview, _, err := tt.svc.DeletePreview(ctx, created.ID, nil)
			if err != nil || preview.Count < 1 || preview.Results[0].Model != tt.model {
				t.Errorf("DeletePreview() = %+v, %v; want the %s first", preview, err, tt.model)
			}
			if _, _, err := client.Products.Patch(ctx, product, tt.product(""), nil); err != nil {
				t.Fatalf("clearing the product's value: %v", err)
			}

			if _, err := tt.svc.Delete(ctx, created.ID, nil); err != nil {
				t.Fatalf("Delete() error = %v", err)
			}
			if _, _, err := tt.svc.Get(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
				t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
			}
		})
	}
}

// nullableValue returns v, or null when v is empty.
func nullableValue(v string) defectdojo.Nullable[string] {
	if v == "" {
		return defectdojo.Null[string]()
	}
	return defectdojo.Value(v)
}

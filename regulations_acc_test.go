package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestAccRegulations(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	name := randName(t, "acc")
	acronym := name[len(name)-12:]

	created, resp, err := client.Regulations.Create(ctx, &defectdojo.Regulation{
		Name:         name,
		Acronym:      acronym,
		Category:     defectdojo.RegulationCategoryPrivacy,
		Jurisdiction: "European Union",
		Description:  "acceptance test",
		Reference:    "https://example.com/regulation",
	}, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Regulations.Delete(ctx, created.ID, nil)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	want := defectdojo.Regulation{
		ID: created.ID, Name: name, Acronym: acronym, Category: defectdojo.RegulationCategoryPrivacy,
		Jurisdiction: "European Union", Description: "acceptance test", Reference: "https://example.com/regulation",
	}
	if resp.StatusCode != http.StatusCreated || created.ID == 0 || *created != want {
		t.Errorf("Create() = %+v (%d), want %+v", created, resp.StatusCode, want)
	}

	// Names and acronyms are unique: a duplicate is a 400 with a field error.
	for field, dup := range map[string]*defectdojo.Regulation{
		"name":    {Name: name, Acronym: acronym + "x", Category: "other", Jurisdiction: "J"},
		"acronym": {Name: name + "x", Acronym: acronym, Category: "other", Jurisdiction: "J"},
	} {
		_, _, err := client.Regulations.Create(ctx, dup, nil)
		var errResp *defectdojo.ErrorResponse
		if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields[field]) == 0 {
			t.Errorf("Create(same %s) error = %v, want a 400 with a %s field error", field, err, field)
		}
	}

	// The client leaves the category check to DefectDojo.
	_, _, err = client.Regulations.Create(ctx, &defectdojo.Regulation{Name: name + "y", Acronym: acronym + "y", Category: "Privacy", Jurisdiction: "J"}, nil)
	if errResp := new(defectdojo.ErrorResponse); !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["category"]) == 0 {
		t.Errorf("Create(unknown category) error = %v, want a 400 with a category field error", err)
	}

	got, _, err := client.Regulations.Get(ctx, created.ID, nil)
	if err != nil || *got != want {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, want)
	}
	byName, _, err := client.Regulations.GetByName(ctx, name, nil)
	if err != nil || byName.ID != created.ID {
		t.Errorf("GetByName() = %+v, %v; want ID %d", byName, err, created.ID)
	}
	byAcronym, _, err := client.Regulations.GetByAcronym(ctx, acronym, nil)
	if err != nil || byAcronym.ID != created.ID {
		t.Errorf("GetByAcronym() = %+v, %v; want ID %d", byAcronym, err, created.ID)
	}
	// A seeded regulation, read only.
	if pci, _, err := client.Regulations.GetByAcronym(ctx, "PCI DSS", nil); err != nil || pci.Category != defectdojo.RegulationCategoryFinance {
		t.Errorf("GetByAcronym(PCI DSS) = %+v, %v; want the seeded regulation", pci, err)
	}
	if _, _, err := client.Regulations.GetByName(ctx, name[:len(name)-1], nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(prefix) error = %v, want ErrNotFound", err)
	}

	// Update sends every field: the empty description and reference clear them.
	got.Name = name + "-renamed"
	got.Category = defectdojo.RegulationCategorySecurity
	got.Description = ""
	got.Reference = ""
	updated, _, err := client.Regulations.Update(ctx, got.ID, got, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if *updated != *got {
		t.Errorf("Update() = %+v, want %+v", updated, got)
	}

	patched, _, err := client.Regulations.Patch(ctx, got.ID, &defectdojo.RegulationPatch{
		Jurisdiction: new("Estonia"),
		Reference:    new("https://example.com/other"),
	}, nil)
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patched.Jurisdiction != "Estonia" || patched.Reference != "https://example.com/other" || patched.Name != got.Name {
		t.Errorf("Patch() = %+v, want the new jurisdiction and reference and the other fields kept", patched)
	}
	if _, _, err := client.Regulations.Patch(ctx, got.ID, &defectdojo.RegulationPatch{Reference: new("not a url")}, nil); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Patch(invalid reference) error = %v, want ErrBadRequest", err)
	}

	// A product loses the regulation silently when it is deleted.
	pt, _, err := client.ProductTypes.Create(ctx, &defectdojo.ProductType{Name: randName(t, "acc-regulation")}, nil)
	if err != nil {
		t.Fatalf("ProductTypes.Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.ProductTypes.Delete(ctx, pt.ID, nil)
		return err
	})
	product, _, err := client.Products.Create(ctx, &defectdojo.Product{
		Name: randName(t, "acc-regulation"), Description: "d", ProdType: pt.ID, Regulations: []int{created.ID, 1},
	}, nil)
	if err != nil {
		t.Fatalf("Products.Create() error = %v", err)
	}

	preview, _, err := client.Regulations.DeletePreview(ctx, created.ID, nil)
	if err != nil || preview.Count < 1 || preview.Results[0].Model != "Regulation" {
		t.Errorf("DeletePreview() = %+v, %v; want the regulation first", preview, err)
	}
	if _, err := client.Regulations.Delete(ctx, created.ID, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Regulations.Get(ctx, created.ID, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
	if p, _, err := client.Products.Get(ctx, product.ID, nil); err != nil || !slices.Equal(p.Regulations, []int{1}) {
		t.Errorf("product regulations after the delete = %v, %v; want [1]", p.Regulations, err)
	}
}

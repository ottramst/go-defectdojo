package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestAccProducts(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	ptID := createProductType(t, client)
	slaID := createSLAConfiguration(t, client)
	name := randName(t, "acc")
	tag := randName(t, "Acc") // a new tag, so its spelling is kept

	created, resp, err := client.Products.Create(ctx, &defectdojo.Product{
		Name:                "  " + name + "  ",
		Description:         "acceptance test\n",
		ProdType:            ptID,
		BusinessCriticality: defectdojo.BusinessCriticalityHigh,
		Platform:            "web",
		ProductManager:      1,
		AuthorizedUsers:     []int{1},
		Regulations:         []int{1},
		Tags:                []string{tag, "a,b"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error {
		_, err := client.Products.Delete(ctx, created.ID)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
	// Text is trimmed; a new tag keeps its case, and commas split tags; the
	// booleans that default to true are false unless they are sent; a zero
	// SLA configuration uses DefectDojo's default.
	if resp.StatusCode != http.StatusCreated || created.Name != name || created.Description != "acceptance test" ||
		created.Platform != "web" || created.Lifecycle != "" || created.ProductManager != 1 || created.TeamManager != 0 ||
		created.EnableFullRiskAcceptance || created.SLAConfiguration != 1 || created.UserRecords != nil || created.Created.IsZero() {
		t.Errorf("Create() = %+v (%d), want the new product", created, resp.StatusCode)
	}
	if got := sorted(created.Tags); !slices.Equal(got, []string{tag, "a", "b"}) {
		t.Errorf("Tags = %v, want [%s a b]", got, tag)
	}
	// A tag takes the spelling of an existing product tag that matches it
	// ignoring case.
	other, _, err := client.Products.Create(ctx, &defectdojo.Product{
		Name: randName(t, "acc"), Description: "d", ProdType: ptID, Tags: []string{strings.ToLower(tag)},
	})
	if err != nil {
		t.Fatalf("Create(lower-case tag) error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error { _, err := client.Products.Delete(ctx, other.ID); return err })
	if !slices.Equal(other.Tags, []string{tag}) {
		t.Errorf("Tags = %v, want [%s] (the existing spelling)", other.Tags, tag)
	}

	// Names are unique: a duplicate is a 400 with a field error. Names that
	// differ only in case are different products.
	_, _, err = client.Products.Create(ctx, &defectdojo.Product{Name: name, Description: "d", ProdType: ptID})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["name"]) == 0 {
		t.Errorf("Create(same name) error = %v, want a 400 with a name field error", err)
	}
	upper, _, err := client.Products.Create(ctx, &defectdojo.Product{Name: strings.ToUpper(name), Description: "d", ProdType: ptID})
	if err != nil {
		t.Fatalf("Create(upper-case name) error = %v", err)
	}
	cleanup(t, func(ctx context.Context) error { _, err := client.Products.Delete(ctx, upper.ID); return err })

	for n, id := range map[string]int{name: created.ID, strings.ToUpper(name): upper.ID} {
		if p, _, err := client.Products.GetByName(ctx, n); err != nil || p.ID != id {
			t.Errorf("GetByName(%q) = %+v, %v; want ID %d", n, p, err, id)
		}
	}
	if _, _, err := client.Products.GetByName(ctx, name[:len(name)-1]); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(prefix) error = %v, want ErrNotFound", err)
	}

	// Fields that Update leaves out are kept.
	patched, _, err := client.Products.Patch(ctx, created.ID, &defectdojo.ProductPatch{
		UserRecords:              defectdojo.Value(0),
		Revenue:                  defectdojo.Value("1000.50"),
		Platform:                 defectdojo.Null[string](),
		EnableFullRiskAcceptance: new(true),
	})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patched.UserRecords == nil || *patched.UserRecords != 0 || patched.Revenue == nil || *patched.Revenue != "1000.50" ||
		patched.Platform != "" || !patched.EnableFullRiskAcceptance || patched.BusinessCriticality != defectdojo.BusinessCriticalityHigh {
		t.Errorf("Patch() = %+v, want user records, revenue and full risk acceptance set and no platform", patched)
	}

	// Read, modify, write: empty nullable fields and lists are cleared, and
	// the fields Update doesn't send are kept.
	got, _, err := client.Products.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	got.BusinessCriticality = ""
	got.ProductManager = 0
	got.Lifecycle = "production"
	got.Tags = nil
	got.Regulations = nil
	got.SLAConfiguration = 0
	updated, _, err := client.Products.Update(ctx, got.ID, got)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.BusinessCriticality != "" || updated.ProductManager != 0 || updated.Lifecycle != "production" ||
		len(updated.Tags) != 0 || len(updated.Regulations) != 0 || !slices.Equal(updated.AuthorizedUsers, []int{1}) ||
		updated.SLAConfiguration != 1 || updated.UserRecords == nil || updated.Revenue == nil || !updated.EnableFullRiskAcceptance {
		t.Errorf("Update() = %+v, want cleared fields and the others kept", updated)
	}

	// Changing the SLA configuration starts a recalculation in the
	// background; until it finishes, another change is a 400. The test stack
	// has no Celery worker, so it never finishes there.
	if p, _, err := client.Products.Patch(ctx, created.ID, &defectdojo.ProductPatch{SLAConfiguration: &slaID}); err != nil || p.SLAConfiguration != slaID {
		t.Fatalf("Patch(SLA configuration) = %+v, %v; want SLA configuration %d", p, err, slaID)
	}
	if _, _, err := client.Products.Patch(ctx, created.ID, &defectdojo.ProductPatch{SLAConfiguration: new(1)}); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Patch(second SLA change) error = %v, want ErrBadRequest", err)
	}

	found := false
	for p, err := range client.Products.ListAll(ctx, &defectdojo.ListProductsOptions{ListOptions: defectdojo.ListOptions{Limit: 1}, ProdType: ptID}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		if p.ProdType != ptID {
			t.Errorf("ListAll() returned product %d of product type %d", p.ID, p.ProdType)
		}
		found = found || p.ID == created.ID
	}
	if !found {
		t.Error("ListAll() did not return the product")
	}
	// The product type filter is sent: the fixture product type has none of
	// the test's products.
	for p, err := range client.Products.ListAll(ctx, &defectdojo.ListProductsOptions{ProdType: 1}) {
		if err != nil {
			t.Fatalf("ListAll(product type 1) error = %v", err)
		}
		if p.ProdType != 1 || p.ID == created.ID || p.ID == upper.ID {
			t.Errorf("ListAll(product type 1) returned product %d of product type %d", p.ID, p.ProdType)
		}
	}

	// An engagement makes the product's delete cascade.
	if n, _, err := client.Products.CountEngagements(ctx, created.ID); err != nil || n != 0 {
		t.Errorf("CountEngagements() = %d, %v; want 0", n, err)
	}
	createEngagement(t, client, created.ID)
	if n, _, err := client.Products.CountEngagements(ctx, created.ID); err != nil || n != 1 {
		t.Errorf("CountEngagements() = %d, %v; want 1", n, err)
	}
	preview, _, err := client.Products.DeletePreview(ctx, created.ID, nil)
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if !slices.ContainsFunc(preview.Objects, func(o *defectdojo.DeletedObject) bool { return o.Model == "Engagement" }) {
		t.Errorf("DeletePreview() = %+v, want the engagement", preview.Objects)
	}

	if _, err := client.Products.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.Products.Get(ctx, created.ID); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
	// The engagements filter rejects products that don't exist.
	if _, _, err := client.Products.CountEngagements(ctx, created.ID); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("CountEngagements(deleted) error = %v, want ErrNotFound", err)
	}
}

// createProductType creates a product type for a test and deletes it, with
// everything in it, when the test ends.
func createProductType(t *testing.T, client *defectdojo.Client) int {
	t.Helper()
	pt, _, err := client.ProductTypes.Create(t.Context(), &defectdojo.ProductType{Name: randName(t, "acc")})
	if err != nil {
		t.Fatalf("creating product type: %v", err)
	}
	cleanup(t, func(ctx context.Context) error { _, err := client.ProductTypes.Delete(ctx, pt.ID); return err })
	return pt.ID
}

// createSLAConfiguration creates an SLA configuration through the raw API,
// until the client has an SLA configurations service. It is deleted when the
// test ends, after the products registered later, which refer to it.
func createSLAConfiguration(t *testing.T, client *defectdojo.Client) int {
	t.Helper()
	body := map[string]any{"name": randName(t, "acc"), "critical": 7, "high": 30, "medium": 90, "low": 120}
	var sla struct {
		ID int `json:"id"`
	}
	rawRequest(t, client, http.MethodPost, "/api/v2/sla_configurations/", body, &sla)
	cleanup(t, func(ctx context.Context) error {
		req, err := client.NewRequest(ctx, http.MethodDelete, "/api/v2/sla_configurations/"+strconv.Itoa(sla.ID)+"/", nil)
		if err != nil {
			return err
		}
		_, err = client.Do(req, nil)
		return err
	})
	return sla.ID
}

// createEngagement creates an engagement in the product through the raw
// API, until the client has an Engagements service. It is deleted with the
// product.
func createEngagement(t *testing.T, client *defectdojo.Client, productID int) {
	t.Helper()
	body := map[string]any{"name": randName(t, "acc"), "product": productID, "target_start": "2026-01-01", "target_end": "2026-12-31"}
	rawRequest(t, client, http.MethodPost, "/api/v2/engagements/", body, nil)
}

func rawRequest(t *testing.T, client *defectdojo.Client, method, path string, body, v any) {
	t.Helper()
	req, err := client.NewRequest(t.Context(), method, path, body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(req, v); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
}

func sorted(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)
	return s
}

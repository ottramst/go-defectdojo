package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/ottramst/go-defectdojo"
)

func TestProductsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/products/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=2&name=Store&name_exact=Storefront&offset=4&prod_type=2")
		writeJSON(t, w, http.StatusOK, fixture(t, "products/list.json"))
	})

	list, _, err := client.Products.List(context.Background(), &defectdojo.ListProductsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 2, Offset: 4},
		Name:        "Store",
		NameExact:   "Storefront",
		ProdType:    2,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 2 || len(list.Products) != 2 {
		t.Fatalf("List() = %d of %d, want 2 of 2", len(list.Products), list.Count)
	}

	// A product created with only the required fields: null fields decode as
	// zero values.
	inv := list.Products[0]
	if inv.Name != "Inventory" || inv.Platform != "" || inv.BusinessCriticality != "" || inv.ProductManager != 0 ||
		inv.UserRecords != nil || inv.Revenue != nil || inv.SLAConfiguration != 1 || !inv.EnableFullRiskAcceptance {
		t.Errorf("first product = %+v, want Inventory with DefectDojo's defaults", inv)
	}
	if list.Products[1].Name != "Storefront" {
		t.Errorf("second product = %q, want Storefront", list.Products[1].Name)
	}
}

func TestProductsService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/products/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=1&prod_type=2")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":"http://x/api/v2/products/?limit=1&offset=1","results":[{"id":1,"name":"a"}]}`))
		case "1":
			testQuery(t, r, "limit=1&offset=1&prod_type=2")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":null,"results":[{"id":2,"name":"b"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})

	var names []string
	for p, err := range client.Products.ListAll(context.Background(), &defectdojo.ListProductsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1},
		ProdType:    2,
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, p.Name)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestProductsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/products/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "1" {
			writeJSON(t, w, http.StatusNotFound, []byte(`{"detail":"No Product matches the given query."}`))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "products/get.json"))
	})

	p, _, err := client.Products.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := &defectdojo.Product{
		ID:                       1,
		Name:                     "Storefront",
		Description:              "The online shop",
		ProdType:                 2,
		BusinessCriticality:      defectdojo.BusinessCriticalityHigh,
		Platform:                 "web",
		Lifecycle:                "production",
		Origin:                   "internal",
		UserRecords:              new(250000),
		Revenue:                  new("1500000.00"),
		ExternalAudience:         true,
		InternetAccessible:       true,
		EnableFullRiskAcceptance: true,
		ProductManager:           2,
		TechnicalContact:         2,
		SLAConfiguration:         1,
		AuthorizedUsers:          []int{2},
		Regulations:              []int{1},
		Tags:                     []string{"pci", "shop"},
		FindingsList:             []int{},
		ProductMeta:              []defectdojo.ProductMeta{{Name: "team", Value: "payments"}},
		Created:                  p.Created,
	}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("Get() = %+v, want %+v", p, want)
	}
	if p.Created.Year() != 2026 {
		t.Errorf("Created = %v", p.Created)
	}

	if _, _, err := client.Products.Get(context.Background(), 5); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestProductsService_GetByName(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/products/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=100&name_exact="+r.URL.Query().Get("name_exact"))
		switch name := r.URL.Query().Get("name_exact"); name {
		case "storefront", "Storefront", "STOREFRONT":
			// name_exact ignores case: names that differ only in case match.
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":1,"name":"Storefront"},{"id":3,"name":"storefront"}]}`))
		default:
			writeJSON(t, w, http.StatusOK, []byte(`{"count":0,"results":[]}`))
		}
	})

	for name, id := range map[string]int{"Storefront": 1, "storefront": 3} {
		p, _, err := client.Products.GetByName(context.Background(), name)
		if err != nil || p.ID != id {
			t.Errorf("GetByName(%q) = %v, %v; want ID %d", name, p, err, id)
		}
	}
	for _, name := range []string{"STOREFRONT", "missing"} {
		if _, _, err := client.Products.GetByName(context.Background(), name); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("GetByName(%q) error = %v, want ErrNotFound", name, err)
		}
	}
	if _, _, err := client.Products.GetByName(context.Background(), ""); err == nil {
		t.Error("GetByName(\"\") error = nil, want error")
	}
}

func TestProductsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/products/", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "Content-Type", "application/json")
		// Read-only fields and prod_numeric_grade, user_records and revenue are
		// not sent; empty nullable fields are null, a zero SLA configuration is
		// left out, and nil lists are [].
		testJSONBody(t, r, `{
			"name":"Storefront","description":"The online shop","prod_type":2,
			"business_criticality":null,"platform":null,"lifecycle":null,"origin":null,
			"external_audience":false,"internet_accessible":true,
			"enable_product_tag_inheritance":false,"enable_simple_risk_acceptance":false,
			"enable_full_risk_acceptance":false,"disable_sla_breach_notifications":false,
			"product_manager":null,"technical_contact":null,"team_manager":null,
			"authorized_users":[],"regulations":[],"tags":[]
		}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "products/get.json"))
	})

	in := &defectdojo.Product{
		ID: 99, Name: "Storefront", Description: "The online shop", ProdType: 2, InternetAccessible: true,
		UserRecords: new(5), FindingsCount: 3, Created: time.Now(),
	}
	p, resp, err := client.Products.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || p.ID != 1 {
		t.Errorf("Create() = %d %+v", resp.StatusCode, p)
	}
	if in.ID != 99 || in.Tags != nil || in.AuthorizedUsers != nil {
		t.Errorf("Create() changed the caller's product: %+v", in)
	}
}

func TestProductsService_Create_duplicate(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/products/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "products/duplicate.json"))
	})
	_, _, err := client.Products.Create(context.Background(), &defectdojo.Product{Name: "Storefront", Description: "d", ProdType: 2})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["name"]) != 1 {
		t.Errorf("Create(duplicate) error = %v, want a 400 with a name field error", err)
	}
}

func TestProductsService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/products/1/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{
			"name":"Storefront","description":"d","prod_type":2,
			"business_criticality":"high","platform":"web","lifecycle":"production","origin":"internal",
			"external_audience":true,"internet_accessible":false,
			"enable_product_tag_inheritance":true,"enable_simple_risk_acceptance":true,
			"enable_full_risk_acceptance":true,"disable_sla_breach_notifications":true,
			"product_manager":2,"technical_contact":3,"team_manager":4,"sla_configuration":5,
			"authorized_users":[2],"regulations":[1,2],"tags":["pci"]
		}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "products/get.json"))
	})

	_, _, err := client.Products.Update(context.Background(), 1, &defectdojo.Product{
		Name: "Storefront", Description: "d", ProdType: 2,
		BusinessCriticality: defectdojo.BusinessCriticalityHigh, Platform: "web", Lifecycle: "production", Origin: "internal",
		ExternalAudience: true, EnableProductTagInheritance: true, EnableSimpleRiskAcceptance: true,
		EnableFullRiskAcceptance: true, DisableSLABreachNotifications: true,
		ProductManager: 2, TechnicalContact: 3, TeamManager: 4, SLAConfiguration: 5,
		AuthorizedUsers: []int{2}, Regulations: []int{1, 2}, Tags: []string{"pci"},
		ProdNumericGrade: new(80), Revenue: new("1.00"),
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestProductsService_Patch(t *testing.T) {
	tests := []struct {
		name  string
		patch *defectdojo.ProductPatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.ProductPatch{}, want: `{}`},
		{
			name: "clear nullable fields",
			patch: &defectdojo.ProductPatch{
				BusinessCriticality: defectdojo.Null[string](), Platform: defectdojo.Null[string](),
				ProdNumericGrade: defectdojo.Null[int](), Revenue: defectdojo.Null[string](), TeamManager: defectdojo.Null[int](),
			},
			want: `{"business_criticality":null,"platform":null,"prod_numeric_grade":null,"revenue":null,"team_manager":null}`,
		},
		{
			name: "set fields",
			patch: &defectdojo.ProductPatch{
				Name: new("Shop"), ProdType: new(3), Lifecycle: defectdojo.Value("retirement"), UserRecords: defectdojo.Value(0),
				EnableFullRiskAcceptance: new(false), SLAConfiguration: new(2), Tags: new([]string{"a"}), Regulations: new([]int{}),
			},
			want: `{"name":"Shop","prod_type":3,"lifecycle":"retirement","user_records":0,"enable_full_risk_acceptance":false,"sla_configuration":2,"tags":["a"],"regulations":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/products/1/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "products/get.json"))
			})
			if _, _, err := client.Products.Patch(context.Background(), 1, tt.patch); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestProductsService_Patch_nilLists(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PATCH /api/v2/products/1/", func(w http.ResponseWriter, r *http.Request) {
		// null is a 400 in DefectDojo, so nil slices are sent as [].
		testJSONBody(t, r, `{"authorized_users":[],"regulations":[],"tags":[]}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "products/get.json"))
	})
	var users, regulations []int
	var tags []string
	patch := &defectdojo.ProductPatch{AuthorizedUsers: &users, Regulations: &regulations, Tags: &tags}
	if _, _, err := client.Products.Patch(context.Background(), 1, patch); err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patch.AuthorizedUsers != &users || patch.Regulations != &regulations || patch.Tags != &tags || users != nil || tags != nil {
		t.Error("Patch() changed the caller's patch")
	}
}

func TestProductsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/products/1/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.Products.Delete(context.Background(), 1)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("StatusCode = %d, want 204", resp.StatusCode)
	}
}

func TestProductsService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/products/1/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=50")
		writeJSON(t, w, http.StatusOK, fixture(t, "products/delete_preview.json"))
	})
	preview, _, err := client.Products.DeletePreview(context.Background(), 1, &defectdojo.ListOptions{Limit: 50})
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 7 || len(preview.Objects) != 7 {
		t.Fatalf("DeletePreview() = %d of %d, want 7 of 7", len(preview.Objects), preview.Count)
	}
	if got := *preview.Objects[6]; got.Model != "Engagement" || got.ID != 1 {
		t.Errorf("Objects[6] = %+v, want the engagement", got)
	}
}

func TestProductsService_CountEngagements(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/engagements/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=1&product=1")
		writeJSON(t, w, http.StatusOK, fixture(t, "products/count_engagements.json"))
	})
	n, _, err := client.Products.CountEngagements(context.Background(), 1)
	if err != nil {
		t.Fatalf("CountEngagements() error = %v", err)
	}
	if n != 1 {
		t.Errorf("CountEngagements() = %d, want 1", n)
	}
}

func TestProductsService_invalidArguments(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	p := &defectdojo.Product{Name: "x", Description: "d", ProdType: 1}
	calls := map[string]func() error{
		"Get zero ID":        func() error { _, _, err := client.Products.Get(ctx, 0); return err },
		"Update negative ID": func() error { _, _, err := client.Products.Update(ctx, -1, p); return err },
		"Update nil":         func() error { _, _, err := client.Products.Update(ctx, 1, nil); return err },
		"Create nil":         func() error { _, _, err := client.Products.Create(ctx, nil); return err },
		"Create empty name": func() error {
			_, _, err := client.Products.Create(ctx, &defectdojo.Product{Description: "d", ProdType: 1})
			return err
		},
		"Create empty description": func() error {
			_, _, err := client.Products.Create(ctx, &defectdojo.Product{Name: "x", ProdType: 1})
			return err
		},
		"Create zero product type": func() error {
			_, _, err := client.Products.Create(ctx, &defectdojo.Product{Name: "x", Description: "d"})
			return err
		},
		"Patch zero ID": func() error {
			_, _, err := client.Products.Patch(ctx, 0, &defectdojo.ProductPatch{})
			return err
		},
		"Patch nil":                func() error { _, _, err := client.Products.Patch(ctx, 1, nil); return err },
		"Delete zero ID":           func() error { _, err := client.Products.Delete(ctx, 0); return err },
		"DeletePreview zero ID":    func() error { _, _, err := client.Products.DeletePreview(ctx, 0, nil); return err },
		"CountEngagements zero ID": func() error { _, _, err := client.Products.CountEngagements(ctx, 0); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("error = nil, want error")
			}
		})
	}
}

func TestProductsService_CountEngagements_missingProduct(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/engagements/", func(w http.ResponseWriter, _ *http.Request) {
		// The product filter rejects IDs that don't exist.
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "products/count_engagements_missing.json"))
	})
	_, _, err := client.Products.CountEngagements(context.Background(), 99999)
	var errResp *defectdojo.ErrorResponse
	if !errors.Is(err, defectdojo.ErrNotFound) || !errors.As(err, &errResp) || errResp.StatusCode != http.StatusBadRequest {
		t.Errorf("CountEngagements(missing) error = %v, want ErrNotFound wrapping the 400", err)
	}
}

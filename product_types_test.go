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

func TestProductTypesService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_types/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "critical_product=true&limit=2&name=Web+Applications&offset=4")
		writeJSON(t, w, http.StatusOK, fixture(t, "product_types/list.json"))
	})

	list, _, err := client.ProductTypes.List(context.Background(), &defectdojo.ListProductTypesOptions{
		ListOptions:     defectdojo.ListOptions{Limit: 2, Offset: 4},
		Name:            "Web Applications",
		CriticalProduct: new(true),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 2 || len(list.Results) != 2 {
		t.Fatalf("List() = %d of %d, want 2 of 2", len(list.Results), list.Count)
	}

	// Product types created by fixtures have null timestamps and description.
	rd := list.Results[0]
	if rd.ID != 1 || rd.Name != "Research and Development" || rd.Description != "" || !rd.Created.IsZero() {
		t.Errorf("first product type = %+v, want the fixture product type with zero timestamps", rd)
	}
	web := list.Results[1]
	if web.Name != "Web Applications" || !web.CriticalProduct || web.KeyProduct || web.Created.IsZero() {
		t.Errorf("second product type = %+v", web)
	}
	if want := []int{1}; !reflect.DeepEqual(web.AuthorizedUsers, want) {
		t.Errorf("AuthorizedUsers = %v, want %v", web.AuthorizedUsers, want)
	}
}

func TestProductTypesService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_types/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "key_product=false&limit=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":"http://x/api/v2/product_types/?limit=1&offset=1","results":[{"id":1,"name":"a"}]}`))
		case "1":
			testQuery(t, r, "key_product=false&limit=1&offset=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":null,"results":[{"id":2,"name":"b"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})

	var names []string
	for pt, err := range client.ProductTypes.ListAll(context.Background(), &defectdojo.ListProductTypesOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1},
		KeyProduct:  new(false),
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, pt.Name)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestProductTypesService_ListAll_error(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_types/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, fixture(t, "errors/invalid_token.json"))
	})
	for _, err := range client.ProductTypes.ListAll(context.Background(), nil) {
		if !errors.Is(err, defectdojo.ErrForbidden) {
			t.Errorf("ListAll() error = %v, want ErrForbidden", err)
		}
	}
}

func TestProductTypesService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_types/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "4" {
			writeJSON(t, w, http.StatusNotFound, []byte(notFound))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "product_types/get.json"))
	})

	pt, _, err := client.ProductTypes.Get(context.Background(), 4, nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := &defectdojo.ProductType{
		ID:              4,
		Name:            "Web Applications",
		Description:     "Customer-facing web applications",
		CriticalProduct: true,
		AuthorizedUsers: []int{1},
		Created:         pt.Created,
		Updated:         pt.Updated,
	}
	if !reflect.DeepEqual(pt, want) {
		t.Errorf("Get() = %+v, want %+v", pt, want)
	}
	if pt.Created.Year() != 2026 || pt.Updated.Before(pt.Created) {
		t.Errorf("Created, Updated = %v, %v", pt.Created, pt.Updated)
	}

	if _, _, err := client.ProductTypes.Get(context.Background(), 5, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestProductTypesService_GetByName(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_types/", func(w http.ResponseWriter, r *http.Request) {
		switch name := r.URL.Query().Get("name"); name {
		case "Web":
			// A server whose filter matches partially must not match.
			writeJSON(t, w, http.StatusOK, []byte(`{"count":1,"results":[{"id":4,"name":"Web Applications"}]}`))
		case "Web Applications":
			writeJSON(t, w, http.StatusOK, []byte(`{"count":1,"results":[{"id":4,"name":"Web Applications"}]}`))
		default:
			writeJSON(t, w, http.StatusOK, []byte(`{"count":0,"results":[]}`))
		}
	})

	pt, _, err := client.ProductTypes.GetByName(context.Background(), "Web Applications", nil)
	if err != nil || pt.ID != 4 {
		t.Errorf("GetByName() = %v, %v; want ID 4", pt, err)
	}
	for _, name := range []string{"Web", "missing"} {
		if _, _, err := client.ProductTypes.GetByName(context.Background(), name, nil); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("GetByName(%q) error = %v, want ErrNotFound", name, err)
		}
	}
	if _, _, err := client.ProductTypes.GetByName(context.Background(), "", nil); err == nil {
		t.Error("GetByName(\"\") error = nil, want error")
	}
}

func TestProductTypesService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/product_types/", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "Content-Type", "application/json")
		// Read-only fields are not sent, and nil AuthorizedUsers is [].
		testJSONBody(t, r, `{"name":"Web Applications","description":"","critical_product":true,"key_product":false,"authorized_users":[]}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "product_types/get.json"))
	})

	in := &defectdojo.ProductType{ID: 99, Name: "Web Applications", CriticalProduct: true, Created: time.Now()}
	pt, resp, err := client.ProductTypes.Create(context.Background(), in, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || pt.ID != 4 {
		t.Errorf("Create() = %d %+v", resp.StatusCode, pt)
	}
	if in.ID != 99 || in.AuthorizedUsers != nil {
		t.Errorf("Create() changed the caller's product type: %+v", in)
	}
}

func TestProductTypesService_Create_duplicate(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/product_types/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/validation.json"))
	})
	_, _, err := client.ProductTypes.Create(context.Background(), &defectdojo.ProductType{Name: "Web Applications"}, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["name"]) != 1 {
		t.Errorf("Create(duplicate) error = %v, want a 400 with a name field error", err)
	}
}

func TestProductTypesService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/product_types/4/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"name":"Web","description":"d","critical_product":false,"key_product":true,"authorized_users":[1,2]}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "product_types/get.json"))
	})

	_, _, err := client.ProductTypes.Update(context.Background(), 4, &defectdojo.ProductType{
		Name: "Web", Description: "d", KeyProduct: true, AuthorizedUsers: []int{1, 2},
	}, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestProductTypesService_Patch(t *testing.T) {
	tests := []struct {
		name  string
		patch *defectdojo.ProductTypePatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.ProductTypePatch{}, want: `{}`},
		{name: "clear description", patch: &defectdojo.ProductTypePatch{Description: defectdojo.Null[string]()}, want: `{"description":null}`},
		{
			name: "set fields",
			patch: &defectdojo.ProductTypePatch{
				Name: new("Web"), Description: defectdojo.Value(""), KeyProduct: new(false), AuthorizedUsers: new([]int{}),
			},
			want: `{"name":"Web","description":"","key_product":false,"authorized_users":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/product_types/4/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "product_types/get.json"))
			})
			if _, _, err := client.ProductTypes.Patch(context.Background(), 4, tt.patch, nil); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestProductTypesService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/product_types/4/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.ProductTypes.Delete(context.Background(), 4, nil)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("StatusCode = %d, want 204", resp.StatusCode)
	}
}

func TestProductTypesService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_types/4/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=250")
		writeJSON(t, w, http.StatusOK, fixture(t, "product_types/delete_preview.json"))
	})
	preview, _, err := client.ProductTypes.DeletePreview(context.Background(), 4, &defectdojo.DeletePreviewProductTypeOptions{ListOptions: defectdojo.ListOptions{Limit: 250}})
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 3 || len(preview.Results) != 3 {
		t.Fatalf("DeletePreview() = %d of %d, want 3 of 3", len(preview.Results), preview.Count)
	}
	if got := preview.Results[1]; got != (defectdojo.DeletedObject{Model: "Product", ID: 2, Name: "Storefront"}) {
		t.Errorf("Objects[1] = %+v", got)
	}
}

func TestProductTypesService_CountProducts(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/products/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=1&prod_type=4")
		writeJSON(t, w, http.StatusOK, fixture(t, "product_types/count_products.json"))
	})
	n, _, err := client.ProductTypes.CountProducts(context.Background(), 4, nil)
	if err != nil {
		t.Fatalf("CountProducts() error = %v", err)
	}
	if n != 1 {
		t.Errorf("CountProducts() = %d, want 1", n)
	}
}

func TestProductTypesService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	pt := &defectdojo.ProductType{Name: "x"}
	calls := map[string]func() error{
		"Get zero ID":        func() error { _, _, err := client.ProductTypes.Get(ctx, 0, nil); return err },
		"Update negative ID": func() error { _, _, err := client.ProductTypes.Update(ctx, -1, pt, nil); return err },
		"Update nil":         func() error { _, _, err := client.ProductTypes.Update(ctx, 1, nil, nil); return err },
		"Create nil":         func() error { _, _, err := client.ProductTypes.Create(ctx, nil, nil); return err },
		"Create empty name":  func() error { _, _, err := client.ProductTypes.Create(ctx, &defectdojo.ProductType{}, nil); return err },
		"Patch zero ID": func() error {
			_, _, err := client.ProductTypes.Patch(ctx, 0, &defectdojo.ProductTypePatch{}, nil)
			return err
		},
		"Patch nil":             func() error { _, _, err := client.ProductTypes.Patch(ctx, 1, nil, nil); return err },
		"Delete zero ID":        func() error { _, err := client.ProductTypes.Delete(ctx, 0, nil); return err },
		"DeletePreview zero ID": func() error { _, _, err := client.ProductTypes.DeletePreview(ctx, 0, nil); return err },
		"CountProducts zero ID": func() error { _, _, err := client.ProductTypes.CountProducts(ctx, 0, nil); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("error = nil, want error")
			}
		})
	}
}

func TestProductTypesService_Patch_nilAuthorizedUsers(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PATCH /api/v2/product_types/4/", func(w http.ResponseWriter, r *http.Request) {
		// null is a 400 in DefectDojo, so a nil slice is sent as [].
		testJSONBody(t, r, `{"authorized_users":[]}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "product_types/get.json"))
	})
	var users []int
	patch := &defectdojo.ProductTypePatch{AuthorizedUsers: &users}
	if _, _, err := client.ProductTypes.Patch(context.Background(), 4, patch, nil); err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patch.AuthorizedUsers != &users || users != nil {
		t.Error("Patch() changed the caller's patch")
	}
}

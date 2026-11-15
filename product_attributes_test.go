package defectdojo_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

// productAttributeServices are the three tables, which share one service type.
func productAttributeServices(client *defectdojo.Client) map[string]*defectdojo.ProductAttributesService {
	return map[string]*defectdojo.ProductAttributesService{
		"product_platforms":  client.ProductPlatforms,
		"product_lifecycles": client.ProductLifecycles,
		"product_origins":    client.ProductOrigins,
	}
}

func TestProductAttributesService_List(t *testing.T) {
	mux, client := setup(t)
	for collection, svc := range productAttributeServices(client) {
		t.Run(collection, func(t *testing.T) {
			mux.HandleFunc("GET /api/v2/"+collection+"/", func(w http.ResponseWriter, r *http.Request) {
				testQuery(t, r, "limit=10")
				writeJSON(t, w, http.StatusOK, fixture(t, "product_attributes/list.json"))
			})
			list, _, err := svc.List(context.Background(), &defectdojo.ListProductAttributesOptions{ListOptions: defectdojo.ListOptions{Limit: 10}})
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if list.Count != 6 || len(list.Results) != 6 {
				t.Fatalf("List() = %d of %d, want 6 of 6", len(list.Results), list.Count)
			}
			want := defectdojo.ProductAttribute{ID: 1, Value: "web service", Name: "API", Icon: "plug", DisplayOrder: 10}
			if list.Results[0] != want {
				t.Errorf("first entry = %+v, want %+v", list.Results[0], want)
			}
		})
	}
}

func TestProductAttributesService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_origins/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":"x","results":[{"id":1,"value":"a"}]}`))
		case "1":
			testQuery(t, r, "limit=1&offset=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":null,"results":[{"id":2,"value":"b"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})
	var values []string
	for a, err := range client.ProductOrigins.ListAll(context.Background(), &defectdojo.ListProductAttributesOptions{ListOptions: defectdojo.ListOptions{Limit: 1}}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		values = append(values, a.Value)
	}
	if fmt.Sprint(values) != "[a b]" {
		t.Errorf("ListAll() = %v, want [a b]", values)
	}
}

func TestProductAttributesService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_platforms/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "8" {
			writeJSON(t, w, http.StatusNotFound, fixture(t, "errors/not_found.json"))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "product_attributes/get.json"))
	})
	got, _, err := client.ProductPlatforms.Get(context.Background(), 8, nil)
	want := defectdojo.ProductAttribute{ID: 8, Value: "kiosk", Name: "Kiosk", Icon: "tv", DisplayOrder: 60}
	if err != nil || *got != want {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, want)
	}
	if _, _, err := client.ProductPlatforms.Get(context.Background(), 9, nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestProductAttributesService_GetByValue(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_lifecycles/", func(w http.ResponseWriter, r *http.Request) {
		// There are no filters: GetByValue reads 100 entries, then the whole
		// table in one page when it is larger.
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if r.URL.Query().Has("offset") || (limit != 100 && limit != 250) {
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
		results := ""
		for i := range min(limit, 150) {
			if results != "" {
				results += ","
			}
			results += fmt.Sprintf(`{"id":%d,"value":"v-%d"}`, i+1, i)
		}
		writeJSON(t, w, http.StatusOK, []byte(`{"count":150,"results":[`+results+`]}`))
	})
	a, _, err := client.ProductLifecycles.GetByValue(context.Background(), "v-120", nil)
	if err != nil || a.ID != 121 {
		t.Errorf("GetByValue() = %v, %v; want ID 121 from the whole table", a, err)
	}
	_, _, err = client.ProductLifecycles.GetByValue(context.Background(), "V-120", nil)
	if !errors.Is(err, defectdojo.ErrNotFound) || !strings.Contains(err.Error(), "product lifecycle") {
		t.Errorf("GetByValue(other case) error = %v, want ErrNotFound naming the product lifecycle", err)
	}
}

func TestProductAttributesService_GetByValue_small(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_platforms/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=100")
		writeJSON(t, w, http.StatusOK, fixture(t, "product_attributes/list.json"))
	})
	if a, _, err := client.ProductPlatforms.GetByValue(context.Background(), "kiosk", nil); err != nil || a.ID != 8 {
		t.Errorf("GetByValue() = %v, %v; want ID 8 from one request", a, err)
	}
}

func TestProductAttributesService_GetByValue_error(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_origins/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, fixture(t, "errors/invalid_token.json"))
	})
	if _, _, err := client.ProductOrigins.GetByValue(context.Background(), "x", nil); !errors.Is(err, defectdojo.ErrForbidden) {
		t.Errorf("GetByValue() error = %v, want ErrForbidden", err)
	}
}

func TestProductAttributesService_Create(t *testing.T) {
	mux, client := setup(t)
	for collection, svc := range productAttributeServices(client) {
		t.Run(collection, func(t *testing.T) {
			mux.HandleFunc("POST /api/v2/"+collection+"/", func(w http.ResponseWriter, r *http.Request) {
				testHeader(t, r, "Content-Type", "application/json")
				// The ID is not sent; the icon and display order are.
				testJSONBody(t, r, `{"value":"kiosk","name":"Kiosk","icon":"","display_order":0}`)
				writeJSON(t, w, http.StatusCreated, fixture(t, "product_attributes/get.json"))
			})
			in := &defectdojo.ProductAttribute{ID: 99, Value: "kiosk", Name: "Kiosk"}
			a, resp, err := svc.Create(context.Background(), in, nil)
			if err != nil || resp.StatusCode != http.StatusCreated || a.ID != 8 {
				t.Errorf("Create() = %+v, %v", a, err)
			}
			if in.ID != 99 {
				t.Errorf("Create() changed the caller's entry: %+v", in)
			}
		})
	}
}

func TestProductAttributesService_Create_duplicate(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/product_platforms/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/product_attribute_duplicate.json"))
	})
	_, _, err := client.ProductPlatforms.Create(context.Background(), &defectdojo.ProductAttribute{Value: "web", Name: "Web"}, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["value"]) != 1 {
		t.Errorf("Create() error = %v, want a 400 with a value field error", err)
	}
}

func TestProductAttributesService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/product_origins/8/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"value":"kiosk","name":"N","icon":"tv","display_order":-5}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "product_attributes/get.json"))
	})
	_, _, err := client.ProductOrigins.Update(context.Background(), 8, &defectdojo.ProductAttribute{Value: "kiosk", Name: "N", Icon: "tv", DisplayOrder: -5}, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestProductAttributesService_Patch(t *testing.T) {
	tests := []struct {
		name  string
		patch *defectdojo.ProductAttributePatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.ProductAttributePatch{}, want: `{}`},
		{name: "clear icon", patch: &defectdojo.ProductAttributePatch{Icon: new("")}, want: `{"icon":""}`},
		{name: "fields", patch: &defectdojo.ProductAttributePatch{Name: new("N"), DisplayOrder: new(0)}, want: `{"name":"N","display_order":0}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/product_lifecycles/8/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "product_attributes/get.json"))
			})
			if _, _, err := client.ProductLifecycles.Patch(context.Background(), 8, tt.patch, nil); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestProductAttributesService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/product_platforms/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "1" {
			writeJSON(t, w, http.StatusConflict, fixture(t, "errors/product_attribute_in_use.json"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if resp, err := client.ProductPlatforms.Delete(context.Background(), 8, nil); err != nil || resp.StatusCode != http.StatusNoContent {
		t.Errorf("Delete() = %v, %v; want 204", resp, err)
	}
	_, err := client.ProductPlatforms.Delete(context.Background(), 1, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrConflict) || !strings.Contains(errResp.Message, "Records that refer to it: product.") {
		t.Errorf("Delete(in use) error = %v, want the 409", err)
	}
}

func TestProductAttributesService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/product_platforms/8/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=10")
		writeJSON(t, w, http.StatusOK, fixture(t, "product_attributes/delete_preview.json"))
	})
	preview, _, err := client.ProductPlatforms.DeletePreview(context.Background(), 8, &defectdojo.DeletePreviewProductAttributeOptions{ListOptions: defectdojo.ListOptions{Limit: 10}})
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 1 || preview.Results[0] != (defectdojo.DeletedObject{Model: "Product_Platform", ID: 8, Name: "Kiosk"}) {
		t.Errorf("DeletePreview() = %d %+v", preview.Count, preview.Results)
	}
}

func TestProductAttributesService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	svc := client.ProductOrigins
	calls := map[string]func() error{
		"Get zero ID":        func() error { _, _, err := svc.Get(ctx, 0, nil); return err },
		"GetByValue empty":   func() error { _, _, err := svc.GetByValue(ctx, "", nil); return err },
		"Create nil":         func() error { _, _, err := svc.Create(ctx, nil, nil); return err },
		"Create empty value": func() error { _, _, err := svc.Create(ctx, &defectdojo.ProductAttribute{Name: "N"}, nil); return err },
		"Create empty name":  func() error { _, _, err := svc.Create(ctx, &defectdojo.ProductAttribute{Value: "v"}, nil); return err },
		"Update zero ID": func() error {
			_, _, err := svc.Update(ctx, 0, &defectdojo.ProductAttribute{Value: "v", Name: "N"}, nil)
			return err
		},
		"Update empty value": func() error {
			_, _, err := svc.Update(ctx, 1, &defectdojo.ProductAttribute{Name: "N"}, nil)
			return err
		},
		"Patch zero ID":         func() error { _, _, err := svc.Patch(ctx, 0, &defectdojo.ProductAttributePatch{}, nil); return err },
		"Patch nil":             func() error { _, _, err := svc.Patch(ctx, 1, nil, nil); return err },
		"Delete zero ID":        func() error { _, err := svc.Delete(ctx, 0, nil); return err },
		"DeletePreview zero ID": func() error { _, _, err := svc.DeletePreview(ctx, 0, nil); return err },
	}
	for name, call := range calls {
		err := call()
		if err == nil {
			t.Errorf("%s: error = nil, want an argument error", name)
		} else if !strings.Contains(err.Error(), "product origin") {
			t.Errorf("%s: error = %v, want it to name the product origin", name, err)
		}
	}
}

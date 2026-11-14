package defectdojo_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestSLAConfigurationsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/sla_configurations/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=3")
		writeJSON(t, w, http.StatusOK, fixture(t, "sla_configurations/list.json"))
	})

	list, _, err := client.SLAConfigurations.List(context.Background(), &defectdojo.ListSLAConfigurationsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 3},
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 3 || len(list.SLAConfigurations) != 3 {
		t.Fatalf("List() = %d of %d, want 3 of 3", len(list.SLAConfigurations), list.Count)
	}
	if def := list.SLAConfigurations[0]; def.ID != 1 || def.Name != "Default" || def.Critical != 7 || !def.EnforceCritical {
		t.Errorf("first SLA configuration = %+v, want the Default fixture", def)
	}
	if none := list.SLAConfigurations[1]; none.Name != "No SLA Enforced" || none.EnforceHigh {
		t.Errorf("second SLA configuration = %+v, want the No SLA Enforced fixture", none)
	}
}

func TestSLAConfigurationsService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/sla_configurations/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":"x","results":[{"id":1,"name":"a"}]}`))
		case "1":
			testQuery(t, r, "limit=1&offset=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":null,"results":[{"id":2,"name":"b"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})

	var names []string
	for sla, err := range client.SLAConfigurations.ListAll(context.Background(), &defectdojo.ListSLAConfigurationsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1},
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, sla.Name)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestSLAConfigurationsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/sla_configurations/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "18" {
			writeJSON(t, w, http.StatusNotFound, []byte(notFound))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "sla_configurations/get.json"))
	})

	sla, _, err := client.SLAConfigurations.Get(context.Background(), 18)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := &defectdojo.SLAConfiguration{
		ID:                       18,
		Name:                     "Strict",
		Description:              "Internet-facing products",
		Critical:                 3,
		EnforceCritical:          true,
		High:                     14,
		EnforceHigh:              true,
		Medium:                   60,
		EnforceMedium:            true,
		Low:                      90,
		RestartSLAOnReactivation: true,
	}
	if !reflect.DeepEqual(sla, want) {
		t.Errorf("Get() = %+v, want %+v", sla, want)
	}

	if _, _, err := client.SLAConfigurations.Get(context.Background(), 5); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestSLAConfigurationsService_GetByName(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/sla_configurations/", func(w http.ResponseWriter, r *http.Request) {
		// There is no name filter: GetByName pages through all of them.
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		wantQuery := "limit=100"
		if offset > 0 {
			wantQuery += "&offset=" + strconv.Itoa(offset)
		}
		testQuery(t, r, wantQuery)
		results := ""
		for i := offset; i < min(offset+100, 150); i++ {
			if results != "" {
				results += ","
			}
			results += fmt.Sprintf(`{"id":%d,"name":"sla-%d"}`, i+1, i)
		}
		writeJSON(t, w, http.StatusOK, []byte(`{"count":150,"results":[`+results+`]}`))
	})

	sla, _, err := client.SLAConfigurations.GetByName(context.Background(), "sla-120")
	if err != nil || sla.ID != 121 {
		t.Errorf("GetByName() = %v, %v; want ID 121 from the second page", sla, err)
	}
	for _, name := range []string{"SLA-120", "sla-1200"} {
		if _, _, err := client.SLAConfigurations.GetByName(context.Background(), name); !errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("GetByName(%q) error = %v, want ErrNotFound", name, err)
		}
	}
}

func TestSLAConfigurationsService_GetByName_error(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/sla_configurations/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, fixture(t, "errors/invalid_token.json"))
	})
	if _, _, err := client.SLAConfigurations.GetByName(context.Background(), "x"); !errors.Is(err, defectdojo.ErrForbidden) {
		t.Errorf("GetByName() error = %v, want ErrForbidden", err)
	}
}

func TestSLAConfigurationsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/sla_configurations/", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "Content-Type", "application/json")
		// The ID is not sent, an empty description is null, and the Enforce
		// fields that are not set are false.
		testJSONBody(t, r, `{"name":"Strict","description":null,"critical":3,"enforce_critical":true,"high":14,"enforce_high":false,`+
			`"medium":60,"enforce_medium":false,"low":90,"enforce_low":false,"restart_sla_on_reactivation":false}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "sla_configurations/get.json"))
	})

	in := &defectdojo.SLAConfiguration{ID: 99, Name: "Strict", Critical: 3, EnforceCritical: true, High: 14, Medium: 60, Low: 90}
	sla, resp, err := client.SLAConfigurations.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || sla.ID != 18 {
		t.Errorf("Create() = %d %+v", resp.StatusCode, sla)
	}
	if in.ID != 99 {
		t.Errorf("Create() changed the caller's SLA configuration: %+v", in)
	}
}

func TestSLAConfigurationsService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/sla_configurations/18/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"name":"Strict","description":"d","critical":1,"enforce_critical":false,"high":2,"enforce_high":true,`+
			`"medium":3,"enforce_medium":false,"low":4,"enforce_low":true,"restart_sla_on_reactivation":true}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "sla_configurations/get.json"))
	})

	_, _, err := client.SLAConfigurations.Update(context.Background(), 18, &defectdojo.SLAConfiguration{
		Name: "Strict", Description: "d", Critical: 1, High: 2, EnforceHigh: true, Medium: 3, Low: 4, EnforceLow: true,
		RestartSLAOnReactivation: true,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestSLAConfigurationsService_Patch(t *testing.T) {
	tests := []struct {
		name  string
		patch *defectdojo.SLAConfigurationPatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.SLAConfigurationPatch{}, want: `{}`},
		{name: "clear description", patch: &defectdojo.SLAConfigurationPatch{Description: defectdojo.Null[string]()}, want: `{"description":null}`},
		{
			name: "set fields",
			patch: &defectdojo.SLAConfigurationPatch{
				Name: new("Strict"), Critical: new(1), EnforceLow: new(false), RestartSLAOnReactivation: new(true),
			},
			want: `{"name":"Strict","critical":1,"enforce_low":false,"restart_sla_on_reactivation":true}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/sla_configurations/18/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "sla_configurations/get.json"))
			})
			if _, _, err := client.SLAConfigurations.Patch(context.Background(), 18, tt.patch); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestSLAConfigurationsService_Patch_recalculating(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PATCH /api/v2/sla_configurations/18/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/sla_recalculating.json"))
	})
	_, _, err := client.SLAConfigurations.Patch(context.Background(), 18, &defectdojo.SLAConfigurationPatch{Critical: new(5)})
	if !errors.Is(err, defectdojo.ErrBadRequest) || !defectdojo.IsSLARecalculating(err) {
		t.Errorf("Patch() error = %v, want a 400 that IsSLARecalculating", err)
	}
}

func TestIsSLARecalculating(t *testing.T) {
	product := `{"non_field_errors":["Finding SLA expiration dates are currently being recalculated. ` +
		`The SLA configuration for this product cannot be changed until the calculation is complete."]}`
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{name: "SLA configuration", status: http.StatusBadRequest, body: string(fixture(t, "errors/sla_recalculating.json")), want: true},
		// Without the sentence-form message of 3.4.300+, the field error counts.
		{name: "product", status: http.StatusBadRequest, body: product, want: true},
		{name: "other 400", status: http.StatusBadRequest, body: string(fixture(t, "errors/validation.json"))},
		{name: "same text, other status", status: http.StatusConflict, body: product},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/products/1/", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.status, []byte(tt.body))
			})
			_, _, err := client.Products.Patch(context.Background(), 1, &defectdojo.ProductPatch{})
			if got := defectdojo.IsSLARecalculating(err); got != tt.want {
				t.Errorf("IsSLARecalculating(%v) = %v, want %v", err, got, tt.want)
			}
		})
	}
	if defectdojo.IsSLARecalculating(nil) || defectdojo.IsSLARecalculating(errors.New("currently being calculated")) {
		t.Error("IsSLARecalculating() = true for an error that is not an ErrorResponse")
	}
}

func TestSLAConfigurationsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/sla_configurations/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "1" {
			writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/sla_default_delete.json"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.SLAConfigurations.Delete(context.Background(), 18)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("StatusCode = %d, want 204", resp.StatusCode)
	}
	_, err = client.SLAConfigurations.Delete(context.Background(), 1)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || errResp.Message != "Unable to delete default SLA Configuration." {
		t.Errorf("Delete(1) error = %v, want the 400 for the Default SLA configuration", err)
	}
}

func TestSLAConfigurationsService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/sla_configurations/18/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=10")
		writeJSON(t, w, http.StatusOK, fixture(t, "sla_configurations/delete_preview.json"))
	})
	preview, _, err := client.SLAConfigurations.DeletePreview(context.Background(), 18, &defectdojo.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 1 || *preview.Objects[0] != (defectdojo.DeletedObject{Model: "SLA_Configuration", ID: 18, Name: "Strict"}) {
		t.Errorf("DeletePreview() = %d %+v", preview.Count, preview.Objects)
	}
}

func TestSLAConfigurationsService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	valid := defectdojo.SLAConfiguration{Name: "x", Critical: 1, High: 1, Medium: 1, Low: 1}
	with := func(f func(*defectdojo.SLAConfiguration)) *defectdojo.SLAConfiguration {
		sla := valid
		f(&sla)
		return &sla
	}
	calls := map[string]func() error{
		"Get zero ID":     func() error { _, _, err := client.SLAConfigurations.Get(ctx, 0); return err },
		"GetByName empty": func() error { _, _, err := client.SLAConfigurations.GetByName(ctx, ""); return err },
		"Create nil":      func() error { _, _, err := client.SLAConfigurations.Create(ctx, nil); return err },
		"Create empty name": func() error {
			_, _, err := client.SLAConfigurations.Create(ctx, with(func(s *defectdojo.SLAConfiguration) { s.Name = "" }))
			return err
		},
		"Create zero days": func() error {
			_, _, err := client.SLAConfigurations.Create(ctx, &defectdojo.SLAConfiguration{Name: "x"})
			return err
		},
		"Create zero low": func() error {
			_, _, err := client.SLAConfigurations.Create(ctx, with(func(s *defectdojo.SLAConfiguration) { s.Low = 0 }))
			return err
		},
		"Update negative ID": func() error { _, _, err := client.SLAConfigurations.Update(ctx, -1, &valid); return err },
		"Update negative high": func() error {
			_, _, err := client.SLAConfigurations.Update(ctx, 1, with(func(s *defectdojo.SLAConfiguration) { s.High = -5 }))
			return err
		},
		"Patch zero ID": func() error {
			_, _, err := client.SLAConfigurations.Patch(ctx, 0, &defectdojo.SLAConfigurationPatch{})
			return err
		},
		"Patch nil": func() error { _, _, err := client.SLAConfigurations.Patch(ctx, 1, nil); return err },
		"Patch zero medium": func() error {
			_, _, err := client.SLAConfigurations.Patch(ctx, 1, &defectdojo.SLAConfigurationPatch{Medium: new(0)})
			return err
		},
		"Delete zero ID":        func() error { _, err := client.SLAConfigurations.Delete(ctx, 0); return err },
		"DeletePreview zero ID": func() error { _, _, err := client.SLAConfigurations.DeletePreview(ctx, 0, nil); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("error = nil, want error")
			}
		})
	}
}

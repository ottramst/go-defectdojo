package defectdojo_test

import (
	"errors"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

// The test types the fixtures load, read only.
func TestAccTestTypes(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	api, _, err := client.TestTypes.GetByName(ctx, "API Test", nil)
	if err != nil || api.ID != 1 || !api.Active {
		t.Fatalf("GetByName(API Test) = %+v, %v; want the fixture with ID 1", api, err)
	}
	if _, _, err := client.TestTypes.GetByName(ctx, "api test", nil); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(other case) error = %v, want ErrNotFound", err)
	}
	got, _, err := client.TestTypes.Get(ctx, api.ID, nil)
	if err != nil || *got != *api {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, api)
	}

	list, _, err := client.TestTypes.List(ctx, &defectdojo.ListTestTypesOptions{ListOptions: defectdojo.ListOptions{Limit: 1}})
	if err != nil || list.Count < 100 {
		t.Fatalf("List() = %+v, %v; want the scanners' test types", list, err)
	}
	n := 0
	names := make(map[string]bool)
	for tt, err := range client.TestTypes.ListAll(ctx, nil) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		if names[tt.Name] {
			t.Errorf("ListAll() yielded %q twice", tt.Name)
		}
		names[tt.Name] = true
		n++
	}
	if n != list.Count {
		t.Errorf("ListAll() yielded %d test types, want %d", n, list.Count)
	}
	if !names["Trivy Scan"] || !names["ZAP Scan"] {
		t.Error("ListAll() is missing the Trivy Scan or ZAP Scan test type")
	}
}

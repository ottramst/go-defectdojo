package defectdojo

import (
	"fmt"
	"testing"
)

// recorder collects what checkContract reports.
type recorder struct{ errors []string }

func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}
func (r *recorder) Fatalf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func TestCheckContract_undocumented(t *testing.T) {
	type object struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	str := &openAPISchema{Type: "string"}
	integer := &openAPISchema{Type: "integer"}
	spec := func(url *openAPISchema) *openAPISpec {
		s := &openAPISpec{}
		props := map[string]*openAPISchema{"name": str}
		if url != nil {
			props["url"] = url
		}
		s.Components.Schemas = map[string]*openAPISchema{"Object": {Type: "object", Properties: props}}
		return s
	}
	tests := []struct {
		name     string
		spec     *openAPISpec
		contract contract
		errors   int
	}{
		{name: "documented", spec: spec(str), contract: contract{value: object{}, schema: "Object"}},
		{name: "missing", spec: spec(nil), contract: contract{value: object{}, schema: "Object"}, errors: 1},
		{name: "undocumented and missing", spec: spec(nil), contract: contract{value: object{}, schema: "Object", undocumented: map[string]string{"url": "r"}}},
		// When the schema has the property, it is checked as usual.
		{name: "undocumented and present", spec: spec(str), contract: contract{value: object{}, schema: "Object", undocumented: map[string]string{"url": "r"}}},
		{name: "undocumented and wrong", spec: spec(integer), contract: contract{value: object{}, schema: "Object", undocumented: map[string]string{"url": "r"}}, errors: 1},
		{name: "undocumented, not a field", spec: spec(str), contract: contract{value: object{}, schema: "Object", undocumented: map[string]string{"other": "r"}}, errors: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r recorder
			checkContract(&r, tt.spec, tt.contract, "3.5.103")
			if len(r.errors) != tt.errors {
				t.Errorf("checkContract() reported %q, want %d errors", r.errors, tt.errors)
			}
		})
	}
}

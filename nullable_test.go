package defectdojo_test

import (
	"encoding/json"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestNullable_MarshalJSON(t *testing.T) {
	type body struct {
		A defectdojo.Nullable[string] `json:"a,omitzero"`
		B defectdojo.Nullable[int]    `json:"b,omitzero"`
		C defectdojo.Nullable[[]int]  `json:"c,omitzero"`
	}
	tests := []struct {
		name string
		in   body
		want string
	}{
		{name: "left out", in: body{}, want: `{}`},
		{name: "null", in: body{A: defectdojo.Null[string](), B: defectdojo.Null[int]()}, want: `{"a":null,"b":null}`},
		{name: "zero values", in: body{A: defectdojo.Value(""), B: defectdojo.Value(0), C: defectdojo.Value([]int{})}, want: `{"a":"","b":0,"c":[]}`},
		{name: "values", in: body{A: defectdojo.Value("x"), B: defectdojo.Value(7)}, want: `{"a":"x","b":7}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNullable_UnmarshalJSON(t *testing.T) {
	var v struct {
		A defectdojo.Nullable[string] `json:"a"`
		B defectdojo.Nullable[int]    `json:"b"`
		C defectdojo.Nullable[int]    `json:"c"`
	}
	if err := json.Unmarshal([]byte(`{"a":"x","b":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if got, ok := v.A.Get(); !ok || got != "x" {
		t.Errorf("A.Get() = %q, %v; want x, true", got, ok)
	}
	if !v.B.IsNull() || v.B.IsZero() {
		t.Errorf("B: IsNull() = %v, IsZero() = %v; want true, false", v.B.IsNull(), v.B.IsZero())
	}
	if _, ok := v.B.Get(); ok {
		t.Error("B.Get() ok = true for null")
	}
	if !v.C.IsZero() || v.C.IsNull() {
		t.Errorf("C: IsZero() = %v, IsNull() = %v; want true, false", v.C.IsZero(), v.C.IsNull())
	}
	if err := json.Unmarshal([]byte(`{"b":"x"}`), &v); err == nil {
		t.Error("Unmarshal(wrong type) error = nil, want error")
	}
}

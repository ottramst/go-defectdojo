package defectdojo

import (
	"bytes"
	"encoding/json"
)

// Nullable is a field of a partial update that can be left out, set to null,
// or set to a value. Its zero value is left out of the request: tag such
// fields with omitzero.
//
//	patch := &defectdojo.ProductTypePatch{
//		Description: defectdojo.Null[string](), // clear the description
//	}
type Nullable[T any] struct {
	value T
	set   bool
	null  bool
}

// Value returns a Nullable that sets the field to v.
func Value[T any](v T) Nullable[T] {
	return Nullable[T]{value: v, set: true}
}

// Null returns a Nullable that sets the field to null.
func Null[T any]() Nullable[T] {
	return Nullable[T]{set: true, null: true}
}

// IsZero reports whether the field is left out. It makes omitzero work.
func (n Nullable[T]) IsZero() bool {
	return !n.set
}

// IsNull reports whether the field is set to null.
func (n Nullable[T]) IsNull() bool {
	return n.set && n.null
}

// Get returns the value and whether the field is set to a value (not left out
// and not null).
func (n Nullable[T]) Get() (T, bool) {
	return n.value, n.set && !n.null
}

// MarshalJSON encodes the value, or null.
func (n Nullable[T]) MarshalJSON() ([]byte, error) {
	if !n.set || n.null {
		return []byte("null"), nil
	}
	return json.Marshal(n.value)
}

// UnmarshalJSON decodes a value or null.
func (n *Nullable[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*n = Null[T]()
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*n = Value(v)
	return nil
}

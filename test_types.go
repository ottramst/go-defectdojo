package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// TestTypesService handles communication with the test type methods of the
// DefectDojo API. Test types are the kinds of tests, among them one per
// scanner that DefectDojo can import ("Trivy Scan", "ZAP Scan"); imports
// create more as needed.
//
// The service only reads test types. DefectDojo can also create them and
// change their flags (POST, PUT and PATCH, which COVERAGE.md lists as not
// implemented), but can't delete them.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type TestTypesService service

// TestType is a DefectDojo test type.
type TestType struct {
	ID int `json:"id"`

	// Name is unique, at most 200 characters, and can't change.
	Name string `json:"name"`

	StaticTool  bool `json:"static_tool"`
	DynamicTool bool `json:"dynamic_tool"`

	// Active offers the test type for new tests.
	Active bool `json:"active"`
}

// ListTestTypesOptions are the options of [TestTypesService.List] and
// [TestTypesService.ListAll].
type ListTestTypesOptions struct {
	ListOptions

	// Name filters by exact, case-sensitive name.
	Name string `url:"name,omitempty"`
}

// TestTypeList is a page of test types.
type TestTypeList struct {
	TestTypes []*TestType

	// Count is the number of test types that match the filters.
	Count int
}

// List returns a page of test types, ordered by ID.
//
// DefectDojo API: GET /api/v2/test_types/
func (s *TestTypesService) List(ctx context.Context, opt *ListTestTypesOptions, opts ...RequestOption) (*TestTypeList, *Response, error) {
	path, err := addQuery("/api/v2/test_types/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*TestType]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &TestTypeList{TestTypes: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the test types, starting at opt.Offset.
// It fetches opt.Limit test types per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *TestTypesService) ListAll(ctx context.Context, opt *ListTestTypesOptions, opts ...RequestOption) iter.Seq2[*TestType, error] {
	base := ListTestTypesOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*TestType, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return result.TestTypes, result.Count, nil
	})
}

// Get returns a test type by its ID.
//
// DefectDojo API: GET /api/v2/test_types/{id}/
func (s *TestTypesService) Get(ctx context.Context, id int, opts ...RequestOption) (*TestType, *Response, error) {
	if err := requirePositive("test type ID", id); err != nil {
		return nil, nil, err
	}
	tt := new(TestType)
	resp, err := s.client.do(ctx, http.MethodGet, "/api/v2/test_types/"+strconv.Itoa(id)+"/", nil, tt, opts)
	if err != nil {
		return nil, resp, err
	}
	return tt, resp, nil
}

// GetByName returns the test type with exactly the given name, such as
// "Trivy Scan". It returns an error matching [ErrNotFound] when there is none.
func (s *TestTypesService) GetByName(ctx context.Context, name string, opts ...RequestOption) (*TestType, *Response, error) {
	if name == "" {
		return nil, nil, errors.New("defectdojo: test type name must not be empty")
	}
	list, resp, err := s.List(ctx, &ListTestTypesOptions{Name: name}, opts...)
	if err != nil {
		return nil, resp, err
	}
	// The filter is exact today; check again in case a version loosens it.
	for _, tt := range list.TestTypes {
		if tt.Name == name {
			return tt, resp, nil
		}
	}
	return nil, resp, fmt.Errorf("defectdojo: test type %q: %w", name, ErrNotFound)
}

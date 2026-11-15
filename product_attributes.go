package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// ProductAttributesService handles communication with the methods of one of
// DefectDojo's three lookup tables of product attributes: the product
// platforms ([Client].ProductPlatforms), lifecycles ([Client].ProductLifecycles)
// and origins ([Client].ProductOrigins). Products refer to their entries by
// Value ([Product].Platform, Lifecycle and Origin).
//
// The three tables have the same fields and endpoints, so unlike the other
// services, one type serves them all, configured with the table's path.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type ProductAttributesService struct {
	client *Client

	// path is the collection's path, such as "/api/v2/product_platforms/".
	path string

	// kind names an entry in errors, such as "product platform".
	kind string
}

// ProductAttribute is an entry of a lookup table of product attributes. The
// same type is used for reads and writes: the read-only ID is ignored when
// writing.
type ProductAttribute struct {
	ID int `json:"id"`

	// Value is the stable machine value that products, imports and
	// automation rules use, such as "web service", at most 50 characters
	// and unique in the table. It can't change after creation: DefectDojo
	// ignores it in updates.
	Value string `json:"value"`

	// Name is the label the UI shows, at most 200 characters.
	Name string `json:"name"`

	// Icon is an optional Font Awesome icon class for the classic UI, at
	// most 100 characters; DefectDojo stores it as "" when unset.
	Icon string `json:"icon"`

	// DisplayOrder orders the entries in the UI, lower first. DefectDojo
	// defaults it to 0.
	DisplayOrder int `json:"display_order"`
}

// productAttributeRequest is the body of create and update requests: the
// writable fields of a [ProductAttribute].
type productAttributeRequest struct {
	Value        string `json:"value"`
	Name         string `json:"name"`
	Icon         string `json:"icon"`
	DisplayOrder int    `json:"display_order"`
}

// newRequest validates an entry and returns its writable fields.
func (s *ProductAttributesService) newRequest(a *ProductAttribute) (*productAttributeRequest, error) {
	if a == nil {
		return nil, fmt.Errorf("defectdojo: %s must not be nil", s.kind)
	}
	switch {
	case a.Value == "":
		return nil, fmt.Errorf("defectdojo: %s Value must not be empty", s.kind)
	case a.Name == "":
		return nil, fmt.Errorf("defectdojo: %s Name must not be empty", s.kind)
	}
	return &productAttributeRequest{Value: a.Value, Name: a.Name, Icon: a.Icon, DisplayOrder: a.DisplayOrder}, nil
}

// ProductAttributePatch is the body of [ProductAttributesService.Patch]. Only
// the fields that are set are changed; an empty Icon clears it. The value
// can't change, so it has no field here.
type ProductAttributePatch struct {
	Name         *string `json:"name,omitempty"`
	Icon         *string `json:"icon,omitempty"`
	DisplayOrder *int    `json:"display_order,omitempty"`
}

// ProductAttributeList is a page of entries.
type ProductAttributeList struct {
	ProductAttributes []*ProductAttribute

	// Count is the number of entries in the table.
	Count int
}

// List returns a page of the table's entries, ordered by DisplayOrder, then
// Name. DefectDojo has no filters for these tables.
//
// DefectDojo API: GET /api/v2/product_platforms/
// DefectDojo API: GET /api/v2/product_lifecycles/
// DefectDojo API: GET /api/v2/product_origins/
func (s *ProductAttributesService) List(ctx context.Context, opt *ListOptions, opts ...RequestOption) (*ProductAttributeList, *Response, error) {
	path, err := addQuery(s.path, opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*ProductAttribute]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ProductAttributeList{ProductAttributes: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the table's entries, starting at
// opt.Offset. It fetches opt.Limit entries per request (100 if zero).
//
// The order (DisplayOrder, then Name) isn't unique, so entries that tie can
// move between pages, and the iterator can skip or repeat them; ask for
// pages larger than the table.
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *ProductAttributesService) ListAll(ctx context.Context, opt *ListOptions, opts ...RequestOption) iter.Seq2[*ProductAttribute, error] {
	base := ListOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*ProductAttribute, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return result.ProductAttributes, result.Count, nil
	})
}

// Get returns an entry by its ID.
//
// DefectDojo API: GET /api/v2/product_platforms/{id}/
// DefectDojo API: GET /api/v2/product_lifecycles/{id}/
// DefectDojo API: GET /api/v2/product_origins/{id}/
func (s *ProductAttributesService) Get(ctx context.Context, id int, opts ...RequestOption) (*ProductAttribute, *Response, error) {
	if err := requirePositive(s.kind+" ID", id); err != nil {
		return nil, nil, err
	}
	a := new(ProductAttribute)
	resp, err := s.client.do(ctx, http.MethodGet, s.entryPath(id), nil, a, opts)
	if err != nil {
		return nil, resp, err
	}
	return a, resp, nil
}

// GetByValue returns the entry with exactly the given value, compared
// case-sensitively. It returns an error matching [ErrNotFound] when there is
// none.
//
// DefectDojo can't filter the tables, so GetByValue reads them in pages of
// 100, which hold the tables DefectDojo comes with; the returned response is
// the last page's.
func (s *ProductAttributesService) GetByValue(ctx context.Context, value string, opts ...RequestOption) (*ProductAttribute, *Response, error) {
	if value == "" {
		return nil, nil, fmt.Errorf("defectdojo: %s value must not be empty", s.kind)
	}
	opt := &ListOptions{Limit: 100}
	for {
		list, resp, err := s.List(ctx, opt, opts...)
		if err != nil {
			return nil, resp, err
		}
		for _, a := range list.ProductAttributes {
			if a.Value == value {
				return a, resp, nil
			}
		}
		opt.Offset += len(list.ProductAttributes)
		if len(list.ProductAttributes) == 0 || opt.Offset >= list.Count {
			return nil, resp, fmt.Errorf("defectdojo: %s %q: %w", s.kind, value, ErrNotFound)
		}
	}
}

// Create creates an entry. Its read-only ID is ignored. A value that is taken
// is a 400 ([ErrBadRequest]) with a "value" field error.
//
// DefectDojo API: POST /api/v2/product_platforms/
// DefectDojo API: POST /api/v2/product_lifecycles/
// DefectDojo API: POST /api/v2/product_origins/
func (s *ProductAttributesService) Create(ctx context.Context, a *ProductAttribute, opts ...RequestOption) (*ProductAttribute, *Response, error) {
	body, err := s.newRequest(a)
	if err != nil {
		return nil, nil, err
	}
	created := new(ProductAttribute)
	resp, err := s.client.do(ctx, http.MethodPost, s.path, body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// Update replaces the writable fields of the entry with the given ID: an
// empty Icon clears it. Value must be set, as DefectDojo requires it, but
// DefectDojo ignores it: set it to the current value.
//
// DefectDojo API: PUT /api/v2/product_platforms/{id}/
// DefectDojo API: PUT /api/v2/product_lifecycles/{id}/
// DefectDojo API: PUT /api/v2/product_origins/{id}/
func (s *ProductAttributesService) Update(ctx context.Context, id int, a *ProductAttribute, opts ...RequestOption) (*ProductAttribute, *Response, error) {
	if err := requirePositive(s.kind+" ID", id); err != nil {
		return nil, nil, err
	}
	body, err := s.newRequest(a)
	if err != nil {
		return nil, nil, err
	}
	updated := new(ProductAttribute)
	resp, err := s.client.do(ctx, http.MethodPut, s.entryPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Patch changes the fields of the entry that are set in patch.
//
// DefectDojo API: PATCH /api/v2/product_platforms/{id}/
// DefectDojo API: PATCH /api/v2/product_lifecycles/{id}/
// DefectDojo API: PATCH /api/v2/product_origins/{id}/
func (s *ProductAttributesService) Patch(ctx context.Context, id int, patch *ProductAttributePatch, opts ...RequestOption) (*ProductAttribute, *Response, error) {
	if err := requirePositive(s.kind+" ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: " + s.kind + " patch must not be nil")
	}
	updated := new(ProductAttribute)
	resp, err := s.client.do(ctx, http.MethodPatch, s.entryPath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Delete deletes an entry by its ID. Deleting an entry that products use is
// a 409 ([ErrConflict]).
//
// DefectDojo API: DELETE /api/v2/product_platforms/{id}/
// DefectDojo API: DELETE /api/v2/product_lifecycles/{id}/
// DefectDojo API: DELETE /api/v2/product_origins/{id}/
func (s *ProductAttributesService) Delete(ctx context.Context, id int, opts ...RequestOption) (*Response, error) {
	if err := requirePositive(s.kind+" ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, s.entryPath(id), nil, nil, opts)
}

// DeletePreview returns a page of the objects that deleting the entry would
// remove, the entry itself included.
//
// DefectDojo API: GET /api/v2/product_platforms/{id}/delete_preview/
// DefectDojo API: GET /api/v2/product_lifecycles/{id}/delete_preview/
// DefectDojo API: GET /api/v2/product_origins/{id}/delete_preview/
func (s *ProductAttributesService) DeletePreview(ctx context.Context, id int, opt *ListOptions, opts ...RequestOption) (*DeletePreviewList, *Response, error) {
	if err := requirePositive(s.kind+" ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(s.entryPath(id)+"delete_preview/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*DeletedObject]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &DeletePreviewList{Objects: page.Results, Count: page.Count}, resp, nil
}

func (s *ProductAttributesService) entryPath(id int) string {
	return s.path + strconv.Itoa(id) + "/"
}

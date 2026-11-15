package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
	"time"
)

// ProductTypesService handles communication with the product type methods of
// the DefectDojo API. Product types group products; the DefectDojo 3.x UI
// calls them "organizations".
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type ProductTypesService service

// ProductType is a DefectDojo product type. The same type is used for reads
// and writes: the read-only fields ID, Created, Updated, ArchivedAt and
// ArchiveBatch are ignored when writing.
type ProductType struct {
	_ [0]func()

	ID int `json:"id"`

	// Name is unique across all product types, compared case-sensitively.
	Name string `json:"name"`

	// Description is at most 4000 characters. DefectDojo stores an unset
	// description as null, which decodes as "".
	Description string `json:"description"`

	CriticalProduct bool `json:"critical_product"`
	KeyProduct      bool `json:"key_product"`

	// AuthorizedUsers are the IDs of the users who may access the product
	// type and its products, in addition to superusers and staff. Writing it
	// needs the permission to manage the product type's members.
	AuthorizedUsers []int `json:"authorized_users"`

	// Created and Updated are null for product types created by fixtures,
	// such as "Research and Development", and are then zero.
	Created time.Time `json:"created,omitzero"`
	Updated time.Time `json:"updated,omitzero"`

	// ArchivedAt and ArchiveBatch are set by the archive feature of
	// DefectDojo Pro (3.5+); they are always zero on other servers.
	ArchivedAt   time.Time `json:"archived_at,omitzero"`
	ArchiveBatch string    `json:"archive_batch,omitempty"`
}

// productTypeRequest is the body of product type create and update requests:
// the writable fields of a [ProductType].
type productTypeRequest struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	CriticalProduct bool   `json:"critical_product"`
	KeyProduct      bool   `json:"key_product"`
	AuthorizedUsers []int  `json:"authorized_users"`
}

// newProductTypeRequest validates a product type and returns its writable
// fields.
func newProductTypeRequest(pt *ProductType) (*productTypeRequest, error) {
	if pt == nil {
		return nil, errors.New("defectdojo: product type must not be nil")
	}
	if pt.Name == "" {
		return nil, errors.New("defectdojo: product type Name must not be empty")
	}
	users := pt.AuthorizedUsers
	if users == nil {
		// null is a 400; an empty list removes all authorized users.
		users = []int{}
	}
	return &productTypeRequest{
		Name:            pt.Name,
		Description:     pt.Description,
		CriticalProduct: pt.CriticalProduct,
		KeyProduct:      pt.KeyProduct,
		AuthorizedUsers: users,
	}, nil
}

// ProductTypePatch is the body of [ProductTypesService.Patch]. Only the fields
// that are set are changed.
type ProductTypePatch struct {
	_ [0]func()

	Name            *string          `json:"name,omitempty"`
	Description     Nullable[string] `json:"description,omitzero"`
	CriticalProduct *bool            `json:"critical_product,omitempty"`
	KeyProduct      *bool            `json:"key_product,omitempty"`
	// AuthorizedUsers replaces the authorized users; a pointer to an empty
	// or nil slice removes them all.
	AuthorizedUsers *[]int `json:"authorized_users,omitempty"`
}

// ListProductTypesOptions are the options of [ProductTypesService.List] and
// [ProductTypesService.ListAll].
type ListProductTypesOptions struct {
	_ [0]func()

	ListOptions

	// Name filters by exact, case-sensitive name.
	Name string `url:"name,omitempty"`

	CriticalProduct *bool `url:"critical_product,omitempty"`
	KeyProduct      *bool `url:"key_product,omitempty"`
}

// ListProductTypesResult is a page of product types.
type ListProductTypesResult struct {
	_ [0]func()

	Results []ProductType

	// Count is the number of product types that match the filters.
	Count int
}

// List returns a page of product types.
//
// DefectDojo API: GET /api/v2/product_types/
func (s *ProductTypesService) List(ctx context.Context, opt *ListProductTypesOptions, opts ...RequestOption) (*ListProductTypesResult, *Response, error) {
	path, err := addQuery("/api/v2/product_types/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[ProductType]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListProductTypesResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the product types, starting at
// opt.Offset. It fetches opt.Limit product types per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *ProductTypesService) ListAll(ctx context.Context, opt *ListProductTypesOptions, opts ...RequestOption) iter.Seq2[*ProductType, error] {
	base := ListProductTypesOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*ProductType, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Results), result.Count, nil
	})
}

// GetProductTypeOptions are the options of [ProductTypesService.Get].
type GetProductTypeOptions struct {
	_ [0]func()
}

// Get returns a product type by its ID.
//
// DefectDojo API: GET /api/v2/product_types/{id}/
func (s *ProductTypesService) Get(ctx context.Context, id int, opt *GetProductTypeOptions, opts ...RequestOption) (*ProductType, *Response, error) {
	if err := requirePositive("product type ID", id); err != nil {
		return nil, nil, err
	}
	pt := new(ProductType)
	resp, err := s.client.do(ctx, http.MethodGet, productTypePath(id), nil, pt, opts)
	if err != nil {
		return nil, resp, err
	}
	return pt, resp, nil
}

// GetProductTypeByNameOptions are the options of [ProductTypesService.GetByName].
type GetProductTypeByNameOptions struct {
	_ [0]func()
}

// GetByName returns the product type with exactly the given name. It returns
// an error matching [ErrNotFound] when there is none.
func (s *ProductTypesService) GetByName(ctx context.Context, name string, opt *GetProductTypeByNameOptions, opts ...RequestOption) (*ProductType, *Response, error) {
	if name == "" {
		return nil, nil, errors.New("defectdojo: product type name must not be empty")
	}
	list, resp, err := s.List(ctx, &ListProductTypesOptions{Name: name}, opts...)
	if err != nil {
		return nil, resp, err
	}
	// DefectDojo strips whitespace from the filter, and may loosen it: check
	// the name again.
	for i := range list.Results {
		pt := &list.Results[i]
		if pt.Name == name {
			return pt, resp, nil
		}
	}
	return nil, resp, fmt.Errorf("defectdojo: product type %q: %w", name, ErrNotFound)
}

// CreateProductTypeOptions are the options of [ProductTypesService.Create].
type CreateProductTypeOptions struct {
	_ [0]func()
}

// Create creates a product type. Its read-only fields are ignored. A name
// that is taken is a 400 ([ErrBadRequest]) with a "name" field error.
//
// DefectDojo API: POST /api/v2/product_types/
func (s *ProductTypesService) Create(ctx context.Context, pt *ProductType, opt *CreateProductTypeOptions, opts ...RequestOption) (*ProductType, *Response, error) {
	body, err := newProductTypeRequest(pt)
	if err != nil {
		return nil, nil, err
	}
	created := new(ProductType)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/product_types/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateProductTypeOptions are the options of [ProductTypesService.Update].
type UpdateProductTypeOptions struct {
	_ [0]func()
}

// Update replaces the writable fields of the product type with the given ID.
// Every writable field is sent: an empty Description clears it, and a nil
// AuthorizedUsers removes all authorized users. Update a product type
// returned by [ProductTypesService.Get] to keep its other fields, or use
// [ProductTypesService.Patch].
//
// DefectDojo API: PUT /api/v2/product_types/{id}/
func (s *ProductTypesService) Update(ctx context.Context, id int, pt *ProductType, opt *UpdateProductTypeOptions, opts ...RequestOption) (*ProductType, *Response, error) {
	if err := requirePositive("product type ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newProductTypeRequest(pt)
	if err != nil {
		return nil, nil, err
	}
	updated := new(ProductType)
	resp, err := s.client.do(ctx, http.MethodPut, productTypePath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// PatchProductTypeOptions are the options of [ProductTypesService.Patch].
type PatchProductTypeOptions struct {
	_ [0]func()
}

// Patch changes the fields of the product type that are set in patch.
//
// DefectDojo API: PATCH /api/v2/product_types/{id}/
func (s *ProductTypesService) Patch(ctx context.Context, id int, patch *ProductTypePatch, opt *PatchProductTypeOptions, opts ...RequestOption) (*ProductType, *Response, error) {
	if err := requirePositive("product type ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: product type patch must not be nil")
	}
	if patch.AuthorizedUsers != nil && *patch.AuthorizedUsers == nil {
		// null is a 400; send [] from a copy, leaving the caller's patch as is.
		p := *patch
		p.AuthorizedUsers = &[]int{}
		patch = &p
	}
	updated := new(ProductType)
	resp, err := s.client.do(ctx, http.MethodPatch, productTypePath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteProductTypeOptions are the options of [ProductTypesService.Delete].
type DeleteProductTypeOptions struct {
	_ [0]func()
}

// Delete deletes a product type by its ID.
//
// Deleting a product type also deletes its products, with their engagements,
// tests and findings. Use [ProductTypesService.DeletePreview] or
// [ProductTypesService.CountProducts] first to see what would be deleted.
//
// When the server runs with asynchronous deletes (DD_ASYNC_OBJECT_DELETE),
// DefectDojo answers before the product type is gone: poll
// [ProductTypesService.Get] until it returns [ErrNotFound].
//
// DefectDojo API: DELETE /api/v2/product_types/{id}/
func (s *ProductTypesService) Delete(ctx context.Context, id int, opt *DeleteProductTypeOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("product type ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, productTypePath(id), nil, nil, opts)
}

// DeletedObject is an object that a delete would remove, as listed by a
// delete preview.
type DeletedObject struct {
	_ [0]func()

	// Model is the DefectDojo model name, such as "Product_Type", "Product"
	// or "Finding".
	Model string `json:"model"`

	// ID is the object ID; it is zero for objects without one.
	ID int `json:"id"`

	Name string `json:"name"`
}

// DeletePreviewResult is a page of the objects a delete would remove.
type DeletePreviewResult struct {
	_ [0]func()

	Results []DeletedObject

	// Count is the number of objects the delete would remove, including the
	// object itself.
	Count int
}

// DeletePreviewProductTypeOptions are the options of [ProductTypesService.DeletePreview].
type DeletePreviewProductTypeOptions struct {
	_ [0]func()

	ListOptions
}

// DeletePreview returns a page of the objects that deleting the product type
// would remove, the product type itself included.
//
// The preview loads the whole tree of related objects on DefectDojo 3.4, which
// is slow for product types with many findings.
//
// DefectDojo API: GET /api/v2/product_types/{id}/delete_preview/
func (s *ProductTypesService) DeletePreview(ctx context.Context, id int, opt *DeletePreviewProductTypeOptions, opts ...RequestOption) (*DeletePreviewResult, *Response, error) {
	if err := requirePositive("product type ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(productTypePath(id)+"delete_preview/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[DeletedObject]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &DeletePreviewResult{Results: page.Results, Count: page.Count}, resp, nil
}

// CountProductTypeProductsOptions are the options of [ProductTypesService.CountProducts].
type CountProductTypeProductsOptions struct {
	_ [0]func()
}

// CountProducts returns the number of products of the product type, which a
// delete of the product type would also delete. It reads the count of the
// products list (GET /api/v2/products/?prod_type={id}), which is cheap on
// every supported version.
func (s *ProductTypesService) CountProducts(ctx context.Context, id int, opt *CountProductTypeProductsOptions, opts ...RequestOption) (int, *Response, error) {
	if err := requirePositive("product type ID", id); err != nil {
		return 0, nil, err
	}
	var page listPage[struct{}]
	path := "/api/v2/products/?limit=1&prod_type=" + strconv.Itoa(id)
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return 0, resp, err
	}
	return page.Count, resp, nil
}

func productTypePath(id int) string {
	return "/api/v2/product_types/" + strconv.Itoa(id) + "/"
}

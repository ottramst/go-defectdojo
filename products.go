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

// ProductsService handles communication with the product methods of the
// DefectDojo API. The DefectDojo 3.x UI calls products "assets".
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type ProductsService service

// Business criticality values of [Product.BusinessCriticality].
const (
	BusinessCriticalityVeryHigh = "very high"
	BusinessCriticalityHigh     = "high"
	BusinessCriticalityMedium   = "medium"
	BusinessCriticalityLow      = "low"
	BusinessCriticalityVeryLow  = "very low"
	BusinessCriticalityNone     = "none"
)

// Product is a DefectDojo product. The same type is used for reads and
// writes: the read-only fields ID, FindingsCount, FindingsList, ProductMeta,
// Created, ArchivedAt and ArchiveBatch are ignored when writing, and so are
// ProdNumericGrade, UserRecords and Revenue (set them with
// [ProductsService.Patch]).
//
// Nullable fields decode as their zero value when they are null, and
// [ProductsService.Create] and [ProductsService.Update] send zero values of
// nullable fields as null.
type Product struct {
	ID int `json:"id"`

	// Name is unique across all products, compared case-sensitively, and is at
	// most 255 characters.
	Name string `json:"name"`

	// Description is required and at most 4000 characters.
	Description string `json:"description"`

	// ProdType is the ID of the product's product type.
	ProdType int `json:"prod_type"`

	// BusinessCriticality is one of the BusinessCriticality constants, or ""
	// when unset.
	BusinessCriticality string `json:"business_criticality"`

	// Platform, Lifecycle and Origin are values of DefectDojo's lookup tables
	// (product platforms, lifecycles and origins), such as "web",
	// "production" and "internal", or "" when unset.
	Platform  string `json:"platform"`
	Lifecycle string `json:"lifecycle"`
	Origin    string `json:"origin"`

	// ProdNumericGrade is the grade DefectDojo calculates from the product's
	// findings. ProdNumericGrade, UserRecords and Revenue are nil when unset.
	ProdNumericGrade *int `json:"prod_numeric_grade"`

	// UserRecords is the estimated number of user records in the product's
	// database.
	UserRecords *int `json:"user_records"`

	// Revenue is the estimated revenue of the application, a decimal such as
	// "1000.00".
	Revenue *string `json:"revenue"`

	ExternalAudience   bool `json:"external_audience"`
	InternetAccessible bool `json:"internet_accessible"`

	// EnableProductTagInheritance makes the product's engagements, tests and
	// findings inherit its tags.
	EnableProductTagInheritance bool `json:"enable_product_tag_inheritance"`

	EnableSimpleRiskAcceptance bool `json:"enable_simple_risk_acceptance"`

	// EnableFullRiskAcceptance defaults to true in DefectDojo, but Create
	// sends false unless it is set.
	EnableFullRiskAcceptance bool `json:"enable_full_risk_acceptance"`

	DisableSLABreachNotifications bool `json:"disable_sla_breach_notifications"`

	// ProductManager, TechnicalContact and TeamManager are user IDs, or 0
	// when unset. Assigning a deactivated user is a 400.
	ProductManager   int `json:"product_manager"`
	TechnicalContact int `json:"technical_contact"`
	TeamManager      int `json:"team_manager"`

	// SLAConfiguration is the ID of the product's SLA configuration. When it
	// is 0, Create uses DefectDojo's default (1) and Update keeps the current
	// one. Changing it starts a recalculation of the findings' SLA dates in
	// the background; until that finishes, another change is a 400.
	SLAConfiguration int `json:"sla_configuration"`

	// AuthorizedUsers are the IDs of the users who may access the product, in
	// addition to superusers, staff and the product type's authorized users.
	// Writing it needs the permission to manage the product's members.
	AuthorizedUsers []int `json:"authorized_users"`

	// Regulations are the IDs of the regulations that apply to the product.
	Regulations []int `json:"regulations"`

	// Tags may not contain spaces, commas or quotes. DefectDojo keeps their
	// case, but tags that differ only in case are merged into the first.
	Tags []string `json:"tags"`

	// FindingsCount and FindingsList are the number and IDs of the product's
	// active findings.
	FindingsCount int   `json:"findings_count"`
	FindingsList  []int `json:"findings_list"`

	// ProductMeta are the product's custom fields.
	ProductMeta []ProductMeta `json:"product_meta"`

	Created time.Time `json:"created,omitzero"`

	// ArchivedAt and ArchiveBatch are set by the archive feature of
	// DefectDojo Pro (3.5+); they are always zero on other servers.
	ArchivedAt   time.Time `json:"archived_at,omitzero"`
	ArchiveBatch string    `json:"archive_batch,omitempty"`
}

// ProductMeta is a custom field of a product.
type ProductMeta struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// productRequest is the body of product create and update requests: the
// writable fields of a [Product], with zero values of nullable fields sent as
// null. prod_numeric_grade, user_records and revenue are left out, so that
// Update keeps them.
type productRequest struct {
	Name                          string   `json:"name"`
	Description                   string   `json:"description"`
	ProdType                      int      `json:"prod_type"`
	BusinessCriticality           *string  `json:"business_criticality"`
	Platform                      *string  `json:"platform"`
	Lifecycle                     *string  `json:"lifecycle"`
	Origin                        *string  `json:"origin"`
	ExternalAudience              bool     `json:"external_audience"`
	InternetAccessible            bool     `json:"internet_accessible"`
	EnableProductTagInheritance   bool     `json:"enable_product_tag_inheritance"`
	EnableSimpleRiskAcceptance    bool     `json:"enable_simple_risk_acceptance"`
	EnableFullRiskAcceptance      bool     `json:"enable_full_risk_acceptance"`
	DisableSLABreachNotifications bool     `json:"disable_sla_breach_notifications"`
	ProductManager                *int     `json:"product_manager"`
	TechnicalContact              *int     `json:"technical_contact"`
	TeamManager                   *int     `json:"team_manager"`
	SLAConfiguration              int      `json:"sla_configuration,omitempty"`
	AuthorizedUsers               []int    `json:"authorized_users"`
	Regulations                   []int    `json:"regulations"`
	Tags                          []string `json:"tags"`
}

// newProductRequest validates a product and returns its writable fields.
func newProductRequest(p *Product) (*productRequest, error) {
	switch {
	case p == nil:
		return nil, errors.New("defectdojo: product must not be nil")
	case p.Name == "":
		return nil, errors.New("defectdojo: product Name must not be empty")
	case p.Description == "":
		return nil, errors.New("defectdojo: product Description must not be empty")
	case p.ProdType <= 0:
		return nil, fmt.Errorf("defectdojo: product ProdType must be positive, got %d", p.ProdType)
	}
	return &productRequest{
		Name:                          p.Name,
		Description:                   p.Description,
		ProdType:                      p.ProdType,
		BusinessCriticality:           orNull(p.BusinessCriticality),
		Platform:                      orNull(p.Platform),
		Lifecycle:                     orNull(p.Lifecycle),
		Origin:                        orNull(p.Origin),
		ExternalAudience:              p.ExternalAudience,
		InternetAccessible:            p.InternetAccessible,
		EnableProductTagInheritance:   p.EnableProductTagInheritance,
		EnableSimpleRiskAcceptance:    p.EnableSimpleRiskAcceptance,
		EnableFullRiskAcceptance:      p.EnableFullRiskAcceptance,
		DisableSLABreachNotifications: p.DisableSLABreachNotifications,
		ProductManager:                orNull(p.ProductManager),
		TechnicalContact:              orNull(p.TechnicalContact),
		TeamManager:                   orNull(p.TeamManager),
		SLAConfiguration:              p.SLAConfiguration,
		// null is a 400 for lists; an empty list removes all entries.
		AuthorizedUsers: orEmpty(p.AuthorizedUsers),
		Regulations:     orEmpty(p.Regulations),
		Tags:            orEmpty(p.Tags),
	}, nil
}

// ProductPatch is the body of [ProductsService.Patch]. Only the fields that
// are set are changed.
type ProductPatch struct {
	Name                          *string          `json:"name,omitempty"`
	Description                   *string          `json:"description,omitempty"`
	ProdType                      *int             `json:"prod_type,omitempty"`
	BusinessCriticality           Nullable[string] `json:"business_criticality,omitzero"`
	Platform                      Nullable[string] `json:"platform,omitzero"`
	Lifecycle                     Nullable[string] `json:"lifecycle,omitzero"`
	Origin                        Nullable[string] `json:"origin,omitzero"`
	ProdNumericGrade              Nullable[int]    `json:"prod_numeric_grade,omitzero"`
	UserRecords                   Nullable[int]    `json:"user_records,omitzero"`
	Revenue                       Nullable[string] `json:"revenue,omitzero"`
	ExternalAudience              *bool            `json:"external_audience,omitempty"`
	InternetAccessible            *bool            `json:"internet_accessible,omitempty"`
	EnableProductTagInheritance   *bool            `json:"enable_product_tag_inheritance,omitempty"`
	EnableSimpleRiskAcceptance    *bool            `json:"enable_simple_risk_acceptance,omitempty"`
	EnableFullRiskAcceptance      *bool            `json:"enable_full_risk_acceptance,omitempty"`
	DisableSLABreachNotifications *bool            `json:"disable_sla_breach_notifications,omitempty"`
	ProductManager                Nullable[int]    `json:"product_manager,omitzero"`
	TechnicalContact              Nullable[int]    `json:"technical_contact,omitzero"`
	TeamManager                   Nullable[int]    `json:"team_manager,omitzero"`
	SLAConfiguration              *int             `json:"sla_configuration,omitempty"`

	// AuthorizedUsers, Regulations and Tags replace the current entries; a
	// pointer to an empty or nil slice removes them all.
	AuthorizedUsers *[]int    `json:"authorized_users,omitempty"`
	Regulations     *[]int    `json:"regulations,omitempty"`
	Tags            *[]string `json:"tags,omitempty"`
}

// ListProductsOptions are the options of [ProductsService.List] and
// [ProductsService.ListAll].
type ListProductsOptions struct {
	ListOptions

	// Name filters by names that contain it, ignoring case.
	Name string `url:"name,omitempty"`

	// NameExact filters by name, ignoring case: it can match several
	// products whose names differ only in case.
	NameExact string `url:"name_exact,omitempty"`

	// ProdType filters by product type ID.
	ProdType int `url:"prod_type,omitempty"`
}

// ProductList is a page of products.
type ProductList struct {
	Products []*Product

	// Count is the number of products that match the filters.
	Count int
}

// List returns a page of products.
//
// DefectDojo API: GET /api/v2/products/
func (s *ProductsService) List(ctx context.Context, opt *ListProductsOptions, opts ...RequestOption) (*ProductList, *Response, error) {
	path, err := addQuery("/api/v2/products/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*Product]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ProductList{Products: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the products, starting at opt.Offset. It
// fetches opt.Limit products per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *ProductsService) ListAll(ctx context.Context, opt *ListProductsOptions, opts ...RequestOption) iter.Seq2[*Product, error] {
	base := ListProductsOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*Product, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return result.Products, result.Count, nil
	})
}

// Get returns a product by its ID.
//
// DefectDojo API: GET /api/v2/products/{id}/
func (s *ProductsService) Get(ctx context.Context, id int, opts ...RequestOption) (*Product, *Response, error) {
	if err := requirePositive("product ID", id); err != nil {
		return nil, nil, err
	}
	p := new(Product)
	resp, err := s.client.do(ctx, http.MethodGet, productPath(id), nil, p, opts)
	if err != nil {
		return nil, resp, err
	}
	return p, resp, nil
}

// GetByName returns the product with exactly the given name, compared
// case-sensitively. It returns an error matching [ErrNotFound] when there is
// none.
func (s *ProductsService) GetByName(ctx context.Context, name string, opts ...RequestOption) (*Product, *Response, error) {
	if name == "" {
		return nil, nil, errors.New("defectdojo: product name must not be empty")
	}
	// name_exact ignores case, so it can match several products.
	list, resp, err := s.List(ctx, &ListProductsOptions{ListOptions: ListOptions{Limit: 100}, NameExact: name}, opts...)
	if err != nil {
		return nil, resp, err
	}
	for _, p := range list.Products {
		if p.Name == name {
			return p, resp, nil
		}
	}
	return nil, resp, fmt.Errorf("defectdojo: product %q: %w", name, ErrNotFound)
}

// Create creates a product. Its read-only fields are ignored. A name that is
// taken is a 400 ([ErrBadRequest]) with a "name" field error.
//
// Every writable field is sent, so the booleans that default to true in
// DefectDojo, such as EnableFullRiskAcceptance, are false unless they are
// set. A zero SLAConfiguration uses DefectDojo's default.
//
// DefectDojo API: POST /api/v2/products/
func (s *ProductsService) Create(ctx context.Context, p *Product, opts ...RequestOption) (*Product, *Response, error) {
	body, err := newProductRequest(p)
	if err != nil {
		return nil, nil, err
	}
	created := new(Product)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/products/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// Update replaces the writable fields of the product with the given ID.
// Every writable field is sent except a zero SLAConfiguration and
// ProdNumericGrade, UserRecords and Revenue, which are kept: empty nullable
// fields are cleared, and nil AuthorizedUsers, Regulations and Tags remove
// all entries. Update a product returned by [ProductsService.Get] to keep its
// other fields, or use [ProductsService.Patch].
//
// DefectDojo API: PUT /api/v2/products/{id}/
func (s *ProductsService) Update(ctx context.Context, id int, p *Product, opts ...RequestOption) (*Product, *Response, error) {
	if err := requirePositive("product ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newProductRequest(p)
	if err != nil {
		return nil, nil, err
	}
	updated := new(Product)
	resp, err := s.client.do(ctx, http.MethodPut, productPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Patch changes the fields of the product that are set in patch.
//
// DefectDojo API: PATCH /api/v2/products/{id}/
func (s *ProductsService) Patch(ctx context.Context, id int, patch *ProductPatch, opts ...RequestOption) (*Product, *Response, error) {
	if err := requirePositive("product ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: product patch must not be nil")
	}
	// null is a 400 for lists; send [] from a copy, leaving the caller's
	// patch as is.
	p := *patch
	p.AuthorizedUsers = orEmptyPtr(p.AuthorizedUsers)
	p.Regulations = orEmptyPtr(p.Regulations)
	p.Tags = orEmptyPtr(p.Tags)
	updated := new(Product)
	resp, err := s.client.do(ctx, http.MethodPatch, productPath(id), &p, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Delete deletes a product by its ID.
//
// Deleting a product also deletes its engagements, with their tests and
// findings. Use [ProductsService.DeletePreview] or
// [ProductsService.CountEngagements] first to see what would be deleted.
//
// When the server runs with asynchronous deletes (DD_ASYNC_OBJECT_DELETE),
// DefectDojo answers before the product is gone: poll [ProductsService.Get]
// until it returns [ErrNotFound].
//
// DefectDojo API: DELETE /api/v2/products/{id}/
func (s *ProductsService) Delete(ctx context.Context, id int, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("product ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, productPath(id), nil, nil, opts)
}

// DeletePreview returns a page of the objects that deleting the product
// would remove, the product itself included.
//
// DefectDojo API: GET /api/v2/products/{id}/delete_preview/
func (s *ProductsService) DeletePreview(ctx context.Context, id int, opt *ListOptions, opts ...RequestOption) (*DeletePreviewList, *Response, error) {
	if err := requirePositive("product ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(productPath(id)+"delete_preview/", opt)
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

// CountEngagements returns the number of engagements of the product, which a
// delete of the product would also delete. It reads the count of the
// engagements list (GET /api/v2/engagements/?product={id}), whose filter
// rejects products that don't exist: the error then also matches
// [ErrNotFound].
func (s *ProductsService) CountEngagements(ctx context.Context, id int, opts ...RequestOption) (int, *Response, error) {
	if err := requirePositive("product ID", id); err != nil {
		return 0, nil, err
	}
	return s.client.count(ctx, "/api/v2/engagements/?limit=1&product="+strconv.Itoa(id), "product", opts)
}

func productPath(id int) string {
	return "/api/v2/products/" + strconv.Itoa(id) + "/"
}

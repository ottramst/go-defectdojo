package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// RegulationsService handles communication with the regulation methods of the
// DefectDojo API. Regulations, such as PCI DSS or HIPAA, are listed on the
// products they apply to ([Product].Regulations).
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type RegulationsService service

// The categories of regulations. DefectDojo checks the category; newer
// versions may add some.
const (
	RegulationCategoryPrivacy    = "privacy"
	RegulationCategoryFinance    = "finance"
	RegulationCategoryEducation  = "education"
	RegulationCategoryMedical    = "medical"
	RegulationCategoryCorporate  = "corporate"
	RegulationCategorySecurity   = "security"
	RegulationCategoryGovernment = "government"
	RegulationCategoryOther      = "other"
)

// Regulation is a DefectDojo regulation. The same type is used for reads and
// writes: the read-only ID is ignored when writing.
type Regulation struct {
	ID int `json:"id"`

	// Name (at most 128 characters) and Acronym (at most 20) are each
	// unique across all regulations, compared case-sensitively.
	Name    string `json:"name"`
	Acronym string `json:"acronym"`

	// Category is one of the RegulationCategory constants.
	Category string `json:"category"`

	// Jurisdiction is the territory the regulation applies to, at most 64
	// characters.
	Jurisdiction string `json:"jurisdiction"`

	// Description and Reference, a URL, are optional; DefectDojo stores
	// them as "" when unset.
	Description string `json:"description"`
	Reference   string `json:"reference"`
}

// regulationRequest is the body of regulation create and update requests: the
// writable fields of a [Regulation].
type regulationRequest struct {
	Name         string `json:"name"`
	Acronym      string `json:"acronym"`
	Category     string `json:"category"`
	Jurisdiction string `json:"jurisdiction"`
	Description  string `json:"description"`
	Reference    string `json:"reference"`
}

// newRegulationRequest validates a regulation and returns its writable fields.
func newRegulationRequest(r *Regulation) (*regulationRequest, error) {
	if r == nil {
		return nil, errors.New("defectdojo: regulation must not be nil")
	}
	switch {
	case r.Name == "":
		return nil, errors.New("defectdojo: regulation Name must not be empty")
	case r.Acronym == "":
		return nil, errors.New("defectdojo: regulation Acronym must not be empty")
	case r.Jurisdiction == "":
		return nil, errors.New("defectdojo: regulation Jurisdiction must not be empty")
	case r.Category == "":
		return nil, errors.New("defectdojo: regulation Category must not be empty")
	}
	return &regulationRequest{
		Name:         r.Name,
		Acronym:      r.Acronym,
		Category:     r.Category,
		Jurisdiction: r.Jurisdiction,
		Description:  r.Description,
		Reference:    r.Reference,
	}, nil
}

// RegulationPatch is the body of [RegulationsService.Patch]. Only the fields
// that are set are changed; an empty Description or Reference clears it.
type RegulationPatch struct {
	Name         *string `json:"name,omitempty"`
	Acronym      *string `json:"acronym,omitempty"`
	Category     *string `json:"category,omitempty"`
	Jurisdiction *string `json:"jurisdiction,omitempty"`
	Description  *string `json:"description,omitempty"`
	Reference    *string `json:"reference,omitempty"`
}

// ListRegulationsOptions are the options of [RegulationsService.List] and
// [RegulationsService.ListAll].
type ListRegulationsOptions struct {
	ListOptions

	// Name filters by exact, case-sensitive name. DefectDojo has no acronym
	// filter; see [RegulationsService.GetByAcronym].
	Name string `url:"name,omitempty"`
}

// RegulationList is a page of regulations.
type RegulationList struct {
	Regulations []*Regulation

	// Count is the number of regulations that match the filters.
	Count int
}

// List returns a page of regulations, ordered by ID.
//
// DefectDojo API: GET /api/v2/regulations/
func (s *RegulationsService) List(ctx context.Context, opt *ListRegulationsOptions, opts ...RequestOption) (*RegulationList, *Response, error) {
	path, err := addQuery("/api/v2/regulations/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*Regulation]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &RegulationList{Regulations: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the regulations, starting at opt.Offset.
// It fetches opt.Limit regulations per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *RegulationsService) ListAll(ctx context.Context, opt *ListRegulationsOptions, opts ...RequestOption) iter.Seq2[*Regulation, error] {
	base := ListRegulationsOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*Regulation, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return result.Regulations, result.Count, nil
	})
}

// Get returns a regulation by its ID.
//
// DefectDojo API: GET /api/v2/regulations/{id}/
func (s *RegulationsService) Get(ctx context.Context, id int, opts ...RequestOption) (*Regulation, *Response, error) {
	if err := requirePositive("regulation ID", id); err != nil {
		return nil, nil, err
	}
	r := new(Regulation)
	resp, err := s.client.do(ctx, http.MethodGet, regulationPath(id), nil, r, opts)
	if err != nil {
		return nil, resp, err
	}
	return r, resp, nil
}

// GetByName returns the regulation with exactly the given name. It returns an
// error matching [ErrNotFound] when there is none.
func (s *RegulationsService) GetByName(ctx context.Context, name string, opts ...RequestOption) (*Regulation, *Response, error) {
	if name == "" {
		return nil, nil, errors.New("defectdojo: regulation name must not be empty")
	}
	list, resp, err := s.List(ctx, &ListRegulationsOptions{Name: name}, opts...)
	if err != nil {
		return nil, resp, err
	}
	// The filter is exact today; check again in case a version loosens it.
	for _, r := range list.Regulations {
		if r.Name == name {
			return r, resp, nil
		}
	}
	return nil, resp, fmt.Errorf("defectdojo: regulation %q: %w", name, ErrNotFound)
}

// GetByAcronym returns the regulation with exactly the given acronym,
// compared case-sensitively. It returns an error matching [ErrNotFound] when
// there is none.
//
// DefectDojo cannot filter regulations by acronym, so GetByAcronym pages
// through all of them; the returned response is the last page's.
func (s *RegulationsService) GetByAcronym(ctx context.Context, acronym string, opts ...RequestOption) (*Regulation, *Response, error) {
	if acronym == "" {
		return nil, nil, errors.New("defectdojo: regulation acronym must not be empty")
	}
	opt := &ListRegulationsOptions{ListOptions: ListOptions{Limit: 100}}
	for {
		list, resp, err := s.List(ctx, opt, opts...)
		if err != nil {
			return nil, resp, err
		}
		for _, r := range list.Regulations {
			if r.Acronym == acronym {
				return r, resp, nil
			}
		}
		opt.Offset += len(list.Regulations)
		if len(list.Regulations) == 0 || opt.Offset >= list.Count {
			return nil, resp, fmt.Errorf("defectdojo: regulation with acronym %q: %w", acronym, ErrNotFound)
		}
	}
}

// Create creates a regulation. Its read-only ID is ignored. A name or acronym
// that is taken is a 400 ([ErrBadRequest]) with a "name" or "acronym" field
// error.
//
// DefectDojo API: POST /api/v2/regulations/
func (s *RegulationsService) Create(ctx context.Context, r *Regulation, opts ...RequestOption) (*Regulation, *Response, error) {
	body, err := newRegulationRequest(r)
	if err != nil {
		return nil, nil, err
	}
	created := new(Regulation)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/regulations/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// Update replaces the writable fields of the regulation with the given ID.
// Every writable field is sent: an empty Description or Reference clears it.
//
// DefectDojo API: PUT /api/v2/regulations/{id}/
func (s *RegulationsService) Update(ctx context.Context, id int, r *Regulation, opts ...RequestOption) (*Regulation, *Response, error) {
	if err := requirePositive("regulation ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newRegulationRequest(r)
	if err != nil {
		return nil, nil, err
	}
	updated := new(Regulation)
	resp, err := s.client.do(ctx, http.MethodPut, regulationPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Patch changes the fields of the regulation that are set in patch.
//
// DefectDojo API: PATCH /api/v2/regulations/{id}/
func (s *RegulationsService) Patch(ctx context.Context, id int, patch *RegulationPatch, opts ...RequestOption) (*Regulation, *Response, error) {
	if err := requirePositive("regulation ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: regulation patch must not be nil")
	}
	updated := new(Regulation)
	resp, err := s.client.do(ctx, http.MethodPatch, regulationPath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Delete deletes a regulation by its ID. The products it applies to lose it
// silently: DefectDojo removes it from their Regulations.
//
// DefectDojo API: DELETE /api/v2/regulations/{id}/
func (s *RegulationsService) Delete(ctx context.Context, id int, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("regulation ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, regulationPath(id), nil, nil, opts)
}

// DeletePreview returns a page of the objects that deleting the regulation
// would remove, the regulation itself included.
//
// DefectDojo API: GET /api/v2/regulations/{id}/delete_preview/
func (s *RegulationsService) DeletePreview(ctx context.Context, id int, opt *ListOptions, opts ...RequestOption) (*DeletePreviewList, *Response, error) {
	if err := requirePositive("regulation ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(regulationPath(id)+"delete_preview/", opt)
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

func regulationPath(id int) string {
	return "/api/v2/regulations/" + strconv.Itoa(id) + "/"
}

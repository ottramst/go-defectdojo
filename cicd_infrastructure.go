package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// CICDInfrastructureService handles communication with the CI/CD
// infrastructure methods of the DefectDojo API: the source code management
// servers, build servers and orchestration engines that engagements refer to
// ([Engagement].CICDSCMServer, CICDBuildServer and CICDOrchestrationEngine).
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type CICDInfrastructureService service

// CICDInfrastructureType is the type of a CI/CD infrastructure object.
type CICDInfrastructureType string

// The types of CI/CD infrastructure. An engagement's CICDSCMServer must be of
// type CICDInfrastructureTypeSCMServer, its CICDBuildServer of type
// CICDInfrastructureTypeBuildServer and its CICDOrchestrationEngine of type
// CICDInfrastructureTypeOrchestration. DefectDojo checks the type.
const (
	CICDInfrastructureTypeSCMServer     CICDInfrastructureType = "scm_server"
	CICDInfrastructureTypeBuildServer   CICDInfrastructureType = "build_server"
	CICDInfrastructureTypeOrchestration CICDInfrastructureType = "orchestration"
)

// CICDInfrastructure is a DefectDojo CI/CD infrastructure object. The same
// type is used for reads and writes: the read-only ID is ignored when
// writing.
type CICDInfrastructure struct {
	ID int `json:"id"`

	// Name is at most 200 characters, unique among the objects of its type.
	Name string `json:"name"`

	// InfrastructureType is a type of CI/CD infrastructure, such as the
	// CICDInfrastructureType constants. It can't change after creation:
	// DefectDojo ignores it in updates.
	InfrastructureType CICDInfrastructureType `json:"infrastructure_type"`

	// Description (at most 2000 characters) and URL (at most 2000) are
	// optional; DefectDojo stores them as "" when unset. DefectDojo leaves
	// them out, in reads and writes, for users without the view or change
	// permission for CI/CD infrastructure: they read as "", and the values
	// of users who may only add objects are ignored.
	Description string `json:"description"`
	URL         string `json:"url"`
}

// cicdInfrastructureRequest is the body of CI/CD infrastructure create and
// update requests: the writable fields of a [CICDInfrastructure].
type cicdInfrastructureRequest struct {
	Name               string                 `json:"name"`
	InfrastructureType CICDInfrastructureType `json:"infrastructure_type"`
	Description        string                 `json:"description"`
	URL                string                 `json:"url"`
}

// newCICDInfrastructureRequest validates a CI/CD infrastructure object and
// returns its writable fields.
func newCICDInfrastructureRequest(ci *CICDInfrastructure) (*cicdInfrastructureRequest, error) {
	if ci == nil {
		return nil, errors.New("defectdojo: CI/CD infrastructure must not be nil")
	}
	switch {
	case ci.Name == "":
		return nil, errors.New("defectdojo: CI/CD infrastructure Name must not be empty")
	case ci.InfrastructureType == "":
		return nil, errors.New("defectdojo: CI/CD infrastructure InfrastructureType must not be empty")
	}
	return &cicdInfrastructureRequest{
		Name:               ci.Name,
		InfrastructureType: ci.InfrastructureType,
		Description:        ci.Description,
		URL:                ci.URL,
	}, nil
}

// CICDInfrastructurePatch is the body of [CICDInfrastructureService.Patch].
// Only the fields that are set are changed; an empty Description or URL
// clears it. The type can't change, so it has no field here.
type CICDInfrastructurePatch struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	URL         *string `json:"url,omitempty"`
}

// ListCICDInfrastructureOptions are the options of
// [CICDInfrastructureService.List] and [CICDInfrastructureService.ListAll].
type ListCICDInfrastructureOptions struct {
	ListOptions

	// Name filters by exact, case-sensitive name.
	Name string `url:"name,omitempty"`

	// InfrastructureType filters by type.
	InfrastructureType CICDInfrastructureType `url:"infrastructure_type,omitempty"`
}

// ListCICDInfrastructureResult is a page of CI/CD infrastructure objects.
type ListCICDInfrastructureResult struct {
	Results []CICDInfrastructure

	// Count is the number of objects that match the filters.
	Count int
}

// List returns a page of CI/CD infrastructure objects, ordered by ID.
//
// DefectDojo API: GET /api/v2/cicd_infrastructure/
func (s *CICDInfrastructureService) List(ctx context.Context, opt *ListCICDInfrastructureOptions, opts ...RequestOption) (*ListCICDInfrastructureResult, *Response, error) {
	path, err := addQuery("/api/v2/cicd_infrastructure/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[CICDInfrastructure]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListCICDInfrastructureResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the CI/CD infrastructure objects, starting
// at opt.Offset. It fetches opt.Limit objects per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *CICDInfrastructureService) ListAll(ctx context.Context, opt *ListCICDInfrastructureOptions, opts ...RequestOption) iter.Seq2[*CICDInfrastructure, error] {
	base := ListCICDInfrastructureOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*CICDInfrastructure, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Results), result.Count, nil
	})
}

// GetCICDInfrastructureOptions are the options of [CICDInfrastructureService.Get].
type GetCICDInfrastructureOptions struct{}

// Get returns a CI/CD infrastructure object by its ID.
//
// DefectDojo API: GET /api/v2/cicd_infrastructure/{id}/
func (s *CICDInfrastructureService) Get(ctx context.Context, id int, opt *GetCICDInfrastructureOptions, opts ...RequestOption) (*CICDInfrastructure, *Response, error) {
	if err := requirePositive("CI/CD infrastructure ID", id); err != nil {
		return nil, nil, err
	}
	ci := new(CICDInfrastructure)
	resp, err := s.client.do(ctx, http.MethodGet, cicdInfrastructurePath(id), nil, ci, opts)
	if err != nil {
		return nil, resp, err
	}
	return ci, resp, nil
}

// GetCICDInfrastructureByNameOptions are the options of [CICDInfrastructureService.GetByName].
type GetCICDInfrastructureByNameOptions struct {
	// InfrastructureType restricts the lookup to one type, one of the
	// CICDInfrastructureType constants. Names are unique within a type.
	InfrastructureType CICDInfrastructureType
}

// GetByName returns the CI/CD infrastructure object with exactly the given
// name, and the type of the options if they give one. Names are unique within
// a type; without a type, any type matches, and several matches are an error
// matching [ErrAmbiguous] that names their IDs. It returns an error matching
// [ErrNotFound] when there is none, and a 400 ([ErrBadRequest]) for an
// unknown type.
func (s *CICDInfrastructureService) GetByName(ctx context.Context, name string, opt *GetCICDInfrastructureByNameOptions, opts ...RequestOption) (*CICDInfrastructure, *Response, error) {
	if name == "" {
		return nil, nil, errors.New("defectdojo: CI/CD infrastructure name must not be empty")
	}
	var infrastructureType CICDInfrastructureType
	if opt != nil {
		infrastructureType = opt.InfrastructureType
	}
	list, resp, err := s.List(ctx, &ListCICDInfrastructureOptions{
		ListOptions:        ListOptions{Limit: 100},
		Name:               name,
		InfrastructureType: infrastructureType,
	}, opts...)
	if err != nil {
		return nil, resp, err
	}
	var (
		found *CICDInfrastructure
		ids   []int
	)
	// DefectDojo strips whitespace from the name filter, and may loosen the
	// filters: check again. There is at most one object per type, so one
	// page holds them.
	for i := range list.Results {
		ci := &list.Results[i]
		if ci.Name == name && (infrastructureType == "" || ci.InfrastructureType == infrastructureType) {
			found = ci
			ids = append(ids, ci.ID)
		}
	}
	switch len(ids) {
	case 0:
		return nil, resp, fmt.Errorf("defectdojo: CI/CD infrastructure %q: %w", name, ErrNotFound)
	case 1:
		return found, resp, nil
	default:
		return nil, resp, fmt.Errorf("%w: %d CI/CD infrastructure objects of different types are named %q: IDs %v; give the type", ErrAmbiguous, len(ids), name, ids)
	}
}

// CreateCICDInfrastructureOptions are the options of [CICDInfrastructureService.Create].
type CreateCICDInfrastructureOptions struct{}

// Create creates a CI/CD infrastructure object. Its read-only ID is ignored.
// A name that another object of the same type has is a 400 ([ErrBadRequest])
// with a "non_field_errors" error.
//
// DefectDojo API: POST /api/v2/cicd_infrastructure/
func (s *CICDInfrastructureService) Create(ctx context.Context, ci *CICDInfrastructure, opt *CreateCICDInfrastructureOptions, opts ...RequestOption) (*CICDInfrastructure, *Response, error) {
	body, err := newCICDInfrastructureRequest(ci)
	if err != nil {
		return nil, nil, err
	}
	created := new(CICDInfrastructure)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/cicd_infrastructure/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateCICDInfrastructureOptions are the options of [CICDInfrastructureService.Update].
type UpdateCICDInfrastructureOptions struct{}

// Update replaces the writable fields of the CI/CD infrastructure object with
// the given ID: an empty Description or URL clears it. InfrastructureType must
// be set (the schema requires it), but DefectDojo ignores it on an existing
// object: set it to the current type. A name that another object of the
// type has is a 409 ([ErrConflict]), not the 400 of
// [CICDInfrastructureService.Create].
//
// DefectDojo API: PUT /api/v2/cicd_infrastructure/{id}/
func (s *CICDInfrastructureService) Update(ctx context.Context, id int, ci *CICDInfrastructure, opt *UpdateCICDInfrastructureOptions, opts ...RequestOption) (*CICDInfrastructure, *Response, error) {
	if err := requirePositive("CI/CD infrastructure ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newCICDInfrastructureRequest(ci)
	if err != nil {
		return nil, nil, err
	}
	updated := new(CICDInfrastructure)
	resp, err := s.client.do(ctx, http.MethodPut, cicdInfrastructurePath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// PatchCICDInfrastructureOptions are the options of [CICDInfrastructureService.Patch].
type PatchCICDInfrastructureOptions struct{}

// Patch changes the fields of the CI/CD infrastructure object that are set in
// patch. A name that another object of the type has is a 409 ([ErrConflict]).
//
// DefectDojo API: PATCH /api/v2/cicd_infrastructure/{id}/
func (s *CICDInfrastructureService) Patch(ctx context.Context, id int, patch *CICDInfrastructurePatch, opt *PatchCICDInfrastructureOptions, opts ...RequestOption) (*CICDInfrastructure, *Response, error) {
	if err := requirePositive("CI/CD infrastructure ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: CI/CD infrastructure patch must not be nil")
	}
	updated := new(CICDInfrastructure)
	resp, err := s.client.do(ctx, http.MethodPatch, cicdInfrastructurePath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteCICDInfrastructureOptions are the options of [CICDInfrastructureService.Delete].
type DeleteCICDInfrastructureOptions struct{}

// Delete deletes a CI/CD infrastructure object by its ID. The engagements
// that refer to it lose the reference.
//
// DefectDojo API: DELETE /api/v2/cicd_infrastructure/{id}/
func (s *CICDInfrastructureService) Delete(ctx context.Context, id int, opt *DeleteCICDInfrastructureOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("CI/CD infrastructure ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, cicdInfrastructurePath(id), nil, nil, opts)
}

// DeletePreviewCICDInfrastructureOptions are the options of [CICDInfrastructureService.DeletePreview].
type DeletePreviewCICDInfrastructureOptions struct {
	ListOptions
}

// DeletePreview returns a page of the objects that deleting the CI/CD
// infrastructure object would remove, the object itself included.
//
// DefectDojo API: GET /api/v2/cicd_infrastructure/{id}/delete_preview/
func (s *CICDInfrastructureService) DeletePreview(ctx context.Context, id int, opt *DeletePreviewCICDInfrastructureOptions, opts ...RequestOption) (*DeletePreviewResult, *Response, error) {
	if err := requirePositive("CI/CD infrastructure ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(cicdInfrastructurePath(id)+"delete_preview/", opt)
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

func cicdInfrastructurePath(id int) string {
	return "/api/v2/cicd_infrastructure/" + strconv.Itoa(id) + "/"
}

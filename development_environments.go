package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// DevelopmentEnvironmentsService handles communication with the development
// environment methods of the DefectDojo API. Tests name the environment they
// ran in, such as "Production" or "Staging".
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type DevelopmentEnvironmentsService service

// DevelopmentEnvironment is a DefectDojo development environment. The same
// type is used for reads and writes: the read-only ID is ignored when writing.
type DevelopmentEnvironment struct {
	ID int `json:"id"`

	// Name is at most 200 characters. DefectDojo doesn't require it to be
	// unique, but keep it unique: DefectDojo's imports find the environment
	// of a scan by name ("Development" when none is given), and fail when
	// several environments have it or, without auto-create, when none has.
	Name string `json:"name"`
}

// developmentEnvironmentRequest is the body of development environment create
// and update requests: the writable fields of a [DevelopmentEnvironment].
type developmentEnvironmentRequest struct {
	Name string `json:"name"`
}

// newDevelopmentEnvironmentRequest validates a development environment and
// returns its writable fields.
func newDevelopmentEnvironmentRequest(env *DevelopmentEnvironment) (*developmentEnvironmentRequest, error) {
	if env == nil {
		return nil, errors.New("defectdojo: development environment must not be nil")
	}
	if env.Name == "" {
		return nil, errors.New("defectdojo: development environment Name must not be empty")
	}
	return &developmentEnvironmentRequest{Name: env.Name}, nil
}

// DevelopmentEnvironmentPatch is the body of
// [DevelopmentEnvironmentsService.Patch]. Only the fields that are set are
// changed.
type DevelopmentEnvironmentPatch struct {
	Name *string `json:"name,omitempty"`
}

// ListDevelopmentEnvironmentsOptions are the options of
// [DevelopmentEnvironmentsService.List] and
// [DevelopmentEnvironmentsService.ListAll]. DefectDojo has no filters for
// development environments, not even by name.
type ListDevelopmentEnvironmentsOptions struct {
	ListOptions
}

// ListDevelopmentEnvironmentsResult is a page of development environments.
type ListDevelopmentEnvironmentsResult struct {
	Results []DevelopmentEnvironment

	// Count is the number of development environments.
	Count int
}

// List returns a page of development environments, ordered by ID.
//
// DefectDojo API: GET /api/v2/development_environments/
func (s *DevelopmentEnvironmentsService) List(ctx context.Context, opt *ListDevelopmentEnvironmentsOptions, opts ...RequestOption) (*ListDevelopmentEnvironmentsResult, *Response, error) {
	path, err := addQuery("/api/v2/development_environments/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[DevelopmentEnvironment]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListDevelopmentEnvironmentsResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the development environments, starting at
// opt.Offset. It fetches opt.Limit development environments per request (100
// if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *DevelopmentEnvironmentsService) ListAll(ctx context.Context, opt *ListDevelopmentEnvironmentsOptions, opts ...RequestOption) iter.Seq2[*DevelopmentEnvironment, error] {
	base := ListDevelopmentEnvironmentsOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*DevelopmentEnvironment, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Results), result.Count, nil
	})
}

// GetDevelopmentEnvironmentOptions are the options of [DevelopmentEnvironmentsService.Get].
type GetDevelopmentEnvironmentOptions struct{}

// Get returns a development environment by its ID.
//
// DefectDojo API: GET /api/v2/development_environments/{id}/
func (s *DevelopmentEnvironmentsService) Get(ctx context.Context, id int, opt *GetDevelopmentEnvironmentOptions, opts ...RequestOption) (*DevelopmentEnvironment, *Response, error) {
	if err := requirePositive("development environment ID", id); err != nil {
		return nil, nil, err
	}
	env := new(DevelopmentEnvironment)
	resp, err := s.client.do(ctx, http.MethodGet, developmentEnvironmentPath(id), nil, env, opts)
	if err != nil {
		return nil, resp, err
	}
	return env, resp, nil
}

// GetDevelopmentEnvironmentByNameOptions are the options of [DevelopmentEnvironmentsService.GetByName].
type GetDevelopmentEnvironmentByNameOptions struct{}

// GetByName returns the development environment with exactly the given name,
// compared case-sensitively. It returns an error matching [ErrNotFound] when
// there is none, and an error matching [ErrAmbiguous] that names their IDs
// when several environments have the name.
//
// DefectDojo cannot filter development environments by name, so GetByName
// pages through all of them; the returned response is the last page's.
func (s *DevelopmentEnvironmentsService) GetByName(ctx context.Context, name string, opt *GetDevelopmentEnvironmentByNameOptions, opts ...RequestOption) (*DevelopmentEnvironment, *Response, error) {
	if name == "" {
		return nil, nil, errors.New("defectdojo: development environment name must not be empty")
	}
	var (
		found *DevelopmentEnvironment
		ids   []int
		resp  *Response
	)
	listOpt := &ListDevelopmentEnvironmentsOptions{ListOptions: ListOptions{Limit: 100}}
	for {
		list, r, err := s.List(ctx, listOpt, opts...)
		resp = r
		if err != nil {
			return nil, resp, err
		}
		for i := range list.Results {
			env := &list.Results[i]
			if env.Name == name {
				found = env
				ids = append(ids, env.ID)
			}
		}
		listOpt.Offset += len(list.Results)
		if len(list.Results) == 0 || listOpt.Offset >= list.Count {
			break
		}
	}
	switch len(ids) {
	case 0:
		return nil, resp, fmt.Errorf("defectdojo: development environment %q: %w", name, ErrNotFound)
	case 1:
		return found, resp, nil
	default:
		return nil, resp, fmt.Errorf("%w: %d development environments are named %q: IDs %v", ErrAmbiguous, len(ids), name, ids)
	}
}

// CreateDevelopmentEnvironmentOptions are the options of [DevelopmentEnvironmentsService.Create].
type CreateDevelopmentEnvironmentOptions struct{}

// Create creates a development environment. Its read-only ID is ignored.
// DefectDojo accepts a name that another environment already has, but then
// fails the imports that name it.
//
// DefectDojo API: POST /api/v2/development_environments/
func (s *DevelopmentEnvironmentsService) Create(ctx context.Context, env *DevelopmentEnvironment, opt *CreateDevelopmentEnvironmentOptions, opts ...RequestOption) (*DevelopmentEnvironment, *Response, error) {
	body, err := newDevelopmentEnvironmentRequest(env)
	if err != nil {
		return nil, nil, err
	}
	created := new(DevelopmentEnvironment)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/development_environments/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateDevelopmentEnvironmentOptions are the options of [DevelopmentEnvironmentsService.Update].
type UpdateDevelopmentEnvironmentOptions struct{}

// Update replaces the writable fields of the development environment with the
// given ID.
//
// DefectDojo API: PUT /api/v2/development_environments/{id}/
func (s *DevelopmentEnvironmentsService) Update(ctx context.Context, id int, env *DevelopmentEnvironment, opt *UpdateDevelopmentEnvironmentOptions, opts ...RequestOption) (*DevelopmentEnvironment, *Response, error) {
	if err := requirePositive("development environment ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newDevelopmentEnvironmentRequest(env)
	if err != nil {
		return nil, nil, err
	}
	updated := new(DevelopmentEnvironment)
	resp, err := s.client.do(ctx, http.MethodPut, developmentEnvironmentPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// PatchDevelopmentEnvironmentOptions are the options of [DevelopmentEnvironmentsService.Patch].
type PatchDevelopmentEnvironmentOptions struct{}

// Patch changes the fields of the development environment that are set in
// patch.
//
// DefectDojo API: PATCH /api/v2/development_environments/{id}/
func (s *DevelopmentEnvironmentsService) Patch(ctx context.Context, id int, patch *DevelopmentEnvironmentPatch, opt *PatchDevelopmentEnvironmentOptions, opts ...RequestOption) (*DevelopmentEnvironment, *Response, error) {
	if err := requirePositive("development environment ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: development environment patch must not be nil")
	}
	updated := new(DevelopmentEnvironment)
	resp, err := s.client.do(ctx, http.MethodPatch, developmentEnvironmentPath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteDevelopmentEnvironmentOptions are the options of [DevelopmentEnvironmentsService.Delete].
type DeleteDevelopmentEnvironmentOptions struct{}

// Delete deletes a development environment by its ID. Deleting an
// environment that tests use is a 409 ([ErrConflict]).
//
// DefectDojo API: DELETE /api/v2/development_environments/{id}/
func (s *DevelopmentEnvironmentsService) Delete(ctx context.Context, id int, opt *DeleteDevelopmentEnvironmentOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("development environment ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, developmentEnvironmentPath(id), nil, nil, opts)
}

// DeletePreviewDevelopmentEnvironmentOptions are the options of [DevelopmentEnvironmentsService.DeletePreview].
type DeletePreviewDevelopmentEnvironmentOptions struct {
	ListOptions
}

// DeletePreview returns a page of the objects that deleting the development
// environment would remove, the environment itself included. It doesn't show
// the tests that block the delete.
//
// DefectDojo API: GET /api/v2/development_environments/{id}/delete_preview/
func (s *DevelopmentEnvironmentsService) DeletePreview(ctx context.Context, id int, opt *DeletePreviewDevelopmentEnvironmentOptions, opts ...RequestOption) (*DeletePreviewResult, *Response, error) {
	if err := requirePositive("development environment ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(developmentEnvironmentPath(id)+"delete_preview/", opt)
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

func developmentEnvironmentPath(id int) string {
	return "/api/v2/development_environments/" + strconv.Itoa(id) + "/"
}

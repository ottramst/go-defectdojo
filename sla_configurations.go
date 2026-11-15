package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
	"strings"
)

// SLAConfigurationsService handles communication with the SLA configuration
// methods of the DefectDojo API. An SLA configuration sets how many days
// findings of each severity may stay open; every product has one.
//
// Changing the days of a configuration makes DefectDojo recalculate the SLA
// expiration dates of its findings in the background; see
// [IsSLARecalculating].
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type SLAConfigurationsService service

// SLAConfiguration is a DefectDojo SLA configuration. The same type is used
// for reads and writes; the read-only ID is ignored when writing.
//
// DefectDojo's fixtures create "Default" (ID 1), which products get unless
// they name another configuration and which cannot be deleted, and "No SLA
// Enforced" (ID 2).
type SLAConfiguration struct {
	ID int `json:"id"`

	// Name is unique across all SLA configurations, compared
	// case-sensitively, and at most 128 characters.
	Name string `json:"name"`

	// Description is at most 512 characters. An unset description is null,
	// which decodes as "".
	Description string `json:"description"`

	// Critical, High, Medium and Low are the number of days in which
	// findings of the severity must be remediated. They must be at least 1:
	// DefectDojo's API accepts 0 and negative days, which its UI rejects, so
	// the client rejects them before sending.
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`

	// EnforceCritical, EnforceHigh, EnforceMedium and EnforceLow assign SLA
	// expiration dates to findings of the severity. DefectDojo's default is
	// true; [SLAConfigurationsService.Create] sends false unless they are
	// set.
	EnforceCritical bool `json:"enforce_critical"`
	EnforceHigh     bool `json:"enforce_high"`
	EnforceMedium   bool `json:"enforce_medium"`
	EnforceLow      bool `json:"enforce_low"`

	// RestartSLAOnReactivation restarts the SLA of findings that a reimport
	// reactivates.
	RestartSLAOnReactivation bool `json:"restart_sla_on_reactivation"`
}

// slaConfigurationRequest is the body of SLA configuration create and update
// requests: the writable fields of an [SLAConfiguration].
type slaConfigurationRequest struct {
	Name                     string  `json:"name"`
	Description              *string `json:"description"`
	Critical                 int     `json:"critical"`
	EnforceCritical          bool    `json:"enforce_critical"`
	High                     int     `json:"high"`
	EnforceHigh              bool    `json:"enforce_high"`
	Medium                   int     `json:"medium"`
	EnforceMedium            bool    `json:"enforce_medium"`
	Low                      int     `json:"low"`
	EnforceLow               bool    `json:"enforce_low"`
	RestartSLAOnReactivation bool    `json:"restart_sla_on_reactivation"`
}

// newSLAConfigurationRequest validates an SLA configuration and returns its
// writable fields.
func newSLAConfigurationRequest(sla *SLAConfiguration) (*slaConfigurationRequest, error) {
	if sla == nil {
		return nil, errors.New("defectdojo: SLA configuration must not be nil")
	}
	if sla.Name == "" {
		return nil, errors.New("defectdojo: SLA configuration Name must not be empty")
	}
	if err := checkSLADays(&sla.Critical, &sla.High, &sla.Medium, &sla.Low); err != nil {
		return nil, err
	}
	return &slaConfigurationRequest{
		Name:                     sla.Name,
		Description:              orNull(sla.Description),
		Critical:                 sla.Critical,
		EnforceCritical:          sla.EnforceCritical,
		High:                     sla.High,
		EnforceHigh:              sla.EnforceHigh,
		Medium:                   sla.Medium,
		EnforceMedium:            sla.EnforceMedium,
		Low:                      sla.Low,
		EnforceLow:               sla.EnforceLow,
		RestartSLAOnReactivation: sla.RestartSLAOnReactivation,
	}, nil
}

// checkSLADays returns an error if one of the days that are set (not nil) is
// below 1, in the order critical, high, medium, low.
func checkSLADays(critical, high, medium, low *int) error {
	for _, d := range []struct {
		name string
		days *int
	}{{"Critical", critical}, {"High", high}, {"Medium", medium}, {"Low", low}} {
		if d.days != nil && *d.days < 1 {
			return fmt.Errorf("defectdojo: SLA configuration %s must be at least 1 day, got %d", d.name, *d.days)
		}
	}
	return nil
}

// SLAConfigurationPatch is the body of [SLAConfigurationsService.Patch]. Only
// the fields that are set are changed.
type SLAConfigurationPatch struct {
	Name                     *string          `json:"name,omitempty"`
	Description              Nullable[string] `json:"description,omitzero"`
	Critical                 *int             `json:"critical,omitempty"`
	EnforceCritical          *bool            `json:"enforce_critical,omitempty"`
	High                     *int             `json:"high,omitempty"`
	EnforceHigh              *bool            `json:"enforce_high,omitempty"`
	Medium                   *int             `json:"medium,omitempty"`
	EnforceMedium            *bool            `json:"enforce_medium,omitempty"`
	Low                      *int             `json:"low,omitempty"`
	EnforceLow               *bool            `json:"enforce_low,omitempty"`
	RestartSLAOnReactivation *bool            `json:"restart_sla_on_reactivation,omitempty"`
}

// ListSLAConfigurationsOptions are the options of
// [SLAConfigurationsService.List] and [SLAConfigurationsService.ListAll].
// DefectDojo has no filters for SLA configurations.
type ListSLAConfigurationsOptions struct {
	ListOptions
}

// ListSLAConfigurationsResult is a page of SLA configurations.
type ListSLAConfigurationsResult struct {
	Results []*SLAConfiguration

	// Count is the number of SLA configurations.
	Count int
}

// List returns a page of SLA configurations, ordered by ID.
//
// DefectDojo API: GET /api/v2/sla_configurations/
func (s *SLAConfigurationsService) List(ctx context.Context, opt *ListSLAConfigurationsOptions, opts ...RequestOption) (*ListSLAConfigurationsResult, *Response, error) {
	path, err := addQuery("/api/v2/sla_configurations/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*SLAConfiguration]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListSLAConfigurationsResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the SLA configurations, starting at
// opt.Offset. It fetches opt.Limit SLA configurations per request (100 if
// zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *SLAConfigurationsService) ListAll(ctx context.Context, opt *ListSLAConfigurationsOptions, opts ...RequestOption) iter.Seq2[*SLAConfiguration, error] {
	base := ListSLAConfigurationsOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*SLAConfiguration, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return result.Results, result.Count, nil
	})
}

// GetSLAConfigurationOptions are the options of [SLAConfigurationsService.Get].
type GetSLAConfigurationOptions struct{}

// Get returns an SLA configuration by its ID.
//
// DefectDojo API: GET /api/v2/sla_configurations/{id}/
func (s *SLAConfigurationsService) Get(ctx context.Context, id int, opt *GetSLAConfigurationOptions, opts ...RequestOption) (*SLAConfiguration, *Response, error) {
	if err := requirePositive("SLA configuration ID", id); err != nil {
		return nil, nil, err
	}
	sla := new(SLAConfiguration)
	resp, err := s.client.do(ctx, http.MethodGet, slaConfigurationPath(id), nil, sla, opts)
	if err != nil {
		return nil, resp, err
	}
	return sla, resp, nil
}

// GetSLAConfigurationByNameOptions are the options of [SLAConfigurationsService.GetByName].
type GetSLAConfigurationByNameOptions struct{}

// GetByName returns the SLA configuration with exactly the given name,
// compared case-sensitively. It returns an error matching [ErrNotFound] when
// there is none.
//
// DefectDojo cannot filter SLA configurations by name, so GetByName pages
// through all of them; the returned response is the last page's.
func (s *SLAConfigurationsService) GetByName(ctx context.Context, name string, opt *GetSLAConfigurationByNameOptions, opts ...RequestOption) (*SLAConfiguration, *Response, error) {
	if name == "" {
		return nil, nil, errors.New("defectdojo: SLA configuration name must not be empty")
	}
	listOpt := &ListSLAConfigurationsOptions{ListOptions: ListOptions{Limit: 100}}
	for {
		list, resp, err := s.List(ctx, listOpt, opts...)
		if err != nil {
			return nil, resp, err
		}
		for _, sla := range list.Results {
			if sla.Name == name {
				return sla, resp, nil
			}
		}
		listOpt.Offset += len(list.Results)
		if len(list.Results) == 0 || listOpt.Offset >= list.Count {
			return nil, resp, fmt.Errorf("defectdojo: SLA configuration %q: %w", name, ErrNotFound)
		}
	}
}

// CreateSLAConfigurationOptions are the options of [SLAConfigurationsService.Create].
type CreateSLAConfigurationOptions struct{}

// Create creates an SLA configuration. A name that is taken is a 400
// ([ErrBadRequest]) with a "name" field error.
//
// Every writable field is sent, so the Enforce fields, which default to true
// in DefectDojo, are false unless they are set.
//
// DefectDojo API: POST /api/v2/sla_configurations/
func (s *SLAConfigurationsService) Create(ctx context.Context, sla *SLAConfiguration, opt *CreateSLAConfigurationOptions, opts ...RequestOption) (*SLAConfiguration, *Response, error) {
	body, err := newSLAConfigurationRequest(sla)
	if err != nil {
		return nil, nil, err
	}
	created := new(SLAConfiguration)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/sla_configurations/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateSLAConfigurationOptions are the options of [SLAConfigurationsService.Update].
type UpdateSLAConfigurationOptions struct{}

// Update replaces the writable fields of the SLA configuration with the given
// ID. Every writable field is sent: an empty Description clears it. Update an
// SLA configuration returned by [SLAConfigurationsService.Get] to keep its
// other fields, or use [SLAConfigurationsService.Patch].
//
// A change of the days or Enforce fields while DefectDojo recalculates the
// SLA dates of an earlier change is a 400; see [IsSLARecalculating].
//
// Days below 1 are refused before sending, also when they come from
// DefectDojo, whose API accepts them: change such a configuration with
// [SLAConfigurationsService.Patch].
//
// DefectDojo API: PUT /api/v2/sla_configurations/{id}/
func (s *SLAConfigurationsService) Update(ctx context.Context, id int, sla *SLAConfiguration, opt *UpdateSLAConfigurationOptions, opts ...RequestOption) (*SLAConfiguration, *Response, error) {
	if err := requirePositive("SLA configuration ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newSLAConfigurationRequest(sla)
	if err != nil {
		return nil, nil, err
	}
	updated := new(SLAConfiguration)
	resp, err := s.client.do(ctx, http.MethodPut, slaConfigurationPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// PatchSLAConfigurationOptions are the options of [SLAConfigurationsService.Patch].
type PatchSLAConfigurationOptions struct{}

// Patch changes the fields of the SLA configuration that are set in patch.
// Days below 1 are rejected before sending.
//
// A change of the days or Enforce fields while DefectDojo recalculates the
// SLA dates of an earlier change is a 400; see [IsSLARecalculating].
//
// DefectDojo API: PATCH /api/v2/sla_configurations/{id}/
func (s *SLAConfigurationsService) Patch(ctx context.Context, id int, patch *SLAConfigurationPatch, opt *PatchSLAConfigurationOptions, opts ...RequestOption) (*SLAConfiguration, *Response, error) {
	if err := requirePositive("SLA configuration ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: SLA configuration patch must not be nil")
	}
	if err := checkSLADays(patch.Critical, patch.High, patch.Medium, patch.Low); err != nil {
		return nil, nil, err
	}
	updated := new(SLAConfiguration)
	resp, err := s.client.do(ctx, http.MethodPatch, slaConfigurationPath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteSLAConfigurationOptions are the options of [SLAConfigurationsService.Delete].
type DeleteSLAConfigurationOptions struct{}

// Delete deletes an SLA configuration by its ID.
//
// Deleting a configuration that a product uses is a 409 ([ErrConflict]), and
// deleting "Default" (ID 1) is a 400 ([ErrBadRequest]).
//
// DefectDojo API: DELETE /api/v2/sla_configurations/{id}/
func (s *SLAConfigurationsService) Delete(ctx context.Context, id int, opt *DeleteSLAConfigurationOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("SLA configuration ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, slaConfigurationPath(id), nil, nil, opts)
}

// DeletePreviewSLAConfigurationOptions are the options of [SLAConfigurationsService.DeletePreview].
type DeletePreviewSLAConfigurationOptions struct {
	ListOptions
}

// DeletePreview returns a page of the objects that deleting the SLA
// configuration would remove, the configuration itself included.
//
// DefectDojo API: GET /api/v2/sla_configurations/{id}/delete_preview/
func (s *SLAConfigurationsService) DeletePreview(ctx context.Context, id int, opt *DeletePreviewSLAConfigurationOptions, opts ...RequestOption) (*DeletePreviewResult, *Response, error) {
	if err := requirePositive("SLA configuration ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(slaConfigurationPath(id)+"delete_preview/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*DeletedObject]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &DeletePreviewResult{Results: page.Results, Count: page.Count}, resp, nil
}

// IsSLARecalculating reports whether err is DefectDojo's 400 for an SLA change
// made while the SLA expiration dates of an earlier change are still being
// recalculated.
//
// Changing an SLA configuration's days or Enforce fields marks it and every
// product using it as updating; changing a product's SLA configuration marks
// that product and its new SLA configuration. Either queues a Celery task that
// recalculates the findings' SLA expiration dates. Until the task finishes,
// another change of a marked configuration's days or Enforce fields, or of a
// marked product's SLA configuration, is a 400; sending the current values
// again is fine. The task never clears the
// mark when the system setting enable_finding_sla is off, nor without a
// running Celery worker.
func IsSLARecalculating(err error) bool {
	var errResp *ErrorResponse
	if !errors.As(err, &errResp) || errResp.StatusCode != http.StatusBadRequest {
		return false
	}
	// "…are currently being calculated. The SLA days for this SLA
	// configuration…" for SLA configurations, "…are currently being
	// recalculated. The SLA configuration for this product…" for products.
	const phrase = "SLA expiration dates are currently being"
	if strings.Contains(errResp.Message, phrase) {
		return true
	}
	for _, msgs := range errResp.Fields {
		for _, msg := range msgs {
			if strings.Contains(msg, phrase) {
				return true
			}
		}
	}
	return false
}

func slaConfigurationPath(id int) string {
	return "/api/v2/sla_configurations/" + strconv.Itoa(id) + "/"
}

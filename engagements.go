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

// EngagementsService handles communication with the engagement methods of
// the DefectDojo API. Engagements group the tests of a product, such as the
// scans a CI/CD pipeline imports.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type EngagementsService service

// EngagementStatus is the status of an engagement.
type EngagementStatus string

// Engagement statuses, the values of [Engagement.Status].
const (
	EngagementStatusNotStarted         EngagementStatus = "Not Started"
	EngagementStatusBlocked            EngagementStatus = "Blocked"
	EngagementStatusCancelled          EngagementStatus = "Cancelled" //nolint:misspell // DefectDojo's spelling
	EngagementStatusCompleted          EngagementStatus = "Completed"
	EngagementStatusInProgress         EngagementStatus = "In Progress"
	EngagementStatusOnHold             EngagementStatus = "On Hold"
	EngagementStatusScheduled          EngagementStatus = "Scheduled"
	EngagementStatusWaitingForResource EngagementStatus = "Waiting for Resource"
)

// EngagementType is the type of an engagement.
type EngagementType string

// Engagement types, the values of [Engagement.EngagementType].
const (
	EngagementTypeInteractive EngagementType = "Interactive"
	EngagementTypeCICD        EngagementType = "CI/CD"
)

// Engagement is a DefectDojo engagement. The same type is used for reads and
// writes: the read-only fields ID, Active, Progress, TmodelPath,
// DoneTesting, RiskAcceptance, Created, Updated, ArchivedAt and ArchiveBatch
// are ignored when writing.
//
// Nullable fields decode as their zero value when they are null, and
// [EngagementsService.Create] and [EngagementsService.Update] send zero
// values of nullable fields as null.
type Engagement struct {
	_ [0]func()

	ID int `json:"id"`

	// Product is the ID of the engagement's product. Changing it moves the
	// engagement to another product.
	Product int `json:"product"`

	// Name is at most 300 characters and need not be unique. An engagement
	// can't be without a name: DefectDojo names it after its TargetStart,
	// such as "2026-01-05", when it is created or updated with an empty or
	// null name.
	Name string `json:"name"`

	// Description and Reason are at most 2000 characters.
	Description string `json:"description"`
	Reason      string `json:"reason"`

	// Version is the version of the product the engagement tested, at most
	// 100 characters.
	Version string `json:"version"`

	// TargetStart and TargetEnd are required. DefectDojo rejects a
	// TargetStart after TargetEnd only on create. Every import into a CI/CD
	// engagement moves TargetEnd to the scan date when that is later.
	TargetStart Date `json:"target_start"`
	TargetEnd   Date `json:"target_end"`

	FirstContacted Date `json:"first_contacted"`

	// Status is one of the EngagementStatus constants. When it is empty,
	// Create uses EngagementStatusNotStarted and Update keeps the current
	// status. Close and Reopen change it, and so does DefectDojo's automatic
	// closing of engagements.
	Status EngagementStatus `json:"status"`

	// EngagementType is EngagementTypeInteractive or EngagementTypeCICD.
	// When it is empty, Create uses EngagementTypeInteractive and Update
	// keeps the current type.
	EngagementType EngagementType `json:"engagement_type"`

	// Active is false once the engagement is closed. Only
	// [EngagementsService.Close] and [EngagementsService.Reopen] change it.
	Active bool `json:"active"`

	// Tracker, TestStrategy and SourceCodeManagementURI are URLs (at most
	// 200, 200 and 600 characters).
	Tracker                 string `json:"tracker"`
	TestStrategy            string `json:"test_strategy"`
	SourceCodeManagementURI string `json:"source_code_management_uri"`

	// BuildID, CommitHash and BranchTag describe what a CI/CD engagement
	// tested, at most 150 characters each.
	BuildID    string `json:"build_id"`
	CommitHash string `json:"commit_hash"`
	BranchTag  string `json:"branch_tag"`

	// ThreatModel, APITest, PenTest and CheckList default to true in
	// DefectDojo, but Create sends false unless they are set.
	ThreatModel bool `json:"threat_model"`
	APITest     bool `json:"api_test"`
	PenTest     bool `json:"pen_test"`
	CheckList   bool `json:"check_list"`

	// DeduplicationOnEngagement limits the deduplication of the engagement's
	// findings to the engagement, instead of the product.
	DeduplicationOnEngagement bool `json:"deduplication_on_engagement"`

	// Lead is the ID of the user who leads the engagement, Requester the ID
	// of a contact, and Preset and ReportType IDs of engagement presets and
	// report types; 0 when unset.
	Lead       int `json:"lead"`
	Requester  int `json:"requester"`
	Preset     int `json:"preset"`
	ReportType int `json:"report_type"`

	// CICDSCMServer, CICDBuildServer and CICDOrchestrationEngine are IDs of
	// CI/CD infrastructure, or 0 when unset.
	CICDSCMServer           int `json:"cicd_scm_server"`
	CICDBuildServer         int `json:"cicd_build_server"`
	CICDOrchestrationEngine int `json:"cicd_orchestration_engine"`

	// Tags must not contain spaces or quotes (a 400), and commas split a tag.
	// When the product passes its tags on (its EnableProductTagInheritance,
	// or DefectDojo's system setting), they are among Tags: DefectDojo adds
	// them on create and puts them back when a write sends other tags without
	// them, but an empty list removes them too. Create and Update leave nil
	// Tags out (so that a new engagement inherits the product's tags, and
	// Update keeps the current ones) and send a non-nil empty slice as [].
	Tags []string `json:"tags"`

	Progress       string `json:"progress"`
	TmodelPath     string `json:"tmodel_path"`
	DoneTesting    bool   `json:"done_testing"`
	RiskAcceptance []int  `json:"risk_acceptance"`

	Created time.Time `json:"created,omitzero"`
	Updated time.Time `json:"updated,omitzero"`

	// ArchivedAt and ArchiveBatch are set by the archive feature of
	// DefectDojo Pro (3.5+); they are always zero on other servers.
	ArchivedAt   time.Time `json:"archived_at,omitzero"`
	ArchiveBatch string    `json:"archive_batch,omitempty"`
}

// engagementRequest is the body of engagement create and update requests:
// the writable fields of an [Engagement], with zero values of nullable
// fields sent as null. An empty status or engagement type is left out: the
// enums don't allow "". Nil tags are left out, so that a new engagement
// inherits its product's tags and Update keeps the current ones; an empty
// list removes them all, inherited ones included.
type engagementRequest struct {
	Product                   int              `json:"product"`
	Name                      *string          `json:"name"`
	Description               *string          `json:"description"`
	Reason                    *string          `json:"reason"`
	Version                   *string          `json:"version"`
	TargetStart               Date             `json:"target_start"`
	TargetEnd                 Date             `json:"target_end"`
	FirstContacted            Date             `json:"first_contacted"`
	Status                    EngagementStatus `json:"status,omitempty"`
	EngagementType            EngagementType   `json:"engagement_type,omitempty"`
	Tracker                   *string          `json:"tracker"`
	TestStrategy              *string          `json:"test_strategy"`
	SourceCodeManagementURI   *string          `json:"source_code_management_uri"`
	BuildID                   *string          `json:"build_id"`
	CommitHash                *string          `json:"commit_hash"`
	BranchTag                 *string          `json:"branch_tag"`
	ThreatModel               bool             `json:"threat_model"`
	APITest                   bool             `json:"api_test"`
	PenTest                   bool             `json:"pen_test"`
	CheckList                 bool             `json:"check_list"`
	DeduplicationOnEngagement bool             `json:"deduplication_on_engagement"`
	Lead                      *int             `json:"lead"`
	Requester                 *int             `json:"requester"`
	Preset                    *int             `json:"preset"`
	ReportType                *int             `json:"report_type"`
	CICDSCMServer             *int             `json:"cicd_scm_server"`
	CICDBuildServer           *int             `json:"cicd_build_server"`
	CICDOrchestrationEngine   *int             `json:"cicd_orchestration_engine"`
	Tags                      []string         `json:"tags,omitzero"`
}

// newEngagementRequest validates an engagement and returns its writable
// fields.
func newEngagementRequest(e *Engagement) (*engagementRequest, error) {
	switch {
	case e == nil:
		return nil, errors.New("defectdojo: engagement must not be nil")
	case e.Product <= 0:
		return nil, fmt.Errorf("defectdojo: engagement Product must be positive, got %d", e.Product)
	case e.TargetStart.IsZero() || e.TargetEnd.IsZero():
		return nil, errors.New("defectdojo: engagement TargetStart and TargetEnd must be set")
	}
	return &engagementRequest{
		Product:                   e.Product,
		Name:                      orNull(e.Name),
		Description:               orNull(e.Description),
		Reason:                    orNull(e.Reason),
		Version:                   orNull(e.Version),
		TargetStart:               e.TargetStart,
		TargetEnd:                 e.TargetEnd,
		FirstContacted:            e.FirstContacted,
		Status:                    e.Status,
		EngagementType:            e.EngagementType,
		Tracker:                   orNull(e.Tracker),
		TestStrategy:              orNull(e.TestStrategy),
		SourceCodeManagementURI:   orNull(e.SourceCodeManagementURI),
		BuildID:                   orNull(e.BuildID),
		CommitHash:                orNull(e.CommitHash),
		BranchTag:                 orNull(e.BranchTag),
		ThreatModel:               e.ThreatModel,
		APITest:                   e.APITest,
		PenTest:                   e.PenTest,
		CheckList:                 e.CheckList,
		DeduplicationOnEngagement: e.DeduplicationOnEngagement,
		Lead:                      orNull(e.Lead),
		Requester:                 orNull(e.Requester),
		Preset:                    orNull(e.Preset),
		ReportType:                orNull(e.ReportType),
		CICDSCMServer:             orNull(e.CICDSCMServer),
		CICDBuildServer:           orNull(e.CICDBuildServer),
		CICDOrchestrationEngine:   orNull(e.CICDOrchestrationEngine),
		Tags:                      e.Tags,
	}, nil
}

// EngagementPatch is the body of [EngagementsService.Patch]. Only the fields
// that are set are changed.
type EngagementPatch struct {
	_ [0]func()

	Product *int `json:"product,omitempty"`

	// Name can't be cleared: null or "" names the engagement after its
	// target start.
	Name Nullable[string] `json:"name,omitzero"`

	// TargetStart and TargetEnd must not point to the zero Date.
	TargetStart *Date `json:"target_start,omitempty"`
	TargetEnd   *Date `json:"target_end,omitempty"`

	Description               Nullable[string]  `json:"description,omitzero"`
	Reason                    Nullable[string]  `json:"reason,omitzero"`
	Version                   Nullable[string]  `json:"version,omitzero"`
	FirstContacted            Nullable[Date]    `json:"first_contacted,omitzero"`
	Status                    *EngagementStatus `json:"status,omitempty"`
	EngagementType            *EngagementType   `json:"engagement_type,omitempty"`
	Tracker                   Nullable[string]  `json:"tracker,omitzero"`
	TestStrategy              Nullable[string]  `json:"test_strategy,omitzero"`
	SourceCodeManagementURI   Nullable[string]  `json:"source_code_management_uri,omitzero"`
	BuildID                   Nullable[string]  `json:"build_id,omitzero"`
	CommitHash                Nullable[string]  `json:"commit_hash,omitzero"`
	BranchTag                 Nullable[string]  `json:"branch_tag,omitzero"`
	ThreatModel               *bool             `json:"threat_model,omitempty"`
	APITest                   *bool             `json:"api_test,omitempty"`
	PenTest                   *bool             `json:"pen_test,omitempty"`
	CheckList                 *bool             `json:"check_list,omitempty"`
	DeduplicationOnEngagement *bool             `json:"deduplication_on_engagement,omitempty"`
	Lead                      Nullable[int]     `json:"lead,omitzero"`
	Requester                 Nullable[int]     `json:"requester,omitzero"`
	Preset                    Nullable[int]     `json:"preset,omitzero"`
	ReportType                Nullable[int]     `json:"report_type,omitzero"`
	CICDSCMServer             Nullable[int]     `json:"cicd_scm_server,omitzero"`
	CICDBuildServer           Nullable[int]     `json:"cicd_build_server,omitzero"`
	CICDOrchestrationEngine   Nullable[int]     `json:"cicd_orchestration_engine,omitzero"`

	// Tags replaces the tags; a pointer to an empty or nil slice removes
	// them all, even those the product passes on (which a later write of
	// other tags brings back).
	Tags *[]string `json:"tags,omitempty"`
}

// ListEngagementsOptions are the options of [EngagementsService.List] and
// [EngagementsService.ListAll].
type ListEngagementsOptions struct {
	_ [0]func()

	ListOptions

	// Product filters by product ID. A product that doesn't exist is a 400.
	Product int `url:"product,omitempty"`

	// Name filters by exact, case-sensitive name.
	Name string `url:"name,omitempty"`

	// Status filters by one of the EngagementStatus constants.
	Status EngagementStatus `url:"status,omitempty"`

	Active *bool `url:"active,omitempty"`
}

// ListEngagementsResult is a page of engagements.
type ListEngagementsResult struct {
	_ [0]func()

	Results []Engagement

	// Count is the number of engagements that match the filters.
	Count int
}

// List returns a page of engagements.
//
// DefectDojo API: GET /api/v2/engagements/
func (s *EngagementsService) List(ctx context.Context, opt *ListEngagementsOptions, opts ...RequestOption) (*ListEngagementsResult, *Response, error) {
	path, err := addQuery("/api/v2/engagements/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[Engagement]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListEngagementsResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the engagements, starting at opt.Offset.
// It fetches opt.Limit engagements per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *EngagementsService) ListAll(ctx context.Context, opt *ListEngagementsOptions, opts ...RequestOption) iter.Seq2[*Engagement, error] {
	base := ListEngagementsOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*Engagement, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Results), result.Count, nil
	})
}

// GetEngagementOptions are the options of [EngagementsService.Get].
type GetEngagementOptions struct {
	_ [0]func()

	_ [0]func()
}

// Get returns an engagement by its ID.
//
// DefectDojo API: GET /api/v2/engagements/{id}/
func (s *EngagementsService) Get(ctx context.Context, id int, opt *GetEngagementOptions, opts ...RequestOption) (*Engagement, *Response, error) {
	if err := requirePositive("engagement ID", id); err != nil {
		return nil, nil, err
	}
	e := new(Engagement)
	resp, err := s.client.do(ctx, http.MethodGet, engagementPath(id), nil, e, opts)
	if err != nil {
		return nil, resp, err
	}
	return e, resp, nil
}

// CreateEngagementOptions are the options of [EngagementsService.Create].
type CreateEngagementOptions struct {
	_ [0]func()

	_ [0]func()
}

// Create creates an engagement. Its read-only fields are ignored.
//
// Every writable field is sent, so the booleans that default to true in
// DefectDojo (ThreatModel, APITest, PenTest and CheckList) are false unless
// they are set. An empty Status or EngagementType uses DefectDojo's default.
//
// DefectDojo API: POST /api/v2/engagements/
func (s *EngagementsService) Create(ctx context.Context, e *Engagement, opt *CreateEngagementOptions, opts ...RequestOption) (*Engagement, *Response, error) {
	body, err := newEngagementRequest(e)
	if err != nil {
		return nil, nil, err
	}
	created := new(Engagement)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/engagements/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateEngagementOptions are the options of [EngagementsService.Update].
type UpdateEngagementOptions struct {
	_ [0]func()

	_ [0]func()
}

// Update replaces the writable fields of the engagement with the given ID.
// Every writable field is sent except an empty Status and EngagementType,
// and nil Tags, which are kept: empty nullable fields are cleared, and an
// empty non-nil Tags removes all tags. Update an engagement returned by
// [EngagementsService.Get] to keep its other fields, or use
// [EngagementsService.Patch].
//
// DefectDojo API: PUT /api/v2/engagements/{id}/
func (s *EngagementsService) Update(ctx context.Context, id int, e *Engagement, opt *UpdateEngagementOptions, opts ...RequestOption) (*Engagement, *Response, error) {
	if err := requirePositive("engagement ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newEngagementRequest(e)
	if err != nil {
		return nil, nil, err
	}
	updated := new(Engagement)
	resp, err := s.client.do(ctx, http.MethodPut, engagementPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// PatchEngagementOptions are the options of [EngagementsService.Patch].
type PatchEngagementOptions struct {
	_ [0]func()

	_ [0]func()
}

// Patch changes the fields of the engagement that are set in patch.
//
// DefectDojo API: PATCH /api/v2/engagements/{id}/
func (s *EngagementsService) Patch(ctx context.Context, id int, patch *EngagementPatch, opt *PatchEngagementOptions, opts ...RequestOption) (*Engagement, *Response, error) {
	if err := requirePositive("engagement ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: engagement patch must not be nil")
	}
	if patch.TargetStart != nil && patch.TargetStart.IsZero() || patch.TargetEnd != nil && patch.TargetEnd.IsZero() {
		// A zero Date is sent as null, a 400.
		return nil, nil, errors.New("defectdojo: engagement patch TargetStart and TargetEnd must not be zero")
	}
	// null is a 400 for lists; send [] from a copy, leaving the caller's
	// patch as is.
	p := *patch
	p.Tags = orEmptyPtr(p.Tags)
	updated := new(Engagement)
	resp, err := s.client.do(ctx, http.MethodPatch, engagementPath(id), &p, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteEngagementOptions are the options of [EngagementsService.Delete].
type DeleteEngagementOptions struct {
	_ [0]func()

	_ [0]func()
}

// Delete deletes an engagement by its ID.
//
// Deleting an engagement also deletes its tests and their findings. Use
// [EngagementsService.DeletePreview] or [EngagementsService.CountTests] first
// to see what would be deleted.
//
// When the server runs with asynchronous deletes (DD_ASYNC_OBJECT_DELETE),
// DefectDojo answers before the engagement is gone: poll
// [EngagementsService.Get] until it returns [ErrNotFound].
//
// DefectDojo API: DELETE /api/v2/engagements/{id}/
func (s *EngagementsService) Delete(ctx context.Context, id int, opt *DeleteEngagementOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("engagement ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, engagementPath(id), nil, nil, opts)
}

// DeletePreviewEngagementOptions are the options of [EngagementsService.DeletePreview].
type DeletePreviewEngagementOptions struct {
	_ [0]func()

	ListOptions
}

// DeletePreview returns a page of the objects that deleting the engagement
// would remove, the engagement itself included.
//
// DefectDojo API: GET /api/v2/engagements/{id}/delete_preview/
func (s *EngagementsService) DeletePreview(ctx context.Context, id int, opt *DeletePreviewEngagementOptions, opts ...RequestOption) (*DeletePreviewResult, *Response, error) {
	if err := requirePositive("engagement ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(engagementPath(id)+"delete_preview/", opt)
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

// CloseEngagementOptions are the options of [EngagementsService.Close].
type CloseEngagementOptions struct {
	_ [0]func()

	_ [0]func()
}

// Close closes the engagement: its Status becomes EngagementStatusCompleted
// and Active false.
//
// DefectDojo API: POST /api/v2/engagements/{id}/close/
func (s *EngagementsService) Close(ctx context.Context, id int, opt *CloseEngagementOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("engagement ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodPost, engagementPath(id)+"close/", nil, nil, opts)
}

// ReopenEngagementOptions are the options of [EngagementsService.Reopen].
type ReopenEngagementOptions struct {
	_ [0]func()

	_ [0]func()
}

// Reopen reopens the engagement: its Status becomes
// EngagementStatusInProgress and Active true.
//
// DefectDojo API: POST /api/v2/engagements/{id}/reopen/
func (s *EngagementsService) Reopen(ctx context.Context, id int, opt *ReopenEngagementOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("engagement ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodPost, engagementPath(id)+"reopen/", nil, nil, opts)
}

// CountEngagementTestsOptions are the options of [EngagementsService.CountTests].
type CountEngagementTestsOptions struct {
	_ [0]func()

	_ [0]func()
}

// CountTests returns the number of tests of the engagement, which a delete
// of the engagement would also delete. It reads the count of the tests list
// (GET /api/v2/tests/?engagement={id}), whose filter rejects engagements that
// don't exist: the error then also matches [ErrNotFound].
func (s *EngagementsService) CountTests(ctx context.Context, id int, opt *CountEngagementTestsOptions, opts ...RequestOption) (int, *Response, error) {
	if err := requirePositive("engagement ID", id); err != nil {
		return 0, nil, err
	}
	return s.client.count(ctx, "/api/v2/tests/?engagement="+strconv.Itoa(id)+"&limit=1", "engagement", opts)
}

func engagementPath(id int) string {
	return "/api/v2/engagements/" + strconv.Itoa(id) + "/"
}

package defectdojo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
	"time"
)

// JiraProjectsService handles communication with the Jira project methods of
// the DefectDojo API. A Jira project configuration connects a product, or an
// engagement, to a project of a Jira instance ([JiraInstancesService]).
// Managing one needs edit permission on its product or engagement, not a
// superuser. The system setting enable_jira only matters for pushing
// findings, not for managing the configurations.
//
// DefectDojo also serves these endpoints as
// /api/v2/jira_product_configurations/.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type JiraProjectsService service

// JiraProject is a DefectDojo Jira project configuration. The same type is
// used for reads and writes; the read-only fields are ignored when writing.
type JiraProject struct {
	_ [0]func()

	ID int `json:"id"`

	// JiraInstance is the ID of the Jira instance. DefectDojo's API accepts
	// none (0, sent as null), which its UI rejects; the client requires one.
	JiraInstance int `json:"jira_instance"`

	// Product and Engagement are the IDs of the product or the engagement
	// the configuration belongs to; exactly one is set, the other is 0. A
	// product can have several configurations, of which DefectDojo uses only
	// one; an engagement can have one. Deleting the product or engagement
	// deletes the configuration.
	Product    int `json:"product"`
	Engagement int `json:"engagement"`

	// ProjectKey is the key of the Jira project (at most 200 characters).
	ProjectKey string `json:"project_key"`

	// IssueTemplateDir is the folder of the Django templates that render
	// issue descriptions, or "" for the instance's.
	IssueTemplateDir string `json:"issue_template_dir"`

	// Component is the Jira component of new issues, or several separated by
	// commas (at most 200 characters).
	Component string `json:"component"`

	// CustomFields maps Jira field IDs to the values of new issues, such as
	// {"customfield_10122": [{"name": "8.0.1"}]}. CloseTransitionFields and
	// ReopenTransitionFields are the fields sent with the close and reopen
	// transitions, such as {"resolution": {"name": "Won't Fix"}}. They are
	// JSON, nil when unset.
	CustomFields           json.RawMessage `json:"custom_fields"`
	CloseTransitionFields  json.RawMessage `json:"close_transition_fields"`
	ReopenTransitionFields json.RawMessage `json:"reopen_transition_fields"`

	// DefaultAssignee is the Jira user that new issues are assigned to, or ""
	// for Jira's default (at most 200 characters).
	DefaultAssignee string `json:"default_assignee"`

	// JiraLabels are the labels of new issues, separated by spaces (at most
	// 200 characters).
	JiraLabels                    string `json:"jira_labels"`
	AddVulnerabilityIDToJiraLabel bool   `json:"add_vulnerability_id_to_jira_label"`

	// PushAllIssues pushes every finding to Jira automatically: its verified
	// findings, if the system setting enforce_verified_status is on.
	PushAllIssues bool `json:"push_all_issues"`

	// EnableEngagementEpicMapping creates a Jira epic for each engagement, of
	// the issue type EpicIssueTypeName ("Epic" by default; the client leaves
	// it out when it is empty, so DefectDojo's default applies on create).
	EnableEngagementEpicMapping bool   `json:"enable_engagement_epic_mapping"`
	EpicIssueTypeName           string `json:"epic_issue_type_name"`

	PushNotes bool `json:"push_notes"`

	// ProductJiraSLANotification posts SLA notifications as comments even
	// when the Jira instance's GlobalJiraSLANotification is off, and
	// SuppressSLANotificationComments never posts them; they can't both be
	// set. SuppressSLANotificationComments is new in DefectDojo 3.5.103 and
	// nil on older servers, which ignore it when writing.
	ProductJiraSLANotification      bool  `json:"product_jira_sla_notification"`
	SuppressSLANotificationComments *bool `json:"suppress_sla_notification_comments"`

	RiskAcceptanceExpirationNotification bool `json:"risk_acceptance_expiration_notification"`

	// Enabled pushes findings to Jira; disabling stops it, also for findings
	// pushed before.
	Enabled bool `json:"enabled"`

	// ArchivedAt and ArchiveBatch are set by DefectDojo Pro's archiving (from
	// DefectDojo 3.5), and are always zero in the open-source edition. They
	// are read-only.
	ArchivedAt   time.Time `json:"archived_at,omitzero"`
	ArchiveBatch string    `json:"archive_batch,omitempty"`
}

// UnmarshalJSON decodes a Jira project, with the JSON fields nil, rather than
// the JSON null, when they are null.
func (p *JiraProject) UnmarshalJSON(data []byte) error {
	type plain JiraProject
	if err := json.Unmarshal(data, (*plain)(p)); err != nil {
		return err
	}
	for _, f := range []*json.RawMessage{&p.CustomFields, &p.CloseTransitionFields, &p.ReopenTransitionFields} {
		if bytes.Equal(*f, []byte("null")) {
			*f = nil
		}
	}
	return nil
}

// jiraProjectRequest is the body of Jira project create and update requests:
// the writable fields of a [JiraProject].
type jiraProjectRequest struct {
	JiraInstance                         int             `json:"jira_instance"`
	Product                              *int            `json:"product"`
	Engagement                           *int            `json:"engagement"`
	ProjectKey                           string          `json:"project_key"`
	IssueTemplateDir                     *string         `json:"issue_template_dir"`
	Component                            string          `json:"component"`
	CustomFields                         json.RawMessage `json:"custom_fields"`
	CloseTransitionFields                json.RawMessage `json:"close_transition_fields"`
	ReopenTransitionFields               json.RawMessage `json:"reopen_transition_fields"`
	DefaultAssignee                      *string         `json:"default_assignee"`
	JiraLabels                           *string         `json:"jira_labels"`
	AddVulnerabilityIDToJiraLabel        bool            `json:"add_vulnerability_id_to_jira_label"`
	PushAllIssues                        bool            `json:"push_all_issues"`
	EnableEngagementEpicMapping          bool            `json:"enable_engagement_epic_mapping"`
	EpicIssueTypeName                    string          `json:"epic_issue_type_name,omitempty"`
	PushNotes                            bool            `json:"push_notes"`
	ProductJiraSLANotification           bool            `json:"product_jira_sla_notification"`
	SuppressSLANotificationComments      *bool           `json:"suppress_sla_notification_comments,omitempty"`
	RiskAcceptanceExpirationNotification bool            `json:"risk_acceptance_expiration_notification"`
	Enabled                              bool            `json:"enabled"`
}

// newJiraProjectRequest validates a Jira project and returns its writable
// fields.
func newJiraProjectRequest(p *JiraProject) (*jiraProjectRequest, error) {
	if p == nil {
		return nil, errors.New("defectdojo: Jira project must not be nil")
	}
	if p.JiraInstance == 0 {
		return nil, errors.New("defectdojo: Jira project JiraInstance must be set")
	}
	if (p.Product == 0) == (p.Engagement == 0) {
		return nil, errors.New("defectdojo: Jira project must have either a Product or an Engagement")
	}
	if p.ProductJiraSLANotification && p.SuppressSLANotificationComments != nil && *p.SuppressSLANotificationComments {
		return nil, errors.New("defectdojo: Jira project ProductJiraSLANotification and SuppressSLANotificationComments can't both be set")
	}
	if err := checkJiraProjectJSON(&p.CustomFields, &p.CloseTransitionFields, &p.ReopenTransitionFields); err != nil {
		return nil, err
	}
	return &jiraProjectRequest{
		JiraInstance:                         p.JiraInstance,
		Product:                              orNull(p.Product),
		Engagement:                           orNull(p.Engagement),
		ProjectKey:                           p.ProjectKey,
		IssueTemplateDir:                     orNull(p.IssueTemplateDir),
		Component:                            p.Component,
		CustomFields:                         nilIfEmpty(p.CustomFields),
		CloseTransitionFields:                nilIfEmpty(p.CloseTransitionFields),
		ReopenTransitionFields:               nilIfEmpty(p.ReopenTransitionFields),
		DefaultAssignee:                      orNull(p.DefaultAssignee),
		JiraLabels:                           orNull(p.JiraLabels),
		AddVulnerabilityIDToJiraLabel:        p.AddVulnerabilityIDToJiraLabel,
		PushAllIssues:                        p.PushAllIssues,
		EnableEngagementEpicMapping:          p.EnableEngagementEpicMapping,
		EpicIssueTypeName:                    p.EpicIssueTypeName,
		PushNotes:                            p.PushNotes,
		ProductJiraSLANotification:           p.ProductJiraSLANotification,
		SuppressSLANotificationComments:      p.SuppressSLANotificationComments,
		RiskAcceptanceExpirationNotification: p.RiskAcceptanceExpirationNotification,
		Enabled:                              p.Enabled,
	}, nil
}

// nilIfEmpty returns nil for an empty JSON value, which is sent as null like
// a nil one, rather than failing the request's encoding.
func nilIfEmpty(v json.RawMessage) json.RawMessage {
	if len(v) == 0 {
		return nil
	}
	return v
}

// checkJiraProjectJSON returns an error if one of the JSON fields that are set
// isn't valid JSON, which would fail the request's encoding. Empty values are
// sent as null.
func checkJiraProjectJSON(fields ...*json.RawMessage) error {
	names := []string{"CustomFields", "CloseTransitionFields", "ReopenTransitionFields"}
	for i, f := range fields {
		if f != nil && len(*f) > 0 && !json.Valid(*f) {
			return fmt.Errorf("defectdojo: Jira project %s is not valid JSON", names[i])
		}
	}
	return nil
}

// JiraProjectPatch is the body of [JiraProjectsService.Patch]. Only the fields
// that are set are changed.
//
// A configuration belongs to a product or an engagement: to move it from one
// to the other, set the new one and [Null] the old one. The JSON fields are
// set with [Value] (of a [json.RawMessage]) or cleared with [Null].
type JiraProjectPatch struct {
	_ [0]func()

	JiraInstance                         *int                      `json:"jira_instance,omitempty"`
	Product                              Nullable[int]             `json:"product,omitzero"`
	Engagement                           Nullable[int]             `json:"engagement,omitzero"`
	ProjectKey                           *string                   `json:"project_key,omitempty"`
	IssueTemplateDir                     Nullable[string]          `json:"issue_template_dir,omitzero"`
	Component                            *string                   `json:"component,omitempty"`
	CustomFields                         Nullable[json.RawMessage] `json:"custom_fields,omitzero"`
	CloseTransitionFields                Nullable[json.RawMessage] `json:"close_transition_fields,omitzero"`
	ReopenTransitionFields               Nullable[json.RawMessage] `json:"reopen_transition_fields,omitzero"`
	DefaultAssignee                      Nullable[string]          `json:"default_assignee,omitzero"`
	JiraLabels                           Nullable[string]          `json:"jira_labels,omitzero"`
	AddVulnerabilityIDToJiraLabel        *bool                     `json:"add_vulnerability_id_to_jira_label,omitempty"`
	PushAllIssues                        *bool                     `json:"push_all_issues,omitempty"`
	EnableEngagementEpicMapping          *bool                     `json:"enable_engagement_epic_mapping,omitempty"`
	EpicIssueTypeName                    *string                   `json:"epic_issue_type_name,omitempty"`
	PushNotes                            *bool                     `json:"push_notes,omitempty"`
	ProductJiraSLANotification           *bool                     `json:"product_jira_sla_notification,omitempty"`
	SuppressSLANotificationComments      *bool                     `json:"suppress_sla_notification_comments,omitempty"`
	RiskAcceptanceExpirationNotification *bool                     `json:"risk_acceptance_expiration_notification,omitempty"`
	Enabled                              *bool                     `json:"enabled,omitempty"`
}

// ListJiraProjectsOptions are the options of [JiraProjectsService.List] and
// [JiraProjectsService.ListAll]. All filters are exact; a JiraInstance,
// Product or Engagement that doesn't exist is a 400.
type ListJiraProjectsOptions struct {
	_ [0]func()

	ListOptions

	JiraInstance                int    `url:"jira_instance,omitempty"`
	Product                     int    `url:"product,omitempty"`
	Engagement                  int    `url:"engagement,omitempty"`
	ProjectKey                  string `url:"project_key,omitempty"`
	Component                   string `url:"component,omitempty"`
	Enabled                     *bool  `url:"enabled,omitempty"`
	PushAllIssues               *bool  `url:"push_all_issues,omitempty"`
	EnableEngagementEpicMapping *bool  `url:"enable_engagement_epic_mapping,omitempty"`
	PushNotes                   *bool  `url:"push_notes,omitempty"`
}

// ListJiraProjectsResult is a page of Jira projects.
type ListJiraProjectsResult struct {
	_ [0]func()

	Results []JiraProject

	// Count is the number of Jira projects that match the filters.
	Count int
}

// List returns a page of the Jira projects the caller may view.
//
// DefectDojo API: GET /api/v2/jira_projects/
func (s *JiraProjectsService) List(ctx context.Context, opt *ListJiraProjectsOptions, opts ...RequestOption) (*ListJiraProjectsResult, *Response, error) {
	path, err := addQuery("/api/v2/jira_projects/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[JiraProject]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListJiraProjectsResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the Jira projects the caller may view,
// starting at opt.Offset. It fetches opt.Limit Jira projects per request (100
// if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *JiraProjectsService) ListAll(ctx context.Context, opt *ListJiraProjectsOptions, opts ...RequestOption) iter.Seq2[*JiraProject, error] {
	base := ListJiraProjectsOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*JiraProject, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Results), result.Count, nil
	})
}

// GetJiraProjectOptions are the options of [JiraProjectsService.Get].
type GetJiraProjectOptions struct {
	_ [0]func()

	_ [0]func()
}

// Get returns a Jira project by its ID.
//
// DefectDojo API: GET /api/v2/jira_projects/{id}/
func (s *JiraProjectsService) Get(ctx context.Context, id int, opt *GetJiraProjectOptions, opts ...RequestOption) (*JiraProject, *Response, error) {
	if err := requirePositive("Jira project ID", id); err != nil {
		return nil, nil, err
	}
	p := new(JiraProject)
	resp, err := s.client.do(ctx, http.MethodGet, jiraProjectPath(id), nil, p, opts)
	if err != nil {
		return nil, resp, err
	}
	return p, resp, nil
}

// CreateJiraProjectOptions are the options of [JiraProjectsService.Create].
type CreateJiraProjectOptions struct {
	_ [0]func()

	_ [0]func()
}

// Create creates a Jira project configuration for a product or an engagement.
//
// A product can get several configurations, of which DefectDojo uses only
// one, so check for an existing one with [JiraProjectsService.List]; a second
// configuration of an engagement is a 400. A product or engagement that
// doesn't exist is a 404 ([ErrNotFound]).
//
// Every writable field is sent, so Enabled, which DefectDojo defaults to true,
// is false unless it is set. An empty EpicIssueTypeName and a nil
// SuppressSLANotificationComments are left out.
//
// DefectDojo API: POST /api/v2/jira_projects/
func (s *JiraProjectsService) Create(ctx context.Context, p *JiraProject, opt *CreateJiraProjectOptions, opts ...RequestOption) (*JiraProject, *Response, error) {
	body, err := newJiraProjectRequest(p)
	if err != nil {
		return nil, nil, err
	}
	created := new(JiraProject)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/jira_projects/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateJiraProjectOptions are the options of [JiraProjectsService.Update].
type UpdateJiraProjectOptions struct {
	_ [0]func()

	_ [0]func()
}

// Update replaces the writable fields of the Jira project with the given ID.
// Every writable field is sent, except an empty EpicIssueTypeName and a nil
// SuppressSLANotificationComments, which keep their values.
//
// Update needs a JiraInstance, as Create does, so a configuration that another
// API client created without one can only be changed with
// [JiraProjectsService.Patch].
//
// DefectDojo API: PUT /api/v2/jira_projects/{id}/
func (s *JiraProjectsService) Update(ctx context.Context, id int, p *JiraProject, opt *UpdateJiraProjectOptions, opts ...RequestOption) (*JiraProject, *Response, error) {
	if err := requirePositive("Jira project ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newJiraProjectRequest(p)
	if err != nil {
		return nil, nil, err
	}
	updated := new(JiraProject)
	resp, err := s.client.do(ctx, http.MethodPut, jiraProjectPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// PatchJiraProjectOptions are the options of [JiraProjectsService.Patch].
type PatchJiraProjectOptions struct {
	_ [0]func()

	_ [0]func()
}

// Patch changes the fields of the Jira project that are set in patch.
// DefectDojo checks the result: it must still have either a product or an
// engagement, and not both SLA notification settings.
//
// DefectDojo API: PATCH /api/v2/jira_projects/{id}/
func (s *JiraProjectsService) Patch(ctx context.Context, id int, patch *JiraProjectPatch, opt *PatchJiraProjectOptions, opts ...RequestOption) (*JiraProject, *Response, error) {
	if err := requirePositive("Jira project ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: Jira project patch must not be nil")
	}
	for name, f := range map[string]Nullable[json.RawMessage]{
		"CustomFields":           patch.CustomFields,
		"CloseTransitionFields":  patch.CloseTransitionFields,
		"ReopenTransitionFields": patch.ReopenTransitionFields,
	} {
		if v, ok := f.Get(); ok && !json.Valid(v) {
			return nil, nil, fmt.Errorf("defectdojo: Jira project %s is not valid JSON", name)
		}
	}
	updated := new(JiraProject)
	resp, err := s.client.do(ctx, http.MethodPatch, jiraProjectPath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteJiraProjectOptions are the options of [JiraProjectsService.Delete].
type DeleteJiraProjectOptions struct {
	_ [0]func()

	_ [0]func()
}

// Delete deletes a Jira project configuration by its ID. It also deletes the
// links between findings and Jira issues made through it.
//
// DefectDojo API: DELETE /api/v2/jira_projects/{id}/
func (s *JiraProjectsService) Delete(ctx context.Context, id int, opt *DeleteJiraProjectOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("Jira project ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, jiraProjectPath(id), nil, nil, opts)
}

// DeletePreviewJiraProjectOptions are the options of [JiraProjectsService.DeletePreview].
type DeletePreviewJiraProjectOptions struct {
	_ [0]func()

	ListOptions
}

// DeletePreview returns a page of the objects that deleting the Jira project
// would remove: the configuration and its links to Jira issues.
//
// DefectDojo API: GET /api/v2/jira_projects/{id}/delete_preview/
func (s *JiraProjectsService) DeletePreview(ctx context.Context, id int, opt *DeletePreviewJiraProjectOptions, opts ...RequestOption) (*DeletePreviewResult, *Response, error) {
	if err := requirePositive("Jira project ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(jiraProjectPath(id)+"delete_preview/", opt)
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

func jiraProjectPath(id int) string {
	return "/api/v2/jira_projects/" + strconv.Itoa(id) + "/"
}

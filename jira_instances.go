package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// JiraInstancesService handles communication with the Jira instance methods
// of the DefectDojo API. A Jira instance is the connection to a Jira server
// that Jira projects ([JiraProjectsService]) push findings to. Managing them
// needs a superuser, or the configuration permissions for Jira instances,
// which DefectDojo lists only while the system setting enable_jira is on.
//
// DefectDojo doesn't contact Jira when an instance is created or changed. It
// also serves these endpoints as /api/v2/jira_configurations/.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type JiraInstancesService service

// JiraIssueType is a Jira issue type DefectDojo can create.
type JiraIssueType string

// The issue types DefectDojo can create for findings.
const (
	JiraIssueTypeTask     JiraIssueType = "Task"
	JiraIssueTypeStory    JiraIssueType = "Story"
	JiraIssueTypeEpic     JiraIssueType = "Epic"
	JiraIssueTypeSpike    JiraIssueType = "Spike"
	JiraIssueTypeBug      JiraIssueType = "Bug"
	JiraIssueTypeSecurity JiraIssueType = "Security"
)

// JiraInstance is a DefectDojo Jira instance. The same type is used for reads
// and writes; the read-only ID is ignored when writing. Its password is
// write-only: see [CreateJiraInstanceRequest].
type JiraInstance struct {
	_ [0]func()

	ID int `json:"id"`

	// ConfigurationName names the instance (at most 2000 characters). It
	// isn't unique, and it can't be "": DefectDojo stores "" only for an
	// instance created without a name, which the client does when it is
	// empty.
	ConfigurationName string `json:"configuration_name"`

	// URL is the Jira server's URL (at most 2000 characters), and Username
	// the user or email address of its account (at most 2000).
	URL      string `json:"url"`
	Username string `json:"username"`

	// DefaultIssueType is the type of new Jira issues, "Bug" by default.
	// DefectDojo offers "Task", "Story", "Epic", "Spike", "Bug" and
	// "Security", and DD_JIRA_EXTRA_ISSUE_TYPES adds more. The client leaves
	// it out when it is empty, so DefectDojo's default applies on create.
	DefaultIssueType JiraIssueType `json:"default_issue_type"`

	// IssueTemplateDir is the folder of the Django templates that render
	// issue descriptions, or "" for the default.
	IssueTemplateDir string `json:"issue_template_dir"`

	// EpicNameID is the number of Jira's Epic Name custom field
	// (customfield_<number>). OpenStatusKey and CloseStatusKey are the IDs
	// of the Jira transitions that reopen and close issues.
	EpicNameID     int `json:"epic_name_id"`
	OpenStatusKey  int `json:"open_status_key"`
	CloseStatusKey int `json:"close_status_key"`

	// The Jira priorities of findings of each severity (at most 200
	// characters each). None can be empty.
	InfoMappingSeverity     string `json:"info_mapping_severity"`
	LowMappingSeverity      string `json:"low_mapping_severity"`
	MediumMappingSeverity   string `json:"medium_mapping_severity"`
	HighMappingSeverity     string `json:"high_mapping_severity"`
	CriticalMappingSeverity string `json:"critical_mapping_severity"`

	// FindingText is added to the issues of findings.
	FindingText string `json:"finding_text"`

	// AcceptedMappingResolution and FalsePositiveMappingResolution are
	// comma-separated Jira resolutions (at most 300 characters) that make
	// the finding of a closed issue risk accepted or a false positive.
	AcceptedMappingResolution      string `json:"accepted_mapping_resolution"`
	FalsePositiveMappingResolution string `json:"false_positive_mapping_resolution"`

	// GlobalJiraSLANotification posts SLA notifications as comments on
	// issues; Jira projects can override it. FindingJiraSync pushes changes
	// to findings to their issues.
	GlobalJiraSLANotification bool `json:"global_jira_sla_notification"`
	FindingJiraSync           bool `json:"finding_jira_sync"`
}

// CreateJiraInstanceRequest is the body of [JiraInstancesService.Create] and
// [JiraInstancesService.Update]: a Jira instance and the password or API token
// of its account, which DefectDojo never returns.
type CreateJiraInstanceRequest struct {
	_ [0]func()

	JiraInstance

	// Password is the password or API token of the Jira account (at most
	// 2000 characters). It is required and can't be blank; DefectDojo stores
	// it without surrounding whitespace.
	Password string `json:"-"`
}

// jiraInstanceRequest is the body of Jira instance create and update
// requests: the writable fields of a [JiraInstance] and the password.
type jiraInstanceRequest struct {
	// ConfigurationName and DefaultIssueType are left out when empty:
	// DefectDojo rejects "" and has defaults ("" and "Bug").
	ConfigurationName              string        `json:"configuration_name,omitempty"`
	URL                            string        `json:"url"`
	Username                       string        `json:"username"`
	Password                       string        `json:"password"`
	DefaultIssueType               JiraIssueType `json:"default_issue_type,omitempty"`
	IssueTemplateDir               *string       `json:"issue_template_dir"`
	EpicNameID                     int           `json:"epic_name_id"`
	OpenStatusKey                  int           `json:"open_status_key"`
	CloseStatusKey                 int           `json:"close_status_key"`
	InfoMappingSeverity            string        `json:"info_mapping_severity"`
	LowMappingSeverity             string        `json:"low_mapping_severity"`
	MediumMappingSeverity          string        `json:"medium_mapping_severity"`
	HighMappingSeverity            string        `json:"high_mapping_severity"`
	CriticalMappingSeverity        string        `json:"critical_mapping_severity"`
	FindingText                    *string       `json:"finding_text"`
	AcceptedMappingResolution      *string       `json:"accepted_mapping_resolution"`
	FalsePositiveMappingResolution *string       `json:"false_positive_mapping_resolution"`
	GlobalJiraSLANotification      bool          `json:"global_jira_sla_notification"`
	FindingJiraSync                bool          `json:"finding_jira_sync"`
}

// newJiraInstanceRequest validates a Jira instance and its password and
// returns the request body.
func newJiraInstanceRequest(ji *CreateJiraInstanceRequest) (*jiraInstanceRequest, error) {
	if ji == nil {
		return nil, errors.New("defectdojo: Jira instance must not be nil")
	}
	for _, f := range []struct{ name, value string }{
		{"URL", ji.URL},
		{"Username", ji.Username},
		{"Password", ji.Password},
		{"InfoMappingSeverity", ji.InfoMappingSeverity},
		{"LowMappingSeverity", ji.LowMappingSeverity},
		{"MediumMappingSeverity", ji.MediumMappingSeverity},
		{"HighMappingSeverity", ji.HighMappingSeverity},
		{"CriticalMappingSeverity", ji.CriticalMappingSeverity},
	} {
		if f.value == "" {
			return nil, fmt.Errorf("defectdojo: Jira instance %s must not be empty", f.name)
		}
	}
	return &jiraInstanceRequest{
		ConfigurationName:              ji.ConfigurationName,
		URL:                            ji.URL,
		Username:                       ji.Username,
		Password:                       ji.Password,
		DefaultIssueType:               ji.DefaultIssueType,
		IssueTemplateDir:               orNull(ji.IssueTemplateDir),
		EpicNameID:                     ji.EpicNameID,
		OpenStatusKey:                  ji.OpenStatusKey,
		CloseStatusKey:                 ji.CloseStatusKey,
		InfoMappingSeverity:            ji.InfoMappingSeverity,
		LowMappingSeverity:             ji.LowMappingSeverity,
		MediumMappingSeverity:          ji.MediumMappingSeverity,
		HighMappingSeverity:            ji.HighMappingSeverity,
		CriticalMappingSeverity:        ji.CriticalMappingSeverity,
		FindingText:                    orNull(ji.FindingText),
		AcceptedMappingResolution:      orNull(ji.AcceptedMappingResolution),
		FalsePositiveMappingResolution: orNull(ji.FalsePositiveMappingResolution),
		GlobalJiraSLANotification:      ji.GlobalJiraSLANotification,
		FindingJiraSync:                ji.FindingJiraSync,
	}, nil
}

// JiraInstancePatch is the body of [JiraInstancesService.Patch]. Only the
// fields that are set are changed.
//
// A change of the URL needs the Password as well, or DefectDojo answers 400
// with a "password" field error; a URL that differs only in a trailing slash
// isn't a change. The client doesn't check this.
type JiraInstancePatch struct {
	_ [0]func()

	ConfigurationName              *string          `json:"configuration_name,omitempty"`
	URL                            *string          `json:"url,omitempty"`
	Username                       *string          `json:"username,omitempty"`
	Password                       *string          `json:"password,omitempty"`
	DefaultIssueType               *JiraIssueType   `json:"default_issue_type,omitempty"`
	IssueTemplateDir               Nullable[string] `json:"issue_template_dir,omitzero"`
	EpicNameID                     *int             `json:"epic_name_id,omitempty"`
	OpenStatusKey                  *int             `json:"open_status_key,omitempty"`
	CloseStatusKey                 *int             `json:"close_status_key,omitempty"`
	InfoMappingSeverity            *string          `json:"info_mapping_severity,omitempty"`
	LowMappingSeverity             *string          `json:"low_mapping_severity,omitempty"`
	MediumMappingSeverity          *string          `json:"medium_mapping_severity,omitempty"`
	HighMappingSeverity            *string          `json:"high_mapping_severity,omitempty"`
	CriticalMappingSeverity        *string          `json:"critical_mapping_severity,omitempty"`
	FindingText                    Nullable[string] `json:"finding_text,omitzero"`
	AcceptedMappingResolution      Nullable[string] `json:"accepted_mapping_resolution,omitzero"`
	FalsePositiveMappingResolution Nullable[string] `json:"false_positive_mapping_resolution,omitzero"`
	GlobalJiraSLANotification      *bool            `json:"global_jira_sla_notification,omitempty"`
	FindingJiraSync                *bool            `json:"finding_jira_sync,omitempty"`
}

// ListJiraInstancesOptions are the options of [JiraInstancesService.List] and
// [JiraInstancesService.ListAll]. DefectDojo can't filter by name.
type ListJiraInstancesOptions struct {
	_ [0]func()

	ListOptions

	// URL is an exact match.
	URL string `url:"url,omitempty"`
}

// ListJiraInstancesResult is a page of Jira instances.
type ListJiraInstancesResult struct {
	_ [0]func()

	Results []JiraInstance

	// Count is the number of Jira instances that match the filters.
	Count int
}

// List returns a page of Jira instances, ordered by ID.
//
// DefectDojo API: GET /api/v2/jira_instances/
func (s *JiraInstancesService) List(ctx context.Context, opt *ListJiraInstancesOptions, opts ...RequestOption) (*ListJiraInstancesResult, *Response, error) {
	path, err := addQuery("/api/v2/jira_instances/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[JiraInstance]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListJiraInstancesResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the Jira instances, starting at
// opt.Offset. It fetches opt.Limit Jira instances per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *JiraInstancesService) ListAll(ctx context.Context, opt *ListJiraInstancesOptions, opts ...RequestOption) iter.Seq2[*JiraInstance, error] {
	base := ListJiraInstancesOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*JiraInstance, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Results), result.Count, nil
	})
}

// GetJiraInstanceOptions are the options of [JiraInstancesService.Get].
type GetJiraInstanceOptions struct {
	_ [0]func()
}

// Get returns a Jira instance by its ID.
//
// DefectDojo API: GET /api/v2/jira_instances/{id}/
func (s *JiraInstancesService) Get(ctx context.Context, id int, opt *GetJiraInstanceOptions, opts ...RequestOption) (*JiraInstance, *Response, error) {
	if err := requirePositive("Jira instance ID", id); err != nil {
		return nil, nil, err
	}
	ji := new(JiraInstance)
	resp, err := s.client.do(ctx, http.MethodGet, jiraInstancePath(id), nil, ji, opts)
	if err != nil {
		return nil, resp, err
	}
	return ji, resp, nil
}

// GetJiraInstanceByNameOptions are the options of [JiraInstancesService.GetByName].
type GetJiraInstanceByNameOptions struct {
	_ [0]func()
}

// GetByName returns the Jira instance whose ConfigurationName is exactly the
// given name, compared case-sensitively. It returns an error matching
// [ErrNotFound] when there is none, and an error matching [ErrAmbiguous] that
// names their IDs when several instances have the name, which DefectDojo
// allows.
//
// DefectDojo can't filter Jira instances by name, so GetByName pages through
// all of them; the returned response is the last page's.
func (s *JiraInstancesService) GetByName(ctx context.Context, name string, opt *GetJiraInstanceByNameOptions, opts ...RequestOption) (*JiraInstance, *Response, error) {
	if name == "" {
		return nil, nil, errors.New("defectdojo: Jira instance name must not be empty")
	}
	var (
		found *JiraInstance
		ids   []int
		resp  *Response
	)
	listOpt := &ListJiraInstancesOptions{ListOptions: ListOptions{Limit: 100}}
	for {
		list, r, err := s.List(ctx, listOpt, opts...)
		resp = r
		if err != nil {
			return nil, resp, err
		}
		for i := range list.Results {
			ji := &list.Results[i]
			if ji.ConfigurationName == name {
				found = ji
				ids = append(ids, ji.ID)
			}
		}
		listOpt.Offset += len(list.Results)
		if len(list.Results) == 0 || listOpt.Offset >= list.Count {
			break
		}
	}
	switch len(ids) {
	case 0:
		return nil, resp, fmt.Errorf("defectdojo: Jira instance %q: %w", name, ErrNotFound)
	case 1:
		return found, resp, nil
	default:
		return nil, resp, fmt.Errorf("%w: %d Jira instances are named %q: IDs %v", ErrAmbiguous, len(ids), name, ids)
	}
}

// CreateJiraInstanceOptions are the options of [JiraInstancesService.Create].
type CreateJiraInstanceOptions struct {
	_ [0]func()
}

// Create creates a Jira instance. DefectDojo doesn't test the connection.
//
// Every writable field is sent, except an empty ConfigurationName or
// DefaultIssueType, which get DefectDojo's defaults. The URL, Username,
// Password and the five severity mappings must not be empty.
//
// DefectDojo API: POST /api/v2/jira_instances/
func (s *JiraInstancesService) Create(ctx context.Context, ji *CreateJiraInstanceRequest, opt *CreateJiraInstanceOptions, opts ...RequestOption) (*JiraInstance, *Response, error) {
	body, err := newJiraInstanceRequest(ji)
	if err != nil {
		return nil, nil, err
	}
	created := new(JiraInstance)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/jira_instances/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateJiraInstanceOptions are the options of [JiraInstancesService.Update].
type UpdateJiraInstanceOptions struct {
	_ [0]func()
}

// Update replaces the writable fields of the Jira instance with the given ID.
// DefectDojo requires the password on every update; to keep it, use
// [JiraInstancesService.Patch]. An empty ConfigurationName or
// DefaultIssueType is left out, which keeps the current value.
//
// DefectDojo API: PUT /api/v2/jira_instances/{id}/
func (s *JiraInstancesService) Update(ctx context.Context, id int, ji *CreateJiraInstanceRequest, opt *UpdateJiraInstanceOptions, opts ...RequestOption) (*JiraInstance, *Response, error) {
	if err := requirePositive("Jira instance ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newJiraInstanceRequest(ji)
	if err != nil {
		return nil, nil, err
	}
	updated := new(JiraInstance)
	resp, err := s.client.do(ctx, http.MethodPut, jiraInstancePath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// PatchJiraInstanceOptions are the options of [JiraInstancesService.Patch].
type PatchJiraInstanceOptions struct {
	_ [0]func()
}

// Patch changes the fields of the Jira instance that are set in patch. A
// patch without a Password keeps the stored one, unless it changes the URL;
// see [JiraInstancePatch].
//
// DefectDojo API: PATCH /api/v2/jira_instances/{id}/
func (s *JiraInstancesService) Patch(ctx context.Context, id int, patch *JiraInstancePatch, opt *PatchJiraInstanceOptions, opts ...RequestOption) (*JiraInstance, *Response, error) {
	if err := requirePositive("Jira instance ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: Jira instance patch must not be nil")
	}
	updated := new(JiraInstance)
	resp, err := s.client.do(ctx, http.MethodPatch, jiraInstancePath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteJiraInstanceOptions are the options of [JiraInstancesService.Delete].
type DeleteJiraInstanceOptions struct {
	_ [0]func()
}

// Delete deletes a Jira instance by its ID.
//
// DefectDojo answers a delete of an instance that Jira projects use with a
// 500 Internal Server Error, not a 409, and [JiraInstancesService.DeletePreview]
// doesn't show them: check [JiraInstancesService.CountJiraProjects] first.
//
// DefectDojo API: DELETE /api/v2/jira_instances/{id}/
func (s *JiraInstancesService) Delete(ctx context.Context, id int, opt *DeleteJiraInstanceOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("Jira instance ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, jiraInstancePath(id), nil, nil, opts)
}

// DeletePreviewJiraInstanceOptions are the options of [JiraInstancesService.DeletePreview].
type DeletePreviewJiraInstanceOptions struct {
	_ [0]func()

	ListOptions
}

// DeletePreview returns a page of the objects that deleting the Jira instance
// would remove, the instance itself included. It doesn't list the Jira
// projects that keep the instance from being deleted; see
// [JiraInstancesService.Delete].
//
// DefectDojo API: GET /api/v2/jira_instances/{id}/delete_preview/
func (s *JiraInstancesService) DeletePreview(ctx context.Context, id int, opt *DeletePreviewJiraInstanceOptions, opts ...RequestOption) (*DeletePreviewResult, *Response, error) {
	if err := requirePositive("Jira instance ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(jiraInstancePath(id)+"delete_preview/", opt)
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

// CountJiraInstanceJiraProjectsOptions are the options of [JiraInstancesService.CountJiraProjects].
type CountJiraInstanceJiraProjectsOptions struct {
	_ [0]func()
}

// CountJiraProjects returns the number of Jira projects that use the Jira
// instance, which keep it from being deleted. It reads the count of the Jira
// projects list (GET /api/v2/jira_projects/?jira_instance={id}), whose filter
// rejects instances that don't exist: the error then also matches
// [ErrNotFound]. The count includes only the Jira projects of products and
// engagements the caller may view.
func (s *JiraInstancesService) CountJiraProjects(ctx context.Context, id int, opt *CountJiraInstanceJiraProjectsOptions, opts ...RequestOption) (int, *Response, error) {
	if err := requirePositive("Jira instance ID", id); err != nil {
		return 0, nil, err
	}
	return s.client.count(ctx, "/api/v2/jira_projects/?jira_instance="+strconv.Itoa(id)+"&limit=1", "jira_instance", opts)
}

func jiraInstancePath(id int) string {
	return "/api/v2/jira_instances/" + strconv.Itoa(id) + "/"
}

package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// ConfigurationPermissionsService handles communication with the
// configuration permission methods of the DefectDojo API. Configuration
// permissions grant a user global rights, such as adding product types or
// changing SLA configurations; [User.ConfigurationPermissions] holds their
// IDs. The endpoints need a superuser.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type ConfigurationPermissionsService service

// ConfigurationPermission is a Django permission that DefectDojo offers as a
// configuration permission.
//
// Its ID is a database ID, which differs between DefectDojo installations:
// look permissions up by Codename. Which permissions exist depends on the
// system settings: the jira_instance permissions only with Jira enabled,
// github_conf only with GitHub enabled, and engagement_survey and question
// only with questionnaires enabled. DefectDojo 3.6 removes the tool_type and
// tool_configuration permissions with their endpoints.
type ConfigurationPermission struct {
	_ [0]func()

	ID int `json:"id"`

	// Name describes the permission, such as "Can add user" (DefectDojo
	// writes some model names oddly, as in "Can add product_ type").
	Name string `json:"name"`

	// Codename identifies the permission, such as "add_product_type".
	Codename string `json:"codename"`
}

// ListConfigurationPermissionsOptions are the options of
// [ConfigurationPermissionsService.List] and
// [ConfigurationPermissionsService.ListAll].
type ListConfigurationPermissionsOptions struct {
	_ [0]func()

	ListOptions

	// Codename and Name are exact, case-sensitive matches.
	Codename string `url:"codename,omitempty"`
	Name     string `url:"name,omitempty"`
}

// ListConfigurationPermissionsResult is a page of configuration permissions.
type ListConfigurationPermissionsResult struct {
	_ [0]func()

	Results []ConfigurationPermission

	// Count is the number of configuration permissions that match the
	// filters.
	Count int
}

// List returns a page of configuration permissions.
//
// DefectDojo API: GET /api/v2/configuration_permissions/
func (s *ConfigurationPermissionsService) List(ctx context.Context, opt *ListConfigurationPermissionsOptions, opts ...RequestOption) (*ListConfigurationPermissionsResult, *Response, error) {
	path, err := addQuery("/api/v2/configuration_permissions/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[ConfigurationPermission]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListConfigurationPermissionsResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the configuration permissions, starting
// at opt.Offset. It fetches opt.Limit permissions per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *ConfigurationPermissionsService) ListAll(ctx context.Context, opt *ListConfigurationPermissionsOptions, opts ...RequestOption) iter.Seq2[*ConfigurationPermission, error] {
	base := ListConfigurationPermissionsOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*ConfigurationPermission, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Results), result.Count, nil
	})
}

// GetConfigurationPermissionOptions are the options of [ConfigurationPermissionsService.Get].
type GetConfigurationPermissionOptions struct {
	_ [0]func()
}

// Get returns a configuration permission by its ID.
//
// DefectDojo API: GET /api/v2/configuration_permissions/{id}/
func (s *ConfigurationPermissionsService) Get(ctx context.Context, id int, opt *GetConfigurationPermissionOptions, opts ...RequestOption) (*ConfigurationPermission, *Response, error) {
	if err := requirePositive("configuration permission ID", id); err != nil {
		return nil, nil, err
	}
	p := new(ConfigurationPermission)
	resp, err := s.client.do(ctx, http.MethodGet, "/api/v2/configuration_permissions/"+strconv.Itoa(id)+"/", nil, p, opts)
	if err != nil {
		return nil, resp, err
	}
	return p, resp, nil
}

// GetConfigurationPermissionByCodenameOptions are the options of [ConfigurationPermissionsService.GetByCodename].
type GetConfigurationPermissionByCodenameOptions struct {
	_ [0]func()
}

// GetByCodename returns the configuration permission with exactly the given
// codename, such as "add_product_type". It returns an error matching
// [ErrNotFound] when there is none, which includes permissions that the
// system settings turn off.
func (s *ConfigurationPermissionsService) GetByCodename(ctx context.Context, codename string, opt *GetConfigurationPermissionByCodenameOptions, opts ...RequestOption) (*ConfigurationPermission, *Response, error) {
	if codename == "" {
		return nil, nil, errors.New("defectdojo: configuration permission codename must not be empty")
	}
	list, resp, err := s.List(ctx, &ListConfigurationPermissionsOptions{Codename: codename}, opts...)
	if err != nil {
		return nil, resp, err
	}
	// DefectDojo strips whitespace from the filter, and may loosen it: check
	// the codename again.
	for i := range list.Results {
		p := &list.Results[i]
		if p.Codename == codename {
			return p, resp, nil
		}
	}
	return nil, resp, fmt.Errorf("defectdojo: configuration permission %q: %w", codename, ErrNotFound)
}

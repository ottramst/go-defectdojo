package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// UsersService handles communication with the user methods of the DefectDojo
// API. Managing users needs a superuser, or the configuration permissions
// for users; only superusers can change staff users, superusers,
// configuration permissions, and the username or email of another user.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type UsersService service

// CreateUserRequest is the body of [UsersService.Create]: a user and its initial
// password.
type CreateUserRequest struct {
	User

	// Password is the user's initial password. It is checked against
	// DefectDojo's password rules (length and character classes, set in the
	// system settings), and stored without surrounding whitespace. DefectDojo
	// requires one unless DD_REQUIRE_PASSWORD_ON_USER is off; without one, the
	// user can only sign in through single sign-on. The API can't change a
	// password later.
	Password string `json:"-"`
}

// userRequest is the body of user create and update requests: the writable
// fields of a [User], and the password on create.
type userRequest struct {
	Username    string `json:"username"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Email       string `json:"email"`
	IsActive    bool   `json:"is_active"`
	IsStaff     bool   `json:"is_staff"`
	IsSuperuser bool   `json:"is_superuser"`
	Password    string `json:"password,omitempty"`

	// ConfigurationPermissions is left out when empty, which keeps the user's
	// permissions: DefectDojo answers an empty list by removing all the
	// user's Django permissions, and a user read with Get has an empty, not
	// a nil, list when it has no configuration permissions.
	ConfigurationPermissions []int `json:"configuration_permissions,omitempty"`
}

// newUserRequest validates a user and returns its writable fields.
func newUserRequest(u *User) (*userRequest, error) {
	if u == nil {
		return nil, errors.New("defectdojo: user must not be nil")
	}
	if u.Username == "" {
		return nil, errors.New("defectdojo: user Username must not be empty")
	}
	if u.Email == "" {
		return nil, errors.New("defectdojo: user Email must not be empty")
	}
	return &userRequest{
		Username:                 u.Username,
		FirstName:                u.FirstName,
		LastName:                 u.LastName,
		Email:                    u.Email,
		IsActive:                 u.IsActive,
		IsStaff:                  u.IsStaff,
		IsSuperuser:              u.IsSuperuser,
		ConfigurationPermissions: u.ConfigurationPermissions,
	}, nil
}

// UserPatch is the body of [UsersService.Patch]. Only the fields that are set
// are changed. The names can't be null: set them to "" to clear them.
type UserPatch struct {
	Username    *string `json:"username,omitempty"`
	FirstName   *string `json:"first_name,omitempty"`
	LastName    *string `json:"last_name,omitempty"`
	Email       *string `json:"email,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
	IsStaff     *bool   `json:"is_staff,omitempty"`
	IsSuperuser *bool   `json:"is_superuser,omitempty"`

	// ConfigurationPermissions replaces the user's configuration
	// permissions. A pointer to an empty or nil slice removes all the user's
	// Django permissions, not only the configuration permissions.
	ConfigurationPermissions *[]int `json:"configuration_permissions,omitempty"`
}

// ListUsersOptions are the options of [UsersService.List] and
// [UsersService.ListAll].
type ListUsersOptions struct {
	ListOptions

	// Username, Email, FirstName and LastName are case-insensitive contains
	// matches.
	Username  string `url:"username,omitempty"`
	Email     string `url:"email,omitempty"`
	FirstName string `url:"first_name,omitempty"`
	LastName  string `url:"last_name,omitempty"`

	IsActive    *bool `url:"is_active,omitempty"`
	IsSuperuser *bool `url:"is_superuser,omitempty"`
}

// ListUsersResult is a page of users.
type ListUsersResult struct {
	Results []User

	// Count is the number of users that match the filters.
	Count int
}

// List returns a page of users, ordered by ID.
//
// DefectDojo API: GET /api/v2/users/
func (s *UsersService) List(ctx context.Context, opt *ListUsersOptions, opts ...RequestOption) (*ListUsersResult, *Response, error) {
	path, err := addQuery("/api/v2/users/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[User]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListUsersResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the users, starting at opt.Offset. It
// fetches opt.Limit users per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *UsersService) ListAll(ctx context.Context, opt *ListUsersOptions, opts ...RequestOption) iter.Seq2[*User, error] {
	base := ListUsersOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*User, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Results), result.Count, nil
	})
}

// GetUserOptions are the options of [UsersService.Get].
type GetUserOptions struct{}

// Get returns a user by its ID.
//
// DefectDojo API: GET /api/v2/users/{id}/
func (s *UsersService) Get(ctx context.Context, id int, opt *GetUserOptions, opts ...RequestOption) (*User, *Response, error) {
	if err := requirePositive("user ID", id); err != nil {
		return nil, nil, err
	}
	u := new(User)
	resp, err := s.client.do(ctx, http.MethodGet, userPath(id), nil, u, opts)
	if err != nil {
		return nil, resp, err
	}
	return u, resp, nil
}

// GetUserByUsernameOptions are the options of [UsersService.GetByUsername].
type GetUserByUsernameOptions struct{}

// GetByUsername returns the user with exactly the given username, compared
// case-sensitively: usernames that differ only in case can coexist. It
// returns an error matching [ErrNotFound] when there is none.
//
// DefectDojo's username filter is a case-insensitive contains match, so
// GetByUsername pages through the users it matches; the returned response is
// the last page's.
func (s *UsersService) GetByUsername(ctx context.Context, username string, opt *GetUserByUsernameOptions, opts ...RequestOption) (*User, *Response, error) {
	if username == "" {
		return nil, nil, errors.New("defectdojo: username must not be empty")
	}
	listOpt := &ListUsersOptions{ListOptions: ListOptions{Limit: 100}, Username: username}
	for {
		list, resp, err := s.List(ctx, listOpt, opts...)
		if err != nil {
			return nil, resp, err
		}
		for i := range list.Results {
			u := &list.Results[i]
			if u.Username == username {
				return u, resp, nil
			}
		}
		listOpt.Offset += len(list.Results)
		if len(list.Results) == 0 || listOpt.Offset >= list.Count {
			return nil, resp, fmt.Errorf("defectdojo: user %q: %w", username, ErrNotFound)
		}
	}
}

// CreateUserOptions are the options of [UsersService.Create].
type CreateUserOptions struct{}

// Create creates a user with the given initial password. Its read-only
// fields are ignored.
//
// Every writable field is sent, so IsActive, which DefectDojo defaults to
// true, is false unless it is set: a zero IsActive creates an inactive user.
// Usernames are unique, compared case-sensitively; emails need not be.
//
// Creating a user also creates its notification settings, copied from the
// template if there is one.
//
// DefectDojo API: POST /api/v2/users/
func (s *UsersService) Create(ctx context.Context, u *CreateUserRequest, opt *CreateUserOptions, opts ...RequestOption) (*User, *Response, error) {
	if u == nil {
		return nil, nil, errors.New("defectdojo: user must not be nil")
	}
	body, err := newUserRequest(&u.User)
	if err != nil {
		return nil, nil, err
	}
	body.Password = u.Password
	created := new(User)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/users/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateUserOptions are the options of [UsersService.Update].
type UpdateUserOptions struct{}

// Update replaces the writable fields of the user with the given ID. Every
// writable field is sent, except empty ConfigurationPermissions, which keep
// the user's permissions; to remove them, use [UsersService.Patch]. A
// non-empty ConfigurationPermissions replaces the user's configuration
// permissions. Update a user returned by [UsersService.Get] to keep its other
// fields, or use [UsersService.Patch].
//
// The API can't change a password: DefectDojo answers a password in an
// update with a 400.
//
// DefectDojo API: PUT /api/v2/users/{id}/
func (s *UsersService) Update(ctx context.Context, id int, u *User, opt *UpdateUserOptions, opts ...RequestOption) (*User, *Response, error) {
	if err := requirePositive("user ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newUserRequest(u)
	if err != nil {
		return nil, nil, err
	}
	updated := new(User)
	resp, err := s.client.do(ctx, http.MethodPut, userPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// PatchUserOptions are the options of [UsersService.Patch].
type PatchUserOptions struct{}

// Patch changes the fields of the user that are set in patch.
//
// DefectDojo API: PATCH /api/v2/users/{id}/
func (s *UsersService) Patch(ctx context.Context, id int, patch *UserPatch, opt *PatchUserOptions, opts ...RequestOption) (*User, *Response, error) {
	if err := requirePositive("user ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: user patch must not be nil")
	}
	if p := orEmptyPtr(patch.ConfigurationPermissions); p != patch.ConfigurationPermissions {
		// Send [] from a copy, leaving the caller's patch as is.
		c := *patch
		c.ConfigurationPermissions = p
		patch = &c
	}
	updated := new(User)
	resp, err := s.client.do(ctx, http.MethodPatch, userPath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteUserOptions are the options of [UsersService.Delete].
type DeleteUserOptions struct{}

// Delete deletes a user by its ID, with its notification settings, alerts
// and the notification webhooks it owns.
//
// Deleting the user the client authenticates as is a 400 ([ErrBadRequest]),
// deleting a staff user or superuser needs a superuser (403), and deleting a
// user that other objects refer to is a 409 ([ErrConflict]), for example
// findings and finding groups it reported or imported, tests and engagements
// it leads, risk acceptances it owns, products that name it as a contact.
// Deactivate such users instead (IsActive false).
//
// DefectDojo API: DELETE /api/v2/users/{id}/
func (s *UsersService) Delete(ctx context.Context, id int, opt *DeleteUserOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("user ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, userPath(id), nil, nil, opts)
}

// DeletePreviewUserOptions are the options of [UsersService.DeletePreview].
type DeletePreviewUserOptions struct {
	ListOptions
}

// DeletePreview returns a page of the objects that deleting the user would
// remove, the user itself included.
//
// DefectDojo API: GET /api/v2/users/{id}/delete_preview/
func (s *UsersService) DeletePreview(ctx context.Context, id int, opt *DeletePreviewUserOptions, opts ...RequestOption) (*DeletePreviewResult, *Response, error) {
	if err := requirePositive("user ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(userPath(id)+"delete_preview/", opt)
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

// ResetUserAPITokenOptions are the options of [UsersService.ResetAPIToken].
type ResetUserAPITokenOptions struct{}

// ResetAPIToken replaces the API token of the user with the given ID with a
// new one, which the user can see in the UI, and notifies the user. It needs
// a superuser, or a global owner.
//
// Resetting the token of the user the client authenticates as is a 400
// ([ErrBadRequest]), and the reset is a 403 ([ErrForbidden]) when API tokens
// are disabled. (DefectDojo's schema describes a request body and a 200
// response with a user; the endpoint takes no body and answers 204.)
//
// DefectDojo API: POST /api/v2/users/{id}/reset_api_token/
func (s *UsersService) ResetAPIToken(ctx context.Context, id int, opt *ResetUserAPITokenOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("user ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodPost, userPath(id)+"reset_api_token/", nil, nil, opts)
}

func userPath(id int) string {
	return "/api/v2/users/" + strconv.Itoa(id) + "/"
}

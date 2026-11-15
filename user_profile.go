package defectdojo

import (
	"context"
	"net/http"
	"time"
)

// UserProfileService handles communication with the user profile method of
// the DefectDojo API: the user the client authenticates as.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type UserProfileService service

// UserProfile is the user the client authenticates as.
type UserProfile struct {
	_ [0]func()

	User *User `json:"user"`

	// UserContactInfo is nil for users without contact information, such as
	// the admin user DefectDojo creates.
	UserContactInfo *UserContactInfo `json:"user_contact_info"`
}

// User is a DefectDojo user.
type User struct {
	_ [0]func()

	ID        int    `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`

	IsActive bool `json:"is_active"`

	// IsStaff and IsSuperuser grant access beyond the authorized users of
	// product types and products. Some configuration endpoints, such as
	// users, system settings, notifications and Jira, need a superuser;
	// staff users can also manage others, such as SLA configurations,
	// development environments and the product platforms, lifecycles and
	// origins.
	IsStaff     bool `json:"is_staff"`
	IsSuperuser bool `json:"is_superuser"`

	// ConfigurationPermissions are the IDs of the configuration permissions
	// the user has; see [ConfigurationPermissionsService].
	// [UsersService.Create] and [UsersService.Update] leave an empty list
	// out, which keeps the user's permissions: remove them with
	// [UsersService.Patch]. (The schema allows null items, an artifact of
	// the many-to-many field; permission IDs are never null.)
	ConfigurationPermissions []int `json:"configuration_permissions"`

	DateJoined time.Time `json:"date_joined,omitzero"`

	// LastLogin is zero for users that never logged in to the UI.
	LastLogin time.Time `json:"last_login,omitzero"`

	// TokenLastReset is when the user's API token was last reset, and
	// TokenExpiry when it expires; both are zero when that never happened or
	// never happens. PasswordLastReset is when the user's password was last
	// reset. All three are read-only.
	TokenLastReset    time.Time `json:"token_last_reset,omitzero"`
	TokenExpiry       time.Time `json:"token_expiry,omitzero"`
	PasswordLastReset time.Time `json:"password_last_reset,omitzero"`
}

// DeduplicationExecutionMode is how DefectDojo runs deduplication for a
// user's imports.
type DeduplicationExecutionMode string

// The deduplication execution modes. "" means the instance default.
const (
	DeduplicationExecutionModeAsync     DeduplicationExecutionMode = "async"
	DeduplicationExecutionModeAsyncWait DeduplicationExecutionMode = "async_wait"
	DeduplicationExecutionModeSync      DeduplicationExecutionMode = "sync"
)

// UserContactInfo is the contact information and personal settings of a
// user.
type UserContactInfo struct {
	_ [0]func()

	ID int `json:"id"`

	// User is the ID of the user.
	User int `json:"user"`

	Title           string `json:"title"`
	PhoneNumber     string `json:"phone_number"`
	CellNumber      string `json:"cell_number"`
	TwitterUsername string `json:"twitter_username"`
	GithubUsername  string `json:"github_username"`
	SlackUsername   string `json:"slack_username"`
	SlackUserID     string `json:"slack_user_id"`

	// BlockExecution makes DefectDojo run the user's background tasks, such
	// as deduplication, before answering.
	BlockExecution bool `json:"block_execution"`

	// DeduplicationExecutionMode is one of the DeduplicationExecutionMode
	// constants, or "" for the instance default.
	DeduplicationExecutionMode DeduplicationExecutionMode `json:"deduplication_execution_mode"`

	ForcePasswordReset bool `json:"force_password_reset"`

	// Language is the user's UI language, or "" for the instance default.
	Language string `json:"language"`
}

// GetUserProfileOptions are the options of [UserProfileService.Get].
type GetUserProfileOptions struct {
	_ [0]func()
}

// Get returns the user the client authenticates as.
//
// DefectDojo API: GET /api/v2/user_profile/
func (s *UserProfileService) Get(ctx context.Context, opt *GetUserProfileOptions, opts ...RequestOption) (*UserProfile, *Response, error) {
	profile := new(UserProfile)
	resp, err := s.client.do(ctx, http.MethodGet, "/api/v2/user_profile/", nil, profile, opts)
	if err != nil {
		return nil, resp, err
	}
	return profile, resp, nil
}

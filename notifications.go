package defectdojo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// NotificationsService handles communication with the notification settings
// methods of the DefectDojo API. The endpoints need a superuser.
//
// Each [NotificationSettings] row chooses, for each event, the channels on
// which DefectDojo notifies. Its scope is one of:
//
//   - system-wide: no user or product, not the template;
//   - the template: no user or product, Template set; DefectDojo copies it
//     into the personal row of each new user;
//   - personal: a user, no product;
//   - per product: a user and a product.
//
// DefectDojo creates the system row when it first sends a notification, and
// a personal row for each new user; use [NotificationsService.GetByScope] to
// find the row of a scope.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type NotificationsService service

// NotificationChannel is a channel DefectDojo notifies on.
type NotificationChannel string

// Notification channels, the values of the event fields of
// [NotificationSettings]. Slack, Microsoft Teams, mail and webhooks also need
// to be enabled in the system settings.
const (
	NotificationChannelSlack    NotificationChannel = "slack"
	NotificationChannelMSTeams  NotificationChannel = "msteams"
	NotificationChannelMail     NotificationChannel = "mail"
	NotificationChannelWebhooks NotificationChannel = "webhooks"
	NotificationChannelAlert    NotificationChannel = "alert"
)

// NotificationSettings is a DefectDojo notification settings row. The same
// type is used for reads and writes: the read-only ID is ignored when
// writing.
//
// Each event field lists the channels (NotificationChannel constants) on
// which DefectDojo notifies of the event; DefectDojo's default is alert only.
// The events are:
//
//   - ProductTypeAdded: a product type is added
//   - ProductAdded: a product is added
//   - EngagementAdded: an engagement is added
//   - TestAdded: a test is added
//   - ScanAdded: an import or reimport creates, updates or closes findings
//   - JiraUpdate: the background Jira synchronization fails
//   - UpcomingEngagement: an engagement is about to start
//   - StaleEngagement: an engagement is past its end date
//   - AutoCloseEngagement: DefectDojo closes an engagement automatically
//   - CloseEngagement: an engagement is closed
//   - UserMentioned: the user is mentioned in a note
//   - CodeReview: a code review is requested
//   - ReviewRequested: a review of findings is requested
//   - Other: other events, such as a revoked API token
//   - SLABreach: a finding breaches its SLA
//   - SLABreachCombined: findings breach their SLA, in one combined notification
//   - RiskAcceptanceExpiration: a risk acceptance expires
//   - ScanAddedEmpty: an import or reimport changes no findings
type NotificationSettings struct {
	_ [0]func()

	ID int `json:"id"`

	// User is the ID of the user the settings belong to, or 0 for the
	// system-wide settings and the template.
	User int `json:"user"`

	// Product is the ID of the product the settings apply to, or 0 for all
	// products.
	Product int `json:"product"`

	// Template marks the template for the personal settings of new users.
	// There can be one.
	Template bool `json:"template"`

	ProductTypeAdded         []NotificationChannel `json:"product_type_added"`
	ProductAdded             []NotificationChannel `json:"product_added"`
	EngagementAdded          []NotificationChannel `json:"engagement_added"`
	TestAdded                []NotificationChannel `json:"test_added"`
	ScanAdded                []NotificationChannel `json:"scan_added"`
	JiraUpdate               []NotificationChannel `json:"jira_update"`
	UpcomingEngagement       []NotificationChannel `json:"upcoming_engagement"`
	StaleEngagement          []NotificationChannel `json:"stale_engagement"`
	AutoCloseEngagement      []NotificationChannel `json:"auto_close_engagement"`
	CloseEngagement          []NotificationChannel `json:"close_engagement"`
	UserMentioned            []NotificationChannel `json:"user_mentioned"`
	CodeReview               []NotificationChannel `json:"code_review"`
	ReviewRequested          []NotificationChannel `json:"review_requested"`
	Other                    []NotificationChannel `json:"other"`
	SLABreach                []NotificationChannel `json:"sla_breach"`
	SLABreachCombined        []NotificationChannel `json:"sla_breach_combined"`
	RiskAcceptanceExpiration []NotificationChannel `json:"risk_acceptance_expiration"`

	// ScanAddedEmpty holds at most one channel when it is written through
	// the API (DefectDojo's API takes a single choice for it, unlike the
	// other events); DefectDojo's UI can set several. Its default is no
	// channel.
	ScanAddedEmpty []NotificationChannel `json:"scan_added_empty"`
}

// UnmarshalJSON decodes notification settings. DefectDojo returns
// scan_added_empty as a list, and as a single channel ("" for none) in the
// response to a write.
func (n *NotificationSettings) UnmarshalJSON(data []byte) error {
	type plain NotificationSettings
	v := struct {
		*plain
		// It shadows the field of plain, which has the same JSON name.
		ScanAddedEmpty json.RawMessage `json:"scan_added_empty"`
	}{plain: (*plain)(n)}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	// As for the other fields, a missing value leaves the field as it is.
	if len(v.ScanAddedEmpty) == 0 {
		return nil
	}
	channels, err := decodeChannels(v.ScanAddedEmpty)
	if err != nil {
		return err
	}
	n.ScanAddedEmpty = channels
	return nil
}

// decodeChannels decodes a JSON list of channels, a single channel, or "" or
// null for none (nil for a missing value or null).
func decodeChannels(data json.RawMessage) ([]NotificationChannel, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var channel NotificationChannel
	if err := json.Unmarshal(data, &channel); err == nil {
		if channel == "" {
			return []NotificationChannel{}, nil
		}
		return []NotificationChannel{channel}, nil
	}
	var channels []NotificationChannel
	if err := json.Unmarshal(data, &channels); err != nil {
		return nil, fmt.Errorf("defectdojo: notification channels must be a list or a string: %w", err)
	}
	return channels, nil
}

// notificationSettingsRequest is the body of notification settings create
// and update requests. Nil events are left out, so that DefectDojo's default
// applies.
type notificationSettingsRequest struct {
	User     *int `json:"user"`
	Product  *int `json:"product"`
	Template bool `json:"template"`

	ProductTypeAdded         []NotificationChannel `json:"product_type_added,omitzero"`
	ProductAdded             []NotificationChannel `json:"product_added,omitzero"`
	EngagementAdded          []NotificationChannel `json:"engagement_added,omitzero"`
	TestAdded                []NotificationChannel `json:"test_added,omitzero"`
	ScanAdded                []NotificationChannel `json:"scan_added,omitzero"`
	JiraUpdate               []NotificationChannel `json:"jira_update,omitzero"`
	UpcomingEngagement       []NotificationChannel `json:"upcoming_engagement,omitzero"`
	StaleEngagement          []NotificationChannel `json:"stale_engagement,omitzero"`
	AutoCloseEngagement      []NotificationChannel `json:"auto_close_engagement,omitzero"`
	CloseEngagement          []NotificationChannel `json:"close_engagement,omitzero"`
	UserMentioned            []NotificationChannel `json:"user_mentioned,omitzero"`
	CodeReview               []NotificationChannel `json:"code_review,omitzero"`
	ReviewRequested          []NotificationChannel `json:"review_requested,omitzero"`
	Other                    []NotificationChannel `json:"other,omitzero"`
	SLABreach                []NotificationChannel `json:"sla_breach,omitzero"`
	SLABreachCombined        []NotificationChannel `json:"sla_breach_combined,omitzero"`
	RiskAcceptanceExpiration []NotificationChannel `json:"risk_acceptance_expiration,omitzero"`

	// ScanAddedEmpty is a single channel, or "" for none. It is always
	// sent: DefectDojo has no default for it in requests, so a PUT that
	// leaves it out would keep it.
	ScanAddedEmpty NotificationChannel `json:"scan_added_empty"`
}

// newNotificationSettingsRequest validates notification settings and returns
// their writable fields.
func newNotificationSettingsRequest(n *NotificationSettings) (*notificationSettingsRequest, error) {
	if n == nil {
		return nil, errors.New("defectdojo: notification settings must not be nil")
	}
	if n.Template && (n.User != 0 || n.Product != 0) {
		return nil, errors.New("defectdojo: the notification settings template can't have a user or product")
	}
	scanAddedEmpty, err := singleChannel(n.ScanAddedEmpty)
	if err != nil {
		return nil, err
	}
	return &notificationSettingsRequest{
		User:                     orNull(n.User),
		Product:                  orNull(n.Product),
		Template:                 n.Template,
		ProductTypeAdded:         n.ProductTypeAdded,
		ProductAdded:             n.ProductAdded,
		EngagementAdded:          n.EngagementAdded,
		TestAdded:                n.TestAdded,
		ScanAdded:                n.ScanAdded,
		JiraUpdate:               n.JiraUpdate,
		UpcomingEngagement:       n.UpcomingEngagement,
		StaleEngagement:          n.StaleEngagement,
		AutoCloseEngagement:      n.AutoCloseEngagement,
		CloseEngagement:          n.CloseEngagement,
		UserMentioned:            n.UserMentioned,
		CodeReview:               n.CodeReview,
		ReviewRequested:          n.ReviewRequested,
		Other:                    n.Other,
		SLABreach:                n.SLABreach,
		SLABreachCombined:        n.SLABreachCombined,
		RiskAcceptanceExpiration: n.RiskAcceptanceExpiration,
		ScanAddedEmpty:           scanAddedEmpty,
	}, nil
}

// singleChannel returns the channel of scan_added_empty as DefectDojo's API
// takes it: "" for none, or the one channel.
func singleChannel(channels []NotificationChannel) (NotificationChannel, error) {
	switch len(channels) {
	case 0:
		return "", nil
	case 1:
		return channels[0], nil
	}
	return "", fmt.Errorf("defectdojo: ScanAddedEmpty can hold one channel through DefectDojo's API, got %d", len(channels))
}

// NotificationSettingsPatch is the body of [NotificationsService.Patch]. Only
// the events that are set are changed; a pointer to a nil or empty slice
// removes every channel. The user, product and template, which identify a
// row, can't be changed.
type NotificationSettingsPatch struct {
	_ [0]func()

	ProductTypeAdded         *[]NotificationChannel `json:"product_type_added,omitempty"`
	ProductAdded             *[]NotificationChannel `json:"product_added,omitempty"`
	EngagementAdded          *[]NotificationChannel `json:"engagement_added,omitempty"`
	TestAdded                *[]NotificationChannel `json:"test_added,omitempty"`
	ScanAdded                *[]NotificationChannel `json:"scan_added,omitempty"`
	JiraUpdate               *[]NotificationChannel `json:"jira_update,omitempty"`
	UpcomingEngagement       *[]NotificationChannel `json:"upcoming_engagement,omitempty"`
	StaleEngagement          *[]NotificationChannel `json:"stale_engagement,omitempty"`
	AutoCloseEngagement      *[]NotificationChannel `json:"auto_close_engagement,omitempty"`
	CloseEngagement          *[]NotificationChannel `json:"close_engagement,omitempty"`
	UserMentioned            *[]NotificationChannel `json:"user_mentioned,omitempty"`
	CodeReview               *[]NotificationChannel `json:"code_review,omitempty"`
	ReviewRequested          *[]NotificationChannel `json:"review_requested,omitempty"`
	Other                    *[]NotificationChannel `json:"other,omitempty"`
	SLABreach                *[]NotificationChannel `json:"sla_breach,omitempty"`
	SLABreachCombined        *[]NotificationChannel `json:"sla_breach_combined,omitempty"`
	RiskAcceptanceExpiration *[]NotificationChannel `json:"risk_acceptance_expiration,omitempty"`

	// ScanAddedEmpty is a single channel, or "" for none.
	ScanAddedEmpty *NotificationChannel `json:"scan_added_empty,omitempty"`
}

// normalized returns a copy of the patch whose pointers to nil slices point
// to empty ones: DefectDojo refuses null.
func (patch *NotificationSettingsPatch) normalized() *NotificationSettingsPatch {
	p := *patch
	p.ProductTypeAdded = orEmptyPtr(p.ProductTypeAdded)
	p.ProductAdded = orEmptyPtr(p.ProductAdded)
	p.EngagementAdded = orEmptyPtr(p.EngagementAdded)
	p.TestAdded = orEmptyPtr(p.TestAdded)
	p.ScanAdded = orEmptyPtr(p.ScanAdded)
	p.JiraUpdate = orEmptyPtr(p.JiraUpdate)
	p.UpcomingEngagement = orEmptyPtr(p.UpcomingEngagement)
	p.StaleEngagement = orEmptyPtr(p.StaleEngagement)
	p.AutoCloseEngagement = orEmptyPtr(p.AutoCloseEngagement)
	p.CloseEngagement = orEmptyPtr(p.CloseEngagement)
	p.UserMentioned = orEmptyPtr(p.UserMentioned)
	p.CodeReview = orEmptyPtr(p.CodeReview)
	p.ReviewRequested = orEmptyPtr(p.ReviewRequested)
	p.Other = orEmptyPtr(p.Other)
	p.SLABreach = orEmptyPtr(p.SLABreach)
	p.SLABreachCombined = orEmptyPtr(p.SLABreachCombined)
	p.RiskAcceptanceExpiration = orEmptyPtr(p.RiskAcceptanceExpiration)
	return &p
}

// ListNotificationSettingsOptions are the options of [NotificationsService.List] and
// [NotificationsService.ListAll].
type ListNotificationSettingsOptions struct {
	_ [0]func()

	ListOptions

	// User and Product filter by ID. An ID that doesn't exist is a 400
	// ([ErrBadRequest]). DefectDojo can't filter for rows without a user or
	// product.
	User    int `url:"user,omitempty"`
	Product int `url:"product,omitempty"`

	Template *bool `url:"template,omitempty"`
}

// ListNotificationSettingsResult is a page of notification settings.
type ListNotificationSettingsResult struct {
	_ [0]func()

	Results []NotificationSettings

	// Count is the number of rows that match the filters.
	Count int
}

// List returns a page of notification settings, ordered by ID.
//
// DefectDojo API: GET /api/v2/notifications/
func (s *NotificationsService) List(ctx context.Context, opt *ListNotificationSettingsOptions, opts ...RequestOption) (*ListNotificationSettingsResult, *Response, error) {
	path, err := addQuery("/api/v2/notifications/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[NotificationSettings]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListNotificationSettingsResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the notification settings, starting at
// opt.Offset. It fetches opt.Limit rows per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *NotificationsService) ListAll(ctx context.Context, opt *ListNotificationSettingsOptions, opts ...RequestOption) iter.Seq2[*NotificationSettings, error] {
	base := ListNotificationSettingsOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*NotificationSettings, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Results), result.Count, nil
	})
}

// GetNotificationSettingsOptions are the options of [NotificationsService.Get].
type GetNotificationSettingsOptions struct {
	_ [0]func()

	_ [0]func()
}

// Get returns notification settings by their ID.
//
// DefectDojo API: GET /api/v2/notifications/{id}/
func (s *NotificationsService) Get(ctx context.Context, id int, opt *GetNotificationSettingsOptions, opts ...RequestOption) (*NotificationSettings, *Response, error) {
	if err := requirePositive("notification settings ID", id); err != nil {
		return nil, nil, err
	}
	n := new(NotificationSettings)
	resp, err := s.client.do(ctx, http.MethodGet, notificationSettingsPath(id), nil, n, opts)
	if err != nil {
		return nil, resp, err
	}
	return n, resp, nil
}

// NotificationScope identifies a row of notification settings: the
// system-wide settings (the zero value), a user's personal settings (User), a
// user's settings for a product (User and Product), or the template for new
// users (Template). It is a value type, compared with ==.
type NotificationScope struct {
	// User is the ID of the user, or 0 for none.
	User int

	// Product is the ID of the product, or 0 for none.
	Product int

	// Template selects the template for new users, which has no user or
	// product.
	Template bool
}

// GetNotificationSettingsByScopeOptions are the options of [NotificationsService.GetByScope].
type GetNotificationSettingsByScopeOptions struct {
	_ [0]func()

	_ [0]func()
}

// GetByScope returns the notification settings of a scope. It returns an
// error matching [ErrNotFound] when there is no such row, also when the user
// or product doesn't exist, and an error matching [ErrAmbiguous] when there
// are several.
//
// DefectDojo can't filter for rows without a user, so finding the
// system-wide settings or the template lists the rows of every user.
func (s *NotificationsService) GetByScope(ctx context.Context, scope NotificationScope, opt *GetNotificationSettingsByScopeOptions, opts ...RequestOption) (*NotificationSettings, *Response, error) {
	user, product, template := scope.User, scope.Product, scope.Template
	if user < 0 || product < 0 {
		return nil, nil, fmt.Errorf("defectdojo: notification settings user and product must not be negative, got %d and %d", user, product)
	}
	if template && (user != 0 || product != 0) {
		return nil, nil, errors.New("defectdojo: the notification settings template has no user or product")
	}
	listOpt := &ListNotificationSettingsOptions{ListOptions: ListOptions{Limit: 100}, User: user, Product: product, Template: &template}
	var (
		found *NotificationSettings
		ids   []string
		resp  *Response
	)
	for {
		list, r, err := s.List(ctx, listOpt, opts...)
		resp = r
		if err != nil {
			return nil, resp, missingFilterObject(err, "user", "product")
		}
		for i := range list.Results {
			n := &list.Results[i]
			if n.User == user && n.Product == product && n.Template == template {
				found = n
				ids = append(ids, strconv.Itoa(n.ID))
			}
		}
		listOpt.Offset += len(list.Results)
		if len(list.Results) == 0 || listOpt.Offset >= list.Count {
			break
		}
	}
	desc := fmt.Sprintf("user %d, product %d, template %t", user, product, template)
	switch len(ids) {
	case 0:
		return nil, resp, fmt.Errorf("defectdojo: notification settings of %s: %w", desc, ErrNotFound)
	case 1:
		return found, resp, nil
	}
	return nil, resp, fmt.Errorf("%w: several notification settings of %s: IDs %v", ErrAmbiguous, desc, ids)
}

// CreateNotificationSettingsOptions are the options of [NotificationsService.Create].
type CreateNotificationSettingsOptions struct {
	_ [0]func()

	_ [0]func()
}

// Create creates notification settings. Nil events are left out, so that
// DefectDojo's default (alert) applies; an empty, non-nil event has no
// channels. A nil ScanAddedEmpty is sent as no channel, its default. A row
// for a scope that has one is a 400 ([ErrBadRequest]), and so is a second
// template.
//
// DefectDojo API: POST /api/v2/notifications/
func (s *NotificationsService) Create(ctx context.Context, n *NotificationSettings, opt *CreateNotificationSettingsOptions, opts ...RequestOption) (*NotificationSettings, *Response, error) {
	body, err := newNotificationSettingsRequest(n)
	if err != nil {
		return nil, nil, err
	}
	created := new(NotificationSettings)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/notifications/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateNotificationSettingsOptions are the options of [NotificationsService.Update].
type UpdateNotificationSettingsOptions struct {
	_ [0]func()

	_ [0]func()
}

// Update replaces the notification settings with the given ID. The user,
// product and template are always sent: DefectDojo resets them when they are
// left out. Nil events are left out, which resets them to DefectDojo's
// default (alert); a nil ScanAddedEmpty is sent as no channel.
//
// Don't update the template with Update: DefectDojo refuses an update with
// Template set while a template exists, the template itself included. Use
// [NotificationsService.Patch].
//
// DefectDojo API: PUT /api/v2/notifications/{id}/
func (s *NotificationsService) Update(ctx context.Context, id int, n *NotificationSettings, opt *UpdateNotificationSettingsOptions, opts ...RequestOption) (*NotificationSettings, *Response, error) {
	if err := requirePositive("notification settings ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newNotificationSettingsRequest(n)
	if err != nil {
		return nil, nil, err
	}
	updated := new(NotificationSettings)
	resp, err := s.client.do(ctx, http.MethodPut, notificationSettingsPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// PatchNotificationSettingsOptions are the options of [NotificationsService.Patch].
type PatchNotificationSettingsOptions struct {
	_ [0]func()

	_ [0]func()
}

// Patch changes the events of the notification settings that are set in
// patch.
//
// DefectDojo API: PATCH /api/v2/notifications/{id}/
func (s *NotificationsService) Patch(ctx context.Context, id int, patch *NotificationSettingsPatch, opt *PatchNotificationSettingsOptions, opts ...RequestOption) (*NotificationSettings, *Response, error) {
	if err := requirePositive("notification settings ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: notification settings patch must not be nil")
	}
	updated := new(NotificationSettings)
	resp, err := s.client.do(ctx, http.MethodPatch, notificationSettingsPath(id), patch.normalized(), updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteNotificationSettingsOptions are the options of [NotificationsService.Delete].
type DeleteNotificationSettingsOptions struct {
	_ [0]func()

	_ [0]func()
}

// Delete deletes notification settings by their ID.
//
// Deleting a user's personal settings stops the user's notifications, unless
// the user is a superuser. DefectDojo creates the system-wide settings again,
// with its defaults, when it next sends a notification.
//
// DefectDojo API: DELETE /api/v2/notifications/{id}/
func (s *NotificationsService) Delete(ctx context.Context, id int, opt *DeleteNotificationSettingsOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("notification settings ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, notificationSettingsPath(id), nil, nil, opts)
}

// DeletePreviewNotificationSettingsOptions are the options of [NotificationsService.DeletePreview].
type DeletePreviewNotificationSettingsOptions struct {
	_ [0]func()

	ListOptions
}

// DeletePreview returns a page of the objects that deleting the notification
// settings would remove, the settings themselves included.
//
// DefectDojo API: GET /api/v2/notifications/{id}/delete_preview/
func (s *NotificationsService) DeletePreview(ctx context.Context, id int, opt *DeletePreviewNotificationSettingsOptions, opts ...RequestOption) (*DeletePreviewResult, *Response, error) {
	if err := requirePositive("notification settings ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(notificationSettingsPath(id)+"delete_preview/", opt)
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

func notificationSettingsPath(id int) string {
	return "/api/v2/notifications/" + strconv.Itoa(id) + "/"
}

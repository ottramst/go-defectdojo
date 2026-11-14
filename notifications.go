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

// Notification channels, the values of the event fields of
// [NotificationSettings]. Slack, Microsoft Teams, mail and webhooks also need
// to be enabled in the system settings.
const (
	NotificationChannelSlack    = "slack"
	NotificationChannelMSTeams  = "msteams"
	NotificationChannelMail     = "mail"
	NotificationChannelWebhooks = "webhooks"
	NotificationChannelAlert    = "alert"
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

	ProductTypeAdded         []string `json:"product_type_added"`
	ProductAdded             []string `json:"product_added"`
	EngagementAdded          []string `json:"engagement_added"`
	TestAdded                []string `json:"test_added"`
	ScanAdded                []string `json:"scan_added"`
	JiraUpdate               []string `json:"jira_update"`
	UpcomingEngagement       []string `json:"upcoming_engagement"`
	StaleEngagement          []string `json:"stale_engagement"`
	AutoCloseEngagement      []string `json:"auto_close_engagement"`
	CloseEngagement          []string `json:"close_engagement"`
	UserMentioned            []string `json:"user_mentioned"`
	CodeReview               []string `json:"code_review"`
	ReviewRequested          []string `json:"review_requested"`
	Other                    []string `json:"other"`
	SLABreach                []string `json:"sla_breach"`
	SLABreachCombined        []string `json:"sla_breach_combined"`
	RiskAcceptanceExpiration []string `json:"risk_acceptance_expiration"`

	// ScanAddedEmpty holds at most one channel when it is written through
	// the API (DefectDojo's API takes a single choice for it, unlike the
	// other events); DefectDojo's UI can set several. Its default is no
	// channel.
	ScanAddedEmpty NotificationChannels `json:"scan_added_empty"`
}

// NotificationChannels is a list of notification channels. It decodes a JSON
// list, and also a single channel as a string ("" for none), which DefectDojo
// returns for [NotificationSettings.ScanAddedEmpty] in the response to a
// write.
type NotificationChannels []string

// UnmarshalJSON decodes a JSON list of channels, a single channel, or "" or
// null for none.
func (c *NotificationChannels) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*c = nil
		return nil
	}
	var channel string
	if err := json.Unmarshal(data, &channel); err == nil {
		if channel == "" {
			*c = NotificationChannels{}
		} else {
			*c = NotificationChannels{channel}
		}
		return nil
	}
	var channels []string
	if err := json.Unmarshal(data, &channels); err != nil {
		return fmt.Errorf("defectdojo: notification channels must be a list or a string: %w", err)
	}
	*c = channels
	return nil
}

// notificationSettingsRequest is the body of notification settings create
// and update requests. Nil events are left out, so that DefectDojo's default
// applies.
type notificationSettingsRequest struct {
	User     *int `json:"user"`
	Product  *int `json:"product"`
	Template bool `json:"template"`

	ProductTypeAdded         []string `json:"product_type_added,omitzero"`
	ProductAdded             []string `json:"product_added,omitzero"`
	EngagementAdded          []string `json:"engagement_added,omitzero"`
	TestAdded                []string `json:"test_added,omitzero"`
	ScanAdded                []string `json:"scan_added,omitzero"`
	JiraUpdate               []string `json:"jira_update,omitzero"`
	UpcomingEngagement       []string `json:"upcoming_engagement,omitzero"`
	StaleEngagement          []string `json:"stale_engagement,omitzero"`
	AutoCloseEngagement      []string `json:"auto_close_engagement,omitzero"`
	CloseEngagement          []string `json:"close_engagement,omitzero"`
	UserMentioned            []string `json:"user_mentioned,omitzero"`
	CodeReview               []string `json:"code_review,omitzero"`
	ReviewRequested          []string `json:"review_requested,omitzero"`
	Other                    []string `json:"other,omitzero"`
	SLABreach                []string `json:"sla_breach,omitzero"`
	SLABreachCombined        []string `json:"sla_breach_combined,omitzero"`
	RiskAcceptanceExpiration []string `json:"risk_acceptance_expiration,omitzero"`

	// ScanAddedEmpty is a single channel, or "" for none. It is always
	// sent: DefectDojo has no default for it in requests, so a PUT that
	// leaves it out would keep it.
	ScanAddedEmpty *string `json:"scan_added_empty,omitempty"`
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
func singleChannel(channels NotificationChannels) (*string, error) {
	switch len(channels) {
	case 0:
		return new(""), nil
	case 1:
		return &channels[0], nil
	}
	return nil, fmt.Errorf("defectdojo: ScanAddedEmpty can hold one channel through DefectDojo's API, got %d", len(channels))
}

// NotificationSettingsPatch is the body of [NotificationsService.Patch]. Only
// the events that are set are changed; a pointer to a nil or empty slice
// removes every channel. The user, product and template, which identify a
// row, can't be changed.
type NotificationSettingsPatch struct {
	ProductTypeAdded         *[]string `json:"product_type_added,omitempty"`
	ProductAdded             *[]string `json:"product_added,omitempty"`
	EngagementAdded          *[]string `json:"engagement_added,omitempty"`
	TestAdded                *[]string `json:"test_added,omitempty"`
	ScanAdded                *[]string `json:"scan_added,omitempty"`
	JiraUpdate               *[]string `json:"jira_update,omitempty"`
	UpcomingEngagement       *[]string `json:"upcoming_engagement,omitempty"`
	StaleEngagement          *[]string `json:"stale_engagement,omitempty"`
	AutoCloseEngagement      *[]string `json:"auto_close_engagement,omitempty"`
	CloseEngagement          *[]string `json:"close_engagement,omitempty"`
	UserMentioned            *[]string `json:"user_mentioned,omitempty"`
	CodeReview               *[]string `json:"code_review,omitempty"`
	ReviewRequested          *[]string `json:"review_requested,omitempty"`
	Other                    *[]string `json:"other,omitempty"`
	SLABreach                *[]string `json:"sla_breach,omitempty"`
	SLABreachCombined        *[]string `json:"sla_breach_combined,omitempty"`
	RiskAcceptanceExpiration *[]string `json:"risk_acceptance_expiration,omitempty"`

	// ScanAddedEmpty is a single channel, or "" for none.
	ScanAddedEmpty *string `json:"scan_added_empty,omitempty"`
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

// ListNotificationsOptions are the options of [NotificationsService.List] and
// [NotificationsService.ListAll].
type ListNotificationsOptions struct {
	ListOptions

	// User and Product filter by ID. An ID that doesn't exist is a 400
	// ([ErrBadRequest]). DefectDojo can't filter for rows without a user or
	// product.
	User    int `url:"user,omitempty"`
	Product int `url:"product,omitempty"`

	Template *bool `url:"template,omitempty"`
}

// NotificationSettingsList is a page of notification settings.
type NotificationSettingsList struct {
	NotificationSettings []*NotificationSettings

	// Count is the number of rows that match the filters.
	Count int
}

// List returns a page of notification settings, ordered by ID.
//
// DefectDojo API: GET /api/v2/notifications/
func (s *NotificationsService) List(ctx context.Context, opt *ListNotificationsOptions, opts ...RequestOption) (*NotificationSettingsList, *Response, error) {
	path, err := addQuery("/api/v2/notifications/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*NotificationSettings]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &NotificationSettingsList{NotificationSettings: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the notification settings, starting at
// opt.Offset. It fetches opt.Limit rows per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *NotificationsService) ListAll(ctx context.Context, opt *ListNotificationsOptions, opts ...RequestOption) iter.Seq2[*NotificationSettings, error] {
	base := ListNotificationsOptions{}
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
		return result.NotificationSettings, result.Count, nil
	})
}

// Get returns notification settings by their ID.
//
// DefectDojo API: GET /api/v2/notifications/{id}/
func (s *NotificationsService) Get(ctx context.Context, id int, opts ...RequestOption) (*NotificationSettings, *Response, error) {
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

// GetByScope returns the notification settings of a scope: those of the user
// and product with the given IDs (0 for none), or the template. It returns an
// error matching [ErrNotFound] when there is no such row, also when the user
// or product doesn't exist, and an error when there are several.
//
// DefectDojo can't filter for rows without a user, so finding the
// system-wide settings or the template lists the rows of every user.
func (s *NotificationsService) GetByScope(ctx context.Context, user, product int, template bool, opts ...RequestOption) (*NotificationSettings, *Response, error) {
	if user < 0 || product < 0 {
		return nil, nil, fmt.Errorf("defectdojo: notification settings user and product must not be negative, got %d and %d", user, product)
	}
	if template && (user != 0 || product != 0) {
		return nil, nil, errors.New("defectdojo: the notification settings template has no user or product")
	}
	opt := &ListNotificationsOptions{ListOptions: ListOptions{Limit: 100}, User: user, Product: product, Template: &template}
	var (
		found *NotificationSettings
		ids   []string
		resp  *Response
	)
	for {
		list, r, err := s.List(ctx, opt, opts...)
		resp = r
		if err != nil {
			return nil, resp, missingFilterObject(err, "user", "product")
		}
		for _, n := range list.NotificationSettings {
			if n.User == user && n.Product == product && n.Template == template {
				found = n
				ids = append(ids, strconv.Itoa(n.ID))
			}
		}
		opt.Offset += len(list.NotificationSettings)
		if len(list.NotificationSettings) == 0 || opt.Offset >= list.Count {
			break
		}
	}
	scope := fmt.Sprintf("user %d, product %d, template %t", user, product, template)
	switch len(ids) {
	case 0:
		return nil, resp, fmt.Errorf("defectdojo: notification settings of %s: %w", scope, ErrNotFound)
	case 1:
		return found, resp, nil
	}
	return nil, resp, fmt.Errorf("defectdojo: several notification settings of %s: IDs %v", scope, ids)
}

// Create creates notification settings. Nil events are left out, so that
// DefectDojo's default (alert) applies; an empty, non-nil event has no
// channels, and so has a nil ScanAddedEmpty, whose default is none. A row for a scope that has one is a 400 ([ErrBadRequest]), and so
// is a second template.
//
// DefectDojo API: POST /api/v2/notifications/
func (s *NotificationsService) Create(ctx context.Context, n *NotificationSettings, opts ...RequestOption) (*NotificationSettings, *Response, error) {
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
func (s *NotificationsService) Update(ctx context.Context, id int, n *NotificationSettings, opts ...RequestOption) (*NotificationSettings, *Response, error) {
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

// Patch changes the events of the notification settings that are set in
// patch.
//
// DefectDojo API: PATCH /api/v2/notifications/{id}/
func (s *NotificationsService) Patch(ctx context.Context, id int, patch *NotificationSettingsPatch, opts ...RequestOption) (*NotificationSettings, *Response, error) {
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

// Delete deletes notification settings by their ID.
//
// Deleting a user's personal settings stops the user's notifications, unless
// the user is a superuser. DefectDojo creates the system-wide settings again,
// with its defaults, when it next sends a notification.
//
// DefectDojo API: DELETE /api/v2/notifications/{id}/
func (s *NotificationsService) Delete(ctx context.Context, id int, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("notification settings ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, notificationSettingsPath(id), nil, nil, opts)
}

// DeletePreview returns a page of the objects that deleting the notification
// settings would remove, the settings themselves included.
//
// DefectDojo API: GET /api/v2/notifications/{id}/delete_preview/
func (s *NotificationsService) DeletePreview(ctx context.Context, id int, opt *ListOptions, opts ...RequestOption) (*DeletePreviewList, *Response, error) {
	if err := requirePositive("notification settings ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(notificationSettingsPath(id)+"delete_preview/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*DeletedObject]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &DeletePreviewList{Objects: page.Results, Count: page.Count}, resp, nil
}

func notificationSettingsPath(id int) string {
	return "/api/v2/notifications/" + strconv.Itoa(id) + "/"
}

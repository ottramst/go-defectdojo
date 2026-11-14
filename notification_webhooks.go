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

// NotificationWebhooksService handles communication with the notification
// webhook methods of the DefectDojo API. DefectDojo posts notifications to
// the webhooks of users and products whose notification settings choose the
// "webhooks" channel. The endpoints need a superuser.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type NotificationWebhooksService service

// Statuses of [NotificationWebhook.Status]. DefectDojo sets them when
// deliveries fail; the API can't change them.
const (
	NotificationWebhookStatusActive = "active"
	// NotificationWebhookStatusActiveTemporary means a delivery failed with
	// a 5xx error or similar, and DefectDojo keeps trying.
	NotificationWebhookStatusActiveTemporary = "active_tmp"
	// NotificationWebhookStatusInactiveTemporary means deliveries failed for
	// a while, and DefectDojo pauses them.
	NotificationWebhookStatusInactiveTemporary = "inactive_tmp"
	// NotificationWebhookStatusInactivePermanent means DefectDojo gave up on
	// the webhook. The API can't reactivate it: create a new one.
	NotificationWebhookStatusInactivePermanent = "inactive_permanent"
)

// NotificationWebhook is a DefectDojo notification webhook. The same type is
// used for reads and writes: the read-only fields ID, Status, FirstError,
// LastError and Note are ignored when writing.
type NotificationWebhook struct {
	ID int `json:"id"`

	// Name is unique across all notification webhooks, compared
	// case-sensitively, and at most 100 characters.
	Name string `json:"name"`

	// URL is where DefectDojo posts notifications, at most 200 characters.
	URL string `json:"url"`

	// HeaderName and HeaderValue are a header that DefectDojo sends with
	// each notification, such as an authorization header; each is at most
	// 100 characters. DefectDojo returns HeaderValue in plain text to every
	// client that may read the webhook.
	HeaderName  string `json:"header_name"`
	HeaderValue string `json:"header_value"`

	// Status is one of the NotificationWebhookStatus constants.
	Status string `json:"status"`

	// FirstError and LastError are when deliveries started failing and last
	// failed, and Note describes the last error. They are zero or empty
	// while deliveries succeed.
	FirstError time.Time `json:"first_error,omitzero"`
	LastError  time.Time `json:"last_error,omitzero"`
	Note       string    `json:"note"`

	// Owner is the ID of the user whose notifications the webhook receives,
	// or 0 for a system webhook. Deleting the user deletes the webhook.
	Owner int `json:"owner"`
}

// notificationWebhookRequest is the body of notification webhook create and
// update requests: the writable fields of a [NotificationWebhook].
type notificationWebhookRequest struct {
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	HeaderName  *string `json:"header_name"`
	HeaderValue *string `json:"header_value"`
	Owner       *int    `json:"owner"`
}

// newNotificationWebhookRequest validates a notification webhook and returns
// its writable fields.
func newNotificationWebhookRequest(w *NotificationWebhook) (*notificationWebhookRequest, error) {
	if w == nil {
		return nil, errors.New("defectdojo: notification webhook must not be nil")
	}
	// DefectDojo stores an empty name or URL when they are left out.
	if w.Name == "" {
		return nil, errors.New("defectdojo: notification webhook Name must not be empty")
	}
	if w.URL == "" {
		return nil, errors.New("defectdojo: notification webhook URL must not be empty")
	}
	return &notificationWebhookRequest{
		Name:        w.Name,
		URL:         w.URL,
		HeaderName:  orNull(w.HeaderName),
		HeaderValue: orNull(w.HeaderValue),
		Owner:       orNull(w.Owner),
	}, nil
}

// NotificationWebhookPatch is the body of [NotificationWebhooksService.Patch].
// Only the fields that are set are changed.
type NotificationWebhookPatch struct {
	Name        *string          `json:"name,omitempty"`
	URL         *string          `json:"url,omitempty"`
	HeaderName  Nullable[string] `json:"header_name,omitzero"`
	HeaderValue Nullable[string] `json:"header_value,omitzero"`
	Owner       Nullable[int]    `json:"owner,omitzero"`
}

// ListNotificationWebhooksOptions are the options of
// [NotificationWebhooksService.List] and [NotificationWebhooksService.ListAll].
// DefectDojo can also filter by header value, which the client leaves out so
// that secrets don't end up in URLs and logs.
type ListNotificationWebhooksOptions struct {
	ListOptions

	// Name, URL and Status are exact, case-sensitive matches.
	Name   string `url:"name,omitempty"`
	URL    string `url:"url,omitempty"`
	Status string `url:"status,omitempty"`

	// Owner filters by the ID of the owner.
	Owner int `url:"owner,omitempty"`
}

// NotificationWebhookList is a page of notification webhooks.
type NotificationWebhookList struct {
	NotificationWebhooks []*NotificationWebhook

	// Count is the number of notification webhooks that match the filters.
	Count int
}

// List returns a page of notification webhooks.
//
// DefectDojo API: GET /api/v2/notification_webhooks/
func (s *NotificationWebhooksService) List(ctx context.Context, opt *ListNotificationWebhooksOptions, opts ...RequestOption) (*NotificationWebhookList, *Response, error) {
	path, err := addQuery("/api/v2/notification_webhooks/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*NotificationWebhook]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &NotificationWebhookList{NotificationWebhooks: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the notification webhooks, starting at
// opt.Offset. It fetches opt.Limit webhooks per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *NotificationWebhooksService) ListAll(ctx context.Context, opt *ListNotificationWebhooksOptions, opts ...RequestOption) iter.Seq2[*NotificationWebhook, error] {
	base := ListNotificationWebhooksOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*NotificationWebhook, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return result.NotificationWebhooks, result.Count, nil
	})
}

// Get returns a notification webhook by its ID.
//
// DefectDojo API: GET /api/v2/notification_webhooks/{id}/
func (s *NotificationWebhooksService) Get(ctx context.Context, id int, opts ...RequestOption) (*NotificationWebhook, *Response, error) {
	if err := requirePositive("notification webhook ID", id); err != nil {
		return nil, nil, err
	}
	w := new(NotificationWebhook)
	resp, err := s.client.do(ctx, http.MethodGet, notificationWebhookPath(id), nil, w, opts)
	if err != nil {
		return nil, resp, err
	}
	return w, resp, nil
}

// GetByName returns the notification webhook with exactly the given name. It
// returns an error matching [ErrNotFound] when there is none.
func (s *NotificationWebhooksService) GetByName(ctx context.Context, name string, opts ...RequestOption) (*NotificationWebhook, *Response, error) {
	if name == "" {
		return nil, nil, errors.New("defectdojo: notification webhook name must not be empty")
	}
	list, resp, err := s.List(ctx, &ListNotificationWebhooksOptions{Name: name}, opts...)
	if err != nil {
		return nil, resp, err
	}
	// The filter is exact today; check again in case a version loosens it.
	for _, w := range list.NotificationWebhooks {
		if w.Name == name {
			return w, resp, nil
		}
	}
	return nil, resp, fmt.Errorf("defectdojo: notification webhook %q: %w", name, ErrNotFound)
}

// Create creates a notification webhook. Its read-only fields are ignored. A
// name that is taken is a 400 ([ErrBadRequest]) with a "name" field error.
// DefectDojo doesn't test the URL.
//
// DefectDojo API: POST /api/v2/notification_webhooks/
func (s *NotificationWebhooksService) Create(ctx context.Context, w *NotificationWebhook, opts ...RequestOption) (*NotificationWebhook, *Response, error) {
	body, err := newNotificationWebhookRequest(w)
	if err != nil {
		return nil, nil, err
	}
	created := new(NotificationWebhook)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/notification_webhooks/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// Update replaces the writable fields of the notification webhook with the
// given ID. Every writable field is sent: an empty HeaderName or HeaderValue
// clears it, and a zero Owner makes it a system webhook. Update a webhook
// returned by [NotificationWebhooksService.Get] to keep its other fields, or
// use [NotificationWebhooksService.Patch].
//
// DefectDojo API: PUT /api/v2/notification_webhooks/{id}/
func (s *NotificationWebhooksService) Update(ctx context.Context, id int, w *NotificationWebhook, opts ...RequestOption) (*NotificationWebhook, *Response, error) {
	if err := requirePositive("notification webhook ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newNotificationWebhookRequest(w)
	if err != nil {
		return nil, nil, err
	}
	updated := new(NotificationWebhook)
	resp, err := s.client.do(ctx, http.MethodPut, notificationWebhookPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Patch changes the fields of the notification webhook that are set in
// patch.
//
// DefectDojo API: PATCH /api/v2/notification_webhooks/{id}/
func (s *NotificationWebhooksService) Patch(ctx context.Context, id int, patch *NotificationWebhookPatch, opts ...RequestOption) (*NotificationWebhook, *Response, error) {
	if err := requirePositive("notification webhook ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: notification webhook patch must not be nil")
	}
	updated := new(NotificationWebhook)
	resp, err := s.client.do(ctx, http.MethodPatch, notificationWebhookPath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Delete deletes a notification webhook by its ID.
//
// DefectDojo API: DELETE /api/v2/notification_webhooks/{id}/
func (s *NotificationWebhooksService) Delete(ctx context.Context, id int, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("notification webhook ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, notificationWebhookPath(id), nil, nil, opts)
}

// DeletePreview returns a page of the objects that deleting the notification
// webhook would remove, the webhook itself included.
//
// DefectDojo API: GET /api/v2/notification_webhooks/{id}/delete_preview/
func (s *NotificationWebhooksService) DeletePreview(ctx context.Context, id int, opt *ListOptions, opts ...RequestOption) (*DeletePreviewList, *Response, error) {
	if err := requirePositive("notification webhook ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(notificationWebhookPath(id)+"delete_preview/", opt)
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

func notificationWebhookPath(id int) string {
	return "/api/v2/notification_webhooks/" + strconv.Itoa(id) + "/"
}

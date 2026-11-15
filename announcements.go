package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// AnnouncementsService handles communication with the announcement methods of
// the DefectDojo API. The announcement is a banner that DefectDojo shows on
// every page to every user who hasn't dismissed it.
//
// DefectDojo keeps at most one announcement, and it always has the ID 1.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type AnnouncementsService service

// The styles of the announcement banner. DefectDojo checks the style.
const (
	AnnouncementStyleInfo    = "info"
	AnnouncementStyleSuccess = "success"
	AnnouncementStyleWarning = "warning"
	AnnouncementStyleDanger  = "danger"
)

// Announcement is the DefectDojo announcement. The same type is used for
// reads and writes: the read-only ID is ignored when writing.
type Announcement struct {
	// ID is always 1.
	ID int `json:"id"`

	// Message is at most 500 characters and may contain basic HTML, such as
	// links.
	Message string `json:"message"`

	// Style is a banner style, such as the AnnouncementStyle constants.
	// DefectDojo defaults it to AnnouncementStyleInfo.
	Style string `json:"style"`

	// Dismissable lets users dismiss the announcement.
	Dismissable bool `json:"dismissable"`
}

// announcementRequest is the body of announcement create and update requests:
// the writable fields of an [Announcement].
type announcementRequest struct {
	Message string `json:"message"`
	// Style is left out when empty, for DefectDojo's default; "" is a 400.
	Style       string `json:"style,omitempty"`
	Dismissable bool   `json:"dismissable"`
}

// newAnnouncementRequest validates an announcement and returns its writable
// fields.
func newAnnouncementRequest(a *Announcement) (*announcementRequest, error) {
	if a == nil {
		return nil, errors.New("defectdojo: announcement must not be nil")
	}
	if a.Message == "" {
		return nil, errors.New("defectdojo: announcement Message must not be empty")
	}
	return &announcementRequest{Message: a.Message, Style: a.Style, Dismissable: a.Dismissable}, nil
}

// AnnouncementPatch is the body of [AnnouncementsService.Patch]. Only the
// fields that are set are changed.
type AnnouncementPatch struct {
	Message     *string `json:"message,omitempty"`
	Style       *string `json:"style,omitempty"`
	Dismissable *bool   `json:"dismissable,omitempty"`
}

// AnnouncementList is a page of announcements: the announcement, or none.
type AnnouncementList struct {
	Announcements []*Announcement

	// Count is the number of announcements, 0 or 1.
	Count int
}

// List returns a page of announcements. DefectDojo keeps at most one, so
// [AnnouncementsService.Current] is usually simpler.
//
// DefectDojo API: GET /api/v2/announcements/
func (s *AnnouncementsService) List(ctx context.Context, opt *ListOptions, opts ...RequestOption) (*AnnouncementList, *Response, error) {
	path, err := addQuery("/api/v2/announcements/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[*Announcement]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &AnnouncementList{Announcements: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the announcements, starting at opt.Offset.
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *AnnouncementsService) ListAll(ctx context.Context, opt *ListOptions, opts ...RequestOption) iter.Seq2[*Announcement, error] {
	base := ListOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*Announcement, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return result.Announcements, result.Count, nil
	})
}

// Current returns the announcement. It returns an error matching
// [ErrNotFound] when there is none.
func (s *AnnouncementsService) Current(ctx context.Context, opts ...RequestOption) (*Announcement, *Response, error) {
	list, resp, err := s.List(ctx, nil, opts...)
	if err != nil {
		return nil, resp, err
	}
	if len(list.Announcements) == 0 {
		return nil, resp, fmt.Errorf("defectdojo: announcement: %w", ErrNotFound)
	}
	return list.Announcements[0], resp, nil
}

// Get returns an announcement by its ID, which is always 1.
//
// DefectDojo API: GET /api/v2/announcements/{id}/
func (s *AnnouncementsService) Get(ctx context.Context, id int, opts ...RequestOption) (*Announcement, *Response, error) {
	if err := requirePositive("announcement ID", id); err != nil {
		return nil, nil, err
	}
	a := new(Announcement)
	resp, err := s.client.do(ctx, http.MethodGet, announcementPath(id), nil, a, opts)
	if err != nil {
		return nil, resp, err
	}
	return a, resp, nil
}

// Create creates the announcement, which gets the ID 1 and is shown to every
// user. Its read-only ID is ignored. When an announcement exists, DefectDojo
// answers with a 400 ([ErrBadRequest]), "No more than one Announcement is
// allowed".
//
// An empty Style gets DefectDojo's default, AnnouncementStyleInfo.
//
// DefectDojo API: POST /api/v2/announcements/
func (s *AnnouncementsService) Create(ctx context.Context, a *Announcement, opts ...RequestOption) (*Announcement, *Response, error) {
	body, err := newAnnouncementRequest(a)
	if err != nil {
		return nil, nil, err
	}
	created := new(Announcement)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/announcements/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// Update replaces the writable fields of the announcement with the given ID.
// An empty Style keeps the current one.
//
// Users who dismissed the announcement don't see the change (until their
// user record is saved, which gives it back to them). To show it to every
// user, delete the announcement and create a new one.
//
// DefectDojo API: PUT /api/v2/announcements/{id}/
func (s *AnnouncementsService) Update(ctx context.Context, id int, a *Announcement, opts ...RequestOption) (*Announcement, *Response, error) {
	if err := requirePositive("announcement ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newAnnouncementRequest(a)
	if err != nil {
		return nil, nil, err
	}
	updated := new(Announcement)
	resp, err := s.client.do(ctx, http.MethodPut, announcementPath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Patch changes the fields of the announcement that are set in patch. An
// empty Message is a 400.
//
// As with [AnnouncementsService.Update], users who dismissed the announcement
// don't see the change.
//
// DefectDojo API: PATCH /api/v2/announcements/{id}/
func (s *AnnouncementsService) Patch(ctx context.Context, id int, patch *AnnouncementPatch, opts ...RequestOption) (*Announcement, *Response, error) {
	if err := requirePositive("announcement ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: announcement patch must not be nil")
	}
	updated := new(Announcement)
	resp, err := s.client.do(ctx, http.MethodPatch, announcementPath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// Delete deletes an announcement by its ID, removing the banner for every
// user.
//
// DefectDojo API: DELETE /api/v2/announcements/{id}/
func (s *AnnouncementsService) Delete(ctx context.Context, id int, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("announcement ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, announcementPath(id), nil, nil, opts)
}

// DeletePreview returns a page of the objects that deleting the announcement
// would remove: the announcement and the copy of each user who hasn't
// dismissed it ("UserAnnouncement").
//
// DefectDojo API: GET /api/v2/announcements/{id}/delete_preview/
func (s *AnnouncementsService) DeletePreview(ctx context.Context, id int, opt *ListOptions, opts ...RequestOption) (*DeletePreviewList, *Response, error) {
	if err := requirePositive("announcement ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(announcementPath(id)+"delete_preview/", opt)
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

func announcementPath(id int) string {
	return "/api/v2/announcements/" + strconv.Itoa(id) + "/"
}

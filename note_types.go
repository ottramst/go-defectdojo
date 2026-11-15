package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// NoteTypesService handles communication with the note type methods of the
// DefectDojo API. Note types classify the notes on findings, tests and
// engagements.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type NoteTypesService service

// NoteType is a DefectDojo note type. The same type is used for reads and
// writes: the read-only ID is ignored when writing.
type NoteType struct {
	_ [0]func()

	ID int `json:"id"`

	// Name is unique across all note types, at most 100 characters.
	Name string `json:"name"`

	// Description is required, at most 200 characters.
	Description string `json:"description"`

	// IsSingle allows only one note of the type per object.
	IsSingle bool `json:"is_single"`

	// IsActive offers the type for new notes. DefectDojo defaults it to true.
	IsActive bool `json:"is_active"`

	// IsMandatory, on an active type, keeps the UI from closing a finding,
	// or setting it inactive or false positive, until it has a note of the
	// type. DefectDojo defaults it to true.
	IsMandatory bool `json:"is_mandatory"`
}

// noteTypeRequest is the body of note type create and update requests: the
// writable fields of a [NoteType].
type noteTypeRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsSingle    bool   `json:"is_single"`
	IsActive    bool   `json:"is_active"`
	IsMandatory bool   `json:"is_mandatory"`
}

// newNoteTypeRequest validates a note type and returns its writable fields.
func newNoteTypeRequest(nt *NoteType) (*noteTypeRequest, error) {
	if nt == nil {
		return nil, errors.New("defectdojo: note type must not be nil")
	}
	switch {
	case nt.Name == "":
		return nil, errors.New("defectdojo: note type Name must not be empty")
	case nt.Description == "":
		return nil, errors.New("defectdojo: note type Description must not be empty")
	}
	return &noteTypeRequest{
		Name:        nt.Name,
		Description: nt.Description,
		IsSingle:    nt.IsSingle,
		IsActive:    nt.IsActive,
		IsMandatory: nt.IsMandatory,
	}, nil
}

// NoteTypePatch is the body of [NoteTypesService.Patch]. Only the fields that
// are set are changed.
type NoteTypePatch struct {
	_ [0]func()

	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsSingle    *bool   `json:"is_single,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
	IsMandatory *bool   `json:"is_mandatory,omitempty"`
}

// ListNoteTypesOptions are the options of [NoteTypesService.List] and
// [NoteTypesService.ListAll].
type ListNoteTypesOptions struct {
	_ [0]func()

	ListOptions

	// Name filters by exact, case-sensitive name.
	Name string `url:"name,omitempty"`

	IsSingle    *bool `url:"is_single,omitempty"`
	IsActive    *bool `url:"is_active,omitempty"`
	IsMandatory *bool `url:"is_mandatory,omitempty"`
}

// ListNoteTypesResult is a page of note types.
type ListNoteTypesResult struct {
	_ [0]func()

	Results []NoteType

	// Count is the number of note types that match the filters.
	Count int
}

// List returns a page of note types, ordered by ID.
//
// DefectDojo API: GET /api/v2/note_type/
func (s *NoteTypesService) List(ctx context.Context, opt *ListNoteTypesOptions, opts ...RequestOption) (*ListNoteTypesResult, *Response, error) {
	path, err := addQuery("/api/v2/note_type/", opt)
	if err != nil {
		return nil, nil, err
	}
	var page listPage[NoteType]
	resp, err := s.client.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		return nil, resp, err
	}
	return &ListNoteTypesResult{Results: page.Results, Count: page.Count}, resp, nil
}

// ListAll returns an iterator over the note types, starting at opt.Offset. It
// fetches opt.Limit note types per request (100 if zero).
//
// Ranging over the iterator again starts over. On an error, the iterator
// yields it once and stops.
func (s *NoteTypesService) ListAll(ctx context.Context, opt *ListNoteTypesOptions, opts ...RequestOption) iter.Seq2[*NoteType, error] {
	base := ListNoteTypesOptions{}
	if opt != nil {
		base = *opt
	}
	return paginate(ctx, base.Offset, base.Limit, func(ctx context.Context, from, size int) ([]*NoteType, int, error) {
		req := base
		req.Offset, req.Limit = from, size
		result, _, err := s.List(ctx, &req, opts...)
		if err != nil {
			return nil, 0, err
		}
		return pointers(result.Results), result.Count, nil
	})
}

// GetNoteTypeOptions are the options of [NoteTypesService.Get].
type GetNoteTypeOptions struct {
	_ [0]func()

	_ [0]func()
}

// Get returns a note type by its ID.
//
// DefectDojo API: GET /api/v2/note_type/{id}/
func (s *NoteTypesService) Get(ctx context.Context, id int, opt *GetNoteTypeOptions, opts ...RequestOption) (*NoteType, *Response, error) {
	if err := requirePositive("note type ID", id); err != nil {
		return nil, nil, err
	}
	nt := new(NoteType)
	resp, err := s.client.do(ctx, http.MethodGet, noteTypePath(id), nil, nt, opts)
	if err != nil {
		return nil, resp, err
	}
	return nt, resp, nil
}

// GetNoteTypeByNameOptions are the options of [NoteTypesService.GetByName].
type GetNoteTypeByNameOptions struct {
	_ [0]func()

	_ [0]func()
}

// GetByName returns the note type with exactly the given name. It returns an
// error matching [ErrNotFound] when there is none.
func (s *NoteTypesService) GetByName(ctx context.Context, name string, opt *GetNoteTypeByNameOptions, opts ...RequestOption) (*NoteType, *Response, error) {
	if name == "" {
		return nil, nil, errors.New("defectdojo: note type name must not be empty")
	}
	list, resp, err := s.List(ctx, &ListNoteTypesOptions{Name: name}, opts...)
	if err != nil {
		return nil, resp, err
	}
	// DefectDojo strips whitespace from the filter, and may loosen it: check
	// the name again.
	for i := range list.Results {
		nt := &list.Results[i]
		if nt.Name == name {
			return nt, resp, nil
		}
	}
	return nil, resp, fmt.Errorf("defectdojo: note type %q: %w", name, ErrNotFound)
}

// CreateNoteTypeOptions are the options of [NoteTypesService.Create].
type CreateNoteTypeOptions struct {
	_ [0]func()

	_ [0]func()
}

// Create creates a note type. Its read-only ID is ignored. A name that is
// taken is a 400 ([ErrBadRequest]) with a "name" field error.
//
// Every writable field is sent, so IsActive and IsMandatory, which DefectDojo
// defaults to true, are false unless they are set.
//
// DefectDojo API: POST /api/v2/note_type/
func (s *NoteTypesService) Create(ctx context.Context, nt *NoteType, opt *CreateNoteTypeOptions, opts ...RequestOption) (*NoteType, *Response, error) {
	body, err := newNoteTypeRequest(nt)
	if err != nil {
		return nil, nil, err
	}
	created := new(NoteType)
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/note_type/", body, created, opts)
	if err != nil {
		return nil, resp, err
	}
	return created, resp, nil
}

// UpdateNoteTypeOptions are the options of [NoteTypesService.Update].
type UpdateNoteTypeOptions struct {
	_ [0]func()

	_ [0]func()
}

// Update replaces the writable fields of the note type with the given ID.
// Every writable field is sent.
//
// DefectDojo API: PUT /api/v2/note_type/{id}/
func (s *NoteTypesService) Update(ctx context.Context, id int, nt *NoteType, opt *UpdateNoteTypeOptions, opts ...RequestOption) (*NoteType, *Response, error) {
	if err := requirePositive("note type ID", id); err != nil {
		return nil, nil, err
	}
	body, err := newNoteTypeRequest(nt)
	if err != nil {
		return nil, nil, err
	}
	updated := new(NoteType)
	resp, err := s.client.do(ctx, http.MethodPut, noteTypePath(id), body, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// PatchNoteTypeOptions are the options of [NoteTypesService.Patch].
type PatchNoteTypeOptions struct {
	_ [0]func()

	_ [0]func()
}

// Patch changes the fields of the note type that are set in patch.
//
// DefectDojo API: PATCH /api/v2/note_type/{id}/
func (s *NoteTypesService) Patch(ctx context.Context, id int, patch *NoteTypePatch, opt *PatchNoteTypeOptions, opts ...RequestOption) (*NoteType, *Response, error) {
	if err := requirePositive("note type ID", id); err != nil {
		return nil, nil, err
	}
	if patch == nil {
		return nil, nil, errors.New("defectdojo: note type patch must not be nil")
	}
	updated := new(NoteType)
	resp, err := s.client.do(ctx, http.MethodPatch, noteTypePath(id), patch, updated, opts)
	if err != nil {
		return nil, resp, err
	}
	return updated, resp, nil
}

// DeleteNoteTypeOptions are the options of [NoteTypesService.Delete].
type DeleteNoteTypeOptions struct {
	_ [0]func()

	_ [0]func()
}

// Delete deletes a note type by its ID.
//
// Deleting a note type also deletes every note of the type, on findings,
// tests, engagements and elsewhere. Use [NoteTypesService.CountNotes] first to
// see how many.
//
// DefectDojo API: DELETE /api/v2/note_type/{id}/
func (s *NoteTypesService) Delete(ctx context.Context, id int, opt *DeleteNoteTypeOptions, opts ...RequestOption) (*Response, error) {
	if err := requirePositive("note type ID", id); err != nil {
		return nil, err
	}
	return s.client.do(ctx, http.MethodDelete, noteTypePath(id), nil, nil, opts)
}

// DeletePreviewNoteTypeOptions are the options of [NoteTypesService.DeletePreview].
type DeletePreviewNoteTypeOptions struct {
	_ [0]func()

	ListOptions
}

// DeletePreview returns a page of the objects that deleting the note type
// would remove: the note type itself, its notes ("Notes"), the notes' links
// to their history ("Notes_history") and to findings, tests and engagements
// ("Test_notes" and the like), and history rows of the type ("NoteHistory",
// from UI edits; they can belong to notes of another type).
//
// DefectDojo API: GET /api/v2/note_type/{id}/delete_preview/
func (s *NoteTypesService) DeletePreview(ctx context.Context, id int, opt *DeletePreviewNoteTypeOptions, opts ...RequestOption) (*DeletePreviewResult, *Response, error) {
	if err := requirePositive("note type ID", id); err != nil {
		return nil, nil, err
	}
	path, err := addQuery(noteTypePath(id)+"delete_preview/", opt)
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

// CountNoteTypeNotesOptions are the options of [NoteTypesService.CountNotes].
type CountNoteTypeNotesOptions struct {
	_ [0]func()

	_ [0]func()
}

// CountNotes returns the number of notes of the note type, which a delete of
// the note type would also delete.
//
// DefectDojo can't filter notes by type, so CountNotes pages through the
// delete preview (GET /api/v2/note_type/{id}/delete_preview/) and counts its
// "Notes" entries; the preview also lists the note type, the notes' history
// and their links.
func (s *NoteTypesService) CountNotes(ctx context.Context, id int, opt *CountNoteTypeNotesOptions, opts ...RequestOption) (int, *Response, error) {
	if err := requirePositive("note type ID", id); err != nil {
		return 0, nil, err
	}
	// DefectDojo 3.4 builds the whole preview for every page, so ask for
	// large pages; 3.5 caps them at 250.
	listOpt := &DeletePreviewNoteTypeOptions{ListOptions: ListOptions{Limit: 250}}
	count := 0
	for {
		page, resp, err := s.DeletePreview(ctx, id, listOpt, opts...)
		if err != nil {
			return 0, resp, err
		}
		for i := range page.Results {
			o := &page.Results[i]
			if o.Model == "Notes" {
				count++
			}
		}
		listOpt.Offset += len(page.Results)
		if len(page.Results) == 0 || listOpt.Offset >= page.Count {
			return count, resp, nil
		}
	}
}

func noteTypePath(id int) string {
	return "/api/v2/note_type/" + strconv.Itoa(id) + "/"
}

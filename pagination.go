package defectdojo

import (
	"context"
	"iter"
)

// defaultPageSize is the page size of the iterators when none is given.
const defaultPageSize = 100

// ListOptions are the pagination options of list endpoints. DefectDojo
// returns 25 items per page by default.
type ListOptions struct {
	// Limit is the number of items per page.
	Limit int `url:"limit,omitempty"`

	// Offset is the number of items to skip.
	Offset int `url:"offset,omitempty"`
}

// listPage is the JSON form of a page of a list endpoint.
type listPage[T any] struct {
	Count   int    `json:"count"`
	Next    string `json:"next"`
	Results []T    `json:"results"`
}

// pageFunc fetches the page of size items that starts at offset from, and
// returns the items and the total number of items.
type pageFunc[T any] func(ctx context.Context, from, size int) (items []T, total int, err error)

// paginate returns an iterator over all items of an offset-paginated
// endpoint, starting at offset from. Pages are fetched lazily, and ranging
// over the iterator again starts over.
//
// The iterator yields an error, with the zero item, when a page fails or ctx
// is done, and then stops. Offset pagination can skip or repeat items when
// they change while iterating.
func paginate[T any](ctx context.Context, from, size int, fetch pageFunc[T]) iter.Seq2[T, error] {
	if size <= 0 {
		size = defaultPageSize
	}
	return func(yield func(T, error) bool) {
		var zero T
		for offset := from; ; {
			if err := ctx.Err(); err != nil {
				yield(zero, err)
				return
			}
			items, total, err := fetch(ctx, offset, size)
			if err != nil {
				yield(zero, err)
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			offset += len(items)
			// An empty page also ends the iteration, in case the total is
			// wrong, to avoid looping forever.
			if len(items) == 0 || offset >= total {
				return
			}
		}
	}
}

// pointers returns pointers to the elements of s, for iterators over a page
// whose results are values.
func pointers[T any](s []T) []*T {
	p := make([]*T, len(s))
	for i := range s {
		p[i] = &s[i]
	}
	return p
}

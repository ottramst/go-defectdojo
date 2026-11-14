package defectdojo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-querystring/query"
)

// A RequestOption customizes a single API request. Options are applied after
// the client has set its headers, so they can override them.
type RequestOption func(*http.Request) error

// WithHeader sets a request header, replacing any value the client set.
func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) error {
		req.Header.Set(key, value)
		return nil
	}
}

// WithQueryParam adds a query parameter to the request URL. Existing query
// parameters are kept as they are.
func WithQueryParam(key, value string) RequestOption {
	return func(req *http.Request) error {
		param := url.QueryEscape(key) + "=" + url.QueryEscape(value)
		if req.URL.RawQuery == "" {
			req.URL.RawQuery = param
		} else {
			req.URL.RawQuery += "&" + param
		}
		return nil
	}
}

// addQuery appends the query parameters encoded from the url struct tags of
// opt to path. A nil opt leaves path unchanged.
func addQuery(path string, opt any) (string, error) {
	if opt == nil {
		return path, nil
	}
	v, err := query.Values(opt)
	if err != nil {
		return "", fmt.Errorf("defectdojo: encoding query parameters: %w", err)
	}
	if len(v) == 0 {
		return path, nil
	}
	return path + "?" + v.Encode(), nil
}

// requirePositive returns an error if a required ID is not positive, so
// that no request is sent to a list endpoint instead of the object.
func requirePositive(name string, value int) error {
	if value <= 0 {
		return fmt.Errorf("defectdojo: %s must be positive, got %d", name, value)
	}
	return nil
}

// validEscapedQuery reports whether q contains no characters that must be
// escaped in a URL query: spaces, control characters and non-ASCII.
func validEscapedQuery(q string) bool {
	for i := 0; i < len(q); i++ {
		if q[i] <= ' ' || q[i] >= 0x7f {
			return false
		}
	}
	return true
}

// validEscapedPath reports whether p contains only characters that may
// appear in an escaped URL path, so that escaped segments such as %2F are
// kept as they are.
func validEscapedPath(p string) bool {
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case strings.IndexByte("-._~!$&'()*+,;=:@/%", c) >= 0:
		default:
			return false
		}
	}
	return true
}

// orNull returns a pointer to v, or nil (sent as null) if v is the zero
// value. Requests send the zero values of nullable fields as null.
func orNull[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

// orEmpty returns s, or an empty slice if s is nil. DefectDojo rejects null
// for lists; an empty list removes all entries.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// orEmptyPtr returns p, or a pointer to an empty slice if p points to a nil
// slice. It never changes *p.
func orEmptyPtr[T any](p *[]T) *[]T {
	if p != nil && *p == nil {
		return &[]T{}
	}
	return p
}

// count returns the count of a list filtered by the ID of a parent object,
// read with limit=1. Filters on foreign keys reject IDs that don't exist with
// a 400 field error on the filter: the error then also matches
// [ErrNotFound].
func (c *Client) count(ctx context.Context, path, filter string, opts []RequestOption) (int, *Response, error) {
	var page listPage[struct{}]
	resp, err := c.do(ctx, http.MethodGet, path, nil, &page, opts)
	if err != nil {
		var errResp *ErrorResponse
		if errors.As(err, &errResp) && errResp.StatusCode == http.StatusBadRequest && len(errResp.Fields[filter]) > 0 {
			return 0, resp, fmt.Errorf("%w: %w", ErrNotFound, err)
		}
		return 0, resp, err
	}
	return page.Count, resp, nil
}

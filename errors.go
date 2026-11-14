package defectdojo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"
)

// Sentinel errors for common API failures. An [*ErrorResponse] matches the
// sentinel for its status code with [errors.Is]:
//
//	if errors.Is(err, defectdojo.ErrNotFound) {
//		// the product type does not exist
//	}
//
// DefectDojo answers a missing or invalid token with 403 Forbidden, not 401,
// so check for [ErrForbidden] when authentication fails.
var (
	ErrBadRequest      = errors.New("defectdojo: bad request")
	ErrUnauthorized    = errors.New("defectdojo: unauthorized")
	ErrForbidden       = errors.New("defectdojo: forbidden")
	ErrNotFound        = errors.New("defectdojo: not found")
	ErrConflict        = errors.New("defectdojo: conflict")
	ErrTooManyRequests = errors.New("defectdojo: too many requests")
)

const (
	// maxErrorBodySize limits how much of an error response body is read.
	maxErrorBodySize = 1 << 20

	// maxTextMessageLength limits the message taken from a body that is not
	// a DefectDojo error, such as the HTML error page of a proxy.
	maxTextMessageLength = 512
)

// ErrorResponse is returned for API responses with a non-2xx status code.
// DefectDojo error bodies look like one of:
//
//	{"detail": "No Product_Type matches the given query."}
//	{"name": ["product_ type with this name already exists."], "message": "name: product_ type with this name already exists."}
type ErrorResponse struct {
	// Response is the HTTP response. Its body has already been read into Body.
	Response *http.Response

	// StatusCode is the HTTP status code of the response.
	StatusCode int

	// Message describes the error: the "message" field of the body, or else
	// its "detail" field.
	Message string

	// Fields holds the validation errors of a 400 Bad Request response, by
	// field name. Errors that concern no single field are under
	// "non_field_errors".
	Fields map[string][]string

	// Body is the raw response body (up to 1 MiB).
	Body []byte
}

func (e *ErrorResponse) Error() string {
	msg := e.Message
	if msg == "" && len(e.Fields) > 0 {
		var parts []string
		for _, k := range slices.Sorted(maps.Keys(e.Fields)) {
			parts = append(parts, k+": "+strings.Join(e.Fields[k], " "))
		}
		msg = strings.Join(parts, "; ")
	}
	if msg == "" && e.Response != nil {
		if loc := e.Response.Header.Get("Location"); loc != "" {
			msg = "redirected to " + loc
		}
	}
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	if e.Response != nil && e.Response.Request != nil {
		req := e.Response.Request
		u := *req.URL
		u.User = nil
		return fmt.Sprintf("defectdojo: %s %s: %d %s", req.Method, u.Redacted(), e.StatusCode, msg)
	}
	return fmt.Sprintf("defectdojo: %d %s", e.StatusCode, msg)
}

// Is reports whether target is the sentinel error for the response's status
// code, such as [ErrNotFound] for 404 Not Found.
func (e *ErrorResponse) Is(target error) bool {
	return e != nil && statusIs(e.StatusCode, target)
}

// statusIs reports whether target is the sentinel error for the HTTP status
// code. It compares the target by identity, like errors.Is does, so that
// targets of any type, including ones that cannot be hashed, are safe.
//
//nolint:errorlint // Is methods compare targets by identity.
func statusIs(code int, target error) bool {
	switch target {
	case ErrBadRequest:
		return code == http.StatusBadRequest
	case ErrUnauthorized:
		return code == http.StatusUnauthorized
	case ErrForbidden:
		return code == http.StatusForbidden
	case ErrNotFound:
		return code == http.StatusNotFound
	case ErrConflict:
		return code == http.StatusConflict
	case ErrTooManyRequests:
		return code == http.StatusTooManyRequests
	}
	return false
}

// CheckResponse returns an [*ErrorResponse] if the response has a non-2xx
// status code, and nil otherwise. It reads the body of error responses.
func CheckResponse(r *http.Response) error {
	if r.StatusCode >= 200 && r.StatusCode <= 299 {
		return nil
	}

	errResp := &ErrorResponse{Response: r, StatusCode: r.StatusCode}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxErrorBodySize))
	if err == nil && len(data) > 0 {
		errResp.Body = data
		parseErrorBody(errResp, data)
	}
	return errResp
}

// ignoredErrorFields are keys of error bodies that are not field errors.
var ignoredErrorFields = map[string]bool{
	"message": true,
	"detail":  true,
	// An advertisement for DefectDojo Pro that 400 responses carry.
	"pro": true,
}

// parseErrorBody sets the fields of e from a DefectDojo error body. Fields of
// an unexpected type are skipped. A body that is not a JSON object, such as
// the error page of a proxy, becomes the message, shortened.
func parseErrorBody(e *ErrorResponse, data []byte) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		e.Message = truncate(strings.TrimSpace(string(data)), maxTextMessageLength)
		return
	}
	var message, detail string
	if v, ok := fields["message"]; ok {
		_ = json.Unmarshal(v, &message)
	}
	if v, ok := fields["detail"]; ok {
		_ = json.Unmarshal(v, &detail)
	}
	e.Message = message
	if e.Message == "" {
		e.Message = detail
	}
	for k, v := range fields {
		if ignoredErrorFields[k] {
			continue
		}
		var msgs []string
		if json.Unmarshal(v, &msgs) != nil {
			var msg string
			if json.Unmarshal(v, &msg) != nil {
				continue
			}
			msgs = []string{msg}
		}
		if e.Fields == nil {
			e.Fields = make(map[string][]string)
		}
		e.Fields[k] = msgs
	}
}

// truncate shortens s to at most n bytes without splitting a UTF-8 sequence.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}

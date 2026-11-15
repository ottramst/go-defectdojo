package defectdojo_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ottramst/go-defectdojo"
)

func TestErrorResponse(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		sentinel    error
		wantMessage string
		wantFields  map[string][]string
	}{
		{name: "validation", status: 400, body: string(fixture(t, "errors/validation.json")), sentinel: defectdojo.ErrBadRequest,
			wantMessage: "name: product_ type with this name already exists.",
			wantFields:  map[string][]string{"name": {"product_ type with this name already exists."}}},
		{name: "validation without message", status: 400, body: `{"non_field_errors":["a","b"],"detail":"x is invalid"}`, sentinel: defectdojo.ErrBadRequest,
			wantMessage: "x is invalid", wantFields: map[string][]string{"non_field_errors": {"a", "b"}}},
		{name: "string field error", status: 400, body: `{"product":"Invalid pk"}`, sentinel: defectdojo.ErrBadRequest,
			wantFields: map[string][]string{"product": {"Invalid pk"}}},
		{name: "unauthorized", status: 401, body: `{"detail":"Authentication credentials were not provided."}`,
			sentinel: defectdojo.ErrUnauthorized, wantMessage: "Authentication credentials were not provided."},
		{name: "invalid token", status: 403, body: string(fixture(t, "errors/invalid_token.json")),
			sentinel: defectdojo.ErrForbidden, wantMessage: "Invalid token."},
		{name: "not found", status: 404, body: string(fixture(t, "errors/not_found.json")),
			sentinel: defectdojo.ErrNotFound, wantMessage: "No Product_Type matches the given query."},
		{name: "conflict", status: 409, body: `{"message":"Cannot delete: 2 products use this SLA configuration."}`,
			sentinel: defectdojo.ErrConflict, wantMessage: "Cannot delete: 2 products use this SLA configuration."},
		{name: "read-only mode", status: 423, body: string(fixture(t, "errors/read_only_mode.json")), sentinel: defectdojo.ErrLocked,
			wantMessage: "Read-only mode is on. DefectDojo does not accept changes until a superuser turns it off in System Settings."},
		{name: "too many requests", status: 429, body: `{"detail":"Request was throttled."}`,
			sentinel: defectdojo.ErrTooManyRequests, wantMessage: "Request was throttled."},
		{name: "plain text body", status: 502, body: "Bad Gateway\n", wantMessage: "Bad Gateway"},
		{name: "unexpected field types", status: 400, body: `{"message":{"x":1},"count":3}`, sentinel: defectdojo.ErrBadRequest},
		// Some views answer with a bare JSON string or list of strings.
		{name: "JSON string body", status: 400, body: `"Users may not delete themselves"`, sentinel: defectdojo.ErrBadRequest,
			wantMessage: "Users may not delete themselves"},
		{name: "JSON string list body", status: 400, body: `["Resetting your own API token via this endpoint is not allowed.","x"]`,
			sentinel: defectdojo.ErrBadRequest, wantMessage: "Resetting your own API token via this endpoint is not allowed. x"},
		{name: "other JSON array body", status: 500, body: `[1]`, wantMessage: `[1]`},
		{name: "empty body", status: 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /api/v2/product_types/7/", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.status, []byte(tt.body))
			})

			_, resp, err := client.ProductTypes.Get(context.Background(), 7)
			var errResp *defectdojo.ErrorResponse
			if !errors.As(err, &errResp) {
				t.Fatalf("error = %v (%T), want *defectdojo.ErrorResponse", err, err)
			}
			if resp == nil || resp.StatusCode != tt.status {
				t.Errorf("Response = %v, want status %d", resp, tt.status)
			}
			if errResp.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", errResp.StatusCode, tt.status)
			}
			if errResp.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", errResp.Message, tt.wantMessage)
			}
			if !reflect.DeepEqual(errResp.Fields, tt.wantFields) {
				t.Errorf("Fields = %v, want %v", errResp.Fields, tt.wantFields)
			}
			if string(errResp.Body) != tt.body {
				t.Errorf("Body = %q, want %q", errResp.Body, tt.body)
			}
			if tt.sentinel != nil && !errors.Is(err, tt.sentinel) {
				t.Errorf("errors.Is(err, %v) = false, want true", tt.sentinel)
			}
			for _, other := range []error{defectdojo.ErrBadRequest, defectdojo.ErrUnauthorized, defectdojo.ErrForbidden, defectdojo.ErrNotFound, defectdojo.ErrConflict, defectdojo.ErrLocked, defectdojo.ErrTooManyRequests} {
				if errors.Is(other, tt.sentinel) {
					continue
				}
				if errors.Is(err, other) {
					t.Errorf("errors.Is(err, %v) = true, want false", other)
				}
			}
			if msg := err.Error(); !strings.Contains(msg, "GET ") || !strings.Contains(msg, "/api/v2/product_types/7/") {
				t.Errorf("Error() = %q, want method and URL", msg)
			}
		})
	}
}

func TestErrorResponse_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *defectdojo.ErrorResponse
		want string
	}{
		{name: "message", err: &defectdojo.ErrorResponse{StatusCode: 404, Message: "gone"}, want: "defectdojo: 404 gone"},
		{name: "fields, sorted", err: &defectdojo.ErrorResponse{StatusCode: 400, Fields: map[string][]string{"name": {"blank"}, "description": {"too", "long"}}},
			want: "defectdojo: 400 description: too long; name: blank"},
		{name: "status text", err: &defectdojo.ErrorResponse{StatusCode: 503}, want: "defectdojo: 503 Service Unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorResponse_redactsUserInfo(t *testing.T) {
	mux, base := setup(t)
	mux.HandleFunc("GET /api/v2/product_types/1/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusNotFound, []byte(notFound))
	})
	u := base.BaseURL()
	u.User = url.UserPassword("user", "secret")
	client, err := defectdojo.NewClient(u.String())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = client.ProductTypes.Get(context.Background(), 1)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Errorf("Error() = %v, want an error without the password", err)
	}
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("error = %v, want an *ErrorResponse", err)
	}
	if errResp.Method != http.MethodGet || !strings.HasSuffix(errResp.URL, "/api/v2/product_types/1/") || strings.Contains(errResp.URL, "user") {
		t.Errorf("Method, URL = %q, %q; want GET and the URL without user info", errResp.Method, errResp.URL)
	}
}

// The error holds no reference to the request, whose Authorization header
// holds the token: printing it whole must not reveal the token.
func TestErrorResponse_holdsNoCredentials(t *testing.T) {
	mux, base := setup(t)
	mux.HandleFunc("GET /api/v2/product_types/1/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "abc")
		writeJSON(t, w, http.StatusNotFound, []byte(notFound))
	})
	client, err := defectdojo.NewClient(base.BaseURL().String(), defectdojo.WithAPIToken("token-secret"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = client.ProductTypes.Get(context.Background(), 1)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("error = %v, want an *ErrorResponse", err)
	}
	if dump := fmt.Sprintf("%#v", errResp); strings.Contains(dump, "token-secret") {
		t.Errorf("%%#v of the error holds the token: %s", dump)
	}
	if errResp.Header.Get("X-Request-Id") != "abc" {
		t.Errorf("Header = %v, want the response headers", errResp.Header)
	}
}

func TestErrorResponse_longTextIsTruncated(t *testing.T) {
	mux, client := setup(t)
	body := strings.Repeat("é", 1000)
	mux.HandleFunc("GET /api/v2/product_types/1/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(body))
	})
	_, _, err := client.ProductTypes.Get(context.Background(), 1)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("error = %v, want *ErrorResponse", err)
	}
	if len(errResp.Message) > 520 || !utf8.ValidString(errResp.Message) || !strings.HasSuffix(errResp.Message, "…") {
		t.Errorf("Message has %d bytes, want a valid UTF-8 string of about 512 bytes ending in …", len(errResp.Message))
	}
	if string(errResp.Body) != body {
		t.Error("Body is not the whole body")
	}
}

package defectdojo_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
		wantErr bool
	}{
		{baseURL: "http://localhost:8080", want: "http://localhost:8080"},
		{baseURL: "https://defectdojo.example.com/", want: "https://defectdojo.example.com"},
		{baseURL: "https://example.com/dojo/", want: "https://example.com/dojo"},
		{baseURL: "https://example.com/dojo?x=1#y", want: "https://example.com/dojo"},
		{baseURL: "localhost:8080", wantErr: true},
		{baseURL: "/relative", wantErr: true},
		{baseURL: "ftp://example.com", wantErr: true},
		{baseURL: "http://[::1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			c, err := defectdojo.NewClient(tt.baseURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewClient(%q) error = nil, want error", tt.baseURL)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewClient(%q) error = %v", tt.baseURL, err)
			}
			if got := c.BaseURL().String(); got != tt.want {
				t.Errorf("BaseURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewClient_optionErrors(t *testing.T) {
	opts := map[string]defectdojo.ClientOption{
		"empty API token": defectdojo.WithAPIToken(""),
		"empty username":  defectdojo.WithBasicAuth("", "p"),
		"nil HTTP client": defectdojo.WithHTTPClient(nil),
	}
	for name, opt := range opts {
		t.Run(name, func(t *testing.T) {
			if _, err := defectdojo.NewClient("http://localhost:8080", opt); err == nil {
				t.Error("NewClient() error = nil, want error")
			}
		})
	}
}

func TestClient_BaseURLIsACopy(t *testing.T) {
	c, err := defectdojo.NewClient("http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL().Host = "changed"
	if got := c.BaseURL().Host; got != "localhost:8080" {
		t.Errorf("BaseURL().Host = %q after modifying a copy, want %q", got, "localhost:8080")
	}
}

func TestNewRequest_headers(t *testing.T) {
	tests := []struct {
		name     string
		opts     []defectdojo.ClientOption
		wantAuth string
	}{
		{name: "API token", opts: []defectdojo.ClientOption{defectdojo.WithAPIToken("abc123")}, wantAuth: "Token abc123"},
		{name: "basic auth", opts: []defectdojo.ClientOption{defectdojo.WithBasicAuth("admin", "secret")}, wantAuth: "Basic YWRtaW46c2VjcmV0"},
		{name: "no auth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := defectdojo.NewClient("http://localhost:8080", tt.opts...)
			if err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
				req, err := c.NewRequest(context.Background(), method, "/api/v2/user_profile/", nil)
				if err != nil {
					t.Fatalf("NewRequest(%s) error = %v", method, err)
				}
				testHeader(t, req, "Authorization", tt.wantAuth)
				testHeader(t, req, "Accept", "application/json")
				testHeader(t, req, "Content-Type", "")
				if ua := req.Header.Get("User-Agent"); !strings.HasPrefix(ua, "go-defectdojo") {
					t.Errorf("User-Agent = %q, want go-defectdojo prefix", ua)
				}
			}
		})
	}
}

func TestNewRequest_optionsOverrideHeaders(t *testing.T) {
	c, err := defectdojo.NewClient("http://localhost:8080", defectdojo.WithAPIToken("abc"), defectdojo.WithUserAgent("my-app/1.0"))
	if err != nil {
		t.Fatal(err)
	}
	req, err := c.NewRequest(context.Background(), http.MethodGet, "/api/v2/user_profile/", nil,
		defectdojo.WithHeader("Authorization", "Bearer token"),
		defectdojo.WithHeader("X-Custom", "1"),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	testHeader(t, req, "Authorization", "Bearer token")
	testHeader(t, req, "X-Custom", "1")
	testHeader(t, req, "User-Agent", "my-app/1.0")
}

func TestNewRequest_optionError(t *testing.T) {
	c, err := defectdojo.NewClient("http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("boom")
	_, err = c.NewRequest(context.Background(), http.MethodGet, "/api/v2/user_profile/", nil,
		func(*http.Request) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Errorf("NewRequest() error = %v, want %v", err, wantErr)
	}
}

func TestNewRequest_url(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		path    string
		opts    []defectdojo.RequestOption
		want    string
	}{
		{name: "plain", baseURL: "http://d:8080", path: "/api/v2/products/", want: "http://d:8080/api/v2/products/"},
		{name: "base path", baseURL: "http://d:8080/dojo/", path: "/api/v2/products/", want: "http://d:8080/dojo/api/v2/products/"},
		{name: "escaped segment", baseURL: "http://d:8080", path: "/api/v2/x/a%2Fb%20c/", want: "http://d:8080/api/v2/x/a%2Fb%20c/"},
		{name: "query string", baseURL: "http://d:8080", path: "/api/v2/products/?limit=1", want: "http://d:8080/api/v2/products/?limit=1"},
		{
			name: "query param option keeps existing query", baseURL: "http://d:8080", path: "/api/v2/x/?b=2&a=1",
			opts: []defectdojo.RequestOption{defectdojo.WithQueryParam("d", "& e")},
			want: "http://d:8080/api/v2/x/?b=2&a=1&d=%26+e",
		},
		{name: "escaped base path", baseURL: "http://d:8080/a%2Fb/", path: "/api/v2/x/", want: "http://d:8080/a%2Fb/api/v2/x/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := defectdojo.NewClient(tt.baseURL)
			if err != nil {
				t.Fatal(err)
			}
			req, err := c.NewRequest(context.Background(), http.MethodGet, tt.path, nil, tt.opts...)
			if err != nil {
				t.Fatalf("NewRequest() error = %v", err)
			}
			if got := req.URL.String(); got != tt.want {
				t.Errorf("URL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewRequest_invalidPath(t *testing.T) {
	c, err := defectdojo.NewClient("http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"api/v2/", "", "/api/%zz", "/api/a#frag", "/api/a b", "/api/é", "/api/x?q=a b", "/api/x?q=é"} {
		if _, err := c.NewRequest(context.Background(), http.MethodGet, path, nil); err == nil {
			t.Errorf("NewRequest(%q) error = nil, want error", path)
		}
	}
}

func TestNewRequest_body(t *testing.T) {
	c, err := defectdojo.NewClient("http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	req, err := c.NewRequest(context.Background(), http.MethodPost, "/api/v2/x/", map[string]string{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}
	testHeader(t, req, "Content-Type", "application/json")
	testJSONBody(t, req, `{"a":"b"}`)

	req, err = c.NewRequest(context.Background(), http.MethodPost, "/api/v2/x/", strings.NewReader("raw"),
		defectdojo.WithHeader("Content-Type", "text/plain"))
	if err != nil {
		t.Fatal(err)
	}
	testHeader(t, req, "Content-Type", "text/plain")
	if data, _ := io.ReadAll(req.Body); string(data) != "raw" {
		t.Errorf("body = %q, want %q", data, "raw")
	}

	if _, err := c.NewRequest(context.Background(), http.MethodPost, "/api/v2/x/", make(chan int)); err == nil {
		t.Error("NewRequest() with unencodable body: error = nil, want error")
	}
}

func TestDo(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /json", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"a":"b"}`))
	})
	mux.HandleFunc("DELETE /no-content", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /empty", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /bad-json", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"a":`))
	})
	mux.HandleFunc("GET /trailing-data", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"a":"b"} junk`))
	})
	mux.HandleFunc("GET /trailing-space", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte("{\"a\":\"b\"}\n\n"))
	})
	mux.HandleFunc("GET /raw", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "raw body")
	})
	ctx := context.Background()

	do := func(method, path string, v any) (*defectdojo.Response, error) {
		t.Helper()
		req, err := client.NewRequest(ctx, method, path, nil)
		if err != nil {
			t.Fatal(err)
		}
		return client.Do(req, v)
	}

	var got map[string]string
	resp, err := do(http.MethodGet, "/json", &got)
	if err != nil {
		t.Fatalf("Do(json) error = %v", err)
	}
	if resp.StatusCode != http.StatusOK || got["a"] != "b" {
		t.Errorf("Do(json) = %d %v, want 200 map[a:b]", resp.StatusCode, got)
	}

	var untouched map[string]string
	if resp, err := do(http.MethodDelete, "/no-content", &untouched); err != nil || resp.StatusCode != http.StatusNoContent {
		t.Errorf("Do(204) = %v, %v; want 204, nil", resp, err)
	}
	if untouched != nil {
		t.Errorf("Do(204) decoded %v, want nil", untouched)
	}
	if _, err := do(http.MethodGet, "/empty", &untouched); err != nil {
		t.Errorf("Do(empty body) error = %v", err)
	}
	if _, err := do(http.MethodGet, "/json", nil); err != nil {
		t.Errorf("Do(nil v) error = %v", err)
	}
	if _, err := do(http.MethodGet, "/bad-json", &got); err == nil {
		t.Error("Do(bad json) error = nil, want error")
	}
	if _, err := do(http.MethodGet, "/trailing-data", &got); err == nil {
		t.Error("Do(trailing data) error = nil, want error")
	}
	if _, err := do(http.MethodGet, "/trailing-space", &got); err != nil {
		t.Errorf("Do(trailing whitespace) error = %v", err)
	}

	var buf bytes.Buffer
	if _, err := do(http.MethodGet, "/raw", &buf); err != nil {
		t.Fatalf("Do(writer) error = %v", err)
	}
	if want := "raw body"; buf.String() != want {
		t.Errorf("Do(writer) wrote %q, want %q", buf.String(), want)
	}
}

func TestDo_doesNotFollowRedirects(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/x/", func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the redirect was followed")
	})
	mux.HandleFunc("POST /api/v2/x", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/v2/x/", http.StatusMovedPermanently)
	})

	req, err := client.NewRequest(context.Background(), http.MethodPost, "/api/v2/x", map[string]int{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(req, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || errResp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("Do() error = %v, want a 301 *ErrorResponse", err)
	}
	if !strings.Contains(err.Error(), "redirected to /api/v2/x/") {
		t.Errorf("Error() = %q, want it to name the redirect target", err)
	}
}

func TestDo_contextCanceled(t *testing.T) {
	_, client := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := client.NewRequest(ctx, http.MethodGet, "/api/v2/user_profile/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(req, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("Do() error = %v, want context.Canceled", err)
	}
}

func TestResponse_Deprecated(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/tool_types/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Deprecated", "This endpoint is deprecated and will be removed in 3.6.0")
		writeJSON(t, w, http.StatusOK, []byte(`{}`))
	})
	req, err := client.NewRequest(context.Background(), http.MethodGet, "/api/v2/tool_types/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Deprecated(); !strings.Contains(got, "3.6.0") {
		t.Errorf("Deprecated() = %q, want the header value", got)
	}
	var nilResp *defectdojo.Response
	if got := nilResp.Deprecated(); got != "" {
		t.Errorf("nil Response Deprecated() = %q, want empty", got)
	}
}

func TestWithHTTPClient(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return http.DefaultTransport.RoundTrip(r)
	})}

	client, err := defectdojo.NewClient(server.URL, defectdojo.WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ProductTypes.Delete(context.Background(), 1, nil); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !called {
		t.Error("custom HTTP client was not used")
	}
	if hc.CheckRedirect != nil {
		t.Error("WithHTTPClient changed the caller's HTTP client")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

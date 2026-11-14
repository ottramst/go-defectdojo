// Package defectdojo is a client for the DefectDojo REST API v2.
//
// Create a client with [NewClient], passing the DefectDojo URL and an
// authentication option, then use its services to call the API:
//
//	client, err := defectdojo.NewClient("https://defectdojo.example.com",
//		defectdojo.WithAPIToken(os.Getenv("DEFECTDOJO_API_TOKEN")))
//	if err != nil {
//		return err
//	}
//	productTypes, _, err := client.ProductTypes.List(ctx, nil)
//
// Every service method takes a [context.Context] first and returns the
// decoded result (if DefectDojo returns one), the [Response] and an error. API
// errors are returned as [*ErrorResponse] and can be matched with [errors.Is]
// against sentinel errors such as [ErrNotFound].
package defectdojo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
)

const (
	modulePath       = "github.com/ottramst/go-defectdojo"
	defaultUserAgent = "go-defectdojo"
)

// A Client manages communication with the DefectDojo API. It is safe for
// concurrent use by multiple goroutines.
type Client struct {
	httpClient *http.Client

	// baseURL is the DefectDojo URL, including any path prefix, without a
	// trailing slash.
	baseURL *url.URL

	// authorization is the value of the Authorization header, if any.
	authorization string

	userAgent string

	// serverVersion caches the result of ServerVersion.
	versionMu     sync.Mutex
	serverVersion string

	// Reuse a single struct instead of allocating one for each service.
	common service

	// Services used for talking to different parts of the DefectDojo API.
	Auth                     *AuthService
	ConfigurationPermissions *ConfigurationPermissionsService
	Engagements              *EngagementsService
	Products                 *ProductsService
	ProductTypes             *ProductTypesService
	SLAConfigurations        *SLAConfigurationsService
	UserProfile              *UserProfileService
	Users                    *UsersService
}

type service struct {
	client *Client
}

// A ClientOption configures a [Client] created with [NewClient].
type ClientOption func(*Client) error

// WithAPIToken authenticates requests with a DefectDojo API v2 token, sent
// as "Authorization: Token <token>". Get one from the user's API v2 key page
// or with [AuthService.Token].
func WithAPIToken(token string) ClientOption {
	return func(c *Client) error {
		if token == "" {
			return errors.New("defectdojo: API token must not be empty")
		}
		c.authorization = "Token " + token
		return nil
	}
}

// WithBasicAuth authenticates requests with a username and password. It works
// also when API tokens are disabled on the server (DD_API_TOKENS_ENABLED).
func WithBasicAuth(username, password string) ClientOption {
	return func(c *Client) error {
		if username == "" {
			return errors.New("defectdojo: username must not be empty")
		}
		c.authorization = "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
		return nil
	}
}

// WithHTTPClient sets the HTTP client used for requests. Use it to configure
// timeouts, TLS, proxies or a retrying transport. The default client has no
// timeout; pass a context with a deadline to bound individual calls.
//
// The client is copied. Unless it has its own CheckRedirect, the copy does not
// follow redirects: DefectDojo redirects requests it cannot serve, such as a
// path without its trailing slash, and following them would turn a POST into a
// GET or end on the HTML login page.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) error {
		if httpClient == nil {
			return errors.New("defectdojo: HTTP client must not be nil")
		}
		c.httpClient = withoutRedirects(httpClient)
		return nil
	}
}

// WithUserAgent sets the User-Agent header sent with every request.
func WithUserAgent(userAgent string) ClientOption {
	return func(c *Client) error {
		c.userAgent = userAgent
		return nil
	}
}

// NewClient returns a new DefectDojo API client for the DefectDojo instance at
// baseURL, for example "https://defectdojo.example.com". If DefectDojo is
// served under a path prefix, include it in the URL.
func NewClient(baseURL string, opts ...ClientOption) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("defectdojo: invalid base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("defectdojo: invalid base URL %q: must be an absolute http or https URL", baseURL)
	}
	// Keep escaped characters of the base path, such as %2F, as they are.
	escaped := strings.TrimSuffix(u.EscapedPath(), "/")
	unescaped, err := url.PathUnescape(escaped)
	if err != nil {
		return nil, fmt.Errorf("defectdojo: invalid base URL: %w", err)
	}
	u.Path = unescaped
	u.RawPath = escaped
	u.RawQuery = ""
	u.Fragment = ""

	c := &Client{
		httpClient: withoutRedirects(&http.Client{}),
		baseURL:    u,
		userAgent:  userAgent(),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	c.initServices()
	return c, nil
}

func (c *Client) initServices() {
	c.common.client = c
	c.Auth = (*AuthService)(&c.common)
	c.ConfigurationPermissions = (*ConfigurationPermissionsService)(&c.common)
	c.Engagements = (*EngagementsService)(&c.common)
	c.Products = (*ProductsService)(&c.common)
	c.ProductTypes = (*ProductTypesService)(&c.common)
	c.SLAConfigurations = (*SLAConfigurationsService)(&c.common)
	c.UserProfile = (*UserProfileService)(&c.common)
	c.Users = (*UsersService)(&c.common)
}

// BaseURL returns a copy of the DefectDojo URL the client sends requests to.
func (c *Client) BaseURL() *url.URL {
	u := *c.baseURL
	return &u
}

// withoutRedirects returns a copy of hc that does not follow redirects,
// unless hc has its own redirect policy.
func withoutRedirects(hc *http.Client) *http.Client {
	clone := *hc
	if clone.CheckRedirect == nil {
		clone.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return &clone
}

// userAgent returns "go-defectdojo/<version>" when the module version is
// known from the build information, and "go-defectdojo" otherwise.
func userAgent() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return defaultUserAgent
	}
	for _, dep := range bi.Deps {
		if dep.Path == modulePath && dep.Version != "" && dep.Version != "(devel)" {
			return defaultUserAgent + "/" + dep.Version
		}
	}
	return defaultUserAgent
}

// NewRequest creates an API request. path is the API path as it appears in
// the DefectDojo API reference, such as "/api/v2/product_types/", and may
// include a query string. Path segments taken from user input must already be
// escaped. DefectDojo paths end with a slash; without it, DefectDojo answers
// with a redirect, which the client returns as an error.
//
// If body is an [io.Reader], it is sent as is; otherwise a non-nil body is
// JSON encoded. Request options are applied last, so they can override any
// header the client sets.
//
// NewRequest and [Client.Do] can be used to call endpoints the client has no
// method for yet.
func (c *Client) NewRequest(ctx context.Context, method, path string, body any, opts ...RequestOption) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("defectdojo: path %q must start with /", path)
	}
	rawPath, rawQuery, _ := strings.Cut(path, "?")
	if strings.Contains(path, "#") || !validEscapedPath(rawPath) || !validEscapedQuery(rawQuery) {
		return nil, fmt.Errorf("defectdojo: path %q must be escaped", path)
	}

	u := *c.baseURL
	u.RawPath = u.EscapedPath() + rawPath
	unescaped, err := url.PathUnescape(u.RawPath)
	if err != nil {
		return nil, fmt.Errorf("defectdojo: invalid path %q: %w", path, err)
	}
	u.Path = unescaped
	u.RawQuery = rawQuery

	var (
		bodyReader  io.Reader
		contentType string
	)
	switch b := body.(type) {
	case nil:
	case io.Reader:
		bodyReader = b
	default:
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("defectdojo: encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.authorization != "" {
		req.Header.Set("Authorization", c.authorization)
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(req); err != nil {
			return nil, err
		}
	}
	return req, nil
}

// Response wraps the [http.Response] returned by DefectDojo. Its body has
// already been read and closed.
type Response struct {
	*http.Response
}

// Deprecated returns the value of the X-Deprecated header, which DefectDojo
// sets on endpoints that are scheduled for removal, and "" otherwise.
func (r *Response) Deprecated() string {
	if r == nil || r.Response == nil {
		return ""
	}
	return r.Header.Get("X-Deprecated")
}

// Do sends an API request and returns the API response.
//
// If the response status is not 2xx, Do returns an [*ErrorResponse]; this
// includes redirects, which the client does not follow. Otherwise the
// response body is handled according to v: a nil v discards it, an
// [io.Writer] receives the raw body, and any other value is decoded from JSON.
// An empty body, such as a 204 No Content response, leaves v unchanged.
func (c *Client) Do(req *http.Request, v any) (*Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		// Drain the body so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	response := &Response{Response: resp}
	if err := CheckResponse(resp); err != nil {
		return response, err
	}

	switch v := v.(type) {
	case nil:
	case io.Writer:
		if _, err := io.Copy(v, resp.Body); err != nil {
			return response, fmt.Errorf("defectdojo: reading response body: %w", err)
		}
	default:
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(v); err != nil {
			if errors.Is(err, io.EOF) {
				return response, nil
			}
			return response, fmt.Errorf("defectdojo: decoding response body: %w", err)
		}
		if _, err := dec.Token(); !errors.Is(err, io.EOF) {
			return response, errors.New("defectdojo: decoding response body: unexpected data after the JSON value")
		}
	}
	return response, nil
}

// do creates and sends a request in one step, for service methods.
func (c *Client) do(ctx context.Context, method, path string, body, v any, opts []RequestOption) (*Response, error) {
	req, err := c.NewRequest(ctx, method, path, body, opts...)
	if err != nil {
		return nil, err
	}
	return c.Do(req, v)
}

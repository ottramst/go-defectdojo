package defectdojo

import (
	"context"
	"errors"
	"net/http"
)

// ServerVersionOptions are the options of [Client.ServerVersion].
type ServerVersionOptions struct{}

// ServerVersion returns the DefectDojo version of the server, such as
// "3.5.103". DefectDojo has no version endpoint, so it is read from the
// OpenAPI schema the server serves, which is about 860 KB. The result is
// cached by the client after the first successful call, and a cached result
// comes with a nil [*Response]; concurrent first calls may each fetch the
// schema.
//
// DefectDojo API: GET /api/v2/oa3/schema/
func (c *Client) ServerVersion(ctx context.Context, _ *ServerVersionOptions, opts ...RequestOption) (string, *Response, error) {
	c.versionMu.Lock()
	cached := c.serverVersion
	c.versionMu.Unlock()
	if cached != "" {
		return cached, nil, nil
	}
	var schema struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	resp, err := c.do(ctx, http.MethodGet, "/api/v2/oa3/schema/?format=json", nil, &schema, opts)
	if err != nil {
		return "", resp, err
	}
	if schema.Info.Version == "" {
		return "", resp, errors.New("defectdojo: the OpenAPI schema has no version")
	}
	// The lock is not held during the request, so that a hanging request does
	// not block calls with other contexts.
	c.versionMu.Lock()
	c.serverVersion = schema.Info.Version
	c.versionMu.Unlock()
	return schema.Info.Version, resp, nil
}

package defectdojo

import (
	"context"
	"errors"
	"net/http"
)

// ServerVersion returns the DefectDojo version of the server, such as
// "3.5.103". DefectDojo has no version endpoint, so it is read from the
// OpenAPI schema the server serves, which is about 1 MB. The result is cached
// by the client after the first successful call.
//
// DefectDojo API: GET /api/v2/oa3/schema/
func (c *Client) ServerVersion(ctx context.Context, opts ...RequestOption) (string, error) {
	c.versionMu.Lock()
	defer c.versionMu.Unlock()
	if c.serverVersion != "" {
		return c.serverVersion, nil
	}
	var schema struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if _, err := c.do(ctx, http.MethodGet, "/api/v2/oa3/schema/?format=json", nil, &schema, opts); err != nil {
		return "", err
	}
	if schema.Info.Version == "" {
		return "", errors.New("defectdojo: the OpenAPI schema has no version")
	}
	c.serverVersion = schema.Info.Version
	return c.serverVersion, nil
}

package defectdojo

import (
	"context"
	"errors"
	"net/http"
)

// tokenRequest is the body of a token request.
type tokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// tokenResponse is the body of a token response.
type tokenResponse struct {
	Token string `json:"token"`
}

// AuthService handles communication with the authentication methods of the
// DefectDojo API.
//
// DefectDojo API docs: https://docs.defectdojo.com/en/api/api-v2-docs/
type AuthService service

// TokenOptions are the options of [AuthService.Token].
type TokenOptions struct {
	_ [0]func()

	_ [0]func()
}

// Token returns the API v2 token of the user with the given username and
// password, creating it if the user has none. DefectDojo returns the same
// token on every call; it does not rotate it.
//
// The endpoint is rate limited per client IP address when the server enables
// DD_RATE_LIMITER_ENABLED, and can be disabled with
// DD_API_TOKEN_AUTH_ENDPOINT_ENABLED. Fetch the token once and reuse it with
// [WithAPIToken], or authenticate with [WithBasicAuth] instead.
//
// DefectDojo API: POST /api/v2/api-token-auth/
func (s *AuthService) Token(ctx context.Context, username, password string, opt *TokenOptions, opts ...RequestOption) (string, *Response, error) {
	if username == "" {
		return "", nil, errors.New("defectdojo: username must not be empty")
	}
	body := tokenRequest{Username: username, Password: password}
	var result tokenResponse
	resp, err := s.client.do(ctx, http.MethodPost, "/api/v2/api-token-auth/", body, &result, opts)
	if err != nil {
		return "", resp, err
	}
	if result.Token == "" {
		return "", resp, errors.New("defectdojo: the response has no token")
	}
	return result.Token, resp, nil
}

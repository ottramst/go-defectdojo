package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestAuthService_Token(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/api-token-auth/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"username":"admin","password":"secret"}`)
		writeJSON(t, w, http.StatusOK, []byte(`{"token":"0123456789abcdef"}`))
	})

	token, _, err := client.Auth.Token(context.Background(), "admin", "secret", nil)
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token != "0123456789abcdef" {
		t.Errorf("Token() = %q", token)
	}
}

func TestAuthService_Token_errors(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/api-token-auth/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"username":"admin","password":"wrong"}`)
		writeJSON(t, w, http.StatusBadRequest, []byte(`{"non_field_errors":["Unable to log in with provided credentials."]}`))
	})
	mux.HandleFunc("POST /empty/api/v2/api-token-auth/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{}`))
	})

	if _, _, err := client.Auth.Token(context.Background(), "admin", "wrong", nil); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Token(wrong password) error = %v, want ErrBadRequest", err)
	}
	if _, _, err := client.Auth.Token(context.Background(), "", "x", nil); err == nil {
		t.Error("Token(empty username) error = nil, want error")
	}

	empty, err := defectdojo.NewClient(client.BaseURL().String() + "/empty")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := empty.Auth.Token(context.Background(), "admin", "x", nil); err == nil {
		t.Error("Token(no token in response) error = nil, want error")
	}
}

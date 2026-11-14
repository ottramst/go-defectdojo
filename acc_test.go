package defectdojo_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"regexp"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

// Acceptance tests run against a real DefectDojo when DEFECTDOJO_ACC is set.
// Start one with `docker compose up -d --wait`, then run `make testacc`.
//
//	DEFECTDOJO_URL        DefectDojo URL (default http://localhost:8080)
//	DEFECTDOJO_API_TOKEN  API v2 token; if empty, basic auth is used
//	DEFECTDOJO_USERNAME   basic auth user (default admin)
//	DEFECTDOJO_PASSWORD   basic auth password (default DefectDojoAcc1!)
//
// The tests need a superuser. Once DEFECTDOJO_ACC is set, a server that cannot
// be reached or rejects the credentials fails the tests instead of skipping
// them.

// accClient returns a client for the acceptance test DefectDojo, or skips the
// test unless DEFECTDOJO_ACC is set.
func accClient(t *testing.T) *defectdojo.Client {
	t.Helper()
	if os.Getenv("DEFECTDOJO_ACC") == "" {
		t.Skip("set DEFECTDOJO_ACC=1 to run acceptance tests")
	}
	auth := defectdojo.WithBasicAuth(envOr("DEFECTDOJO_USERNAME", "admin"), envOr("DEFECTDOJO_PASSWORD", "DefectDojoAcc1!"))
	if token := os.Getenv("DEFECTDOJO_API_TOKEN"); token != "" {
		auth = defectdojo.WithAPIToken(token)
	}
	client, err := defectdojo.NewClient(envOr("DEFECTDOJO_URL", "http://localhost:8080"), auth)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

// releaseVersion matches DefectDojo release versions such as "3.5.103".
var releaseVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// randName returns prefix followed by a random suffix, usable as a unique
// object name.
func randName(t *testing.T, prefix string) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return prefix + "-" + hex.EncodeToString(b)
}

// cleanup registers fn to run when the test ends, with a context that is not
// canceled with the test's.
func cleanup(t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	t.Cleanup(func() {
		if err := fn(context.WithoutCancel(t.Context())); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})
}

func TestAccServerVersion(t *testing.T) {
	client := accClient(t)
	v, err := client.ServerVersion(t.Context())
	if err != nil {
		t.Fatalf("ServerVersion() error = %v", err)
	}
	// CI sets DD_VERSION to the image tag it started; tags that are not
	// releases, such as nightly-dev, serve versions like "3.6.0-dev".
	if want := os.Getenv("DD_VERSION"); releaseVersion.MatchString(want) && v != want {
		t.Errorf("ServerVersion() = %q, want %q (DD_VERSION)", v, want)
	}
	t.Logf("DefectDojo %s", v)
}

func TestAccAuthToken(t *testing.T) {
	client := accClient(t)
	if os.Getenv("DEFECTDOJO_API_TOKEN") != "" {
		t.Skip("needs basic auth credentials")
	}
	token, _, err := client.Auth.Token(t.Context(), envOr("DEFECTDOJO_USERNAME", "admin"), envOr("DEFECTDOJO_PASSWORD", "DefectDojoAcc1!"))
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	tokenClient, err := defectdojo.NewClient(client.BaseURL().String(), defectdojo.WithAPIToken(token))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := tokenClient.ProductTypes.List(t.Context(), &defectdojo.ListProductTypesOptions{ListOptions: defectdojo.ListOptions{Limit: 1}}); err != nil {
		t.Errorf("List() with the token error = %v", err)
	}
}

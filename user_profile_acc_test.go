package defectdojo_test

import (
	"os"
	"testing"
)

func TestAccUserProfile(t *testing.T) {
	client := accClient(t)
	profile, _, err := client.UserProfile.Get(t.Context())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	// The acceptance tests run as a superuser, admin by default.
	if profile.User == nil || profile.User.ID == 0 || !profile.User.IsSuperuser || !profile.User.IsActive || profile.User.DateJoined.IsZero() {
		t.Errorf("Get() user = %+v, want an active superuser", profile.User)
	}
	if want := envOr("DEFECTDOJO_USERNAME", "admin"); os.Getenv("DEFECTDOJO_API_TOKEN") == "" && profile.User.Username != want {
		t.Errorf("Username = %q, want %q", profile.User.Username, want)
	}
}

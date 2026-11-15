package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/ottramst/go-defectdojo"
)

func TestUserProfileService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/user_profile/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "user_profile/get.json"))
	})

	profile, _, err := client.UserProfile.Get(context.Background(), nil)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	joined, err := time.Parse(time.RFC3339Nano, "2026-11-14T12:07:36.179289Z")
	if err != nil {
		t.Fatal(err)
	}
	wantUser := &defectdojo.User{
		ID: 2, Username: "alice", FirstName: "Alice", LastName: "Smith", Email: "alice@example.com",
		IsActive: true, ConfigurationPermissions: []int{}, DateJoined: joined,
	}
	if !reflect.DeepEqual(profile.User, wantUser) {
		t.Errorf("User = %+v, want %+v", profile.User, wantUser)
	}
	wantInfo := &defectdojo.UserContactInfo{
		ID: 1, User: 2, Title: "AppSec engineer", PhoneNumber: "+3725551234", SlackUsername: "alice", BlockExecution: true,
	}
	if !reflect.DeepEqual(profile.UserContactInfo, wantInfo) {
		t.Errorf("UserContactInfo = %+v, want %+v", profile.UserContactInfo, wantInfo)
	}
}

func TestUserProfileService_Get_noContactInfo(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/user_profile/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, []byte(`{"user":{"id":1,"username":"admin","is_superuser":true},"user_contact_info":null}`))
	})
	profile, _, err := client.UserProfile.Get(context.Background(), nil)
	if err != nil || profile.UserContactInfo != nil || !profile.User.IsSuperuser {
		t.Errorf("Get() = %+v, %v; want a superuser without contact information", profile, err)
	}
}

func TestUserProfileService_Get_error(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/user_profile/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, fixture(t, "errors/invalid_token.json"))
	})
	if _, _, err := client.UserProfile.Get(context.Background(), nil); !errors.Is(err, defectdojo.ErrForbidden) {
		t.Errorf("Get() error = %v, want ErrForbidden", err)
	}
}

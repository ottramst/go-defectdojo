package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestSystemSettingsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/system_settings/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "")
		writeJSON(t, w, http.StatusOK, fixture(t, "system_settings/list.json"))
	})

	settings, _, err := client.SystemSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if settings.ID != 1 || settings.MaxDupes == nil || *settings.MaxDupes != 10 || settings.EmailFrom != "no-reply@example.com" {
		t.Errorf("Get() = %+v, want ID 1, max_dupes 10 and the default sender", settings)
	}
	if settings.JiraMinimumSeverity != "Low" || settings.JiraWebhookSecret == "" || settings.JiraLabels != "" {
		t.Errorf("Get() Jira = %q, %q, %q; want Low, a secret and no labels", settings.JiraMinimumSeverity, settings.JiraWebhookSecret, settings.JiraLabels)
	}
	if settings.RiskAcceptanceFormDefaultDays == nil || *settings.RiskAcceptanceFormDefaultDays != 180 {
		t.Errorf("Get() RiskAcceptanceFormDefaultDays = %v, want 180", settings.RiskAcceptanceFormDefaultDays)
	}
	if !settings.EnableFindingSLA || settings.MinimumPasswordLength != 9 || settings.MaximumPasswordLength != 48 {
		t.Errorf("Get() = %+v, want the default SLA and password settings", settings)
	}
	if settings.ReadOnlyMode == nil || *settings.ReadOnlyMode || settings.InstallID == "" {
		t.Errorf("Get() ReadOnlyMode = %v, InstallID = %q; want false and an ID", settings.ReadOnlyMode, settings.InstallID)
	}
}

func TestSystemSettingsService_Get_olderServer(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/system_settings/", func(w http.ResponseWriter, _ *http.Request) {
		// DefectDojo before 3.5.100 has no read_only_mode.
		writeJSON(t, w, http.StatusOK, fixture(t, "system_settings/list_3.4.json"))
	})

	settings, _, err := client.SystemSettings.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if settings.ReadOnlyMode != nil {
		t.Errorf("ReadOnlyMode = %v, want nil", *settings.ReadOnlyMode)
	}
}

func TestSystemSettingsService_Get_notOne(t *testing.T) {
	for _, body := range []string{`{"count":0,"results":[]}`, `{"count":2,"results":[{"id":1},{"id":2}]}`} {
		mux, client := setup(t)
		mux.HandleFunc("GET /api/v2/system_settings/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusOK, []byte(body))
		})
		if settings, _, err := client.SystemSettings.Get(context.Background()); err == nil {
			t.Errorf("Get(%s) = %+v, want an error", body, settings)
		}
	}
}

func TestSystemSettingsService_Get_error(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/system_settings/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusForbidden, []byte(`{"detail":"You do not have permission to perform this action."}`))
	})
	if _, _, err := client.SystemSettings.Get(context.Background()); !errors.Is(err, defectdojo.ErrForbidden) {
		t.Errorf("Get() error = %v, want ErrForbidden", err)
	}
}

func TestSystemSettingsService_Patch(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PATCH /api/v2/system_settings/1/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"team_name":"AppSec","max_dupes":null,"slack_token":"xoxb-fake-token","enable_jira":false,"minimum_password_length":12,"maximum_password_length":12}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "system_settings/patch.json"))
	})

	settings, _, err := client.SystemSettings.Patch(context.Background(), 1, &defectdojo.SystemSettingsPatch{
		TeamName:              new("AppSec"),
		MaxDupes:              defectdojo.Null[int](),
		SlackToken:            new("xoxb-fake-token"),
		EnableJira:            new(false),
		MinimumPasswordLength: new(12),
		MaximumPasswordLength: new(12),
	})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if settings.TeamName != "AppSec" || settings.MaxDupes != nil || settings.SlackToken != "xoxb-fake-token" {
		t.Errorf("Patch() = %+v, want the patched team name, no max_dupes and the token", settings)
	}
}

func TestSystemSettingsService_Patch_invalid(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	tests := []struct {
		name  string
		id    int
		patch *defectdojo.SystemSettingsPatch
	}{
		{"zero ID", 0, &defectdojo.SystemSettingsPatch{TeamName: new("x")}},
		{"nil patch", 1, nil},
		{"minimum too short", 1, &defectdojo.SystemSettingsPatch{MinimumPasswordLength: new(8)}},
		{"maximum too long", 1, &defectdojo.SystemSettingsPatch{MaximumPasswordLength: new(49)}},
		{"minimum above maximum", 1, &defectdojo.SystemSettingsPatch{MinimumPasswordLength: new(20), MaximumPasswordLength: new(15)}},
	}
	for _, tt := range tests {
		// The mux has no handler, so a request would fail with a 404.
		if _, _, err := client.SystemSettings.Patch(ctx, tt.id, tt.patch); err == nil || errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("Patch(%s) error = %v, want an error before sending", tt.name, err)
		}
	}
}

func TestSystemSettingsService_Patch_readOnly(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PATCH /api/v2/system_settings/1/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusLocked, fixture(t, "errors/read_only_mode.json"))
	})
	if _, _, err := client.SystemSettings.Patch(context.Background(), 1, &defectdojo.SystemSettingsPatch{TeamName: new("x")}); !errors.Is(err, defectdojo.ErrLocked) {
		t.Errorf("Patch() error = %v, want ErrLocked", err)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"3.5.103", "3.5.100", 1},
		{"3.4.304", "3.5.100", -1},
		{"3.5", "3.5.0", 0},
		{"3.6.1-dev", "3.6.1", 0},
		{"3.6.0-dev", "3.5.103", 1},
	}
	for _, tt := range tests {
		if got := defectdojo.CompareVersions(tt.a, tt.b); got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

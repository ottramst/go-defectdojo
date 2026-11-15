package defectdojo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestAccSystemSettings(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	before, _, err := client.SystemSettings.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	// Restore what the test changes. It never touches read_only_mode,
	// enable_finding_sla, the password rules, api_expose_error_details or
	// filter_string_matching, which would affect the other tests.
	cleanup(t, func(ctx context.Context) error {
		_, _, err := client.SystemSettings.Patch(ctx, before.ID, &defectdojo.SystemSettingsPatch{
			TeamName:                new(before.TeamName),
			DisclaimerNotes:         new(before.DisclaimerNotes),
			EngagementAutoCloseDays: new(before.EngagementAutoCloseDays),
			MaxDupes:                nullableInt(before.MaxDupes),
			JiraLabels:              nullableString(before.JiraLabels),
		})
		return err
	})

	version, _, err := client.ServerVersion(ctx, nil)
	if err != nil {
		t.Fatalf("ServerVersion() error = %v", err)
	}
	if hasReadOnlyMode := defectdojo.CompareVersions(version, "3.5.100") >= 0; hasReadOnlyMode != (before.ReadOnlyMode != nil) {
		t.Errorf("ReadOnlyMode = %v on %s, want it set from 3.5.100", before.ReadOnlyMode, version)
	} else if hasReadOnlyMode && *before.ReadOnlyMode {
		t.Fatal("the test server is in read-only mode")
	}
	if before.InstallID == "" {
		t.Error("InstallID is empty")
	}

	patched, _, err := client.SystemSettings.Patch(ctx, before.ID, &defectdojo.SystemSettingsPatch{
		TeamName:                new("  acceptance  "),
		DisclaimerNotes:         new("acceptance test"),
		EngagementAutoCloseDays: new(before.EngagementAutoCloseDays + 1),
		MaxDupes:                defectdojo.Null[int](),
		JiraLabels:              defectdojo.Value("security acc"),
	})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	// Text is trimmed.
	if patched.TeamName != "acceptance" || patched.DisclaimerNotes != "acceptance test" ||
		patched.EngagementAutoCloseDays != before.EngagementAutoCloseDays+1 || patched.MaxDupes != nil || patched.JiraLabels != "security acc" {
		t.Errorf("Patch() = %+v, want the patched values", patched)
	}
	// Fields left out keep their values.
	if patched.EmailFrom != before.EmailFrom || patched.EnableFindingSLA != before.EnableFindingSLA || patched.JiraWebhookSecret != before.JiraWebhookSecret {
		t.Errorf("Patch() changed fields it didn't send: %+v", patched)
	}

	got, _, err := client.SystemSettings.Get(ctx)
	if err != nil || got.TeamName != "acceptance" || got.MaxDupes != nil {
		t.Errorf("Get() = %+v, %v; want the patched values", got, err)
	}

	// "" clears text that isn't nullable, null nullable text.
	cleared, _, err := client.SystemSettings.Patch(ctx, before.ID, &defectdojo.SystemSettingsPatch{
		TeamName:   new(""),
		MaxDupes:   defectdojo.Value(0),
		JiraLabels: defectdojo.Null[string](),
	})
	if err != nil {
		t.Fatalf("Patch(clear) error = %v", err)
	}
	if cleared.TeamName != "" || cleared.MaxDupes == nil || *cleared.MaxDupes != 0 || cleared.JiraLabels != "" {
		t.Errorf("Patch(clear) = %+v, want no team name, max_dupes 0 and no labels", cleared)
	}

	// DefectDojo checks the password lengths, but not that the minimum
	// doesn't exceed the maximum; the client checks both before sending.
	_, _, err = client.SystemSettings.Patch(ctx, before.ID, &defectdojo.SystemSettingsPatch{MinimumPasswordLength: new(8)})
	if err == nil || errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Patch(minimum 8) error = %v, want a client-side error", err)
	}
	// The Jira webhook secret is nullable, but can't be blank.
	_, _, err = client.SystemSettings.Patch(ctx, before.ID, &defectdojo.SystemSettingsPatch{JiraWebhookSecret: defectdojo.Value("")})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["jira_webhook_secret"]) == 0 {
		t.Errorf("Patch(empty Jira webhook secret) error = %v, want a 400 on jira_webhook_secret", err)
	}
}

func nullableInt(v *int) defectdojo.Nullable[int] {
	if v == nil {
		return defectdojo.Null[int]()
	}
	return defectdojo.Value(*v)
}

// nullableString restores a nullable string read as v. A read can't tell
// null from "", so "" is restored as null.
func nullableString(v string) defectdojo.Nullable[string] {
	if v == "" {
		return defectdojo.Null[string]()
	}
	return defectdojo.Value(v)
}

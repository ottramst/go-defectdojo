package defectdojo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestAccJiraInstances(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	name := randName(t, "acc")

	// DefectDojo doesn't contact Jira, so an unreachable URL works, also
	// while the system setting enable_jira is off.
	in := &defectdojo.JiraInstanceWrite{
		JiraInstance: defectdojo.JiraInstance{
			ConfigurationName:       name,
			URL:                     "https://" + name + ".example.invalid",
			Username:                "  bot  ",
			EpicNameID:              10011,
			OpenStatusKey:           11,
			CloseStatusKey:          31,
			InfoMappingSeverity:     "Lowest",
			LowMappingSeverity:      "Low",
			MediumMappingSeverity:   "Medium",
			HighMappingSeverity:     "High",
			CriticalMappingSeverity: "Highest",
			FindingText:             "acceptance test",
		},
		Password: "fake-token",
	}
	created, _, err := client.JiraInstances.Create(ctx, in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	deleteJiraInstanceAtCleanup(t, client, created.ID)
	// Text is trimmed; the issue type and the SLA notification get
	// DefectDojo's defaults, and nullable text reads as "".
	if created.Username != "bot" || created.DefaultIssueType != "Bug" || created.IssueTemplateDir != "" || created.FindingText != "acceptance test" {
		t.Errorf("Create() = %+v, want the trimmed username and the defaults", created)
	}

	// Names aren't unique.
	twin, _, err := client.JiraInstances.Create(ctx, in)
	if err != nil {
		t.Fatalf("Create(same name) error = %v", err)
	}
	deleteJiraInstanceAtCleanup(t, client, twin.ID)
	if _, _, err := client.JiraInstances.GetByName(ctx, name); err == nil || errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(duplicate) error = %v, want an error naming both", err)
	}
	if _, err := client.JiraInstances.Delete(ctx, twin.ID); err != nil {
		t.Fatalf("Delete(twin) error = %v", err)
	}

	got, _, err := client.JiraInstances.Get(ctx, created.ID)
	if err != nil || *got != *created {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, created)
	}
	if byName, _, err := client.JiraInstances.GetByName(ctx, name); err != nil || byName.ID != created.ID {
		t.Errorf("GetByName() = %+v, %v; want ID %d", byName, err, created.ID)
	}

	// Read, modify, write: a PUT needs the password.
	got.ConfigurationName = name + "-renamed"
	got.DefaultIssueType = "Security"
	got.FindingText = ""
	got.GlobalJiraSLANotification = false
	if _, _, err := client.JiraInstances.Update(ctx, got.ID, &defectdojo.JiraInstanceWrite{JiraInstance: *got}); err == nil {
		t.Error("Update(without password) error = nil, want a client-side error")
	}
	updated, _, err := client.JiraInstances.Update(ctx, got.ID, &defectdojo.JiraInstanceWrite{JiraInstance: *got, Password: "fake-token-2"})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.ConfigurationName != got.ConfigurationName || updated.DefaultIssueType != "Security" || updated.FindingText != "" || updated.GlobalJiraSLANotification {
		t.Errorf("Update() = %+v, want %+v", updated, got)
	}

	// A patch keeps the password, unless the URL changes; a trailing slash
	// isn't a change.
	if _, _, err := client.JiraInstances.Patch(ctx, got.ID, &defectdojo.JiraInstancePatch{Username: new("bot2"), URL: new(got.URL + "/")}); err != nil {
		t.Errorf("Patch(without password) error = %v", err)
	}
	_, _, err = client.JiraInstances.Patch(ctx, got.ID, &defectdojo.JiraInstancePatch{URL: new("https://other-" + name + ".example.invalid")})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["password"]) == 0 {
		t.Errorf("Patch(URL without password) error = %v, want a 400 on password", err)
	}
	newURL := "https://other-" + name + ".example.invalid"
	patched, _, err := client.JiraInstances.Patch(ctx, got.ID, &defectdojo.JiraInstancePatch{URL: new(newURL), Password: new("fake-token-3"), FindingText: defectdojo.Value("patched")})
	if err != nil {
		t.Fatalf("Patch(URL and password) error = %v", err)
	}
	if patched.URL != newURL || patched.Username != "bot2" || patched.FindingText != "patched" {
		t.Errorf("Patch() = %+v, want the new URL, username and finding text", patched)
	}

	list, _, err := client.JiraInstances.List(ctx, &defectdojo.ListJiraInstancesOptions{URL: newURL})
	if err != nil || list.Count != 1 || list.JiraInstances[0].ID != created.ID {
		t.Errorf("List(URL) = %+v, %v; want the instance", list, err)
	}

	if n, _, err := client.JiraInstances.CountJiraProjects(ctx, created.ID); err != nil || n != 0 {
		t.Errorf("CountJiraProjects() = %d, %v; want 0", n, err)
	}
	if _, _, err := client.JiraInstances.CountJiraProjects(ctx, created.ID+100000); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("CountJiraProjects(missing) error = %v, want ErrNotFound", err)
	}

	preview, _, err := client.JiraInstances.DeletePreview(ctx, created.ID, nil)
	if err != nil || preview.Count != 1 || preview.Objects[0].Model != "JIRA_Instance" {
		t.Errorf("DeletePreview() = %+v, %v; want the instance", preview, err)
	}

	if _, err := client.JiraInstances.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.JiraInstances.Get(ctx, created.ID); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}

// deleteJiraInstanceAtCleanup deletes the Jira instance when the test ends,
// unless the test deleted it.
func deleteJiraInstanceAtCleanup(t *testing.T, client *defectdojo.Client, id int) {
	t.Helper()
	cleanup(t, func(ctx context.Context) error {
		_, err := client.JiraInstances.Delete(ctx, id)
		if errors.Is(err, defectdojo.ErrNotFound) {
			return nil
		}
		return err
	})
}

package defectdojo_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ottramst/go-defectdojo"
)

func TestAccJiraProjects(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()

	// The instance is registered for cleanup first, so it is deleted after
	// the product type, whose products take their Jira projects with them.
	instanceID := createJiraInstance(t, client)
	productTypeID := createProductType(t, client)
	productID := createProductIn(t, client, productTypeID, false)

	created, _, err := client.JiraProjects.Create(ctx, &defectdojo.JiraProject{
		JiraInstance:  instanceID,
		Product:       productID,
		ProjectKey:    "  SEC  ",
		Component:     "web,api",
		CustomFields:  json.RawMessage(`{"b": 1, "a": {"name": "x"}}`),
		JiraLabels:    "security",
		PushAllIssues: true,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	// Text is trimmed, the epic issue type gets its default, and unset JSON
	// fields read as nil.
	if created.ProjectKey != "SEC" || created.Product != productID || created.Engagement != 0 || created.EpicIssueTypeName != "Epic" ||
		created.CloseTransitionFields != nil || !jsonEqual(t, created.CustomFields, `{"a":{"name":"x"},"b":1}`) {
		t.Errorf("Create() = %+v", created)
	}

	// A GET returns the JSON with sorted keys.
	got, _, err := client.JiraProjects.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ProjectKey != "SEC" || !jsonEqual(t, got.CustomFields, `{"a":{"name":"x"},"b":1}`) || !got.PushAllIssues {
		t.Errorf("Get() = %+v", got)
	}

	// A product can get several configurations.
	second, _, err := client.JiraProjects.Create(ctx, &defectdojo.JiraProject{JiraInstance: instanceID, Product: productID, ProjectKey: "SEC2"})
	if err != nil {
		t.Fatalf("Create(second for the product) error = %v", err)
	}
	list, _, err := client.JiraProjects.List(ctx, &defectdojo.ListJiraProjectsOptions{Product: productID})
	if err != nil || list.Count != 2 {
		t.Errorf("List(product) = %+v, %v; want 2", list, err)
	}
	if _, err := client.JiraProjects.Delete(ctx, second.ID); err != nil {
		t.Fatalf("Delete(second) error = %v", err)
	}

	// Read, modify, write.
	got.ProjectKey = "OPS"
	got.Component = ""
	got.CustomFields = nil
	got.CloseTransitionFields = json.RawMessage(`{"resolution":{"name":"Won't Fix"}}`)
	got.DefaultAssignee = "alice"
	got.PushNotes = true
	updated, _, err := client.JiraProjects.Update(ctx, got.ID, got)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.ProjectKey != "OPS" || updated.Component != "" || updated.CustomFields != nil || updated.DefaultAssignee != "alice" ||
		!updated.PushNotes || !jsonEqual(t, updated.CloseTransitionFields, `{"resolution":{"name":"Won't Fix"}}`) {
		t.Errorf("Update() = %+v", updated)
	}

	patched, _, err := client.JiraProjects.Patch(ctx, got.ID, &defectdojo.JiraProjectPatch{
		Enabled:               new(false),
		CloseTransitionFields: defectdojo.Null[json.RawMessage](),
		DefaultAssignee:       defectdojo.Null[string](),
	})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patched.Enabled || patched.CloseTransitionFields != nil || patched.DefaultAssignee != "" || patched.ProjectKey != "OPS" {
		t.Errorf("Patch() = %+v", patched)
	}

	checkSuppressSLANotificationComments(t, client, created.ID, instanceID, productID)

	// An engagement can have one configuration.
	engagement, _, err := client.Engagements.Create(ctx, &defectdojo.Engagement{
		Name: randName(t, "acc"), Product: productID,
		TargetStart: defectdojo.Date{Year: 2026, Month: time.January, Day: 1}, TargetEnd: defectdojo.Date{Year: 2026, Month: time.January, Day: 1},
	})
	if err != nil {
		t.Fatalf("creating engagement: %v", err)
	}
	forEngagement, _, err := client.JiraProjects.Create(ctx, &defectdojo.JiraProject{JiraInstance: instanceID, Engagement: engagement.ID, ProjectKey: "ENG"})
	if err != nil || forEngagement.Engagement != engagement.ID || forEngagement.Product != 0 {
		t.Fatalf("Create(engagement) = %+v, %v", forEngagement, err)
	}
	_, _, err = client.JiraProjects.Create(ctx, &defectdojo.JiraProject{JiraInstance: instanceID, Engagement: engagement.ID, ProjectKey: "ENG2"})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["engagement"]) == 0 {
		t.Errorf("Create(second for the engagement) error = %v, want a 400 on engagement", err)
	}
	// A product that doesn't exist is a 404.
	if _, _, err := client.JiraProjects.Create(ctx, &defectdojo.JiraProject{JiraInstance: instanceID, Product: productID + 100000}); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Create(missing product) error = %v, want ErrNotFound", err)
	}

	// The instance can't be deleted while it is used: DefectDojo answers 500.
	if n, _, err := client.JiraInstances.CountJiraProjects(ctx, instanceID); err != nil || n != 2 {
		t.Errorf("CountJiraProjects() = %d, %v; want 2", n, err)
	}
	if _, err := client.JiraInstances.Delete(ctx, instanceID); !errors.As(err, &errResp) || errResp.StatusCode != http.StatusInternalServerError {
		t.Errorf("JiraInstances.Delete(in use) error = %v, want a 500", err)
	}

	preview, _, err := client.JiraProjects.DeletePreview(ctx, created.ID, nil)
	if err != nil || preview.Objects[0].Model != "JIRA_Project" {
		t.Errorf("DeletePreview() = %+v, %v; want the configuration", preview, err)
	}
	if _, err := client.JiraProjects.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.JiraProjects.Get(ctx, created.ID); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}

	// Deleting the engagement deletes its configuration.
	if _, err := client.Engagements.Delete(ctx, engagement.ID); err != nil {
		t.Fatalf("deleting engagement: %v", err)
	}
	if _, _, err := client.JiraProjects.Get(ctx, forEngagement.ID); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(configuration of a deleted engagement) error = %v, want ErrNotFound", err)
	}
}

// checkSuppressSLANotificationComments checks suppress_sla_notification_comments,
// which DefectDojo 3.5.103 added, on the Jira project.
func checkSuppressSLANotificationComments(t *testing.T, client *defectdojo.Client, id, instanceID, productID int) {
	t.Helper()
	ctx := t.Context()
	version, err := client.ServerVersion(ctx)
	if err != nil {
		t.Fatalf("ServerVersion() error = %v", err)
	}
	patched, _, err := client.JiraProjects.Patch(ctx, id, &defectdojo.JiraProjectPatch{SuppressSLANotificationComments: new(true)})
	if err != nil {
		t.Fatalf("Patch(suppress) error = %v", err)
	}
	if defectdojo.CompareVersions(version, "3.5.103") < 0 {
		// Older servers drop the field.
		if patched.SuppressSLANotificationComments != nil {
			t.Errorf("SuppressSLANotificationComments = %v on %s, want nil", *patched.SuppressSLANotificationComments, version)
		}
		return
	}
	if patched.SuppressSLANotificationComments == nil || !*patched.SuppressSLANotificationComments {
		t.Errorf("SuppressSLANotificationComments = %v, want true", patched.SuppressSLANotificationComments)
	}
	// DefectDojo rejects both SLA settings; the client checks it before
	// sending, so send them raw.
	body := map[string]any{"jira_instance": instanceID, "product": productID, "product_jira_sla_notification": true, "suppress_sla_notification_comments": true}
	req, err := client.NewRequest(ctx, http.MethodPost, "/api/v2/jira_projects/", body)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(req, nil)
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || len(errResp.Fields["suppress_sla_notification_comments"]) == 0 {
		t.Errorf("POST with both SLA settings error = %v, want a 400 on suppress_sla_notification_comments", err)
	}
	if _, _, err := client.JiraProjects.Patch(ctx, id, &defectdojo.JiraProjectPatch{SuppressSLANotificationComments: new(false)}); err != nil {
		t.Errorf("Patch(suppress off) error = %v", err)
	}
}

// createJiraInstance creates a Jira instance, which is deleted when the test
// ends.
func createJiraInstance(t *testing.T, client *defectdojo.Client) int {
	t.Helper()
	name := randName(t, "acc")
	ji, _, err := client.JiraInstances.Create(t.Context(), &defectdojo.JiraInstanceWrite{
		JiraInstance: defectdojo.JiraInstance{
			ConfigurationName: name, URL: "https://" + name + ".example.invalid", Username: "bot",
			InfoMappingSeverity: "Lowest", LowMappingSeverity: "Low", MediumMappingSeverity: "Medium",
			HighMappingSeverity: "High", CriticalMappingSeverity: "Highest",
		},
		Password: "fake-token",
	})
	if err != nil {
		t.Fatalf("creating Jira instance: %v", err)
	}
	deleteJiraInstanceAtCleanup(t, client, ji.ID)
	return ji.ID
}

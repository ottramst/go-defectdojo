package defectdojo_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

func TestJiraProjectsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/jira_projects/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "enabled=true&jira_instance=1&product=7")
		writeJSON(t, w, http.StatusOK, fixture(t, "jira_projects/list_3.4.json"))
	})

	list, _, err := client.JiraProjects.List(context.Background(), &defectdojo.ListJiraProjectsOptions{
		JiraInstance: 1, Product: 7, Enabled: new(true),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 1 || len(list.JiraProjects) != 1 || list.JiraProjects[0].ProjectKey != "SEC" {
		t.Errorf("List() = %+v, want the configuration of the fixture", list)
	}
}

func TestJiraProjectsService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/jira_projects/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":"x","results":[{"id":1}]}`))
		case "1":
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":null,"results":[{"id":2}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})

	var ids []int
	for p, err := range client.JiraProjects.ListAll(context.Background(), &defectdojo.ListJiraProjectsOptions{ListOptions: defectdojo.ListOptions{Limit: 1}}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		ids = append(ids, p.ID)
	}
	if want := []int{1, 2}; !reflect.DeepEqual(ids, want) {
		t.Errorf("IDs = %v, want %v", ids, want)
	}
}

func TestJiraProjectsService_Get(t *testing.T) {
	for _, tt := range []struct {
		fixture  string
		suppress *bool
	}{
		{"jira_projects/get.json", new(false)},
		// DefectDojo before 3.5.103 has no suppress_sla_notification_comments.
		{"jira_projects/get_3.4.json", nil},
	} {
		t.Run(tt.fixture, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("GET /api/v2/jira_projects/2/", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, http.StatusOK, fixture(t, tt.fixture))
			})

			p, _, err := client.JiraProjects.Get(context.Background(), 2)
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if p.ID != 2 || p.JiraInstance == 0 || p.Product == 0 || p.Engagement != 0 || p.ProjectKey != "SEC" || p.Component != "web,api" ||
				p.JiraLabels != "security defectdojo" || !p.PushAllIssues || !p.Enabled || p.EpicIssueTypeName != "Epic" || p.DefaultAssignee != "" {
				t.Errorf("Get() = %+v, want the product configuration of the fixture", p)
			}
			if !jsonEqual(t, p.CustomFields, `{"customfield_10122":[{"name":"8.0.1"}]}`) || !jsonEqual(t, p.CloseTransitionFields, `{"resolution":{"name":"Won't Fix"}}`) {
				t.Errorf("Get() JSON fields = %s, %s", p.CustomFields, p.CloseTransitionFields)
			}
			// null decodes as nil, not as the JSON null.
			if p.ReopenTransitionFields != nil {
				t.Errorf("ReopenTransitionFields = %q, want nil", p.ReopenTransitionFields)
			}
			if !reflect.DeepEqual(p.SuppressSLANotificationComments, tt.suppress) {
				t.Errorf("SuppressSLANotificationComments = %v, want %v", p.SuppressSLANotificationComments, tt.suppress)
			}
			if !p.ArchivedAt.IsZero() || p.ArchiveBatch != "" {
				t.Errorf("archive fields = %v, %q; want zero", p.ArchivedAt, p.ArchiveBatch)
			}
		})
	}
}

func TestJiraProject_UnmarshalJSON(t *testing.T) {
	var p defectdojo.JiraProject
	if err := json.Unmarshal([]byte(`{"id":1,"custom_fields":{},"close_transition_fields":null,"reopen_transition_fields":[1]}`), &p); err != nil {
		t.Fatal(err)
	}
	if string(p.CustomFields) != "{}" || p.CloseTransitionFields != nil || string(p.ReopenTransitionFields) != "[1]" {
		t.Errorf("JiraProject = %+v, want {} kept, null as nil and the list kept", p)
	}
	if err := json.Unmarshal([]byte(`{"id":"x"}`), &p); err == nil {
		t.Error("Unmarshal(bad ID) error = nil, want an error")
	}
}

func TestJiraProjectsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/jira_projects/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{
			"jira_instance": 1,
			"product": 7,
			"engagement": null,
			"project_key": "SEC",
			"issue_template_dir": null,
			"component": "web,api",
			"custom_fields": {"customfield_10122": [{"name": "8.0.1"}]},
			"close_transition_fields": null,
			"reopen_transition_fields": null,
			"default_assignee": null,
			"jira_labels": "security defectdojo",
			"add_vulnerability_id_to_jira_label": false,
			"push_all_issues": true,
			"enable_engagement_epic_mapping": false,
			"push_notes": false,
			"product_jira_sla_notification": false,
			"suppress_sla_notification_comments": true,
			"risk_acceptance_expiration_notification": false,
			"enabled": true
		}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "jira_projects/create_3.4.json"))
	})

	in := defectdojo.JiraProject{
		ID:                              99, // read-only, not sent
		JiraInstance:                    1,
		Product:                         7,
		ProjectKey:                      "SEC",
		Component:                       "web,api",
		CustomFields:                    json.RawMessage(`{"customfield_10122": [{"name": "8.0.1"}]}`),
		JiraLabels:                      "security defectdojo",
		PushAllIssues:                   true,
		SuppressSLANotificationComments: new(true),
		Enabled:                         true,
	}
	created, _, err := client.JiraProjects.Create(context.Background(), &in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != 2 || created.ProjectKey != "SEC" {
		t.Errorf("Create() = %+v, want the created configuration", created)
	}
	if in.ID != 99 || *in.SuppressSLANotificationComments != true {
		t.Errorf("Create() changed its argument to %+v", in)
	}
}

func TestJiraProjectsService_Create_invalid(t *testing.T) {
	_, client := setup(t)
	valid := defectdojo.JiraProject{JiraInstance: 1, Product: 7}
	tests := map[string]func(*defectdojo.JiraProject){
		"no Jira instance":         func(p *defectdojo.JiraProject) { p.JiraInstance = 0 },
		"no product or engagement": func(p *defectdojo.JiraProject) { p.Product = 0 },
		"product and engagement":   func(p *defectdojo.JiraProject) { p.Engagement = 3 },
		"both SLA settings": func(p *defectdojo.JiraProject) {
			p.ProductJiraSLANotification = true
			p.SuppressSLANotificationComments = new(true)
		},
		"invalid custom fields": func(p *defectdojo.JiraProject) { p.CustomFields = json.RawMessage(`{`) },
	}
	for name, mutate := range tests {
		p := valid
		mutate(&p)
		// The mux has no handler, so a request would fail with a 404.
		if _, _, err := client.JiraProjects.Create(context.Background(), &p); err == nil || errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("Create(%s) error = %v, want an error before sending", name, err)
		}
	}
	if _, _, err := client.JiraProjects.Create(context.Background(), nil); err == nil {
		t.Error("Create(nil) error = nil, want an error")
	}
}

func TestJiraProjectsService_Create_engagementTaken(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/jira_projects/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/jira_project_engagement_taken.json"))
	})
	_, _, err := client.JiraProjects.Create(context.Background(), &defectdojo.JiraProject{JiraInstance: 1, Engagement: 3})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || len(errResp.Fields["engagement"]) == 0 {
		t.Errorf("Create() error = %v, want a 400 on engagement", err)
	}
}

func TestJiraProjectsService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/jira_projects/2/", func(w http.ResponseWriter, r *http.Request) {
		// An empty epic issue type and a nil SuppressSLANotificationComments
		// are left out.
		testJSONBody(t, r, `{
			"jira_instance": 1, "product": null, "engagement": 3, "project_key": "", "issue_template_dir": null, "component": "",
			"custom_fields": null, "close_transition_fields": null, "reopen_transition_fields": null,
			"default_assignee": "alice", "jira_labels": null, "add_vulnerability_id_to_jira_label": false, "push_all_issues": false,
			"enable_engagement_epic_mapping": true, "push_notes": true, "product_jira_sla_notification": true,
			"risk_acceptance_expiration_notification": true, "enabled": false
		}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "jira_projects/get_3.4.json"))
	})

	_, _, err := client.JiraProjects.Update(context.Background(), 2, &defectdojo.JiraProject{
		JiraInstance: 1, Engagement: 3, DefaultAssignee: "alice", EnableEngagementEpicMapping: true, PushNotes: true,
		ProductJiraSLANotification: true, RiskAcceptanceExpirationNotification: true,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if _, _, err := client.JiraProjects.Update(context.Background(), 0, &defectdojo.JiraProject{JiraInstance: 1, Product: 7}); err == nil {
		t.Error("Update(0) error = nil, want an error")
	}
}

func TestJiraProjectsService_Patch(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PATCH /api/v2/jira_projects/2/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{"product":null,"engagement":3,"custom_fields":{"a":1},"close_transition_fields":null,"enabled":false}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "jira_projects/get_3.4.json"))
	})

	_, _, err := client.JiraProjects.Patch(context.Background(), 2, &defectdojo.JiraProjectPatch{
		Product:               defectdojo.Null[int](),
		Engagement:            defectdojo.Value(3),
		CustomFields:          defectdojo.Value(json.RawMessage(`{"a":1}`)),
		CloseTransitionFields: defectdojo.Null[json.RawMessage](),
		Enabled:               new(false),
	})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if _, _, err := client.JiraProjects.Patch(context.Background(), 2, &defectdojo.JiraProjectPatch{CustomFields: defectdojo.Value(json.RawMessage(`{`))}); err == nil {
		t.Error("Patch(invalid JSON) error = nil, want an error")
	}
	if _, _, err := client.JiraProjects.Patch(context.Background(), 2, nil); err == nil {
		t.Error("Patch(nil) error = nil, want an error")
	}
}

func TestJiraProjectsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/jira_projects/2/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v2/jira_projects/2/delete_preview/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "jira_projects/delete_preview.json"))
	})

	if _, err := client.JiraProjects.Delete(context.Background(), 2); err != nil {
		t.Errorf("Delete() error = %v", err)
	}
	preview, _, err := client.JiraProjects.DeletePreview(context.Background(), 2, nil)
	if err != nil || preview.Count != 1 || preview.Objects[0].Model != "JIRA_Project" {
		t.Errorf("DeletePreview() = %+v, %v; want the configuration", preview, err)
	}
	if _, err := client.JiraProjects.Delete(context.Background(), 0); err == nil {
		t.Error("Delete(0) error = nil, want an error")
	}
}

// jsonEqual reports whether got is the same JSON as want.
func jsonEqual(t *testing.T, got json.RawMessage, want string) bool {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(g, w)
}

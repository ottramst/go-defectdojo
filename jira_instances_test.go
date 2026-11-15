package defectdojo_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/ottramst/go-defectdojo"
)

// acmeJira is the Jira instance of the jira_instances fixtures.
var acmeJira = defectdojo.JiraInstance{
	ID:                        1,
	ConfigurationName:         "Acme Jira",
	URL:                       "https://jira.example.com",
	Username:                  "defectdojo-bot",
	DefaultIssueType:          "Security",
	EpicNameID:                10011,
	OpenStatusKey:             11,
	CloseStatusKey:            31,
	InfoMappingSeverity:       "Lowest",
	LowMappingSeverity:        "Low",
	MediumMappingSeverity:     "Medium",
	HighMappingSeverity:       "High",
	CriticalMappingSeverity:   "Highest",
	FindingText:               "Reported by DefectDojo",
	AcceptedMappingResolution: "Won't Fix",
	GlobalJiraSLANotification: true,
}

func TestJiraInstancesService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/jira_instances/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=10&url=https%3A%2F%2Fjira.example.com")
		writeJSON(t, w, http.StatusOK, fixture(t, "jira_instances/list.json"))
	})

	list, _, err := client.JiraInstances.List(context.Background(), &defectdojo.ListJiraInstancesOptions{
		ListOptions: defectdojo.ListOptions{Limit: 10},
		URL:         "https://jira.example.com",
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 1 || len(list.JiraInstances) != 1 || !reflect.DeepEqual(*list.JiraInstances[0], acmeJira) {
		t.Errorf("List() = %+v, want the Acme Jira fixture", list)
	}
}

func TestJiraInstancesService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/jira_instances/", func(w http.ResponseWriter, r *http.Request) {
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
	for ji, err := range client.JiraInstances.ListAll(context.Background(), &defectdojo.ListJiraInstancesOptions{ListOptions: defectdojo.ListOptions{Limit: 1}}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		ids = append(ids, ji.ID)
	}
	if want := []int{1, 2}; !reflect.DeepEqual(ids, want) {
		t.Errorf("IDs = %v, want %v", ids, want)
	}
}

func TestJiraInstancesService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/jira_instances/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "1" {
			writeJSON(t, w, http.StatusNotFound, []byte(notFound))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "jira_instances/get.json"))
	})

	ji, _, err := client.JiraInstances.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !reflect.DeepEqual(*ji, acmeJira) {
		t.Errorf("Get() = %+v, want %+v", *ji, acmeJira)
	}
	if _, _, err := client.JiraInstances.Get(context.Background(), 2); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.JiraInstances.Get(context.Background(), 0); err == nil {
		t.Error("Get(0) error = nil, want an error")
	}
}

func TestJiraInstancesService_GetByName(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/jira_instances/", func(w http.ResponseWriter, r *http.Request) {
		// There is no name filter: GetByName pages through all of them.
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=100")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":4,"results":[{"id":1,"configuration_name":"Acme"},{"id":2,"configuration_name":"twice"}]}`))
		case "2":
			writeJSON(t, w, http.StatusOK, []byte(`{"count":4,"results":[{"id":3,"configuration_name":"twice"},{"id":4,"configuration_name":"Other"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})
	ctx := context.Background()

	if ji, _, err := client.JiraInstances.GetByName(ctx, "Other"); err != nil || ji.ID != 4 {
		t.Errorf("GetByName(Other) = %v, %v; want ID 4 from the second page", ji, err)
	}
	if _, _, err := client.JiraInstances.GetByName(ctx, "acme"); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(acme) error = %v, want ErrNotFound", err)
	}
	_, _, err := client.JiraInstances.GetByName(ctx, "twice")
	if !errors.Is(err, defectdojo.ErrAmbiguous) || errors.Is(err, defectdojo.ErrNotFound) || !strings.Contains(err.Error(), "[2 3]") {
		t.Errorf("GetByName(twice) error = %v, want an error naming IDs 2 and 3", err)
	}
	if _, _, err := client.JiraInstances.GetByName(ctx, ""); err == nil {
		t.Error("GetByName(\"\") error = nil, want an error")
	}
}

func TestJiraInstancesService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/jira_instances/", func(w http.ResponseWriter, r *http.Request) {
		testJSONBody(t, r, `{
			"configuration_name": "Acme Jira",
			"url": "https://jira.example.com",
			"username": "defectdojo-bot",
			"password": "fake-token",
			"default_issue_type": "Security",
			"issue_template_dir": null,
			"epic_name_id": 10011,
			"open_status_key": 11,
			"close_status_key": 31,
			"info_mapping_severity": "Lowest",
			"low_mapping_severity": "Low",
			"medium_mapping_severity": "Medium",
			"high_mapping_severity": "High",
			"critical_mapping_severity": "Highest",
			"finding_text": "Reported by DefectDojo",
			"accepted_mapping_resolution": "Won't Fix",
			"false_positive_mapping_resolution": null,
			"global_jira_sla_notification": true,
			"finding_jira_sync": false
		}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "jira_instances/create.json"))
	})

	in := defectdojo.JiraInstanceWrite{JiraInstance: acmeJira, Password: "fake-token"}
	in.ID = 99 // read-only, not sent
	orig := in
	created, _, err := client.JiraInstances.Create(context.Background(), &in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !reflect.DeepEqual(*created, acmeJira) {
		t.Errorf("Create() = %+v, want %+v", *created, acmeJira)
	}
	if in != orig {
		t.Errorf("Create() changed its argument to %+v", in)
	}
}

func TestJiraInstancesService_Create_defaults(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/jira_instances/", func(w http.ResponseWriter, r *http.Request) {
		// DefectDojo rejects "" for the name and the issue type, which have
		// defaults, so they are left out.
		testJSONBody(t, r, `{
			"url": "https://jira.example.com", "username": "bot", "password": "x",
			"issue_template_dir": null, "epic_name_id": 0, "open_status_key": 0, "close_status_key": 0,
			"info_mapping_severity": "a", "low_mapping_severity": "a", "medium_mapping_severity": "a",
			"high_mapping_severity": "a", "critical_mapping_severity": "a",
			"finding_text": null, "accepted_mapping_resolution": null, "false_positive_mapping_resolution": null,
			"global_jira_sla_notification": false, "finding_jira_sync": false
		}`)
		writeJSON(t, w, http.StatusCreated, []byte(`{"id":2}`))
	})

	_, _, err := client.JiraInstances.Create(context.Background(), &defectdojo.JiraInstanceWrite{
		JiraInstance: defectdojo.JiraInstance{
			URL: "https://jira.example.com", Username: "bot",
			InfoMappingSeverity: "a", LowMappingSeverity: "a", MediumMappingSeverity: "a", HighMappingSeverity: "a", CriticalMappingSeverity: "a",
		},
		Password: "x",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestJiraInstancesService_Create_invalid(t *testing.T) {
	_, client := setup(t)
	valid := defectdojo.JiraInstanceWrite{JiraInstance: acmeJira, Password: "x"}
	tests := map[string]func(*defectdojo.JiraInstanceWrite){
		"no URL":              func(ji *defectdojo.JiraInstanceWrite) { ji.URL = "" },
		"no username":         func(ji *defectdojo.JiraInstanceWrite) { ji.Username = "" },
		"no password":         func(ji *defectdojo.JiraInstanceWrite) { ji.Password = "" },
		"no critical mapping": func(ji *defectdojo.JiraInstanceWrite) { ji.CriticalMappingSeverity = "" },
		"no info mapping":     func(ji *defectdojo.JiraInstanceWrite) { ji.InfoMappingSeverity = "" },
	}
	for name, mutate := range tests {
		ji := valid
		mutate(&ji)
		// The mux has no handler, so a request would fail with a 404.
		if _, _, err := client.JiraInstances.Create(context.Background(), &ji); err == nil || errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("Create(%s) error = %v, want an error before sending", name, err)
		}
	}
	if _, _, err := client.JiraInstances.Create(context.Background(), nil); err == nil {
		t.Error("Create(nil) error = nil, want an error")
	}
}

func TestJiraInstancesService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/jira_instances/1/", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["password"] != "new-token" || body["url"] != "https://jira.example.com" {
			t.Errorf("body = %v, want the password and the URL", body)
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "jira_instances/get.json"))
	})

	if _, _, err := client.JiraInstances.Update(context.Background(), 1, &defectdojo.JiraInstanceWrite{JiraInstance: acmeJira, Password: "new-token"}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if _, _, err := client.JiraInstances.Update(context.Background(), 0, &defectdojo.JiraInstanceWrite{JiraInstance: acmeJira, Password: "x"}); err == nil {
		t.Error("Update(0) error = nil, want an error")
	}
}

func TestJiraInstancesService_Patch(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PATCH /api/v2/jira_instances/1/", func(w http.ResponseWriter, r *http.Request) {
		// The client doesn't check that a URL change has a password.
		testJSONBody(t, r, `{"url":"https://jira2.example.com","finding_text":null}`)
		writeJSON(t, w, http.StatusBadRequest, fixture(t, "errors/jira_url_without_password.json"))
	})

	_, _, err := client.JiraInstances.Patch(context.Background(), 1, &defectdojo.JiraInstancePatch{
		URL:         new("https://jira2.example.com"),
		FindingText: defectdojo.Null[string](),
	})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || len(errResp.Fields["password"]) == 0 {
		t.Errorf("Patch(URL) error = %v, want a 400 on password", err)
	}
	if _, _, err := client.JiraInstances.Patch(context.Background(), 1, nil); err == nil {
		t.Error("Patch(nil) error = nil, want an error")
	}
}

func TestJiraInstancesService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/jira_instances/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "1" {
			// DefectDojo doesn't map the ProtectedError of an instance in use.
			writeJSON(t, w, http.StatusInternalServerError, fixture(t, "errors/jira_instance_in_use.json"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if _, err := client.JiraInstances.Delete(context.Background(), 2); err != nil {
		t.Errorf("Delete() error = %v", err)
	}
	var errResp *defectdojo.ErrorResponse
	if _, err := client.JiraInstances.Delete(context.Background(), 1); !errors.As(err, &errResp) || errResp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Delete(in use) error = %v, want a 500", err)
	}
}

func TestJiraInstancesService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/jira_instances/1/delete_preview/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusOK, fixture(t, "jira_instances/delete_preview.json"))
	})

	preview, _, err := client.JiraInstances.DeletePreview(context.Background(), 1, nil)
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count != 1 || preview.Objects[0].Model != "JIRA_Instance" {
		t.Errorf("DeletePreview() = %+v, want the instance only", preview)
	}
}

func TestJiraInstancesService_CountJiraProjects(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/jira_projects/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("jira_instance") {
		case "1":
			testQuery(t, r, "jira_instance=1&limit=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"results":[{"id":5}]}`))
		default:
			writeJSON(t, w, http.StatusBadRequest, []byte(`{"jira_instance":["Select a valid choice. That choice is not one of the available choices."]}`))
		}
	})

	if n, _, err := client.JiraInstances.CountJiraProjects(context.Background(), 1); err != nil || n != 2 {
		t.Errorf("CountJiraProjects() = %d, %v; want 2", n, err)
	}
	if _, _, err := client.JiraInstances.CountJiraProjects(context.Background(), 9); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("CountJiraProjects(missing) error = %v, want ErrNotFound", err)
	}
}

func TestJiraInstancesService_invalidID(t *testing.T) {
	_, client := setup(t)
	ctx := context.Background()
	// The mux has no handler, so a request would fail with a 404.
	checks := map[string]error{}
	_, _, checks["Patch"] = client.JiraInstances.Patch(ctx, 0, &defectdojo.JiraInstancePatch{})
	_, checks["Delete"] = client.JiraInstances.Delete(ctx, 0)
	_, _, checks["DeletePreview"] = client.JiraInstances.DeletePreview(ctx, 0, nil)
	_, _, checks["CountJiraProjects"] = client.JiraInstances.CountJiraProjects(ctx, -1)
	for name, err := range checks {
		if err == nil || errors.Is(err, defectdojo.ErrNotFound) {
			t.Errorf("%s(0) error = %v, want an error before sending", name, err)
		}
	}
}

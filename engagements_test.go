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

func TestEngagementsService_List(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/engagements/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "active=true&limit=2&name=CI&offset=4&product=1&status=In+Progress")
		writeJSON(t, w, http.StatusOK, fixture(t, "engagements/list.json"))
	})

	list, _, err := client.Engagements.List(context.Background(), &defectdojo.ListEngagementsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 2, Offset: 4},
		Product:     1,
		Name:        "CI",
		Status:      defectdojo.EngagementStatusInProgress,
		Active:      new(true),
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list.Count != 2 || len(list.Engagements) != 2 {
		t.Fatalf("List() = %d of %d, want 2 of 2", len(list.Engagements), list.Count)
	}
	// An engagement created with only the required fields: null fields
	// decode as zero values, and DefectDojo named it after its target start.
	e := list.Engagements[1]
	if e.Name != "2026-11-01" || e.Description != "" || e.Lead != 0 || !e.FirstContacted.IsZero() ||
		e.Status != defectdojo.EngagementStatusNotStarted || e.EngagementType != defectdojo.EngagementTypeInteractive {
		t.Errorf("second engagement = %+v, want DefectDojo's defaults", e)
	}
}

func TestEngagementsService_ListAll(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/engagements/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "":
			testQuery(t, r, "limit=1&product=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":"http://x/api/v2/engagements/?limit=1&offset=1","results":[{"id":1,"name":"a"}]}`))
		case "1":
			testQuery(t, r, "limit=1&offset=1&product=1")
			writeJSON(t, w, http.StatusOK, []byte(`{"count":2,"next":null,"results":[{"id":2,"name":"b"}]}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
	})

	var names []string
	for e, err := range client.Engagements.ListAll(context.Background(), &defectdojo.ListEngagementsOptions{
		ListOptions: defectdojo.ListOptions{Limit: 1},
		Product:     1,
	}) {
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		names = append(names, e.Name)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestEngagementsService_Get(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/engagements/{id}/", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "1" {
			writeJSON(t, w, http.StatusNotFound, []byte(`{"detail":"No Engagement matches the given query."}`))
			return
		}
		writeJSON(t, w, http.StatusOK, fixture(t, "engagements/get.json"))
	})

	e, _, err := client.Engagements.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	want := &defectdojo.Engagement{
		ID:                      1,
		Product:                 1,
		Name:                    "CI",
		Description:             "Scans of the main branch",
		Version:                 "1.4.0",
		TargetStart:             defectdojo.Date{Year: 2026, Month: time.November, Day: 1},
		TargetEnd:               defectdojo.Date{Year: 2026, Month: time.December, Day: 31},
		FirstContacted:          defectdojo.Date{Year: 2026, Month: time.October, Day: 15},
		Status:                  defectdojo.EngagementStatusInProgress,
		EngagementType:          defectdojo.EngagementTypeCICD,
		Active:                  true,
		SourceCodeManagementURI: "https://git.example/shop",
		BuildID:                 "42",
		CommitHash:              "abc123",
		BranchTag:               "main",
		ThreatModel:             true,
		APITest:                 true,
		PenTest:                 true,
		CheckList:               true,
		Lead:                    1,
		Tags:                    []string{"ci"},
		Progress:                "threat_model",
		TmodelPath:              "none",
		RiskAcceptance:          []int{},
		Created:                 e.Created,
		Updated:                 e.Updated,
	}
	if !reflect.DeepEqual(e, want) {
		t.Errorf("Get() = %+v, want %+v", e, want)
	}
	if e.Created.Year() != 2026 || e.Updated.Before(e.Created) {
		t.Errorf("Created, Updated = %v, %v", e.Created, e.Updated)
	}

	if _, _, err := client.Engagements.Get(context.Background(), 5); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestEngagementsService_Create(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("POST /api/v2/engagements/", func(w http.ResponseWriter, r *http.Request) {
		testHeader(t, r, "Content-Type", "application/json")
		// Read-only fields, an empty status and engagement type, and nil tags
		// are not sent; empty nullable fields are null.
		testJSONBody(t, r, `{
			"product":1,"name":null,"description":null,"reason":null,"version":null,
			"target_start":"2026-11-01","target_end":"2026-12-31","first_contacted":null,
			"tracker":null,"test_strategy":null,"source_code_management_uri":null,
			"build_id":"42","commit_hash":null,"branch_tag":null,
			"threat_model":true,"api_test":false,"pen_test":false,"check_list":false,"deduplication_on_engagement":false,
			"lead":null,"requester":null,"preset":null,"report_type":null,
			"cicd_scm_server":null,"cicd_build_server":null,"cicd_orchestration_engine":null
		}`)
		writeJSON(t, w, http.StatusCreated, fixture(t, "engagements/get.json"))
	})

	in := &defectdojo.Engagement{
		ID: 99, Product: 1, BuildID: "42", ThreatModel: true, Active: true, Progress: "x",
		TargetStart: defectdojo.Date{Year: 2026, Month: time.November, Day: 1},
		TargetEnd:   defectdojo.Date{Year: 2026, Month: time.December, Day: 31},
	}
	e, resp, err := client.Engagements.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || e.ID != 1 {
		t.Errorf("Create() = %d %+v", resp.StatusCode, e)
	}
	if in.ID != 99 || in.Tags != nil {
		t.Errorf("Create() changed the caller's engagement: %+v", in)
	}
}

func TestEngagementsService_Update(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PUT /api/v2/engagements/1/", func(w http.ResponseWriter, r *http.Request) {
		// A non-nil empty Tags is sent, and removes all tags.
		testJSONBody(t, r, `{
			"product":2,"name":"CI","description":"d","reason":"r","version":"1",
			"target_start":"2026-11-01","target_end":"2026-12-31","first_contacted":"2026-10-15",
			"status":"Completed","engagement_type":"CI/CD",
			"tracker":"https://t.example/1","test_strategy":"https://t.example/s","source_code_management_uri":"https://git.example/shop",
			"build_id":"42","commit_hash":"abc","branch_tag":"main",
			"threat_model":false,"api_test":true,"pen_test":false,"check_list":true,"deduplication_on_engagement":true,
			"lead":1,"requester":2,"preset":3,"report_type":4,
			"cicd_scm_server":5,"cicd_build_server":6,"cicd_orchestration_engine":7,
			"tags":[]
		}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "engagements/get.json"))
	})

	_, _, err := client.Engagements.Update(context.Background(), 1, &defectdojo.Engagement{
		Product: 2, Name: "CI", Description: "d", Reason: "r", Version: "1",
		TargetStart:    defectdojo.Date{Year: 2026, Month: time.November, Day: 1},
		TargetEnd:      defectdojo.Date{Year: 2026, Month: time.December, Day: 31},
		FirstContacted: defectdojo.Date{Year: 2026, Month: time.October, Day: 15},
		Status:         defectdojo.EngagementStatusCompleted, EngagementType: defectdojo.EngagementTypeCICD,
		Tracker: "https://t.example/1", TestStrategy: "https://t.example/s", SourceCodeManagementURI: "https://git.example/shop",
		BuildID: "42", CommitHash: "abc", BranchTag: "main",
		APITest: true, CheckList: true, DeduplicationOnEngagement: true,
		Lead: 1, Requester: 2, Preset: 3, ReportType: 4,
		CICDSCMServer: 5, CICDBuildServer: 6, CICDOrchestrationEngine: 7,
		Tags: []string{},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestEngagementsService_Patch(t *testing.T) {
	end := defectdojo.Date{Year: 2027, Month: time.March, Day: 1}
	tests := []struct {
		name  string
		patch *defectdojo.EngagementPatch
		want  string
	}{
		{name: "empty", patch: &defectdojo.EngagementPatch{}, want: `{}`},
		{
			name: "clear nullable fields",
			patch: &defectdojo.EngagementPatch{
				Description: defectdojo.Null[string](), FirstContacted: defectdojo.Null[defectdojo.Date](), Lead: defectdojo.Null[int](),
			},
			want: `{"description":null,"first_contacted":null,"lead":null}`,
		},
		{
			name: "set fields",
			patch: &defectdojo.EngagementPatch{
				Product: new(2), TargetEnd: &end, Status: new(defectdojo.EngagementStatusOnHold),
				Name: defectdojo.Value("CI"), PenTest: new(false), Tags: new([]string{"a"}),
			},
			want: `{"product":2,"target_end":"2027-03-01","status":"On Hold","name":"CI","pen_test":false,"tags":["a"]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, client := setup(t)
			mux.HandleFunc("PATCH /api/v2/engagements/1/", func(w http.ResponseWriter, r *http.Request) {
				testJSONBody(t, r, tt.want)
				writeJSON(t, w, http.StatusOK, fixture(t, "engagements/get.json"))
			})
			if _, _, err := client.Engagements.Patch(context.Background(), 1, tt.patch); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}
		})
	}
}

func TestEngagementsService_Patch_nilTags(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("PATCH /api/v2/engagements/1/", func(w http.ResponseWriter, r *http.Request) {
		// null is a 400 in DefectDojo, so a nil slice is sent as [].
		testJSONBody(t, r, `{"tags":[]}`)
		writeJSON(t, w, http.StatusOK, fixture(t, "engagements/get.json"))
	})
	var tags []string
	patch := &defectdojo.EngagementPatch{Tags: &tags}
	if _, _, err := client.Engagements.Patch(context.Background(), 1, patch); err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if patch.Tags != &tags || tags != nil {
		t.Error("Patch() changed the caller's patch")
	}
}

func TestEngagementsService_Delete(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("DELETE /api/v2/engagements/1/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	resp, err := client.Engagements.Delete(context.Background(), 1)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("StatusCode = %d, want 204", resp.StatusCode)
	}
}

func TestEngagementsService_DeletePreview(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/engagements/1/delete_preview/", func(w http.ResponseWriter, r *http.Request) {
		testQuery(t, r, "limit=50")
		writeJSON(t, w, http.StatusOK, fixture(t, "engagements/delete_preview.json"))
	})
	preview, _, err := client.Engagements.DeletePreview(context.Background(), 1, &defectdojo.ListOptions{Limit: 50})
	if err != nil {
		t.Fatalf("DeletePreview() error = %v", err)
	}
	if preview.Count == 0 || preview.Objects[0].Model != "Engagement" || preview.Objects[0].ID != 1 {
		t.Errorf("DeletePreview() = %+v, want the engagement first", preview.Objects)
	}
}

func TestEngagementsService_CloseReopen(t *testing.T) {
	mux, client := setup(t)
	for _, action := range []string{"close", "reopen"} {
		mux.HandleFunc("POST /api/v2/engagements/1/"+action+"/", func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > 0 {
				t.Errorf("%s: request body of %d bytes, want none", action, r.ContentLength)
			}
			writeJSON(t, w, http.StatusOK, []byte(`{}`))
		})
	}
	if _, err := client.Engagements.Close(context.Background(), 1); err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if _, err := client.Engagements.Reopen(context.Background(), 1); err != nil {
		t.Errorf("Reopen() error = %v", err)
	}
}

func TestEngagementsService_CountTests(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("GET /api/v2/tests/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("engagement") != "1" {
			// The engagement filter rejects IDs that don't exist.
			writeJSON(t, w, http.StatusBadRequest, fixture(t, "engagements/count_tests_missing.json"))
			return
		}
		testQuery(t, r, "engagement=1&limit=1")
		writeJSON(t, w, http.StatusOK, fixture(t, "engagements/count_tests.json"))
	})
	n, _, err := client.Engagements.CountTests(context.Background(), 1)
	if err != nil || n != 1 {
		t.Errorf("CountTests() = %d, %v; want 1", n, err)
	}
	if _, _, err := client.Engagements.CountTests(context.Background(), 99999); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("CountTests(missing) error = %v, want ErrNotFound", err)
	}
}

func TestEngagementsService_invalidArguments(t *testing.T) {
	mux, client := setup(t)
	mux.HandleFunc("/", func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	})
	ctx := context.Background()
	day := defectdojo.Date{Year: 2026, Month: time.January, Day: 1}
	e := &defectdojo.Engagement{Product: 1, TargetStart: day, TargetEnd: day}
	calls := map[string]func() error{
		"Get zero ID":        func() error { _, _, err := client.Engagements.Get(ctx, 0); return err },
		"Update negative ID": func() error { _, _, err := client.Engagements.Update(ctx, -1, e); return err },
		"Update nil":         func() error { _, _, err := client.Engagements.Update(ctx, 1, nil); return err },
		"Create nil":         func() error { _, _, err := client.Engagements.Create(ctx, nil); return err },
		"Create zero product": func() error {
			_, _, err := client.Engagements.Create(ctx, &defectdojo.Engagement{TargetStart: day, TargetEnd: day})
			return err
		},
		"Create zero target start": func() error {
			_, _, err := client.Engagements.Create(ctx, &defectdojo.Engagement{Product: 1, TargetEnd: day})
			return err
		},
		"Create zero target end": func() error {
			_, _, err := client.Engagements.Create(ctx, &defectdojo.Engagement{Product: 1, TargetStart: day})
			return err
		},
		"Patch zero ID": func() error {
			_, _, err := client.Engagements.Patch(ctx, 0, &defectdojo.EngagementPatch{})
			return err
		},
		"Patch nil": func() error { _, _, err := client.Engagements.Patch(ctx, 1, nil); return err },
		"Patch zero target start": func() error {
			_, _, err := client.Engagements.Patch(ctx, 1, &defectdojo.EngagementPatch{TargetStart: &defectdojo.Date{}})
			return err
		},
		"Patch zero target end": func() error {
			_, _, err := client.Engagements.Patch(ctx, 1, &defectdojo.EngagementPatch{TargetEnd: &defectdojo.Date{}})
			return err
		},
		"Delete zero ID":        func() error { _, err := client.Engagements.Delete(ctx, 0); return err },
		"DeletePreview zero ID": func() error { _, _, err := client.Engagements.DeletePreview(ctx, 0, nil); return err },
		"Close zero ID":         func() error { _, err := client.Engagements.Close(ctx, 0); return err },
		"Reopen zero ID":        func() error { _, err := client.Engagements.Reopen(ctx, 0); return err },
		"CountTests zero ID":    func() error { _, _, err := client.Engagements.CountTests(ctx, 0); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("error = nil, want error")
			}
		})
	}
}

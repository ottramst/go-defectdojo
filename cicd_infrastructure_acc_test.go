package defectdojo_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ottramst/go-defectdojo"
)

func TestAccCICDInfrastructure(t *testing.T) {
	client := accClient(t)
	ctx := t.Context()
	name := randName(t, "acc")

	create := func(ci *defectdojo.CICDInfrastructure) *defectdojo.CICDInfrastructure {
		t.Helper()
		created, resp, err := client.CICDInfrastructure.Create(ctx, ci)
		if err != nil {
			t.Fatalf("Create(%s) error = %v", ci.InfrastructureType, err)
		}
		cleanup(t, func(ctx context.Context) error {
			_, err := client.CICDInfrastructure.Delete(ctx, created.ID)
			if errors.Is(err, defectdojo.ErrNotFound) {
				return nil
			}
			return err
		})
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Create() status = %d, want 201", resp.StatusCode)
		}
		return created
	}
	scm := create(&defectdojo.CICDInfrastructure{
		Name: name, InfrastructureType: defectdojo.CICDInfrastructureTypeSCMServer,
		Description: "acceptance test", URL: "https://scm.example.com",
	})
	want := defectdojo.CICDInfrastructure{
		ID: scm.ID, Name: name, InfrastructureType: "scm_server", Description: "acceptance test", URL: "https://scm.example.com",
	}
	if *scm != want {
		t.Errorf("Create() = %+v, want %+v", scm, want)
	}
	// The same name with another type is another object.
	build := create(&defectdojo.CICDInfrastructure{Name: name, InfrastructureType: defectdojo.CICDInfrastructureTypeBuildServer})
	orchestration := create(&defectdojo.CICDInfrastructure{Name: name + "-k8s", InfrastructureType: defectdojo.CICDInfrastructureTypeOrchestration})

	// The same name and type is a 400 on create.
	_, _, err := client.CICDInfrastructure.Create(ctx, &defectdojo.CICDInfrastructure{Name: name, InfrastructureType: "scm_server"})
	var errResp *defectdojo.ErrorResponse
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["non_field_errors"]) == 0 {
		t.Errorf("Create(same name and type) error = %v, want a 400 with non_field_errors", err)
	}
	// DefectDojo checks the type.
	if _, _, err := client.CICDInfrastructure.Create(ctx, &defectdojo.CICDInfrastructure{Name: name, InfrastructureType: "SCM"}); !errors.Is(err, defectdojo.ErrBadRequest) {
		t.Errorf("Create(unknown type) error = %v, want ErrBadRequest", err)
	}

	got, _, err := client.CICDInfrastructure.Get(ctx, scm.ID)
	if err != nil || *got != want {
		t.Errorf("Get() = %+v, %v; want %+v", got, err, want)
	}
	byName, _, err := client.CICDInfrastructure.GetByName(ctx, name, defectdojo.CICDInfrastructureTypeBuildServer)
	if err != nil || byName.ID != build.ID {
		t.Errorf("GetByName(build server) = %+v, %v; want ID %d", byName, err, build.ID)
	}
	if _, _, err := client.CICDInfrastructure.GetByName(ctx, name, ""); err == nil || errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("GetByName(any type) error = %v, want an error for the two types", err)
	}
	if k8s, _, err := client.CICDInfrastructure.GetByName(ctx, name+"-k8s", ""); err != nil || k8s.ID != orchestration.ID {
		t.Errorf("GetByName(unique name, any type) = %+v, %v; want ID %d", k8s, err, orchestration.ID)
	}

	// Update ignores the type, which can't change.
	updated, _, err := client.CICDInfrastructure.Update(ctx, scm.ID, &defectdojo.CICDInfrastructure{
		Name: name + "-renamed", InfrastructureType: defectdojo.CICDInfrastructureTypeOrchestration,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if *updated != (defectdojo.CICDInfrastructure{ID: scm.ID, Name: name + "-renamed", InfrastructureType: "scm_server"}) {
		t.Errorf("Update() = %+v, want the new name, the type kept and the description and URL cleared", updated)
	}
	patched, _, err := client.CICDInfrastructure.Patch(ctx, scm.ID, &defectdojo.CICDInfrastructurePatch{Name: new(name), URL: new("https://git.example.com")})
	if err != nil || patched.Name != name || patched.URL != "https://git.example.com" {
		t.Errorf("Patch() = %+v, %v; want the name and URL", patched, err)
	}
	// Renaming onto a name of the same type is a 409.
	if _, _, err := client.CICDInfrastructure.Patch(ctx, orchestration.ID, &defectdojo.CICDInfrastructurePatch{Name: new(name + "-k8s")}); err != nil {
		t.Fatalf("Patch(same name) error = %v", err)
	}
	other := create(&defectdojo.CICDInfrastructure{Name: name + "-other", InfrastructureType: "scm_server"})
	if _, _, err := client.CICDInfrastructure.Patch(ctx, other.ID, &defectdojo.CICDInfrastructurePatch{Name: new(name)}); !errors.Is(err, defectdojo.ErrConflict) {
		t.Errorf("Patch(taken name) error = %v, want ErrConflict", err)
	}

	// Engagements link to one object of each type.
	product := createProductIn(t, client, createProductType(t, client), false)
	day := defectdojo.Date{Year: 2026, Month: time.January, Day: 1}
	engagement, _, err := client.Engagements.Create(ctx, &defectdojo.Engagement{
		Product: product, TargetStart: day, TargetEnd: day,
		CICDSCMServer: scm.ID, CICDBuildServer: build.ID, CICDOrchestrationEngine: orchestration.ID,
	})
	if err != nil {
		t.Fatalf("creating engagement: %v", err)
	}
	if engagement.CICDSCMServer != scm.ID || engagement.CICDBuildServer != build.ID || engagement.CICDOrchestrationEngine != orchestration.ID {
		t.Errorf("engagement links = %d %d %d, want %d %d %d", engagement.CICDSCMServer, engagement.CICDBuildServer,
			engagement.CICDOrchestrationEngine, scm.ID, build.ID, orchestration.ID)
	}
	// An object of the wrong type is a 400.
	_, _, err = client.Engagements.Patch(ctx, engagement.ID, &defectdojo.EngagementPatch{CICDSCMServer: defectdojo.Value(build.ID)})
	if !errors.As(err, &errResp) || !errors.Is(err, defectdojo.ErrBadRequest) || len(errResp.Fields["cicd_scm_server"]) == 0 {
		t.Errorf("linking a build server as the SCM server: error = %v, want a 400 on cicd_scm_server", err)
	}

	preview, _, err := client.CICDInfrastructure.DeletePreview(ctx, scm.ID, nil)
	if err != nil || preview.Count < 1 || preview.Objects[0].Model != "CICDInfrastructure" {
		t.Errorf("DeletePreview() = %+v, %v; want the object first", preview, err)
	}
	// Deleting an object clears the engagement's link.
	if _, err := client.CICDInfrastructure.Delete(ctx, scm.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, err := client.CICDInfrastructure.Get(ctx, scm.ID); !errors.Is(err, defectdojo.ErrNotFound) {
		t.Errorf("Get(deleted) error = %v, want ErrNotFound", err)
	}
	e, _, err := client.Engagements.Get(ctx, engagement.ID)
	if err != nil || e.CICDSCMServer != 0 || e.CICDBuildServer != build.ID {
		t.Errorf("engagement after the delete = %+v, %v; want the SCM link cleared and the build server kept", e, err)
	}
}

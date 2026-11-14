package defectdojo_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ottramst/go-defectdojo"
)

func ExampleNewClient() {
	client, err := defectdojo.NewClient("https://defectdojo.example.com",
		defectdojo.WithAPIToken(os.Getenv("DEFECTDOJO_API_TOKEN")))
	if err != nil {
		log.Fatal(err)
	}
	list, _, err := client.ProductTypes.List(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(list.Count, "product types")
}

func ExampleAuthService_Token() {
	ctx := context.Background()
	login, err := defectdojo.NewClient("https://defectdojo.example.com")
	if err != nil {
		log.Fatal(err)
	}
	token, _, err := login.Auth.Token(ctx, "admin", os.Getenv("DEFECTDOJO_PASSWORD"))
	if err != nil {
		log.Fatal(err)
	}
	client, err := defectdojo.NewClient("https://defectdojo.example.com", defectdojo.WithAPIToken(token))
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleProductTypesService_ListAll() {
	var client *defectdojo.Client // from NewClient
	for pt, err := range client.ProductTypes.ListAll(context.Background(), nil) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(pt.ID, pt.Name)
	}
}

func ExampleProductTypesService_Patch() {
	var client *defectdojo.Client // from NewClient
	// Only the fields that are set change: clear the description and mark
	// the product type as critical.
	_, _, err := client.ProductTypes.Patch(context.Background(), 4, &defectdojo.ProductTypePatch{
		Description:     defectdojo.Null[string](),
		CriticalProduct: new(true),
	})
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleProductTypesService_Delete() {
	var client *defectdojo.Client // from NewClient
	ctx := context.Background()
	// Deleting a product type deletes its products too: refuse if it has any.
	n, _, err := client.ProductTypes.CountProducts(ctx, 4)
	if err != nil {
		log.Fatal(err)
	}
	if n > 0 {
		log.Fatalf("product type 4 still has %d products", n)
	}
	if _, err := client.ProductTypes.Delete(ctx, 4); err != nil && !errors.Is(err, defectdojo.ErrNotFound) {
		log.Fatal(err)
	}
}

func ExampleProductsService_Create() {
	var client *defectdojo.Client // from NewClient
	// Fields left empty are sent as null; booleans that DefectDojo defaults
	// to true must be set.
	p, _, err := client.Products.Create(context.Background(), &defectdojo.Product{
		Name:                     "Storefront",
		Description:              "The online shop",
		ProdType:                 2,
		Platform:                 "web",
		BusinessCriticality:      defectdojo.BusinessCriticalityHigh,
		EnableFullRiskAcceptance: true,
		Tags:                     []string{"pci"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(p.ID)
}

func ExampleSLAConfigurationsService_Patch() {
	var client *defectdojo.Client // from NewClient
	_, _, err := client.SLAConfigurations.Patch(context.Background(), 3, &defectdojo.SLAConfigurationPatch{
		Critical: new(3),
		High:     new(14),
	})
	if defectdojo.IsSLARecalculating(err) {
		// DefectDojo is still recalculating the SLA dates of an earlier
		// change; try again once its Celery task is done.
		return
	}
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleEngagementsService_Create() {
	var client *defectdojo.Client // from NewClient
	// A long-lived engagement for a CI/CD pipeline's scans. Imports move the
	// target end to the scan date, so a far-away one keeps it stable.
	e, _, err := client.Engagements.Create(context.Background(), &defectdojo.Engagement{
		Product:        7,
		Name:           "CI",
		EngagementType: defectdojo.EngagementTypeCICD,
		TargetStart:    defectdojo.DateOf(time.Now()),
		TargetEnd:      defectdojo.Date{Year: 2099, Month: time.December, Day: 31},
		BranchTag:      "main",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(e.ID)
}

func ExampleUsersService_Create() {
	var client *defectdojo.Client // from NewClient
	ctx := context.Background()
	// Permission IDs differ between DefectDojo installations: look them up
	// by codename.
	perm, _, err := client.ConfigurationPermissions.GetByCodename(ctx, "add_product_type")
	if err != nil {
		log.Fatal(err)
	}
	u, _, err := client.Users.Create(ctx, &defectdojo.UserCreate{
		User: defectdojo.User{
			Username:                 "dependency-track",
			Email:                    "dependency-track@example.com",
			IsActive:                 true, // false unless set
			ConfigurationPermissions: []int{perm.ID},
		},
		Password: "Initial-Passw0rd!",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(u.ID)
}

func ExampleUserProfileService_Get() {
	var client *defectdojo.Client // from NewClient
	profile, _, err := client.UserProfile.Get(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	if !profile.User.IsSuperuser {
		log.Fatalf("%s is not a superuser", profile.User.Username)
	}
}

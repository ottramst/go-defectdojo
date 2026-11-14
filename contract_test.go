package defectdojo

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestContract checks the client's request and response types against the
// OpenAPI schema of every supported DefectDojo version. It catches fields
// that DefectDojo renames, removes, retypes or makes required, which happens
// in minor releases. Run it with `make contract`, which downloads the schemas
// into .cache/specs first.
func TestContract(t *testing.T) {
	dir := os.Getenv("DEFECTDOJO_SPEC_DIR")
	if dir == "" {
		t.Skip("set DEFECTDOJO_SPEC_DIR to a directory of defectdojo-<version>.json schemas, or run `make contract`")
	}
	versions := supportedVersions(t)
	for _, v := range versions {
		t.Run(v, func(t *testing.T) {
			spec := loadContractSpec(t, filepath.Join(dir, "defectdojo-"+v+".json"))
			for _, c := range contracts {
				t.Run(c.schema, func(t *testing.T) {
					checkContract(t, spec, c, v)
				})
			}
		})
	}
}

// A contract maps a Go type to the schema component it is sent as or decoded
// from.
type contract struct {
	value  any
	schema string

	// request types are sent: every field must be writable, and every
	// property the schema requires must be sent.
	request bool

	// since maps properties that newer DefectDojo versions added to the
	// first version that has them. They are not checked on older versions.
	since map[string]string

	// nonZero lists request properties that are never sent as null because
	// the request's constructor rejects their zero value.
	nonZero []string

	// specBug maps properties whose type in the schema is wrong to the
	// reason. Their type is not checked, and the test fails once the type
	// matches, so that the exception is removed.
	specBug map[string]string
}

// contracts lists every type the client sends or decodes. Add new types here.
var contracts = []contract{
	{value: ProductType{}, schema: "ProductType", since: map[string]string{"archived_at": "3.5.0", "archive_batch": "3.5.0"}},
	{value: productTypeRequest{}, schema: "ProductTypeRequest", request: true},
	{value: ProductTypePatch{}, schema: "PatchedProductTypeRequest", request: true},
	{value: Product{}, schema: "Product", since: map[string]string{"archived_at": "3.5.0", "archive_batch": "3.5.0"}},
	{value: ProductMeta{}, schema: "ProductMeta"},
	{value: productRequest{}, schema: "ProductRequest", request: true},
	{value: ProductPatch{}, schema: "PatchedProductRequest", request: true},
	{value: listPage[Product]{}, schema: "PaginatedProductList"},
	{value: Engagement{}, schema: "Engagement", since: map[string]string{"archived_at": "3.5.0", "archive_batch": "3.5.0"}},
	{value: engagementRequest{}, schema: "EngagementRequest", request: true, nonZero: []string{"target_start", "target_end"}},
	{value: EngagementPatch{}, schema: "PatchedEngagementRequest", request: true},
	{value: listPage[Engagement]{}, schema: "PaginatedEngagementList"},
	{value: SLAConfiguration{}, schema: "SLAConfiguration"},
	{value: slaConfigurationRequest{}, schema: "SLAConfigurationRequest", request: true},
	{value: SLAConfigurationPatch{}, schema: "PatchedSLAConfigurationRequest", request: true},
	{value: listPage[SLAConfiguration]{}, schema: "PaginatedSLAConfigurationList"},
	{value: SystemSettings{}, schema: "SystemSettings", since: map[string]string{"read_only_mode": "3.5.100"}},
	{value: SystemSettingsPatch{}, schema: "PatchedSystemSettingsRequest", request: true, since: map[string]string{"read_only_mode": "3.5.100"}},
	{value: listPage[SystemSettings]{}, schema: "PaginatedSystemSettingsList"},
	{value: userRequest{}, schema: "UserRequest", request: true},
	{value: UserPatch{}, schema: "PatchedUserRequest", request: true},
	{value: listPage[User]{}, schema: "PaginatedUserList"},
	{value: ConfigurationPermission{}, schema: "ConfigurationPermission"},
	{value: listPage[ConfigurationPermission]{}, schema: "PaginatedConfigurationPermissionList"},
	{value: NotificationWebhook{}, schema: "NotificationWebhooks"},
	{value: notificationWebhookRequest{}, schema: "NotificationWebhooksRequest", request: true},
	{value: NotificationWebhookPatch{}, schema: "PatchedNotificationWebhooksRequest", request: true},
	{value: listPage[NotificationWebhook]{}, schema: "PaginatedNotificationWebhooksList"},
	{value: NotificationSettings{}, schema: "Notifications", specBug: map[string]string{
		// A single choice in requests, but a list in responses.
		"scan_added_empty": "the model's MultiSelectField is serialized as a ChoiceField",
	}},
	{value: notificationSettingsRequest{}, schema: "NotificationsRequest", request: true},
	{value: NotificationSettingsPatch{}, schema: "PatchedNotificationsRequest", request: true},
	{value: listPage[NotificationSettings]{}, schema: "PaginatedNotificationsList"},
	{value: UserProfile{}, schema: "UserProfile"},
	{value: User{}, schema: "User"},
	{value: UserContactInfo{}, schema: "UserContactInfo"},
	{value: DeletedObject{}, schema: "DeletePreview"},
	{value: listPage[ProductType]{}, schema: "PaginatedProductTypeList"},
	{value: tokenRequest{}, schema: "ForcedResetAuthTokenRequest", request: true},
	{value: tokenResponse{}, schema: "ForcedResetAuthToken"},
}

// supportedVersions returns the versions to check: those in
// DEFECTDOJO_SPEC_VERSIONS (space separated), such as "nightly" for a schema
// downloaded from a server, or else the supported versions.
func supportedVersions(t *testing.T) []string {
	t.Helper()
	if v := strings.Fields(os.Getenv("DEFECTDOJO_SPEC_VERSIONS")); len(v) > 0 {
		return v
	}
	data, err := os.ReadFile(filepath.Join(".github", "defectdojo-versions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Versions []string `json:"versions"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Versions) == 0 {
		t.Fatal(".github/defectdojo-versions.json lists no versions")
	}
	return file.Versions
}

type openAPISpec struct {
	Components struct {
		Schemas map[string]*openAPISchema `json:"schemas"`
	} `json:"components"`
}

type openAPISchema struct {
	Ref        string                    `json:"$ref"`
	Type       string                    `json:"type"`
	Format     string                    `json:"format"`
	Nullable   bool                      `json:"nullable"`
	ReadOnly   bool                      `json:"readOnly"`
	Required   []string                  `json:"required"`
	Properties map[string]*openAPISchema `json:"properties"`
	Items      *openAPISchema            `json:"items"`
	AllOf      []*openAPISchema          `json:"allOf"`
	OneOf      []*openAPISchema          `json:"oneOf"`
}

func loadContractSpec(t *testing.T, path string) *openAPISpec {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run `make contract` to download the schemas)", err)
	}
	var spec openAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return &spec
}

// resolve follows $ref and single-element allOf wrappers, keeping the
// nullable and readOnly flags of the wrappers.
func (s *openAPISpec) resolve(p *openAPISchema) *openAPISchema {
	nullable, readOnly := p.Nullable, p.ReadOnly
	for range 10 {
		switch {
		case p.Ref != "":
			p = s.Components.Schemas[strings.TrimPrefix(p.Ref, "#/components/schemas/")]
		case len(p.AllOf) == 1:
			p = p.AllOf[0]
		default:
			r := *p
			r.Nullable = r.Nullable || nullable
			r.ReadOnly = r.ReadOnly || readOnly
			return &r
		}
		if p == nil {
			return &openAPISchema{}
		}
	}
	return p
}

var (
	timeType     = reflect.TypeFor[time.Time]()
	dateType     = reflect.TypeFor[Date]()
	nullableName = reflect.TypeFor[Nullable[int]]().Name()[:len("Nullable")]
)

func checkContract(t *testing.T, spec *openAPISpec, c contract, version string) {
	schema, ok := spec.Components.Schemas[c.schema]
	if !ok {
		t.Fatalf("schema %s does not exist", c.schema)
	}
	typ := reflect.TypeOf(c.value)
	sent := make(map[string]bool)
	for i := range typ.NumField() {
		f := typ.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.Anonymous || name == "" || name == "-" {
			continue
		}
		sent[name] = true
		// Versions that are not releases, such as "nightly", are the newest.
		release := version != "" && version[0] >= '0' && version[0] <= '9'
		if since, ok := c.since[name]; ok && release && compareVersions(version, since) < 0 {
			if _, ok := schema.Properties[name]; ok {
				t.Errorf("%s.%s: property %q exists before %s; update since", typ.Name(), f.Name, name, since)
			}
			continue
		}
		prop, ok := schema.Properties[name]
		if !ok {
			t.Errorf("%s.%s: property %q is not in %s", typ.Name(), f.Name, name, c.schema)
			continue
		}
		prop = spec.resolve(prop)
		if c.request && prop.ReadOnly {
			t.Errorf("%s.%s: property %q is read-only in %s", typ.Name(), f.Name, name, c.schema)
		}
		ft, nullable := unwrap(f.Type)
		if nullable && !prop.Nullable {
			t.Errorf("%s.%s: Nullable, but property %q is not nullable in %s", typ.Name(), f.Name, name, c.schema)
		}
		if c.request && sendsNull(f) && !prop.Nullable && !slices.Contains(c.nonZero, name) {
			t.Errorf("%s.%s: sent as null when nil, but property %q is not nullable in %s", typ.Name(), f.Name, name, c.schema)
		}
		err := checkType(spec, ft, prop)
		if _, bug := c.specBug[name]; bug {
			if err == nil {
				t.Errorf("%s.%s: property %q matches the schema; remove it from specBug", typ.Name(), f.Name, name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s.%s: property %q: %v", typ.Name(), f.Name, name, err)
		}
	}
	for name := range c.specBug {
		if !sent[name] {
			t.Errorf("%s: specBug lists %q, which is not a field", typ.Name(), name)
		}
	}
	for _, name := range c.nonZero {
		if !sent[name] {
			t.Errorf("%s: nonZero lists %q, which is not a field", typ.Name(), name)
		}
	}
	if c.request {
		for _, req := range schema.Required {
			if !sent[req] {
				t.Errorf("%s: %s requires property %q, which is not sent", typ.Name(), c.schema, req)
			}
		}
	}
}

// sendsNull reports whether a request field can be sent as null: a pointer,
// or a Date (whose zero value is null), that is not left out when nil or
// zero.
func sendsNull(f reflect.StructField) bool {
	_, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
	return (f.Type.Kind() == reflect.Pointer || f.Type == dateType) &&
		!strings.Contains(opts, "omitempty") && !strings.Contains(opts, "omitzero")
}

// compareVersions compares two dotted versions such as "3.5.103". Missing
// parts count as 0, and a suffix such as "-dev" is ignored, so that
// "3.6.1-dev" compares as "3.6.1".
func compareVersions(a, b string) int {
	a, _, _ = strings.Cut(a, "-")
	b, _, _ = strings.Cut(b, "-")
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if c := cmp.Compare(x, y); c != 0 {
			return c
		}
	}
	return 0
}

// unwrap returns the type a field is encoded as, without pointers and
// Nullable, and whether it is a Nullable.
func unwrap(ft reflect.Type) (reflect.Type, bool) {
	for ft.Kind() == reflect.Pointer {
		ft = ft.Elem()
	}
	if ft.Kind() == reflect.Struct && strings.HasPrefix(ft.Name(), nullableName+"[") {
		v, _ := ft.FieldByName("value")
		return v.Type, true
	}
	return ft, false
}

func checkType(spec *openAPISpec, ft reflect.Type, prop *openAPISchema) error {
	want := prop.Type
	if want == "" && len(prop.OneOf) > 0 {
		// Enums of a value or "" are oneOf the value's type and a blank enum.
		want = spec.resolve(prop.OneOf[0]).Type
	}
	if want == "" {
		return nil // untyped: any JSON value
	}
	var got string
	switch {
	case ft == timeType:
		if want != "string" || prop.Format != "date-time" {
			return fmt.Errorf("Go time.Time, schema type %q format %q", want, prop.Format)
		}
		return nil
	case ft == dateType:
		if want != "string" || prop.Format != "date" {
			return fmt.Errorf("Go Date, schema type %q format %q", want, prop.Format)
		}
		return nil
	case ft.Kind() == reflect.String:
		got = "string"
	case ft.Kind() >= reflect.Int && ft.Kind() <= reflect.Uint64:
		got = "integer"
	case ft.Kind() == reflect.Float32 || ft.Kind() == reflect.Float64:
		got = "number"
	case ft.Kind() == reflect.Bool:
		got = "boolean"
	case ft.Kind() == reflect.Slice:
		got = "array"
		if want == "array" && prop.Items != nil {
			elem, _ := unwrap(ft.Elem())
			if err := checkType(spec, elem, spec.resolve(prop.Items)); err != nil {
				return fmt.Errorf("items: %w", err)
			}
		}
	case ft.Kind() == reflect.Map || ft.Kind() == reflect.Struct:
		got = "object"
	default:
		return fmt.Errorf("unsupported Go type %s", ft)
	}
	if got != want {
		return fmt.Errorf("Go %s is %s, schema type is %s", ft, got, want)
	}
	return nil
}

# go-defectdojo

[![CI/CD](https://github.com/ottramst/go-defectdojo/actions/workflows/main.yml/badge.svg?branch=main)](https://github.com/ottramst/go-defectdojo/actions/workflows/main.yml)
[![CodeQL](https://github.com/ottramst/go-defectdojo/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/ottramst/go-defectdojo/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/ottramst/go-defectdojo/badge)](https://scorecard.dev/viewer/?uri=github.com/ottramst/go-defectdojo)
[![Go Reference](https://pkg.go.dev/badge/github.com/ottramst/go-defectdojo.svg)](https://pkg.go.dev/github.com/ottramst/go-defectdojo)
[![Go Report Card](https://goreportcard.com/badge/github.com/ottramst/go-defectdojo)](https://goreportcard.com/report/github.com/ottramst/go-defectdojo)

An idiomatic Go client for the [OWASP DefectDojo](https://github.com/DefectDojo/django-DefectDojo) REST API v2.

> [!NOTE]
> go-defectdojo is pre-1.0: the API may still change between minor versions. Breaking changes are
> listed in the [CHANGELOG](CHANGELOG.md). See the [roadmap](#roadmap) for what is planned.

## Features

- **Context-aware:** every method takes a `context.Context` for cancellation and deadlines.
- **Typed errors:** `*defectdojo.ErrorResponse` carries the status code, DefectDojo's message and its
  per-field validation errors, and works with `errors.Is(err, defectdojo.ErrNotFound)` and the other
  sentinels.
- **Round-trip friendly types:** read an object, change it, and write it back. Read-only fields are
  never sent.
- **Partial updates that can clear fields:** `Patch` methods send only what you set, and
  `defectdojo.Null[T]()` sends an explicit `null`.
- **Iterators:** `ListAll` methods return an `iter.Seq2` that follows DefectDojo's pagination.
- **Safe by default:** redirects are not followed, so a mistyped path never turns a write into a read
  of the login page.
- **Escape hatch:** `client.NewRequest` and `client.Do` reach any endpoint that has no method yet,
  with authentication and error handling included.
- **Tested against real DefectDojo:** acceptance tests run against every supported version in CI, and
  a contract test checks the client's types against each version's OpenAPI schema.
- **One small dependency:** [go-querystring](https://github.com/google/go-querystring).

### Supported APIs

| API | Status |
| --- | --- |
| Authentication | API token for a username and password ✅ |
| Product types | CRUD, partial update, list with an iterator, lookup by name, delete preview ✅ |
| Products | CRUD, partial update, list with an iterator, lookup by name, delete preview ✅ |
| Engagements | CRUD, partial update, list with an iterator, close and reopen, delete preview ✅ |
| SLA configurations | CRUD, partial update, list with an iterator, lookup by name, delete preview, detection of the recalculation lock ✅ |
| Server version | from the served OpenAPI schema ✅ |
| User profile | the authenticated user and its contact information ✅ |
| Users | CRUD, partial update, list with an iterator, lookup by username, delete preview, API token reset ✅ |
| Configuration permissions | list with an iterator, lookup by codename ✅ |

[COVERAGE.md](COVERAGE.md) lists every endpoint of the DefectDojo OpenAPI schemas and the method that
implements it.

## Installation

```shell
go get github.com/ottramst/go-defectdojo
```

## Usage

```go
import "github.com/ottramst/go-defectdojo"
```

Create a client with the DefectDojo URL and credentials, then use its services:

```go
client, err := defectdojo.NewClient("https://defectdojo.example.com",
	defectdojo.WithAPIToken(os.Getenv("DEFECTDOJO_API_TOKEN")))
if err != nil {
	log.Fatal(err)
}

productTypes, _, err := client.ProductTypes.List(ctx, nil)
```

Methods return the decoded result, the `*defectdojo.Response` (which wraps `*http.Response`) and an
error. Methods for endpoints without a response body, such as `Delete`, return only the
`*defectdojo.Response` and the error.

### Authentication

| Option | Header |
| --- | --- |
| `defectdojo.WithAPIToken(token)` | `Authorization: Token <token>`, the API v2 key of a user |
| `defectdojo.WithBasicAuth(user, password)` | `Authorization: Basic ...`; works also when API tokens are disabled on the server |

`client.Auth.Token(ctx, user, password)` returns the API token of a user. DefectDojo returns the same
token every time, and may rate limit the endpoint, so fetch it once.

Most configuration endpoints need a superuser, and `system_settings`, `notifications` and
`notification_webhooks` only accept superusers.

### Creating and updating objects

```go
pt, _, err := client.ProductTypes.Create(ctx, &defectdojo.ProductType{
	Name:            "Web Applications",
	Description:     "Customer-facing web applications",
	CriticalProduct: true,
})

// Update replaces every writable field: read it, change it, write it back.
pt, _, err = client.ProductTypes.Get(ctx, pt.ID)
pt.KeyProduct = true
pt, _, err = client.ProductTypes.Update(ctx, pt.ID, pt)

// Patch changes only the fields that are set, and can set them to null.
pt, _, err = client.ProductTypes.Patch(ctx, pt.ID, &defectdojo.ProductTypePatch{
	Description: defectdojo.Null[string](),
	KeyProduct:  new(false),
})
```

Deleting a product type also deletes its products, with their engagements, tests and findings.
`ProductTypes.CountProducts` and `ProductTypes.DeletePreview` show what a delete would remove.

### Errors

```go
_, _, err := client.ProductTypes.Get(ctx, 42)
if errors.Is(err, defectdojo.ErrNotFound) {
	// the product type does not exist
}

var errResp *defectdojo.ErrorResponse
if errors.As(err, &errResp) {
	log.Printf("DefectDojo answered %d: %s %v", errResp.StatusCode, errResp.Message, errResp.Fields)
}
```

DefectDojo answers a missing or invalid token with **403 Forbidden** (`ErrForbidden`), not 401, and a
name that is taken with **400 Bad Request** and a field error, not 409.

### Timeouts, retries and custom transports

The default HTTP client has no timeout: pass a context with a deadline, or bring your own client. The
client does not retry; plug in a retrying transport if you need one:

```go
client, err := defectdojo.NewClient(defectdojoURL,
	defectdojo.WithAPIToken(token),
	defectdojo.WithHTTPClient(&http.Client{Timeout: 30 * time.Second, Transport: myRetryingTransport}),
)
```

`WithHTTPClient` copies the client and turns off following redirects, unless it has its own
`CheckRedirect`.

### Endpoints without a method

```go
req, err := client.NewRequest(ctx, http.MethodGet, "/api/v2/user_profile/", nil)
if err != nil {
	return err
}
var profile map[string]any
if _, err := client.Do(req, &profile); err != nil {
	return err
}
```

The path is the escaped API path, including its trailing slash. Responses with a non-2xx status,
including redirects, return an `*ErrorResponse`. Deprecated endpoints set the `X-Deprecated` header,
which `Response.Deprecated()` returns.

More examples are in the [package documentation](https://pkg.go.dev/github.com/ottramst/go-defectdojo#pkg-examples).

## Compatibility

| | Supported |
| --- | --- |
| Go | The two newest releases (currently 1.26 and 1.27) |
| DefectDojo | The latest patch of the two newest minor releases, tested in CI: 3.5.103 and 3.4.304 |

### Support policy

DefectDojo ships a minor release every month and patches every week, and supports only its latest
release. The client supports, and tests in CI, the latest patch of the two newest minor releases;
[`.github/defectdojo-versions.json`](.github/defectdojo-versions.json) lists them. Older versions get
best-effort support. A nightly canary job runs the tests against DefectDojo's development build, and
a weekly job opens an issue when a new minor release is out.

DefectDojo 2.x is not supported: 3.0 removed the role-based access endpoints from the open-source
edition. DefectDojo Pro features are not tested.

## Roadmap

The path to v1.0 is tracked in [milestones](https://github.com/ottramst/go-defectdojo/milestones) and
on the [DefectDojo roadmap board](https://github.com/users/ottramst/projects/4), together with
[terraform-provider-defectdojo](https://github.com/ottramst/terraform-provider-defectdojo). Each epic
issue lists its work as sub-issues:

| Milestone | Scope |
| --- | --- |
| [v0.1.0](https://github.com/ottramst/go-defectdojo/milestone/1) | Foundation: client core, product types, products, engagements, user profile and authentication ✅ |
| [v0.2.0](https://github.com/ottramst/go-defectdojo/milestone/2) | SLA configurations, users, notifications and notification webhooks |
| [v0.3.0](https://github.com/ottramst/go-defectdojo/milestone/3) | System settings, Jira instances and Jira projects |
| [v0.4.0](https://github.com/ottramst/go-defectdojo/milestone/4) | Lookup objects: regulations, note types, development environments, announcements, CI/CD infrastructure, product attributes, test types |
| [v1.0.0](https://github.com/ottramst/go-defectdojo/milestone/5) | API freeze and compatibility guarantees |

Findings, tests and imports, and DefectDojo's experimental API v3, are
[backlog epics](https://github.com/ottramst/go-defectdojo/issues?q=is%3Aissue+is%3Aopen+label%3Aepic+no%3Amilestone).
Contributions are welcome.

## Development

```shell
make test        # unit tests
make lint        # golangci-lint
make up          # DefectDojo on localhost:8080 (admin / DefectDojoAcc1!), about 30 seconds
make testacc     # unit + acceptance tests against it
make contract    # check the client's types against the supported versions' schemas
make coverage    # regenerate COVERAGE.md
make down        # stop the stack and delete its data
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

[MIT](LICENSE)

# Contributing

Thanks for your interest in improving go-defectdojo. Bug reports, feature requests and pull requests
are all welcome. By participating you agree to follow the [Code of Conduct](.github/CODE_OF_CONDUCT.md).

## Reporting issues

Use the [issue templates](https://github.com/ottramst/go-defectdojo/issues/new/choose). Always include
the go-defectdojo, Go and DefectDojo versions. Report security vulnerabilities privately, as described
in [SECURITY.md](SECURITY.md).

Planned work is tracked in [milestones](https://github.com/ottramst/go-defectdojo/milestones): each
epic issue lists its work as sub-issues. Comment on a sub-issue before starting on it, so work isn't
duplicated.

## Development setup

Requirements: [Go](https://go.dev/doc/install) (1.26 or later),
[golangci-lint](https://golangci-lint.run/) (the version pinned in `.github/workflows/lint.yml`),
Docker and `jq`.

```shell
make build   # build the package
make test    # unit tests
make lint    # golangci-lint
make fmt     # gofmt -s
```

## Acceptance tests

Acceptance tests (`TestAcc*`) run against a real DefectDojo. They are skipped unless `DEFECTDOJO_ACC`
is set; once it is set, a server that cannot be reached fails them. A Docker Compose file starts
PostgreSQL, Valkey and DefectDojo in about 30 seconds; `DD_VERSION` selects the image tag and
defaults to the newest supported version.

```shell
make up
# make down && make up DD_VERSION=3.4.304

make testacc
# or a single test:
DEFECTDOJO_ACC=1 go test -v -run TestAccProductTypes ./...

make down
```

The tests log in as the superuser `admin` / `DefectDojoAcc1!`. To run them against another DefectDojo,
set `DEFECTDOJO_URL` and either `DEFECTDOJO_API_TOKEN` or `DEFECTDOJO_USERNAME` and
`DEFECTDOJO_PASSWORD`. Each test creates its own uniquely named objects and deletes them afterwards.

Behavior that differs between DefectDojo versions must be covered for each supported version: run the
suite against every version in `.github/defectdojo-versions.json` before opening a pull request.

## Contract test

`make contract` downloads the OpenAPI schema of every supported version into `.cache/specs/` and checks
the client's request and response types against them: every field must exist, have the right type and
nullability, and be writable in request types, and request types must send every required field. Add
each new type to the `contracts` list in `contract_test.go`. Fields that only newer versions have are
listed in its `since` map.

## Supported DefectDojo versions

CI tests the latest patch of the two newest DefectDojo minor releases (see the
[support policy](README.md#support-policy)). They are listed in one place,
`.github/defectdojo-versions.json`, oldest first; the test matrix, `make up`, `make contract` and
`make coverage` read them from there. The `Upstream` workflow opens an issue when a new minor release
is out. To support it:

1. Read the DefectDojo release notes for API changes.
2. Update `.github/defectdojo-versions.json`: drop the oldest minor and add the new release. Set the
   `DD_VERSION` default in `docker-compose.yml` to the new release too; CI checks that they match.
3. Run `make contract`, `make coverage`, and the acceptance tests against both versions.
4. Update the tested versions in `README.md` and `CLAUDE.md`, and add a `NOTES` entry to the CHANGELOG.

## Pull requests

- Keep each pull request focused on one change, and add or update tests for it.
- Use a [Conventional Commits](https://www.conventionalcommits.org/) title, e.g.
  `feat(products): add the Products service`; pull requests are squash-merged with that title. Mark
  breaking changes with `!`.
- Add an entry to `CHANGELOG.md` under `## Unreleased` (`BREAKING CHANGES`, `FEATURES`,
  `ENHANCEMENTS`, `BUG FIXES` or `NOTES`).
- CI must pass: the `CI gate` check covers lint, build and unit tests on every supported Go version,
  the API compatibility check, the contract test, `COVERAGE.md`, and the acceptance test matrix.

## Compatibility

From v1.0.0, the module follows [semantic versioning](https://semver.org/). A change that can break code
using the module, such as removing or renaming an exported identifier, changing a signature or a field type,
or adding a method to an interface, needs a new major version with a `/v2` module path. Adding methods,
functions, types, constants, options and struct fields is compatible: exported structs have an unexported
`_ [0]func()` field, so they can't be compared or written as unkeyed literals, which adding a field would
break. The v1 conventions for new API are in [CLAUDE.md](CLAUDE.md) ("v1 API conventions").

The `API compatibility` job of CI runs [gorelease](https://pkg.go.dev/golang.org/x/exp/cmd/gorelease) against
the latest v1 release and fails on incompatible changes. The only exceptions are the ones in the
[stability policy](README.md#stability), when DefectDojo changes its API and keeping the old shape isn't
possible. Such a pull request adds the file `.github/api-break-allowed`, containing the latest release (for
example `v1.2.0`), and its CHANGELOG entry goes under `BREAKING CHANGES`. The check then warns instead of
failing, on the pull request, on `main` and on later pull requests, until the next release makes the file
stale; the release pull request deletes it.

To run the check locally on a clean checkout:

```bash
(cd tools/gorelease && go build -o /tmp/gorelease golang.org/x/exp/cmd/gorelease)
/tmp/gorelease -base=v1.0.0   # the latest v1 release
```

## Releases

Maintainers release by pushing a `vX.Y.Z` tag:

1. Pick the version: a patch release for bug fixes, a minor release for new features or supported
   DefectDojo versions. `gorelease` suggests one.
2. In a release pull request, move the `## Unreleased` entries in `CHANGELOG.md` under a new `## vX.Y.Z`
   heading, and delete `.github/api-break-allowed` if it exists.
3. After it is merged, tag the merge commit and push the tag. The release workflow publishes the
   `## vX.Y.Z` section as the GitHub release notes and warms the Go module proxy, so that pkg.go.dev picks
   up the version.

Never push a tag only to test the workflow: versions cannot be removed from the Go module proxy. Tag the client
before every release of
[terraform-provider-defectdojo](https://github.com/ottramst/terraform-provider-defectdojo) that needs
its changes.

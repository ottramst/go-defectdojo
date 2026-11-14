## Related issue

Fixes #<!-- issue number -->

## Description

<!-- What does this change, and why? Call out any design decisions, DefectDojo quirks or trade-offs. -->

## Checklist

- [ ] Unit tests added or updated, and `make test` passes
- [ ] Acceptance tests added or updated (`TestAcc...`), and `make testacc` passes against a local stack (`make up`)
- [ ] Tested against every supported DefectDojo version (`make up DD_VERSION=...`, see `.github/defectdojo-versions.json`) where behavior differs
- [ ] New types are in the `contracts` list of `contract_test.go`, and `make contract` passes
- [ ] New service methods carry a `// DefectDojo API: METHOD /path` annotation, and `make coverage` was run
- [ ] `make lint` passes
- [ ] `CHANGELOG.md` updated under `## Unreleased`

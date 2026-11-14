default: fmt lint test

build:
	go build -v ./...

lint:
	golangci-lint run

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

# The supported DefectDojo versions, oldest first. .github/defectdojo-versions.json
# is the only place they are listed.
DEFECTDOJO_VERSIONS := $(shell jq -r '.versions | join(" ")' .github/defectdojo-versions.json)
DD_VERSION ?= $(lastword $(DEFECTDOJO_VERSIONS))
SPEC_DIR = .cache/specs

# Starts the acceptance test stack: DefectDojo $(DD_VERSION) on localhost:8080.
up:
	DD_VERSION=$(DD_VERSION) docker compose up -d --wait

# Stops it and deletes its data; needed before switching versions.
down:
	DD_VERSION=$(DD_VERSION) docker compose down -v

# Needs a running DefectDojo: make up
testacc:
	DEFECTDOJO_ACC=1 go test -v -cover -timeout 15m ./...

# Downloads the OpenAPI schemas of the supported versions once into .cache/.
specs:
	@mkdir -p $(SPEC_DIR)
	@for v in $(DEFECTDOJO_VERSIONS); do \
		f=$(SPEC_DIR)/defectdojo-$$v.json; \
		[ -s $$f ] || { curl -fsSL --retry 3 -o $$f.tmp https://github.com/DefectDojo/django-DefectDojo/releases/download/$$v/oas.json && mv $$f.tmp $$f; } || exit 1; \
	done

# Checks the client's types against the schemas of every supported version.
contract: specs
	DEFECTDOJO_SPEC_DIR=$(SPEC_DIR) go test -v -count=1 -run TestContract .

# Regenerates COVERAGE.md from the schemas.
coverage: specs
	cd tools/coverage && go run . -root ../.. -versions "$(DEFECTDOJO_VERSIONS)"

.PHONY: default build lint fmt test up down testacc specs contract coverage

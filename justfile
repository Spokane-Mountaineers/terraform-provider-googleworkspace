# Command runner for terraform-provider-google-workspace.
# Run `just` to list recipes.

set shell := ["bash", "-uc"]

binary := "terraform-provider-googleworkspace"
mirror_host := "registry.opentofu.org"
mirror_ns := "spokane-mountaineers"
mirror_type := "googleworkspace"
version := "0.0.0-dev"
# Resource type prefix (hyphen-free); also the tfplugindocs provider name.
provider_name := "googleworkspace"
# Terraform binary version tfplugindocs downloads for schema export (doc-build only).
tf_version := "1.9.8"

# List available recipes.
default:
    @just --list

# Format code (go fmt + gofmt -s).
[group: 'dev']
fmt:
    go fmt ./...
    gofmt -s -w .

# Verify formatting; fail if anything is unformatted.
[group: 'quality']
fmt-check:
    @test -z "$(gofmt -s -l .)" || { echo "unformatted files:"; gofmt -s -l .; exit 1; }

# Apply automated fixes for deprecated API usage.
[group: 'dev']
fix:
    go fix ./...

# Tidy module dependencies.
[group: 'dev']
tidy:
    go mod tidy

# Run go vet.
[group: 'quality']
vet:
    go vet ./...

# Run the linter.
[group: 'quality']
lint:
    golangci-lint run

# Run the linter and apply autofixes.
[group: 'quality']
lint-fix:
    golangci-lint run --fix

# Run unit tests.
[group: 'testing']
test:
    go test ./...

# Run tests with coverage.
[group: 'testing']
test-cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out | tail -1

# Run acceptance tests (hits the live Workspace API; requires credentials).
[group: 'testing']
testacc:
    TF_ACC=1 go test ./... -v -timeout 30m

# Build the provider binary.
[group: 'build']
build:
    go build -ldflags "-X main.version={{ version }}" -o {{ binary }} .

# Build and install into the local ~/.terraform.d/plugins filesystem mirror.
[group: 'build']
install: build
    #!/usr/bin/env bash
    set -euo pipefail
    os_arch="$(go env GOOS)_$(go env GOARCH)"
    dest="${HOME}/.terraform.d/plugins/{{ mirror_host }}/{{ mirror_ns }}/{{ mirror_type }}/{{ version }}/${os_arch}"
    mkdir -p "${dest}"
    cp {{ binary }} "${dest}/{{ binary }}_v{{ version }}"
    echo "installed to ${dest}"

# Generate registry documentation (docs/) from schema, templates, and examples.
[group: 'docs']
docs:
    go tool tfplugindocs generate \
        --provider-name {{ provider_name }} \
        --rendered-provider-name "Google Workspace" \
        --tf-version {{ tf_version }}

# Verify generated docs are current; fails if `just docs` would change anything.
[group: 'docs']
docs-check: docs
    @git diff --exit-code -- docs/ || { echo "docs out of date: run 'just docs' and commit"; exit 1; }

# Full local check: format, vet, lint, test.
[group: 'quality']
ci: fmt-check vet lint test

# Remove build artifacts.
[group: 'build']
clean:
    rm -f {{ binary }} coverage.out

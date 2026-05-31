# Command runner for terraform-provider-google-workspace.
# Run `just` to list recipes.

set shell := ["bash", "-uc"]

binary := "terraform-provider-google-workspace"
mirror_host := "registry.spokanemountaineers.org"
mirror_ns := "spokane-mountaineers"
mirror_type := "google-workspace"
version := "0.0.0-dev"

# List available recipes.
default:
    @just --list

# Format code (go fmt + gofmt -s).
fmt:
    go fmt ./...
    gofmt -s -w .

# Verify formatting; fail if anything is unformatted.
fmt-check:
    @test -z "$(gofmt -s -l .)" || { echo "unformatted files:"; gofmt -s -l .; exit 1; }

# Apply automated fixes for deprecated API usage.
fix:
    go fix ./...

# Tidy module dependencies.
tidy:
    go mod tidy

# Run go vet.
vet:
    go vet ./...

# Run the linter.
lint:
    golangci-lint run

# Run the linter and apply autofixes.
lint-fix:
    golangci-lint run --fix

# Run unit tests.
test:
    go test ./...

# Run tests with coverage.
test-cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out | tail -1

# Run acceptance tests (hits the live Workspace API; requires credentials).
testacc:
    TF_ACC=1 go test ./... -v -timeout 30m

# Build the provider binary.
build:
    go build -ldflags "-X main.version={{ version }}" -o {{ binary }} .

# Build and install into the local ~/.terraform.d/plugins filesystem mirror.
install: build
    #!/usr/bin/env bash
    set -euo pipefail
    os_arch="$(go env GOOS)_$(go env GOARCH)"
    dest="${HOME}/.terraform.d/plugins/{{ mirror_host }}/{{ mirror_ns }}/{{ mirror_type }}/{{ version }}/${os_arch}"
    mkdir -p "${dest}"
    cp {{ binary }} "${dest}/{{ binary }}_v{{ version }}"
    echo "installed to ${dest}"

# Full local check: format, vet, lint, test.
ci: fmt-check vet lint test

# Remove build artifacts.
clean:
    rm -f {{ binary }} coverage.out

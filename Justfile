# hermes-cli task runner. Run `just` to list recipes.

version := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
commit  := `git rev-parse --short HEAD 2>/dev/null || echo "none"`
date    := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
ldflags := "-X main.Version=" + version + " -X main.Commit=" + commit + " -X main.BuildDate=" + date

# List available recipes
default:
    @just --list

# Build static binary to bin/hermes
build:
    CGO_ENABLED=0 go build -ldflags "{{ldflags}}" -o bin/hermes ./cmd/hermes

# Install hermes into GOBIN
install:
    go install -ldflags "{{ldflags}}" ./cmd/hermes

# Remove build artifacts
clean:
    rm -rf bin/
    go clean

# Run the test suite
test:
    go test -v ./...

# Vet, plus golangci-lint when available
lint:
    go vet ./...
    @command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed"

# Format all Go code
fmt:
    go fmt ./...

# Tidy go.mod / go.sum
tidy:
    go mod tidy

# Build then run, e.g. `just run "doctor --json"`
run *ARGS: build
    ./bin/hermes {{ARGS}}

# Full quality gate: lint + test + build
check: lint test build
    @echo "All checks passed"

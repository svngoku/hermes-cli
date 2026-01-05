.PHONY: build clean test lint run install

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(DATE)"

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/hermes ./cmd/hermes

install:
	go install $(LDFLAGS) ./cmd/hermes

clean:
	rm -rf bin/
	go clean

test:
	go test -v ./...

lint:
	go vet ./...
	@which golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed"

run: build
	./bin/hermes $(ARGS)

fmt:
	go fmt ./...

tidy:
	go mod tidy

check: lint test build
	@echo "All checks passed"

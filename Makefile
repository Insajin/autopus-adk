BINARY := auto
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X github.com/anthropics/autopus-adk/pkg/version.version=$(VERSION) -X github.com/anthropics/autopus-adk/pkg/version.commit=$(COMMIT) -X github.com/anthropics/autopus-adk/pkg/version.date=$(DATE)"

.PHONY: build test test-unit test-integration test-e2e test-all update-golden lint clean install

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/auto

test:
	go test -race -count=1 -tags integration ./...

test-unit:
	go test -race -count=1 ./...

test-integration:
	go test -race -count=1 -tags integration ./...

test-e2e: build
	AUTOPUS_TEST_BINARY=./bin/auto go test -race -count=1 -tags e2e ./e2e/...

test-all:
	go test -race -count=1 -tags 'integration e2e' ./...

update-golden:
	go test -race -count=1 -tags e2e -update ./e2e/...

lint:
	go vet ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	rm -rf bin/ coverage.out

install: build
	cp bin/$(BINARY) $(GOPATH)/bin/$(BINARY)

BINARY := auto
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X github.com/insajin/autopus-adk/pkg/version.version=$(VERSION) -X github.com/insajin/autopus-adk/pkg/version.commit=$(COMMIT) -X github.com/insajin/autopus-adk/pkg/version.date=$(DATE)"

.PHONY: build test test-unit test-integration test-e2e test-all update-golden lint clean install generate-templates

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/auto

# Go의 기본 테스트 타임아웃은 10분이다. 이 저장소에서 internal/cli와
# internal/companionmanifest는 단독으로도 각각 7분을 넘기므로, 타임아웃 없는
# `go test ./...`는 병렬 경합이 얹히는 순간 항상 실패한다. CI는 레인마다
# -timeout을 명시해서 (ci.yaml의 5m/8m/15m/20m/30m) 통과해 왔고, 그래서
# 로컬 진입점만 깨진 상태로 오래 남아 있었다. 아래 값은 그 CI 레인에서
# 가져온 것이며, 임의로 줄이면 다시 같은 증상이 난다.
UNIT_TIMEOUT        ?= 20m
INTEGRATION_TIMEOUT ?= 30m

test:
	go test -race -count=1 -timeout=$(INTEGRATION_TIMEOUT) -tags integration ./...

test-unit:
	go test -race -count=1 -timeout=$(UNIT_TIMEOUT) ./...

test-integration:
	go test -race -count=1 -timeout=$(INTEGRATION_TIMEOUT) -tags integration ./...

test-e2e: build
	AUTOPUS_TEST_BINARY=./bin/auto go test -race -count=1 -timeout=$(INTEGRATION_TIMEOUT) -tags e2e ./e2e/...

test-all:
	go test -race -count=1 -timeout=$(INTEGRATION_TIMEOUT) -tags 'integration e2e' ./...

update-golden:
	go test -race -count=1 -tags e2e ./e2e/... -update

lint:
	go vet ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

generate-templates:
	go run ./cmd/generate-templates

clean:
	rm -rf bin/ coverage.out

install: build
	cp bin/$(BINARY) $(GOPATH)/bin/$(BINARY)
	@if [ "$$(uname -s)" = "Darwin" ]; then \
		xattr -cr $(GOPATH)/bin/$(BINARY) 2>/dev/null || true; \
		codesign --force --sign - $(GOPATH)/bin/$(BINARY) >/dev/null 2>&1 || true; \
	fi

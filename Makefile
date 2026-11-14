include tools/versions.mk

.DEFAULT_GOAL := help
SHELL := /bin/sh
export GOTOOLCHAIN := local
export GOFLAGS := -mod=readonly
export GOSUMDB := sum.golang.org

PKG ?= ./...
TESTFLAGS ?=
PACKET ?= P00
FUZZ_PKG ?=
FUZZ_TARGET ?=
FUZZTIME ?= 30s

.PHONY: help tools toolchain fmt fmt-check vet test lint mod-check api-check docs-check generate check race security fuzz build cross

help: ## List recipes, prerequisites and side effects; no writes.
	@printf '%s\n' 'Prerequisites: Go $(GO_VERSION), GNU make, POSIX shell; make tools additionally needs curl, tar, awk and SHA-256 tooling.' 'Run make tools explicitly before lint/check/security; checks never install tools.' ''
	@awk 'BEGIN {FS = ":.*## "} /^[a-z][a-z-]*:.*## / {printf "  %-14s %s\n", $$1, $$2}' Makefile
	@printf '\n%s\n' 'Options: PKG=./... TESTFLAGS=... PACKET=P00; fuzz requires FUZZ_PKG=./path FUZZ_TARGET=FuzzName [FUZZTIME=30s].' 'snapshot: deferred to P10; no target, publishing or hooks.'

tools: ## Install pinned verified tools in .bin/; needs network and setup prerequisites.
	sh scripts/install-tools.sh

toolchain:
	@test "$$(go env GOVERSION)" = 'go$(GO_VERSION)' || { printf '%s\n' 'Go $(GO_VERSION) must be on PATH (GOTOOLCHAIN=local).' >&2; exit 1; }

fmt: toolchain ## Format Go source with go fmt ./...; writes Go files.
	go fmt ./...

fmt-check: toolchain ## Check gofmt on cmd/ and internal/; no source writes.
	@set -eu; report=$$(mktemp); trap 'rm -f "$$report"' 0; trap 'exit 1' HUP INT TERM; \
	find cmd internal -type f -name '*.go' -exec gofmt -l {} + > "$$report"; \
	if [ -s "$$report" ]; then cat "$$report"; exit 1; fi

vet: toolchain ## Run go vet ./...; Go cache only (already included in lint).
	go vet ./...

test: toolchain ## Run go test $(PKG) $(TESTFLAGS); test temp/cache only.
	go test $(PKG) $(TESTFLAGS)

lint: toolchain ## Verify v2 configuration, then lint ./...; needs make tools; cache only.
	.bin/golangci-lint config verify
	.bin/golangci-lint run ./...

mod-check: toolchain ## Check go mod tidy -diff; no module writes; may use module cache/network.
	go mod tidy -diff

api-check: toolchain ## Reconcile offline catalog/schema/coverage for PACKET (default P00); no writes.
	go run ./internal/cmd/checkapi --packet $(PACKET)

docs-check: toolchain ## Check local documentation links and catalog consistency offline; no writes.
	go run ./internal/cmd/checkdocs

generate: toolchain ## Regenerate committed embedded metadata from the catalog offline; writes generated JSON.
	go run ./internal/cmd/genspec

check: fmt-check lint test mod-check api-check docs-check ## Run the reviewed packet gate; no source writes; needs make tools.

race: toolchain ## Run race-enabled tests on ./...; needs C compiler and supported host; test temp/cache only.
	CGO_ENABLED=1 go test -race ./...

security: toolchain ## Scan reachable vulnerabilities; needs make tools and advisory network; cache only.
	@status=0; .bin/govulncheck ./... || status=$$?; \
	case $$status in \
	  0) ;; \
	  3) printf '%s\n' 'Security gate failed: reachable vulnerabilities reported.' >&2 ;; \
	  *) printf '%s\n' "Security gate incomplete: scanner/infrastructure failure (exit $$status); not a clean scan." >&2 ;; \
	esac; exit $$status

fuzz: toolchain ## Fuzz one explicit package/target for FUZZTIME seconds; fuzz cache only.
	@set -eu; \
	case '$(FUZZ_PKG)' in ./?*) ;; *) printf '%s\n' 'Set FUZZ_PKG to one ./package.' >&2; exit 2 ;; esac; \
	case '$(FUZZ_PKG)' in *...*|*[!A-Za-z0-9_./-]*) printf '%s\n' 'FUZZ_PKG must name one package, not a pattern/list.' >&2; exit 2 ;; esac; \
	printf '%s\n' '$(FUZZ_TARGET)' | grep -Eq '^Fuzz[A-Za-z0-9_]+$$' || { printf '%s\n' 'Set FUZZ_TARGET to one FuzzName.' >&2; exit 2; }; \
	printf '%s\n' '$(FUZZTIME)' | grep -Eq '^[1-9][0-9]*s$$' || { printf '%s\n' 'FUZZTIME must be a positive bounded number of seconds (for example 30s).' >&2; exit 2; }; \
	targets=$$(go test '$(FUZZ_PKG)' -list '^$(FUZZ_TARGET)$$'); \
	printf '%s\n' "$$targets" | grep -Fxq '$(FUZZ_TARGET)' || { printf '%s\n' 'Named fuzz target was not found in FUZZ_PKG.' >&2; exit 2; }; \
	go test '$(FUZZ_PKG)' -run '^$$' -fuzz '^$(FUZZ_TARGET)$$' -fuzztime='$(FUZZTIME)'

build: toolchain ## Build ./cmd/plane-cli for the host into bin/plane-cli; writes build output.
	mkdir -p bin
	go build -trimpath -o bin/plane-cli ./cmd/plane-cli

cross: toolchain ## Build CGO-free Linux/macOS amd64/arm64 binaries into bin/; writes build output.
	@set -eu; mkdir -p bin; \
	for os in linux darwin; do \
	  for arch in amd64 arm64; do \
	    printf 'Building %s/%s\n' "$$os" "$$arch"; \
	    CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -o "bin/plane-cli-$$os-$$arch" ./cmd/plane-cli; \
	  done; \
	done

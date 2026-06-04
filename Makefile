# APRSGo build orchestration.
#
# The web UI (Nuxt SSG) is embedded into the Go binary via `//go:embed
# all:web/dist`, so the frontend MUST be generated before building/testing the
# Go module. These targets enforce that order.

SHELL := /bin/bash
WEB_DIR := web
WEB_DIST := $(WEB_DIR)/dist
BIN := aprsgo
DIST_DIR := release

# Version stamp embedded into release archive names.
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)

# Target platforms for cross-compilation (GOOS/GOARCH pairs).
PLATFORMS := \
	darwin/amd64 \
	darwin/arm64 \
	freebsd/386 \
	freebsd/amd64 \
	freebsd/arm \
	freebsd/arm64 \
	linux/386 \
	linux/amd64 \
	linux/loong64 \
	linux/mips \
	linux/mips64 \
	linux/mips64le \
	linux/mipsle \
	linux/riscv64 \
	linux/arm64 \
	linux/arm \
	openbsd/386 \
	openbsd/amd64 \
	openbsd/arm \
	openbsd/arm64 \
	openbsd/riscv64 \
	solaris/amd64 \
	windows/386 \
	windows/amd64 \
	windows/arm64

.PHONY: all web web-install web-clean build build-fast run test test-race vet fmt tidy clean release release-clean

all: build

## web-install: install frontend dependencies (pnpm)
web-install:
	cd $(WEB_DIR) && pnpm install

## web-clean: remove the previously generated UI bundle and Nuxt caches
web-clean:
	rm -rf $(WEB_DIST) $(WEB_DIR)/.nuxt $(WEB_DIR)/.output

## web: clean and (re)generate the static UI bundle into web/dist.
## Always produces a fresh bundle so embedded assets never go stale.
web: web-install web-clean
	cd $(WEB_DIR) && pnpm generate

## build: regenerate the UI from scratch, then build the Go binary.
## This guarantees the embedded web bundle matches the current sources.
build: web
	go build -o $(BIN) .

## build-fast: build without regenerating the UI (uses existing web/dist).
## Use during Go-only iteration; run `make web` first at least once.
build-fast: $(WEB_DIST)
	go build -o $(BIN) .

# Ensure a bundle exists before any Go build/test/vet (does NOT force a rebuild).
$(WEB_DIST):
	cd $(WEB_DIR) && pnpm install && pnpm generate

## run: build and run
run: build
	./$(BIN)

## test: run all Go tests (requires web/dist to exist for the main package)
test: $(WEB_DIST)
	go test ./...

## test-race: run all Go tests with the race detector
test-race: $(WEB_DIST)
	go test -race ./...

## vet: run go vet
vet: $(WEB_DIST)
	go vet ./...

## fmt: gofmt all Go sources
fmt:
	gofmt -w .

## tidy: tidy go modules
tidy:
	go mod tidy

## clean: remove build artifacts and generated UI
clean: web-clean
	rm -f $(BIN)

## release-clean: remove release artifacts
release-clean:
	rm -rf $(DIST_DIR)

## release: cross-compile the web-embedded binary for all target platforms.
## Regenerates the UI from scratch first; each platform binary embeds the same
## fresh bundle. Output: release/aprsgo-<version>-<os>-<arch>[.exe] archives.
release: web release-clean
	@mkdir -p $(DIST_DIR)
	@set -e; \
	for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		name="$(BIN)-$(VERSION)-$$os-$$arch"; \
		out="$(DIST_DIR)/$$name$$ext"; \
		echo "==> building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags "-s -w" -o "$$out" .; \
	done; \
	echo "==> release artifacts in $(DIST_DIR)/"; \
	ls -lh $(DIST_DIR)


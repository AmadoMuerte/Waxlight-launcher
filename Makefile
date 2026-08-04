SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

GO ?= go
GOFMT ?= gofmt
NPM ?= npm
GIT ?= git

VERSION ?=
RELEASE_DIR ?= release
BRANCH ?= main
REMOTE ?= origin

WAILS_VERSION ?= v2.11.0
GOVULNCHECK_VERSION ?= v1.6.0

WAILS_TAGS := desktop,production

ifeq ($(shell $(GO) env GOOS),linux)
WAILS_TAGS := $(WAILS_TAGS),webkit2_41
endif

CURRENT_VERSION := $(shell node -p "require('./wails.json').info.productVersion" 2>/dev/null || echo unknown)
RELEASE_TAG = v$(VERSION)

.DEFAULT_GOAL := help

.PHONY: \
	help \
	install \
	frontend-install \
	frontend \
	build \
	wails-build \
	build-windows \
	test \
	test-backend \
	test-frontend \
	format \
	format-check \
	lint \
	race \
	vet \
	security \
	security-patterns \
	vulncheck \
	package-linux \
	release-check \
	check-version-argument \
	check-tools \
	check-clean \
	check-branch \
	check-tag \
	check-synced \
	set-version \
	release \
	clean

help:
	@echo "Waxlight Launcher"
	@echo
	@echo "Current version: $(CURRENT_VERSION)"
	@echo
	@echo "Available commands:"
	@echo "  make install"
	@echo "      Install frontend dependencies."
	@echo
	@echo "  make build"
	@echo "      Build the local Go application."
	@echo
	@echo "  make wails-build"
	@echo "      Build the local Wails application."
	@echo
	@echo "  make test"
	@echo "      Run backend and frontend tests."
	@echo
	@echo "  make vet"
	@echo "      Run go vet."
	@echo
	@echo "  make format"
	@echo "      Format Go and frontend source files."
	@echo
	@echo "  make format-check"
	@echo "      Check Go and frontend source formatting."
	@echo
	@echo "  make lint"
	@echo "      Run Go and frontend static analysis."
	@echo
	@echo "  make security"
	@echo "      Run security-pattern and vulnerability checks."
	@echo
	@echo "  make release-check VERSION=X.Y.Z"
	@echo "      Run all checks for a release."
	@echo
	@echo "  make release VERSION=X.Y.Z"
	@echo "      Update versions, commit, push main, create a tag and start"
	@echo "      the automatic GitHub release workflow."
	@echo
	@echo "Example:"
	@echo "  make release VERSION=0.1.3"

check-version-argument:
	@if [[ -z "$(VERSION)" ]]; then \
		echo "error: VERSION is required"; \
		echo "usage: make release VERSION=0.1.3"; \
		exit 1; \
	fi
	@if [[ ! "$(VERSION)" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$$ ]]; then \
		echo "error: invalid semantic version: $(VERSION)"; \
		exit 1; \
	fi

check-tools:
	@for command in "$(GO)" "$(NPM)" "$(GIT)" node; do \
		if ! command -v "$$command" >/dev/null 2>&1; then \
			echo "error: required command is unavailable: $$command"; \
			exit 1; \
		fi; \
	done

check-clean:
	@if [[ -n "$$($(GIT) status --porcelain)" ]]; then \
		echo "error: working tree is not clean"; \
		$(GIT) status --short; \
		echo; \
		echo "Commit or stash these changes before creating a release."; \
		exit 1; \
	fi

check-branch:
	@current_branch="$$($(GIT) branch --show-current)"; \
	if [[ "$$current_branch" != "$(BRANCH)" ]]; then \
		echo "error: releases must be created from $(BRANCH)"; \
		echo "current branch: $$current_branch"; \
		exit 1; \
	fi

check-tag: check-version-argument
	@if $(GIT) rev-parse "$(RELEASE_TAG)" >/dev/null 2>&1; then \
		echo "error: local tag $(RELEASE_TAG) already exists"; \
		exit 1; \
	fi
	@if $(GIT) ls-remote \
		--exit-code \
		--tags \
		"$(REMOTE)" \
		"refs/tags/$(RELEASE_TAG)" >/dev/null 2>&1; then \
		echo "error: remote tag $(RELEASE_TAG) already exists"; \
		exit 1; \
	fi

check-synced:
	@$(GIT) fetch "$(REMOTE)" "$(BRANCH)"
	@local_commit="$$($(GIT) rev-parse HEAD)"; \
	remote_commit="$$($(GIT) rev-parse "$(REMOTE)/$(BRANCH)")"; \
	if [[ "$$local_commit" != "$$remote_commit" ]]; then \
		echo "error: local $(BRANCH) is not synchronized with $(REMOTE)/$(BRANCH)"; \
		echo; \
		echo "Local:  $$local_commit"; \
		echo "Remote: $$remote_commit"; \
		echo; \
		echo "Push or pull your changes before releasing."; \
		exit 1; \
	fi

frontend-install:
	$(NPM) ci --include=dev --prefix frontend

install: frontend-install

frontend:
	$(NPM) --prefix frontend run build

build: frontend
	mkdir -p build
	$(GO) build \
		-buildvcs=false \
		-tags "$(WAILS_TAGS)" \
		-trimpath \
		-ldflags "-w -s" \
		-o build/waxlight \
		./cmd/waxlight

wails-build:
	cd cmd/waxlight && \
	wails build \
		-clean \
		-trimpath \
		-ldflags="-s -w"

build-windows:
	@if ! command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then \
		echo "error: x86_64-w64-mingw32-gcc is required"; \
		echo "Install it on Arch Linux with:"; \
		echo "  sudo pacman -S mingw-w64-gcc"; \
		exit 1; \
	fi
	cd cmd/waxlight && \
	CGO_ENABLED=1 \
	CC=x86_64-w64-mingw32-gcc \
	CXX=x86_64-w64-mingw32-g++ \
	wails build \
		-clean \
		-platform windows/amd64 \
		-trimpath \
		-ldflags="-s -w"

test-backend:
	$(GO) test ./...

test-frontend:
	$(NPM) --prefix frontend test

test: frontend test-backend test-frontend

format:
	$(GOFMT) -w $$($(GIT) ls-files '*.go')
	$(NPM) --prefix frontend run format

format-check:
	@unformatted="$$($(GOFMT) -l $$($(GIT) ls-files '*.go'))"; \
	if [[ -n "$$unformatted" ]]; then \
		echo "error: Go files need formatting:"; \
		printf '%s\n' "$$unformatted"; \
		exit 1; \
	fi
	$(NPM) --prefix frontend run format:check

# CI validates Linux build-tagged source; use same target for cross-platform hooks.
lint:
	GOOS=linux $(GO) vet ./...
	$(NPM) --prefix frontend run lint

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

security-patterns:
	./scripts/check-security-patterns.sh

vulncheck:
	GOTOOLCHAIN=auto \
	$(GO) run \
		golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) \
		./...

security: security-patterns vulncheck

package-linux: check-version-argument
	./scripts/build-linux.sh "$(VERSION)" "$(RELEASE_DIR)"

release-check: check-version-argument frontend-install
	./scripts/check-version.sh "$(VERSION)"
	$(MAKE) test
	$(MAKE) race
	$(MAKE) vet
	$(MAKE) security

set-version: check-version-argument
	@VERSION="$(VERSION)" node -e 'const fs = require("fs"); const version = process.env.VERSION; const files = ["wails.json", "cmd/waxlight/wails.json"]; for (const file of files) { const value = JSON.parse(fs.readFileSync(file, "utf8")); value.info = value.info || {}; value.info.productVersion = version; fs.writeFileSync(file, JSON.stringify(value, null, 2) + "\n"); console.log("Updated " + file + " to " + version); }'
	@./scripts/check-version.sh "$(VERSION)"

release: \
	check-version-argument \
	check-tools \
	check-clean \
	check-branch \
	check-tag \
	check-synced
	@echo
	@echo "Preparing Waxlight Launcher $(RELEASE_TAG)..."
	@echo

	$(MAKE) set-version VERSION="$(VERSION)"
	$(MAKE) release-check VERSION="$(VERSION)"

	@echo
	@echo "Creating release commit..."
	$(GIT) add wails.json cmd/waxlight/wails.json
	$(GIT) commit -m "chore(release): $(RELEASE_TAG)"

	@echo
	@echo "Pushing $(BRANCH)..."
	$(GIT) push "$(REMOTE)" "$(BRANCH)"

	@echo
	@echo "Creating annotated tag $(RELEASE_TAG)..."
	$(GIT) tag \
		-a "$(RELEASE_TAG)" \
		-m "Waxlight Launcher $(RELEASE_TAG)"

	@echo
	@echo "Pushing release tag..."
	$(GIT) push "$(REMOTE)" "$(RELEASE_TAG)"

	@echo
	@echo "Release workflow started for $(RELEASE_TAG)."
	@echo "GitHub Actions will build Linux and Windows packages"
	@echo "and publish the GitHub Release automatically."
	@if command -v gh >/dev/null 2>&1; then \
		echo; \
		echo "Recent GitHub Actions runs:"; \
		gh run list --limit 5 || true; \
	fi

clean:
	$(GO) clean -cache
	rm -rf build/bin
	rm -rf release
	rm -rf frontend/dist

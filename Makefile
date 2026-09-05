SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

GO ?= go
GOFMT ?= gofmt
NPM ?= npm
GIT ?= git

VERSION ?=
RELEASE_DIR ?= release
BRANCH ?= dev
REMOTE ?= origin
AUTO ?=

WAILS_VERSION ?= v2.11.0
GOVULNCHECK_VERSION ?= v1.6.0

WAILS_TAGS := desktop,production

ifeq ($(shell $(GO) env GOOS),linux)
WAILS_TAGS := $(WAILS_TAGS),webkit2_41
endif

CURRENT_VERSION := $(shell node -p "require('./wails.json').info.productVersion" 2>/dev/null || echo unknown)
RELEASE_TAG = v$(VERSION)
RELEASE_BRANCH = release/$(RELEASE_TAG)

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
	architecture \
	nix-update-hash \
	security-patterns \
	vulncheck \
	api-inventory \
	api-docs \
	api-docs-dev \
	api-docs-build \
	api-docs-preview \
	package-linux \
	release-check \
	check-version-argument \
	check-tools \
	check-clean \
	check-branch \
	check-tag \
	check-synced \
	release-notes \
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
	@echo "  make nix-update-hash"
	@echo "      Update the Go vendorHash in nix/waxlight.nix when a nix build"
	@echo "      reports a hash mismatch."
	@echo
	@echo "  make api-inventory"
	@echo "      Regenerate the checked-in Wails API inventory."
	@echo
	@echo "  make api-docs"
	@echo "      Generate the Wails API documentation and checked-in inventory."
	@echo
	@echo "  make api-docs-dev"
	@echo "      Regenerate API documentation and start the VitePress site."
	@echo
	@echo "  make api-docs-build"
	@echo "      Regenerate API documentation and build the static VitePress site."
	@echo
	@echo "  make release-check VERSION=X.Y.Z"
	@echo "      Run all checks for a release."
	@echo
	@echo "  make release VERSION=X.Y.Z [AUTO=1]"
	@echo "      Prepare releases/vX.Y.Z.md, wait for you to write the"
	@echo "      release notes (or generate them from commit history with"
	@echo "      AUTO=1), then update versions, validate, push a release branch,"
	@echo "      and open a pull request into dev. Merging dev into main publishes it."
	@echo
	@echo "  make release-notes VERSION=X.Y.Z [AUTO=1]"
	@echo "      Prepare or reuse releases/vX.Y.Z.md and wait for you to"
	@echo "      write the release notes without starting a release."
	@echo "      With AUTO=1 the notes are generated automatically."
	@echo
	@echo "Example:"
	@echo "  make release VERSION=0.1.3"

check-version-argument:
	@if [[ -z "$(VERSION)" ]]; then \
		echo "error: VERSION is required"; \
		echo "usage: make release VERSION=0.1.3"; \
		exit 1; \
	fi
	@if [[ ! "$(VERSION)" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$$ ]]; then \
		echo "error: invalid semantic version: $(VERSION)"; \
		exit 1; \
	fi

check-tools:
	@for command in "$(GO)" "$(NPM)" "$(GIT)" node gh; do \
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
	if ! $(GIT) merge-base --is-ancestor "$$remote_commit" "$$local_commit"; then \
		echo "error: local $(BRANCH) is behind or diverged from $(REMOTE)/$(BRANCH)"; \
		echo; \
		echo "Local:  $$local_commit"; \
		echo "Remote: $$remote_commit"; \
		echo; \
		echo "Update local $(BRANCH) before releasing."; \
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
	# The generated models.ts carries whitespace-only blank lines; normalize
	# them so the checked-in bindings stay diff-check clean.
	sed -i 's/[[:space:]]*$$//' frontend/src/wailsjs/go/models.ts

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

api-inventory api-docs:
	$(GO) run github.com/AmadoMuerte/wailsdoc/cmd/wailsdoc generate

api-docs-dev: api-docs
	$(GO) run github.com/AmadoMuerte/wailsdoc/cmd/wailsdoc serve

api-docs-build: api-docs
	$(GO) run github.com/AmadoMuerte/wailsdoc/cmd/wailsdoc build

api-docs-preview: api-docs-build
	$(NPM) --prefix docs/site run preview

format:
	$(GOFMT) -w $$($(GIT) ls-files --cached --others --exclude-standard '*.go' | while IFS= read -r file; do [[ -f "$$file" ]] && printf '%s\n' "$$file"; done)
	$(NPM) --prefix frontend run format

format-check:
	@unformatted="$$($(GOFMT) -l $$($(GIT) ls-files --cached --others --exclude-standard '*.go' | while IFS= read -r file; do [[ -f "$$file" ]] && printf '%s\n' "$$file"; done))"; \
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

architecture:
	./scripts/check-architecture.sh

nix-update-hash:
	./scripts/update-nix-vendor-hash.sh

security-patterns:
	./scripts/check-security-patterns.sh

vulncheck:
	GOTOOLCHAIN=auto \
	$(GO) run \
		golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) \
		./...

security: security-patterns vulncheck architecture

package-linux: check-version-argument
	./scripts/build-linux.sh "$(VERSION)" "$(RELEASE_DIR)"

release-check: check-version-argument frontend-install
	./scripts/check-version.sh "$(VERSION)"
	$(MAKE) test
	$(MAKE) race
	$(MAKE) vet
	$(MAKE) security

set-version: check-version-argument
	@VERSION="$(VERSION)" node -e 'const fs = require("fs"); const version = process.env.VERSION; const files = ["wails.json", "cmd/waxlight/wails.json", "internal/version/wails.json"]; for (const file of files) { const value = JSON.parse(fs.readFileSync(file, "utf8")); value.info = value.info || {}; value.info.productVersion = version; fs.writeFileSync(file, JSON.stringify(value, null, 2) + "\n"); console.log("Updated " + file + " to " + version); }'
	@./scripts/check-version.sh "$(VERSION)"

release-notes: check-version-argument
	./scripts/prepare-release-notes.sh "$(VERSION)" "$(AUTO)"

release: \
	check-version-argument \
	check-tools \
	check-clean \
	check-branch \
	check-tag \
	check-synced
	@gh auth status >/dev/null
	$(MAKE) release-notes VERSION="$(VERSION)" AUTO="$(AUTO)"
	@echo
	@echo "Preparing Waxlight Launcher $(RELEASE_TAG)..."
	@echo

	$(MAKE) set-version VERSION="$(VERSION)"
	$(MAKE) release-check VERSION="$(VERSION)"

	@echo
	@echo "Creating $(RELEASE_BRANCH)..."
	$(GIT) switch -c "$(RELEASE_BRANCH)"
	$(GIT) add wails.json cmd/waxlight/wails.json internal/version/wails.json releases/v$(VERSION).md
	$(GIT) commit -m "chore(release): $(RELEASE_TAG)"

	@echo
	@echo "Pushing $(RELEASE_BRANCH)..."
	$(GIT) push -u "$(REMOTE)" "$(RELEASE_BRANCH)"

	@echo
	gh pr create \
		--base "$(BRANCH)" \
		--head "$(RELEASE_BRANCH)" \
		--title "chore(release): $(RELEASE_TAG)" \
		--body "Prepare Waxlight Launcher $(RELEASE_TAG). After this is merged into dev, promote dev to main to publish the release."

clean:
	$(GO) clean -cache
	rm -rf build/bin
	rm -rf release
	rm -rf frontend/dist

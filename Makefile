GO ?= go
NPM ?= npm
VERSION ?= 0.1.2
RELEASE_DIR ?= release
WAILS_TAGS := desktop,production

ifeq ($(shell $(GO) env GOOS),linux)
WAILS_TAGS := $(WAILS_TAGS),webkit2_41
endif

.PHONY: build wails-build frontend test vet security package-linux release-check clean

build: frontend
	mkdir -p build
	$(GO) build -buildvcs=false -tags "$(WAILS_TAGS)" -ldflags "-w -s" -o build/waxlight ./cmd/waxlight

wails-build:
	cd cmd/waxlight && wails build

frontend:
	$(NPM) --prefix frontend run build

test: frontend
	$(GO) test ./...
	$(NPM) --prefix frontend test

vet: frontend
	$(GO) vet ./...

security:
	./scripts/check-security-patterns.sh
	govulncheck ./...

package-linux:
	./scripts/build-linux.sh $(VERSION) $(RELEASE_DIR)

release-check: test vet security
	./scripts/check-version.sh $(VERSION)

clean:
	$(GO) clean -cache

GO ?= go
NPM ?= npm
WAILS_TAGS := desktop,production

ifeq ($(shell $(GO) env GOOS),linux)
WAILS_TAGS := $(WAILS_TAGS),webkit2_41
endif

.PHONY: build wails-build frontend test clean

build: frontend
	mkdir -p build
	$(GO) build -buildvcs=false -tags "$(WAILS_TAGS)" -ldflags "-w -s" -o build/waxlight ./cmd/waxlight

wails-build:
	cd cmd/waxlight && wails build

frontend:
	$(NPM) --prefix frontend run build

test:
	$(GO) test ./...
	$(NPM) --prefix frontend test

clean:
	$(GO) clean -cache

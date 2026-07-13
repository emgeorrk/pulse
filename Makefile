APP_NAME := Pulse
BIN_DIR  := bin
BINARY   := $(BIN_DIR)/pulse
BUNDLE   := $(BIN_DIR)/$(APP_NAME).app
PLIST    := build/darwin/Info.plist
ICON     := build/darwin/AppIcon.icns
GOTAGS   ?=

.PHONY: all build build-debug bundle sign run run-debug once appicon generate test vet lint lint-fix clean help

all: sign ### build and sign the .app (default)

build: ### compile the binary (CGO required: systray/Cocoa + Mach)
	CGO_ENABLED=1 go build $(if $(GOTAGS),-tags $(GOTAGS),) -o $(BINARY) ./cmd/pulse

build-debug: ### compile with the pprof profiling server (-tags debug)
	$(MAKE) build GOTAGS=debug

run-debug: ### build, sign, and launch a debug build (pprof on localhost:6060)
	$(MAKE) run GOTAGS=debug

bundle: build ### build $(APP_NAME).app with LSUIElement=true (no Dock icon)
	rm -rf $(BUNDLE)
	mkdir -p $(BUNDLE)/Contents/MacOS $(BUNDLE)/Contents/Resources
	cp $(BINARY) $(BUNDLE)/Contents/MacOS/pulse
	cp $(PLIST) $(BUNDLE)/Contents/Info.plist
	cp $(ICON) $(BUNDLE)/Contents/Resources/AppIcon.icns

sign: bundle ### ad-hoc sign for local use
	codesign -s - --force $(BUNDLE)

run: sign ### build, sign, and launch
	open $(BUNDLE)

once: build ### print one metrics frame to stdout (sensor check without UI)
	$(BINARY) -once

appicon: ### regenerate $(ICON) from build/darwin/AppIcon.svg
	./scripts/gen-appicon.sh

generate: ### regenerate gomock mocks (go tool mockgen)
	go generate ./...

test: ### unit tests
	go test ./...

vet: ### static analysis
	go vet ./...

lint: ### golangci-lint
	golangci-lint run

lint-fix: ### golangci-lint with autofix
	golangci-lint run --fix

clean: ### remove build artifacts
	rm -rf $(BIN_DIR)

help: ### list targets
	@grep -E '^[a-zA-Z_-]+:.*### .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*### "}; {printf "  %-8s %s\n", $$1, $$2}'

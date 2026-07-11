APP_NAME := Pulse
BIN_DIR  := bin
BINARY   := $(BIN_DIR)/pulse
BUNDLE   := $(BIN_DIR)/$(APP_NAME).app
PLIST    := build/darwin/Info.plist

.PHONY: all build bundle sign run once test vet clean help

all: sign ### build and sign the .app (default)

build: ### compile the binary (CGO required: systray/Cocoa + Mach)
	CGO_ENABLED=1 go build -o $(BINARY) ./cmd/pulse

bundle: build ### build $(APP_NAME).app with LSUIElement=true (no Dock icon)
	rm -rf $(BUNDLE)
	mkdir -p $(BUNDLE)/Contents/MacOS
	cp $(BINARY) $(BUNDLE)/Contents/MacOS/pulse
	cp $(PLIST) $(BUNDLE)/Contents/Info.plist

sign: bundle ### ad-hoc sign for local use
	codesign -s - --force $(BUNDLE)

run: sign ### build, sign, and launch
	open $(BUNDLE)

once: build ### print one metrics frame to stdout (sensor check without UI)
	$(BINARY) -once

test: ### unit tests
	go test ./...

vet: ### static analysis
	go vet ./...

clean: ### remove build artifacts
	rm -rf $(BIN_DIR)

help: ### list targets
	@grep -E '^[a-zA-Z_-]+:.*### .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*### "}; {printf "  %-8s %s\n", $$1, $$2}'

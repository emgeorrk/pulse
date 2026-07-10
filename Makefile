APP_NAME := Pulse
BIN_DIR  := bin
BINARY   := $(BIN_DIR)/pulse
BUNDLE   := $(BIN_DIR)/$(APP_NAME).app
PLIST    := build/darwin/Info.plist

.PHONY: all build bundle sign run once test vet clean help

all: sign ### собрать и подписать .app (по умолчанию)

build: ### скомпилировать бинарник (CGO обязателен: systray/Cocoa + Mach)
	CGO_ENABLED=1 go build -o $(BINARY) ./cmd/pulse

bundle: build ### собрать $(APP_NAME).app с LSUIElement=true (без иконки в Dock)
	rm -rf $(BUNDLE)
	mkdir -p $(BUNDLE)/Contents/MacOS
	cp $(BINARY) $(BUNDLE)/Contents/MacOS/pulse
	cp $(PLIST) $(BUNDLE)/Contents/Info.plist

sign: bundle ### ad-hoc подпись для локального запуска
	codesign -s - --force $(BUNDLE)

run: sign ### собрать, подписать и запустить
	open $(BUNDLE)

once: build ### напечатать один кадр метрик в stdout (проверка сенсоров без UI)
	$(BINARY) -once

test: ### юнит-тесты
	go test ./...

vet: ### статический анализ
	go vet ./...

clean: ### удалить артефакты сборки
	rm -rf $(BIN_DIR)

help: ### список целей
	@grep -E '^[a-zA-Z_-]+:.*### .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*### "}; {printf "  %-8s %s\n", $$1, $$2}'

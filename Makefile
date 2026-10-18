# Bun's own scripts call `bun` by name, so it has to be on PATH.
export PATH := $(HOME)/.bun/bin:$(PATH)
BUN := bun

.PHONY: web build test install

web:
	cd web && $(BUN) install && $(BUN) run build
	touch web/dist/.gitkeep

build: web
	go build -o dropcatch ./cmd/dropcatch

test:
	go test ./...
	cd web && $(BUN) run verify

install: build
	./dropcatch install-agent

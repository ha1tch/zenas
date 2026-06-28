# Makefile - zenas, a Z80 macro assembler in Go
#
# Copyright (c) 2026 haitch
# Licensed under the Apache License, Version 2.0

BINARY  := zenas
PKG     := github.com/ha1tch/zenas
BIN_DIR := build
VERSION := $(shell cat VERSION 2>/dev/null)

GO      := go
GOFLAGS :=

.PHONY: all build test vet fmt smoke install clean version help test-sjasmplus

all: build

## build: compile the zenas binary into build/
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY) .
	@echo "built $(BIN_DIR)/$(BINARY) ($(VERSION))"

## test: run the Go test suite once
test:
	$(GO) test ./... -count=1

## vet: run go vet across the module
vet:
	$(GO) vet ./...

## fmt: format all Go source
fmt:
	$(GO) fmt ./...

## smoke: assemble the bundled examples as a quick sanity check
smoke: build
	@echo "-- smoke: assembling examples"
	@for f in examples/*.asm; do \
		printf '   %s ... ' "$$f"; \
		if $(BIN_DIR)/$(BINARY) assemble "$$f" /dev/null >/dev/null 2>&1; then \
			echo "ok"; \
		else \
			echo "FAIL (expected for in-progress syntax)"; \
		fi; \
	done

## test-sjasmplus: optional cross-check against sjasmplus (builds it from source).
## Verifies base Z80 and Z80N encodings byte-for-byte against the de facto ZX
## Spectrum Next assembler. Needs git, make and a C++17 compiler. Not part of
## `make test`. Override with SJASMPLUS=/path/to/sjasmplus to skip the build.
test-sjasmplus: build
	@SJASMPLUS="$(SJASMPLUS)" ZENAS="$(BIN_DIR)/$(BINARY)" tools/sjasmplus_compare.sh --build

## install: install zenas into GOBIN / GOPATH bin
install:
	$(GO) install $(GOFLAGS) .

## version: print the project version
version:
	@echo "$(VERSION)"

## clean: remove build artefacts
clean:
	rm -rf $(BIN_DIR)

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'

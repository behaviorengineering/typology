.PHONY: help build test vet

.DEFAULT_GOAL := help

BINARY := bin/typology
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/behaviorengineering/typology/internal/cli.version=$(VERSION)

help:
	@echo "typology — architecture discover, validate, emit"
	@echo ""
	@echo "  make build    Build $(BINARY)"
	@echo "  make test     go test ./..."
	@echo "  make vet      go vet ./..."

build:
	@mkdir -p $(dir $(BINARY))
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/typology

test:
	go test ./...

vet:
	go vet ./...

.PHONY: help build test vet smoke cable-board-sample

.DEFAULT_GOAL := help

BINARY := bin/typology
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/behaviorengineering/typology/internal/cli.version=$(VERSION)
SMOKE_REPO := testdata/tiny-module
SMOKE_CATALOG := $(SMOKE_REPO)/.typology/typology.yaml
VIEWER := viewer/cable-board

help:
	@echo "typology — architecture discover, validate, emit"
	@echo ""
	@echo "  make build               Build $(BINARY)"
	@echo "  make test                go test ./..."
	@echo "  make vet                 go vet ./..."
	@echo "  make smoke               Build + read-only CLI checks on $(SMOKE_REPO)"
	@echo "  make cable-board-sample  Harvest $(SMOKE_REPO) into $(VIEWER)/public/assembly-graph.json"

build:
	@mkdir -p $(dir $(BINARY))
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/typology

test:
	go test ./...

vet:
	go vet ./...

smoke: build
	@tmp=$$(mktemp -d) && \
	trap 'rm -rf "$$tmp"' EXIT && \
	./$(BINARY) validate $(SMOKE_REPO) && \
	./$(BINARY) show graph --catalog $(SMOKE_CATALOG) --json > "$$tmp/graph.json" && \
	python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d.get("nodes"), "graph nodes empty"' "$$tmp/graph.json" && \
	./$(BINARY) assembly-graph $(SMOKE_REPO) --out "$$tmp/assembly-graph.json" && \
	python3 scripts/check-assembly-graph.py "$$tmp/assembly-graph.json" && \
	echo "smoke: ok"

cable-board-sample: build
	./$(VIEWER)/scripts/load-graph.sh $(SMOKE_REPO) tiny-module --label "Tiny module sample" --make-default

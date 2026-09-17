.PHONY: help build test vet smoke cable-board-sample cable-board-slices

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
	@echo "  make cable-board-sample  Harvest $(SMOKE_REPO) into named tiny-module board"
	@echo "  make cable-board-slices  Project every catalog slice into named boards"

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
	./$(BINARY) assembly-graph $(SMOKE_REPO) --catalog $(SMOKE_CATALOG) --slice billing --out "$$tmp/billing.json" && \
	python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d.get("slice")=="billing"; assert d.get("nodes")' "$$tmp/billing.json" && \
	echo "smoke: ok"

cable-board-sample: build
	./$(VIEWER)/scripts/load-graph.sh $(SMOKE_REPO) tiny-module --label "Tiny module sample" --make-default

cable-board-slices: build
	./$(VIEWER)/scripts/load-graph.sh $(SMOKE_REPO) --all-slices --catalog $(SMOKE_CATALOG)
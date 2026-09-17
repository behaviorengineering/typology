.PHONY: help build test vet smoke cable-board-sample cable-board-slices cable-board-dist

.DEFAULT_GOAL := help

BINARY := bin/typology
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/behaviorengineering/typology/internal/cli.version=$(VERSION)
SMOKE_REPO := testdata/tiny-module
SMOKE_CATALOG := $(SMOKE_REPO)/.typology/typology.yaml
VIEWER := viewer/cable-board
VIEWER_DIST := internal/boardsviewer/dist

help:
	@echo "typology — architecture discover, validate, emit"
	@echo ""
	@echo "  make build               Build $(BINARY)"
	@echo "  make test                go test ./..."
	@echo "  make vet                 go vet ./..."
	@echo "  make smoke               Build + read-only CLI checks on $(SMOKE_REPO)"
	@echo "  make cable-board-dist    Build Vite SPA into $(VIEWER_DIST) for embed"
	@echo "  make cable-board-sample  Harvest $(SMOKE_REPO) into named tiny-module board"
	@echo "  make cable-board-slices  Project every catalog slice into named boards"

build:
	@mkdir -p $(dir $(BINARY))
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/typology

test:
	go test ./...

vet:
	go vet ./...

# Production SPA for typology boards serve (go:embed). Requires Node/npm.
cable-board-dist:
	cd $(VIEWER) && npm ci && npm run build
	rm -rf $(VIEWER_DIST)
	mkdir -p $(VIEWER_DIST)
	cp -R $(VIEWER)/dist/. $(VIEWER_DIST)/
	# Board JSON is served from XDG/--viewer at runtime, not from the embed.
	rm -rf $(VIEWER_DIST)/boards $(VIEWER_DIST)/boards.json $(VIEWER_DIST)/assembly-graph.json
	@test -f $(VIEWER_DIST)/index.html
	@test -d $(VIEWER_DIST)/assets
	@echo "cable-board-dist: wrote $(VIEWER_DIST)"

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
	bash scripts/check-prefix-boards.sh && \
	echo "smoke: ok"

cable-board-sample: build
	./$(BINARY) boards register $(SMOKE_REPO) tiny-module \
		--label "Tiny module sample" --make-default --viewer $(VIEWER)/public

cable-board-slices: build
	./$(BINARY) boards register $(SMOKE_REPO) --all-slices \
		--prefix tiny --catalog $(SMOKE_CATALOG) --viewer $(VIEWER)/public

package discover_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
	"github.com/behaviorengineering/typology/internal/discover"
	"github.com/behaviorengineering/typology/internal/gorepo"
)

func TestDiscover_tinyModule(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	result, err := discover.Run(discover.Options{RepoRoot: repo})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packages) < 3 {
		t.Fatalf("packages: %v", result.Packages)
	}
	if len(result.Typology.Slices) == 0 {
		t.Fatal("expected proposed slices")
	}
	for _, s := range result.Typology.Slices {
		if s.ID != "billing" {
			continue
		}
		kinds := map[catalog.DocPageKind]bool{}
		for _, p := range s.Docs.Pages {
			kinds[p.Kind] = true
		}
		if !kinds[catalog.DocOverview] || !kinds[catalog.DocComponents] || !kinds[catalog.DocContracts] {
			t.Fatalf("billing docs missing core pages: %+v", s.Docs.Pages)
		}
		if kinds[catalog.DocCLI] || kinds[catalog.DocPresentation] {
			t.Fatalf("billing docs should omit empty CLI/UI pages: %+v", s.Docs.Pages)
		}
		if kinds[catalog.DocPipelines] {
			t.Fatalf("discover should not invent pipelines without opRuns: %+v", s.Docs.Pages)
		}
		return
	}
	t.Fatal("expected billing slice")
}

func TestDiscover_graphSummary(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	summary, err := discover.AnalyzeGraph(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Nodes) == 0 {
		t.Fatal("expected graph nodes")
	}
	// tiny-module internal/ledger is a leaf (imported by billing/store, imports nothing)
	foundLedger := false
	for _, leaf := range summary.Leaves {
		if leaf == "./internal/ledger" {
			foundLedger = true
			break
		}
	}
	if !foundLedger {
		t.Fatalf("expected ./internal/ledger in leaves: %+v", summary.Leaves)
	}
}

func TestDiscover_workspaceGraph(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.work"), "go 1.26.5\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(root, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, "engine", "svc", "svc.go"), "package svc\n\nimport \"example.com/ws/lib/widget\"\n\nfunc Run() { _ = widget.New }\n")
	mustWrite(t, filepath.Join(root, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, "lib", "widget", "widget.go"), "package widget\n\nvar New = 1\n")

	modules, err := gorepo.ResolveModules(root, []string{"engine", "lib"}, "")
	if err != nil {
		t.Fatal(err)
	}
	summary, err := discover.AnalyzeGraphInModules(root, modules)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := summary.Nodes["./engine/svc"]; !ok {
		t.Fatalf("expected ./engine/svc node, got %+v", summary.Nodes)
	}
	if _, ok := summary.Nodes["./lib/widget"]; !ok {
		t.Fatalf("expected ./lib/widget node, got %+v", summary.Nodes)
	}
	imports := summary.Nodes["./engine/svc"].Imports
	found := false
	for _, imp := range imports {
		if imp == "./lib/widget" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected cross-module import ./lib/widget, got %+v", imports)
	}
}

func TestDiscover_workspaceRequiresScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.work"), "go 1.26.5\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(root, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, "engine", "svc", "svc.go"), "package svc\n")
	mustWrite(t, filepath.Join(root, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, "lib", "widget", "widget.go"), "package widget\n")

	if _, err := discover.AnalyzeGraph(root); err == nil {
		t.Fatal("expected an unscoped multi-module workspace to fail")
	}
}

func TestDiscover_workspaceScopeExcludesSibling(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.work"), "go 1.26.5\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(root, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, "engine", "svc", "svc.go"), "package svc\n")
	mustWrite(t, filepath.Join(root, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, "lib", "widget", "widget.go"), "package widget\n")

	modules, err := gorepo.ResolveModules(root, []string{"engine"}, "")
	if err != nil {
		t.Fatal(err)
	}
	summary, err := discover.AnalyzeGraphInModules(root, modules)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := summary.Nodes["./lib/widget"]; ok {
		t.Fatalf("out-of-scope package appeared in graph: %+v", summary.Nodes)
	}
	if _, ok := summary.Nodes["./engine/svc"]; !ok {
		t.Fatalf("in-scope package missing from graph: %+v", summary.Nodes)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

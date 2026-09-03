package sourceindex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestBuild_indexesFixturePackages(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	idx, err := sourceindex.Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	pkg, ok := idx.Package("internal/billing/httpapi")
	if !ok {
		t.Fatalf("expected package evidence for httpapi, got %+v", idx.Packages)
	}
	if pkg.Name != "httpapi" {
		t.Fatalf("package name = %q, want httpapi", pkg.Name)
	}
	if !pkg.HasStaticAnchor() {
		t.Fatalf("expected static anchor in %+v", pkg)
	}
	found := false
	for _, fn := range pkg.ExportedFuncs {
		if fn == "Handle" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Handle exported func, got %+v", pkg.ExportedFuncs)
	}
}

func TestBuild_indexesWorkspaceRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.work"), "go 1.26.5\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(root, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, "engine", "cmd", "app", "main.go"), "package main\n\nfunc main() {}\n")
	mustWrite(t, filepath.Join(root, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, "lib", "widget", "widget.go"), "package widget\n\nfunc New() {}\n")

	idx, err := sourceindex.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := idx.Package("engine/cmd/app"); !ok {
		t.Fatalf("expected engine/cmd/app, got %+v", idx.Packages)
	}
	if _, ok := idx.Package("lib/widget"); !ok {
		t.Fatalf("expected lib/widget, got %+v", idx.Packages)
	}
}

func TestBuild_marksUnanchoredPackage(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/sourceindex\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "internal", "empty", "empty.go"), "package empty\n\nfunc hidden() {}\n")

	idx, err := sourceindex.Build(repo)
	if err != nil {
		t.Fatal(err)
	}
	pkg, ok := idx.Package("internal/empty")
	if !ok {
		t.Fatalf("expected package evidence for internal/empty, got %+v", idx.Packages)
	}
	if pkg.HasStaticAnchor() {
		t.Fatalf("expected unanchored package, got %+v", pkg)
	}
	if strings.Contains(strings.Join(pkg.ExportedFuncs, ","), "hidden") {
		t.Fatalf("unexported helper should not be treated as anchor: %+v", pkg)
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

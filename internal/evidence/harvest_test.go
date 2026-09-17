package evidence_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/evidence"
	"github.com/behaviorengineering/typology/roles"
)

func TestHarvest_tinyModule_goOnly(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := evidence.Harvest(evidence.HarvestOptions{RepoRoot: repo})
	if err != nil {
		t.Fatal(err)
	}
	if !h.HasGo {
		t.Fatal("expected HasGo")
	}
	if h.HasPy {
		t.Fatal("expected no HasPy")
	}
	if len(h.Graph) == 0 || len(h.Topo.Packages) == 0 {
		t.Fatalf("empty harvest graph/topo: graph=%d pkgs=%d", len(h.Graph), len(h.Topo.Packages))
	}
}

func TestHarvest_pythonTiny(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", "..", "testdata", "python_tiny"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := evidence.Harvest(evidence.HarvestOptions{RepoRoot: repo})
	if err != nil {
		t.Fatal(err)
	}
	if !h.HasPy {
		t.Fatal("expected HasPy")
	}
	if h.HasGo {
		t.Fatal("expected no HasGo")
	}
}

func TestHarvest_mixedLanguages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/mixed\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(root, "internal", "svc", "svc.go"), "package svc\n\nfunc Run() {}\n")
	mustWrite(t, filepath.Join(root, "pyproject.toml"), "[project]\nname = \"mixed\"\n")
	mustWrite(t, filepath.Join(root, "src", "appboard", "__init__.py"), "")
	mustWrite(t, filepath.Join(root, "src", "appboard", "models.py"), "from dataclasses import dataclass\n\n@dataclass\nclass Board:\n    name: str\n")
	mustWrite(t, filepath.Join(root, "src", "appserver", "__init__.py"), "")
	mustWrite(t, filepath.Join(root, "src", "appserver", "api.py"), "from appboard.models import Board\n\ndef serve() -> Board:\n    return Board(name=\"x\")\n")

	h, err := evidence.Harvest(evidence.HarvestOptions{RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !h.HasGo || !h.HasPy {
		t.Fatalf("HasGo=%v HasPy=%v", h.HasGo, h.HasPy)
	}
	foundGo, foundPy := false, false
	for path := range h.Index.Packages {
		if strings.Contains(path, "internal/svc") || strings.HasSuffix(path, "svc") {
			foundGo = true
		}
		if strings.Contains(path, "appboard") || strings.Contains(path, "appserver") {
			foundPy = true
		}
	}
	if !foundGo || !foundPy {
		t.Fatalf("expected merged go+python packages, index=%v", h.Index.Packages)
	}
}

func TestHarvest_errors(t *testing.T) {
	t.Parallel()
	_, err := evidence.Harvest(evidence.HarvestOptions{})
	if err == nil {
		t.Fatal("empty root: expected error")
	}
	code, ok := terrors.CodeOf(err)
	if !ok || code != terrors.CodeInvalid {
		t.Fatalf("empty root code=%v ok=%v err=%v", code, ok, err)
	}

	empty := t.TempDir()
	_, err = evidence.Harvest(evidence.HarvestOptions{RepoRoot: empty})
	if err == nil {
		t.Fatal("module-less repo: expected error")
	}
}

func TestHarvest_moduleScope_workspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.work"), "go 1.27\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(root, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(root, "engine", "svc", "svc.go"), "package svc\n\nfunc Run() {}\n")
	mustWrite(t, filepath.Join(root, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(root, "lib", "widget", "widget.go"), "package widget\n\nvar New = 1\n")

	h, err := evidence.Harvest(evidence.HarvestOptions{RepoRoot: root, Module: "engine"})
	if err != nil {
		t.Fatal(err)
	}
	for path := range h.Index.Packages {
		if strings.Contains(path, "widget") || strings.Contains(path, "lib/") {
			t.Fatalf("moduleScope=engine should exclude lib packages, got %q", path)
		}
	}
	foundEngine := false
	for path := range h.Index.Packages {
		if strings.Contains(path, "svc") || strings.Contains(path, "engine") {
			foundEngine = true
			break
		}
	}
	if !foundEngine {
		t.Fatalf("expected engine packages, got %v", h.Index.Packages)
	}
}

func TestHarvest_catalogModules_workspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.work"), "go 1.27\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(root, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(root, "engine", "svc", "svc.go"), "package svc\n\nfunc Run() {}\n")
	mustWrite(t, filepath.Join(root, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(root, "lib", "widget", "widget.go"), "package widget\n\nvar New = 1\n")
	// Nested Python root must not mask an unscoped Go workspace.
	mustWrite(t, filepath.Join(root, "tools", "pyproject.toml"), "[project]\nname = \"tools\"\n")
	mustWrite(t, filepath.Join(root, "tools", "src", "toolpkg", "__init__.py"), "")

	_, err := evidence.Harvest(evidence.HarvestOptions{RepoRoot: root})
	if err == nil {
		t.Fatal("unscoped workspace: expected error")
	}
	if !strings.Contains(err.Error(), "multiple Go modules") {
		t.Fatalf("err=%v, want multiple Go modules", err)
	}

	h, err := evidence.Harvest(evidence.HarvestOptions{
		RepoRoot: root,
		Modules:  []string{"engine", "lib"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !h.HasGo {
		t.Fatal("expected HasGo with catalog modules")
	}
	foundEngine, foundLib := false, false
	for path := range h.Index.Packages {
		if strings.Contains(path, "svc") || strings.Contains(path, "engine") {
			foundEngine = true
		}
		if strings.Contains(path, "widget") || strings.Contains(path, "lib/") {
			foundLib = true
		}
	}
	if !foundEngine || !foundLib {
		t.Fatalf("expected engine+lib packages, got %v", h.Index.Packages)
	}
}

func TestWriteFiles_rolesRoundTrip(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := evidence.Harvest(evidence.HarvestOptions{RepoRoot: repo})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	contracts := filepath.Join(dir, "package_contracts.md")
	rolesOut := filepath.Join(dir, "package_roles.yaml")
	rlm := filepath.Join(dir, "package_rlm_context.md")
	if err := evidence.WriteFiles(h, contracts, rolesOut, rlm); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(rolesOut)
	if err != nil {
		t.Fatal(err)
	}
	topo, err := roles.ParseYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(topo.Packages) == 0 {
		t.Fatal("expected roles packages after round-trip")
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

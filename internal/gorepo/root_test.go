package gorepo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/typology/internal/gorepo"
)

func TestFindRoot_prefersGoWork(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.work"), "go 1.26.5\n\nuse (\n\t./engine\n)\n")
	mustWrite(t, filepath.Join(root, "engine", "go.mod"), "module example.com/engine\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, ".typology", "typology.yaml"), "id: demo\nslices: []\n")

	got, err := gorepo.FindRoot(filepath.Join(root, ".typology"))
	if err != nil {
		t.Fatal(err)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != absRoot {
		t.Fatalf("FindRoot = %q, want %q", got, absRoot)
	}
}

func TestModules_workspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.work"), "go 1.26.5\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(root, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(root, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.26.5\n")

	mods, err := gorepo.Modules(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 2 {
		t.Fatalf("modules = %+v", mods)
	}
	if mods[0].Rel != "engine" || mods[0].Path != "example.com/ws/engine" {
		t.Fatalf("engine module = %+v", mods[0])
	}
	if mods[1].Rel != "lib" || mods[1].Path != "example.com/ws/lib" {
		t.Fatalf("lib module = %+v", mods[1])
	}
}

func TestModules_workspacePreferredWhenRootAlsoHasGoMod(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.work"), "go 1.27\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/root\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(root, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(root, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.27\n")

	mods, err := gorepo.Modules(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 2 || mods[0].Rel != "engine" || mods[1].Rel != "lib" {
		t.Fatalf("modules = %+v, want workspace modules", mods)
	}
}

func TestModules_singleModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/single\n\ngo 1.26.5\n")

	mods, err := gorepo.Modules(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 || mods[0].Rel != "." || mods[0].Path != "example.com/single" {
		t.Fatalf("modules = %+v", mods)
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

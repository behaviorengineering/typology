package pyrepo_test

import (
	"os"
	"path/filepath"
	"testing"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/pyrepo"
)

func TestFindRoots_markersAndSkips(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "pyproject.toml"), "[project]\nname = \"root\"\n")
	mustWrite(t, filepath.Join(root, "apps", "api", "setup.cfg"), "[metadata]\nname = api\n")
	mustWrite(t, filepath.Join(root, "legacy", "setup.py"), "from setuptools import setup\nsetup(name='legacy')\n")
	mustWrite(t, filepath.Join(root, "venv", "lib", "pyproject.toml"), "[project]\nname = \"skip-me\"\n")
	mustWrite(t, filepath.Join(root, ".git", "hooks", "setup.py"), "print('no')\n")
	mustWrite(t, filepath.Join(root, "node_modules", "pkg", "setup.cfg"), "[metadata]\nname = npm\n")

	roots, err := pyrepo.FindRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]struct{}{}
	for _, r := range roots {
		got[r.Rel] = struct{}{}
	}
	for _, want := range []string{".", "apps/api", "legacy"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("missing root %q in %+v", want, roots)
		}
	}
	for _, skip := range []string{"venv/lib", ".git/hooks", "node_modules/pkg"} {
		if _, ok := got[skip]; ok {
			t.Fatalf("unexpected skipped root %q in %+v", skip, roots)
		}
	}
	if len(roots) < 2 {
		t.Fatalf("expected sorted nested roots, got %+v", roots)
	}
	for i := 1; i < len(roots); i++ {
		if roots[i-1].Rel > roots[i].Rel {
			t.Fatalf("roots not sorted by rel: %+v", roots)
		}
	}
}

func TestFindRoots_emptyRoot(t *testing.T) {
	t.Parallel()
	_, err := pyrepo.FindRoots("")
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := terrors.CodeOf(err)
	if !ok || code != terrors.CodeInvalid {
		t.Fatalf("code=%v ok=%v err=%v", code, ok, err)
	}
}

func TestHasPythonRoot(t *testing.T) {
	t.Parallel()
	pythonTiny, err := filepath.Abs(filepath.Join("..", "..", "testdata", "python_tiny"))
	if err != nil {
		t.Fatal(err)
	}
	ok, err := pyrepo.HasPythonRoot(pythonTiny)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected python_tiny to have a Python root")
	}
	empty := t.TempDir()
	ok, err = pyrepo.HasPythonRoot(empty)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("empty dir should not report a Python root")
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

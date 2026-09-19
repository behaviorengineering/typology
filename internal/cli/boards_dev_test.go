package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindCableBoardSrcExplicit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := filepath.Join(root, "viewer", "cable-board")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "package.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findCableBoardSrc(src)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	want, _ := filepath.Abs(src)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFindCableBoardSrcFromProvidersLayout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := filepath.Join(root, "providers", "typology", "viewer", "cable-board")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "package.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	got, err := findCableBoardSrc("")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	want, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
	}
	gotEval, _ := filepath.EvalSymlinks(got)
	wantEval, _ := filepath.EvalSymlinks(want)
	if gotEval != wantEval {
		t.Fatalf("got %q want %q", gotEval, wantEval)
	}
}

func TestSplitServeHostPort(t *testing.T) {
	t.Parallel()
	h, p := splitServeHostPort("127.0.0.1:5199")
	if h != "127.0.0.1" || p != "5199" {
		t.Fatalf("got %s %s", h, p)
	}
	h, p = splitServeHostPort(":5200")
	if h != "127.0.0.1" || p != "5200" {
		t.Fatalf("got %s %s", h, p)
	}
}

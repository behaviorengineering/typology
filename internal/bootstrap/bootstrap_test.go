package bootstrap_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/bootstrap"
)

func TestRun_singleModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/consumer\n\ngo 1.27\n")

	var calls []call
	var stdout, stderr bytes.Buffer
	result, err := bootstrap.Run(context.Background(), bootstrap.Options{
		RepoRoot: root,
		Stdout:   &stdout,
		Stderr:   &stderr,
		RunCommand: func(_ context.Context, dir string, args []string, _, _ io.Writer) error {
			calls = append(calls, call{dir: dir, args: append([]string(nil), args...)})
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Module.Dir != root {
		t.Fatalf("module dir = %q, want %q", result.Module.Dir, root)
	}
	if result.Version != bootstrap.DefaultToolVersion {
		t.Fatalf("version = %q, want %q", result.Version, bootstrap.DefaultToolVersion)
	}
	want := []call{
		{dir: root, args: []string{"get", "-tool", "github.com/behaviorengineering/typology/cmd/typology@v0.0.5"}},
		{dir: root, args: []string{"mod", "tidy"}},
		{dir: root, args: []string{"tool", "typology", "version"}},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRun_requiresModuleSelectorForWorkspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.work"), "go 1.27\n\nuse (\n\t./engine\n\t./lib\n)\n")
	writeFile(t, filepath.Join(root, "engine", "go.mod"), "module example.com/engine\n\ngo 1.27\n")
	writeFile(t, filepath.Join(root, "lib", "go.mod"), "module example.com/lib\n\ngo 1.27\n")

	_, err := bootstrap.Run(context.Background(), bootstrap.Options{RepoRoot: root})
	if err == nil {
		t.Fatal("Run() error = nil, want ambiguous workspace error")
	}
	if got := err.Error(); got == "" || !contains(got, "pass --module") {
		t.Fatalf("error = %q, want module selector guidance", got)
	}
}

func TestRun_selectsWorkspaceModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.work"), "go 1.27\n\nuse (\n\t./engine\n\t./lib\n)\n")
	writeFile(t, filepath.Join(root, "engine", "go.mod"), "module example.com/engine\n\ngo 1.27\n")
	writeFile(t, filepath.Join(root, "lib", "go.mod"), "module example.com/lib\n\ngo 1.27\n")

	var dirs []string
	_, err := bootstrap.Run(context.Background(), bootstrap.Options{
		RepoRoot: root,
		Module:   "./engine",
		RunCommand: func(_ context.Context, dir string, _ []string, _, _ io.Writer) error {
			dirs = append(dirs, dir)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(root, "engine")
	if len(dirs) != 3 {
		t.Fatalf("command count = %d, want 3", len(dirs))
	}
	for _, dir := range dirs {
		if dir != wantDir {
			t.Fatalf("command dir = %q, want %q", dir, wantDir)
		}
	}
}

func TestRun_rejectsUnpinnedVersion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/consumer\n\ngo 1.27\n")

	_, err := bootstrap.Run(context.Background(), bootstrap.Options{
		RepoRoot: root,
		Version:  "latest",
	})
	if err == nil {
		t.Fatal("Run() error = nil, want invalid version error")
	}
	if got := err.Error(); got == "" || !contains(got, "tagged Go module version") {
		t.Fatalf("error = %q, want tagged version guidance", got)
	}
}

type call struct {
	dir  string
	args []string
}

func contains(s, want string) bool {
	return strings.Contains(s, want)
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

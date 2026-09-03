package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/cli"
)

func TestCLI_validate_ok(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	var out, errOut bytes.Buffer
	code := cli.Run([]string{"validate", repo}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "validate: ok") {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestCLI_show_slices(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	var out, errOut bytes.Buffer
	code := cli.Run([]string{"show", "--catalog", filepath.Join(repo, ".typology", "typology.yaml")}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "billing") {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestCLI_nil_writers(t *testing.T) {
	t.Parallel()
	if code := cli.Run([]string{"version"}, nil, nil, &bytes.Buffer{}); code != 2 {
		t.Fatalf("nil stdout exit=%d", code)
	}
	if code := cli.Run([]string{"version"}, nil, &bytes.Buffer{}, nil); code != 2 {
		t.Fatalf("nil stderr exit=%d", code)
	}
}

func TestCLI_init_rejectsUnpinnedVersion(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/consumer\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := cli.Run([]string{"init", repo, "--version", "latest"}, nil, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "tagged Go module version") {
		t.Fatalf("stderr=%q", errOut.String())
	}
}

func TestCLI_show_graph(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	var out, errOut bytes.Buffer
	code := cli.Run([]string{"show", "graph", "--catalog", filepath.Join(repo, ".typology", "typology.yaml")}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Typology Import Graph") {
		t.Fatalf("stdout=%q", out.String())
	}
	if !strings.Contains(out.String(), "Source evidence (AST)") {
		t.Fatalf("stdout missing source evidence summary: %q", out.String())
	}
}

func TestCLI_architecture_writesReport(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	outPath := filepath.Join(t.TempDir(), "architecture.md")
	var out, errOut bytes.Buffer
	code := cli.Run([]string{"architecture", repo, "--out", outPath}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# Typology Architecture Brief") {
		t.Fatalf("report missing title: %s", data)
	}
	if !strings.Contains(out.String(), "architecture: wrote") {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestCLI_architecture_requiresScope(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.work"), "go 1.26.5\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(repo, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "engine", "svc", "svc.go"), "package svc\n")
	mustWrite(t, filepath.Join(repo, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "lib", "widget", "widget.go"), "package widget\n")
	mustWrite(t, filepath.Join(repo, ".typology", "typology.yaml"), "id: workspace\nslices: []\n")

	var out, errOut bytes.Buffer
	code := cli.Run([]string{"architecture", repo}, nil, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit=%d, want 1; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "declare scope.modules or pass --module") {
		t.Fatalf("stderr=%q", errOut.String())
	}
}

func TestCLI_architecture_moduleOverride(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.work"), "go 1.26.5\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(repo, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "engine", "svc", "svc.go"), "package svc\n\nfunc Run() {}\n")
	mustWrite(t, filepath.Join(repo, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "lib", "widget", "widget.go"), "package widget\n")
	mustWrite(t, filepath.Join(repo, ".typology", "typology.yaml"), `id: workspace
slices:
  - id: service
    objective: Run the service.
    owns:
      - id: service-package
        path: engine/svc
        layer: domain
`)
	outPath := filepath.Join(repo, "architecture.md")

	var out, errOut bytes.Buffer
	code := cli.Run([]string{"architecture", repo, "--module", "engine", "--out", outPath}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- `engine`") {
		t.Fatalf("report missing selected module: %s", data)
	}
}

func TestCLI_discover_defaultDraftPath(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/draft-test\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "internal", "billing", "billing.go"), "package billing\n")
	mustWrite(t, filepath.Join(repo, "internal", "ledger", "ledger.go"), "package ledger\n")

	var out, errOut bytes.Buffer
	code := cli.Run([]string{"discover", repo}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}

	expectedPath := filepath.Join(repo, "tmp", "typology", "typology.yaml")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected draft catalog at %s: %v\nstdout=%q", expectedPath, err, out.String())
	}
	if !strings.Contains(out.String(), expectedPath) {
		t.Fatalf("stdout did not report draft path %q: %q", expectedPath, out.String())
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

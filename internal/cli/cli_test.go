package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
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
	if !strings.Contains(string(data), "./engine/svc") {
		t.Fatalf("report missing scoped engine package: %s", data)
	}
	if strings.Contains(string(data), "./lib/widget") {
		t.Fatalf("report should not include unscoped lib package: %s", data)
	}
}

func TestCLI_discover_defaultDraftPath(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/draft-test\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "internal", "billing", "billing.go"), "package billing\n\nfunc Charge() {}\n")
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
	contractsPath := filepath.Join(repo, "tmp", "typology", "package_contracts.md")
	data, err := os.ReadFile(contractsPath)
	if err != nil {
		t.Fatalf("expected package contracts at %s: %v", contractsPath, err)
	}
	if !strings.Contains(string(data), "Charge") {
		t.Fatalf("contracts missing Charge export:\n%s", data)
	}
}

func TestCLI_contracts(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/contracts-test\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "internal", "cliexec", "exec.go"), "package cliexec\n\nfunc Run(cmd string) error { return nil }\n")

	var out, errOut bytes.Buffer
	code := cli.Run([]string{"contracts", repo}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	contractsPath := filepath.Join(repo, "tmp", "typology", "package_contracts.md")
	data, err := os.ReadFile(contractsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "./internal/cliexec") {
		t.Fatalf("missing cliexec path:\n%s", text)
	}
	if !strings.Contains(text, "Run") {
		t.Fatalf("missing Run export:\n%s", text)
	}
	if !strings.Contains(text, "hasMain: false") {
		t.Fatalf("expected hasMain false:\n%s", text)
	}
}

func TestCLI_assemblyGraph_tinyModule(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	out := filepath.Join(t.TempDir(), "assembly-graph.json")
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"assembly-graph", repo, "--out", out}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "assembly-graph: wrote") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"nodes"`) || !strings.Contains(string(raw), `"edges"`) {
		t.Fatalf("unexpected json: %s", raw)
	}
	if !strings.Contains(string(raw), "fills_dto") && !strings.Contains(string(raw), `"role"`) {
		t.Fatalf("expected roles or role kinds in json: %s", raw)
	}
}

func TestCLI_emit_ok(t *testing.T) {
	t.Parallel()
	src, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	typ, err := catalog.LoadYAML(filepath.Join(src, ".typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	catalogPath := filepath.Join(repo, ".typology", "typology.yaml")
	if err := catalog.SaveYAML(catalogPath, typ); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"emit", repo, "--catalog", catalogPath, "--go-only"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "emit: ok") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(repo, ".typology", "README.md")); err != nil {
		t.Fatalf("README not written: %v", err)
	}
}

func TestCLI_remediate(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"remediate", repo, "billing"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	code = cli.Run([]string{"remediate", repo, "no-such-slice"}, nil, &stdout, &stderr)
	if code == 0 {
		t.Fatal("unknown slice should fail")
	}
}

func TestCLI_validate_missingDoc_exits1(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	typ, err := catalog.LoadYAML(filepath.Join(repo, ".typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	typ.Slices[0].Docs.Pages = []catalog.DocPage{{
		Kind: catalog.DocOverview,
		Path: "docs/develop/billing/missing-overview.md",
	}}
	catalogPath := filepath.Join(t.TempDir(), "typology.yaml")
	if err := catalog.SaveYAML(catalogPath, typ); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"validate", repo, "--catalog", catalogPath, "billing"}, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d want 1 stderr=%s", code, stderr.String())
	}
}

func TestCLI_show_json(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	catalogPath := filepath.Join(repo, ".typology", "typology.yaml")
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"show", "--catalog", catalogPath, "--json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var catalogDoc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &catalogDoc); err != nil {
		t.Fatalf("catalog json: %v\n%s", err, stdout.String())
	}
	if _, ok := catalogDoc["slices"]; !ok {
		t.Fatalf("catalog json missing slices: %v", catalogDoc)
	}

	stdout.Reset()
	stderr.Reset()
	code = cli.Run([]string{"show", "graph", "--catalog", catalogPath, "--json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("graph exit=%d stderr=%s", code, stderr.String())
	}
	var graphDoc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &graphDoc); err != nil {
		t.Fatalf("graph json: %v\n%s", err, stdout.String())
	}
	if _, ok := graphDoc["nodes"]; !ok {
		t.Fatalf("graph json missing nodes: %v", graphDoc)
	}
}

func TestCLI_init_pinnedVersion(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/init-ok\n\ngo 1.27\n")
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"init", repo, "--version", "v0.0.5"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "init: configured") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCLI_usageErrors(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"nope"}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("unknown command exit=%d", code)
	}
	stderr.Reset()
	if code := cli.Run([]string{"validate"}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("validate missing repo exit=%d stderr=%s", code, stderr.String())
	}
	stderr.Reset()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	if code := cli.Run([]string{"architecture", repo, "--nope"}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("unknown flag exit=%d stderr=%s", code, stderr.String())
	}
	stderr.Reset()
	if code := cli.Run([]string{"assembly-graph"}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("assembly-graph usage exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage: typology assembly-graph") {
		t.Fatalf("stderr=%q", stderr.String())
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

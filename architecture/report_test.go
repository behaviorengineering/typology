package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/architecture"
	"github.com/behaviorengineering/typology/catalog"
)

func TestBuildAndRenderMarkdown(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "testdata", "tiny-module")
	typ, err := catalog.LoadYAML(filepath.Join(repo, ".typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	report, err := architecture.Build(architecture.BuildOptions{
		RepoRoot: repo,
		Catalog:  typ,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("unexpected findings: %+v", report.Findings)
	}
	body, err := architecture.RenderMarkdown(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Typology Architecture Brief",
		"## Intended architecture",
		"## Observed topology",
		"## Agent review protocol",
		"`billing`",
		"internal/billing/store",
		"```mermaid",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered report missing %q:\n%s", want, body)
		}
	}
	bodyAgain, err := architecture.RenderMarkdown(report)
	if err != nil {
		t.Fatal(err)
	}
	if bodyAgain != body {
		t.Fatal("rendered report is not deterministic")
	}
}

func TestBuildReportsUnmappedPackage(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "testdata", "tiny-module")
	typ, err := catalog.LoadYAML(filepath.Join(repo, ".typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	typ.Slices = typ.Slices[:1]
	typ.SliceBindings = nil
	typ.ComponentBindings = nil

	report, err := architecture.Build(architecture.BuildOptions{
		RepoRoot: repo,
		Catalog:  typ,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, finding := range report.Findings {
		if strings.Contains(finding.Message, "unmapped package") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected unmapped package finding, got %+v", report.Findings)
	}
}

func TestBuildHonorsCatalogScope(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.work"), "go 1.26.5\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(repo, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "engine", "svc", "svc.go"), "package svc\n")
	mustWrite(t, filepath.Join(repo, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "lib", "widget", "widget.go"), "package widget\n")

	report, err := architecture.Build(architecture.BuildOptions{
		RepoRoot: repo,
		Catalog: catalog.Typology{
			ID:    "workspace",
			Scope: catalog.Scope{Modules: []string{"engine"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Modules) != 1 || report.Modules[0] != "engine" {
		t.Fatalf("report modules = %+v, want [engine]", report.Modules)
	}
	if _, ok := report.Graph.Nodes["./lib/widget"]; ok {
		t.Fatalf("out-of-scope package appeared in report: %+v", report.Graph.Nodes)
	}
	body, err := architecture.RenderMarkdown(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "- `engine`") {
		t.Fatalf("report does not name inspected module:\n%s", body)
	}
}

func TestBuildReportsForbiddenBinding(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "testdata", "tiny-module")
	typ, err := catalog.LoadYAML(filepath.Join(repo, ".typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	typ.ComponentBindings = []catalog.ComponentBinding{{
		From: "billing-store",
		To:   "ledger-core",
		Rule: catalog.BindingMustNot,
	}}

	report, err := architecture.Build(architecture.BuildOptions{
		RepoRoot: repo,
		Catalog:  typ,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, finding := range report.Findings {
		if strings.Contains(finding.Message, "forbidden") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected forbidden binding finding, got %+v", report.Findings)
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

func TestWriteMarkdownPreservesHumanFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "docs", "architecture.md")
	body := "# Generated\n\n" + architecture.GeneratedMarker + "\n"
	written, err := architecture.WriteMarkdown(path, body)
	if err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("expected initial report to be written")
	}

	human := "# Human architecture\n\nReviewed by the team.\n"
	if err := os.WriteFile(path, []byte(human), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err = architecture.WriteMarkdown(path, body)
	if err != nil {
		t.Fatal(err)
	}
	if written {
		t.Fatal("expected human-owned report to be preserved")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != human {
		t.Fatalf("human report changed: %s", data)
	}
}

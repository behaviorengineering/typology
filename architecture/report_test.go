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

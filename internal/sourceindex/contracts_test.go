package sourceindex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/gorepo"
	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestFormatPackageContractsMarkdown(t *testing.T) {
	t.Parallel()
	idx := sourceindex.Index{
		Packages: map[string]sourceindex.PackageEvidence{
			"internal/cliexec": {
				Path:          "internal/cliexec",
				Name:          "cliexec",
				ExportedFuncs: []string{"Run", "Command"},
			},
			"cmd/app": {
				Path:    "cmd/app",
				Name:    "main",
				HasMain: true,
			},
		},
	}
	md := sourceindex.FormatPackageContractsMarkdown(idx)
	if !strings.Contains(md, "## ./cmd/app") {
		t.Fatalf("missing cmd/app section:\n%s", md)
	}
	if !strings.Contains(md, "hasMain: true") {
		t.Fatalf("expected hasMain true:\n%s", md)
	}
	if !strings.Contains(md, "## ./internal/cliexec") {
		t.Fatalf("missing cliexec section:\n%s", md)
	}
	if !strings.Contains(md, "exportedFuncs: Run, Command") {
		t.Fatalf("missing cliexec exports:\n%s", md)
	}
	if !strings.Contains(md, "hasMain: false") {
		t.Fatalf("expected hasMain false for cliexec:\n%s", md)
	}
}

func TestWritePackageContractsFile_tinyModule(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	out := filepath.Join(t.TempDir(), "package_contracts.md")
	modules, err := gorepo.ResolveModules(repo, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceindex.WritePackageContractsFile(repo, modules, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "Handle") {
		t.Fatalf("expected Handle export in contracts:\n%s", text)
	}
	if !strings.Contains(text, "internal/billing/httpapi") {
		t.Fatalf("expected httpapi path in contracts:\n%s", text)
	}
}

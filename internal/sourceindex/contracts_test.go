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
				Path:         "cmd/app",
				Name:         "main",
				HasMain:      true,
				DeliveryHint: sourceindex.DeliveryCLI,
			},
			"internal/board": {
				Path:          "internal/board",
				Name:          "board",
				PackageDoc:    "Package board holds JSON data types for the board.",
				ExportedDecls: []string{"Item"},
				JSONTags:      true,
				DeliveryHint:  sourceindex.DeliveryDTO,
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
	if !strings.Contains(md, "deliveryHint: cli") {
		t.Fatalf("expected cli hint:\n%s", md)
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
	if !strings.Contains(md, "packageDoc: Package board holds JSON data types") {
		t.Fatalf("expected packageDoc:\n%s", md)
	}
	if !strings.Contains(md, "deliveryHint: dto") {
		t.Fatalf("expected dto hint:\n%s", md)
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
	if !strings.Contains(text, "internal/board") {
		t.Fatalf("expected board path:\n%s", text)
	}
	if !strings.Contains(text, "jsonTags: true") {
		t.Fatalf("expected jsonTags for board:\n%s", text)
	}
	if !strings.Contains(text, "deliveryHint: dto") {
		t.Fatalf("expected dto deliveryHint for board:\n%s", text)
	}
	if !strings.Contains(text, "packageDoc: Package board holds JSON data types for the board.") {
		t.Fatalf("expected packageDoc for board:\n%s", text)
	}
	if !strings.Contains(text, "internal/server") {
		t.Fatalf("expected server path:\n%s", text)
	}
	if !strings.Contains(text, "goEmbed: true") {
		t.Fatalf("expected goEmbed for server:\n%s", text)
	}
	if !strings.Contains(text, "Mux.ServeHTTP") {
		t.Fatalf("expected exported method Mux.ServeHTTP:\n%s", text)
	}
	if !strings.Contains(text, "deliveryHint: server-ui") {
		t.Fatalf("expected server-ui for server:\n%s", text)
	}
	if !strings.Contains(text, "internal/grpcserver") {
		t.Fatalf("expected grpcserver path:\n%s", text)
	}
	if !strings.Contains(text, "deliveryHint: server-grpc") {
		t.Fatalf("expected server-grpc for grpcserver:\n%s", text)
	}
}

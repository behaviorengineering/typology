package validate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
	"github.com/behaviorengineering/typology/validate"
)

func TestValidate_tinyModule_ok(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "testdata", "tiny-module")
	catalogPath := filepath.Join(repo, ".typology", "typology.yaml")
	typ, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	issues := validate.Run(validate.Options{RepoRoot: repo, Catalog: typ})
	if len(issues) != 0 {
		t.Fatalf("validate: %v", issues)
	}
}

func TestValidate_tinyModule_missingDoc(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "testdata", "tiny-module")
	catalogPath := filepath.Join(repo, ".typology", "typology.yaml")
	typ, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	// Point one page at a missing path.
	typ.Slices[0].Docs.Pages = []catalog.DocPage{{
		Kind: catalog.DocOverview,
		Path: "docs/develop/billing/missing-overview.md",
	}}
	issues := validate.Run(validate.Options{RepoRoot: repo, Catalog: typ, SliceID: "billing"})
	found := false
	for _, issue := range issues {
		if issue.Slice == "billing" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected billing doc issues, got %v", issues)
	}
}

func TestValidate_tinyModule_missingSubprogramPage(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/missingpage\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "pkg.go"), "package root\n\nfunc Root() {}\n")
	typ := catalog.Typology{
		ID: "tiny",
		Slices: []catalog.Slice{{
			ID: "billing",
			Owns: []catalog.Component{{
				ID: "billing-store", Path: ".", Layer: catalog.LayerDomain,
			}},
			Subprograms: []catalog.Subprogram{{
				ID: "invoice", OwnerComponent: "billing-store",
				Objective: "Mint an invoice record from a store request.",
			}},
			Docs: catalog.DocCluster{Pages: []catalog.DocPage{{
				Kind: catalog.DocOverview,
				Path: "overview.md",
			}}},
		}},
	}
	if err := os.WriteFile(filepath.Join(repo, "overview.md"), []byte("# o\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	issues := validate.Run(validate.Options{RepoRoot: repo, Catalog: typ, SliceID: "billing", SkipImports: true})
	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "subprogram page missing") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected missing subprogram page, got %v", issues)
	}
}
func TestValidate_tinyModule_unmappedPackage(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "testdata", "tiny-module")
	catalogPath := filepath.Join(repo, ".typology", "typology.yaml")
	typ, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	// Drop the ledger slice so internal/ledger is unmapped.
	typ.Slices = []catalog.Slice{typ.Slices[0]}
	typ.SliceBindings = nil
	typ.ComponentBindings = nil

	issues := validate.Run(validate.Options{RepoRoot: repo, Catalog: typ})
	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Message, "unmapped package") && strings.Contains(issue.Message, "internal/ledger") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected unmapped package issue for internal/ledger, got %v", issues)
	}
}

func TestValidate_surfaceMissingStaticAnchor(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/anchortest\n\ngo 1.26.5\n")
	mustWrite(t, filepath.Join(repo, "internal", "billing", "store", "store.go"), "package store\n\nfunc Total() int { return 1 }\n")
	mustWrite(t, filepath.Join(repo, "internal", "billing", "httpapi", "http.go"), "package httpapi\n\nfunc handle() int { return 1 }\n")
	mustWrite(t, filepath.Join(repo, "docs", "develop", "billing", "overview.md"), "# Overview\n")
	mustWrite(t, filepath.Join(repo, "docs", "develop", "billing", "components.md"), "# Components\n")

	typ := catalog.Typology{
		ID: "tiny",
		Slices: []catalog.Slice{{
			ID:        "billing",
			Objective: "Test source anchors for billing.",
			Owns: []catalog.Component{
				{ID: "billing-store", Path: "internal/billing/store", Layer: catalog.LayerDomain},
			},
			Surfaces: []catalog.Surface{{
				ID:   "billing-api",
				Kind: catalog.InteractionAPI,
				Components: []catalog.Component{{
					ID:   "billing-http",
					Path: "internal/billing/httpapi",
				}},
			}},
			Docs: catalog.DocCluster{Pages: []catalog.DocPage{
				{Kind: catalog.DocOverview, Path: "docs/develop/billing/overview.md"},
				{Kind: catalog.DocComponents, Path: "docs/develop/billing/components.md"},
			}},
		}},
	}

	issues := validate.Run(validate.Options{RepoRoot: repo, Catalog: typ})
	if !hasMessage(issues, "surface \"billing-api\": no static source anchor found in 1 component package(s)") {
		t.Fatalf("expected missing source anchor issue, got %v", issues)
	}
}

func TestValidate_surfaceStaticAnchor_ok(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "testdata", "tiny-module")
	catalogPath := filepath.Join(repo, ".typology", "typology.yaml")
	typ, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	issues := validate.Run(validate.Options{RepoRoot: repo, Catalog: typ, SliceID: "billing"})
	if hasMessage(issues, "no static source anchor") {
		t.Fatalf("expected billing API surface to be anchored, got %v", issues)
	}
}

func hasMessage(issues []catalog.Issue, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue.Message, want) {
			return true
		}
	}
	return false
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

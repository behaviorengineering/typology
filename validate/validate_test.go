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
	catalogPath := filepath.Join(repo, "architecture", "typology.yaml")
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
	catalogPath := filepath.Join(repo, "architecture", "typology.yaml")
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
	typ := catalog.Typology{
		ID: "tiny",
		Slices: []catalog.Slice{{
			ID: "billing",
			Owns: []catalog.Component{{
				ID: "billing-store", Path: ".", Layer: catalog.LayerDomain,
			}},
			Subprograms: []catalog.Subprogram{{
				ID: "invoice", OwnerComponent: "billing-store",
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

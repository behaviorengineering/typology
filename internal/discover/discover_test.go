package discover_test

import (
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
	"github.com/behaviorengineering/typology/internal/discover"
)

func TestDiscover_tinyModule(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	result, err := discover.Run(discover.Options{RepoRoot: repo})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packages) < 3 {
		t.Fatalf("packages: %v", result.Packages)
	}
	if len(result.Typology.Slices) == 0 {
		t.Fatal("expected proposed slices")
	}
	for _, s := range result.Typology.Slices {
		if s.ID != "billing" {
			continue
		}
		kinds := map[catalog.DocPageKind]bool{}
		for _, p := range s.Docs.Pages {
			kinds[p.Kind] = true
		}
		if !kinds[catalog.DocOverview] || !kinds[catalog.DocComponents] || !kinds[catalog.DocContracts] {
			t.Fatalf("billing docs missing core pages: %+v", s.Docs.Pages)
		}
		if kinds[catalog.DocCLI] || kinds[catalog.DocPresentation] {
			t.Fatalf("billing docs should omit empty CLI/UI pages: %+v", s.Docs.Pages)
		}
		if kinds[catalog.DocPipelines] {
			t.Fatalf("discover should not invent pipelines without opRuns: %+v", s.Docs.Pages)
		}
		return
	}
	t.Fatal("expected billing slice")
}

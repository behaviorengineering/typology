package discover_test

import (
	"path/filepath"
	"testing"

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
}

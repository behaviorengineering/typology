package emit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
	"github.com/behaviorengineering/typology/internal/emit"
)

func TestEmit_docs(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	src := filepath.Join("..", "..", "testdata", "tiny-module")
	typ, err := catalog.LoadYAML(filepath.Join(src, "architecture", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := emit.Run(emit.Options{RepoRoot: repo, Catalog: typ, GoOnly: true}); err != nil {
		t.Fatal(err)
	}
	if err := emit.Run(emit.Options{RepoRoot: repo, Catalog: typ, DocsOnly: true}); err != nil {
		t.Fatal(err)
	}
	overview := filepath.Join(repo, "docs/develop/billing/overview.md")
	data, err := os.ReadFile(overview)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "typology:generated") {
		t.Fatalf("missing generated marker: %s", data)
	}
}

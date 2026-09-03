package remediate_test

import (
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
	"github.com/behaviorengineering/typology/internal/remediate"
)

func TestRemediate_protocol(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	typ, err := catalog.LoadYAML(filepath.Join(repo, ".typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := remediate.Run(remediate.Options{
		RepoRoot: repo,
		Catalog:  typ,
		SliceID:  "billing",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Protocol) == 0 {
		t.Fatal("expected protocol steps")
	}
}

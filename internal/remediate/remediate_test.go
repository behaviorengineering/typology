package remediate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
	terrors "github.com/behaviorengineering/typology/errors"
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

func TestRemediate_errors(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	typ, err := catalog.LoadYAML(filepath.Join(repo, ".typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = remediate.Run(remediate.Options{RepoRoot: repo, Catalog: typ, SliceID: ""})
	if err == nil {
		t.Fatal("empty slice: expected error")
	}
	code, ok := terrors.CodeOf(err)
	if !ok || code != terrors.CodeInvalid {
		t.Fatalf("empty slice code=%v ok=%v err=%v", code, ok, err)
	}
	_, err = remediate.Run(remediate.Options{RepoRoot: repo, Catalog: typ, SliceID: "no-such-slice"})
	if err == nil {
		t.Fatal("unknown slice: expected error")
	}
	code, ok = terrors.CodeOf(err)
	if !ok || code != terrors.CodeNotFound {
		t.Fatalf("unknown slice code=%v ok=%v err=%v", code, ok, err)
	}
}

func TestRemediate_scopesViolationsToSlice(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/scope\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(repo, "billing", "billing.go"), "package billing\n")
	mustWrite(t, filepath.Join(repo, "ledger", "ledger.go"), "package ledger\n")
	mustWrite(t, filepath.Join(repo, "docs", "develop", "billing", "overview.md"), "# billing\n")
	mustWrite(t, filepath.Join(repo, "docs", "develop", "billing", "components.md"), "# components\n")
	// ledger overview intentionally missing so only ledger has a doc violation.
	typ := catalog.Typology{
		ID: "scope",
		Slices: []catalog.Slice{
			{
				ID:        "billing",
				Objective: "Billing slice.",
				Owns: []catalog.Component{{
					ID: "billing-core", Path: "billing", Layer: catalog.LayerDomain,
				}},
				Docs: catalog.DocCluster{Pages: []catalog.DocPage{
					{Kind: catalog.DocOverview, Path: "docs/develop/billing/overview.md"},
					{Kind: catalog.DocComponents, Path: "docs/develop/billing/components.md"},
				}},
			},
			{
				ID:        "ledger",
				Objective: "Ledger slice.",
				Owns: []catalog.Component{{
					ID: "ledger-core", Path: "ledger", Layer: catalog.LayerDomain,
				}},
				Docs: catalog.DocCluster{Pages: []catalog.DocPage{
					{Kind: catalog.DocOverview, Path: "docs/develop/ledger/overview.md"},
				}},
			},
		},
	}
	report, err := remediate.Run(remediate.Options{
		RepoRoot: repo,
		Catalog:  typ,
		SliceID:  "billing",
		Module:   ".",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range report.Violations {
		if v.Slice != "" && v.Slice != "billing" {
			t.Fatalf("billing remediate leaked other-slice violation: %+v", v)
		}
	}
	ledger, err := remediate.Run(remediate.Options{
		RepoRoot: repo,
		Catalog:  typ,
		SliceID:  "ledger",
		Module:   ".",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Violations) == 0 {
		t.Fatal("expected ledger doc violations")
	}
	for _, v := range ledger.Violations {
		if v.Slice != "" && v.Slice != "ledger" {
			t.Fatalf("ledger remediate leaked other-slice violation: %+v", v)
		}
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

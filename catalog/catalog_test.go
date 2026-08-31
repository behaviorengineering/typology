package catalog_test

import (
	"testing"

	"github.com/behaviorengineering/typology/catalog"
)

func TestValidateStructure_ok(t *testing.T) {
	t.Parallel()
	typ := catalog.Typology{
		ID: "tiny",
		Slices: []catalog.Slice{
			{
				ID: "billing",
				Owns: []catalog.Component{
					{ID: "billing-store", Path: "internal/billing/store", Layer: catalog.LayerDomain},
					{ID: "billing-http", Path: "internal/billing/httpapi", Layer: catalog.LayerInteraction, Kind: catalog.InteractionAPI},
				},
				Jobs: []catalog.Job{{ID: "sync", OwnerComponent: "billing-store", Gate: catalog.GateAuto}},
			},
		},
		SliceBindings: []catalog.SliceBinding{
			{From: "billing", To: "ledger", Kind: catalog.SliceReads},
		},
	}
	// missing ledger slice should fail
	if len(typ.ValidateStructure()) == 0 {
		t.Fatal("expected structure issues")
	}
	typ.Slices = append(typ.Slices, catalog.Slice{
		ID: "ledger",
		Owns: []catalog.Component{
			{ID: "ledger-core", Path: "internal/ledger", Layer: catalog.LayerDomain},
		},
	})
	if issues := typ.ValidateStructure(); len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
}

func TestDefaultDocCluster(t *testing.T) {
	t.Parallel()
	c := catalog.DefaultDocCluster("billing", catalog.DefaultDocsRoot)
	if len(c.Pages) != 6 {
		t.Fatalf("want 6 pages, got %d", len(c.Pages))
	}
	if c.Pages[0].Kind != catalog.DocOverview {
		t.Fatalf("first kind = %q", c.Pages[0].Kind)
	}
}

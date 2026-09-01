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
				OpRuns: []catalog.OpRun{{ID: "sync", OwnerComponent: "billing-store", Gate: catalog.GateAuto}},
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

func TestValidateStructure_subprogramAndActuator(t *testing.T) {
	t.Parallel()
	ok := catalog.Typology{
		ID: "tiny",
		Slices: []catalog.Slice{
			{
				ID: "billing",
				Owns: []catalog.Component{
					{ID: "billing-store", Path: "internal/billing/store", Layer: catalog.LayerDomain},
					{ID: "billing-http", Path: "internal/billing/httpapi", Layer: catalog.LayerInteraction, Kind: catalog.InteractionAPI},
				},
				Subprograms: []catalog.Subprogram{{
					ID:             "invoice",
					OwnerComponent: "billing-store",
					Input:          "invoice request",
					Output:         "invoice record",
					Store:          []string{"internal/billing/store"},
					Gate:           catalog.GateAuto,
				}},
				Actuators: []catalog.Actuator{{
					ID:             "invoice-webhook",
					OwnerComponent: "billing-http",
					Signals:        []string{"invoice.minted"},
					Emits:          []string{"webhook"},
					Gate:           catalog.GateAuto,
				}},
				OpRuns: []catalog.OpRun{
					{ID: "mint-invoice", OwnerComponent: "billing-store", Gate: catalog.GateAuto, Runs: "invoice"},
					{ID: "push-invoice", OwnerComponent: "billing-http", Gate: catalog.GateAuto, Actuates: "invoice-webhook"},
				},
			},
		},
	}
	if issues := ok.ValidateStructure(); len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}

	both := ok
	both.Slices[0].OpRuns = []catalog.OpRun{{
		ID: "mixed", OwnerComponent: "billing-store", Runs: "invoice", Actuates: "invoice-webhook",
	}}
	if !hasIssue(both.ValidateStructure(), `opRun "mixed": runs and actuates are mutually exclusive`) {
		t.Fatalf("expected mutually exclusive issue, got %v", both.ValidateStructure())
	}

	unknownRun := ok
	unknownRun.Slices[0].OpRuns = []catalog.OpRun{{
		ID: "sync", OwnerComponent: "billing-store", Runs: "missing",
	}}
	if !hasIssue(unknownRun.ValidateStructure(), `opRun "sync": runs "missing" is not a subprogram on this slice`) {
		t.Fatalf("expected unknown runs issue, got %v", unknownRun.ValidateStructure())
	}

	noSignals := ok
	noSignals.Slices[0].Actuators = []catalog.Actuator{{
		ID:             "invoice-webhook",
		OwnerComponent: "billing-http",
		Emits:          []string{"webhook"},
	}}
	if !hasIssue(noSignals.ValidateStructure(), `actuator "invoice-webhook": missing signals`) {
		t.Fatalf("expected missing signals issue, got %v", noSignals.ValidateStructure())
	}

	collision := ok
	collision.Slices[0].Actuators = []catalog.Actuator{{
		ID:             "invoice",
		OwnerComponent: "billing-http",
		Signals:        []string{"invoice.minted"},
		Emits:          []string{"webhook"},
	}}
	if !hasIssue(collision.ValidateStructure(), `actuator "invoice": id already used by a subprogram`) {
		t.Fatalf("expected id collision issue, got %v", collision.ValidateStructure())
	}
}

func hasIssue(issues []catalog.Issue, want string) bool {
	for _, iss := range issues {
		if iss.Message == want {
			return true
		}
	}
	return false
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

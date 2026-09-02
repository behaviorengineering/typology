package catalog_test

import (
	"testing"

	"github.com/behaviorengineering/typology/catalog"
)

func billingFixtureSlice() catalog.Slice {
	return catalog.Slice{
		ID: "billing",
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
		Subprograms: []catalog.Subprogram{{
			ID:             "invoice",
			OwnerComponent: "billing-store",
			Objective:      "Mint an invoice record from a store request.",
			Input:          "invoice request",
			Output:         "invoice record",
			Store:          []string{"internal/billing/store"},
			Gate:           catalog.GateAuto,
		}},
		Actuators: []catalog.Actuator{{
			ID:             "invoice-webhook",
			OwnerComponent: "billing-http",
			Objective:      "Notify external systems when an invoice is minted.",
			Signals:        []string{"invoice.minted"},
			Emits:          []string{"webhook"},
			Gate:           catalog.GateAuto,
		}},
		OpRuns: []catalog.OpRun{
			{ID: "mint-invoice", OwnerComponent: "billing-store", Gate: catalog.GateAuto, Runs: "invoice"},
			{ID: "push-invoice", OwnerComponent: "billing-http", Gate: catalog.GateAuto, Actuates: "invoice-webhook"},
		},
	}
}

func TestValidateStructure_ok(t *testing.T) {
	t.Parallel()
	typ := catalog.Typology{
		ID: "tiny",
		Slices: []catalog.Slice{
			billingFixtureSlice(),
		},
		SliceBindings: []catalog.SliceBinding{
			{From: "billing", To: "ledger", Kind: catalog.SliceReads},
		},
	}
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
		ID:     "tiny",
		Slices: []catalog.Slice{billingFixtureSlice()},
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

	noObjective := ok
	noObjective.Slices[0].Subprograms = []catalog.Subprogram{{
		ID:             "invoice",
		OwnerComponent: "billing-store",
		Input:          "invoice request",
		Output:         "invoice record",
	}}
	if !hasIssue(noObjective.ValidateStructure(), `subprogram "invoice": missing objective`) {
		t.Fatalf("expected missing subprogram objective, got %v", noObjective.ValidateStructure())
	}

	noActObjective := ok
	noActObjective.Slices[0].Actuators = []catalog.Actuator{{
		ID:             "invoice-webhook",
		OwnerComponent: "billing-http",
		Signals:        []string{"invoice.minted"},
		Emits:          []string{"webhook"},
	}}
	if !hasIssue(noActObjective.ValidateStructure(), `actuator "invoice-webhook": missing objective`) {
		t.Fatalf("expected missing actuator objective, got %v", noActObjective.ValidateStructure())
	}
}

func TestValidateStructure_rejectsInteractionInOwns(t *testing.T) {
	t.Parallel()
	s := billingFixtureSlice()
	s.Owns = append(s.Owns, catalog.Component{
		ID: "billing-http", Path: "internal/billing/httpapi", Layer: catalog.LayerInteraction, Kind: catalog.InteractionAPI,
	})
	s.Surfaces = nil
	typ := catalog.Typology{ID: "tiny", Slices: []catalog.Slice{s}}
	if !hasIssue(typ.ValidateStructure(), `component "billing-http": interaction packages belong on surfaces, not owns`) {
		t.Fatalf("expected interaction-in-owns issue, got %v", typ.ValidateStructure())
	}
}

func TestSliceAllComponents(t *testing.T) {
	t.Parallel()
	s := billingFixtureSlice()
	comps := s.AllComponents()
	if len(comps) != 2 {
		t.Fatalf("want 2 components, got %d", len(comps))
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
	s := billingFixtureSlice()
	c := catalog.DefaultDocCluster(s, catalog.DefaultDocsRoot)
	want := []catalog.DocPageKind{
		catalog.DocOverview,
		catalog.DocComponents,
		catalog.DocContracts,
		catalog.DocPipelines,
	}
	if len(c.Pages) != len(want) {
		t.Fatalf("want %d pages, got %d: %+v", len(want), len(c.Pages), c.Pages)
	}
	for i, kind := range want {
		if c.Pages[i].Kind != kind {
			t.Fatalf("page %d kind = %q, want %q", i, c.Pages[i].Kind, kind)
		}
	}
	if c.Pages[0].Path != "docs/develop/billing/overview.md" {
		t.Fatalf("overview path = %q", c.Pages[0].Path)
	}
}

func TestDefaultDocCluster_domainOnly(t *testing.T) {
	t.Parallel()
	s := catalog.Slice{ID: "ledger", Owns: []catalog.Component{{ID: "ledger-core", Path: "internal/ledger", Layer: catalog.LayerDomain}}}
	c := catalog.DefaultDocCluster(s, catalog.DefaultDocsRoot)
	if len(c.Pages) != 2 {
		t.Fatalf("want overview+components, got %d: %+v", len(c.Pages), c.Pages)
	}
}

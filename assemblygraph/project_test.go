package assemblygraph_test

import (
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/typology/assemblygraph"
	"github.com/behaviorengineering/typology/catalog"
)

func TestProject_sliceKeepsOwnedAndBoundaryStubs(t *testing.T) {
	t.Parallel()
	typ := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{
			{
				ID: "billing",
				Owns: []catalog.Component{
					{ID: "billing-store", Path: "internal/billing/store"},
				},
				Surfaces: []catalog.Surface{
					{
						ID:   "billing-api",
						Kind: catalog.InteractionAPI,
						Components: []catalog.Component{
							{ID: "billing-http", Path: "internal/billing/httpapi"},
						},
					},
				},
			},
			{
				ID: "ledger",
				Owns: []catalog.Component{
					{ID: "ledger-core", Path: "internal/ledger"},
				},
			},
		},
		Libraries: []catalog.Library{
			{
				ID: "config",
				Owns: []catalog.Component{
					{ID: "config", Path: "internal/config"},
				},
			},
		},
		SliceBindings: []catalog.SliceBinding{
			{From: "billing", To: "ledger", Kind: catalog.SliceReads},
		},
		ComponentBindings: []catalog.ComponentBinding{
			{From: "billing-store", To: "ledger-core", Rule: catalog.BindingReads},
		},
	}

	raw := assemblygraph.Graph{
		Nodes: []assemblygraph.Node{
			node("internal/billing/store", "aggregator"),
			node("internal/billing/httpapi", "http_surface"),
			node("internal/ledger", "dto"),
			node("internal/config", "config"),
			node("internal/orphan", "unknown"),
			node("cmd/demo", "entrypoint"),
			node("internal/other", "aggregator"),
		},
		Edges: []assemblygraph.Edge{
			edge("internal/billing/httpapi", "internal/billing/store", "imports"),
			edge("internal/billing/store", "internal/ledger", "fills_dto"),
			edge("internal/billing/store", "internal/config", "reads_config"),
			edge("internal/billing/store", "internal/orphan", "imports"),
			edge("cmd/demo", "internal/billing/httpapi", "imports"),
			edge("internal/other", "internal/ledger", "imports"), // unrelated: drop
		},
	}

	got, stats, err := assemblygraph.Project(raw, typ, assemblygraph.ProjectOptions{SliceID: "billing"})
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if got.Slice != "billing" {
		t.Fatalf("slice=%q", got.Slice)
	}
	if stats.OwnedNodes != 2 {
		t.Fatalf("owned nodes=%d want 2 (store+httpapi); stats=%+v", stats.OwnedNodes, stats)
	}
	// ledger (declared), config (missing slice binding), orphan (unowned), cmd/demo (inbound unowned)
	if stats.BoundaryNodes != 4 {
		t.Fatalf("boundary nodes=%d want 4; stats=%+v nodes=%v", stats.BoundaryNodes, stats, pathsOf(got))
	}
	if stats.InternalEdges != 1 {
		t.Fatalf("internal edges=%d want 1", stats.InternalEdges)
	}
	if stats.BoundaryEdges != 4 {
		t.Fatalf("boundary edges=%d want 4", stats.BoundaryEdges)
	}

	byPath := map[string]assemblygraph.Node{}
	for _, n := range got.Nodes {
		byPath[n.Path] = n
	}
	if _, ok := byPath["internal/other"]; ok {
		t.Fatal("unrelated other package must not appear")
	}
	ledger := byPath["internal/ledger"]
	if !ledger.IsBoundary || ledger.BoundaryKind != assemblygraph.BoundarySlice || ledger.OwnerID != "ledger" {
		t.Fatalf("ledger stub: %+v", ledger)
	}
	cfg := byPath["internal/config"]
	if !cfg.IsBoundary || cfg.BoundaryKind != assemblygraph.BoundaryLibrary || cfg.OwnerID != "config" {
		t.Fatalf("config stub: %+v", cfg)
	}
	orphan := byPath["internal/orphan"]
	if !orphan.IsBoundary || orphan.BoundaryKind != assemblygraph.BoundaryUnowned {
		t.Fatalf("orphan stub: %+v", orphan)
	}
	demo := byPath["cmd/demo"]
	if !demo.IsBoundary || demo.BoundaryKind != assemblygraph.BoundaryUnowned {
		t.Fatalf("inbound stub: %+v", demo)
	}

	var ledgerEdge, configEdge, orphanEdge, inboundEdge *assemblygraph.Edge
	for i := range got.Edges {
		e := &got.Edges[i]
		switch {
		case e.Target == assemblygraph.NormalizeID("internal/ledger"):
			ledgerEdge = e
		case e.Target == assemblygraph.NormalizeID("internal/config"):
			configEdge = e
		case e.Target == assemblygraph.NormalizeID("internal/orphan"):
			orphanEdge = e
		case e.Source == assemblygraph.NormalizeID("cmd/demo"):
			inboundEdge = e
		}
	}
	if ledgerEdge == nil || ledgerEdge.BindingStatus != assemblygraph.BindingDeclared {
		t.Fatalf("ledger edge binding: %+v", ledgerEdge)
	}
	if configEdge == nil || configEdge.BindingStatus != assemblygraph.BindingMissing {
		// billing -> config has no SliceBinding in this fixture
		t.Fatalf("config edge binding: %+v", configEdge)
	}
	if orphanEdge == nil || orphanEdge.BindingStatus != assemblygraph.BindingMissing {
		t.Fatalf("orphan edge binding: %+v", orphanEdge)
	}
	if inboundEdge == nil || inboundEdge.BindingStatus != assemblygraph.BindingMissing {
		t.Fatalf("inbound edge binding: %+v", inboundEdge)
	}
	if stats.MissingBindings != 3 {
		t.Fatalf("missing bindings=%d want 3", stats.MissingBindings)
	}
}

func TestProject_missingSlice(t *testing.T) {
	t.Parallel()
	_, _, err := assemblygraph.Project(assemblygraph.Graph{}, catalog.Typology{}, assemblygraph.ProjectOptions{SliceID: "nope"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProject_noOwnedPathsInGraph(t *testing.T) {
	t.Parallel()
	typ := catalog.Typology{
		Slices: []catalog.Slice{{
			ID:   "billing",
			Owns: []catalog.Component{{ID: "billing-store", Path: "internal/billing/store"}},
		}},
	}
	raw := assemblygraph.Graph{
		Nodes: []assemblygraph.Node{node("internal/other", "unknown")},
	}
	_, _, err := assemblygraph.Project(raw, typ, assemblygraph.ProjectOptions{SliceID: "billing"})
	if err == nil {
		t.Fatal("expected error when no owned paths appear")
	}
}

func TestProject_tinyModuleBilling(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", "testdata", "tiny-module"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := assemblygraph.Build(assemblygraph.BuildOptions{RepoRoot: repo})
	if err != nil {
		t.Fatal(err)
	}
	typ, err := catalog.LoadYAML(filepath.Join(repo, ".typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got, stats, err := assemblygraph.Project(raw, typ, assemblygraph.ProjectOptions{SliceID: "billing"})
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if stats.OwnedNodes < 1 {
		t.Fatalf("expected owned nodes, got %+v", stats)
	}
	for _, n := range got.Nodes {
		if n.IsBoundary {
			continue
		}
		if n.Path != "internal/billing/store" && n.Path != "internal/billing/httpapi" {
			t.Fatalf("unexpected owned node %q", n.Path)
		}
	}
}

func node(path, role string) assemblygraph.Node {
	return assemblygraph.Node{
		ID:   assemblygraph.NormalizeID(path),
		Path: path,
		Role: role,
	}
}

func edge(from, to, kind string) assemblygraph.Edge {
	return assemblygraph.Edge{
		ID:       assemblygraph.NormalizeID(from) + "__" + assemblygraph.NormalizeID(to),
		Source:   assemblygraph.NormalizeID(from),
		Target:   assemblygraph.NormalizeID(to),
		RoleKind: kind,
	}
}

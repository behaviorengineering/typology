package sourceindex_test

import (
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestBuildMechanicalGrouping(t *testing.T) {
	t.Parallel()
	topo := sourceindex.RoleTopology{
		Packages: []sourceindex.RoleNode{
			{Path: "cmd/demo", Role: sourceindex.RoleEntrypoint},
			{Path: "internal/server", Role: sourceindex.RoleServer, Evidence: []string{"delivery:ui"}},
			{Path: "internal/board", Role: sourceindex.RoleDTO},
			{Path: "internal/agent", Role: sourceindex.RoleAggregator},
			{Path: "internal/analyze", Role: sourceindex.RoleAggregator},
			{Path: "internal/ledger", Role: sourceindex.RoleAggregator},
		},
		Edges: []sourceindex.RoleEdge{
			{From: "internal/agent", To: "internal/ledger", Kind: sourceindex.EdgeImports},
			{From: "internal/analyze", To: "internal/ledger", Kind: sourceindex.EdgeImports},
		},
	}
	g := sourceindex.BuildMechanicalGrouping(topo)
	if len(g.DeliveryPaths) != 1 || g.DeliveryPaths[0] != "internal/server" {
		t.Fatalf("delivery=%v", g.DeliveryPaths)
	}
	if len(g.EntrypointPaths) != 1 || g.EntrypointPaths[0] != "cmd/demo" {
		t.Fatalf("entrypoint=%v", g.EntrypointPaths)
	}
	if got := g.LibraryByRole[sourceindex.RoleDTO]; len(got) != 1 || got[0] != "internal/board" {
		t.Fatalf("dto library=%v", got)
	}
	if len(g.ProductSeeds) != 1 || len(g.ProductSeeds[0].Paths) != 3 {
		t.Fatalf("product seeds=%+v", g.ProductSeeds)
	}
	md := sourceindex.FormatMechanicalGroupingMarkdown(g)
	for _, needle := range []string{"Delivery surfaces", "`internal/server`", "Library candidates", "`dto`", "Product slice seeds"} {
		if !strings.Contains(md, needle) {
			t.Fatalf("markdown missing %q:\n%s", needle, md)
		}
	}
}

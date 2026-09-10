package sourceindex_test

import (
	"testing"

	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestRoleTopologyHelpers(t *testing.T) {
	t.Parallel()
	topo := sourceindex.RoleTopology{
		Packages: []sourceindex.RoleNode{
			{Path: "internal/server", Role: sourceindex.RoleServer, Evidence: []string{"delivery:ui", "embeds_static"}},
			{Path: "internal/board", Role: sourceindex.RoleDTO, Evidence: []string{"json_tags"}},
		},
		Edges: []sourceindex.RoleEdge{
			{From: "internal/server", To: "internal/board", Kind: sourceindex.EdgeFillsDTO},
		},
	}
	if _, ok := topo.Package("internal/server"); !ok {
		t.Fatal("expected package lookup to find server")
	}
	if got := topo.PackagesByRole(sourceindex.RoleServer); len(got) != 1 || got[0].Path != "internal/server" {
		t.Fatalf("unexpected server packages: %+v", got)
	}
	if !topo.HasEdge("internal/server", "internal/board", sourceindex.EdgeFillsDTO) {
		t.Fatal("expected fills_dto edge")
	}
	if !topo.Packages[0].HasEvidence("delivery:ui") {
		t.Fatal("expected delivery:ui evidence helper to match")
	}
}

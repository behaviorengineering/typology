package sourceindex_test

import (
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestBuildMechanicalGroupingUnreachedWithoutDoorEdges(t *testing.T) {
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
	if len(g.ProductSeeds) != 0 {
		t.Fatalf("expected no door-private product seeds, got %+v", g.ProductSeeds)
	}
	for _, p := range []string{"internal/board", "internal/agent", "internal/analyze", "internal/ledger"} {
		if !containsPath(g.UnreachedPaths, p) {
			t.Fatalf("expected unreached %s: %v", p, g.UnreachedPaths)
		}
	}
	md := sourceindex.FormatMechanicalGroupingMarkdown(g)
	for _, needle := range []string{"Delivery surfaces", "Door walks", "Unreached", "Library candidates", "`dto`"} {
		if !strings.Contains(md, needle) {
			t.Fatalf("markdown missing %q:\n%s", needle, md)
		}
	}
}

func TestBuildMechanicalGroupingDoorWalkPrivateShared(t *testing.T) {
	t.Parallel()
	topo := sourceindex.RoleTopology{
		Packages: []sourceindex.RoleNode{
			{Path: "cmd/gitboard", Role: sourceindex.RoleEntrypoint},
			{Path: "internal/server", Role: sourceindex.RoleServer},
			{Path: "internal/board", Role: sourceindex.RoleDTO},
			{Path: "internal/config", Role: sourceindex.RoleConfig},
			{Path: "internal/cliexec", Role: sourceindex.RoleExecRunner},
			{Path: "internal/dashboard", Role: sourceindex.RoleAggregator},
			{Path: "internal/llm", Role: sourceindex.RoleAdapter},
			{Path: "internal/observability", Role: sourceindex.RoleObservability},
			{Path: "internal/orphan", Role: sourceindex.RoleUnknown},
		},
		Edges: []sourceindex.RoleEdge{
			{From: "cmd/gitboard", To: "internal/server", Kind: sourceindex.EdgeServesServer},
			{From: "cmd/gitboard", To: "internal/config", Kind: sourceindex.EdgeReadsConfig},
			{From: "cmd/gitboard", To: "internal/board", Kind: sourceindex.EdgeImports},
			{From: "cmd/gitboard", To: "internal/cliexec", Kind: sourceindex.EdgeUsesRunner},
			{From: "internal/server", To: "internal/dashboard", Kind: sourceindex.EdgeImports},
			{From: "internal/server", To: "internal/board", Kind: sourceindex.EdgeImports},
			{From: "internal/server", To: "internal/config", Kind: sourceindex.EdgeReadsConfig},
			{From: "internal/server", To: "internal/observability", Kind: sourceindex.EdgeImports},
			{From: "internal/dashboard", To: "internal/llm", Kind: sourceindex.EdgeImports},
			{From: "internal/dashboard", To: "internal/board", Kind: sourceindex.EdgeFillsDTO},
		},
	}
	g := sourceindex.BuildMechanicalGrouping(topo)

	if containsPath(g.SharedPaths, "internal/dashboard") {
		t.Fatalf("dashboard must not be shared (CLI must not flood through server): shared=%v", g.SharedPaths)
	}
	for _, p := range []string{"internal/board", "internal/config"} {
		if !containsPath(g.SharedPaths, p) {
			t.Fatalf("expected shared %s: %v", p, g.SharedPaths)
		}
	}

	byDoor := map[string]sourceindex.DoorWalk{}
	for _, w := range g.DoorWalks {
		byDoor[w.DoorPath] = w
	}
	cli := byDoor["cmd/gitboard"]
	srv := byDoor["internal/server"]
	if !containsPath(cli.PrivatePaths, "internal/cliexec") {
		t.Fatalf("cli private=%v", cli.PrivatePaths)
	}
	if containsPath(cli.PrivatePaths, "internal/dashboard") {
		t.Fatalf("cli must not claim dashboard private: %v", cli.PrivatePaths)
	}
	if !containsPath(srv.PrivatePaths, "internal/dashboard") || !containsPath(srv.PrivatePaths, "internal/llm") || !containsPath(srv.PrivatePaths, "internal/observability") {
		t.Fatalf("server private=%v", srv.PrivatePaths)
	}
	if len(cli.CrossDoorWiring) == 0 || !strings.Contains(strings.Join(cli.CrossDoorWiring, " "), "internal/server") {
		t.Fatalf("expected CLI cross-door wiring to server: %v", cli.CrossDoorWiring)
	}
	if !containsPath(g.UnreachedPaths, "internal/orphan") {
		t.Fatalf("orphan unreached=%v", g.UnreachedPaths)
	}
	if len(g.ProductSeeds) != 1 || len(g.ProductSeeds[0].Paths) != 2 {
		t.Fatalf("product seeds=%+v", g.ProductSeeds)
	}
	for _, p := range []string{"internal/dashboard", "internal/llm"} {
		if !containsPath(g.ProductSeeds[0].Paths, p) {
			t.Fatalf("product seed missing %s: %+v", p, g.ProductSeeds[0].Paths)
		}
	}

	md := sourceindex.FormatMechanicalGroupingMarkdown(g)
	for _, needle := range []string{
		"Door walks",
		"Door-private packages",
		"Shared across doors",
		"Unreached",
		"`cmd/gitboard`",
		"`internal/cliexec`",
		"`internal/board`",
		"`internal/orphan`",
		"Product slice seeds",
		"`internal/dashboard`",
	} {
		if !strings.Contains(md, needle) {
			t.Fatalf("markdown missing %q:\n%s", needle, md)
		}
	}
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

package sourceindex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/discover"
	"github.com/behaviorengineering/typology/internal/gorepo"
	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestBuildRoleTopology_tinyModule(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	modules, err := gorepo.ResolveModules(repo, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	idx, err := sourceindex.BuildInModules(repo, modules)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := discover.ImportGraphInModules(repo, modules)
	if err != nil {
		t.Fatal(err)
	}
	topo := sourceindex.BuildRoleTopology(idx, graph)
	byPath := map[string]sourceindex.RoleNode{}
	for _, n := range topo.Packages {
		byPath[n.Path] = n
	}

	board := byPath["internal/board"]
	if board.Role != sourceindex.RoleDTO {
		t.Fatalf("board role=%q want dto (must not use folder name)", board.Role)
	}
	server := byPath["internal/server"]
	if server.Role != sourceindex.RoleHTTPSurface {
		t.Fatalf("server role=%q want http_surface", server.Role)
	}
	runner := byPath["internal/runner"]
	if runner.Role != sourceindex.RoleExecRunner {
		t.Fatalf("runner role=%q want exec_runner (classifier must not special-case basename)", runner.Role)
	}
	kitchen := byPath["internal/kitchen"]
	if kitchen.Role != sourceindex.RoleAggregator {
		t.Fatalf("kitchen role=%q want aggregator evidence=%v", kitchen.Role, kitchen.Evidence)
	}
	cfg := byPath["internal/config"]
	if cfg.Role != sourceindex.RoleConfig {
		t.Fatalf("config role=%q want config evidence=%v", cfg.Role, cfg.Evidence)
	}
	trace := byPath["internal/traceboot"]
	if trace.Role != sourceindex.RoleObservability {
		t.Fatalf("traceboot role=%q want observability evidence=%v", trace.Role, trace.Evidence)
	}
	agent := byPath["internal/agent"]
	if agent.Role != sourceindex.RoleAggregator {
		t.Fatalf("agent role=%q want aggregator (uses runner, no os/exec) evidence=%v", agent.Role, agent.Evidence)
	}
	analyze := byPath["internal/analyze"]
	if analyze.Role != sourceindex.RoleAggregator {
		t.Fatalf("analyze role=%q want aggregator (JSON+funcs is not dto) evidence=%v", analyze.Role, analyze.Evidence)
	}
	if analyze.Role == sourceindex.RoleDTO {
		t.Fatal("analyze must not be dto when it exports funcs")
	}

	// No path-token evidence strings.
	for _, n := range topo.Packages {
		for _, e := range n.Evidence {
			if strings.Contains(e, "dashboard") || strings.Contains(e, "cliexec") || strings.Contains(e, "folder") {
				t.Fatalf("path-name evidence leaked: %q on %s", e, n.Path)
			}
		}
	}
}

func TestWriteEvidenceFiles_tinyModule(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	modules, err := gorepo.ResolveModules(repo, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := discover.ImportGraphInModules(repo, modules)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	contractsOut := filepath.Join(dir, "package_contracts.md")
	rolesOut := filepath.Join(dir, "package_roles.yaml")
	if err := sourceindex.WriteEvidenceFiles(repo, modules, contractsOut, rolesOut, graph); err != nil {
		t.Fatal(err)
	}
	roles, err := os.ReadFile(rolesOut)
	if err != nil {
		t.Fatal(err)
	}
	text := string(roles)
	if !strings.Contains(text, "role: dto") {
		t.Fatalf("expected dto in roles:\n%s", text)
	}
	if !strings.Contains(text, "role: http_surface") {
		t.Fatalf("expected http_surface in roles:\n%s", text)
	}
	if !strings.Contains(text, "role: exec_runner") {
		t.Fatalf("expected exec_runner in roles:\n%s", text)
	}
	if !strings.Contains(text, "role: aggregator") {
		t.Fatalf("expected aggregator in roles:\n%s", text)
	}
	if !strings.Contains(text, "role: observability") {
		t.Fatalf("expected observability in roles:\n%s", text)
	}
	if !strings.Contains(text, "role: config") {
		t.Fatalf("expected config in roles:\n%s", text)
	}
	contracts, err := os.ReadFile(contractsOut)
	if err != nil {
		t.Fatal(err)
	}
	ct := string(contracts)
	if !strings.Contains(ct, "role: dto") {
		t.Fatalf("contracts missing role:\n%s", ct)
	}
	if !strings.Contains(ct, "importsOsExec:") {
		t.Fatalf("contracts missing importsOsExec:\n%s", ct)
	}
}

func TestClassifyPackage_noPathNameHeuristic(t *testing.T) {
	t.Parallel()
	// A package whose folder looks like "dashboard" but code is JSON-only is a dto.
	ev := sourceindex.PackageEvidence{
		Path:          "internal/dashboard",
		Name:          "dashboard",
		JSONTags:      true,
		ExportedDecls: []string{"Row"},
	}
	topo := sourceindex.BuildRoleTopology(sourceindex.Index{
		Packages: map[string]sourceindex.PackageEvidence{"internal/dashboard": ev},
	}, nil)
	if len(topo.Packages) != 1 || topo.Packages[0].Role != sourceindex.RoleDTO {
		t.Fatalf("got %+v; path word dashboard must not force ui", topo.Packages)
	}
}

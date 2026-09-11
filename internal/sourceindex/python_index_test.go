package sourceindex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/evidence"
	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestBuildPython_tinyRoles(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "python_tiny")
	idx, graph, err := sourceindex.BuildPython(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Packages) == 0 {
		t.Fatal("expected python packages")
	}
	topo := sourceindex.BuildRoleTopology(idx, graph)
	byPath := map[string]sourceindex.RoleNode{}
	for _, n := range topo.Packages {
		byPath[n.Path] = n
		if n.Language != sourceindex.LangPython {
			t.Fatalf("%s language=%q want python", n.Path, n.Language)
		}
	}
	board := byPath["src/appboard"]
	if board.Role != sourceindex.RoleDTO {
		t.Fatalf("appboard role=%q want dto evidence=%v", board.Role, board.Evidence)
	}
	server := byPath["src/appserver"]
	if server.Role != sourceindex.RoleHTTPSurface {
		t.Fatalf("appserver role=%q want server evidence=%v", server.Role, server.Evidence)
	}
	foundFill := false
	for _, e := range topo.Edges {
		if e.Kind == sourceindex.EdgeFillsDTO && e.From == "src/appserver" && e.To == "src/appboard" {
			foundFill = true
		}
	}
	if !foundFill {
		t.Fatalf("expected fills_dto edge appserver -> appboard; edges=%v graph=%v", topo.Edges, graph)
	}
}

func TestHarvest_pythonTinyWritesLanguage(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "python_tiny")
	h, err := evidence.Harvest(repo, "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	rolesOut := filepath.Join(dir, "package_roles.yaml")
	if err := evidence.WriteFiles(h, filepath.Join(dir, "package_contracts.md"), rolesOut, filepath.Join(dir, "package_rlm_context.md")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(rolesOut)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "language: python") {
		t.Fatalf("expected language: python in roles:\n%s", text)
	}
	if !strings.Contains(text, "role: dto") {
		t.Fatalf("expected dto:\n%s", text)
	}
	if !strings.Contains(text, "role: server") {
		t.Fatalf("expected server:\n%s", text)
	}
}

func TestBuildRoleTopology_tinyModule_setsLanguageGo(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	h, err := evidence.Harvest(repo, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range h.Topo.Packages {
		if n.Language != sourceindex.LangGo {
			t.Fatalf("%s language=%q want go", n.Path, n.Language)
		}
	}
}

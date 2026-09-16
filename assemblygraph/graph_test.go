package assemblygraph_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/assemblygraph"
	"github.com/behaviorengineering/typology/internal/evidence"
	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestBuild_tinyModule(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", "testdata", "tiny-module"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := assemblygraph.Build(assemblygraph.BuildOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(g.Nodes) == 0 {
		t.Fatal("expected nodes")
	}
	if len(g.Edges) == 0 {
		t.Fatal("expected edges")
	}

	byPath := map[string]assemblygraph.Node{}
	for _, n := range g.Nodes {
		byPath[n.Path] = n
		if n.ID == "" || n.Path == "" {
			t.Fatalf("empty id/path: %+v", n)
		}
		if n.Role == "" {
			t.Fatalf("missing role on %s", n.Path)
		}
	}
	board, ok := byPath["internal/board"]
	if !ok {
		t.Fatalf("missing internal/board; paths=%v", pathsOf(g))
	}
	if board.Role != "dto" {
		t.Fatalf("board role=%q, want dto", board.Role)
	}

	var fillsDTO bool
	for _, e := range g.Edges {
		if e.RoleKind == "fills_dto" {
			fillsDTO = true
			break
		}
	}
	if !fillsDTO {
		t.Fatalf("expected at least one fills_dto edge; edges=%v", g.Edges)
	}
}

func TestWriteJSON_roundTrip(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", "testdata", "tiny-module"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := assemblygraph.Build(assemblygraph.BuildOptions{RepoRoot: repo})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	out := filepath.Join(t.TempDir(), "assembly-graph.json")
	if err := assemblygraph.WriteJSON(out, g); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var got assemblygraph.Graph
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Nodes) != len(g.Nodes) || len(got.Edges) != len(g.Edges) {
		t.Fatalf("round-trip size mismatch: nodes %d/%d edges %d/%d",
			len(got.Nodes), len(g.Nodes), len(got.Edges), len(g.Edges))
	}
}

func TestNormalizeID(t *testing.T) {
	t.Parallel()
	if got := assemblygraph.NormalizeID("internal/board"); got != "internal__board" {
		t.Fatalf("got %q", got)
	}
	if got := assemblygraph.NormalizeID("."); got != "root" {
		t.Fatalf("root got %q", got)
	}
}

func TestFromHarvest_wrongWayAndUnknownLayer(t *testing.T) {
	t.Parallel()
	h := evidence.Result{
		Graph: map[string][]string{
			"internal/board": {"cmd/demo"},
			"internal/misc":  {},
			"cmd/demo":       {},
		},
		Topo: sourceindex.RoleTopology{
			Packages: []sourceindex.RoleNode{
				{Path: "internal/board", Role: sourceindex.RoleDTO, Confidence: 0.9},
				{Path: "cmd/demo", Role: sourceindex.RoleEntrypoint, Confidence: 0.9},
				{Path: "internal/misc", Role: sourceindex.RoleUnknown, Confidence: 0.4},
			},
		},
	}
	g := assemblygraph.FromHarvest(h)
	byPath := map[string]assemblygraph.Node{}
	for _, n := range g.Nodes {
		byPath[n.Path] = n
	}
	misc, ok := byPath["internal/misc"]
	if !ok {
		t.Fatalf("missing internal/misc; %+v", g.Nodes)
	}
	if misc.Layer != 2 {
		t.Fatalf("unknown role layer=%d want 2", misc.Layer)
	}
	var ww *assemblygraph.Edge
	for i := range g.Edges {
		e := &g.Edges[i]
		if e.Source == assemblygraph.NormalizeID("internal/board") &&
			e.Target == assemblygraph.NormalizeID("cmd/demo") {
			ww = e
			break
		}
	}
	if ww == nil {
		t.Fatalf("missing board->demo edge; %+v", g.Edges)
	}
	if !ww.WrongWay {
		t.Fatalf("expected wrongWay edge, got %+v", ww)
	}
	if strings.TrimSpace(ww.WrongWayReason) == "" {
		t.Fatal("expected wrongWayReason")
	}
}

func TestBuild_jsonFieldContract(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", "testdata", "tiny-module"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := assemblygraph.Build(assemblygraph.BuildOptions{RepoRoot: repo})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	nodes, ok := doc["nodes"].([]any)
	if !ok || len(nodes) == 0 {
		t.Fatalf("nodes missing: %v", doc["nodes"])
	}
	edges, ok := doc["edges"].([]any)
	if !ok || len(edges) == 0 {
		t.Fatalf("edges missing: %v", doc["edges"])
	}
	node0, ok := nodes[0].(map[string]any)
	if !ok {
		t.Fatal("node0 not object")
	}
	for _, key := range []string{"id", "path", "inDegree", "outDegree", "imports", "importedBy", "isHub", "isLeaf", "layer"} {
		if _, ok := node0[key]; !ok {
			t.Fatalf("node missing key %q: %v", key, node0)
		}
	}
	edge0, ok := edges[0].(map[string]any)
	if !ok {
		t.Fatal("edge0 not object")
	}
	for _, key := range []string{"id", "source", "target"} {
		if _, ok := edge0[key]; !ok {
			t.Fatalf("edge missing key %q: %v", key, edge0)
		}
	}
}

func TestBuild_moduleScope_excludesSibling(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.work"), "go 1.27\n\nuse (\n\t./engine\n\t./lib\n)\n")
	mustWrite(t, filepath.Join(root, "engine", "go.mod"), "module example.com/ws/engine\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(root, "engine", "util", "util.go"), "package util\n\nfunc Help() {}\n")
	mustWrite(t, filepath.Join(root, "engine", "svc", "svc.go"), "package svc\n\nimport \"example.com/ws/engine/util\"\n\nfunc Run() { util.Help() }\n")
	mustWrite(t, filepath.Join(root, "lib", "go.mod"), "module example.com/ws/lib\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(root, "lib", "widget", "widget.go"), "package widget\n\nvar New = 1\n")
	mustWrite(t, filepath.Join(root, "lib", "other", "other.go"), "package other\n\nimport \"example.com/ws/lib/widget\"\n\nfunc Use() { _ = widget.New }\n")

	g, err := assemblygraph.Build(assemblygraph.BuildOptions{RepoRoot: root, Module: "engine"})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range g.Nodes {
		if strings.Contains(n.Path, "widget") || strings.Contains(n.Path, "lib/") {
			t.Fatalf("module=engine should exclude lib, got %q", n.Path)
		}
	}
	found := false
	for _, n := range g.Nodes {
		if strings.Contains(n.Path, "svc") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected engine svc node, got %v", pathsOf(g))
	}
}

func pathsOf(g assemblygraph.Graph) []string {
	out := make([]string, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		out = append(out, n.Path)
	}
	return out
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

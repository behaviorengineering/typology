package sourceindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/gorepo"
)

func TestFormatPackageRLMContext_tinyModule(t *testing.T) {
	repo := filepath.Join("..", "..", "testdata", "tiny-module")
	if _, err := os.Stat(repo); err != nil {
		t.Skip("tiny-module fixture missing")
	}
	modules, err := gorepo.ResolveModules(repo, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	idx, err := BuildInModules(repo, modules)
	if err != nil {
		t.Fatal(err)
	}
	topo := BuildRoleTopology(idx, map[string][]string{})
	md := FormatPackageRLMContextMarkdown(idx, topo)
	if !strings.Contains(md, "mechanicalRole:") && !strings.Contains(md, "exportedDecls:") {
		t.Fatalf("expected role or exports in RLM context, got:\n%s", md[:min(400, len(md))])
	}
	board := FormatPackageRLMContextForPath(idx, topo, "internal/board")
	if board == "" {
		t.Fatal("board context empty")
	}
	if !strings.Contains(board, "jsonTags: true") {
		t.Fatalf("board should show jsonTags: %s", board)
	}
	if strings.Contains(board, "Exported bodies") && strings.Contains(board, "func Collect") {
		t.Fatal("board DTO should not look like aggregator Collect")
	}
	server := FormatPackageRLMContextForPath(idx, topo, "internal/server")
	if server == "" {
		t.Fatal("server context empty")
	}
	if !strings.Contains(server, "goEmbed: true") && !strings.Contains(server, "importsNetHTTP: true") {
		t.Fatalf("server should show delivery flags: %s", server)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

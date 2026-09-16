package roles_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/roles"
)

func TestBuildGroupingFromYAML(t *testing.T) {
	t.Parallel()
	raw := []byte(`packages:
  - path: cmd/demo
    role: entrypoint
  - path: internal/server
    role: server
  - path: internal/board
    role: dto
  - path: internal/agent
    role: aggregator
  - path: internal/ledger
    role: aggregator
edges:
  - from: cmd/demo
    to: internal/server
    kind: serves_server
  - from: cmd/demo
    to: internal/board
    kind: imports
  - from: internal/server
    to: internal/agent
    kind: imports
  - from: internal/server
    to: internal/board
    kind: imports
  - from: internal/agent
    to: internal/ledger
    kind: imports
`)
	g, err := roles.BuildGroupingFromYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	md := roles.FormatGroupingMarkdown(g)
	for _, needle := range []string{
		"`cmd/demo`",
		"`internal/server`",
		"Door-private packages",
		"Shared across doors",
		"`internal/board`",
		"`internal/agent`",
		"`internal/ledger`",
		"`dto`",
	} {
		if !strings.Contains(md, needle) {
			t.Fatalf("missing %q:\n%s", needle, md)
		}
	}
	if strings.Contains(md, "- `cmd/demo`:") && strings.Contains(md, "`internal/agent`") {
		// agent must be server-private, not listed under cmd private line with flood
		for _, line := range strings.Split(md, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "- `cmd/demo`:") && strings.Contains(line, "internal/agent") {
				t.Fatalf("CLI door must not claim server-private agent:\n%s", md)
			}
		}
	}
}

func TestParseYAML_invalid(t *testing.T) {
	t.Parallel()
	_, err := roles.ParseYAML([]byte("packages: [\n"))
	if err == nil {
		t.Fatal("expected parse error")
	}
	_, err = roles.BuildGroupingFromYAML([]byte("packages: [\n"))
	if err == nil {
		t.Fatal("expected grouping parse error")
	}
}

func TestFormatPackageRLMContextForPath_board(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", "testdata", "tiny-module"))
	if err != nil {
		t.Fatal(err)
	}
	md, err := roles.FormatPackageRLMContextForPath(repo, "internal/board", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(md) == "" {
		t.Fatal("expected non-empty RLM context for internal/board")
	}
	if !strings.Contains(md, "board") && !strings.Contains(md, "internal/board") {
		t.Fatalf("context should mention board:\n%s", md)
	}
}

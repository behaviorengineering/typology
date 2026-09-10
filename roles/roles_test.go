package roles_test

import (
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
  - from: internal/agent
    to: internal/ledger
    kind: imports
`)
	g, err := roles.BuildGroupingFromYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	md := roles.FormatGroupingMarkdown(g)
	for _, needle := range []string{"`cmd/demo`", "`internal/server`", "`dto`", "`internal/agent`", "`internal/ledger`"} {
		if !strings.Contains(md, needle) {
			t.Fatalf("missing %q:\n%s", needle, md)
		}
	}
}

package emit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
	"github.com/behaviorengineering/typology/internal/emit"
)

func TestEmit_docs(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	src := filepath.Join("..", "..", "testdata", "tiny-module")
	typ, err := catalog.LoadYAML(filepath.Join(src, "architecture", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := emit.Run(emit.Options{RepoRoot: repo, Catalog: typ, GoOnly: true}); err != nil {
		t.Fatal(err)
	}
	if err := emit.Run(emit.Options{RepoRoot: repo, Catalog: typ, DocsOnly: true}); err != nil {
		t.Fatal(err)
	}
	overview := filepath.Join(repo, "docs/develop/billing/overview.md")
	data, err := os.ReadFile(overview)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "typology:generated") {
		t.Fatalf("missing generated marker: %s", data)
	}
	components := filepath.Join(repo, "docs/develop/billing/components.md")
	compData, err := os.ReadFile(components)
	if err != nil {
		t.Fatal(err)
	}
	body := string(compData)
	if !strings.Contains(body, "## Owns") || !strings.Contains(body, "## Surfaces") {
		t.Fatalf("missing Owns/Surfaces: %s", body)
	}
	if !strings.Contains(body, "## Subprograms") || !strings.Contains(body, "invoice") {
		t.Fatalf("missing subprograms table: %s", body)
	}
	if !strings.Contains(body, "## Actuators") || !strings.Contains(body, "invoice-webhook") {
		t.Fatalf("missing actuators table: %s", body)
	}
	readme := filepath.Join(repo, "docs/develop/billing/README.md")
	hub, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hub), "Surfaces") || !strings.Contains(string(hub), "subprograms/invoice.md") {
		t.Fatalf("missing tree hub: %s", hub)
	}
	invoice := filepath.Join(repo, "docs/develop/billing/subprograms/invoice.md")
	invBody, err := os.ReadFile(invoice)
	if err != nil {
		t.Fatalf("missing subprogram page: %v", err)
	}
	if !strings.Contains(string(invBody), "Mint an invoice record from a store request.") {
		t.Fatalf("subprogram stub missing objective: %s", invBody)
	}
	actuator := filepath.Join(repo, "docs/develop/billing/actuators/invoice-webhook.md")
	actBody, err := os.ReadFile(actuator)
	if err != nil {
		t.Fatalf("missing actuator page: %v", err)
	}
	if !strings.Contains(string(actBody), "Notify external systems when an invoice is minted.") {
		t.Fatalf("actuator stub missing objective: %s", actBody)
	}
}

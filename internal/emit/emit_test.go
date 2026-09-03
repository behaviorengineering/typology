package emit_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
	"github.com/behaviorengineering/typology/internal/emit"
)

func TestEmit_agentsPointer(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	src := filepath.Join("..", "..", "testdata", "tiny-module")
	typ, err := catalog.LoadYAML(filepath.Join(src, ".typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if err := emit.Run(emit.Options{RepoRoot: repo, Catalog: typ, GoOnly: true}); err != nil {
		t.Fatal(err)
	}
	agentsPath := filepath.Join(repo, "AGENTS.md")
	agents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	if !strings.Contains(string(agents), "typology:generated") || !strings.Contains(string(agents), ".typology/README.md") {
		t.Fatalf("AGENTS.md missing typology pointer: %s", agents)
	}

	// Running emit again should not duplicate the pointer.
	if err := emit.Run(emit.Options{RepoRoot: repo, Catalog: typ, GoOnly: true}); err != nil {
		t.Fatal(err)
	}
	agents2, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(agents2, []byte("typology:generated")) != 1 {
		t.Fatalf("AGENTS.md pointer duplicated: %s", agents2)
	}

	// A pre-existing AGENTS.md without the marker gets the section appended.
	repo2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo2, "AGENTS.md"), []byte("# Custom Agents\n\nLocal notes.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := emit.Run(emit.Options{RepoRoot: repo2, Catalog: typ, GoOnly: true}); err != nil {
		t.Fatal(err)
	}
	agents3, err := os.ReadFile(filepath.Join(repo2, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents3), "Local notes.") {
		t.Fatalf("existing AGENTS.md content lost: %s", agents3)
	}
	if !strings.Contains(string(agents3), ".typology/README.md") {
		t.Fatalf("AGENTS.md pointer not appended: %s", agents3)
	}
}

func TestEmit_docs(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	src := filepath.Join("..", "..", "testdata", "tiny-module")
	typ, err := catalog.LoadYAML(filepath.Join(src, ".typology", "typology.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := emit.Run(emit.Options{RepoRoot: repo, Catalog: typ, GoOnly: true}); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(repo, ".typology", "typology.yaml")
	if _, err := os.Stat(catalogPath); err != nil {
		t.Fatalf("catalog not written to %s: %v", catalogPath, err)
	}
	typologyReadmePath := filepath.Join(repo, ".typology", "README.md")
	typologyReadme, err := os.ReadFile(typologyReadmePath)
	if err != nil {
		t.Fatalf(".typology readme not written: %v", err)
	}
	if !strings.Contains(string(typologyReadme), "typology:generated") ||
		!strings.Contains(string(typologyReadme), "typology-cli") ||
		!strings.Contains(string(typologyReadme), "typology architecture") ||
		!strings.Contains(string(typologyReadme), "scope.modules") ||
		!strings.Contains(string(typologyReadme), "does not widen that scope") {
		t.Fatalf(".typology readme missing agent instructions: %s", typologyReadme)
	}
	toolsPath := filepath.Join(repo, ".typology", "tools.yaml")
	tools, err := os.ReadFile(toolsPath)
	if err != nil {
		t.Fatalf("tools index not written: %v", err)
	}
	toolsBody := string(tools)
	if !strings.Contains(toolsBody, `command: "billing mint-invoice"`) ||
		!strings.Contains(toolsBody, `summary: "Mint an invoice record from a store request."`) ||
		!strings.Contains(toolsBody, `actuates: "invoice-webhook"`) {
		t.Fatalf("tools index missing expected entries: %s", toolsBody)
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

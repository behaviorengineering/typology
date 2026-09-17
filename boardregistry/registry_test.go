package boardregistry_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/boardregistry"
)

func TestUpsertAndMaterialize(t *testing.T) {
	cfg := t.TempDir()
	data := t.TempDir()
	public := t.TempDir()
	t.Setenv("TYPOLOGY_CONFIG_DIR", cfg)
	t.Setenv("TYPOLOGY_DATA_DIR", data)

	paths, err := boardregistry.ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.ConfigDir != cfg || paths.DataDir != data {
		t.Fatalf("paths=%+v", paths)
	}
	wantViewer := filepath.Join(data, "viewer", "public")
	if got := paths.ViewerPublicDir(); got != wantViewer {
		t.Fatalf("ViewerPublicDir=%q want %q", got, wantViewer)
	}

	id := boardregistry.PrefixedID("consilium", "chronology")
	if id != "consilium-chronology" {
		t.Fatalf("id=%q", id)
	}
	label := boardregistry.DefaultLabel("consilium", "chronology", "")
	if label != "Consilium · Chronology" {
		t.Fatalf("label=%q", label)
	}

	graphAbs := paths.GraphAbs(id)
	if err := os.MkdirAll(filepath.Dir(graphAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(graphAbs, []byte(`{"nodes":[],"edges":[],"slice":"chronology"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := boardregistry.LoadYAML(paths.YAMLPath())
	if err != nil {
		t.Fatal(err)
	}
	reg, err = boardregistry.Upsert(reg, boardregistry.UpsertOptions{
		Board: boardregistry.Board{
			ID:     id,
			Label:  label,
			Repo:   "consilium",
			Source: "/tmp/consilium",
			Slice:  "chronology",
			Graph:  boardregistry.GraphRel(id),
		},
		MakeDefault: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := boardregistry.SaveYAML(paths.YAMLPath(), reg); err != nil {
		t.Fatal(err)
	}

	// Same id, other repo must fail.
	_, err = boardregistry.Upsert(reg, boardregistry.UpsertOptions{
		Board: boardregistry.Board{
			ID:    id,
			Label: "Other",
			Repo:  "gitboard",
			Graph: boardregistry.GraphRel(id),
		},
	})
	if err == nil {
		t.Fatal("expected cross-repo refusal")
	}

	// Second prefix ok.
	id2 := boardregistry.PrefixedID("gitboard", "chronology")
	g2 := paths.GraphAbs(id2)
	if err := os.MkdirAll(filepath.Dir(g2), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(g2, []byte(`{"nodes":[],"edges":[]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err = boardregistry.Upsert(reg, boardregistry.UpsertOptions{
		Board: boardregistry.Board{
			ID:    id2,
			Label: boardregistry.DefaultLabel("gitboard", "chronology", ""),
			Repo:  "gitboard",
			Graph: boardregistry.GraphRel(id2),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := boardregistry.SaveYAML(paths.YAMLPath(), reg); err != nil {
		t.Fatal(err)
	}

	if err := boardregistry.Materialize(boardregistry.MaterializeOptions{
		ViewerPublicDir: public,
		Paths:           paths,
		Registry:        reg,
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(public, "boards.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, `"id": "consilium-chronology"`) || !strings.Contains(text, `"repo": "consilium"`) {
		t.Fatalf("manifest missing consilium: %s", text)
	}
	if !strings.Contains(text, `"id": "gitboard-chronology"`) {
		t.Fatalf("manifest missing gitboard: %s", text)
	}
	if _, err := os.Stat(filepath.Join(public, "boards", id, "assembly-graph.json")); err != nil {
		t.Fatal(err)
	}
}

func TestPrefixedIDIdempotent(t *testing.T) {
	t.Parallel()
	if got := boardregistry.PrefixedID("alpha", "alpha-billing"); got != "alpha-billing" {
		t.Fatalf("got %q", got)
	}
}

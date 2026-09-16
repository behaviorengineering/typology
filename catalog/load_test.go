package catalog_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/typology/catalog"
	terrors "github.com/behaviorengineering/typology/errors"
)

func TestSaveLoadYAML_roundTrip(t *testing.T) {
	t.Parallel()
	in := catalog.Typology{
		ID: "demo",
		Slices: []catalog.Slice{{
			ID:        "billing",
			Objective: "Charge customers.",
			Owns: []catalog.Component{{
				ID: "store", Path: "internal/store", Layer: catalog.LayerDomain,
			}},
		}},
		Libraries: []catalog.Library{{
			ID:      "shared",
			Purpose: "Shared helpers.",
			Owns: []catalog.Component{{
				ID: "util", Path: "internal/util", Layer: catalog.LayerDomain,
			}},
		}},
	}
	path := filepath.Join(t.TempDir(), ".typology", "typology.yaml")
	if err := catalog.SaveYAML(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := catalog.LoadYAML(path)
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "demo" || len(out.Slices) != 1 || out.Slices[0].ID != "billing" {
		t.Fatalf("unexpected slices: %+v", out)
	}
	if len(out.Libraries) != 1 || out.Libraries[0].ID != "shared" {
		t.Fatalf("unexpected libraries: %+v", out.Libraries)
	}
	if out.Slices[0].Owns[0].Path != "internal/store" {
		t.Fatalf("owns path lost: %+v", out.Slices[0].Owns)
	}
}

func TestLoadYAML_errors(t *testing.T) {
	t.Parallel()
	_, err := catalog.LoadYAML(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("missing file: expected error")
	}
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("slices: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = catalog.LoadYAML(bad)
	if err == nil {
		t.Fatal("invalid yaml: expected error")
	}
	code, ok := terrors.CodeOf(err)
	if !ok || code != terrors.CodeInvalid {
		t.Fatalf("invalid yaml code=%v ok=%v err=%v", code, ok, err)
	}
}

func TestFindCatalog(t *testing.T) {
	t.Parallel()
	preferred := t.TempDir()
	def := filepath.Join(preferred, ".typology", "typology.yaml")
	fallbackPath := filepath.Join(preferred, "typology.yaml")
	if err := os.MkdirAll(filepath.Dir(def), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(def, []byte("id: preferred\nslices: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fallbackPath, []byte("id: fallback\nslices: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.FindCatalog(preferred)
	if err != nil {
		t.Fatal(err)
	}
	if got != def {
		t.Fatalf("prefer .typology path: got %q want %q", got, def)
	}

	onlyFallback := t.TempDir()
	rootYAML := filepath.Join(onlyFallback, "typology.yaml")
	if err := os.WriteFile(rootYAML, []byte("id: root\nslices: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = catalog.FindCatalog(onlyFallback)
	if err != nil {
		t.Fatal(err)
	}
	if got != rootYAML {
		t.Fatalf("fallback path: got %q want %q", got, rootYAML)
	}

	empty := t.TempDir()
	_, err = catalog.FindCatalog(empty)
	if err == nil {
		t.Fatal("expected not found")
	}
	code, ok := terrors.CodeOf(err)
	if !ok || code != terrors.CodeNotFound {
		t.Fatalf("code=%v ok=%v err=%v", code, ok, err)
	}
}

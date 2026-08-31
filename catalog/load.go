package catalog

import (
	"os"
	"path/filepath"

	terrors "github.com/behaviorengineering/typology/errors"
	"gopkg.in/yaml.v3"
)

// DefaultCatalogRel is the usual catalog path under a repo root.
const DefaultCatalogRel = "architecture/typology.yaml"

// DefaultDocsRoot is the usual per-slice docs prefix under a repo root.
const DefaultDocsRoot = "docs/develop"

// LoadYAML reads a typology from a YAML file.
func LoadYAML(path string) (Typology, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Typology{}, terrors.Wrap(err, terrors.CodeUnavailable, "catalog.LoadYAML", "read catalog").
			With("path", path)
	}
	var t Typology
	if err := yaml.Unmarshal(data, &t); err != nil {
		return Typology{}, terrors.Wrap(err, terrors.CodeInvalid, "catalog.LoadYAML", "parse catalog").
			With("path", path)
	}
	return t, nil
}

// SaveYAML writes a typology to YAML.
func SaveYAML(path string, t Typology) error {
	data, err := yaml.Marshal(&t)
	if err != nil {
		return terrors.Wrap(err, terrors.CodeInternal, "catalog.SaveYAML", "marshal catalog")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "catalog.SaveYAML", "mkdir catalog dir").
			With("path", path)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "catalog.SaveYAML", "write catalog").
			With("path", path)
	}
	return nil
}

// FindCatalog walks repoRoot for architecture/typology.yaml.
func FindCatalog(repoRoot string) (string, error) {
	candidates := []string{
		filepath.Join(repoRoot, filepath.FromSlash(DefaultCatalogRel)),
		filepath.Join(repoRoot, "typology.yaml"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", terrors.New(terrors.CodeNotFound, "catalog.FindCatalog", "no catalog found").
		With("repo", repoRoot).
		With("expected", DefaultCatalogRel)
}

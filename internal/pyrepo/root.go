// Package pyrepo discovers Python project roots for typology harvest.
package pyrepo

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
)

// Root is one Python project root under a repository.
type Root struct {
	// Dir is absolute path to the project root (directory containing pyproject/setup).
	Dir string
	// Rel is repo-relative slash path ("." for repo root).
	Rel string
}

// FindRoots walks repoRoot for Python project markers.
// Markers: pyproject.toml, setup.cfg, setup.py. Nested .git / venv / node_modules skipped.
func FindRoots(repoRoot string) ([]Root, error) {
	repo := strings.TrimSpace(repoRoot)
	if repo == "" {
		return nil, terrors.New(terrors.CodeInvalid, "pyrepo.FindRoots", "repo root empty")
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return nil, terrors.Wrap(err, terrors.CodeInvalid, "pyrepo.FindRoots", "abs repo").
			With("repo", repo)
	}
	if resolved, err := filepath.EvalSymlinks(absRepo); err == nil {
		absRepo = resolved
	}

	seen := map[string]struct{}{}
	var roots []Root
	err = filepath.WalkDir(absRepo, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			switch name {
			case ".git", ".cursor", ".typology", "node_modules", "vendor",
				".venv", "venv", "__pycache__", ".tox", ".mypy_cache", ".pytest_cache":
				if path != absRepo {
					return filepath.SkipDir
				}
			}
			return nil
		}
		switch d.Name() {
		case "pyproject.toml", "setup.cfg", "setup.py":
		default:
			return nil
		}
		dir := filepath.Dir(path)
		if _, ok := seen[dir]; ok {
			return nil
		}
		seen[dir] = struct{}{}
		rel, err := filepath.Rel(absRepo, dir)
		if err != nil {
			return terrors.Wrap(err, terrors.CodeInternal, "pyrepo.FindRoots", "rel root").
				With("dir", dir)
		}
		if rel == "." {
			rel = "."
		}
		roots = append(roots, Root{Dir: dir, Rel: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Rel < roots[j].Rel })
	return roots, nil
}

// HasPythonRoot reports whether FindRoots would return at least one root.
func HasPythonRoot(repoRoot string) (bool, error) {
	roots, err := FindRoots(repoRoot)
	if err != nil {
		return false, err
	}
	return len(roots) > 0, nil
}

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

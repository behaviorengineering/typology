// Package gorepo resolves Go module and workspace roots for Typology.
package gorepo

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
)

// Module describes one go.mod under a repo or workspace root.
type Module struct {
	// Dir is the absolute path to the module directory.
	Dir string
	// Path is the module path from go.mod.
	Path string
	// Rel is the path of the module relative to the workspace/repo root
	// (for example "engine"), or "." when the module is the repo root.
	Rel string
}

// FindRoot walks up from startDir until it finds go.work or go.mod.
// When both exist in the same directory, go.work wins so multi-module
// workspaces resolve as a unit.
func FindRoot(startDir string) (string, error) {
	dir := strings.TrimSpace(startDir)
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", terrors.Wrap(err, terrors.CodeInvalid, "gorepo.FindRoot", "abs start").
			With("dir", startDir)
	}
	for {
		work := filepath.Join(abs, "go.work")
		mod := filepath.Join(abs, "go.mod")
		if fileExists(work) {
			return abs, nil
		}
		if fileExists(mod) {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	return "", terrors.New(terrors.CodeNotFound, "gorepo.FindRoot", "go.work or go.mod not found").
		With("dir", startDir)
}

// Modules returns the go.mod modules covered by repoRoot.
// A root with go.mod alone yields one module. A root with go.work yields
// every use entry that has a go.mod.
func Modules(repoRoot string) ([]Module, error) {
	abs, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return nil, terrors.Wrap(err, terrors.CodeInvalid, "gorepo.Modules", "abs repo").
			With("repo", repoRoot)
	}
	workPath := filepath.Join(abs, "go.work")
	if fileExists(workPath) {
		uses, err := ParseGoWorkUses(workPath)
		if err != nil {
			return nil, terrors.Wrap(err, terrors.CodeInvalid, "gorepo.Modules", "parse go.work").
				With("path", workPath)
		}
		if len(uses) == 0 {
			return nil, terrors.New(terrors.CodeInvalid, "gorepo.Modules", "go.work has no use entries").
				With("path", workPath)
		}
		out := make([]Module, 0, len(uses))
		for _, use := range uses {
			dir := filepath.Join(abs, filepath.FromSlash(use))
			modFile := filepath.Join(dir, "go.mod")
			if !fileExists(modFile) {
				return nil, terrors.New(terrors.CodeNotFound, "gorepo.Modules", "go.work use missing go.mod").
					With("use", use).
					With("path", modFile)
			}
			modPath, err := ReadModulePath(modFile)
			if err != nil {
				return nil, err
			}
			out = append(out, Module{Dir: dir, Path: modPath, Rel: use})
		}
		return out, nil
	}
	if fileExists(filepath.Join(abs, "go.mod")) {
		modPath, err := ReadModulePath(filepath.Join(abs, "go.mod"))
		if err != nil {
			return nil, err
		}
		return []Module{{Dir: abs, Path: modPath, Rel: "."}}, nil
	}
	return nil, terrors.New(terrors.CodeNotFound, "gorepo.Modules", "go.work or go.mod not found").
		With("repo", abs)
}

// ResolveModules selects the modules Typology should inspect. An explicit
// override selects one module; otherwise configured contains the catalog's
// repository-relative scope. An unscoped multi-module workspace fails closed.
func ResolveModules(repoRoot string, configured []string, override string) ([]Module, error) {
	modules, err := Modules(repoRoot)
	if err != nil {
		return nil, err
	}
	selectors := append([]string(nil), configured...)
	if strings.TrimSpace(override) != "" {
		selectors = []string{override}
	}
	if len(selectors) == 0 {
		if len(modules) == 1 {
			return modules, nil
		}
		return nil, terrors.New(terrors.CodeInvalid, "gorepo.ResolveModules", "multiple Go modules found; declare scope.modules or pass --module").
			With("modules", moduleNames(modules))
	}
	if strings.TrimSpace(override) == "" {
		for _, selector := range selectors {
			if err := validateScopeSelector(selector); err != nil {
				return nil, err
			}
		}
	}

	selected := make([]Module, 0, len(selectors))
	seen := map[string]struct{}{}
	for _, raw := range selectors {
		selector := strings.TrimSpace(raw)
		if selector == "" {
			return nil, terrors.New(terrors.CodeInvalid, "gorepo.ResolveModules", "module selector is empty")
		}
		module, ok := findModule(modules, selector)
		if !ok {
			return nil, terrors.New(terrors.CodeNotFound, "gorepo.ResolveModules", "module selector did not match a workspace module").
				With("selector", selector).
				With("modules", moduleNames(modules))
		}
		if _, ok := seen[module.Rel]; ok {
			return nil, terrors.New(terrors.CodeInvalid, "gorepo.ResolveModules", "duplicate module selector").
				With("selector", selector)
		}
		seen[module.Rel] = struct{}{}
		selected = append(selected, module)
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Rel < selected[j].Rel
	})
	return selected, nil
}

func validateScopeSelector(raw string) error {
	selector := strings.TrimSpace(raw)
	normalized := path.Clean(strings.ReplaceAll(selector, `\`, "/"))
	if selector == "" {
		return terrors.New(terrors.CodeInvalid, "gorepo.ResolveModules", "module selector is empty")
	}
	if strings.HasPrefix(normalized, "/") || normalized == ".." || strings.HasPrefix(normalized, "../") || strings.Contains(normalized, ":") {
		return terrors.New(terrors.CodeInvalid, "gorepo.ResolveModules", "catalog module scope must be repository-relative").
			With("selector", raw)
	}
	return nil
}

func findModule(modules []Module, selector string) (Module, bool) {
	normalized := filepath.ToSlash(strings.TrimPrefix(selector, "./"))
	if normalized == "" {
		normalized = "."
	}
	for _, module := range modules {
		if normalized == filepath.ToSlash(module.Rel) ||
			normalized == filepath.ToSlash(module.Path) ||
			filepath.Clean(selector) == filepath.Clean(module.Dir) {
			return module, true
		}
	}
	return Module{}, false
}

func moduleNames(modules []Module) string {
	names := make([]string, 0, len(modules))
	for _, module := range modules {
		names = append(names, fmt.Sprintf("%s (%s)", module.Rel, module.Path))
	}
	return strings.Join(names, ", ")
}

// ReadModulePath returns the module line from a go.mod file.
func ReadModulePath(modFile string) (string, error) {
	data, err := os.ReadFile(modFile)
	if err != nil {
		return "", terrors.Wrap(err, terrors.CodeUnavailable, "gorepo.ReadModulePath", "read go.mod").
			With("path", modFile)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", terrors.New(terrors.CodeInvalid, "gorepo.ReadModulePath", "module line missing").
		With("path", modFile)
}

// ParseGoWorkUses returns relative use paths from a go.work file
// (for example "engine" from "./engine").
func ParseGoWorkUses(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var uses []string
	inUse := false
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if i := strings.Index(trimmed, "//"); i >= 0 {
			trimmed = strings.TrimSpace(trimmed[:i])
		}
		if trimmed == "" {
			continue
		}
		if !inUse {
			if trimmed == "use (" {
				inUse = true
				continue
			}
			if strings.HasPrefix(trimmed, "use ") {
				arg := strings.TrimSpace(strings.TrimPrefix(trimmed, "use "))
				arg = strings.Trim(arg, `"`)
				if arg != "" && arg != "(" {
					uses = append(uses, normalizeWorkUse(arg))
				}
			}
			continue
		}
		if trimmed == ")" {
			inUse = false
			continue
		}
		uses = append(uses, normalizeWorkUse(strings.Trim(trimmed, `"`)))
	}
	return uses, nil
}

func normalizeWorkUse(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "./")
	return filepath.ToSlash(path)
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

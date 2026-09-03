package sourceindex

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/gorepo"
)

// PackageEvidence summarizes static source evidence for one Go package.
type PackageEvidence struct {
	Path          string   `json:"path"`
	Name          string   `json:"name"`
	Files         []string `json:"files,omitempty"`
	ExportedDecls []string `json:"exportedDecls,omitempty"`
	ExportedFuncs []string `json:"exportedFuncs,omitempty"`
	HasMain       bool     `json:"hasMain,omitempty"`
}

// HasStaticAnchor reports whether the package has at least one exported symbol
// or a main entrypoint that can act as a deterministic source anchor.
func (p PackageEvidence) HasStaticAnchor() bool {
	return p.HasMain || len(p.ExportedDecls) > 0 || len(p.ExportedFuncs) > 0
}

// Index is a source evidence map keyed by normalized package path.
type Index struct {
	Packages map[string]PackageEvidence `json:"packages"`
}

// Build scans the repo and indexes local Go packages using go list plus AST.
func Build(repoRoot string) (Index, error) {
	repo := strings.TrimSpace(repoRoot)
	if repo == "" {
		return Index{}, terrors.New(terrors.CodeInvalid, "sourceindex.Build", "repo root empty")
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return Index{}, terrors.Wrap(err, terrors.CodeInvalid, "sourceindex.Build", "abs repo").
			With("repo", repo)
	}
	pkgs, err := listPackages(absRepo)
	if err != nil {
		return Index{}, err
	}
	index := Index{Packages: make(map[string]PackageEvidence, len(pkgs))}
	for _, pkg := range pkgs {
		ev, err := scanPackage(absRepo, pkg)
		if err != nil {
			return Index{}, err
		}
		absDir, err := filepath.Abs(pkg.Dir)
		if err != nil {
			return Index{}, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.Build", "abs package dir").
				With("dir", pkg.Dir)
		}
		rel, err := filepath.Rel(absRepo, absDir)
		if err != nil {
			return Index{}, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.Build", "rel package dir").
				With("dir", absDir)
		}
		ev.Path = normalizePath(rel)
		index.Packages[normalizePath(rel)] = ev
	}
	return index, nil
}

// Package returns the evidence for a normalized or relative package path.
func (i Index) Package(path string) (PackageEvidence, bool) {
	if len(i.Packages) == 0 {
		return PackageEvidence{}, false
	}
	want := normalizePath(path)
	if ev, ok := i.Packages[want]; ok {
		return ev, true
	}
	for key, ev := range i.Packages {
		key = normalizePath(key)
		if key == want || strings.HasSuffix(key, "/"+want) {
			return ev, true
		}
	}
	return PackageEvidence{}, false
}

// HasPackage reports whether the package path exists in the index.
func (i Index) HasPackage(path string) bool {
	_, ok := i.Package(path)
	return ok
}

// AnchoredPackages returns the number of packages with static anchors.
func (i Index) AnchoredPackages() int {
	count := 0
	for _, ev := range i.Packages {
		if ev.HasStaticAnchor() {
			count++
		}
	}
	return count
}

type listPackage struct {
	Dir             string   `json:"Dir"`
	Name            string   `json:"Name"`
	GoFiles         []string `json:"GoFiles"`
	CompiledGoFiles []string `json:"CompiledGoFiles"`
}

func listPackages(repoRoot string) ([]listPackage, error) {
	modules, err := gorepo.Modules(repoRoot)
	if err != nil {
		return nil, err
	}
	var all []listPackage
	for _, mod := range modules {
		pkgs, err := listPackagesInModule(mod.Dir)
		if err != nil {
			return nil, err
		}
		all = append(all, pkgs...)
	}
	return all, nil
}

func listPackagesInModule(moduleRoot string) ([]listPackage, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = moduleRoot
	// Isolate each module from an enclosing workspace so sibling modules stay out of scope.
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.listPackages", "go list failed").
				With("stderr", strings.TrimSpace(string(ee.Stderr))).
				With("dir", moduleRoot)
		}
		return nil, terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.listPackages", "go list").
			With("dir", moduleRoot)
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	var pkgs []listPackage
	for {
		var pkg listPackage
		if err := dec.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			return nil, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.listPackages", "decode go list json").
				With("dir", moduleRoot)
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

func scanPackage(repoRoot string, pkg listPackage) (PackageEvidence, error) {
	absDir, err := filepath.Abs(pkg.Dir)
	if err != nil {
		return PackageEvidence{}, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.scanPackage", "abs package dir").
			With("dir", pkg.Dir)
	}
	files := pkg.GoFiles
	if len(files) == 0 {
		files = pkg.CompiledGoFiles
	}
	ev := PackageEvidence{
		Path:  normalizePath(pkg.Dir),
		Name:  pkg.Name,
		Files: make([]string, 0, len(files)),
	}
	fset := token.NewFileSet()
	exportedDecls := map[string]struct{}{}
	exportedFuncs := map[string]struct{}{}
	for _, file := range files {
		abs := filepath.Join(absDir, file)
		rel, err := filepath.Rel(repoRoot, abs)
		if err != nil {
			return PackageEvidence{}, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.scanPackage", "rel file").
				With("file", abs)
		}
		ev.Files = append(ev.Files, normalizePath(rel))
		parsed, err := parser.ParseFile(fset, abs, nil, 0)
		if err != nil {
			return PackageEvidence{}, terrors.Wrap(err, terrors.CodeInvalid, "sourceindex.scanPackage", "parse file").
				With("file", abs)
		}
		for _, decl := range parsed.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil {
					continue
				}
				if d.Name.Name == "main" && pkg.Name == "main" {
					ev.HasMain = true
				}
				if ast.IsExported(d.Name.Name) {
					exportedFuncs[d.Name.Name] = struct{}{}
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(s.Name.Name) {
							exportedDecls[s.Name.Name] = struct{}{}
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							if ast.IsExported(name.Name) {
								exportedDecls[name.Name] = struct{}{}
							}
						}
					}
				}
			}
		}
	}
	ev.ExportedDecls = sortedKeys(exportedDecls)
	ev.ExportedFuncs = sortedKeys(exportedFuncs)
	return ev, nil
}

func normalizePath(path string) string {
	return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(path), "./"))
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

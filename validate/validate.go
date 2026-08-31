package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/typology/catalog"
	"github.com/behaviorengineering/typology/internal/discover"
)

// Options configures repo validation.
type Options struct {
	RepoRoot    string
	Catalog     catalog.Typology
	SliceID     string
	SkipImports bool
}

// Run validates catalog structure and repo paths/imports.
func Run(opts Options) []catalog.Issue {
	var issues []catalog.Issue
	issues = append(issues, opts.Catalog.ValidateStructure()...)

	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		return issues
	}

	for _, s := range opts.Catalog.Slices {
		if opts.SliceID != "" && s.ID != opts.SliceID {
			continue
		}
		issues = append(issues, checkSlice(repo, s)...)
	}
	if opts.SliceID == "" && !opts.SkipImports {
		issues = append(issues, checkImports(repo, opts.Catalog)...)
	}
	catalog.SortIssues(issues)
	return issues
}

func checkSlice(repoRoot string, s catalog.Slice) []catalog.Issue {
	var issues []catalog.Issue
	for _, c := range s.Owns {
		if strings.TrimSpace(c.Path) == "" {
			issues = append(issues, catalog.Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("component %q: empty path", c.ID),
			})
			continue
		}
		abs := filepath.Join(repoRoot, filepath.FromSlash(c.Path))
		if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
			issues = append(issues, catalog.Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("owned path %q does not exist", c.Path),
			})
		}
	}
	for _, p := range s.Docs.Pages {
		if strings.TrimSpace(p.Path) == "" {
			issues = append(issues, catalog.Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("DocPage %q: empty path", p.Kind),
			})
			continue
		}
		abs := filepath.Join(repoRoot, filepath.FromSlash(p.Path))
		if _, err := os.Stat(abs); err != nil {
			issues = append(issues, catalog.Issue{
				Slice:   s.ID,
				Message: fmt.Sprintf("DocPage %s path missing: %s", p.Kind, p.Path),
			})
		}
	}
	return issues
}

func checkImports(repoRoot string, topo catalog.Typology) []catalog.Issue {
	graph, err := discover.ImportGraph(repoRoot)
	if err != nil {
		return []catalog.Issue{{Message: fmt.Sprintf("import graph: %v", err)}}
	}
	pathToComp := map[string]string{}
	compToSlice := map[string]string{}
	for _, s := range topo.Slices {
		for _, c := range s.Owns {
			if c.Path != "" {
				key := normalizePkgPath(c.Path)
				pathToComp[key] = c.ID
				compToSlice[c.ID] = s.ID
			}
		}
	}

	var issues []catalog.Issue
	for _, b := range topo.ComponentBindings {
		fromPath := normalizePkgPath(compPath(topo, b.From))
		toPath := normalizePkgPath(compPath(topo, b.To))
		if fromPath == "" || toPath == "" {
			continue
		}
		imports := graph[fromPath]
		if imports == nil {
			imports = graph["./"+fromPath]
		}
		has := false
		for _, imp := range imports {
			imp = normalizePkgPath(imp)
			if imp == toPath || strings.HasPrefix(imp, toPath+"/") {
				has = true
				break
			}
		}
		switch b.Rule {
		case catalog.BindingMustNot:
			if has {
				issues = append(issues, catalog.Issue{
					Slice:   compToSlice[b.From],
					Message: fmt.Sprintf("ComponentBinding %s -> %s forbidden (%s) but import exists", b.From, b.To, b.Rule),
				})
			}
		case catalog.BindingMust:
			if !has {
				issues = append(issues, catalog.Issue{
					Slice:   compToSlice[b.From],
					Message: fmt.Sprintf("ComponentBinding %s -> %s required (%s) but import missing", b.From, b.To, b.Rule),
				})
			}
		}
	}

	// Cross-slice imports without SliceBinding.
	for from, imports := range graph {
		from = normalizePkgPath(from)
		fromComp := pathToComp[from]
		if fromComp == "" {
			continue
		}
		fromSlice := compToSlice[fromComp]
		for _, imp := range imports {
			imp = normalizePkgPath(imp)
			toComp := pathToComp[imp]
			if toComp == "" {
				continue
			}
			toSlice := compToSlice[toComp]
			if toSlice == "" || fromSlice == toSlice {
				continue
			}
			if !hasSliceBinding(topo, fromSlice, toSlice) {
				issues = append(issues, catalog.Issue{
					Slice:   fromSlice,
					Message: fmt.Sprintf("SliceBinding %s -> %s missing but cross-slice import exists (%s -> %s)", fromSlice, toSlice, fromComp, toComp),
				})
			}
		}
	}
	return issues
}

func compPath(t catalog.Typology, id string) string {
	for _, c := range t.ComponentByID() {
		if c.ID == id {
			return c.Path
		}
	}
	return ""
}

func hasSliceBinding(t catalog.Typology, from, to string) bool {
	for _, b := range t.SliceBindings {
		if b.From == from && b.To == to {
			return true
		}
	}
	return false
}

func normalizePkgPath(p string) string {
	return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "./"))
}

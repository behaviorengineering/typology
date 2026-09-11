package sourceindex

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/behaviorengineering/typology/internal/pyrepo"
	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/tamnd/gopapy/v2/parser2"
)

// BuildPython scans Python packages under repoRoot and returns an Index plus
// first-party import graph. Parsing uses pure-Go gopapy (no CPython).
func BuildPython(repoRoot string) (Index, map[string][]string, error) {
	repo := strings.TrimSpace(repoRoot)
	if repo == "" {
		return Index{}, nil, terrors.New(terrors.CodeInvalid, "sourceindex.BuildPython", "repo root empty")
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return Index{}, nil, terrors.Wrap(err, terrors.CodeInvalid, "sourceindex.BuildPython", "abs repo").
			With("repo", repo)
	}
	if resolved, err := filepath.EvalSymlinks(absRepo); err == nil {
		absRepo = resolved
	}
	roots, err := pyrepo.FindRoots(absRepo)
	if err != nil {
		return Index{}, nil, err
	}
	if len(roots) == 0 {
		return Index{Packages: map[string]PackageEvidence{}}, map[string][]string{}, nil
	}

	index := Index{Packages: make(map[string]PackageEvidence)}
	nameToPath := map[string]string{} // importable top-level name -> package path
	for _, root := range roots {
		pkgs, err := listPythonPackages(absRepo, root.Dir)
		if err != nil {
			return Index{}, nil, err
		}
		for _, pkg := range pkgs {
			ev, err := scanPythonPackage(absRepo, pkg)
			if err != nil {
				return Index{}, nil, err
			}
			key := normalizePath(ev.Path)
			index.Packages[key] = ev
			top := strings.Split(pkg.Name, ".")[0]
			if top != "" {
				// Prefer shortest path for top-level name (package root over subpackages).
				if prev, ok := nameToPath[top]; !ok || len(key) < len(prev) {
					nameToPath[top] = key
				}
			}
			if pkg.Name != "" {
				nameToPath[pkg.Name] = key
			}
		}
	}

	graph := collectPythonImportGraph(absRepo, index, nameToPath)
	return index, graph, nil
}

type pythonPkg struct {
	RelPath string // repo-relative
	AbsDir  string
	Name    string // importable name (last path segment, or dotted under src/)
	Files   []string
}

func listPythonPackages(absRepo, projectRoot string) ([]pythonPkg, error) {
	searchRoots := []string{projectRoot}
	srcDir := filepath.Join(projectRoot, "src")
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		searchRoots = []string{srcDir}
	}

	var out []pythonPkg
	seen := map[string]struct{}{}
	for _, search := range searchRoots {
		err := filepath.WalkDir(search, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !d.IsDir() {
				return nil
			}
			name := d.Name()
			switch name {
			case ".git", ".venv", "venv", "__pycache__", ".tox", "node_modules", "tests", "test", ".eggs":
				if path != search {
					return filepath.SkipDir
				}
			}
			if strings.HasSuffix(name, ".egg-info") || strings.HasSuffix(name, ".dist-info") {
				return filepath.SkipDir
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			var pyFiles []string
			hasInit := false
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				n := e.Name()
				if n == "__init__.py" {
					hasInit = true
				}
				if strings.HasSuffix(n, ".py") && !strings.HasPrefix(n, ".") {
					pyFiles = append(pyFiles, n)
				}
			}
			if len(pyFiles) == 0 {
				return nil
			}
			// Package if __init__.py present, or directory under search with .py modules
			// that is not the search root itself (namespace / flat module dirs).
			if !hasInit && path == search {
				return nil
			}
			if !hasInit {
				// Only treat as package when it has multiple modules or is not a tests folder.
				if len(pyFiles) == 0 {
					return nil
				}
			}
			rel, err := filepath.Rel(absRepo, path)
			if err != nil {
				return terrors.Wrap(err, terrors.CodeInternal, "sourceindex.listPythonPackages", "rel").
					With("path", path)
			}
			relSlash := normalizePath(rel)
			if _, ok := seen[relSlash]; ok {
				return nil
			}
			seen[relSlash] = struct{}{}
			importName := filepath.Base(path)
			// src/foo -> foo; nested src/pkg/sub -> still base name for top-level only
			if strings.Contains(relSlash, "/") {
				parts := strings.Split(relSlash, "/")
				if len(parts) >= 2 && parts[0] == "src" {
					importName = parts[1]
					if len(parts) > 2 {
						importName = strings.Join(parts[1:], ".")
					}
				} else {
					importName = strings.ReplaceAll(relSlash, "/", ".")
				}
			}
			sort.Strings(pyFiles)
			out = append(out, pythonPkg{
				RelPath: relSlash,
				AbsDir:  path,
				Name:    importName,
				Files:   pyFiles,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out, nil
}

func scanPythonPackage(absRepo string, pkg pythonPkg) (PackageEvidence, error) {
	ev := PackageEvidence{
		Path:     pkg.RelPath,
		Name:     pkg.Name,
		Language: LangPython,
		Files:    make([]string, 0, len(pkg.Files)),
	}
	exportedDecls := map[string]struct{}{}
	exportedFuncs := map[string]struct{}{}
	exportedMethods := map[string]struct{}{}
	var exportedBodies []SymbolBody

	for _, file := range pkg.Files {
		abs := filepath.Join(pkg.AbsDir, file)
		rel, err := filepath.Rel(absRepo, abs)
		if err != nil {
			return PackageEvidence{}, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.scanPythonPackage", "rel file").
				With("file", abs)
		}
		ev.Files = append(ev.Files, normalizePath(rel))
		raw, err := os.ReadFile(abs)
		if err != nil {
			return PackageEvidence{}, terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.scanPythonPackage", "read file").
				With("file", abs)
		}
		src := string(raw)
		mod, err := parser2.ParseFile(abs, src)
		if err != nil {
			// Fail-closed on parse: skip file but keep inventory (syntax errors should not invent roles).
			continue
		}
		scanPythonModule(mod, src, &ev, exportedDecls, exportedFuncs, exportedMethods, &exportedBodies)
	}
	ev.ExportedDecls = sortedKeys(exportedDecls)
	ev.ExportedFuncs = sortedKeys(exportedFuncs)
	ev.ExportedMethods = sortedKeys(exportedMethods)
	ev.ExportedBodies = exportedBodies
	if ev.HasMain {
		ev.DeliveryHint = DeliveryCLI
	} else if ev.ImportsNetHTTP && ev.HTTPSurfaceIdent {
		ev.DeliveryHint = DeliveryServerHTTP
	} else if ev.JSONTags && len(ev.ExportedFuncs) == 0 && len(ev.ExportedMethods) == 0 {
		ev.DeliveryHint = DeliveryDTO
	}
	return ev, nil
}

func scanPythonModule(
	mod *parser2.Module,
	src string,
	ev *PackageEvidence,
	exportedDecls, exportedFuncs, exportedMethods map[string]struct{},
	exportedBodies *[]SymbolBody,
) {
	if mod == nil {
		return
	}
	walkPythonStmts(mod.Body, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
}

func walkPythonStmts(
	stmts []parser2.Stmt,
	src string,
	ev *PackageEvidence,
	exportedDecls, exportedFuncs, exportedMethods map[string]struct{},
	exportedBodies *[]SymbolBody,
) {
	for _, st := range stmts {
		switch n := st.(type) {
		case *parser2.Import:
			for _, a := range n.Names {
				markPythonImport(ev, a.Name)
			}
		case *parser2.ImportFrom:
			modName := strings.TrimSpace(n.Module)
			if modName != "" {
				markPythonImport(ev, modName)
			}
		case *parser2.FunctionDef:
			if isPublicPythonName(n.Name) {
				exportedFuncs[n.Name] = struct{}{}
				*exportedBodies = append(*exportedBodies, SymbolBody{Name: n.Name, Kind: "func", Source: snippetAround(src, n.P)})
			}
			inspectDecorators(n.DecoratorList, ev)
			walkPythonStmts(n.Body, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
		case *parser2.AsyncFunctionDef:
			if isPublicPythonName(n.Name) {
				exportedFuncs[n.Name] = struct{}{}
				*exportedBodies = append(*exportedBodies, SymbolBody{Name: n.Name, Kind: "func", Source: snippetAround(src, n.P)})
			}
			inspectDecorators(n.DecoratorList, ev)
			walkPythonStmts(n.Body, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
		case *parser2.ClassDef:
			if isPublicPythonName(n.Name) {
				exportedDecls[n.Name] = struct{}{}
				*exportedBodies = append(*exportedBodies, SymbolBody{Name: n.Name, Kind: "type", Source: snippetAround(src, n.P)})
			}
			for _, base := range n.Bases {
				if nameLooksLike(base, "BaseModel", "TypedDict") {
					ev.JSONTags = true
				}
			}
			inspectDecorators(n.DecoratorList, ev)
			for _, d := range n.DecoratorList {
				if nameLooksLike(d, "dataclass") {
					ev.JSONTags = true
				}
			}
			for _, bodyStmt := range n.Body {
				if fn, ok := bodyStmt.(*parser2.FunctionDef); ok && isPublicPythonName(fn.Name) {
					exportedMethods[n.Name+"."+fn.Name] = struct{}{}
				}
				if fn, ok := bodyStmt.(*parser2.AsyncFunctionDef); ok && isPublicPythonName(fn.Name) {
					exportedMethods[n.Name+"."+fn.Name] = struct{}{}
				}
			}
			walkPythonStmts(n.Body, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
		case *parser2.If:
			if isDunderMainGuard(n.Test) {
				ev.HasMain = true
			}
			walkPythonStmts(n.Body, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
			walkPythonStmts(n.Orelse, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
		case *parser2.Assign:
			inspectExpr(n.Value, ev)
			for _, t := range n.Targets {
				inspectExpr(t, ev)
			}
		case *parser2.ExprStmt:
			inspectExpr(n.Value, ev)
		case *parser2.AnnAssign:
			inspectExpr(n.Value, ev)
			inspectExpr(n.Annotation, ev)
		case *parser2.For:
			walkPythonStmts(n.Body, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
			walkPythonStmts(n.Orelse, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
		case *parser2.While:
			walkPythonStmts(n.Body, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
			walkPythonStmts(n.Orelse, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
		case *parser2.With:
			walkPythonStmts(n.Body, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
		case *parser2.Try:
			walkPythonStmts(n.Body, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
			walkPythonStmts(n.Orelse, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
			walkPythonStmts(n.Finalbody, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
			for _, h := range n.Handlers {
				if h != nil {
					walkPythonStmts(h.Body, src, ev, exportedDecls, exportedFuncs, exportedMethods, exportedBodies)
				}
			}
		}
	}
}

func markPythonImport(ev *PackageEvidence, mod string) {
	mod = strings.TrimSpace(mod)
	if mod == "" {
		return
	}
	top := strings.Split(mod, ".")[0]
	switch top {
	case "fastapi", "flask", "starlette", "aiohttp", "django":
		ev.ImportsNetHTTP = true
		ev.HTTPSurfaceIdent = true
	case "subprocess":
		ev.ImportsOsExec = true
	case "grpc", "grpcio":
		ev.ImportsGRPC = true
	case "opentelemetry":
		ev.ImportsOTel = true
	case "prometheus_client":
		ev.ImportsPrometheus = true
	case "pydantic":
		ev.JSONTags = true
	case "dataclasses":
		ev.JSONTags = true
	case "typing_extensions", "typing":
		// TypedDict may appear; do not set alone
	}
}

func inspectDecorators(decorators []parser2.Expr, ev *PackageEvidence) {
	for _, d := range decorators {
		if nameLooksLike(d, "app.get", "app.post", "app.put", "app.delete", "router.get", "router.post",
			"get", "post", "put", "delete", "route", "api_view") {
			ev.HTTPSurfaceIdent = true
			ev.ImportsNetHTTP = true
		}
		if nameLooksLike(d, "dataclass") {
			ev.JSONTags = true
		}
	}
}

func inspectExpr(expr parser2.Expr, ev *PackageEvidence) {
	switch n := expr.(type) {
	case *parser2.Call:
		if nameLooksLike(n.Func, "FastAPI", "Flask", "Starlette", "APIRouter", "Application") {
			ev.ImportsNetHTTP = true
			ev.HTTPSurfaceIdent = true
		}
		if nameLooksLike(n.Func, "run", "Popen", "call", "system") {
			// Only count as exec when subprocess already imported; soft signal via ImportsOsExec already set.
		}
		inspectExpr(n.Func, ev)
		for _, a := range n.Args {
			inspectExpr(a, ev)
		}
	case *parser2.Attribute:
		inspectExpr(n.Value, ev)
	case *parser2.Name:
		if n.Id == "TypedDict" {
			ev.JSONTags = true
		}
	}
}

func isDunderMainGuard(test parser2.Expr) bool {
	// __name__ == "__main__"
	cmp, ok := test.(*parser2.Compare)
	if !ok || len(cmp.Ops) != 1 || len(cmp.Comparators) != 1 {
		return false
	}
	if cmp.Ops[0] != "Eq" && cmp.Ops[0] != "==" {
		// gopapy may use Eq as op name
		if !strings.EqualFold(cmp.Ops[0], "Eq") && cmp.Ops[0] != "==" {
			return false
		}
	}
	left, ok := cmp.Left.(*parser2.Name)
	if !ok || left.Id != "__name__" {
		return false
	}
	right, ok := cmp.Comparators[0].(*parser2.Constant)
	if !ok {
		return false
	}
	s, ok := right.Value.(string)
	return ok && s == "__main__"
}

func nameLooksLike(expr parser2.Expr, names ...string) bool {
	got := exprName(expr)
	for _, want := range names {
		if got == want || strings.HasSuffix(got, "."+want) {
			return true
		}
	}
	return false
}

func exprName(expr parser2.Expr) string {
	switch n := expr.(type) {
	case *parser2.Name:
		return n.Id
	case *parser2.Attribute:
		base := exprName(n.Value)
		if base == "" {
			return n.Attr
		}
		return base + "." + n.Attr
	case *parser2.Call:
		return exprName(n.Func)
	default:
		return ""
	}
}

func isPublicPythonName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "_") {
		return false
	}
	r, _ := utf8First(name)
	return unicode.IsLetter(r)
}

func utf8First(s string) (rune, int) {
	for _, r := range s {
		return r, 1
	}
	return 0, 0
}

func snippetAround(src string, p parser2.Pos) string {
	lines := strings.Split(src, "\n")
	if p.Line <= 0 || p.Line > len(lines) {
		if len(src) > 400 {
			return src[:400]
		}
		return src
	}
	start := p.Line - 1
	end := start + 12
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

// collectPythonImportGraph builds first-party edges by re-reading package files.
func collectPythonImportGraph(absRepo string, index Index, nameToPath map[string]string) map[string][]string {
	graph := map[string][]string{}
	for path, ev := range index.Packages {
		seen := map[string]struct{}{}
		var targets []string
		for _, relFile := range ev.Files {
			abs := filepath.Join(absRepo, filepath.FromSlash(relFile))
			raw, err := os.ReadFile(abs)
			if err != nil {
				continue
			}
			mod, err := parser2.ParseFile(abs, string(raw))
			if err != nil || mod == nil {
				continue
			}
			for _, st := range mod.Body {
				var mods []string
				switch n := st.(type) {
				case *parser2.Import:
					for _, a := range n.Names {
						mods = append(mods, a.Name)
					}
				case *parser2.ImportFrom:
					if n.Module != "" {
						mods = append(mods, n.Module)
					}
				}
				for _, m := range mods {
					top := strings.Split(m, ".")[0]
					target, ok := nameToPath[top]
					if !ok {
						// also try full dotted name
						target, ok = nameToPath[m]
					}
					if !ok || target == path {
						continue
					}
					if _, dup := seen[target]; dup {
						continue
					}
					seen[target] = struct{}{}
					targets = append(targets, target)
				}
			}
		}
		if len(targets) > 0 {
			sort.Strings(targets)
			graph[path] = targets
		}
	}
	return graph
}

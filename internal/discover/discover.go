package discover

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/behaviorengineering/typology/catalog"
	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/gorepo"
	"github.com/behaviorengineering/typology/internal/sourceindex"
)

// Options configures discovery.
type Options struct {
	RepoRoot string
	DocsRoot string
	Modules  []string
	Module   string
}

// Result is a proposed typology draft.
type Result struct {
	Typology catalog.Typology `json:"typology"`
	Module   string           `json:"module"`
	Packages []string         `json:"packages"`
	Graph    GraphSummary     `json:"graph,omitempty"`
}

// Run scans a Go module or workspace and proposes slices, libraries, and bindings.
func Run(opts Options) (Result, error) {
	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		return Result{}, terrors.New(terrors.CodeInvalid, "discover.Run", "repo root empty")
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeInvalid, "discover.Run", "abs repo").
			With("repo", repo)
	}
	modules, err := gorepo.ResolveModules(absRepo, opts.Modules, opts.Module)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeFailedPrecondition, "discover.Run", "resolve modules")
	}
	pkgs, err := listPackages(modules)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeUnavailable, "discover.Run", "list packages")
	}
	graph, err := ImportGraphInModules(absRepo, modules)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeUnavailable, "discover.Run", "build import graph")
	}
	graphSummary := BuildGraphSummary(graph)

	idx, err := sourceindex.BuildInModules(absRepo, modules)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeUnavailable, "discover.Run", "build source index")
	}
	roleByPath := map[string]sourceindex.RoleNode{}
	for _, n := range sourceindex.BuildRoleTopology(idx, graph).Packages {
		roleByPath[normalizeDiscoverPath(n.Path)] = n
	}

	platformPaths := map[string]string{} // normalized -> display path
	for _, p := range graphSummary.PlatformLeaves {
		platformPaths[normalizeDiscoverPath(p)] = normalizeDiscoverPath(p)
	}

	docsRoot := opts.DocsRoot
	if docsRoot == "" {
		docsRoot = catalog.DefaultDocsRoot
	}

	libraries, libForPath := draftLibraries(platformPaths)
	slicePkgs := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		if _, ok := platformPaths[normalizeDiscoverPath(p)]; ok {
			continue
		}
		slicePkgs = append(slicePkgs, p)
	}

	clusters := clusterPackages(slicePkgs)
	slices := make([]catalog.Slice, 0, len(clusters))
	sliceForPath := map[string]string{}
	for _, clusterID := range sortedKeys(clusters) {
		paths := clusters[clusterID]
		var owns []catalog.Component
		surfaceByKind := map[catalog.InteractionKind][]catalog.Component{}
		for _, p := range paths {
			compID := componentID(p)
			layer, kind := layerFromRole(roleByPath[normalizeDiscoverPath(p)])
			if layer == catalog.LayerDomain {
				owns = append(owns, catalog.Component{
					ID:    compID,
					Path:  normalizeDiscoverPath(p),
					Layer: layer,
				})
			} else {
				surfaceByKind[kind] = append(surfaceByKind[kind], catalog.Component{
					ID:   compID,
					Path: normalizeDiscoverPath(p),
				})
			}
			sliceForPath[normalizeDiscoverPath(p)] = clusterID
		}
		if len(owns) == 0 && len(surfaceByKind) == 0 {
			continue
		}
		slice := catalog.Slice{
			ID:       clusterID,
			Owns:     owns,
			Surfaces: catalog.BuildSurfaces(clusterID, surfaceByKind),
		}
		slice.Docs = catalog.DefaultDocCluster(slice, docsRoot)
		slices = append(slices, slice)
	}

	// Avoid library ids colliding with slice ids.
	sliceIDs := map[string]struct{}{}
	for _, s := range slices {
		sliceIDs[s.ID] = struct{}{}
	}
	for i := range libraries {
		id := libraries[i].ID
		if _, ok := sliceIDs[id]; !ok {
			continue
		}
		newID := id + "-lib"
		libraries[i].ID = newID
		for path, libID := range libForPath {
			if libID == id {
				libForPath[path] = newID
			}
		}
	}

	var sliceBindings []catalog.SliceBinding
	seenSliceBind := map[string]struct{}{}
	var compBindings []catalog.ComponentBinding
	seenCompBind := map[string]struct{}{}

	for from, imports := range graph {
		fromNorm := normalizeDiscoverPath(from)
		fromSlice := sliceForPath[fromNorm]
		fromLib := libForPath[fromNorm]
		fromComp := componentID(fromNorm)
		for _, imp := range imports {
			toNorm := normalizeDiscoverPath(imp)
			toSlice := sliceForPath[toNorm]
			toLib := libForPath[toNorm]
			toComp := componentID(toNorm)
			if fromSlice != "" && toLib != "" {
				key := fromSlice + "->" + toLib
				if _, ok := seenSliceBind[key]; !ok {
					seenSliceBind[key] = struct{}{}
					sliceBindings = append(sliceBindings, catalog.SliceBinding{
						From: fromSlice,
						To:   toLib,
						Kind: catalog.SliceReads,
					})
				}
				continue
			}
			if fromLib != "" {
				// Libraries must not gain domain knowledge via slice imports; skip draft bindings.
				continue
			}
			if fromSlice == "" || toSlice == "" {
				continue
			}
			if fromSlice != toSlice {
				key := fromSlice + "->" + toSlice
				if _, ok := seenSliceBind[key]; !ok {
					seenSliceBind[key] = struct{}{}
					sliceBindings = append(sliceBindings, catalog.SliceBinding{
						From: fromSlice,
						To:   toSlice,
						Kind: catalog.SliceReads,
					})
				}
			} else if fromComp != toComp {
				key := fromComp + "->" + toComp
				if _, ok := seenCompBind[key]; !ok {
					seenCompBind[key] = struct{}{}
					compBindings = append(compBindings, catalog.ComponentBinding{
						From: fromComp,
						To:   toComp,
						Rule: catalog.BindingReads,
					})
				}
			}
		}
	}

	t := catalog.Typology{
		ID:                filepath.Base(absRepo),
		Scope:             catalog.Scope{Modules: moduleRels(modules)},
		Slices:            slices,
		Libraries:         libraries,
		SliceBindings:     sliceBindings,
		ComponentBindings: compBindings,
	}
	return Result{
		Typology: t,
		Module:   moduleLabel(modules),
		Packages: pkgs,
		Graph:    graphSummary,
	}, nil
}

func draftLibraries(platformPaths map[string]string) ([]catalog.Library, map[string]string) {
	libForPath := map[string]string{}
	if len(platformPaths) == 0 {
		return nil, libForPath
	}
	paths := make([]string, 0, len(platformPaths))
	for _, p := range platformPaths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	usedIDs := map[string]struct{}{}
	libraries := make([]catalog.Library, 0, len(paths))
	for _, p := range paths {
		id := libraryIDFromPath(p)
		base := id
		for i := 2; ; i++ {
			if _, ok := usedIDs[id]; !ok {
				break
			}
			id = fmt.Sprintf("%s-%d", base, i)
		}
		usedIDs[id] = struct{}{}
		libForPath[p] = id
		libraries = append(libraries, catalog.Library{
			ID:      id,
			Purpose: "Shared technical package without domain knowledge (discover draft)",
			Owns: []catalog.Component{{
				ID:    componentID(p),
				Path:  p,
				Layer: catalog.LayerDomain,
			}},
		})
	}
	return libraries, libForPath
}

func libraryIDFromPath(relPkg string) string {
	base := filepath.Base(normalizeDiscoverPath(relPkg))
	if base == "" || base == "." || base == "/" {
		return "library"
	}
	return base
}

func normalizeDiscoverPath(p string) string {
	return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "./"))
}

// ImportGraph returns import edges between local packages (dir paths relative to
// the repo or workspace root, for example "./engine/internal/cli").
func ImportGraph(repoRoot string) (map[string][]string, error) {
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, terrors.Wrap(err, terrors.CodeInvalid, "discover.ImportGraph", "abs repo").
			With("repo", repoRoot)
	}
	modules, err := gorepo.ResolveModules(absRepo, nil, "")
	if err != nil {
		return nil, terrors.Wrap(err, terrors.CodeFailedPrecondition, "discover.ImportGraph", "resolve modules")
	}
	return ImportGraphInModules(absRepo, modules)
}

// ImportGraphInModules returns import edges for the selected modules.
func ImportGraphInModules(repoRoot string, modules []gorepo.Module) (map[string][]string, error) {
	_, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, terrors.Wrap(err, terrors.CodeInvalid, "discover.ImportGraphInModules", "abs repo").
			With("repo", repoRoot)
	}
	local := map[string]string{}
	type pkgRef struct {
		repo    string
		imports []string
	}
	var refs []pkgRef
	for _, mod := range modules {
		modPkgs, err := listPackageInfosInModule(mod.Dir)
		if err != nil {
			return nil, terrors.Wrap(err, terrors.CodeUnavailable, "discover.ImportGraph", "list packages").
				With("module", mod.Path)
		}
		for _, pkg := range modPkgs {
			repoRel, err := importToRel(mod.Dir, pkg.ImportPath)
			if err != nil {
				return nil, terrors.Wrap(err, terrors.CodeInvalid, "discover.ImportGraph", "resolve package path").
					With("module", mod.Path).
					With("package", pkg.ImportPath)
			}
			repoRel = workspacePkgPath(mod.Rel, strings.TrimPrefix(repoRel, "./"))
			local[pkg.ImportPath] = repoRel
			refs = append(refs, pkgRef{repo: repoRel, imports: pkg.Imports})
		}
	}

	graph := map[string][]string{}
	for _, ref := range refs {
		graph[ref.repo] = nil
		for _, imp := range ref.imports {
			if rel, ok := local[imp]; ok && rel != ref.repo {
				graph[ref.repo] = append(graph[ref.repo], rel)
			}
		}
		sort.Strings(graph[ref.repo])
	}
	return graph, nil
}

func goCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd
}

func listPackages(modules []gorepo.Module) ([]string, error) {
	var pkgs []string
	seen := map[string]struct{}{}
	for _, mod := range modules {
		modPkgs, err := listPackagesInModule(mod.Dir)
		if err != nil {
			return nil, err
		}
		for _, p := range modPkgs {
			repoRel := workspacePkgPath(mod.Rel, p)
			if _, ok := seen[repoRel]; ok {
				continue
			}
			seen[repoRel] = struct{}{}
			pkgs = append(pkgs, repoRel)
		}
	}
	sort.Strings(pkgs)
	return pkgs, nil
}

func moduleRels(modules []gorepo.Module) []string {
	rels := make([]string, 0, len(modules))
	for _, module := range modules {
		rels = append(rels, filepath.ToSlash(module.Rel))
	}
	sort.Strings(rels)
	return rels
}

func listPackagesInModule(modRoot string) ([]string, error) {
	infos, err := listPackageInfosInModule(modRoot)
	if err != nil {
		return nil, err
	}
	modPath, err := gorepo.ReadModulePath(filepath.Join(modRoot, "go.mod"))
	if err != nil {
		return nil, err
	}
	var pkgs []string
	for _, info := range infos {
		rel, err := importToRel(modRoot, info.ImportPath)
		if err != nil {
			return nil, terrors.Wrap(err, terrors.CodeInvalid, "discover.listPackages", "resolve package path").
				With("module", modPath).
				With("package", info.ImportPath)
		}
		pkgs = append(pkgs, rel)
	}
	sort.Strings(pkgs)
	return pkgs, nil
}

type packageInfo struct {
	ImportPath string
	Imports    []string
}

func listPackageInfosInModule(modRoot string) ([]packageInfo, error) {
	cmd := goCmd(modRoot, "list", "-json", "./...")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, terrors.Wrap(err, terrors.CodeUnavailable, "discover.listPackages", "go list failed").
			With("stderr", strings.TrimSpace(stderr.String())).
			With("dir", modRoot)
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	var infos []packageInfo
	for {
		var info packageInfo
		if err := dec.Decode(&info); err != nil {
			if err == io.EOF {
				break
			}
			return nil, terrors.Wrap(err, terrors.CodeInternal, "discover.listPackages", "decode go list json").
				With("dir", modRoot)
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func importToRel(modRoot, importPath string) (string, error) {
	modPath, err := gorepo.ReadModulePath(filepath.Join(modRoot, "go.mod"))
	if err != nil {
		return "", err
	}
	if importPath == modPath {
		return ".", nil
	}
	prefix := modPath + "/"
	if !strings.HasPrefix(importPath, prefix) {
		return "", terrors.New(terrors.CodeInvalid, "discover.importToRel", "package outside module").
			With("import", importPath).
			With("module", modPath)
	}
	return "./" + strings.TrimPrefix(importPath, prefix), nil
}

func workspacePkgPath(modRel, pkgRel string) string {
	pkg := strings.TrimPrefix(pkgRel, "./")
	if modRel == "." || modRel == "" {
		if pkg == "" || pkg == "." {
			return "."
		}
		return "./" + pkg
	}
	if pkg == "" || pkg == "." {
		return "./" + filepath.ToSlash(modRel)
	}
	return "./" + filepath.ToSlash(filepath.Join(modRel, pkg))
}

func moduleLabel(modules []gorepo.Module) string {
	if len(modules) == 1 {
		return modules[0].Path
	}
	parts := make([]string, 0, len(modules))
	for _, m := range modules {
		parts = append(parts, m.Path)
	}
	sort.Strings(parts)
	return "workspace:" + strings.Join(parts, ",")
}

func clusterPackages(pkgs []string) map[string][]string {
	clusters := map[string][]string{}
	for _, p := range pkgs {
		id := clusterID(p)
		clusters[id] = append(clusters[id], p)
	}
	for id := range clusters {
		sort.Strings(clusters[id])
	}
	return clusters
}

func clusterID(relPkg string) string {
	rel := strings.TrimPrefix(relPkg, "./")
	parts := strings.Split(rel, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "root"
	}
	if parts[0] == "internal" && len(parts) > 1 {
		return parts[1]
	}
	if parts[0] == "cmd" && len(parts) > 1 {
		return parts[1]
	}
	return parts[0]
}

func componentID(relPkg string) string {
	rel := strings.TrimPrefix(relPkg, "./")
	return strings.ReplaceAll(rel, "/", "-")
}

func layerFromRole(node sourceindex.RoleNode) (catalog.Layer, catalog.InteractionKind) {
	switch node.Role {
	case sourceindex.RoleEntrypoint:
		return catalog.LayerInteraction, catalog.InteractionCLI
	case sourceindex.RoleHTTPSurface:
		for _, e := range node.Evidence {
			if e == "embeds_static" {
				return catalog.LayerInteraction, catalog.InteractionUI
			}
		}
		return catalog.LayerInteraction, catalog.InteractionAPI
	default:
		return catalog.LayerDomain, ""
	}
}

// inferLayer is retained for tests that call the old name; it never uses path tokens.
func inferLayer(relPkg string) (catalog.Layer, catalog.InteractionKind) {
	_ = relPkg
	return catalog.LayerDomain, ""
}

// PackageNode provides degree metrics and caller relationships for a package.
type PackageNode struct {
	Path         string   `json:"path"`
	InDegree     int      `json:"inDegree"`
	OutDegree    int      `json:"outDegree"`
	Imports      []string `json:"imports,omitempty"`
	ImportedBy   []string `json:"importedBy,omitempty"`
	IsLeaf       bool     `json:"isLeaf"`
	IsHub        bool     `json:"isHub"`
	SoleImporter string   `json:"soleImporter,omitempty"`
}

// MergeSuggestion captures a heuristic merge candidate.
type MergeSuggestion struct {
	SourcePackage string `json:"sourcePackage"`
	TargetPackage string `json:"targetPackage"`
	Reason        string `json:"reason"`
}

// StemCollision captures packages sharing a stem but with distinct importers.
type StemCollision struct {
	Stem     string   `json:"stem"`
	Packages []string `json:"packages"`
	Warning  string   `json:"warning"`
}

// GraphSummary provides in/out degree metrics, leaves, hubs, merge suggestions, and stem collision warnings.
type GraphSummary struct {
	Nodes            map[string]PackageNode `json:"nodes"`
	Leaves           []string               `json:"leaves"`
	Hubs             []string               `json:"hubs"`
	MergeSuggestions []MergeSuggestion      `json:"mergeSuggestions,omitempty"`
	StemCollisions   []StemCollision        `json:"stemCollisions,omitempty"`
	PlatformLeaves   []string               `json:"platformLeaves,omitempty"`
}

// AnalyzeGraph analyzes package import topology from a repo root.
func AnalyzeGraph(repoRoot string) (GraphSummary, error) {
	graph, err := ImportGraph(repoRoot)
	if err != nil {
		return GraphSummary{}, err
	}
	return BuildGraphSummary(graph), nil
}

// AnalyzeGraphInModules analyzes topology for the selected modules.
func AnalyzeGraphInModules(repoRoot string, modules []gorepo.Module) (GraphSummary, error) {
	graph, err := ImportGraphInModules(repoRoot, modules)
	if err != nil {
		return GraphSummary{}, err
	}
	return BuildGraphSummary(graph), nil
}

// BuildGraphSummary derives node metrics, leaves, hubs, merge suggestions, and stem collision warnings.
func BuildGraphSummary(graph map[string][]string) GraphSummary {
	allPkgs := map[string]struct{}{}
	importedByMap := map[string][]string{}

	for from, toList := range graph {
		allPkgs[from] = struct{}{}
		for _, to := range toList {
			allPkgs[to] = struct{}{}
			importedByMap[to] = append(importedByMap[to], from)
		}
	}

	nodes := make(map[string]PackageNode, len(allPkgs))
	var leaves []string
	var hubs []string
	var platformLeaves []string
	var suggestions []MergeSuggestion

	sortedPkgs := make([]string, 0, len(allPkgs))
	for p := range allPkgs {
		sortedPkgs = append(sortedPkgs, p)
	}
	sort.Strings(sortedPkgs)

	for _, p := range sortedPkgs {
		imports := append([]string(nil), graph[p]...)
		sort.Strings(imports)
		callers := append([]string(nil), importedByMap[p]...)
		sort.Strings(callers)

		inDeg := len(callers)
		outDeg := len(imports)
		isLeaf := outDeg == 0 && inDeg > 0
		isHub := inDeg >= 3 || outDeg >= 4

		var soleImporter string
		if inDeg == 1 && callers[0] != p {
			soleImporter = callers[0]
			suggestions = append(suggestions, MergeSuggestion{
				SourcePackage: p,
				TargetPackage: soleImporter,
				Reason:        fmt.Sprintf("sole importer: only imported by %s", soleImporter),
			})
		}

		if isLeaf {
			leaves = append(leaves, p)
		}
		if isHub {
			hubs = append(hubs, p)
		}

		clean := strings.ToLower(p)
		if (strings.Contains(clean, "config") || strings.Contains(clean, "telemetry") ||
			strings.Contains(clean, "auth") || strings.Contains(clean, "logger") ||
			strings.Contains(clean, "logging")) && inDeg >= 2 {
			platformLeaves = append(platformLeaves, p)
		}

		nodes[p] = PackageNode{
			Path:         p,
			InDegree:     inDeg,
			OutDegree:    outDeg,
			Imports:      imports,
			ImportedBy:   callers,
			IsLeaf:       isLeaf,
			IsHub:        isHub,
			SoleImporter: soleImporter,
		}
	}

	stemMap := map[string][]string{}
	for _, p := range sortedPkgs {
		base := filepath.Base(p)
		if len(base) >= 4 {
			stem := base[:4]
			stemMap[stem] = append(stemMap[stem], p)
		}
	}
	var collisions []StemCollision
	for stem, group := range stemMap {
		if len(group) < 2 {
			continue
		}
		firstCallers := nodes[group[0]].ImportedBy
		disjoint := true
		for _, other := range group[1:] {
			otherCallers := nodes[other].ImportedBy
			if len(firstCallers) > 0 && len(otherCallers) > 0 && hasOverlap(firstCallers, otherCallers) {
				disjoint = false
				break
			}
		}
		if disjoint && (len(nodes[group[0]].ImportedBy) > 0 || len(nodes[group[1]].ImportedBy) > 0) {
			collisions = append(collisions, StemCollision{
				Stem:     stem,
				Packages: group,
				Warning:  fmt.Sprintf("packages share stem %q but have different importers; verify if distinct bounded contexts before merging", stem),
			})
		}
	}

	return GraphSummary{
		Nodes:            nodes,
		Leaves:           leaves,
		Hubs:             hubs,
		MergeSuggestions: suggestions,
		StemCollisions:   collisions,
		PlatformLeaves:   platformLeaves,
	}
}

func hasOverlap(a, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := set[v]; ok {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

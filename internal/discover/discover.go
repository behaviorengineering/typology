package discover

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/behaviorengineering/typology/catalog"
	terrors "github.com/behaviorengineering/typology/errors"
)

// Options configures discovery.
type Options struct {
	RepoRoot string
	DocsRoot string
}

// Result is a proposed typology draft.
type Result struct {
	Typology catalog.Typology `json:"typology"`
	Module   string           `json:"module"`
	Packages []string         `json:"packages"`
	Graph    GraphSummary     `json:"graph,omitempty"`
}

// Run scans a Go module and proposes slices plus bindings.
func Run(opts Options) (Result, error) {
	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		return Result{}, terrors.New(terrors.CodeInvalid, "discover.Run", "repo root empty")
	}
	modRoot, modPath, err := moduleRoot(repo)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeFailedPrecondition, "discover.Run", "resolve module")
	}
	pkgs, err := listPackages(modRoot)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeUnavailable, "discover.Run", "list packages")
	}
	graph, err := ImportGraph(modRoot)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeUnavailable, "discover.Run", "build import graph")
	}

	clusters := clusterPackages(pkgs)
	docsRoot := opts.DocsRoot
	if docsRoot == "" {
		docsRoot = catalog.DefaultDocsRoot
	}

	slices := make([]catalog.Slice, 0, len(clusters))
	sliceForPath := map[string]string{}
	for _, clusterID := range sortedKeys(clusters) {
		paths := clusters[clusterID]
		var owns []catalog.Component
		surfaceByKind := map[catalog.InteractionKind][]catalog.Component{}
		for _, p := range paths {
			compID := componentID(modPath, p)
			layer, kind := inferLayer(p)
			if layer == catalog.LayerDomain {
				owns = append(owns, catalog.Component{
					ID:    compID,
					Path:  p,
					Layer: layer,
				})
			} else {
				surfaceByKind[kind] = append(surfaceByKind[kind], catalog.Component{
					ID:   compID,
					Path: p,
				})
			}
			sliceForPath[p] = clusterID
		}
		slice := catalog.Slice{
			ID:       clusterID,
			Owns:     owns,
			Surfaces: catalog.BuildSurfaces(clusterID, surfaceByKind),
		}
		slice.Docs = catalog.DefaultDocCluster(slice, docsRoot)
		slices = append(slices, slice)
	}

	var sliceBindings []catalog.SliceBinding
	seenSliceBind := map[string]struct{}{}
	var compBindings []catalog.ComponentBinding
	seenCompBind := map[string]struct{}{}

	for from, imports := range graph {
		fromSlice := sliceForPath[from]
		if fromSlice == "" {
			continue
		}
		fromComp := componentID(modPath, from)
		for _, imp := range imports {
			toSlice := sliceForPath[imp]
			if toSlice == "" {
				continue
			}
			toComp := componentID(modPath, imp)
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
		ID:                filepath.Base(modRoot),
		Slices:            slices,
		SliceBindings:     sliceBindings,
		ComponentBindings: compBindings,
	}
	graphSummary := BuildGraphSummary(graph)
	return Result{
		Typology: t,
		Module:   modPath,
		Packages: pkgs,
		Graph:    graphSummary,
	}, nil
}

// ImportGraph returns import edges between local packages (dir paths relative to module root).
func ImportGraph(repoRoot string) (map[string][]string, error) {
	modRoot, modPath, err := moduleRoot(repoRoot)
	if err != nil {
		return nil, terrors.Wrap(err, terrors.CodeFailedPrecondition, "discover.ImportGraph", "resolve module")
	}
	pkgs, err := listPackages(modRoot)
	if err != nil {
		return nil, terrors.Wrap(err, terrors.CodeUnavailable, "discover.ImportGraph", "list packages")
	}
	local := map[string]string{}
	for _, p := range pkgs {
		importPath := modPath + "/" + strings.TrimPrefix(p, "./")
		if p == "." {
			importPath = modPath
		}
		local[importPath] = p
	}

	graph := map[string][]string{}
	for _, p := range pkgs {
		graph[p] = nil
		imports, err := packageImports(modRoot, p)
		if err != nil {
			return nil, terrors.Wrap(err, terrors.CodeUnavailable, "discover.ImportGraph", "package imports").
				With("pkg", p)
		}
		from := p
		for _, imp := range imports {
			if rel, ok := local[imp]; ok && rel != from {
				graph[from] = append(graph[from], rel)
			}
		}
		sort.Strings(graph[from])
	}
	return graph, nil
}

func moduleRoot(repoRoot string) (string, string, error) {
	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", terrors.Wrap(err, terrors.CodeInvalid, "discover.moduleRoot", "abs repo").
			With("repo", repoRoot)
	}
	dir := abs
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			modPath, err := readModulePath(filepath.Join(dir, "go.mod"))
			if err != nil {
				return "", "", err
			}
			return dir, modPath, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", "", terrors.New(terrors.CodeNotFound, "discover.moduleRoot", "go.mod not found").
		With("repo", repoRoot)
}

func readModulePath(modFile string) (string, error) {
	data, err := os.ReadFile(modFile)
	if err != nil {
		return "", terrors.Wrap(err, terrors.CodeUnavailable, "discover.readModulePath", "read go.mod").
			With("path", modFile)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", terrors.New(terrors.CodeInvalid, "discover.readModulePath", "module line missing").
		With("path", modFile)
}

func goCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd
}

func listPackages(dir string) ([]string, error) {
	cmd := goCmd(dir, "list", "./...")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, terrors.Wrap(err, terrors.CodeUnavailable, "discover.listPackages", "go list failed").
				With("stderr", strings.TrimSpace(string(ee.Stderr))).
				With("dir", dir)
		}
		return nil, terrors.Wrap(err, terrors.CodeUnavailable, "discover.listPackages", "go list").
			With("dir", dir)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var pkgs []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rel, err := importToRel(dir, line)
		if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, rel)
	}
	sort.Strings(pkgs)
	return pkgs, nil
}

func importToRel(modRoot, importPath string) (string, error) {
	modPath, err := readModulePath(filepath.Join(modRoot, "go.mod"))
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

func packageImports(dir, relPkg string) ([]string, error) {
	cmd := goCmd(dir, "list", "-json", relPkg)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, terrors.Wrap(err, terrors.CodeUnavailable, "discover.packageImports", "go list -json failed").
				With("stderr", strings.TrimSpace(string(ee.Stderr))).
				With("pkg", relPkg)
		}
		return nil, terrors.Wrap(err, terrors.CodeUnavailable, "discover.packageImports", "go list -json").
			With("pkg", relPkg)
	}
	var info struct {
		ImportPath string   `json:"ImportPath"`
		Imports    []string `json:"Imports"`
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	if err := dec.Decode(&info); err != nil {
		return nil, terrors.Wrap(err, terrors.CodeInternal, "discover.packageImports", "decode go list json").
			With("pkg", relPkg)
	}
	return info.Imports, nil
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

func componentID(modPath, relPkg string) string {
	rel := strings.TrimPrefix(relPkg, "./")
	rel = strings.ReplaceAll(rel, "/", "-")
	return strings.TrimPrefix(rel, modPath+"-")
}

func inferLayer(relPkg string) (catalog.Layer, catalog.InteractionKind) {
	rel := strings.TrimPrefix(relPkg, "./")
	switch {
	case strings.Contains(rel, "/http") || strings.HasSuffix(rel, "api") || strings.HasSuffix(rel, "/api") || strings.Contains(rel, "api/") || strings.Contains(rel, "/api/"):
		return catalog.LayerInteraction, catalog.InteractionAPI
	case strings.HasPrefix(rel, "cmd/") || strings.Contains(rel, "/cli") || strings.HasSuffix(rel, "/cli") || rel == "cli" || strings.Contains(rel, "cli/"):
		return catalog.LayerInteraction, catalog.InteractionCLI
	case strings.HasPrefix(rel, "ui/") || strings.Contains(rel, "/ui/") || strings.HasPrefix(rel, "viewer/") || strings.Contains(rel, "/viewer") || strings.HasSuffix(rel, "/view") || strings.HasSuffix(rel, "/views") || strings.Contains(rel, "/view/") || strings.Contains(rel, "/views/") || rel == "view" || rel == "views" || rel == "viewer":
		return catalog.LayerInteraction, catalog.InteractionUI
	default:
		return catalog.LayerDomain, ""
	}
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

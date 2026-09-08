package sourceindex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/gorepo"
	"gopkg.in/yaml.v3"
)

// Observed package roles from code evidence (never from folder names).
const (
	RoleEntrypoint  = "entrypoint"
	RoleHTTPSurface = "http_surface"
	RoleDTO         = "dto"
	RoleExecRunner  = "exec_runner"
	RoleAggregator  = "aggregator"
	RoleAdapter     = "adapter"
	RoleConfig      = "config"
	RoleUnknown     = "unknown"
)

// Edge kinds after role revisit.
const (
	EdgeFillsDTO    = "fills_dto"
	EdgeUsesRunner  = "uses_runner"
	EdgeServesHTTP  = "serves_http"
	EdgeComposes    = "composes"
	EdgeReadsConfig = "reads_config"
	EdgeImports     = "imports"
)

const (
	confidenceStage1     = 0.90
	confidenceStage2     = 0.80
	confidenceConflict   = 0.40
	confidencePublishBar = 0.80
)

// RoleNode is one package in the observed topology.
type RoleNode struct {
	Path           string   `yaml:"path" json:"path"`
	Role           string   `yaml:"role" json:"role"`
	Confidence     float64  `yaml:"confidence" json:"confidence"`
	Evidence       []string `yaml:"evidence,omitempty" json:"evidence,omitempty"`
	InspectedStage int      `yaml:"inspected_stage" json:"inspected_stage"`
	CandidateRole  string   `yaml:"candidate_role,omitempty" json:"candidate_role,omitempty"`
}

// RoleEdge is a labeled import between packages after revisit.
type RoleEdge struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to" json:"to"`
	Kind string `yaml:"kind" json:"kind"`
}

// RoleTopology is the pre-cluster observed map.
type RoleTopology struct {
	Packages []RoleNode `yaml:"packages" json:"packages"`
	Edges    []RoleEdge `yaml:"edges,omitempty" json:"edges,omitempty"`
}

type roleCandidate struct {
	Role       string
	Stage      int
	Confidence float64
	Evidence   []string
}

// BuildRoleTopology classifies packages from AST evidence and the import graph.
// Stage 1 uses delivery hints; Stage 2 uses imports/interfaces; revisit labels edges.
// Folder/path names are never used as evidence.
func BuildRoleTopology(idx Index, importGraph map[string][]string) RoleTopology {
	paths := make([]string, 0, len(idx.Packages))
	for p := range idx.Packages {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	nodes := make([]RoleNode, 0, len(paths))
	byPath := make(map[string]RoleNode, len(paths))
	for _, p := range paths {
		ev := idx.Packages[p]
		internalOut := countInternalImports(p, importGraph)
		node := classifyPackage(ev, internalOut)
		nodes = append(nodes, node)
		byPath[p] = node
	}

	edges := labelEdges(importGraph, byPath)
	nodes = revisitNodes(nodes, edges, byPath)

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Kind < edges[j].Kind
	})

	return RoleTopology{Packages: nodes, Edges: edges}
}

func classifyPackage(ev PackageEvidence, internalOut int) RoleNode {
	path := normalizePath(ev.Path)

	// Stage 1: unique delivery hints take priority and do not compete.
	if ev.HasMain {
		return RoleNode{
			Path: path, Role: RoleEntrypoint, Confidence: confidenceStage1,
			Evidence: []string{"has_main"}, InspectedStage: 1,
		}
	}
	if ev.GoEmbed || (ev.ImportsNetHTTP && ev.HTTPSurfaceIdent) {
		evidence := []string{}
		if ev.GoEmbed {
			evidence = append(evidence, "go_embed")
		}
		if ev.EmbedsStatic {
			evidence = append(evidence, "embeds_static")
		}
		if ev.ImportsNetHTTP {
			evidence = append(evidence, "imports_net_http")
		}
		if ev.HTTPSurfaceIdent {
			evidence = append(evidence, "http_surface_ident")
		}
		return RoleNode{
			Path: path, Role: RoleHTTPSurface, Confidence: confidenceStage1,
			Evidence: evidence, InspectedStage: 1,
		}
	}
	if ev.JSONTags && len(ev.ExportedFuncs) == 0 && len(ev.ExportedMethods) == 0 {
		return RoleNode{
			Path: path, Role: RoleDTO, Confidence: confidenceStage1,
			Evidence: []string{"json_tags", "no_exported_funcs", "no_exported_methods"},
			InspectedStage: 1,
		}
	}

	candidates := make([]roleCandidate, 0, 4)
	if ev.ImportsOsExec && exportsRunnerSurface(ev) {
		candidates = append(candidates, roleCandidate{
			Role: RoleExecRunner, Stage: 2, Confidence: confidenceStage2,
			Evidence: []string{"imports_os_exec", "exports_run_surface"},
		})
	}
	if internalOut >= 2 && exportsOrchestration(ev) {
		candidates = append(candidates, roleCandidate{
			Role: RoleAggregator, Stage: 2, Confidence: confidenceStage2,
			Evidence: []string{"orchestration_export", "internal_imports_ge_2"},
		})
	}
	if looksLikeConfig(ev) && internalOut <= 1 {
		candidates = append(candidates, roleCandidate{
			Role: RoleConfig, Stage: 2, Confidence: confidenceStage2,
			Evidence: []string{"load_save_exports"},
		})
	}
	if !ev.GoEmbed && !ev.HTTPSurfaceIdent && ev.ImportsNetHTTP && exportsClientSurface(ev) {
		candidates = append(candidates, roleCandidate{
			Role: RoleAdapter, Stage: 2, Confidence: confidenceStage2,
			Evidence: []string{"imports_net_http", "exports_client_surface", "not_http_surface"},
		})
	}

	return publishNode(path, candidates)
}

func publishNode(path string, candidates []roleCandidate) RoleNode {
	if len(candidates) == 0 {
		return RoleNode{
			Path:           path,
			Role:           RoleUnknown,
			Confidence:     0,
			InspectedStage: 2,
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		return candidates[i].Stage < candidates[j].Stage
	})
	best := candidates[0]
	if len(candidates) > 1 && candidates[1].Role != best.Role {
		return RoleNode{
			Path:           path,
			Role:           RoleUnknown,
			Confidence:     confidenceConflict,
			Evidence:       mergeEvidence(candidates),
			InspectedStage: best.Stage,
			CandidateRole:  best.Role,
		}
	}
	role := best.Role
	conf := best.Confidence
	if conf < confidencePublishBar {
		return RoleNode{
			Path:           path,
			Role:           RoleUnknown,
			Confidence:     conf,
			Evidence:       best.Evidence,
			InspectedStage: best.Stage,
			CandidateRole:  role,
		}
	}
	return RoleNode{
		Path:           path,
		Role:           role,
		Confidence:     conf,
		Evidence:       best.Evidence,
		InspectedStage: best.Stage,
	}
}

func mergeEvidence(cands []roleCandidate) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, c := range cands {
		for _, e := range c.Evidence {
			if _, ok := seen[e]; ok {
				continue
			}
			seen[e] = struct{}{}
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out
}

func exportsRunnerSurface(ev PackageEvidence) bool {
	for _, name := range ev.ExportedDecls {
		if name == "Runner" || name == "Exec" {
			return true
		}
	}
	hasRunOrCommand := false
	for _, name := range ev.ExportedFuncs {
		if name == "Run" || name == "Command" {
			hasRunOrCommand = true
		}
	}
	for _, name := range ev.ExportedMethods {
		base := methodBase(name)
		if base == "Run" || base == "Command" || base == "LookPath" {
			hasRunOrCommand = true
		}
	}
	return hasRunOrCommand
}

func exportsOrchestration(ev PackageEvidence) bool {
	for _, name := range ev.ExportedFuncs {
		if name == "Collect" || name == "Build" || name == "Assemble" {
			return true
		}
	}
	for _, name := range ev.ExportedMethods {
		base := methodBase(name)
		if base == "Collect" || base == "Build" || base == "Assemble" {
			return true
		}
	}
	return false
}

func looksLikeConfig(ev PackageEvidence) bool {
	hasLoad, hasSave := false, false
	for _, name := range ev.ExportedFuncs {
		switch name {
		case "Load", "Init":
			hasLoad = true
		case "Save":
			hasSave = true
		}
	}
	return hasLoad && (hasSave || ev.JSONTags)
}

func exportsClientSurface(ev PackageEvidence) bool {
	for _, name := range ev.ExportedDecls {
		if name == "Client" || strings.HasSuffix(name, "Client") {
			return true
		}
	}
	for _, name := range ev.ExportedFuncs {
		if name == "New" || strings.HasPrefix(name, "New") {
			return true
		}
	}
	return false
}

func methodBase(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func countInternalImports(path string, graph map[string][]string) int {
	want := normalizePath(path)
	for key, outs := range graph {
		keyNorm := normalizePath(strings.TrimPrefix(key, "./"))
		if keyNorm != want {
			continue
		}
		n := 0
		for _, out := range outs {
			if strings.TrimSpace(out) != "" {
				n++
			}
		}
		return n
	}
	return 0
}

func labelEdges(graph map[string][]string, byPath map[string]RoleNode) []RoleEdge {
	var edges []RoleEdge
	for from, tos := range graph {
		fromPath := normalizePath(strings.TrimPrefix(from, "./"))
		fromNode, okFrom := byPath[fromPath]
		if !okFrom {
			continue
		}
		for _, to := range tos {
			toPath := normalizePath(strings.TrimPrefix(to, "./"))
			toNode, okTo := byPath[toPath]
			if !okTo {
				continue
			}
			kind := EdgeImports
			switch {
			case toNode.Role == RoleDTO:
				kind = EdgeFillsDTO
			case toNode.Role == RoleExecRunner:
				kind = EdgeUsesRunner
			case toNode.Role == RoleConfig:
				kind = EdgeReadsConfig
			case fromNode.Role == RoleHTTPSurface && (toNode.Role == RoleAggregator || toNode.Role == RoleAdapter):
				kind = EdgeServesHTTP
			case fromNode.Role == RoleAggregator:
				kind = EdgeComposes
			case fromNode.Role == RoleEntrypoint && toNode.Role == RoleHTTPSurface:
				kind = EdgeServesHTTP
			}
			edges = append(edges, RoleEdge{From: fromPath, To: toPath, Kind: kind})
		}
	}
	return edges
}

func revisitNodes(nodes []RoleNode, edges []RoleEdge, byPath map[string]RoleNode) []RoleNode {
	importersOf := map[string]map[string]struct{}{}
	for _, e := range edges {
		to := byPath[e.To]
		if to.Role != RoleDTO && e.Kind != EdgeFillsDTO {
			continue
		}
		if importersOf[e.To] == nil {
			importersOf[e.To] = map[string]struct{}{}
		}
		importersOf[e.To][e.From] = struct{}{}
	}
	out := make([]RoleNode, len(nodes))
	copy(out, nodes)
	for i := range out {
		n := &out[i]
		if n.Role != RoleDTO {
			continue
		}
		if len(importersOf[n.Path]) >= 2 {
			n.Evidence = appendUnique(n.Evidence, "shared_dto_multi_importer")
		}
	}
	return out
}

func appendUnique(list []string, item string) []string {
	for _, v := range list {
		if v == item {
			return list
		}
	}
	return append(list, item)
}

// FormatRoleTopologyMarkdown renders a compact role map for LLMs.
func FormatRoleTopologyMarkdown(topo RoleTopology) string {
	var b strings.Builder
	b.WriteString("# Observed package roles\n\n")
	b.WriteString("Roles come from AST, imports, and interfaces. Folder names are not evidence.\n")
	b.WriteString("Use this topology before clustering. Do not invent ownership from path words.\n\n")
	for _, n := range topo.Packages {
		display := "./" + strings.TrimPrefix(n.Path, "./")
		fmt.Fprintf(&b, "## %s\n", display)
		fmt.Fprintf(&b, "- role: %s\n", n.Role)
		fmt.Fprintf(&b, "- confidence: %.2f\n", n.Confidence)
		if n.CandidateRole != "" && n.Role == RoleUnknown {
			fmt.Fprintf(&b, "- candidate_role: %s\n", n.CandidateRole)
		}
		fmt.Fprintf(&b, "- inspected_stage: %d\n", n.InspectedStage)
		if len(n.Evidence) > 0 {
			fmt.Fprintf(&b, "- evidence: %s\n", strings.Join(n.Evidence, ", "))
		}
		b.WriteByte('\n')
	}
	if len(topo.Edges) > 0 {
		b.WriteString("## Edges\n\n")
		for _, e := range topo.Edges {
			fmt.Fprintf(&b, "- %s -> %s (%s)\n", e.From, e.To, e.Kind)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// WritePackageRolesFile builds roles from modules and writes package_roles.yaml.
func WritePackageRolesFile(repoRoot string, modules []gorepo.Module, outPath string, importGraph map[string][]string) error {
	out := strings.TrimSpace(outPath)
	if out == "" {
		return terrors.New(terrors.CodeInvalid, "sourceindex.WritePackageRolesFile", "out path empty")
	}
	idx, err := BuildInModules(repoRoot, modules)
	if err != nil {
		return err
	}
	topo := BuildRoleTopology(idx, importGraph)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WritePackageRolesFile", "mkdir roles dir").
			With("path", out)
	}
	data, err := yaml.Marshal(topo)
	if err != nil {
		return terrors.Wrap(err, terrors.CodeInternal, "sourceindex.WritePackageRolesFile", "marshal roles").
			With("path", out)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WritePackageRolesFile", "write roles file").
			With("path", out)
	}
	return nil
}

// WritePackageRolesMarkdownFile writes the markdown companion next to the YAML roles file.
func WritePackageRolesMarkdownFile(repoRoot string, modules []gorepo.Module, outPath string, importGraph map[string][]string) error {
	out := strings.TrimSpace(outPath)
	if out == "" {
		return terrors.New(terrors.CodeInvalid, "sourceindex.WritePackageRolesMarkdownFile", "out path empty")
	}
	idx, err := BuildInModules(repoRoot, modules)
	if err != nil {
		return err
	}
	topo := BuildRoleTopology(idx, importGraph)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WritePackageRolesMarkdownFile", "mkdir").
			With("path", out)
	}
	return os.WriteFile(out, []byte(FormatRoleTopologyMarkdown(topo)), 0o644)
}

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
	RoleEntrypoint    = "entrypoint"
	RoleHTTPSurface   = "http_surface"
	RoleDTO           = "dto"
	RoleExecRunner    = "exec_runner"
	RoleAggregator    = "aggregator"
	RoleAdapter       = "adapter"
	RoleConfig        = "config"
	RoleObservability = "observability"
	RoleUnknown       = "unknown"
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
// Stage 1 uses isolated AST/import-prefix facts; Stage 2 uses graph math.
// Folder/path names and English function names are never used as evidence.
func BuildRoleTopology(idx Index, importGraph map[string][]string) RoleTopology {
	paths := make([]string, 0, len(idx.Packages))
	for p := range idx.Packages {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	byPath := make(map[string]RoleNode, len(paths))

	// Pass 1: Stage 1 unique isolated facts.
	for _, p := range paths {
		if node, ok := classifyStage1(idx.Packages[p]); ok {
			byPath[p] = node
		}
	}

	// Pass 2a: exec_runner (direct os/exec only).
	for _, p := range paths {
		if _, labeled := byPath[p]; labeled {
			continue
		}
		ev := idx.Packages[p]
		if !ev.ImportsOsExec {
			continue
		}
		byPath[p] = RoleNode{
			Path: normalizePath(p), Role: RoleExecRunner, Confidence: confidenceStage2,
			Evidence: []string{"imports_os_exec"}, InspectedStage: 2,
		}
	}

	// Pass 2b: config (yaml/json codec + tags, no domain internals, not observability).
	for _, p := range paths {
		if _, labeled := byPath[p]; labeled {
			continue
		}
		ev := idx.Packages[p]
		if !isConfigEvidence(ev) {
			continue
		}
		if domainImportCount(p, importGraph, byPath) != 0 {
			continue
		}
		evidence := []string{}
		if ev.ImportsYAML {
			evidence = append(evidence, "imports_yaml")
		}
		if ev.ImportsEncodingJSON {
			evidence = append(evidence, "imports_encoding_json")
		}
		if ev.YAMLTags {
			evidence = append(evidence, "yaml_tags")
		}
		if ev.JSONTags {
			evidence = append(evidence, "json_tags")
		}
		byPath[p] = RoleNode{
			Path: normalizePath(p), Role: RoleConfig, Confidence: confidenceStage2,
			Evidence: evidence, InspectedStage: 2,
		}
	}

	// Pass 2c: adapter vs aggregator from graph remaining after subtract.
	for _, p := range paths {
		if _, labeled := byPath[p]; labeled {
			continue
		}
		ev := idx.Packages[p]
		remaining := domainImportCount(p, importGraph, byPath)
		importsRunner := importsRole(p, importGraph, byPath, RoleExecRunner)
		candidates := make([]roleCandidate, 0, 2)

		adapterSignal := (ev.ImportsNetHTTP && !ev.GoEmbed && !ev.HTTPSurfaceIdent) || importsRunner
		if adapterSignal && remaining == 0 {
			evidence := []string{}
			if ev.ImportsNetHTTP {
				evidence = append(evidence, "imports_net_http", "not_http_surface")
			}
			if importsRunner {
				evidence = append(evidence, "imports_exec_runner")
			}
			evidence = append(evidence, "domain_imports_eq_0")
			candidates = append(candidates, roleCandidate{
				Role: RoleAdapter, Stage: 2, Confidence: confidenceStage2, Evidence: evidence,
			})
		}

		hasLogic := len(ev.ExportedFuncs) > 0 || len(ev.ExportedMethods) > 0
		if hasLogic && remaining >= 1 {
			candidates = append(candidates, roleCandidate{
				Role: RoleAggregator, Stage: 2, Confidence: confidenceStage2,
				Evidence: []string{"exported_logic", "domain_imports_ge_1"},
			})
		}

		byPath[p] = publishNode(normalizePath(p), candidates)
	}

	nodes := make([]RoleNode, 0, len(paths))
	for _, p := range paths {
		nodes = append(nodes, byPath[p])
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

func classifyStage1(ev PackageEvidence) (RoleNode, bool) {
	path := normalizePath(ev.Path)
	if ev.HasMain {
		return RoleNode{
			Path: path, Role: RoleEntrypoint, Confidence: confidenceStage1,
			Evidence: []string{"has_main"}, InspectedStage: 1,
		}, true
	}
	if ev.ImportsOTel || ev.ImportsPrometheus {
		evidence := []string{}
		if ev.ImportsOTel {
			evidence = append(evidence, "imports_otel")
		}
		if ev.ImportsPrometheus {
			evidence = append(evidence, "imports_prometheus")
		}
		return RoleNode{
			Path: path, Role: RoleObservability, Confidence: confidenceStage1,
			Evidence: evidence, InspectedStage: 1,
		}, true
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
			evidence = append(evidence, "serve_http")
		}
		return RoleNode{
			Path: path, Role: RoleHTTPSurface, Confidence: confidenceStage1,
			Evidence: evidence, InspectedStage: 1,
		}, true
	}
	if ev.JSONTags && len(ev.ExportedFuncs) == 0 && len(ev.ExportedMethods) == 0 {
		return RoleNode{
			Path: path, Role: RoleDTO, Confidence: confidenceStage1,
			Evidence: []string{"json_tags", "no_exported_funcs", "no_exported_methods"},
			InspectedStage: 1,
		}, true
	}
	return RoleNode{}, false
}

func isConfigEvidence(ev PackageEvidence) bool {
	if ev.ImportsOTel || ev.ImportsPrometheus {
		return false
	}
	codec := ev.ImportsYAML || ev.ImportsEncodingJSON
	tags := ev.YAMLTags || ev.JSONTags
	return codec && tags
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

func graphOuts(path string, graph map[string][]string) []string {
	want := normalizePath(path)
	for key, outs := range graph {
		keyNorm := normalizePath(strings.TrimPrefix(key, "./"))
		if keyNorm != want {
			continue
		}
		var cleaned []string
		for _, out := range outs {
			out = normalizePath(strings.TrimSpace(out))
			if out == "" {
				continue
			}
			cleaned = append(cleaned, out)
		}
		return cleaned
	}
	return nil
}

// domainImportCount counts internal imports that are not dto/config/observability/exec_runner.
func domainImportCount(path string, graph map[string][]string, byPath map[string]RoleNode) int {
	n := 0
	for _, to := range graphOuts(path, graph) {
		role := byPath[normalizePath(to)].Role
		switch role {
		case RoleDTO, RoleConfig, RoleObservability, RoleExecRunner:
			continue
		default:
			n++
		}
	}
	return n
}

func importsRole(path string, graph map[string][]string, byPath map[string]RoleNode, role string) bool {
	for _, to := range graphOuts(path, graph) {
		if byPath[normalizePath(to)].Role == role {
			return true
		}
	}
	return false
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

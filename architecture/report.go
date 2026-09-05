// Package architecture builds human-readable projections of a Typology catalog
// and the Go repository it describes.
package architecture

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/behaviorengineering/typology/catalog"
	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/discover"
	"github.com/behaviorengineering/typology/internal/gorepo"
	"github.com/behaviorengineering/typology/validate"
)

// DefaultReportRel is the default human-readable architecture brief path.
const DefaultReportRel = "docs/architecture/typology.md"

// GeneratedMarker identifies a report that Typology may refresh.
const GeneratedMarker = "<!-- typology:generated -->"

// BuildOptions configures an architecture report.
type BuildOptions struct {
	RepoRoot    string
	Catalog     catalog.Typology
	SkipImports bool
	Modules     []string
	Module      string
}

// Report combines the intended catalog with observed repository evidence.
type Report struct {
	Catalog  catalog.Typology `json:"catalog"`
	Modules  []string         `json:"modules,omitempty"`
	Graph    Graph            `json:"graph"`
	Findings []catalog.Issue  `json:"findings,omitempty"`
}

// Graph is the public architecture view of the observed import graph.
type Graph struct {
	Nodes            map[string]Node   `json:"nodes"`
	Leaves           []string          `json:"leaves,omitempty"`
	Hubs             []string          `json:"hubs,omitempty"`
	MergeSuggestions []MergeSuggestion `json:"mergeSuggestions,omitempty"`
	StemCollisions   []StemCollision   `json:"stemCollisions,omitempty"`
	PlatformLeaves   []string          `json:"platformLeaves,omitempty"`
}

// Node describes one observed Go package.
type Node struct {
	Path         string   `json:"path"`
	InDegree     int      `json:"inDegree"`
	OutDegree    int      `json:"outDegree"`
	Imports      []string `json:"imports,omitempty"`
	ImportedBy   []string `json:"importedBy,omitempty"`
	IsLeaf       bool     `json:"isLeaf"`
	IsHub        bool     `json:"isHub"`
	SoleImporter string   `json:"soleImporter,omitempty"`
}

// MergeSuggestion identifies a package that may belong with its sole importer.
type MergeSuggestion struct {
	SourcePackage string `json:"sourcePackage"`
	TargetPackage string `json:"targetPackage"`
	Reason        string `json:"reason"`
}

// StemCollision identifies similarly named packages with different callers.
type StemCollision struct {
	Stem     string   `json:"stem"`
	Packages []string `json:"packages"`
	Warning  string   `json:"warning"`
}

// Build observes the repository and records validation findings without
// treating those findings as narrative conclusions.
func Build(opts BuildOptions) (Report, error) {
	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		return Report{}, terrors.New(terrors.CodeInvalid, "architecture.Build", "repo root empty")
	}
	selectors := opts.Catalog.Scope.Modules
	if len(opts.Modules) > 0 {
		selectors = opts.Modules
	}
	modules, err := gorepo.ResolveModules(repo, selectors, opts.Module)
	if err != nil {
		return Report{}, terrors.Wrap(err, terrors.CodeFailedPrecondition, "architecture.Build", "resolve module scope")
	}
	graph, err := discover.AnalyzeGraphInModules(repo, modules)
	if err != nil {
		return Report{}, terrors.Wrap(err, terrors.CodeUnavailable, "architecture.Build", "analyze import graph")
	}
	findings := validate.Run(validate.Options{
		RepoRoot:    repo,
		Catalog:     opts.Catalog,
		SkipImports: opts.SkipImports,
		Modules:     moduleRels(modules),
	})
	return Report{
		Catalog:  opts.Catalog,
		Modules:  moduleRels(modules),
		Graph:    publicGraph(graph),
		Findings: findings,
	}, nil
}

func moduleRels(modules []gorepo.Module) []string {
	rels := make([]string, 0, len(modules))
	for _, module := range modules {
		rels = append(rels, filepath.ToSlash(module.Rel))
	}
	sort.Strings(rels)
	return rels
}

// RenderMarkdown renders a deterministic human-readable architecture brief.
func RenderMarkdown(report Report) (string, error) {
	data := prepareMarkdown(report)
	tmpl, err := template.New("architecture").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(markdownTemplate)
	if err != nil {
		return "", terrors.Wrap(err, terrors.CodeInternal, "architecture.RenderMarkdown", "parse template")
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", terrors.Wrap(err, terrors.CodeInternal, "architecture.RenderMarkdown", "execute template")
	}
	return out.String(), nil
}

// WriteMarkdown writes a generated brief unless a human owns the file. It
// returns false when it preserves an existing file without the marker.
func WriteMarkdown(path, body string) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, terrors.New(terrors.CodeInvalid, "architecture.WriteMarkdown", "output path empty")
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, terrors.Wrap(err, terrors.CodeUnavailable, "architecture.WriteMarkdown", "read existing report").
			With("path", path)
	}
	if len(existing) > 0 && !strings.Contains(string(existing), GeneratedMarker) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, terrors.Wrap(err, terrors.CodeUnavailable, "architecture.WriteMarkdown", "mkdir report directory").
			With("path", path)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return false, terrors.Wrap(err, terrors.CodeUnavailable, "architecture.WriteMarkdown", "write report").
			With("path", path)
	}
	return true, nil
}

type markdownData struct {
	Catalog        catalog.Typology
	Modules        []string
	Slices         []sliceRow
	SliceBindings  []catalog.SliceBinding
	ComponentBinds []catalog.ComponentBinding
	Graph          graphData
	Findings       []findingRow
	MermaidLines   []string
}

type sliceRow struct {
	ID         string
	Objective  string
	Route      string
	Components []string
	Surfaces   []string
	Programs   []string
}

type graphData struct {
	NodeCount        int
	PackagePaths     []string
	HubRows          []graphNodeRow
	LeafRows         []graphNodeRow
	MergeSuggestions []MergeSuggestion
	StemCollisions   []StemCollision
	PlatformLeaves   []string
}

type graphNodeRow struct {
	Path      string
	InDegree  int
	OutDegree int
}

type findingRow struct {
	Slice   string
	Message string
}

func prepareMarkdown(report Report) markdownData {
	data := markdownData{
		Catalog:        report.Catalog,
		Modules:        append([]string(nil), report.Modules...),
		SliceBindings:  append([]catalog.SliceBinding(nil), report.Catalog.SliceBindings...),
		ComponentBinds: append([]catalog.ComponentBinding(nil), report.Catalog.ComponentBindings...),
		Findings:       make([]findingRow, 0, len(report.Findings)),
		Graph: graphData{
			NodeCount:        len(report.Graph.Nodes),
			MergeSuggestions: append([]MergeSuggestion(nil), report.Graph.MergeSuggestions...),
			StemCollisions:   append([]StemCollision(nil), report.Graph.StemCollisions...),
			PlatformLeaves:   append([]string(nil), report.Graph.PlatformLeaves...),
		},
	}
	packagePaths := make([]string, 0, len(report.Graph.Nodes))
	for path := range report.Graph.Nodes {
		packagePaths = append(packagePaths, path)
	}
	sort.Strings(packagePaths)
	data.Graph.PackagePaths = packagePaths
	for _, s := range report.Catalog.Slices {
		row := sliceRow{ID: s.ID, Objective: s.Objective, Route: s.Route}
		for _, component := range s.AllComponents() {
			row.Components = append(row.Components, component.Path)
		}
		for _, surface := range s.Surfaces {
			row.Surfaces = append(row.Surfaces, surface.ID+" ("+string(surface.Kind)+")")
		}
		for _, program := range s.Subprograms {
			row.Programs = append(row.Programs, program.ID)
		}
		for _, actuator := range s.Actuators {
			row.Programs = append(row.Programs, actuator.ID+" (actuator)")
		}
		data.Slices = append(data.Slices, row)
	}
	for _, finding := range report.Findings {
		data.Findings = append(data.Findings, findingRow{Slice: finding.Slice, Message: finding.Message})
	}
	for _, path := range report.Graph.Hubs {
		if node, ok := report.Graph.Nodes[path]; ok {
			data.Graph.HubRows = append(data.Graph.HubRows, graphNodeRow{
				Path: path, InDegree: node.InDegree, OutDegree: node.OutDegree,
			})
		}
	}
	for _, path := range report.Graph.Leaves {
		if node, ok := report.Graph.Nodes[path]; ok {
			data.Graph.LeafRows = append(data.Graph.LeafRows, graphNodeRow{
				Path: path, InDegree: node.InDegree, OutDegree: node.OutDegree,
			})
		}
	}
	sort.Slice(data.Graph.MergeSuggestions, func(i, j int) bool {
		if data.Graph.MergeSuggestions[i].SourcePackage != data.Graph.MergeSuggestions[j].SourcePackage {
			return data.Graph.MergeSuggestions[i].SourcePackage < data.Graph.MergeSuggestions[j].SourcePackage
		}
		return data.Graph.MergeSuggestions[i].TargetPackage < data.Graph.MergeSuggestions[j].TargetPackage
	})
	sort.Slice(data.Graph.StemCollisions, func(i, j int) bool {
		return data.Graph.StemCollisions[i].Stem < data.Graph.StemCollisions[j].Stem
	})
	if len(data.Slices) <= 20 {
		data.MermaidLines = mermaidLines(report.Catalog)
	}
	return data
}

func publicGraph(summary discover.GraphSummary) Graph {
	nodes := make(map[string]Node, len(summary.Nodes))
	for path, node := range summary.Nodes {
		nodes[path] = Node{
			Path:         node.Path,
			InDegree:     node.InDegree,
			OutDegree:    node.OutDegree,
			Imports:      append([]string(nil), node.Imports...),
			ImportedBy:   append([]string(nil), node.ImportedBy...),
			IsLeaf:       node.IsLeaf,
			IsHub:        node.IsHub,
			SoleImporter: node.SoleImporter,
		}
	}
	merges := make([]MergeSuggestion, 0, len(summary.MergeSuggestions))
	for _, suggestion := range summary.MergeSuggestions {
		merges = append(merges, MergeSuggestion{
			SourcePackage: suggestion.SourcePackage,
			TargetPackage: suggestion.TargetPackage,
			Reason:        suggestion.Reason,
		})
	}
	collisions := make([]StemCollision, 0, len(summary.StemCollisions))
	for _, collision := range summary.StemCollisions {
		collisions = append(collisions, StemCollision{
			Stem: collision.Stem, Packages: append([]string(nil), collision.Packages...), Warning: collision.Warning,
		})
	}
	return Graph{
		Nodes:            nodes,
		Leaves:           append([]string(nil), summary.Leaves...),
		Hubs:             append([]string(nil), summary.Hubs...),
		MergeSuggestions: merges,
		StemCollisions:   collisions,
		PlatformLeaves:   append([]string(nil), summary.PlatformLeaves...),
	}
}

func mermaidLines(t catalog.Typology) []string {
	lines := make([]string, 0, len(t.Slices)+len(t.SliceBindings))
	for _, s := range t.Slices {
		lines = append(lines, "  "+mermaidID(s.ID)+"[\""+mermaidLabel(s.ID)+"\"]")
	}
	for _, binding := range t.SliceBindings {
		lines = append(lines, "  "+mermaidID(binding.From)+" -->|"+mermaidLabel(string(binding.Kind))+"| "+mermaidID(binding.To))
	}
	return lines
}

func mermaidID(value string) string {
	var b strings.Builder
	b.WriteString("slice_")
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func mermaidLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

const markdownTemplate = `# Typology Architecture Brief

` + GeneratedMarker + `

This document is a human-readable projection of the confirmed Typology catalog and the observed Go repository. The catalog remains the machine source of truth. This brief helps people inspect whether the design matches the code.

## How to read this brief

The **intended architecture** comes from the catalog. The **observed topology** comes from the Go import graph. Findings name evidence that needs an agent or architect to fix or record as boundary debt. Typology does not infer a final design decision from a finding.

## Intended architecture

{{if .MermaidLines}}### Bounded-context map

` + "```mermaid" + `
flowchart LR
{{range .MermaidLines}}{{.}}
{{end}}` + "```" + `
{{end}}

### Bounded contexts

| Slice | Objective | Route |
|-------|-----------|-------|
{{range .Slices}}| ` + "`{{.ID}}`" + ` | {{.Objective}} | {{.Route}} |
{{else}}| _(none)_ | | |
{{end}}

### Context details

{{range .Slices}}#### ` + "`{{.ID}}`" + `

{{.Objective}}

- Packages: {{join .Components ", "}}
{{- if .Surfaces}}
- Surfaces: {{join .Surfaces ", "}}
{{- else}}
- Surfaces: _(none)_
{{- end}}
{{- if .Programs}}
- Programs: {{join .Programs ", "}}
{{- else}}
- Programs: _(none)_
{{- end}}

{{end}}
### Declared coupling

{{if .SliceBindings}}| From | To | Kind |
|------|----|------|
{{range .SliceBindings}}| ` + "`{{.From}}`" + ` | ` + "`{{.To}}`" + ` | {{.Kind}} |
{{end}}{{else}}No slice bindings are declared.
{{end}}

{{if .ComponentBinds}}| Component | Component | Rule |
|-----------|-----------|------|
{{range .ComponentBinds}}| ` + "`{{.From}}`" + ` | ` + "`{{.To}}`" + ` | {{.Rule}} |
{{end}}{{else}}No component bindings are declared.
{{end}}

## Observed topology

The repository contains **{{.Graph.NodeCount}} Go packages** in the inspected modules:
{{range .Graph.PackagePaths}}- ` + "`{{.}}`" + `
{{end}}

{{if .Graph.HubRows}}### High-coupling packages

| Package | In-degree | Out-degree |
|---------|-----------|------------|
{{range .Graph.HubRows}}| ` + "`{{.Path}}`" + ` | {{.InDegree}} | {{.OutDegree}} |
{{end}}{{end}}
{{if .Graph.LeafRows}}### Leaf packages

Leaf packages have no outgoing local imports. They may be valid infrastructure leaves or packages that should sit inside their only caller.

| Package | Imported by |
|---------|-------------|
{{range .Graph.LeafRows}}| ` + "`{{.Path}}`" + ` | {{.InDegree}} |
{{end}}{{end}}
{{if .Graph.PlatformLeaves}}### Shared platform leaves

{{join .Graph.PlatformLeaves ", "}}

{{end}}
{{if .Graph.MergeSuggestions}}### Merge candidates

These are heuristics for review, not automatic moves.

{{range .Graph.MergeSuggestions}}- ` + "`{{.SourcePackage}}`" + ` may sit with ` + "`{{.TargetPackage}}`" + `: {{.Reason}}
{{end}}{{end}}
{{if .Graph.StemCollisions}}### Similar package stems

{{range .Graph.StemCollisions}}- ` + "`{{.Stem}}`" + `: {{join .Packages ", "}}. {{.Warning}}
{{end}}{{end}}

## Drift and design questions

{{if .Findings}}The following findings need a correction or an explicit boundary-debt decision:

{{range .Findings}}- {{if .Slice}}` + "`{{.Slice}}`" + `: {{.Message}}{{else}}{{.Message}}{{end}}
{{end}}{{else}}No catalog, path, import, or documentation findings were reported.
{{end}}

## Agent review protocol

1. Read the relevant catalog rows and the evidence named by each finding.
2. Fix the code or catalog when the boundary is wrong.
3. Record a temporary boundary decision in the Typology journey when the design needs a later refactor.
4. Re-run ` + "`typology architecture REPO`" + ` and ` + "`typology validate REPO`" + `.
5. Remove the generated marker only after a human accepts the narrative as a reviewed explanation.
`

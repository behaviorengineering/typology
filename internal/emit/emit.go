package emit

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/behaviorengineering/typology/catalog"
	terrors "github.com/behaviorengineering/typology/errors"
)

var tmplFuncs = template.FuncMap{
	"sliceBindings": sliceBindingsFor,
	"compBindings":  compBindingsFor,
	"join":          strings.Join,
	"compPaths": func(comps []catalog.Component) string {
		parts := make([]string, 0, len(comps))
		for _, c := range comps {
			if c.Path != "" {
				parts = append(parts, c.Path)
			}
		}
		return strings.Join(parts, ", ")
	},
	"yamlQuote": strconv.Quote,
	"apiComponents": func(s catalog.Slice) []catalog.Component {
		if surf, ok := s.SurfaceByKind(catalog.InteractionAPI); ok {
			return surf.Components
		}
		return nil
	},
	"cliComponents": func(s catalog.Slice) []catalog.Component {
		if surf, ok := s.SurfaceByKind(catalog.InteractionCLI); ok {
			return surf.Components
		}
		return nil
	},
	"uiComponents": func(s catalog.Slice) []catalog.Component {
		if surf, ok := s.SurfaceByKind(catalog.InteractionUI); ok {
			return surf.Components
		}
		return nil
	},
}

// Options configures emit.
type Options struct {
	RepoRoot string
	Catalog  catalog.Typology
	DocsOnly bool
	GoOnly   bool
}

// Run writes catalog YAML and DocPage skeletons.
func Run(opts Options) error {
	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		return terrors.New(terrors.CodeInvalid, "emit.Run", "repo root empty")
	}
	if !opts.DocsOnly {
		catalogPath := filepath.Join(repo, filepath.FromSlash(catalog.DefaultCatalogRel))
		if err := catalog.SaveYAML(catalogPath, opts.Catalog); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "emit.Run", "save catalog").
				With("path", catalogPath)
		}
		if err := emitTypologyReadme(repo); err != nil {
			return err
		}
		if err := emitAgentsPointer(repo); err != nil {
			return err
		}
		if err := emitToolsIndex(repo, opts.Catalog); err != nil {
			return err
		}
	}
	if !opts.GoOnly {
		for _, s := range opts.Catalog.Slices {
			if err := emitSliceDocs(repo, s, opts.Catalog); err != nil {
				return terrors.Wrap(err, terrors.CodeUnavailable, "emit.Run", "emit slice docs").
					With("slice", s.ID)
			}
		}
	}
	return nil
}

func emitTypologyReadme(repoRoot string) error {
	body, err := renderGenerated("typology-readme", typologyReadmeTmpl, nil)
	if err != nil {
		return terrors.Wrap(err, terrors.CodeInternal, "emit.Run", "render typology readme")
	}
	return writeGenerated(repoRoot, ".typology/README.md", body)
}

func emitAgentsPointer(repoRoot string) error {
	const marker = "<!-- typology:generated -->"
	section := "\n" + marker + "\n\nFor guidance on architecture, code structure, and validation using Typology principles, load the skills listed in [.typology/README.md](.typology/README.md).\n"
	fullFile := "# Agents\n" + section

	abs := filepath.Join(repoRoot, "AGENTS.md")
	existing, err := os.ReadFile(abs)
	if err != nil && !os.IsNotExist(err) {
		return terrors.Wrap(err, terrors.CodeUnavailable, "emit.emitAgentsPointer", "read AGENTS.md").
			With("path", abs)
	}
	if os.IsNotExist(err) {
		if err := os.WriteFile(abs, []byte(fullFile), 0o644); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "emit.emitAgentsPointer", "write AGENTS.md").
				With("path", abs)
		}
		return nil
	}
	if bytes.Contains(existing, []byte(marker)) {
		return nil
	}
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		existing = append(existing, '\n')
	}
	if err := os.WriteFile(abs, append(existing, []byte(section)...), 0o644); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "emit.emitAgentsPointer", "append AGENTS.md section").
			With("path", abs)
	}
	return nil
}

func emitToolsIndex(repoRoot string, t catalog.Typology) error {
	body, err := renderGenerated("tools", toolsTmpl, toolsData{
		Typology: t.ID,
		Tools:    cliTools(t),
	})
	if err != nil {
		return terrors.Wrap(err, terrors.CodeInternal, "emit.emitToolsIndex", "render tools index")
	}
	return writeGeneratedWithMarker(repoRoot, ".typology/tools.yaml", body, "# typology:generated")
}

func cliTools(t catalog.Typology) []cliTool {
	var tools []cliTool
	for _, s := range t.Slices {
		subprograms := make(map[string]string, len(s.Subprograms))
		for _, sp := range s.Subprograms {
			subprograms[sp.ID] = sp.Objective
		}
		actuators := make(map[string]string, len(s.Actuators))
		for _, a := range s.Actuators {
			actuators[a.ID] = a.Objective
		}
		for _, run := range s.OpRuns {
			command := strings.TrimSpace(run.CLI)
			if command == "" {
				continue
			}
			tool := cliTool{
				ID:             run.ID,
				Command:        command,
				Slice:          s.ID,
				OwnerComponent: run.OwnerComponent,
				Gate:           string(run.Gate),
				Runs:           run.Runs,
				Actuates:       run.Actuates,
			}
			switch {
			case run.Runs != "":
				tool.Summary = subprograms[run.Runs]
			case run.Actuates != "":
				tool.Summary = actuators[run.Actuates]
			}
			tools = append(tools, tool)
		}
	}
	return tools
}

func emitSliceDocs(repoRoot string, s catalog.Slice, t catalog.Typology) error {
	pages := s.Docs.Pages
	if len(pages) == 0 {
		pages = catalog.DefaultDocCluster(s, catalog.DefaultDocsRoot).Pages
	}
	for _, page := range pages {
		body, err := renderPage(s, page, t)
		if err != nil {
			return err
		}
		if err := writeGenerated(repoRoot, page.Path, body); err != nil {
			return err
		}
	}
	if err := writeGenerated(repoRoot, catalog.SliceReadmePath(s.ID, catalog.DefaultDocsRoot), renderReadme(s, pages)); err != nil {
		return err
	}
	for _, sp := range s.Subprograms {
		path := catalog.SubprogramPagePath(s.ID, sp.ID, catalog.DefaultDocsRoot)
		body, err := renderGenerated("subprogram", subprogramTmpl, subprogramData{Slice: s, Subprogram: sp})
		if err != nil {
			return err
		}
		if err := writeGenerated(repoRoot, path, body); err != nil {
			return err
		}
	}
	for _, a := range s.Actuators {
		path := catalog.ActuatorPagePath(s.ID, a.ID, catalog.DefaultDocsRoot)
		body, err := renderGenerated("actuator", actuatorTmpl, actuatorData{Slice: s, Actuator: a})
		if err != nil {
			return err
		}
		if err := writeGenerated(repoRoot, path, body); err != nil {
			return err
		}
	}
	return nil
}

func writeGenerated(repoRoot, rel, body string) error {
	return writeGeneratedWithMarker(repoRoot, rel, body, "<!-- typology:generated -->")
}

func writeGeneratedWithMarker(repoRoot, rel, body, marker string) error {
	abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "emit.writeGenerated", "mkdir docs").
			With("path", rel)
	}
	existing, err := os.ReadFile(abs)
	if err != nil && !os.IsNotExist(err) {
		return terrors.Wrap(err, terrors.CodeUnavailable, "emit.writeGenerated", "read existing page").
			With("path", rel)
	}
	if len(existing) > 0 && !strings.Contains(string(existing), marker) {
		return nil
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "emit.writeGenerated", "write page").
			With("path", rel)
	}
	return nil
}

type pageData struct {
	S catalog.Slice
	T catalog.Typology
}

type subprogramData struct {
	Slice      catalog.Slice
	Subprogram catalog.Subprogram
}

type actuatorData struct {
	Slice    catalog.Slice
	Actuator catalog.Actuator
}

type toolsData struct {
	Typology string
	Tools    []cliTool
}

type cliTool struct {
	ID             string
	Command        string
	Summary        string
	Slice          string
	OwnerComponent string
	Gate           string
	Runs           string
	Actuates       string
}

func renderGenerated(name, tmpl string, data any) (string, error) {
	parsed, err := template.New(name).Funcs(tmplFuncs).Parse(tmpl)
	if err != nil {
		return "", terrors.Wrap(err, terrors.CodeInternal, "emit.renderGenerated", "parse template").
			With("name", name)
	}
	var b strings.Builder
	if err := parsed.Execute(&b, data); err != nil {
		return "", terrors.Wrap(err, terrors.CodeInternal, "emit.renderGenerated", "execute template").
			With("name", name)
	}
	return b.String(), nil
}

func renderPage(s catalog.Slice, page catalog.DocPage, t catalog.Typology) (string, error) {
	data := pageData{S: s, T: t}
	var tmpl string
	switch page.Kind {
	case catalog.DocOverview:
		tmpl = overviewTmpl
	case catalog.DocComponents:
		tmpl = componentsTmpl
	case catalog.DocContracts:
		tmpl = contractsTmpl
	case catalog.DocCLI:
		tmpl = cliTmpl
	case catalog.DocPresentation:
		tmpl = presentationTmpl
	case catalog.DocPipelines:
		tmpl = pipelinesTmpl
	default:
		return "", terrors.New(terrors.CodeInvalid, "emit.renderPage", "unknown doc kind").
			With("kind", string(page.Kind))
	}
	return renderGenerated(string(page.Kind), tmpl, data)
}

func hasPageKind(pages []catalog.DocPage, kind catalog.DocPageKind) bool {
	for _, p := range pages {
		if p.Kind == kind {
			return true
		}
	}
	return false
}

func pageRel(pages []catalog.DocPage, kind catalog.DocPageKind) string {
	for _, p := range pages {
		if p.Kind == kind {
			base := filepath.Base(filepath.FromSlash(p.Path))
			return base
		}
	}
	return string(kind) + ".md"
}

func writeTreeChildren(b *strings.Builder, items []string, indent string) {
	for i, item := range items {
		prefix := indent + "├── "
		if i == len(items)-1 {
			prefix = indent + "└── "
		}
		b.WriteString(prefix)
		b.WriteString(item)
		b.WriteString("\n")
	}
}

func renderReadme(s catalog.Slice, pages []catalog.DocPage) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(s.ID)
	b.WriteString(" (develop)\n\n")
	b.WriteString("<!-- typology:generated -->\n\n")
	if strings.TrimSpace(s.Objective) != "" {
		b.WriteString(strings.TrimSpace(s.Objective))
		b.WriteString("\n\n")
	}
	b.WriteString("Human map follows Typology (slice → owns → subprograms → surfaces). DocPageKind files are leaves.\n\n")
	b.WriteString("```text\nSlice\n")
	overview := pageRel(pages, catalog.DocOverview)
	owns := pageRel(pages, catalog.DocComponents)
	b.WriteString("├── Overview          [")
	b.WriteString(overview)
	b.WriteString("](")
	b.WriteString(overview)
	b.WriteString(")\n")
	b.WriteString("├── Owns              [")
	b.WriteString(owns)
	b.WriteString("](")
	b.WriteString(owns)
	b.WriteString(")\n")
	if len(s.Subprograms) > 0 {
		b.WriteString("├── Subprograms\n")
		items := make([]string, 0, len(s.Subprograms))
		for _, sp := range s.Subprograms {
			rel := "subprograms/" + sp.ID + ".md"
			items = append(items, "["+sp.ID+"]("+rel+")")
		}
		writeTreeChildren(&b, items, "│   ")
	}
	if len(s.Actuators) > 0 {
		b.WriteString("├── Actuators\n")
		items := make([]string, 0, len(s.Actuators))
		for _, a := range s.Actuators {
			rel := "actuators/" + a.ID + ".md"
			items = append(items, "["+a.ID+"]("+rel+")")
		}
		writeTreeChildren(&b, items, "│   ")
	}
	surfaceSlots := []struct {
		kind  catalog.DocPageKind
		label string
	}{
		{catalog.DocCLI, "CLI"},
		{catalog.DocPresentation, "UI"},
		{catalog.DocContracts, "API"},
		{catalog.DocPipelines, "Jobs"},
	}
	var present []string
	for _, slot := range surfaceSlots {
		if hasPageKind(pages, slot.kind) {
			rel := pageRel(pages, slot.kind)
			present = append(present, slot.label+"           ["+rel+"]("+rel+")")
		}
	}
	if len(present) > 0 {
		b.WriteString("└── Surfaces\n")
		writeTreeChildren(&b, present, "    ")
	}
	b.WriteString("```\n")
	return b.String()
}

const overviewTmpl = `# Overview

<!-- typology:generated -->

{{if .S.Objective}}{{.S.Objective}}{{else}}Bounded context {{.S.ID}}.{{end}}

{{if .S.Route}}Route: {{.S.Route}}{{end}}

Human map: slice → Overview → Owns → Subprograms → Surfaces (CLI, UI, API, Jobs). See [README.md](README.md). Domain packages and program indexes live on [components.md](components.md).
`

const componentsTmpl = `# Components

<!-- typology:generated -->

## Owns

Typology ` + "`owns[]`" + `: domain packages on this slice.

| Component | Path | Layer |
|-----------|------|-------|
{{range .S.Owns}}| {{.ID}} | {{.Path}} | {{.Layer}} |
{{else}}| _(none)_ | | |
{{end}}

## Surfaces

Typology ` + "`surfaces[]`" + `: interaction packages grouped by kind. Domain packages stay under Owns.

| Surface | Kind | Components |
|---------|------|------------|
{{range .S.Surfaces}}| {{.ID}} | {{.Kind}} | {{compPaths .Components}} |
{{else}}| _(none)_ | | |
{{end}}

{{if .S.Subprograms}}## Subprograms

Standing programs. Input, output, store, and gate live on the subpage.

| Id | Owner | Gate | Page |
|----|-------|------|------|
{{range .S.Subprograms}}| {{.ID}} | {{.OwnerComponent}} | {{.Gate}} | [subprograms/{{.ID}}.md](subprograms/{{.ID}}.md) |
{{end}}
{{end}}{{if .S.Actuators}}## Actuators

Signal in, emit out. Detail lives on the subpage.

| Id | Owner | Gate | Page |
|----|-------|------|------|
{{range .S.Actuators}}| {{.ID}} | {{.OwnerComponent}} | {{.Gate}} | [actuators/{{.ID}}.md](actuators/{{.ID}}.md) |
{{end}}
{{end}}## Cross-slice

| From | To | Kind |
|------|----|------|
{{range sliceBindings .T .S.ID}}| {{.From}} | {{.To}} | {{.Kind}} |
{{else}}| _(none)_ | | |
{{end}}

| From | To | Rule |
|------|----|------|
{{range compBindings .T .S.ID}}| {{.From}} | {{.To}} | {{.Rule}} |
{{else}}| _(none)_ | | |
{{end}}
`

const subprogramTmpl = `# {{.Subprogram.ID}}

<!-- typology:generated -->

{{if .Subprogram.Objective}}{{.Subprogram.Objective}}{{else}}Slice ` + "`{{.Slice.ID}}`" + ` subprogram (set catalog objective before fill).{{end}}

Owner component: ` + "`{{.Subprogram.OwnerComponent}}`" + `.

| Field | Value |
|-------|-------|
| Objective | {{.Subprogram.Objective}} |
| Input | {{.Subprogram.Input}} |
| Output | {{.Subprogram.Output}} |
| Store | {{join .Subprogram.Store ", "}} |
| Gate | {{.Subprogram.Gate}} |
`

const actuatorTmpl = `# {{.Actuator.ID}}

<!-- typology:generated -->

{{if .Actuator.Objective}}{{.Actuator.Objective}}{{else}}Slice ` + "`{{.Slice.ID}}`" + ` actuator (set catalog objective before fill).{{end}}

Owner component: ` + "`{{.Actuator.OwnerComponent}}`" + `.

| Field | Value |
|-------|-------|
| Objective | {{.Actuator.Objective}} |
| Signals | {{join .Actuator.Signals ", "}} |
| Emits | {{join .Actuator.Emits ", "}} |
| Gate | {{.Actuator.Gate}} |
`

const contractsTmpl = `# Contracts

<!-- typology:generated -->

HTTP/API surface for slice {{.S.ID}}. Endpoint types are not modeled in typology v1.

| Component | Path |
|-----------|------|
{{range apiComponents .S}}| {{.ID}} | {{.Path}} |
{{else}}| _(none)_ | |
{{end}}
`

const cliTmpl = `# CLI

<!-- typology:generated -->

Operator surface for slice {{.S.ID}}.

| Component | Path |
|-----------|------|
{{range cliComponents .S}}| {{.ID}} | {{.Path}} |
{{else}}| _(none)_ | |
{{end}}
`

const presentationTmpl = `# Presentation

<!-- typology:generated -->

Viewer/UI wiring for slice {{.S.ID}}.

| Component | Path |
|-----------|------|
{{range uiComponents .S}}| {{.ID}} | {{.Path}} |
{{else}}| _(none)_ | |
{{end}}
`

const pipelinesTmpl = `# AI pipelines

<!-- typology:generated -->

Operator runs for slice {{.S.ID}} (CLI, HTTP, human gate, signal, or later schedule).

| OpRun | Owner | Gate | CLI | Runs | Actuates |
|-------|-------|------|-----|------|----------|
{{range .S.OpRuns}}| {{.ID}} | {{.OwnerComponent}} | {{.Gate}} | {{.CLI}} | {{.Runs}} | {{.Actuates}} |
{{else}}| _(none)_ | | | | | |
{{end}}
`

const typologyReadmeTmpl = `# Typology

<!-- typology:generated -->

This directory holds the Typology catalog for this repository. Use it to guide architecture decisions and code structure: slices are bounded contexts, components are packages, and bindings are the allowed couplings. One repository owns one catalog and its architecture docs.

` + "`typology.yaml`" + ` is the source of truth for scope, slices, components, bindings, subprograms, actuators, and opRuns. In a multi-module workspace, ` + "`scope.modules`" + ` declares which repository-local modules Typology inspects; ` + "`go.work`" + ` does not widen that scope.

## For Agents

Before you change this catalog or the code it describes, load these skills:

- ` + "`typology-journey`" + ` — first map, resume ` + "`typology-journey.md`" + `
- ` + "`typology-catalog`" + ` — model, YAML shape, subprograms, actuators, bindings
- ` + "`typology-cli`" + ` — discover, emit, validate, remediate
- ` + "`typology-docs`" + ` — fill and evaluate develop DocPages

If your host does not have these skills, install the Typology module and symlink the skills from ` + "`$TYPOLOGY_ROOT/skills/`" + ` into your host skills directory (see the Typology module ` + "`AGENTS.md`" + `).

## Consumer bootstrap

Before running Typology commands in a consumer, register the CLI as a Go tool:

` + "```bash" + `
go run github.com/behaviorengineering/typology/cmd/typology@v0.0.5 init .
go tool typology version
` + "```" + `

The bootstrap updates the selected module's ` + "`go.mod`" + ` and ` + "`go.sum`" + `, but it does not add the CLI to application imports or binaries. If ` + "`go.work`" + ` covers more than one module, pass ` + "`--module PATH`" + ` explicitly.

## Typical commands

- ` + "`typology init REPO [--module PATH] [--version VERSION]`" + ` — registers the CLI as a Go tool in the consumer module
- ` + "`typology discover REPO [--module PATH]`" + ` — writes a draft to ` + "`tmp/typology/typology.yaml`" + `; use ` + "`--module`" + ` when the repository has multiple Go modules
- ` + "`typology emit REPO`" + ` — writes ` + "`.typology/typology.yaml`" + ` and DocPages
- ` + "`typology architecture REPO [--module PATH]`" + ` — writes ` + "`docs/architecture/typology.md`" + ` for human review within ` + "`scope.modules`" + `
- ` + "`typology validate REPO [--module PATH]`" + ` — checks the catalog and scoped modules against each other
- ` + "`typology remediate REPO SLICE [--module PATH]`" + ` — agent-scoped violations for one slice and module scope

## Workflow

Catalog first, code second, validation last:

1. Update ` + "`typology.yaml`" + ` to declare the intended architecture, components, bindings, and programs before writing the code.
2. Implement the code to match the catalog.
3. In a multi-module repository, set ` + "`scope.modules`" + ` to the modules owned by this catalog. Do not rely on ` + "`go.work`" + ` as Typology scope.
4. Run ` + "`typology architecture REPO`" + ` to give humans a readable comparison of the catalog and the observed Go topology.
5. Have an agent or architect fix each finding or record the boundary debt in ` + "`.typology/typology-journey.md`" + `.
6. Run ` + "`typology validate REPO`" + ` and fix every issue before considering the change done.

A green catalog means the code matches the declared slices, components, and bindings.

## Files

- ` + "`typology.yaml`" + ` — confirmed catalog
- ` + "`docs/architecture/typology.md`" + ` — generated architecture brief; remove its marker after human acceptance
- ` + "`tools.yaml`" + ` — generated CLI tool index from catalog ` + "`opRuns`" + `
- ` + "`typology-journey.md`" + ` — first-map session file (created during a journey)
- ` + "`README.md`" + ` — this file
`

const toolsTmpl = `# typology:generated
# Generated from .typology/typology.yaml. Do not edit this file.

typology: {{yamlQuote .Typology}}
{{if .Tools}}tools:
{{range .Tools}}  - id: {{yamlQuote .ID}}
    command: {{yamlQuote .Command}}
{{if .Summary}}    summary: {{yamlQuote .Summary}}
{{end}}    slice: {{yamlQuote .Slice}}
    ownerComponent: {{yamlQuote .OwnerComponent}}
{{if .Gate}}    gate: {{yamlQuote .Gate}}
{{end}}{{if .Runs}}    runs: {{yamlQuote .Runs}}
{{end}}{{if .Actuates}}    actuates: {{yamlQuote .Actuates}}
{{end}}{{end}}{{else}}tools: []
{{end}}`

func sliceBindingsFor(t catalog.Typology, sliceID string) []catalog.SliceBinding {
	var out []catalog.SliceBinding
	for _, b := range t.SliceBindings {
		if b.From == sliceID || b.To == sliceID {
			out = append(out, b)
		}
	}
	return out
}

func compBindingsFor(t catalog.Typology, sliceID string) []catalog.ComponentBinding {
	compIDs := map[string]struct{}{}
	for _, s := range t.Slices {
		if s.ID != sliceID {
			continue
		}
		for _, c := range s.AllComponents() {
			compIDs[c.ID] = struct{}{}
		}
	}
	var out []catalog.ComponentBinding
	for _, b := range t.ComponentBindings {
		if _, ok := compIDs[b.From]; ok {
			out = append(out, b)
		}
	}
	return out
}

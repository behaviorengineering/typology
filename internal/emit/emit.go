package emit

import (
	"os"
	"path/filepath"
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
	if len(existing) > 0 && !strings.Contains(string(existing), "<!-- typology:generated -->") {
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
	b.WriteString("├── Overview          [" + overview + "](" + overview + ")\n")
	b.WriteString("├── Owns              [" + owns + "](" + owns + ")\n")
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

Slice ` + "`{{.Slice.ID}}`" + ` subprogram. Owner component: ` + "`{{.Subprogram.OwnerComponent}}`" + `.

| Field | Value |
|-------|-------|
| Input | {{.Subprogram.Input}} |
| Output | {{.Subprogram.Output}} |
| Store | {{join .Subprogram.Store ", "}} |
| Gate | {{.Subprogram.Gate}} |
`

const actuatorTmpl = `# {{.Actuator.ID}}

<!-- typology:generated -->

Slice ` + "`{{.Slice.ID}}`" + ` actuator. Owner component: ` + "`{{.Actuator.OwnerComponent}}`" + `.

| Field | Value |
|-------|-------|
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

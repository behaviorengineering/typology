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
	"apiComponents": func(owns []catalog.Component) []catalog.Component {
		return filterKind(owns, catalog.InteractionAPI)
	},
	"cliComponents": func(owns []catalog.Component) []catalog.Component {
		return filterKind(owns, catalog.InteractionCLI)
	},
	"uiComponents": func(owns []catalog.Component) []catalog.Component {
		return filterKind(owns, catalog.InteractionUI)
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
		pages = catalog.DefaultDocCluster(s.ID, catalog.DefaultDocsRoot).Pages
	}
	for _, page := range pages {
		body, err := renderPage(s, page, t)
		if err != nil {
			return err
		}
		abs := filepath.Join(repoRoot, filepath.FromSlash(page.Path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "emit.emitSliceDocs", "mkdir docs").
				With("path", page.Path)
		}
		existing, err := os.ReadFile(abs)
		if err != nil && !os.IsNotExist(err) {
			return terrors.Wrap(err, terrors.CodeUnavailable, "emit.emitSliceDocs", "read existing page").
				With("path", page.Path)
		}
		if len(existing) > 0 && !strings.Contains(string(existing), "<!-- typology:generated -->") {
			continue
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "emit.emitSliceDocs", "write page").
				With("path", page.Path)
		}
	}
	return nil
}

type pageData struct {
	S catalog.Slice
	T catalog.Typology
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
	parsed, err := template.New(string(page.Kind)).Funcs(tmplFuncs).Parse(tmpl)
	if err != nil {
		return "", terrors.Wrap(err, terrors.CodeInternal, "emit.renderPage", "parse template").
			With("kind", string(page.Kind))
	}
	var b strings.Builder
	if err := parsed.Execute(&b, data); err != nil {
		return "", terrors.Wrap(err, terrors.CodeInternal, "emit.renderPage", "execute template").
			With("kind", string(page.Kind))
	}
	return b.String(), nil
}

const overviewTmpl = `# Overview

<!-- typology:generated -->

{{if .S.Objective}}{{.S.Objective}}{{else}}Bounded context {{.S.ID}}.{{end}}

{{if .S.Route}}Route: {{.S.Route}}{{end}}

## Owned components

| Component | Path | Layer |
|-----------|------|-------|
{{range .S.Owns}}| {{.ID}} | {{.Path}} | {{.Layer}}{{if .Kind}}/{{.Kind}}{{end}} |
{{end}}
`

const componentsTmpl = `# Components

<!-- typology:generated -->

## Composition

| Component | Path | Layer |
|-----------|------|-------|
{{range .S.Owns}}| {{.ID}} | {{.Path}} | {{.Layer}}{{if .Kind}}/{{.Kind}}{{end}} |
{{end}}

## SliceBindings

| From | To | Kind |
|------|----|------|
{{range sliceBindings .T .S.ID}}| {{.From}} | {{.To}} | {{.Kind}} |
{{else}}| _(none)_ | | |
{{end}}

## ComponentBindings

| From | To | Rule |
|------|----|------|
{{range compBindings .T .S.ID}}| {{.From}} | {{.To}} | {{.Rule}} |
{{else}}| _(none)_ | | |
{{end}}
`

const contractsTmpl = `# Contracts

<!-- typology:generated -->

HTTP/API surface for slice {{.S.ID}}. Endpoint types are not modeled in typology v1.

| Component | Path |
|-----------|------|
{{range apiComponents .S.Owns}}| {{.ID}} | {{.Path}} |
{{else}}| _(none)_ | |
{{end}}
`

const cliTmpl = `# CLI

<!-- typology:generated -->

Operator surface for slice {{.S.ID}}.

| Component | Path |
|-----------|------|
{{range cliComponents .S.Owns}}| {{.ID}} | {{.Path}} |
{{else}}| _(none)_ | |
{{end}}
`

const presentationTmpl = `# Presentation

<!-- typology:generated -->

Viewer/UI wiring for slice {{.S.ID}}.

| Component | Path |
|-----------|------|
{{range uiComponents .S.Owns}}| {{.ID}} | {{.Path}} |
{{else}}| _(none)_ | |
{{end}}
`

const pipelinesTmpl = `# AI pipelines

<!-- typology:generated -->

Background jobs for slice {{.S.ID}}.

| Job | Owner | Gate | CLI |
|-----|-------|------|-----|
{{range .S.Jobs}}| {{.ID}} | {{.OwnerComponent}} | {{.Gate}} | {{.CLI}} |
{{else}}| _(none)_ | | | |
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
		for _, c := range s.Owns {
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

func filterKind(owns []catalog.Component, kind catalog.InteractionKind) []catalog.Component {
	var out []catalog.Component
	for _, c := range owns {
		if c.Layer == catalog.LayerInteraction && c.Kind == kind {
			out = append(out, c)
		}
	}
	return out
}

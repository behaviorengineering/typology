package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/typology/catalog"
	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/discover"
	"github.com/behaviorengineering/typology/internal/emit"
	"github.com/behaviorengineering/typology/internal/remediate"
	"github.com/behaviorengineering/typology/internal/sourceindex"
	"github.com/behaviorengineering/typology/validate"
)

var version = "dev"

// Run dispatches typology subcommands. Returns an exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = stdin
	if stdout == nil || stderr == nil {
		return 2
	}
	if len(args) == 0 {
		printUsage(stdout)
		return 2
	}
	switch args[0] {
	case "discover":
		return runDiscover(args[1:], stdout, stderr)
	case "emit":
		return runEmit(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "show":
		return runShow(args[1:], stdout, stderr)
	case "remediate":
		return runRemediate(args[1:], stdout, stderr)
	case "version", "-version", "--version":
		_, _ = fmt.Fprintf(stdout, "typology %s\n", version)
		return 0
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "typology: unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "typology - discover, validate, and emit architecture catalogs")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  typology discover REPO [--out PATH] [--docs-root PATH] [--suggest-merges]")
	_, _ = fmt.Fprintln(w, "  typology emit REPO [--catalog PATH] [--docs-only] [--go-only]")
	_, _ = fmt.Fprintln(w, "  typology validate REPO [--catalog PATH] [SLICE]")
	_, _ = fmt.Fprintln(w, "  typology show [SLICE|graph] [--json] [--catalog PATH]")
	_, _ = fmt.Fprintln(w, "  typology remediate REPO SLICE [--catalog PATH]")
	_, _ = fmt.Fprintln(w, "  typology version")
}

func defaultCatalogPath(repo string) string {
	return filepath.Join(repo, filepath.FromSlash(catalog.DefaultCatalogRel))
}

func defaultDraftCatalogPath(repo string) string {
	return filepath.Join(repo, filepath.FromSlash(catalog.DefaultDraftCatalogRel))
}

func runDiscover(args []string, stdout, stderr io.Writer) int {
	repo, rest, ok := firstArg(args)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "usage: typology discover REPO [--out PATH] [--docs-root PATH]")
		return 2
	}
	out := defaultDraftCatalogPath(repo)
	docsRoot := catalog.DefaultDocsRoot
	suggestMerges := false
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--out":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "discover: --out requires path")
				return 2
			}
			out = rest[i+1]
			i++
		case "--docs-root":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "discover: --docs-root requires path")
				return 2
			}
			docsRoot = rest[i+1]
			i++
		case "--suggest-merges":
			suggestMerges = true
		default:
			_, _ = fmt.Fprintf(stderr, "discover: unknown flag %q\n", rest[i])
			return 2
		}
	}
	result, err := discover.Run(discover.Options{RepoRoot: repo, DocsRoot: docsRoot})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "discover: %v\n", err)
		return 1
	}
	if err := catalog.SaveYAML(out, result.Typology); err != nil {
		_, _ = fmt.Fprintf(stderr, "discover: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "discover: wrote draft catalog (%d slices, %d packages) -> %s\n",
		len(result.Typology.Slices), len(result.Packages), out)
	if suggestMerges && len(result.Graph.MergeSuggestions) > 0 {
		_, _ = fmt.Fprintln(stdout, "\nMerge candidates (sole importer / companion heuristics):")
		for _, m := range result.Graph.MergeSuggestions {
			_, _ = fmt.Fprintf(stdout, "  - %s -> %s (%s)\n", m.SourcePackage, m.TargetPackage, m.Reason)
		}
	}
	_, _ = fmt.Fprintln(stdout, "discover: review and rename before emit/validate")
	return 0
}

func runEmit(args []string, stdout, stderr io.Writer) int {
	repo, rest, ok := firstArg(args)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "usage: typology emit REPO [--catalog PATH] [--docs-only] [--go-only]")
		return 2
	}
	catalogPath := defaultCatalogPath(repo)
	docsOnly := false
	goOnly := false
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--catalog":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "emit: --catalog requires path")
				return 2
			}
			catalogPath = rest[i+1]
			i++
		case "--docs-only":
			docsOnly = true
		case "--go-only":
			goOnly = true
		default:
			_, _ = fmt.Fprintf(stderr, "emit: unknown flag %q\n", rest[i])
			return 2
		}
	}
	t, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "emit: %v\n", err)
		return 1
	}
	if err := emit.Run(emit.Options{
		RepoRoot: repo,
		Catalog:  t,
		DocsOnly: docsOnly,
		GoOnly:   goOnly,
	}); err != nil {
		_, _ = fmt.Fprintf(stderr, "emit: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, "emit: ok")
	return 0
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	repo, rest, ok := firstArg(args)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "usage: typology validate REPO [--catalog PATH] [SLICE]")
		return 2
	}
	catalogPath := defaultCatalogPath(repo)
	var sliceID string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--catalog":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "validate: --catalog requires path")
				return 2
			}
			catalogPath = rest[i+1]
			i++
		default:
			if strings.HasPrefix(rest[i], "-") {
				_, _ = fmt.Fprintf(stderr, "validate: unknown flag %q\n", rest[i])
				return 2
			}
			if sliceID != "" {
				_, _ = fmt.Fprintln(stderr, "usage: typology validate REPO [--catalog PATH] [SLICE]")
				return 2
			}
			sliceID = rest[i]
		}
	}
	t, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "validate: %v\n", err)
		return 1
	}
	issues := validate.Run(validate.Options{
		RepoRoot: repo,
		Catalog:  t,
		SliceID:  sliceID,
	})
	if len(issues) == 0 {
		if sliceID == "" {
			_, _ = fmt.Fprintln(stdout, "validate: ok")
		} else {
			_, _ = fmt.Fprintf(stdout, "validate: ok (%s)\n", sliceID)
		}
		return 0
	}
	for _, issue := range issues {
		if issue.Slice != "" {
			_, _ = fmt.Fprintf(stderr, "validate: %s: %s\n", issue.Slice, issue.Message)
		} else {
			_, _ = fmt.Fprintf(stderr, "validate: %s\n", issue.Message)
		}
	}
	_, _ = fmt.Fprintf(stderr, "validate: %d issue(s)\n", len(issues))
	return 1
}

func runShow(args []string, stdout, stderr io.Writer) int {
	asJSON := false
	catalogPath := ""
	var sliceID string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			asJSON = true
		case a == "--catalog":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "show: --catalog requires path")
				return 2
			}
			catalogPath = args[i+1]
			i++
		case strings.HasPrefix(a, "-"):
			_, _ = fmt.Fprintf(stderr, "show: unknown flag %q\n", a)
			return 2
		default:
			if sliceID != "" {
				_, _ = fmt.Fprintln(stderr, "usage: typology show [SLICE] [--json] [--catalog PATH]")
				return 2
			}
			sliceID = a
		}
	}
	if catalogPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "show: %v\n",
				terrors.Wrap(err, terrors.CodeUnavailable, "cli.show", "get working directory"))
			return 1
		}
		p, err := catalog.FindCatalog(wd)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "show: %v\n", err)
			return 1
		}
		catalogPath = p
	}
	t, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "show: %v\n", err)
		return 1
	}
	if sliceID == "" {
		if asJSON {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(t); err != nil {
				_, _ = fmt.Fprintf(stderr, "show: %v\n",
					terrors.Wrap(err, terrors.CodeInternal, "cli.show", "encode catalog json"))
				return 1
			}
			return 0
		}
		for _, s := range t.Slices {
			_, _ = fmt.Fprintln(stdout, s.ID)
		}
		return 0
	}
	if sliceID == "graph" {
		repo := filepath.Dir(catalogPath)
		if repo == "" || repo == "." {
			if wd, err := os.Getwd(); err == nil {
				repo = wd
			}
		}
		summary, err := discover.AnalyzeGraph(repo)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "show graph: %v\n", err)
			return 1
		}
		if asJSON {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(summary); err != nil {
				_, _ = fmt.Fprintf(stderr, "show graph: %v\n",
					terrors.Wrap(err, terrors.CodeInternal, "cli.show", "encode graph json"))
				return 1
			}
			return 0
		}
		printGraphSummary(stdout, summary)
		if idx, err := sourceindex.Build(repo); err == nil {
			printSourceSummary(stdout, idx)
		} else {
			_, _ = fmt.Fprintf(stderr, "show graph source index: %v\n", err)
		}
		return 0
	}
	s, ok := t.LookupSlice(sliceID)
	if !ok {
		_, _ = fmt.Fprintf(stderr, "show: unknown slice %q\n", sliceID)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		_, _ = fmt.Fprintf(stderr, "show: %v\n",
			terrors.Wrap(err, terrors.CodeInternal, "cli.show", "encode slice json").
				With("slice", sliceID))
		return 1
	}
	return 0
}

func runRemediate(args []string, stdout, stderr io.Writer) int {
	repo, rest, ok := firstArg(args)
	if !ok || len(rest) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: typology remediate REPO SLICE [--catalog PATH]")
		return 2
	}
	sliceID := rest[0]
	catalogPath := defaultCatalogPath(repo)
	for i := 1; i < len(rest); i++ {
		if rest[i] == "--catalog" {
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "remediate: --catalog requires path")
				return 2
			}
			catalogPath = rest[i+1]
			i++
			continue
		}
		_, _ = fmt.Fprintf(stderr, "remediate: unknown flag %q\n", rest[i])
		return 2
	}
	t, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "remediate: %v\n", err)
		return 1
	}
	report, err := remediate.Run(remediate.Options{
		RepoRoot: repo,
		Catalog:  t,
		SliceID:  sliceID,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "remediate: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		_, _ = fmt.Fprintf(stderr, "remediate: %v\n",
			terrors.Wrap(err, terrors.CodeInternal, "cli.remediate", "encode report json").
				With("slice", sliceID))
		return 1
	}
	if len(report.Violations) > 0 {
		return 1
	}
	return 0
}

func firstArg(args []string) (repo string, rest []string, ok bool) {
	if len(args) == 0 {
		return "", nil, false
	}
	return args[0], args[1:], true
}

func printGraphSummary(w io.Writer, s discover.GraphSummary) {
	_, _ = fmt.Fprintf(w, "=== Typology Import Graph (%d packages) ===\n\n", len(s.Nodes))
	if len(s.Hubs) > 0 {
		_, _ = fmt.Fprintln(w, "Hubs (high coupling):")
		for _, h := range s.Hubs {
			node := s.Nodes[h]
			_, _ = fmt.Fprintf(w, "  - %s (in: %d, out: %d)\n", h, node.InDegree, node.OutDegree)
		}
		_, _ = fmt.Fprintln(w)
	}
	if len(s.Leaves) > 0 {
		_, _ = fmt.Fprintln(w, "Leaves (out-degree 0):")
		for _, l := range s.Leaves {
			node := s.Nodes[l]
			_, _ = fmt.Fprintf(w, "  - %s (in: %d)\n", l, node.InDegree)
		}
		_, _ = fmt.Fprintln(w)
	}
	if len(s.MergeSuggestions) > 0 {
		_, _ = fmt.Fprintln(w, "Merge suggestions (heuristics):")
		for _, m := range s.MergeSuggestions {
			_, _ = fmt.Fprintf(w, "  - %s -> %s (%s)\n", m.SourcePackage, m.TargetPackage, m.Reason)
		}
		_, _ = fmt.Fprintln(w)
	}
	if len(s.StemCollisions) > 0 {
		_, _ = fmt.Fprintln(w, "Stem collision warnings:")
		for _, c := range s.StemCollisions {
			_, _ = fmt.Fprintf(w, "  - %s: %s\n", strings.Join(c.Packages, ", "), c.Warning)
		}
		_, _ = fmt.Fprintln(w)
	}
	if len(s.PlatformLeaves) > 0 {
		_, _ = fmt.Fprintln(w, "Platform utility leaves (keep small):")
		for _, pl := range s.PlatformLeaves {
			_, _ = fmt.Fprintf(w, "  - %s\n", pl)
		}
		_, _ = fmt.Fprintln(w)
	}
}

func printSourceSummary(w io.Writer, idx sourceindex.Index) {
	_, _ = fmt.Fprintf(w, "Source evidence (AST):\n")
	_, _ = fmt.Fprintf(w, "  - packages: %d\n", len(idx.Packages))
	_, _ = fmt.Fprintf(w, "  - anchored packages: %d\n", idx.AnchoredPackages())
}

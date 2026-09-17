package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/behaviorengineering/typology/architecture"
	"github.com/behaviorengineering/typology/assemblygraph"
	"github.com/behaviorengineering/typology/catalog"
	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/bootstrap"
	"github.com/behaviorengineering/typology/internal/discover"
	"github.com/behaviorengineering/typology/internal/emit"
	"github.com/behaviorengineering/typology/internal/evidence"
	"github.com/behaviorengineering/typology/internal/gorepo"
	"github.com/behaviorengineering/typology/internal/remediate"
	"github.com/behaviorengineering/typology/internal/sourceindex"
	"github.com/behaviorengineering/typology/validate"
)

// version is injected by GoReleaser / make build via -ldflags -X.
// go install does not apply those ldflags, so reportVersion falls back to
// runtime/debug.BuildInfo (module version from the toolchain).
var version = "dev"

func reportVersion() string {
	moduleVersion := ""
	if bi, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = bi.Main.Version
	}
	return resolveVersion(version, moduleVersion)
}

func resolveVersion(ldflag, moduleVersion string) string {
	if v := strings.TrimSpace(ldflag); v != "" && v != "dev" {
		return v
	}
	if v := strings.TrimSpace(moduleVersion); v != "" && v != "(devel)" {
		return v
	}
	return "dev"
}

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
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "discover":
		return runDiscover(args[1:], stdout, stderr)
	case "contracts":
		return runContracts(args[1:], stdout, stderr)
	case "emit":
		return runEmit(args[1:], stdout, stderr)
	case "architecture":
		return runArchitecture(args[1:], stdout, stderr)
	case "assembly-graph":
		return runAssemblyGraph(args[1:], stdout, stderr)
	case "boards":
		return runBoards(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "show":
		return runShow(args[1:], stdout, stderr)
	case "remediate":
		return runRemediate(args[1:], stdout, stderr)
	case "version", "-version", "--version":
		_, _ = fmt.Fprintf(stdout, "typology %s\n", reportVersion())
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
	_, _ = fmt.Fprintln(w, "  typology init REPO [--module PATH] [--version VERSION]")
	_, _ = fmt.Fprintln(w, "  typology discover REPO [--module PATH] [--out PATH] [--docs-root PATH] [--suggest-merges]")
	_, _ = fmt.Fprintln(w, "  typology contracts REPO [--module PATH] [--out PATH]")
	_, _ = fmt.Fprintln(w, "  typology emit REPO [--catalog PATH] [--docs-only] [--go-only]")
	_, _ = fmt.Fprintln(w, "  typology architecture REPO [--module PATH] [--catalog PATH] [--out PATH]")
	_, _ = fmt.Fprintln(w, "  typology assembly-graph REPO [--module PATH] [--catalog PATH] [--slice SLICE|--all-slices] [--out PATH|--out-dir DIR]")
	_, _ = fmt.Fprintln(w, "  typology boards register REPO BOARD_ID [--prefix PREFIX] [--slice ID] [--viewer PUBLIC_DIR] [...]")
	_, _ = fmt.Fprintln(w, "  typology boards register REPO --all-slices [--prefix PREFIX] [--viewer PUBLIC_DIR] [...]")
	_, _ = fmt.Fprintln(w, "  typology boards register   # interactive wizard (TTY)")
	_, _ = fmt.Fprintln(w, "  typology boards sync --viewer PUBLIC_DIR")
	_, _ = fmt.Fprintln(w, "  typology boards path [--yaml|--config|--data]")
	_, _ = fmt.Fprintln(w, "  typology validate REPO [--module PATH] [--catalog PATH] [SLICE]")
	_, _ = fmt.Fprintln(w, "  typology show [SLICE|graph] [--module PATH] [--json] [--catalog PATH]")
	_, _ = fmt.Fprintln(w, "  typology remediate REPO SLICE [--module PATH] [--catalog PATH]")
	_, _ = fmt.Fprintln(w, "  typology version")
}

func defaultCatalogPath(repo string) string {
	return filepath.Join(repo, filepath.FromSlash(catalog.DefaultCatalogRel))
}

// loadCatalogModules returns scope.modules when a catalog exists; missing catalog is empty.
func loadCatalogModules(repo, catalogPath string) []string {
	path := strings.TrimSpace(catalogPath)
	if path == "" {
		path = defaultCatalogPath(repo)
	}
	typ, err := catalog.LoadYAML(path)
	if err != nil {
		return nil
	}
	return typ.Scope.Modules
}

func defaultDraftCatalogPath(repo string) string {
	return filepath.Join(repo, filepath.FromSlash(catalog.DefaultDraftCatalogRel))
}

func defaultPackageContractsPath(repo string) string {
	return filepath.Join(repo, filepath.FromSlash(catalog.DefaultPackageContractsRel))
}

func defaultPackageRolesPath(repo string) string {
	return filepath.Join(repo, filepath.FromSlash(catalog.DefaultPackageRolesRel))
}

func defaultPackageRLMContextPath(repo string) string {
	return filepath.Join(repo, filepath.FromSlash(catalog.DefaultPackageRLMContextRel))
}

func runInit(args []string, stdout, stderr io.Writer) int {
	repo, rest, ok := firstArg(args)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "usage: typology init REPO [--module PATH] [--version VERSION]")
		return 2
	}
	module := ""
	version := ""
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--module":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "init: --module requires path")
				return 2
			}
			module = rest[i+1]
			i++
		case "--version":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "init: --version requires version")
				return 2
			}
			version = rest[i+1]
			i++
		default:
			_, _ = fmt.Fprintf(stderr, "init: unknown flag %q\n", rest[i])
			return 2
		}
	}
	result, err := bootstrap.Run(context.Background(), bootstrap.Options{
		RepoRoot: repo,
		Module:   module,
		Version:  version,
		Stdout:   stdout,
		Stderr:   stderr,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "init: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "init: configured %s in %s\n", bootstrap.ToolPackage+"@"+result.Version, result.Module.Dir)
	return 0
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
	module := ""
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--module":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "discover: --module requires path")
				return 2
			}
			module = rest[i+1]
			i++
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
	result, err := discover.Run(discover.Options{RepoRoot: repo, DocsRoot: docsRoot, Module: module})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "discover: %v\n", err)
		return 1
	}
	if err := catalog.SaveYAML(out, result.Typology); err != nil {
		_, _ = fmt.Fprintf(stderr, "discover: %v\n", err)
		return 1
	}
	contractsOut := defaultPackageContractsPath(repo)
	rolesOut := defaultPackageRolesPath(repo)
	if err := writePackageEvidence(repo, module, contractsOut, rolesOut); err != nil {
		_, _ = fmt.Fprintf(stderr, "discover: package evidence: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "discover: wrote draft catalog (%d slices, %d packages) -> %s\n",
		len(result.Typology.Slices), len(result.Packages), out)
	_, _ = fmt.Fprintf(stdout, "discover: wrote package contracts -> %s\n", contractsOut)
	_, _ = fmt.Fprintf(stdout, "discover: wrote package roles -> %s\n", rolesOut)
	_, _ = fmt.Fprintf(stdout, "discover: wrote package RLM context -> %s\n", defaultPackageRLMContextPath(repo))
	if suggestMerges && len(result.Graph.MergeSuggestions) > 0 {
		_, _ = fmt.Fprintln(stdout, "\nMerge candidates (sole importer / companion heuristics):")
		for _, m := range result.Graph.MergeSuggestions {
			_, _ = fmt.Fprintf(stdout, "  - %s -> %s (%s)\n", m.SourcePackage, m.TargetPackage, m.Reason)
		}
	}
	_, _ = fmt.Fprintln(stdout, "discover: review and rename before emit/validate")
	return 0
}

func runContracts(args []string, stdout, stderr io.Writer) int {
	repo, rest, ok := firstArg(args)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "usage: typology contracts REPO [--module PATH] [--out PATH]")
		return 2
	}
	out := defaultPackageContractsPath(repo)
	module := ""
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--module":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "contracts: --module requires path")
				return 2
			}
			module = rest[i+1]
			i++
		case "--out":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "contracts: --out requires path")
				return 2
			}
			out = rest[i+1]
			i++
		default:
			_, _ = fmt.Fprintf(stderr, "contracts: unknown flag %q\n", rest[i])
			return 2
		}
	}
	if err := writePackageEvidence(repo, module, out, defaultPackageRolesPath(repo)); err != nil {
		_, _ = fmt.Fprintf(stderr, "contracts: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "contracts: wrote package contracts -> %s\n", out)
	_, _ = fmt.Fprintf(stdout, "contracts: wrote package roles -> %s\n", defaultPackageRolesPath(repo))
	_, _ = fmt.Fprintf(stdout, "contracts: wrote package RLM context -> %s\n", defaultPackageRLMContextPath(repo))
	return 0
}

func writePackageEvidence(repo, module, contractsOut, rolesOut string) error {
	h, err := evidence.Harvest(evidence.HarvestOptions{
		RepoRoot: repo,
		Modules:  loadCatalogModules(repo, ""),
		Module:   module,
	})
	if err != nil {
		return err
	}
	rlmOut := filepath.Join(filepath.Dir(rolesOut), "package_rlm_context.md")
	return evidence.WriteFiles(h, contractsOut, rolesOut, rlmOut)
}

func writePackageContracts(repo, module, outPath string) error {
	return writePackageEvidence(repo, module, outPath, defaultPackageRolesPath(repo))
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

func runArchitecture(args []string, stdout, stderr io.Writer) int {
	repo, rest, ok := firstArg(args)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "usage: typology architecture REPO [--catalog PATH] [--out PATH]")
		return 2
	}
	catalogPath := defaultCatalogPath(repo)
	outPath := filepath.Join(repo, filepath.FromSlash(architecture.DefaultReportRel))
	module := ""
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--module":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "architecture: --module requires path")
				return 2
			}
			module = rest[i+1]
			i++
		case "--catalog":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "architecture: --catalog requires path")
				return 2
			}
			catalogPath = rest[i+1]
			i++
		case "--out":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "architecture: --out requires path")
				return 2
			}
			outPath = rest[i+1]
			i++
		default:
			_, _ = fmt.Fprintf(stderr, "architecture: unknown flag %q\n", rest[i])
			return 2
		}
	}
	t, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "architecture: %v\n", err)
		return 1
	}
	report, err := architecture.Build(architecture.BuildOptions{
		RepoRoot: repo,
		Catalog:  t,
		Module:   module,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "architecture: %v\n", err)
		return 1
	}
	body, err := architecture.RenderMarkdown(report)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "architecture: %v\n", err)
		return 1
	}
	written, err := architecture.WriteMarkdown(outPath, body)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "architecture: %v\n", err)
		return 1
	}
	if written {
		_, _ = fmt.Fprintf(stdout, "architecture: wrote %s\n", outPath)
	} else {
		_, _ = fmt.Fprintf(stdout, "architecture: preserved human-owned %s\n", outPath)
	}
	if len(report.Findings) > 0 {
		_, _ = fmt.Fprintf(stderr, "architecture: %d finding(s); review the brief and fix or record each one\n", len(report.Findings))
		return 1
	}
	return 0
}

func runAssemblyGraph(args []string, stdout, stderr io.Writer) int {
	repo, rest, ok := firstArg(args)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "usage: typology assembly-graph REPO [--module PATH] [--catalog PATH] [--slice SLICE|--all-slices] [--out PATH|--out-dir DIR]")
		return 2
	}
	outPath := assemblygraph.DefaultPath(repo)
	outDir := ""
	module := ""
	catalogPath := ""
	sliceID := ""
	allSlices := false
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--module":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "assembly-graph: --module requires path")
				return 2
			}
			module = rest[i+1]
			i++
		case "--catalog":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "assembly-graph: --catalog requires path")
				return 2
			}
			catalogPath = rest[i+1]
			i++
		case "--slice":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "assembly-graph: --slice requires id")
				return 2
			}
			sliceID = rest[i+1]
			i++
		case "--all-slices":
			allSlices = true
		case "--out":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "assembly-graph: --out requires path")
				return 2
			}
			outPath = rest[i+1]
			i++
		case "--out-dir":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "assembly-graph: --out-dir requires path")
				return 2
			}
			outDir = rest[i+1]
			i++
		default:
			_, _ = fmt.Fprintf(stderr, "assembly-graph: unknown flag %q\n", rest[i])
			return 2
		}
	}
	if sliceID != "" && allSlices {
		_, _ = fmt.Fprintln(stderr, "assembly-graph: use --slice or --all-slices, not both")
		return 2
	}
	if allSlices && outDir == "" {
		outDir = filepath.Join(repo, filepath.FromSlash(assemblygraph.DefaultBoardsRel))
	}
	if sliceID == "" && !allSlices && outDir != "" {
		_, _ = fmt.Fprintln(stderr, "assembly-graph: --out-dir requires --slice or --all-slices")
		return 2
	}

	if catalogPath == "" {
		catalogPath = defaultCatalogPath(repo)
	}
	g, err := assemblygraph.Build(assemblygraph.BuildOptions{
		RepoRoot: repo,
		Modules:  loadCatalogModules(repo, catalogPath),
		Module:   module,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "assembly-graph: %v\n", err)
		return 1
	}

	if sliceID == "" && !allSlices {
		if err := assemblygraph.WriteJSON(outPath, g); err != nil {
			_, _ = fmt.Fprintf(stderr, "assembly-graph: %v\n", err)
			return 1
		}
		wrong := countWrongWay(g)
		_, _ = fmt.Fprintf(stdout, "assembly-graph: wrote %s (%d nodes, %d edges, %d wrong-way)\n",
			outPath, len(g.Nodes), len(g.Edges), wrong)
		return 0
	}

	typ, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "assembly-graph: %v\n", err)
		return 1
	}
	if issues := typ.ValidateStructure(); len(issues) > 0 {
		_, _ = fmt.Fprintf(stderr, "assembly-graph: catalog structure invalid (%d issue(s)); fix catalog first\n", len(issues))
		for _, issue := range issues {
			_, _ = fmt.Fprintf(stderr, "  - %s\n", issue.Message)
		}
		return 1
	}

	if sliceID != "" {
		projected, stats, err := assemblygraph.Project(g, typ, assemblygraph.ProjectOptions{SliceID: sliceID})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "assembly-graph: %v\n", err)
			return 1
		}
		writePath := outPath
		if outDir != "" {
			writePath = filepath.Join(outDir, sliceID, "assembly-graph.json")
		}
		if err := assemblygraph.WriteJSON(writePath, projected); err != nil {
			_, _ = fmt.Fprintf(stderr, "assembly-graph: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout,
			"assembly-graph: wrote %s (slice %s: %d owned, %d boundary stubs, %d internal cables, %d boundary cables, %d missing bindings, %d wrong-way)\n",
			writePath, stats.SliceID, stats.OwnedNodes, stats.BoundaryNodes,
			stats.InternalEdges, stats.BoundaryEdges, stats.MissingBindings, stats.WrongWay)
		return 0
	}

	written := 0
	for _, s := range typ.Slices {
		projected, stats, err := assemblygraph.Project(g, typ, assemblygraph.ProjectOptions{SliceID: s.ID})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "assembly-graph: slice %s: %v\n", s.ID, err)
			return 1
		}
		writePath := filepath.Join(outDir, s.ID, "assembly-graph.json")
		if err := assemblygraph.WriteJSON(writePath, projected); err != nil {
			_, _ = fmt.Fprintf(stderr, "assembly-graph: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout,
			"assembly-graph: wrote %s (slice %s: %d owned, %d boundary stubs, %d boundary cables, %d missing bindings)\n",
			writePath, stats.SliceID, stats.OwnedNodes, stats.BoundaryNodes, stats.BoundaryEdges, stats.MissingBindings)
		written++
	}
	_, _ = fmt.Fprintf(stdout, "assembly-graph: wrote %d slice board(s) under %s\n", written, outDir)
	return 0
}

func countWrongWay(g assemblygraph.Graph) int {
	wrong := 0
	for _, e := range g.Edges {
		if e.WrongWay {
			wrong++
		}
	}
	return wrong
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	repo, rest, ok := firstArg(args)
	if !ok {
		_, _ = fmt.Fprintln(stderr, "usage: typology validate REPO [--catalog PATH] [SLICE]")
		return 2
	}
	catalogPath := defaultCatalogPath(repo)
	var sliceID string
	module := ""
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--module":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "validate: --module requires path")
				return 2
			}
			module = rest[i+1]
			i++
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
		Modules:  t.Scope.Modules,
		Module:   module,
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
	module := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--json":
			asJSON = true
		case a == "--module":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "show: --module requires path")
				return 2
			}
			module = args[i+1]
			i++
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
				_, _ = fmt.Fprintln(stderr, "usage: typology show [SLICE|graph] [--module PATH] [--json] [--catalog PATH]")
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
		repo, err := repoRootFromCatalogPath(catalogPath)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "show graph: %v\n", err)
			return 1
		}
		modules, err := gorepo.ResolveModules(repo, t.Scope.Modules, module)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "show graph: %v\n", err)
			return 1
		}
		summary, err := discover.AnalyzeGraphInModules(repo, modules)
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
		if idx, err := sourceindex.BuildInModules(repo, modules); err == nil {
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
	module := ""
	for i := 1; i < len(rest); i++ {
		if rest[i] == "--module" {
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "remediate: --module requires path")
				return 2
			}
			module = rest[i+1]
			i++
			continue
		}
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
		Modules:  t.Scope.Modules,
		Module:   module,
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

func repoRootFromCatalogPath(catalogPath string) (string, error) {
	dir := filepath.Dir(catalogPath)
	if dir == "" {
		dir = "."
	}
	root, err := gorepo.FindRoot(dir)
	if err != nil {
		return "", terrors.Wrap(err, terrors.CodeNotFound, "cli.repoRootFromCatalogPath", "resolve repo root").
			With("catalog", catalogPath)
	}
	return root, nil
}

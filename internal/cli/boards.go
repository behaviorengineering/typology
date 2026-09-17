package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/behaviorengineering/typology/assemblygraph"
	"github.com/behaviorengineering/typology/boardregistry"
	"github.com/behaviorengineering/typology/catalog"
)

func runBoards(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: typology boards register|sync|path [flags]")
		return 2
	}
	switch args[0] {
	case "register":
		return runBoardsRegister(args[1:], stdout, stderr)
	case "sync":
		return runBoardsSync(args[1:], stdout, stderr)
	case "path":
		return runBoardsPath(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "boards: unknown subcommand %q\n", args[0])
		_, _ = fmt.Fprintln(stderr, "usage: typology boards register|sync|path [flags]")
		return 2
	}
}

func runBoardsPath(args []string, stdout, stderr io.Writer) int {
	which := "yaml"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--yaml":
			which = "yaml"
		case "--data":
			which = "data"
		case "--config":
			which = "config"
		default:
			_, _ = fmt.Fprintf(stderr, "boards path: unknown flag %q\n", args[i])
			return 2
		}
	}
	paths, err := boardregistry.ResolvePaths()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards path: %v\n", err)
		return 1
	}
	switch which {
	case "data":
		_, _ = fmt.Fprintln(stdout, paths.DataDir)
	case "config":
		_, _ = fmt.Fprintln(stdout, paths.ConfigDir)
	default:
		_, _ = fmt.Fprintln(stdout, paths.YAMLPath())
	}
	return 0
}

const boardsRegisterUsage = `usage: typology boards register REPO BOARD_ID [flags]
       typology boards register REPO --all-slices [flags]
       typology boards register                    # interactive wizard (TTY)
       typology boards register --id ID --label TEXT [flags]   # graph already in share dir

flags: [--prefix|--repo PREFIX] [--module PATH] [--label TEXT] [--slice ID]
       [--catalog PATH] [--viewer PUBLIC_DIR] [--make-default]
       (metadata-only also accepts --source PATH)`

func runBoardsRegister(args []string, stdout, stderr io.Writer) int {
	var (
		repoPath, boardID, label, prefix, module, sliceID, catalogPath, viewerPublic, source string
		makeDefault, allSlices                                                               bool
	)

	i := 0
	for i < len(args) && !strings.HasPrefix(args[i], "-") {
		if repoPath == "" {
			repoPath = args[i]
		} else if boardID == "" {
			boardID = args[i]
		} else {
			_, _ = fmt.Fprintf(stderr, "boards register: unexpected argument %q\n", args[i])
			_, _ = fmt.Fprintln(stderr, boardsRegisterUsage)
			return 2
		}
		i++
	}
	for ; i < len(args); i++ {
		switch args[i] {
		case "--id":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "boards register: --id requires value")
				return 2
			}
			boardID = args[i+1]
			i++
		case "--label":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "boards register: --label requires value")
				return 2
			}
			label = args[i+1]
			i++
		case "--repo", "--prefix":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "boards register: --prefix/--repo requires value")
				return 2
			}
			prefix = args[i+1]
			i++
		case "--source":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "boards register: --source requires value")
				return 2
			}
			source = args[i+1]
			i++
		case "--module":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "boards register: --module requires value")
				return 2
			}
			module = args[i+1]
			i++
		case "--slice":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "boards register: --slice requires value")
				return 2
			}
			sliceID = args[i+1]
			i++
		case "--catalog":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "boards register: --catalog requires value")
				return 2
			}
			catalogPath = args[i+1]
			i++
		case "--viewer":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "boards register: --viewer requires public dir path")
				return 2
			}
			viewerPublic = args[i+1]
			i++
		case "--make-default":
			makeDefault = true
		case "--all-slices":
			allSlices = true
		default:
			_, _ = fmt.Fprintf(stderr, "boards register: unknown flag %q\n", args[i])
			_, _ = fmt.Fprintln(stderr, boardsRegisterUsage)
			return 2
		}
	}

	prefix = strings.TrimSpace(prefix)
	boardID = strings.TrimSpace(boardID)
	label = strings.TrimSpace(label)
	repoPath = strings.TrimSpace(repoPath)
	sliceID = strings.TrimSpace(sliceID)
	viewerPublic = strings.TrimSpace(viewerPublic)

	if allSlices && sliceID != "" {
		_, _ = fmt.Fprintln(stderr, "boards register: use --slice or --all-slices, not both")
		return 2
	}
	if allSlices && boardID != "" {
		_, _ = fmt.Fprintln(stderr, "boards register: --all-slices does not take BOARD_ID")
		return 2
	}

	// Bare register: interactive wizard when stdin is a TTY; otherwise usage.
	if repoPath == "" && boardID == "" && !allSlices {
		return runBoardsRegisterWizard(stdout, stderr)
	}

	paths, err := boardregistry.ResolvePaths()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return 1
	}

	if repoPath != "" {
		if allSlices {
			return harvestAllSlices(stdout, stderr, paths, harvestOpts{
				Repo:    repoPath,
				Prefix:  prefix,
				Module:  module,
				Catalog: catalogPath,
				Viewer:  viewerPublic,
				Label:   label,
				MakeDef: makeDefault,
				Source:  firstNonEmpty(source, repoPath),
			})
		}
		if boardID == "" {
			_, _ = fmt.Fprintln(stderr, boardsRegisterUsage)
			return 2
		}
		return harvestOne(stdout, stderr, paths, harvestOpts{
			Repo:    repoPath,
			BoardID: boardID,
			Prefix:  prefix,
			Module:  module,
			Slice:   sliceID,
			Catalog: catalogPath,
			Viewer:  viewerPublic,
			Label:   label,
			MakeDef: makeDefault,
			Source:  firstNonEmpty(source, repoPath),
		})
	}

	// Metadata-only: graph must already exist under the share dir.
	if boardID == "" || label == "" {
		_, _ = fmt.Fprintln(stderr, boardsRegisterUsage)
		return 2
	}
	finalID := boardregistry.PrefixedID(prefix, boardID)
	return upsertAndMaybeMaterialize(stdout, stderr, paths, boardregistry.Board{
		ID:      finalID,
		Label:   boardregistry.DefaultLabel(prefix, boardID, label),
		Repo:    prefix,
		Source:  source,
		Module:  module,
		Slice:   sliceID,
		Catalog: catalogPath,
		Graph:   boardregistry.GraphRel(finalID),
	}, makeDefault, viewerPublic, true)
}

type harvestOpts struct {
	Repo    string
	BoardID string
	Prefix  string
	Module  string
	Slice   string
	Catalog string
	Viewer  string
	Label   string
	MakeDef bool
	Source  string
	// SliceIDs, when set, harvests only those slices (wizard multi-select).
	SliceIDs []string
}

func harvestOne(stdout, stderr io.Writer, paths boardregistry.Paths, opts harvestOpts) int {
	finalID := boardregistry.PrefixedID(opts.Prefix, opts.BoardID)
	if !boardregistry.ValidID(finalID) {
		_, _ = fmt.Fprintf(stderr, "boards register: board id %q is unsafe\n", finalID)
		return 2
	}
	g, err := assemblygraph.Build(assemblygraph.BuildOptions{RepoRoot: opts.Repo, Module: opts.Module})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return 1
	}
	outPath := paths.GraphAbs(finalID)
	sliceForBoard := opts.Slice
	if opts.Slice != "" {
		catalogPath := opts.Catalog
		if catalogPath == "" {
			catalogPath = defaultCatalogPath(opts.Repo)
		}
		typ, code := loadBoardCatalog(stderr, catalogPath)
		if code != 0 {
			return code
		}
		projected, stats, err := assemblygraph.Project(g, typ, assemblygraph.ProjectOptions{SliceID: opts.Slice})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
			return 1
		}
		if err := assemblygraph.WriteJSON(outPath, projected); err != nil {
			_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout,
			"boards register: harvested %s (slice %s: %d owned, %d boundary stubs, %d missing bindings)\n",
			outPath, stats.SliceID, stats.OwnedNodes, stats.BoundaryNodes, stats.MissingBindings)
	} else {
		if err := assemblygraph.WriteJSON(outPath, g); err != nil {
			_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "boards register: harvested %s (%d nodes, %d edges)\n",
			outPath, len(g.Nodes), len(g.Edges))
	}
	label := boardregistry.DefaultLabel(opts.Prefix, opts.BoardID, opts.Label)
	return upsertAndMaybeMaterialize(stdout, stderr, paths, boardregistry.Board{
		ID:      finalID,
		Label:   label,
		Repo:    opts.Prefix,
		Source:  opts.Source,
		Module:  opts.Module,
		Slice:   sliceForBoard,
		Catalog: opts.Catalog,
		Graph:   boardregistry.GraphRel(finalID),
	}, opts.MakeDef, opts.Viewer, false)
}

func harvestAllSlices(stdout, stderr io.Writer, paths boardregistry.Paths, opts harvestOpts) int {
	catalogPath := opts.Catalog
	if catalogPath == "" {
		catalogPath = defaultCatalogPath(opts.Repo)
	}
	typ, code := loadBoardCatalog(stderr, catalogPath)
	if code != 0 {
		return code
	}
	g, err := assemblygraph.Build(assemblygraph.BuildOptions{RepoRoot: opts.Repo, Module: opts.Module})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return 1
	}
	reg, err := boardregistry.LoadYAML(paths.YAMLPath())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return 1
	}

	slices := typ.Slices
	if len(opts.SliceIDs) > 0 {
		want := map[string]bool{}
		for _, id := range opts.SliceIDs {
			want[strings.TrimSpace(id)] = true
		}
		filtered := make([]catalog.Slice, 0, len(opts.SliceIDs))
		for _, s := range typ.Slices {
			if want[strings.TrimSpace(s.ID)] {
				filtered = append(filtered, s)
			}
		}
		slices = filtered
	}

	written := 0
	for idx, s := range slices {
		stem := strings.TrimSpace(s.ID)
		if !boardregistry.ValidID(stem) {
			_, _ = fmt.Fprintf(stderr, "boards register: skipping unsafe slice id %q\n", s.ID)
			continue
		}
		finalID := boardregistry.PrefixedID(opts.Prefix, stem)
		if !boardregistry.ValidID(finalID) {
			_, _ = fmt.Fprintf(stderr, "boards register: computed board id %q is unsafe\n", finalID)
			return 2
		}
		projected, stats, err := assemblygraph.Project(g, typ, assemblygraph.ProjectOptions{SliceID: s.ID})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "boards register: slice %s: %v\n", s.ID, err)
			return 1
		}
		outPath := paths.GraphAbs(finalID)
		if err := assemblygraph.WriteJSON(outPath, projected); err != nil {
			_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
			return 1
		}
		makeDef := opts.MakeDef && idx == 0
		label := boardregistry.DefaultLabel(opts.Prefix, stem, opts.Label)
		reg, err = boardregistry.Upsert(reg, boardregistry.UpsertOptions{
			Board: boardregistry.Board{
				ID:      finalID,
				Label:   label,
				Repo:    opts.Prefix,
				Source:  opts.Source,
				Module:  opts.Module,
				Slice:   s.ID,
				Catalog: catalogPath,
				Graph:   boardregistry.GraphRel(finalID),
			},
			MakeDefault: makeDef,
		})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout,
			"boards register: harvested %s (slice %s: %d owned, %d boundary stubs, %d missing bindings)\n",
			outPath, stats.SliceID, stats.OwnedNodes, stats.BoundaryNodes, stats.MissingBindings)
		written++
	}
	if written == 0 {
		_, _ = fmt.Fprintln(stderr, "boards register: no slices harvested")
		return 1
	}
	if err := boardregistry.SaveYAML(paths.YAMLPath(), reg); err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "boards register: upserted %d board(s) in %s\n", written, paths.YAMLPath())
	return materializeIfNeeded(stdout, stderr, paths, reg, opts.Viewer)
}

func upsertAndMaybeMaterialize(
	stdout, stderr io.Writer,
	paths boardregistry.Paths,
	board boardregistry.Board,
	makeDefault bool,
	viewerPublic string,
	requireExistingGraph bool,
) int {
	if requireExistingGraph {
		if _, err := os.Stat(paths.GraphAbs(board.ID)); err != nil {
			_, _ = fmt.Fprintf(stderr, "boards register: share graph missing at %s (%v); pass REPO to harvest\n",
				paths.GraphAbs(board.ID), err)
			return 1
		}
	}
	reg, err := boardregistry.LoadYAML(paths.YAMLPath())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return 1
	}
	reg, err = boardregistry.Upsert(reg, boardregistry.UpsertOptions{
		Board:       board,
		MakeDefault: makeDefault,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return 1
	}
	if err := boardregistry.SaveYAML(paths.YAMLPath(), reg); err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "boards register: upserted %s in %s\n", board.ID, paths.YAMLPath())
	return materializeIfNeeded(stdout, stderr, paths, reg, viewerPublic)
}

func materializeIfNeeded(stdout, stderr io.Writer, paths boardregistry.Paths, reg boardregistry.Registry, viewerPublic string) int {
	if strings.TrimSpace(viewerPublic) == "" {
		return 0
	}
	if err := boardregistry.Materialize(boardregistry.MaterializeOptions{
		ViewerPublicDir: viewerPublic,
		Paths:           paths,
		Registry:        reg,
	}); err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: materialize: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "boards register: materialized into %s\n", viewerPublic)
	return 0
}

func loadBoardCatalog(stderr io.Writer, catalogPath string) (catalog.Typology, int) {
	typ, err := catalog.LoadYAML(catalogPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return catalog.Typology{}, 1
	}
	if issues := typ.ValidateStructure(); len(issues) > 0 {
		_, _ = fmt.Fprintf(stderr, "boards register: catalog structure invalid (%d issue(s)); fix catalog first\n", len(issues))
		for _, issue := range issues {
			_, _ = fmt.Fprintf(stderr, "  - %s\n", issue.Message)
		}
		return catalog.Typology{}, 1
	}
	return typ, 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func runBoardsSync(args []string, stdout, stderr io.Writer) int {
	viewerPublic := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--viewer":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "boards sync: --viewer requires public dir path")
				return 2
			}
			viewerPublic = args[i+1]
			i++
		default:
			_, _ = fmt.Fprintf(stderr, "boards sync: unknown flag %q\n", args[i])
			return 2
		}
	}
	if strings.TrimSpace(viewerPublic) == "" {
		_, _ = fmt.Fprintln(stderr, "usage: typology boards sync --viewer PUBLIC_DIR")
		return 2
	}
	paths, err := boardregistry.ResolvePaths()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards sync: %v\n", err)
		return 1
	}
	reg, err := boardregistry.LoadYAML(paths.YAMLPath())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards sync: %v\n", err)
		return 1
	}
	if len(reg.Boards) == 0 {
		_, _ = fmt.Fprintf(stderr, "boards sync: no boards in %s; register first\n", paths.YAMLPath())
		return 1
	}
	if err := boardregistry.Materialize(boardregistry.MaterializeOptions{
		ViewerPublicDir: viewerPublic,
		Paths:           paths,
		Registry:        reg,
	}); err != nil {
		_, _ = fmt.Fprintf(stderr, "boards sync: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "boards sync: wrote %s (%d boards) from %s\n",
		filepath.Join(viewerPublic, "boards.json"), len(reg.Boards), paths.YAMLPath())
	return 0
}

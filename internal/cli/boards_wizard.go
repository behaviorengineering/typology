package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/behaviorengineering/typology/boardregistry"
	"github.com/behaviorengineering/typology/catalog"
)

var wizardIDSanitize = regexp.MustCompile(`[^a-z0-9]+`)

// runBoardsRegisterWizard runs the interactive Huh form when stdin is a TTY.
// Non-TTY (CI) fails closed with usage.
func runBoardsRegisterWizard(stdout, stderr io.Writer) int {
	if !stdinIsTTY() {
		_, _ = fmt.Fprintln(stderr, "boards register: interactive wizard requires a TTY; pass REPO BOARD_ID or flags for CI")
		_, _ = fmt.Fprintln(stderr, boardsRegisterUsage)
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return 1
	}

	repoPath := cwd
	prefix := sanitizeBoardStem(filepath.Base(cwd))
	module := ""
	label := ""
	viewerPublic := defaultViewerPublicHint()
	makeDefault := true
	mode := "all-slices" // all-slices | selected | full-module
	boardID := ""
	var selectedSlices []string

	catalogPath := defaultCatalogPath(repoPath)
	hasCatalog := false
	if _, err := os.Stat(catalogPath); err == nil {
		hasCatalog = true
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Repository root").
				Description("Consumer repo to harvest").
				Value(&repoPath).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return errors.New("repo path required")
					}
					info, err := os.Stat(s)
					if err != nil || !info.IsDir() {
						return fmt.Errorf("not a directory: %s", s)
					}
					return nil
				}),
			huh.NewInput().
				Title("Prefix (multi-repo id namespace)").
				Description("Optional. Board ids become <prefix>-<slice>").
				Value(&prefix).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return nil
					}
					if !boardregistry.ValidID(s) {
						return fmt.Errorf("prefix %q is unsafe (use lowercase, digits, hyphens)", s)
					}
					return nil
				}),
			huh.NewInput().
				Title("Go module path (optional)").
				Description("Pass when the workspace has multiple modules").
				Value(&module),
		),
	)
	if err := form.Run(); err != nil {
		return wizardErr(stderr, err)
	}
	repoPath = strings.TrimSpace(repoPath)
	prefix = strings.TrimSpace(prefix)
	module = strings.TrimSpace(module)
	catalogPath = defaultCatalogPath(repoPath)
	hasCatalog = false
	if _, err := os.Stat(catalogPath); err == nil {
		hasCatalog = true
	}

	if hasCatalog {
		modeForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("What should we register?").
					Options(
						huh.NewOption("All catalog slices", "all-slices"),
						huh.NewOption("Selected slices", "selected"),
						huh.NewOption("Full module (one board)", "full-module"),
					).
					Value(&mode),
			),
		)
		if err := modeForm.Run(); err != nil {
			return wizardErr(stderr, err)
		}
	} else {
		mode = "full-module"
		_, _ = fmt.Fprintf(stdout, "boards register: no catalog at %s; registering full module\n", catalogPath)
	}

	if mode == "selected" {
		typ, err := catalog.LoadYAML(catalogPath)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
			return 1
		}
		opts := make([]huh.Option[string], 0, len(typ.Slices))
		for _, s := range typ.Slices {
			id := strings.TrimSpace(s.ID)
			if id == "" {
				continue
			}
			opts = append(opts, huh.NewOption(id, id).Selected(true))
		}
		if len(opts) == 0 {
			_, _ = fmt.Fprintln(stderr, "boards register: catalog has no slices")
			return 1
		}
		pick := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Slices to register").
					Options(opts...).
					Value(&selectedSlices).
					Validate(func(v []string) error {
						if len(v) == 0 {
							return errors.New("select at least one slice")
						}
						return nil
					}),
			),
		)
		if err := pick.Run(); err != nil {
			return wizardErr(stderr, err)
		}
	}

	if mode == "full-module" {
		boardID = sanitizeBoardStem(filepath.Base(repoPath))
		if prefix != "" {
			boardID = sanitizeBoardStem(prefix)
		}
		idForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Board id").
					Value(&boardID).
					Validate(func(s string) error {
						s = strings.TrimSpace(s)
						if !boardregistry.ValidID(s) {
							return fmt.Errorf("board id %q is unsafe", s)
						}
						return nil
					}),
			),
		)
		if err := idForm.Run(); err != nil {
			return wizardErr(stderr, err)
		}
		boardID = strings.TrimSpace(boardID)
	}

	tail := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Label override (optional)").
				Value(&label),
			huh.NewInput().
				Title("Viewer public dir (optional)").
				Description("Materialize boards.json into this Vite public/ tree").
				Value(&viewerPublic),
			huh.NewConfirm().
				Title("Make the first board the default?").
				Affirmative("Yes").
				Negative("No").
				Value(&makeDefault),
		),
	)
	if err := tail.Run(); err != nil {
		return wizardErr(stderr, err)
	}
	label = strings.TrimSpace(label)
	viewerPublic = strings.TrimSpace(viewerPublic)

	paths, err := boardregistry.ResolvePaths()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
		return 1
	}

	opts := harvestOpts{
		Repo:    repoPath,
		Prefix:  prefix,
		Module:  module,
		Catalog: catalogPath,
		Viewer:  viewerPublic,
		Label:   label,
		MakeDef: makeDefault,
		Source:  repoPath,
	}

	switch mode {
	case "all-slices":
		return harvestAllSlices(stdout, stderr, paths, opts)
	case "selected":
		opts.SliceIDs = selectedSlices
		return harvestAllSlices(stdout, stderr, paths, opts)
	default:
		opts.BoardID = boardID
		return harvestOne(stdout, stderr, paths, opts)
	}
}

// stdinIsTTY reports whether the process stdin is an interactive terminal.
// Overridden in tests.
var stdinIsTTY = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func wizardErr(stderr io.Writer, err error) int {
	if errors.Is(err, huh.ErrUserAborted) {
		_, _ = fmt.Fprintln(stderr, "boards register: cancelled")
		return 130
	}
	_, _ = fmt.Fprintf(stderr, "boards register: %v\n", err)
	return 1
}

func sanitizeBoardStem(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = wizardIDSanitize.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "board"
	}
	if !boardregistry.ValidID(s) {
		return "board"
	}
	return s
}

func defaultViewerPublicHint() string {
	// Best-effort: typology module checkout viewer public/, else empty.
	candidates := []string{
		filepath.Join("viewer", "cable-board", "public"),
		filepath.Join("providers", "typology", "viewer", "cable-board", "public"),
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for _, rel := range candidates {
		p := filepath.Join(cwd, rel)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return ""
}

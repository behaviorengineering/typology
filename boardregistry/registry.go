// Package boardregistry stores the durable multi-repo cable board registry
// under the operator XDG config directory as YAML, with harvested graphs under
// the XDG data directory. The Vite viewer receives a materialized projection.
package boardregistry

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
	"gopkg.in/yaml.v3"
)

const (
	envConfigDir = "TYPOLOGY_CONFIG_DIR"
	envDataDir   = "TYPOLOGY_DATA_DIR"
	appName      = "typology"
	boardsFile   = "boards.yaml"
	boardsSubdir = "boards"
)

var idRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Registry is the durable YAML document under ~/.config/typology/boards.yaml.
type Registry struct {
	DefaultBoard string  `yaml:"defaultBoard,omitempty" json:"defaultBoard,omitempty"`
	Boards       []Board `yaml:"boards" json:"boards"`
}

// Board is one registered cable board.
type Board struct {
	ID      string `yaml:"id" json:"id"`
	Label   string `yaml:"label" json:"label"`
	Repo    string `yaml:"repo,omitempty" json:"repo,omitempty"`
	Source  string `yaml:"source,omitempty" json:"source,omitempty"`
	Module  string `yaml:"module,omitempty" json:"module,omitempty"`
	Slice   string `yaml:"slice,omitempty" json:"slice,omitempty"`
	Catalog string `yaml:"catalog,omitempty" json:"catalog,omitempty"`
	// Graph is relative to the data directory (boards/<id>/assembly-graph.json).
	Graph string `yaml:"graph" json:"graph"`
}

// ViewerManifest is the JSON projection served by the Vite public/ tree.
type ViewerManifest struct {
	DefaultBoard string              `json:"defaultBoard,omitempty"`
	Boards       []ViewerManifestEntry `json:"boards"`
}

// ViewerManifestEntry is one board in the viewer projection.
type ViewerManifestEntry struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Graph string `json:"graph"`
	Repo  string `json:"repo,omitempty"`
}

// Paths resolves config and data roots.
type Paths struct {
	ConfigDir string
	DataDir   string
}

// ResolvePaths returns XDG-style config and data directories for typology.
func ResolvePaths() (Paths, error) {
	cfg := strings.TrimSpace(os.Getenv(envConfigDir))
	if cfg == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return Paths{}, terrors.Wrap(err, terrors.CodeUnavailable, "boardregistry.ResolvePaths", "user config dir")
		}
		cfg = filepath.Join(base, appName)
	}
	data := strings.TrimSpace(os.Getenv(envDataDir))
	if data == "" {
		base, err := userDataDir()
		if err != nil {
			return Paths{}, terrors.Wrap(err, terrors.CodeUnavailable, "boardregistry.ResolvePaths", "user data dir")
		}
		data = filepath.Join(base, appName)
	}
	return Paths{ConfigDir: cfg, DataDir: data}, nil
}

func userDataDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

// YAMLPath is the durable registry file.
func (p Paths) YAMLPath() string {
	return filepath.Join(p.ConfigDir, boardsFile)
}

// ViewerPublicDir is the operator-local Vite public/ tree under the XDG data dir.
// Wizard defaults and optional materialize targets prefer this over a module checkout.
func (p Paths) ViewerPublicDir() string {
	return filepath.Join(p.DataDir, "viewer", "public")
}

// GraphRel returns the data-dir-relative graph path for a board id.
func GraphRel(boardID string) string {
	return filepath.ToSlash(filepath.Join(boardsSubdir, boardID, "assembly-graph.json"))
}

// GraphAbs returns the absolute share-dir path for a board id.
func (p Paths) GraphAbs(boardID string) string {
	return filepath.Join(p.DataDir, filepath.FromSlash(GraphRel(boardID)))
}

// LoadYAML reads the registry, or an empty registry when the file is missing.
func LoadYAML(path string) (Registry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Registry{Boards: []Board{}}, nil
		}
		return Registry{}, terrors.Wrap(err, terrors.CodeUnavailable, "boardregistry.LoadYAML", "read").
			With("path", path)
	}
	var reg Registry
	if err := yaml.Unmarshal(raw, &reg); err != nil {
		return Registry{}, terrors.Wrap(err, terrors.CodeInvalid, "boardregistry.LoadYAML", "parse").
			With("path", path)
	}
	if reg.Boards == nil {
		reg.Boards = []Board{}
	}
	return reg, nil
}

// SaveYAML writes the registry atomically enough for local use.
func SaveYAML(path string, reg Registry) error {
	if err := Validate(reg); err != nil {
		return err
	}
	raw, err := yaml.Marshal(&reg)
	if err != nil {
		return terrors.Wrap(err, terrors.CodeInternal, "boardregistry.SaveYAML", "marshal")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "boardregistry.SaveYAML", "mkdir").
			With("path", path)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "boardregistry.SaveYAML", "write").
			With("path", path)
	}
	return nil
}

// Validate checks ids and cross-repo claims.
func Validate(reg Registry) error {
	seen := map[string]string{}
	for i, b := range reg.Boards {
		id := strings.TrimSpace(b.ID)
		if !idRE.MatchString(id) {
			return terrors.New(terrors.CodeInvalid, "boardregistry.Validate",
				fmt.Sprintf("board[%d]: id %q is unsafe", i, b.ID))
		}
		if strings.TrimSpace(b.Label) == "" {
			return terrors.New(terrors.CodeInvalid, "boardregistry.Validate",
				fmt.Sprintf("board %s: empty label", id))
		}
		if strings.TrimSpace(b.Graph) == "" {
			return terrors.New(terrors.CodeInvalid, "boardregistry.Validate",
				fmt.Sprintf("board %s: empty graph", id))
		}
		if repo := strings.TrimSpace(b.Repo); repo != "" && !idRE.MatchString(repo) {
			return terrors.New(terrors.CodeInvalid, "boardregistry.Validate",
				fmt.Sprintf("board %s: repo %q is unsafe", id, b.Repo))
		}
		if prev, ok := seen[id]; ok {
			return terrors.New(terrors.CodeInvalid, "boardregistry.Validate",
				fmt.Sprintf("duplicate board id %q (also repo %q)", id, prev))
		}
		seen[id] = strings.TrimSpace(b.Repo)
	}
	if def := strings.TrimSpace(reg.DefaultBoard); def != "" {
		if _, ok := seen[def]; !ok {
			return terrors.New(terrors.CodeInvalid, "boardregistry.Validate",
				fmt.Sprintf("defaultBoard %q not in boards", def))
		}
	}
	return nil
}

// UpsertOptions configures a registry upsert.
type UpsertOptions struct {
	Board        Board
	MakeDefault  bool
	AllowMissing bool // when true, empty registry file is fine
}

// Upsert inserts or updates a board. Refuses when the id is claimed by another repo.
func Upsert(reg Registry, opts UpsertOptions) (Registry, error) {
	b := opts.Board
	b.ID = strings.TrimSpace(b.ID)
	b.Label = strings.TrimSpace(b.Label)
	b.Repo = strings.TrimSpace(b.Repo)
	b.Source = strings.TrimSpace(b.Source)
	b.Module = strings.TrimSpace(b.Module)
	b.Slice = strings.TrimSpace(b.Slice)
	b.Catalog = strings.TrimSpace(b.Catalog)
	b.Graph = filepath.ToSlash(strings.TrimSpace(b.Graph))
	if b.Graph == "" {
		b.Graph = GraphRel(b.ID)
	}
	if !idRE.MatchString(b.ID) {
		return reg, terrors.New(terrors.CodeInvalid, "boardregistry.Upsert",
			fmt.Sprintf("board id %q is unsafe", b.ID))
	}
	if b.Label == "" {
		return reg, terrors.New(terrors.CodeInvalid, "boardregistry.Upsert", "label empty")
	}
	if b.Repo != "" && !idRE.MatchString(b.Repo) {
		return reg, terrors.New(terrors.CodeInvalid, "boardregistry.Upsert",
			fmt.Sprintf("repo %q is unsafe", b.Repo))
	}

	out := reg
	out.Boards = append([]Board(nil), reg.Boards...)
	for i := range out.Boards {
		if out.Boards[i].ID != b.ID {
			continue
		}
		existingRepo := strings.TrimSpace(out.Boards[i].Repo)
		if existingRepo != "" && b.Repo != "" && existingRepo != b.Repo {
			return reg, terrors.New(terrors.CodeFailedPrecondition, "boardregistry.Upsert",
				fmt.Sprintf("board id %q already claimed by repo %q; refusing overwrite from %q",
					b.ID, existingRepo, b.Repo))
		}
		if existingRepo != "" && b.Repo == "" {
			return reg, terrors.New(terrors.CodeFailedPrecondition, "boardregistry.Upsert",
				fmt.Sprintf("board id %q already claimed by repo %q; pass matching --prefix to refresh",
					b.ID, existingRepo))
		}
		out.Boards[i] = b
		if opts.MakeDefault || strings.TrimSpace(out.DefaultBoard) == "" {
			out.DefaultBoard = b.ID
		}
		return out, Validate(out)
	}
	out.Boards = append(out.Boards, b)
	if opts.MakeDefault || strings.TrimSpace(out.DefaultBoard) == "" {
		out.DefaultBoard = b.ID
	}
	return out, Validate(out)
}

// MaterializeOptions configures projection into the Vite public/ tree.
type MaterializeOptions struct {
	ViewerPublicDir string
	Paths           Paths
	Registry        Registry
}

// Materialize copies share graphs into viewer public/boards and writes boards.json.
func Materialize(opts MaterializeOptions) error {
	public := strings.TrimSpace(opts.ViewerPublicDir)
	if public == "" {
		return terrors.New(terrors.CodeInvalid, "boardregistry.Materialize", "viewer public dir empty")
	}
	if err := Validate(opts.Registry); err != nil {
		return err
	}
	viewerBoards := make([]ViewerManifestEntry, 0, len(opts.Registry.Boards))
	for _, b := range opts.Registry.Boards {
		src := opts.Paths.GraphAbs(b.ID)
		if b.Graph != "" {
			src = filepath.Join(opts.Paths.DataDir, filepath.FromSlash(b.Graph))
		}
		if _, err := os.Stat(src); err != nil {
			return terrors.Wrap(err, terrors.CodeFailedPrecondition, "boardregistry.Materialize",
				"share graph missing").With("board", b.ID).With("path", src)
		}
		dstDir := filepath.Join(public, "boards", b.ID)
		dst := filepath.Join(dstDir, "assembly-graph.json")
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "boardregistry.Materialize", "mkdir").
				With("path", dstDir)
		}
		if err := copyFile(src, dst); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "boardregistry.Materialize", "copy graph").
				With("from", src).With("to", dst)
		}
		entry := ViewerManifestEntry{
			ID:    b.ID,
			Label: b.Label,
			Graph: "/boards/" + b.ID + "/assembly-graph.json",
			Repo:  b.Repo,
		}
		viewerBoards = append(viewerBoards, entry)
	}
	manifest := ViewerManifest{
		DefaultBoard: opts.Registry.DefaultBoard,
		Boards:       viewerBoards,
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return terrors.Wrap(err, terrors.CodeInternal, "boardregistry.Materialize", "marshal json")
	}
	raw = append(raw, '\n')
	out := filepath.Join(public, "boards.json")
	if err := os.WriteFile(out, raw, 0o644); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "boardregistry.Materialize", "write boards.json").
			With("path", out)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dst)
}

// ValidID reports whether id is safe for ?board=, paths, and localStorage keys.
func ValidID(id string) bool {
	return idRE.MatchString(strings.TrimSpace(id))
}

// PrefixedID builds <prefix>-<stem> unless stem already starts with prefix-.
func PrefixedID(prefix, stem string) string {
	prefix = strings.TrimSpace(prefix)
	stem = strings.TrimSpace(stem)
	if prefix == "" {
		return stem
	}
	if stem == prefix || strings.HasPrefix(stem, prefix+"-") {
		return stem
	}
	return prefix + "-" + stem
}

// DefaultLabel builds "Repo · Slice" style labels.
func DefaultLabel(prefix, stem, explicit string) string {
	if strings.TrimSpace(explicit) != "" && prefix == "" {
		return strings.TrimSpace(explicit)
	}
	if prefix == "" {
		if explicit != "" {
			return strings.TrimSpace(explicit)
		}
		return stem
	}
	repoTitle := titleCase(prefix)
	if strings.TrimSpace(explicit) != "" {
		repoTitle = strings.TrimSpace(explicit)
	}
	bare := stem
	if strings.HasPrefix(stem, prefix+"-") {
		bare = strings.TrimPrefix(stem, prefix+"-")
	}
	if bare == "" || bare == prefix {
		return repoTitle
	}
	return repoTitle + " · " + titleCase(bare)
}

func titleCase(raw string) string {
	parts := strings.Split(raw, "-")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, strings.ToUpper(p[:1])+p[1:])
	}
	return strings.Join(out, " ")
}

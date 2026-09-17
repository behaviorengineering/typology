// Package assemblygraph builds a product-neutral package wiring graph for
// assembly-board style viewers: imports, observed roles, role-edge kinds, and
// wrong-way layer marks.
package assemblygraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/discover"
	"github.com/behaviorengineering/typology/internal/evidence"
	"github.com/behaviorengineering/typology/internal/sourceindex"
)

// DefaultRel is the default JSON path under a repo root (next to other
// tmp/typology evidence).
const DefaultRel = "tmp/typology/assembly-graph.json"

// DefaultBoardsRel is the default directory for --all-slices board JSON files.
const DefaultBoardsRel = "tmp/typology/boards"

// Graph is the portable assembly wiring document.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
	// Slice is set when this graph is a catalog slice projection.
	Slice string `json:"slice,omitempty"`
}

// BoundaryKind names why a package appears on a slice board without being owned.
const (
	BoundarySlice   = "slice"
	BoundaryLibrary = "library"
	BoundaryUnowned = "unowned"
)

// BindingStatus reports whether a boundary cable is allowed by the catalog.
const (
	BindingDeclared = "declared"
	BindingMissing  = "missing"
)

// Node is one package in the wiring graph.
type Node struct {
	ID             string   `json:"id"`
	Path           string   `json:"path"`
	InDegree       int      `json:"inDegree"`
	OutDegree      int      `json:"outDegree"`
	Imports        []string `json:"imports"`
	ImportedBy     []string `json:"importedBy"`
	IsHub          bool     `json:"isHub"`
	IsLeaf         bool     `json:"isLeaf"`
	Role           string   `json:"role,omitempty"`
	RoleConfidence float64  `json:"roleConfidence,omitempty"`
	Layer          int      `json:"layer"`
	// IsBoundary marks a lightweight stub for a package outside the selected slice.
	IsBoundary bool `json:"isBoundary,omitempty"`
	// BoundaryKind is slice, library, or unowned when IsBoundary is true.
	BoundaryKind string `json:"boundaryKind,omitempty"`
	// OwnerID is the catalog slice or library id that claims this stub, when known.
	OwnerID string `json:"ownerId,omitempty"`
}

// Edge is one directed import cable, optionally labeled by role kind.
type Edge struct {
	ID             string `json:"id"`
	Source         string `json:"source"`
	Target         string `json:"target"`
	RoleKind       string `json:"roleKind,omitempty"`
	WrongWay       bool   `json:"wrongWay,omitempty"`
	WrongWayReason string `json:"wrongWayReason,omitempty"`
	// BindingStatus is declared or missing for cables that cross the slice boundary.
	BindingStatus string `json:"bindingStatus,omitempty"`
	// BoundaryKind mirrors the external side of a boundary cable (slice/library/unowned).
	BoundaryKind string `json:"boundaryKind,omitempty"`
}

// BuildOptions configures an assembly graph harvest.
type BuildOptions struct {
	RepoRoot string
	// Modules is catalog scope.modules (repository-relative).
	Modules []string
	// Module is an optional single-module override (--module).
	Module string
}

// Build harvests package evidence and returns an assembly wiring graph.
func Build(opts BuildOptions) (Graph, error) {
	repo := strings.TrimSpace(opts.RepoRoot)
	if repo == "" {
		return Graph{}, terrors.New(terrors.CodeInvalid, "assemblygraph.Build", "repo root empty")
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return Graph{}, terrors.Wrap(err, terrors.CodeInvalid, "assemblygraph.Build", "abs repo").
			With("repo", repo)
	}
	h, err := evidence.Harvest(evidence.HarvestOptions{
		RepoRoot: absRepo,
		Modules:  opts.Modules,
		Module:   strings.TrimSpace(opts.Module),
	})
	if err != nil {
		return Graph{}, terrors.Wrap(err, terrors.CodeUnavailable, "assemblygraph.Build", "harvest").
			With("repo", absRepo)
	}
	return FromHarvest(h), nil
}

// FromHarvest builds a Graph from an existing evidence harvest (no I/O).
func FromHarvest(h evidence.Result) Graph {
	return fromSummary(discover.BuildGraphSummary(h.Graph), h.Topo)
}

// WriteJSON writes g as indented JSON (with trailing newline) to path.
func WriteJSON(path string, g Graph) error {
	out := strings.TrimSpace(path)
	if out == "" {
		return terrors.New(terrors.CodeInvalid, "assemblygraph.WriteJSON", "path empty")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "assemblygraph.WriteJSON", "mkdir").
			With("path", out)
	}
	raw, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return terrors.Wrap(err, terrors.CodeInternal, "assemblygraph.WriteJSON", "marshal")
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(out, raw, 0o644); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "assemblygraph.WriteJSON", "write").
			With("path", out)
	}
	return nil
}

// DefaultPath returns the default assembly-graph JSON path under repoRoot.
func DefaultPath(repoRoot string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(DefaultRel))
}

func fromSummary(summary discover.GraphSummary, topo sourceindex.RoleTopology) Graph {
	roleByPath := map[string]sourceindex.RoleNode{}
	for _, n := range topo.Packages {
		roleByPath[normPath(n.Path)] = n
	}
	kindByPair := map[string]string{}
	for _, e := range topo.Edges {
		kindByPair[normPath(e.From)+"->"+normPath(e.To)] = e.Kind
	}

	paths := make([]string, 0, len(summary.Nodes))
	for p := range summary.Nodes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	layerByID := map[string]int{}
	roleByID := map[string]string{}
	nodes := make([]Node, 0, len(paths))
	for _, p := range paths {
		n := summary.Nodes[p]
		id := NormalizeID(p)
		path := normPath(p)
		role := roleByPath[path]
		roleName := strings.TrimSpace(role.Role)
		if roleName == "" {
			roleName = sourceindex.RoleUnknown
		}
		layer := roleLayer(roleName)
		layerByID[id] = layer
		roleByID[id] = roleName
		nodes = append(nodes, Node{
			ID:             id,
			Path:           path,
			InDegree:       n.InDegree,
			OutDegree:      n.OutDegree,
			Imports:        normalizeList(n.Imports),
			ImportedBy:     normalizeList(n.ImportedBy),
			IsHub:          n.IsHub,
			IsLeaf:         n.IsLeaf,
			Role:           roleName,
			RoleConfidence: role.Confidence,
			Layer:          layer,
		})
	}

	edges := make([]Edge, 0)
	for _, n := range nodes {
		for _, dest := range n.Imports {
			tgtID := NormalizeID(dest)
			kind := kindByPair[n.Path+"->"+normPath(dest)]
			ww, reason := wrongWay(roleByID[n.ID], layerByID[n.ID], roleByID[tgtID], layerByID[tgtID])
			edges = append(edges, Edge{
				ID:             n.ID + "__" + tgtID,
				Source:         n.ID,
				Target:         tgtID,
				RoleKind:       kind,
				WrongWay:       ww,
				WrongWayReason: reason,
			})
		}
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	return Graph{Nodes: nodes, Edges: edges}
}

// roleLayer: lower = outer (may import inward); higher = inner (must not import outer).
func roleLayer(role string) int {
	switch role {
	case sourceindex.RoleEntrypoint:
		return 0
	case sourceindex.RoleHTTPSurface:
		return 1
	case sourceindex.RoleAggregator, sourceindex.RoleAdapter, sourceindex.RoleExecRunner, sourceindex.RoleUnknown:
		return 2
	case sourceindex.RoleDTO:
		return 3
	case sourceindex.RoleConfig, sourceindex.RoleObservability:
		return 4
	default:
		return 2
	}
}

func wrongWay(srcRole string, srcLayer int, tgtRole string, tgtLayer int) (bool, string) {
	if srcRole == "" || tgtRole == "" {
		return false, ""
	}
	if srcLayer > tgtLayer {
		return true, fmt.Sprintf("%s (layer %d) imports outer %s (layer %d)", srcRole, srcLayer, tgtRole, tgtLayer)
	}
	if (srcRole == sourceindex.RoleDTO || srcRole == sourceindex.RoleConfig || srcRole == sourceindex.RoleObservability) &&
		(tgtRole == sourceindex.RoleHTTPSurface || tgtRole == sourceindex.RoleEntrypoint || tgtRole == sourceindex.RoleAggregator) {
		return true, fmt.Sprintf("%s must not import %s", srcRole, tgtRole)
	}
	return false, ""
}

// NormalizeID maps a package path to a stable node id (slashes -> __).
func NormalizeID(path string) string {
	s := normPath(path)
	if s == "" || s == "." {
		return "root"
	}
	return strings.ReplaceAll(s, "/", "__")
}

func normPath(path string) string {
	return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(path), "./"))
}

func normalizeList(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		s := normPath(p)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

package assemblygraph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/behaviorengineering/typology/catalog"
	terrors "github.com/behaviorengineering/typology/errors"
)

// ProjectOptions configures a slice projection of a raw assembly graph.
type ProjectOptions struct {
	// SliceID selects one confirmed catalog slice.
	SliceID string
}

// ProjectStats summarizes a projected slice board for CLI reporting.
type ProjectStats struct {
	SliceID         string
	OwnedNodes      int
	BoundaryNodes   int
	InternalEdges   int
	BoundaryEdges   int
	MissingBindings int
	WrongWay        int
}

// Project keeps packages owned by one catalog slice as full nodes and turns
// cross-owner neighbors into lightweight boundary stubs. Inbound callers and
// outbound dependencies are both retained so the board shows where the slice
// is entered and where it reaches out.
func Project(raw Graph, typ catalog.Typology, opts ProjectOptions) (Graph, ProjectStats, error) {
	sliceID := strings.TrimSpace(opts.SliceID)
	if sliceID == "" {
		return Graph{}, ProjectStats{}, terrors.New(terrors.CodeInvalid, "assemblygraph.Project", "slice id empty")
	}
	slice, ok := typ.LookupSlice(sliceID)
	if !ok {
		return Graph{}, ProjectStats{}, terrors.New(terrors.CodeNotFound, "assemblygraph.Project",
			fmt.Sprintf("slice %q not in catalog", sliceID)).With("slice", sliceID)
	}

	ownedPaths := map[string]struct{}{}
	for _, c := range slice.AllComponents() {
		p := normPath(c.Path)
		if p == "" || p == "." {
			continue
		}
		ownedPaths[p] = struct{}{}
	}
	if len(ownedPaths) == 0 {
		return Graph{}, ProjectStats{}, terrors.New(terrors.CodeFailedPrecondition, "assemblygraph.Project",
			fmt.Sprintf("slice %q owns no component paths", sliceID)).With("slice", sliceID)
	}

	pathOwner, pathComp := ownershipIndex(typ)
	rawByPath := map[string]Node{}
	rawByID := map[string]Node{}
	for _, n := range raw.Nodes {
		p := normPath(n.Path)
		rawByPath[p] = n
		rawByID[n.ID] = n
	}

	ownedPresent := 0
	for p := range ownedPaths {
		if _, ok := rawByPath[p]; ok {
			ownedPresent++
		}
	}
	if ownedPresent == 0 {
		return Graph{}, ProjectStats{}, terrors.New(terrors.CodeFailedPrecondition, "assemblygraph.Project",
			fmt.Sprintf("slice %q: none of %d owned paths appear in the observed graph", sliceID, len(ownedPaths))).
			With("slice", sliceID)
	}

	keepIDs := map[string]struct{}{}
	stubMeta := map[string]stubInfo{}
	var keptEdges []Edge

	for _, e := range raw.Edges {
		srcPath := pathOf(rawByID, e.Source)
		tgtPath := pathOf(rawByID, e.Target)
		if srcPath == "" || tgtPath == "" {
			continue
		}
		srcOwned := inSet(ownedPaths, srcPath)
		tgtOwned := inSet(ownedPaths, tgtPath)
		if !srcOwned && !tgtOwned {
			continue
		}
		edge := e
		if srcOwned && tgtOwned {
			keptEdges = append(keptEdges, edge)
			keepIDs[e.Source] = struct{}{}
			keepIDs[e.Target] = struct{}{}
			continue
		}
		// Boundary: one side outside the selected slice.
		var externalPath, externalID string
		if srcOwned {
			externalPath, externalID = tgtPath, e.Target
			keepIDs[e.Source] = struct{}{}
		} else {
			externalPath, externalID = srcPath, e.Source
			keepIDs[e.Target] = struct{}{}
		}
		keepIDs[externalID] = struct{}{}
		info := classifyStub(externalPath, pathOwner)
		stubMeta[externalID] = info
		edge.BoundaryKind = info.Kind
		edge.BindingStatus = bindingStatus(typ, sliceID, srcPath, tgtPath, srcOwned, pathOwner, pathComp)
		keptEdges = append(keptEdges, edge)
	}

	// Owned packages with no edges still appear as isolated posts.
	for p := range ownedPaths {
		n, ok := rawByPath[p]
		if !ok {
			continue
		}
		keepIDs[n.ID] = struct{}{}
	}

	nodes := make([]Node, 0, len(keepIDs))
	for id := range keepIDs {
		n, ok := rawByID[id]
		if !ok {
			continue
		}
		if info, isStub := stubMeta[id]; isStub {
			n.IsBoundary = true
			n.BoundaryKind = info.Kind
			n.OwnerID = info.OwnerID
			n.Imports = nil
			n.ImportedBy = nil
			n.IsHub = false
			n.IsLeaf = true
		} else {
			n.IsBoundary = false
			n.BoundaryKind = ""
			n.OwnerID = sliceID
		}
		nodes = append(nodes, n)
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(keptEdges, func(i, j int) bool { return keptEdges[i].ID < keptEdges[j].ID })
	recomputeDegrees(nodes, keptEdges)

	stats := ProjectStats{SliceID: sliceID}
	for _, n := range nodes {
		if n.IsBoundary {
			stats.BoundaryNodes++
		} else {
			stats.OwnedNodes++
		}
	}
	for _, e := range keptEdges {
		if e.BindingStatus != "" {
			stats.BoundaryEdges++
			if e.BindingStatus == BindingMissing {
				stats.MissingBindings++
			}
		} else {
			stats.InternalEdges++
		}
		if e.WrongWay {
			stats.WrongWay++
		}
	}

	return Graph{Nodes: nodes, Edges: keptEdges, Slice: sliceID}, stats, nil
}

type stubInfo struct {
	Kind    string
	OwnerID string
}

func ownershipIndex(typ catalog.Typology) (map[string]catalog.ComponentOwner, map[string]string) {
	pathOwner := map[string]catalog.ComponentOwner{}
	pathComp := map[string]string{}
	for _, s := range typ.Slices {
		for _, c := range s.AllComponents() {
			p := normPath(c.Path)
			if p == "" {
				continue
			}
			pathOwner[p] = catalog.ComponentOwner{Kind: catalog.OwnerSlice, ID: s.ID}
			pathComp[p] = c.ID
		}
	}
	for _, lib := range typ.Libraries {
		for _, c := range lib.Owns {
			p := normPath(c.Path)
			if p == "" {
				continue
			}
			pathOwner[p] = catalog.ComponentOwner{Kind: catalog.OwnerLibrary, ID: lib.ID}
			pathComp[p] = c.ID
		}
	}
	return pathOwner, pathComp
}

func classifyStub(path string, pathOwner map[string]catalog.ComponentOwner) stubInfo {
	owner, ok := pathOwner[path]
	if !ok || owner.ID == "" {
		return stubInfo{Kind: BoundaryUnowned}
	}
	switch owner.Kind {
	case catalog.OwnerLibrary:
		return stubInfo{Kind: BoundaryLibrary, OwnerID: owner.ID}
	case catalog.OwnerSlice:
		return stubInfo{Kind: BoundarySlice, OwnerID: owner.ID}
	default:
		return stubInfo{Kind: BoundaryUnowned}
	}
}

func bindingStatus(
	typ catalog.Typology,
	selectedSlice string,
	srcPath, tgtPath string,
	srcOwned bool,
	pathOwner map[string]catalog.ComponentOwner,
	pathComp map[string]string,
) string {
	srcOwner := pathOwner[srcPath]
	tgtOwner := pathOwner[tgtPath]
	srcComp := pathComp[srcPath]
	tgtComp := pathComp[tgtPath]

	// Component-level allowance (must / reads) counts as declared.
	if srcComp != "" && tgtComp != "" && hasComponentAllowance(typ, srcComp, tgtComp) {
		return BindingDeclared
	}

	var fromOwner, toOwner string
	if srcOwned {
		fromOwner = selectedSlice
		toOwner = tgtOwner.ID
		if toOwner == "" {
			return BindingMissing
		}
	} else {
		fromOwner = srcOwner.ID
		toOwner = selectedSlice
		if fromOwner == "" {
			return BindingMissing
		}
	}
	if hasSliceBinding(typ, fromOwner, toOwner) {
		return BindingDeclared
	}
	return BindingMissing
}

func hasSliceBinding(typ catalog.Typology, from, to string) bool {
	for _, b := range typ.SliceBindings {
		if b.From == from && b.To == to {
			return true
		}
	}
	return false
}

func hasComponentAllowance(typ catalog.Typology, fromComp, toComp string) bool {
	for _, b := range typ.ComponentBindings {
		if b.From != fromComp || b.To != toComp {
			continue
		}
		switch b.Rule {
		case catalog.BindingMust, catalog.BindingReads:
			return true
		}
	}
	return false
}

func pathOf(byID map[string]Node, id string) string {
	n, ok := byID[id]
	if !ok {
		return ""
	}
	return normPath(n.Path)
}

func inSet(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}

func recomputeDegrees(nodes []Node, edges []Edge) {
	inDeg := map[string]int{}
	outDeg := map[string]int{}
	imports := map[string][]string{}
	importedBy := map[string][]string{}
	pathByID := map[string]string{}
	for _, n := range nodes {
		pathByID[n.ID] = n.Path
	}
	for _, e := range edges {
		outDeg[e.Source]++
		inDeg[e.Target]++
		if p := pathByID[e.Target]; p != "" {
			imports[e.Source] = append(imports[e.Source], p)
		}
		if p := pathByID[e.Source]; p != "" {
			importedBy[e.Target] = append(importedBy[e.Target], p)
		}
	}
	for i := range nodes {
		n := &nodes[i]
		n.InDegree = inDeg[n.ID]
		n.OutDegree = outDeg[n.ID]
		n.Imports = normalizeList(imports[n.ID])
		n.ImportedBy = normalizeList(importedBy[n.ID])
		if n.IsBoundary {
			n.IsHub = false
			n.IsLeaf = true
			continue
		}
		degree := n.InDegree + n.OutDegree
		n.IsHub = degree >= 4
		n.IsLeaf = n.OutDegree == 0
	}
}

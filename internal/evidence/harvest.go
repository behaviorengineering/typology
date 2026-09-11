// Package evidence orchestrates multi-language package harvest for typology CLI.
package evidence

import (
	"path/filepath"
	"sort"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/discover"
	"github.com/behaviorengineering/typology/internal/gorepo"
	"github.com/behaviorengineering/typology/internal/pyrepo"
	"github.com/behaviorengineering/typology/internal/sourceindex"
)

// Result is a language-merged package index and role topology.
type Result struct {
	Index sourceindex.Index
	Topo  sourceindex.RoleTopology
	Graph map[string][]string
	HasGo bool
	HasPy bool
}

// Harvest builds evidence for all supported languages under repoRoot.
// moduleScope applies only to Go module selection.
func Harvest(repoRoot, moduleScope string) (Result, error) {
	repo := strings.TrimSpace(repoRoot)
	if repo == "" {
		return Result{}, terrors.New(terrors.CodeInvalid, "evidence.Harvest", "repo root empty")
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return Result{}, terrors.Wrap(err, terrors.CodeInvalid, "evidence.Harvest", "abs repo").
			With("repo", repo)
	}

	goModules, goErr := gorepo.ResolveModules(absRepo, nil, moduleScope)
	hasGo := goErr == nil && len(goModules) > 0

	pyRoots, pyErr := pyrepo.FindRoots(absRepo)
	if pyErr != nil {
		return Result{}, pyErr
	}
	hasPy := len(pyRoots) > 0

	if !hasGo && !hasPy {
		if goErr != nil {
			return Result{}, goErr
		}
		return Result{}, terrors.New(terrors.CodeFailedPrecondition, "evidence.Harvest",
			"no Go modules or Python project roots found")
	}

	merged := sourceindex.Index{Packages: map[string]sourceindex.PackageEvidence{}}
	mergedGraph := map[string][]string{}

	if hasGo {
		idx, err := sourceindex.BuildInModules(absRepo, goModules)
		if err != nil {
			return Result{}, err
		}
		graph, err := discover.ImportGraphInModules(absRepo, goModules)
		if err != nil {
			return Result{}, err
		}
		mergeIndex(merged, idx)
		mergeGraph(mergedGraph, graph)
	}
	if hasPy {
		idx, graph, err := sourceindex.BuildPython(absRepo)
		if err != nil {
			return Result{}, err
		}
		if len(idx.Packages) == 0 {
			return Result{}, terrors.New(terrors.CodeFailedPrecondition, "evidence.Harvest",
				"Python project root found but no packages harvested")
		}
		mergeIndex(merged, idx)
		mergeGraph(mergedGraph, graph)
	}

	topo := sourceindex.BuildRoleTopology(merged, mergedGraph)
	return Result{
		Index: merged,
		Topo:  topo,
		Graph: mergedGraph,
		HasGo: hasGo,
		HasPy: hasPy,
	}, nil
}

// WriteFiles writes contracts, roles, and RLM context from a harvest result.
func WriteFiles(h Result, contractsOut, rolesOut, rlmContextOut string) error {
	return sourceindex.WriteEvidenceFilesFromHarvest(
		sourceindex.HarvestResult{
			Index: h.Index,
			Topo:  h.Topo,
			Graph: h.Graph,
			HasGo: h.HasGo,
			HasPy: h.HasPy,
		},
		contractsOut, rolesOut, rlmContextOut,
	)
}

func mergeIndex(dst sourceindex.Index, src sourceindex.Index) {
	for k, v := range src.Packages {
		dst.Packages[k] = v
	}
}

func mergeGraph(dst map[string][]string, src map[string][]string) {
	for from, tos := range src {
		seen := map[string]struct{}{}
		for _, t := range dst[from] {
			seen[t] = struct{}{}
		}
		for _, t := range tos {
			if _, ok := seen[t]; ok {
				continue
			}
			dst[from] = append(dst[from], t)
			seen[t] = struct{}{}
		}
		sort.Strings(dst[from])
	}
}

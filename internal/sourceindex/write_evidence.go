package sourceindex

import (
	"os"
	"path/filepath"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/gorepo"
	"gopkg.in/yaml.v3"
)

// HarvestResult is a language-merged package index and role topology.
type HarvestResult struct {
	Index Index
	Topo  RoleTopology
	Graph map[string][]string
	HasGo bool
	HasPy bool
}

// WriteEvidenceFilesFromHarvest writes contracts, roles, and RLM context from a harvest.
func WriteEvidenceFilesFromHarvest(h HarvestResult, contractsOut, rolesOut, rlmContextOut string) error {
	if len(h.Index.Packages) == 0 {
		return terrors.New(terrors.CodeFailedPrecondition, "sourceindex.WriteEvidenceFilesFromHarvest",
			"harvest has no packages")
	}
	idx := h.Index
	topo := h.Topo

	if strings.TrimSpace(contractsOut) != "" {
		if err := os.MkdirAll(filepath.Dir(contractsOut), 0o755); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WriteEvidenceFilesFromHarvest", "mkdir contracts").
				With("path", contractsOut)
		}
		if err := os.WriteFile(contractsOut, []byte(FormatPackageContractsMarkdownWithRoles(idx, topo)), 0o644); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WriteEvidenceFilesFromHarvest", "write contracts").
				With("path", contractsOut)
		}
	}
	if strings.TrimSpace(rolesOut) != "" {
		if err := os.MkdirAll(filepath.Dir(rolesOut), 0o755); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WriteEvidenceFilesFromHarvest", "mkdir roles").
				With("path", rolesOut)
		}
		data, err := yaml.Marshal(topo)
		if err != nil {
			return terrors.Wrap(err, terrors.CodeInternal, "sourceindex.WriteEvidenceFilesFromHarvest", "marshal roles")
		}
		if err := os.WriteFile(rolesOut, data, 0o644); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WriteEvidenceFilesFromHarvest", "write roles").
				With("path", rolesOut)
		}
	}
	rlmOut := strings.TrimSpace(rlmContextOut)
	if rlmOut == "" && strings.TrimSpace(rolesOut) != "" {
		rlmOut = filepath.Join(filepath.Dir(rolesOut), "package_rlm_context.md")
	}
	if rlmOut != "" {
		if err := os.MkdirAll(filepath.Dir(rlmOut), 0o755); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WriteEvidenceFilesFromHarvest", "mkdir rlm context").
				With("path", rlmOut)
		}
		if err := os.WriteFile(rlmOut, []byte(FormatPackageRLMContextMarkdown(idx, topo)), 0o644); err != nil {
			return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WriteEvidenceFilesFromHarvest", "write rlm context").
				With("path", rlmOut)
		}
	}
	return nil
}

// WriteEvidenceFilesWithRLM writes contracts, roles, and optional RLM context markdown.
// When modules is empty, callers should use evidence.Harvest instead.
func WriteEvidenceFilesWithRLM(repoRoot string, modules []gorepo.Module, contractsOut, rolesOut, rlmContextOut string, importGraph map[string][]string) error {
	if len(modules) == 0 {
		return terrors.New(terrors.CodeInvalid, "sourceindex.WriteEvidenceFilesWithRLM",
			"modules empty; use evidence.Harvest for multi-language repos")
	}
	idx, err := BuildInModules(repoRoot, modules)
	if err != nil {
		return err
	}
	topo := BuildRoleTopology(idx, importGraph)
	return WriteEvidenceFilesFromHarvest(HarvestResult{Index: idx, Topo: topo, Graph: importGraph, HasGo: true},
		contractsOut, rolesOut, rlmContextOut)
}

// Package roles exposes observed package-role topology and mechanical grouping
// for consumers such as Majordomo. Classification itself lives in Typology's
// source index; this package is the public YAML and grouping bridge.
package roles

import (
	"fmt"

	"github.com/behaviorengineering/typology/internal/sourceindex"
	"gopkg.in/yaml.v3"
)

// Topology is the public package-role map written as package_roles.yaml.
type Topology struct {
	Packages []Node `yaml:"packages" json:"packages"`
	Edges    []Edge `yaml:"edges,omitempty" json:"edges,omitempty"`
}

// Node is one observed package role.
type Node struct {
	Path           string   `yaml:"path" json:"path"`
	Role           string   `yaml:"role" json:"role"`
	Confidence     float64  `yaml:"confidence" json:"confidence"`
	Evidence       []string `yaml:"evidence,omitempty" json:"evidence,omitempty"`
	InspectedStage int      `yaml:"inspected_stage" json:"inspected_stage"`
	CandidateRole  string   `yaml:"candidate_role,omitempty" json:"candidate_role,omitempty"`
}

// Edge is a labeled import between observed packages.
type Edge struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to" json:"to"`
	Kind string `yaml:"kind" json:"kind"`
}

// Grouping is the deterministic seed derived from roles and edges.
type Grouping = sourceindex.MechanicalGrouping

// Component is one connected aggregator/adapter set.
type Component = sourceindex.MechanicalComponent

// ParseYAML loads a package_roles.yaml document.
func ParseYAML(raw []byte) (Topology, error) {
	var topo Topology
	if err := yaml.Unmarshal(raw, &topo); err != nil {
		return Topology{}, fmt.Errorf("roles.ParseYAML: %w", err)
	}
	return topo, nil
}

// BuildGrouping derives deterministic groups from a public topology.
func BuildGrouping(topo Topology) Grouping {
	return sourceindex.BuildMechanicalGrouping(toInternal(topo))
}

// BuildGroupingFromYAML parses package_roles YAML and builds mechanical groups.
func BuildGroupingFromYAML(raw []byte) (Grouping, error) {
	topo, err := ParseYAML(raw)
	if err != nil {
		return Grouping{}, err
	}
	return BuildGrouping(topo), nil
}

// FormatGroupingMarkdown renders the seed for LLM cluster input.
func FormatGroupingMarkdown(g Grouping) string {
	return sourceindex.FormatMechanicalGroupingMarkdown(g)
}

func toInternal(topo Topology) sourceindex.RoleTopology {
	out := sourceindex.RoleTopology{
		Packages: make([]sourceindex.RoleNode, 0, len(topo.Packages)),
		Edges:    make([]sourceindex.RoleEdge, 0, len(topo.Edges)),
	}
	for _, n := range topo.Packages {
		out.Packages = append(out.Packages, sourceindex.RoleNode{
			Path:           n.Path,
			Role:           n.Role,
			Confidence:     n.Confidence,
			Evidence:       n.Evidence,
			InspectedStage: n.InspectedStage,
			CandidateRole:  n.CandidateRole,
		})
	}
	for _, e := range topo.Edges {
		out.Edges = append(out.Edges, sourceindex.RoleEdge{
			From: e.From,
			To:   e.To,
			Kind: e.Kind,
		})
	}
	return out
}

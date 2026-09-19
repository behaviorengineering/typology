package sourceindex_test

import (
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestClassifyPackage_softShapeDTOAllowsHelpers(t *testing.T) {
	t.Parallel()
	ev := sourceindex.PackageEvidence{
		Path:          "internal/payloads",
		Name:          "payloads",
		JSONTags:      true,
		ExportedDecls: []string{"EventPayload", "ListPayload", "DatedItem"},
		ExportedFuncs: []string{"EventURL", "ListForInstance", "Validate"},
	}
	topo := sourceindex.BuildRoleTopology(sourceindex.Index{
		Packages: map[string]sourceindex.PackageEvidence{
			"internal/payloads": ev,
			"internal/server": {
				Path: "internal/server", Name: "server", HasMain: true,
			},
		},
	}, map[string][]string{
		"internal/server":   {"internal/payloads"},
		"internal/payloads": {},
	})
	var payloads sourceindex.RoleNode
	for _, n := range topo.Packages {
		if n.Path == "internal/payloads" {
			payloads = n
			break
		}
	}
	if payloads.Role != sourceindex.RoleDTO {
		t.Fatalf("payloads role=%q want dto evidence=%v", payloads.Role, payloads.Evidence)
	}
	if !strings.Contains(strings.Join(payloads.Evidence, ","), "decl_heavy_export_surface") {
		t.Fatalf("expected decl_heavy evidence, got %v", payloads.Evidence)
	}
	if !topo.HasEdge("internal/server", "internal/payloads", sourceindex.EdgeFillsDTO) {
		t.Fatalf("expected fills_dto into soft dto; edges=%v", topo.Edges)
	}
}

func TestClassifyPackage_softShapeDTORejectsHTTP(t *testing.T) {
	t.Parallel()
	ev := sourceindex.PackageEvidence{
		Path:           "internal/lexapi",
		Name:           "lexapi",
		JSONTags:       true,
		ImportsNetHTTP: true,
		ExportedDecls:  []string{"Payload", "SheetRow"},
		ExportedFuncs:  []string{"MountWithResolver", "PayloadForInstance"},
	}
	topo := sourceindex.BuildRoleTopology(sourceindex.Index{
		Packages: map[string]sourceindex.PackageEvidence{"internal/lexapi": ev},
	}, nil)
	if len(topo.Packages) != 1 || topo.Packages[0].Role == sourceindex.RoleDTO {
		t.Fatalf("http package must not be dto, got %+v", topo.Packages)
	}
}

func TestClassifyPackage_softShapeDTORejectsFuncHeavy(t *testing.T) {
	t.Parallel()
	ev := sourceindex.PackageEvidence{
		Path:          "internal/chronology",
		Name:          "chronology",
		JSONTags:      true,
		ExportedDecls: []string{"Row", "Spine"},
		ExportedFuncs: []string{"A", "B", "C", "D", "E", "F", "G"},
	}
	topo := sourceindex.BuildRoleTopology(sourceindex.Index{
		Packages: map[string]sourceindex.PackageEvidence{"internal/chronology": ev},
	}, nil)
	if len(topo.Packages) != 1 || topo.Packages[0].Role == sourceindex.RoleDTO {
		t.Fatalf("func-heavy package must not be dto, got %+v", topo.Packages)
	}
}

func TestClassifyPackage_pureJSONStillDTO(t *testing.T) {
	t.Parallel()
	ev := sourceindex.PackageEvidence{
		Path:          "internal/board",
		Name:          "board",
		JSONTags:      true,
		ExportedDecls: []string{"Row"},
	}
	topo := sourceindex.BuildRoleTopology(sourceindex.Index{
		Packages: map[string]sourceindex.PackageEvidence{"internal/board": ev},
	}, nil)
	if len(topo.Packages) != 1 || topo.Packages[0].Role != sourceindex.RoleDTO {
		t.Fatalf("pure json package want dto, got %+v", topo.Packages)
	}
}

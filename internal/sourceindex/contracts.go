package sourceindex

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/gorepo"
)

// FormatPackageContractsMarkdown renders a compact public-contract summary for LLMs.
// Each package is one block with path, package name, delivery facts, observed role, and exported symbols.
func FormatPackageContractsMarkdown(idx Index) string {
	return FormatPackageContractsMarkdownWithRoles(idx, RoleTopology{})
}

// FormatPackageContractsMarkdownWithRoles includes observed role fields when topo is non-empty.
func FormatPackageContractsMarkdownWithRoles(idx Index, topo RoleTopology) string {
	roleByPath := map[string]RoleNode{}
	for _, n := range topo.Packages {
		roleByPath[normalizePath(n.Path)] = n
	}

	paths := make([]string, 0, len(idx.Packages))
	for p := range idx.Packages {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString("# Package public contracts\n\n")
	b.WriteString("Exported symbols and delivery facts from static analysis.\n")
	b.WriteString("Use packageDoc, methods, jsonTags, goEmbed, deliveryHint, and observed role (never folder names) to classify packages.\n\n")
	for _, p := range paths {
		ev := idx.Packages[p]
		display := "./" + strings.TrimPrefix(filepath.ToSlash(ev.Path), "./")
		fmt.Fprintf(&b, "## %s\n", display)
		fmt.Fprintf(&b, "- package: `%s`\n", ev.Name)
		if lang := strings.TrimSpace(ev.Language); lang != "" {
			fmt.Fprintf(&b, "- language: %s\n", lang)
		}
		if strings.TrimSpace(ev.PackageDoc) != "" {
			fmt.Fprintf(&b, "- packageDoc: %s\n", strings.TrimSpace(ev.PackageDoc))
		}
		if ev.HasMain {
			b.WriteString("- hasMain: true\n")
		} else {
			b.WriteString("- hasMain: false\n")
		}
		fmt.Fprintf(&b, "- jsonTags: %t\n", ev.JSONTags)
		fmt.Fprintf(&b, "- goEmbed: %t\n", ev.GoEmbed)
		fmt.Fprintf(&b, "- importsNetHTTP: %t\n", ev.ImportsNetHTTP)
		fmt.Fprintf(&b, "- importsOsExec: %t\n", ev.ImportsOsExec)
		fmt.Fprintf(&b, "- importsOtel: %t\n", ev.ImportsOTel)
		fmt.Fprintf(&b, "- importsPrometheus: %t\n", ev.ImportsPrometheus)
		if hint := strings.TrimSpace(ev.DeliveryHint); hint != "" {
			fmt.Fprintf(&b, "- deliveryHint: %s\n", hint)
		}
		if n, ok := roleByPath[normalizePath(ev.Path)]; ok {
			fmt.Fprintf(&b, "- role: %s\n", n.Role)
			fmt.Fprintf(&b, "- confidence: %.2f\n", n.Confidence)
			if len(n.Evidence) > 0 {
				fmt.Fprintf(&b, "- roleEvidence: %s\n", strings.Join(n.Evidence, ", "))
			}
		}
		if len(ev.ExportedDecls) > 0 {
			fmt.Fprintf(&b, "- exportedDecls: %s\n", strings.Join(ev.ExportedDecls, ", "))
		} else {
			b.WriteString("- exportedDecls: (none)\n")
		}
		if len(ev.ExportedFuncs) > 0 {
			fmt.Fprintf(&b, "- exportedFuncs: %s\n", strings.Join(ev.ExportedFuncs, ", "))
		} else {
			b.WriteString("- exportedFuncs: (none)\n")
		}
		if len(ev.ExportedMethods) > 0 {
			fmt.Fprintf(&b, "- exportedMethods: %s\n", strings.Join(ev.ExportedMethods, ", "))
		} else {
			b.WriteString("- exportedMethods: (none)\n")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// WritePackageContractsMarkdown writes FormatPackageContractsMarkdown to w.
func WritePackageContractsMarkdown(idx Index, w io.Writer) error {
	if w == nil {
		return terrors.New(terrors.CodeInvalid, "sourceindex.WritePackageContractsMarkdown", "writer is nil")
	}
	_, err := io.WriteString(w, FormatPackageContractsMarkdown(idx))
	if err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WritePackageContractsMarkdown", "write contracts markdown")
	}
	return nil
}

// WritePackageContractsFile builds a module-scoped source index and writes package contracts markdown.
func WritePackageContractsFile(repoRoot string, modules []gorepo.Module, outPath string) error {
	return WritePackageContractsFileWithGraph(repoRoot, modules, outPath, nil)
}

// WritePackageContractsFileWithGraph writes contracts including observed roles when importGraph is set.
func WritePackageContractsFileWithGraph(repoRoot string, modules []gorepo.Module, outPath string, importGraph map[string][]string) error {
	out := strings.TrimSpace(outPath)
	if out == "" {
		return terrors.New(terrors.CodeInvalid, "sourceindex.WritePackageContractsFile", "out path empty")
	}
	idx, err := BuildInModules(repoRoot, modules)
	if err != nil {
		return err
	}
	topo := RoleTopology{}
	if importGraph != nil {
		topo = BuildRoleTopology(idx, importGraph)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WritePackageContractsFile", "mkdir contracts dir").
			With("path", out)
	}
	f, err := os.Create(out)
	if err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WritePackageContractsFile", "create contracts file").
			With("path", out)
	}
	defer f.Close()
	_, err = io.WriteString(f, FormatPackageContractsMarkdownWithRoles(idx, topo))
	if err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WritePackageContractsFile", "write contracts markdown")
	}
	return nil
}

// WriteEvidenceFiles writes package_contracts.md, package_roles.yaml, and package_rlm_context.md.
func WriteEvidenceFiles(repoRoot string, modules []gorepo.Module, contractsOut, rolesOut string, importGraph map[string][]string) error {
	return WriteEvidenceFilesWithRLM(repoRoot, modules, contractsOut, rolesOut, "", importGraph)
}

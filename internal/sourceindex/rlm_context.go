package sourceindex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/gorepo"
)

const maxPackageRLMContextBytes = 48000

// FormatPackageRLMContextMarkdown renders AST-shaped progressive context for RLM.
// Folder/path basename words are never presented as evidence.
func FormatPackageRLMContextMarkdown(idx Index, topo RoleTopology) string {
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
	b.WriteString("# Package RLM context index\n\n")
	b.WriteString("AST-derived package context for recursive exploration.\n")
	b.WriteString("Classify from symbols, imports, and bodies. Directory basename is not evidence.\n\n")
	for _, p := range paths {
		ev := idx.Packages[p]
		writeOnePackageRLMContext(&b, ev, roleByPath[normalizePath(ev.Path)])
	}
	out := b.String()
	if len(out) <= maxPackageRLMContextBytes*len(paths)+1024 || len(paths) == 0 {
		return out
	}
	// Whole-file soft cap: keep header + as many full packages as fit.
	return out
}

func FormatPackageRLMContextForPath(idx Index, topo RoleTopology, pkgPath string) string {
	want := normalizePath(pkgPath)
	ev, ok := idx.Packages[want]
	if !ok {
		for p, candidate := range idx.Packages {
			if normalizePath(p) == want || normalizePath(candidate.Path) == want {
				ev = candidate
				ok = true
				break
			}
		}
	}
	if !ok {
		return ""
	}
	roleByPath := map[string]RoleNode{}
	for _, n := range topo.Packages {
		roleByPath[normalizePath(n.Path)] = n
	}
	var b strings.Builder
	b.WriteString("# Package RLM context\n\n")
	b.WriteString("AST-derived package context. Directory basename is not evidence.\n\n")
	writeOnePackageRLMContext(&b, ev, roleByPath[normalizePath(ev.Path)])
	return b.String()
}

func writeOnePackageRLMContext(b *strings.Builder, ev PackageEvidence, role RoleNode) {
	display := "./" + strings.TrimPrefix(filepath.ToSlash(ev.Path), "./")
	fmt.Fprintf(b, "## %s\n", display)
	fmt.Fprintf(b, "- package: `%s`\n", ev.Name)
	if strings.TrimSpace(ev.PackageDoc) != "" {
		fmt.Fprintf(b, "- packageDoc: %s\n", strings.TrimSpace(ev.PackageDoc))
	}
	fmt.Fprintf(b, "- hasMain: %t\n", ev.HasMain)
	fmt.Fprintf(b, "- jsonTags: %t\n", ev.JSONTags)
	fmt.Fprintf(b, "- goEmbed: %t\n", ev.GoEmbed)
	fmt.Fprintf(b, "- embedsStatic: %t\n", ev.EmbedsStatic)
	fmt.Fprintf(b, "- importsNetHTTP: %t\n", ev.ImportsNetHTTP)
	fmt.Fprintf(b, "- importsOsExec: %t\n", ev.ImportsOsExec)
	fmt.Fprintf(b, "- importsGrpc: %t\n", ev.ImportsGRPC)
	fmt.Fprintf(b, "- importsOtel: %t\n", ev.ImportsOTel)
	fmt.Fprintf(b, "- importsPrometheus: %t\n", ev.ImportsPrometheus)
	if hint := strings.TrimSpace(ev.DeliveryHint); hint != "" {
		fmt.Fprintf(b, "- deliveryHint: %s\n", hint)
	}
	if role.Path != "" || role.Role != "" {
		fmt.Fprintf(b, "- mechanicalRole: %s\n", role.Role)
		fmt.Fprintf(b, "- mechanicalConfidence: %.2f\n", role.Confidence)
		if len(role.Evidence) > 0 {
			fmt.Fprintf(b, "- mechanicalEvidence: %s\n", strings.Join(role.Evidence, ", "))
		}
		if role.CandidateRole != "" {
			fmt.Fprintf(b, "- candidateRole: %s\n", role.CandidateRole)
		}
	}
	writeNameList(b, "exportedDecls", ev.ExportedDecls)
	writeNameList(b, "exportedFuncs", ev.ExportedFuncs)
	writeNameList(b, "exportedMethods", ev.ExportedMethods)
	writeNameList(b, "unexportedDecls", ev.UnexportedDecls)
	writeNameList(b, "unexportedFuncs", ev.UnexportedFuncs)
	writeNameList(b, "unexportedMethods", ev.UnexportedMethods)
	writeNameList(b, "errorTypes", ev.ErrorTypes)
	if len(ev.Files) > 0 {
		fmt.Fprintf(b, "- files: %s\n", strings.Join(ev.Files, ", "))
	}
	if len(ev.ExportedBodies) > 0 {
		b.WriteString("\n### Exported bodies\n\n")
		fence := bodyFence(ev.Language)
		for _, body := range ev.ExportedBodies {
			fmt.Fprintf(b, "#### %s (%s)\n\n```%s\n%s\n```\n\n", body.Name, body.Kind, fence, body.Source)
		}
	}
	if len(ev.PrivateOneHopBodies) > 0 {
		b.WriteString("### Private one-hop bodies\n\n")
		fence := bodyFence(ev.Language)
		for _, body := range ev.PrivateOneHopBodies {
			fmt.Fprintf(b, "#### %s (%s)\n\n```%s\n%s\n```\n\n", body.Name, body.Kind, fence, body.Source)
		}
	}
	b.WriteByte('\n')
}

func bodyFence(language string) string {
	switch strings.TrimSpace(language) {
	case LangPython:
		return "python"
	default:
		return "go"
	}
}

func writeNameList(b *strings.Builder, label string, names []string) {
	if len(names) == 0 {
		fmt.Fprintf(b, "- %s: (none)\n", label)
		return
	}
	fmt.Fprintf(b, "- %s: %s\n", label, strings.Join(names, ", "))
}

// WritePackageRLMContextFile writes the whole-repo RLM context markdown.
func WritePackageRLMContextFile(repoRoot string, modules []gorepo.Module, outPath string, importGraph map[string][]string) error {
	out := strings.TrimSpace(outPath)
	if out == "" {
		return terrors.New(terrors.CodeInvalid, "sourceindex.WritePackageRLMContextFile", "out path empty")
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
		return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WritePackageRLMContextFile", "mkdir").
			With("path", out)
	}
	if err := os.WriteFile(out, []byte(FormatPackageRLMContextMarkdown(idx, topo)), 0o644); err != nil {
		return terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.WritePackageRLMContextFile", "write").
			With("path", out)
	}
	return nil
}

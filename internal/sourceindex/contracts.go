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
// Each package is one block with path, package name, hasMain, and exported symbols.
func FormatPackageContractsMarkdown(idx Index) string {
	paths := make([]string, 0, len(idx.Packages))
	for p := range idx.Packages {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString("# Package public contracts\n\n")
	b.WriteString("Exported symbols and main entrypoints from static Go analysis.\n")
	b.WriteString("Use this to place packages under owns[] vs surfaces[] (kind: cli requires a real CLI/main delivery package).\n\n")
	for _, p := range paths {
		ev := idx.Packages[p]
		display := "./" + strings.TrimPrefix(filepath.ToSlash(ev.Path), "./")
		fmt.Fprintf(&b, "## %s\n", display)
		fmt.Fprintf(&b, "- package: `%s`\n", ev.Name)
		if ev.HasMain {
			b.WriteString("- hasMain: true\n")
		} else {
			b.WriteString("- hasMain: false\n")
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
	out := strings.TrimSpace(outPath)
	if out == "" {
		return terrors.New(terrors.CodeInvalid, "sourceindex.WritePackageContractsFile", "out path empty")
	}
	idx, err := BuildInModules(repoRoot, modules)
	if err != nil {
		return err
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
	if err := WritePackageContractsMarkdown(idx, f); err != nil {
		return err
	}
	return nil
}

package sourceindex

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	terrors "github.com/behaviorengineering/typology/errors"
	"github.com/behaviorengineering/typology/internal/gorepo"
)

// Delivery hint values for fail-closed package classification.
const (
	DeliveryCLI        = "cli"
	DeliveryServerHTTP = "server-http"
	DeliveryServerUI   = "server-ui"
	DeliveryServerGRPC = "server-grpc"
	DeliveryDTO        = "dto"
)

// PackageEvidence summarizes static source evidence for one Go package.
type PackageEvidence struct {
	Path             string   `json:"path"`
	Name             string   `json:"name"`
	Files            []string `json:"files,omitempty"`
	PackageDoc       string   `json:"packageDoc,omitempty"`
	ExportedDecls    []string `json:"exportedDecls,omitempty"`
	ExportedFuncs    []string `json:"exportedFuncs,omitempty"`
	ExportedMethods  []string `json:"exportedMethods,omitempty"`
	HasMain          bool     `json:"hasMain,omitempty"`
	JSONTags         bool     `json:"jsonTags,omitempty"`
	GoEmbed          bool     `json:"goEmbed,omitempty"`
	EmbedsStatic     bool     `json:"embedsStatic,omitempty"`
	ImportsNetHTTP       bool   `json:"importsNetHTTP,omitempty"`
	ImportsOsExec        bool   `json:"importsOsExec,omitempty"`
	ImportsGRPC          bool   `json:"importsGrpc,omitempty"`
	ImportsOTel          bool   `json:"importsOtel,omitempty"`
	ImportsPrometheus    bool   `json:"importsPrometheus,omitempty"`
	HTTPSurfaceIdent     bool   `json:"httpSurfaceIdent,omitempty"`
	GRPCServerIdent      bool   `json:"grpcServerIdent,omitempty"`
	DeliveryHint         string `json:"deliveryHint,omitempty"`
}

// HasStaticAnchor reports whether the package has at least one exported symbol
// or a main entrypoint that can act as a deterministic source anchor.
func (p PackageEvidence) HasStaticAnchor() bool {
	return p.HasMain || len(p.ExportedDecls) > 0 || len(p.ExportedFuncs) > 0 || len(p.ExportedMethods) > 0
}

// Index is a source evidence map keyed by normalized package path.
type Index struct {
	Packages map[string]PackageEvidence `json:"packages"`
}

// Build scans the repo and indexes local Go packages using go list plus AST.
func Build(repoRoot string) (Index, error) {
	repo := strings.TrimSpace(repoRoot)
	if repo == "" {
		return Index{}, terrors.New(terrors.CodeInvalid, "sourceindex.Build", "repo root empty")
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return Index{}, terrors.Wrap(err, terrors.CodeInvalid, "sourceindex.Build", "abs repo").
			With("repo", repo)
	}
	modules, err := gorepo.ResolveModules(absRepo, nil, "")
	if err != nil {
		return Index{}, err
	}
	return BuildInModules(absRepo, modules)
}

// BuildInModules scans only the selected Go modules for source evidence.
func BuildInModules(repoRoot string, modules []gorepo.Module) (Index, error) {
	repo := strings.TrimSpace(repoRoot)
	if repo == "" {
		return Index{}, terrors.New(terrors.CodeInvalid, "sourceindex.BuildInModules", "repo root empty")
	}
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return Index{}, terrors.Wrap(err, terrors.CodeInvalid, "sourceindex.BuildInModules", "abs repo").
			With("repo", repo)
	}
	if resolved, err := filepath.EvalSymlinks(absRepo); err == nil {
		absRepo = resolved
	}
	pkgs, err := listPackagesInModules(modules)
	if err != nil {
		return Index{}, err
	}
	index := Index{Packages: make(map[string]PackageEvidence, len(pkgs))}
	for _, pkg := range pkgs {
		ev, err := scanPackage(absRepo, pkg)
		if err != nil {
			return Index{}, err
		}
		absDir, err := filepath.Abs(pkg.Dir)
		if err != nil {
			return Index{}, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.Build", "abs package dir").
				With("dir", pkg.Dir)
		}
		if resolved, err := filepath.EvalSymlinks(absDir); err == nil {
			absDir = resolved
		}
		rel, err := filepath.Rel(absRepo, absDir)
		if err != nil {
			return Index{}, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.Build", "rel package dir").
				With("dir", absDir)
		}
		ev.Path = normalizePath(rel)
		index.Packages[normalizePath(rel)] = ev
	}
	return index, nil
}

// Package returns the evidence for a normalized or relative package path.
func (i Index) Package(path string) (PackageEvidence, bool) {
	if len(i.Packages) == 0 {
		return PackageEvidence{}, false
	}
	want := normalizePath(path)
	if ev, ok := i.Packages[want]; ok {
		return ev, true
	}
	for key, ev := range i.Packages {
		key = normalizePath(key)
		if key == want || strings.HasSuffix(key, "/"+want) {
			return ev, true
		}
	}
	return PackageEvidence{}, false
}

// HasPackage reports whether the package path exists in the index.
func (i Index) HasPackage(path string) bool {
	_, ok := i.Package(path)
	return ok
}

// AnchoredPackages returns the number of packages with static anchors.
func (i Index) AnchoredPackages() int {
	count := 0
	for _, ev := range i.Packages {
		if ev.HasStaticAnchor() {
			count++
		}
	}
	return count
}

type listPackage struct {
	Dir             string   `json:"Dir"`
	Name            string   `json:"Name"`
	GoFiles         []string `json:"GoFiles"`
	CompiledGoFiles []string `json:"CompiledGoFiles"`
}

func listPackagesInModules(modules []gorepo.Module) ([]listPackage, error) {
	var all []listPackage
	for _, mod := range modules {
		pkgs, err := listPackagesInModule(mod.Dir)
		if err != nil {
			return nil, err
		}
		all = append(all, pkgs...)
	}
	return all, nil
}

func listPackagesInModule(moduleRoot string) ([]listPackage, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = moduleRoot
	// Isolate each module from an enclosing workspace so sibling modules stay out of scope.
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.listPackages", "go list failed").
				With("stderr", strings.TrimSpace(string(ee.Stderr))).
				With("dir", moduleRoot)
		}
		return nil, terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.listPackages", "go list").
			With("dir", moduleRoot)
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	var pkgs []listPackage
	for {
		var pkg listPackage
		if err := dec.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			return nil, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.listPackages", "decode go list json").
				With("dir", moduleRoot)
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

func scanPackage(repoRoot string, pkg listPackage) (PackageEvidence, error) {
	absDir, err := filepath.Abs(pkg.Dir)
	if err != nil {
		return PackageEvidence{}, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.scanPackage", "abs package dir").
			With("dir", pkg.Dir)
	}
	files := pkg.GoFiles
	if len(files) == 0 {
		files = pkg.CompiledGoFiles
	}
	ev := PackageEvidence{
		Path:  normalizePath(pkg.Dir),
		Name:  pkg.Name,
		Files: make([]string, 0, len(files)),
	}
	fset := token.NewFileSet()
	exportedDecls := map[string]struct{}{}
	exportedFuncs := map[string]struct{}{}
	exportedMethods := map[string]struct{}{}
	httpSurfaceIdent := false
	grpcServerIdent := false
	for _, file := range files {
		abs := filepath.Join(absDir, file)
		rel, err := filepath.Rel(repoRoot, abs)
		if err != nil {
			return PackageEvidence{}, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.scanPackage", "rel file").
				With("file", abs)
		}
		ev.Files = append(ev.Files, normalizePath(rel))
		parsed, err := parser.ParseFile(fset, abs, nil, parser.ParseComments)
		if err != nil {
			return PackageEvidence{}, terrors.Wrap(err, terrors.CodeInvalid, "sourceindex.scanPackage", "parse file").
				With("file", abs)
		}
		if ev.PackageDoc == "" && parsed.Doc != nil {
			ev.PackageDoc = compactPackageDoc(parsed.Doc.Text())
		}
		for _, imp := range parsed.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			switch path {
			case "net/http":
				ev.ImportsNetHTTP = true
			case "os/exec":
				ev.ImportsOsExec = true
			case "google.golang.org/grpc":
				ev.ImportsGRPC = true
			}
			if strings.HasPrefix(path, "go.opentelemetry.io/") {
				ev.ImportsOTel = true
			}
			if strings.HasPrefix(path, "github.com/prometheus/client_golang") {
				ev.ImportsPrometheus = true
			}
		}
		for _, decl := range parsed.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.Name == "main" && pkg.Name == "main" && d.Recv == nil {
					ev.HasMain = true
				}
				if !ast.IsExported(d.Name.Name) {
					continue
				}
				if isGRPCServerIdent(d.Name.Name) {
					grpcServerIdent = true
				}
				if d.Recv != nil {
					recvType := receiverTypeName(d.Recv)
					if recvType == "" {
						continue
					}
					key := recvType + "." + d.Name.Name
					exportedMethods[key] = struct{}{}
					if isHTTPSurfaceIdent(d.Name.Name) {
						httpSurfaceIdent = true
					}
					continue
				}
				exportedFuncs[d.Name.Name] = struct{}{}
				if isHTTPSurfaceIdent(d.Name.Name) {
					httpSurfaceIdent = true
				}
			case *ast.GenDecl:
				if hasGoEmbed(d.Doc) {
					ev.GoEmbed = true
					if goEmbedLooksStatic(d.Doc) {
						ev.EmbedsStatic = true
					}
				}
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(s.Name.Name) {
							exportedDecls[s.Name.Name] = struct{}{}
							if isHTTPSurfaceIdent(s.Name.Name) {
								httpSurfaceIdent = true
							}
							if isGRPCServerIdent(s.Name.Name) {
								grpcServerIdent = true
							}
						}
						if structHasJSONTag(s.Type) {
							ev.JSONTags = true
						}
					case *ast.ValueSpec:
						if hasGoEmbed(s.Doc) || hasGoEmbed(d.Doc) {
							ev.GoEmbed = true
							if goEmbedLooksStatic(s.Doc) || goEmbedLooksStatic(d.Doc) {
								ev.EmbedsStatic = true
							}
						}
						for _, name := range s.Names {
							if ast.IsExported(name.Name) {
								exportedDecls[name.Name] = struct{}{}
								if isHTTPSurfaceIdent(name.Name) {
									httpSurfaceIdent = true
								}
								if isGRPCServerIdent(name.Name) {
									grpcServerIdent = true
								}
							}
						}
					}
				}
			}
		}
	}
	ev.ExportedDecls = sortedKeys(exportedDecls)
	ev.ExportedFuncs = sortedKeys(exportedFuncs)
	ev.ExportedMethods = sortedKeys(exportedMethods)
	ev.HTTPSurfaceIdent = httpSurfaceIdent
	ev.GRPCServerIdent = grpcServerIdent
	ev.DeliveryHint = deliveryHint(ev, httpSurfaceIdent)
	return ev, nil
}

func deliveryHint(ev PackageEvidence, httpSurfaceIdent bool) string {
	if ev.HasMain {
		return DeliveryCLI
	}
	if ev.GoEmbed && ev.EmbedsStatic {
		return DeliveryServerUI
	}
	if ev.GoEmbed || (ev.ImportsNetHTTP && httpSurfaceIdent) {
		return DeliveryServerHTTP
	}
	if ev.ImportsGRPC && ev.GRPCServerIdent {
		return DeliveryServerGRPC
	}
	if ev.JSONTags && len(ev.ExportedFuncs) == 0 && len(ev.ExportedMethods) == 0 {
		return DeliveryDTO
	}
	return ""
}

func isHTTPSurfaceIdent(name string) bool {
	switch name {
	case "NewMux", "NewHandler", "Handler", "ServeHTTP":
		return true
	default:
		return false
	}
}

func isGRPCServerIdent(name string) bool {
	switch {
	case name == "Server":
		return true
	case strings.HasPrefix(name, "Register") && strings.HasSuffix(name, "Server"):
		return true
	default:
		return false
	}
}

func receiverTypeName(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	expr := fields.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

func structHasJSONTag(expr ast.Expr) bool {
	st, ok := expr.(*ast.StructType)
	if !ok || st.Fields == nil {
		return false
	}
	for _, field := range st.Fields.List {
		if field.Tag == nil {
			continue
		}
		tag := field.Tag.Value
		if strings.Contains(tag, `json:"`) {
			return true
		}
	}
	return false
}

func hasGoEmbed(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, c := range doc.List {
		text := strings.TrimSpace(c.Text)
		if strings.HasPrefix(text, "//go:embed") || strings.HasPrefix(text, "/*go:embed") {
			return true
		}
	}
	return false
}

func goEmbedLooksStatic(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, c := range doc.List {
		text := strings.ToLower(strings.TrimSpace(c.Text))
		if !strings.HasPrefix(text, "//go:embed") && !strings.HasPrefix(text, "/*go:embed") {
			continue
		}
		if strings.Contains(text, ".html") || strings.Contains(text, ".js") ||
			strings.Contains(text, ".css") || strings.Contains(text, "static") ||
			strings.Contains(text, "web") || strings.Contains(text, "ui/") {
			return true
		}
	}
	return false
}

func compactPackageDoc(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	if i := strings.Index(text, ". "); i >= 0 {
		text = text[:i+1]
	}
	const maxRunes = 240
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	runes := []rune(text)
	cut := runes[:maxRunes]
	for len(cut) > 0 && !unicode.IsSpace(cut[len(cut)-1]) {
		cut = cut[:len(cut)-1]
	}
	return strings.TrimSpace(string(cut)) + "…"
}

func normalizePath(path string) string {
	return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(path), "./"))
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

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

// SymbolBody is one AST-extracted source fragment for RLM progressive context.
type SymbolBody struct {
	Name   string `json:"name" yaml:"name"`
	Kind   string `json:"kind" yaml:"kind"` // type, func, method, value, error
	Source string `json:"source" yaml:"source"`
}

// PackageEvidence summarizes static source evidence for one package.
type PackageEvidence struct {
	Path                string       `json:"path"`
	Name                string       `json:"name"`
	Language            string       `json:"language,omitempty"` // go|python
	Files               []string     `json:"files,omitempty"`
	PackageDoc          string       `json:"packageDoc,omitempty"`
	ExportedDecls       []string     `json:"exportedDecls,omitempty"`
	ExportedFuncs       []string     `json:"exportedFuncs,omitempty"`
	ExportedMethods     []string     `json:"exportedMethods,omitempty"`
	UnexportedDecls     []string     `json:"unexportedDecls,omitempty"`
	UnexportedFuncs     []string     `json:"unexportedFuncs,omitempty"`
	UnexportedMethods   []string     `json:"unexportedMethods,omitempty"`
	ErrorTypes          []string     `json:"errorTypes,omitempty"`
	ExportedBodies      []SymbolBody `json:"exportedBodies,omitempty"`
	PrivateOneHopBodies []SymbolBody `json:"privateOneHopBodies,omitempty"`
	HasMain             bool         `json:"hasMain,omitempty"`
	JSONTags            bool         `json:"jsonTags,omitempty"` // Go tags or Python typed fields
	GoEmbed             bool         `json:"goEmbed,omitempty"`
	EmbedsStatic        bool         `json:"embedsStatic,omitempty"`
	ImportsNetHTTP      bool         `json:"importsNetHTTP,omitempty"`
	ImportsOsExec       bool         `json:"importsOsExec,omitempty"`
	ImportsGRPC         bool         `json:"importsGrpc,omitempty"`
	ImportsOTel         bool         `json:"importsOtel,omitempty"`
	ImportsPrometheus   bool         `json:"importsPrometheus,omitempty"`
	HTTPSurfaceIdent    bool         `json:"httpSurfaceIdent,omitempty"`
	// Mechanical HTTP serving signals (production .go only; never from _test.go).
	HTTPMuxParam          bool `json:"httpMuxParam,omitempty"`          // exported func takes *http.ServeMux
	HTTPRouteRegister     bool `json:"httpRouteRegister,omitempty"`     // Handle / HandleFunc registration
	HTTPListenServe       bool `json:"httpListenServe,omitempty"`       // ListenAndServe / Serve*
	HTTPHandlerSignature  bool `json:"httpHandlerSignature,omitempty"`  // exported (ResponseWriter, *Request)
	HTTPHandlerResult     bool `json:"httpHandlerResult,omitempty"`     // returns http.Handler / HandlerFunc
	HTTPFileServer        bool `json:"httpFileServer,omitempty"`        // http.FileServer
	HTTPResponseHelper    bool `json:"httpResponseHelper,omitempty"`    // http.Error / Status* corroboration
	ImportsHTTPFramework  bool `json:"importsHttpFramework,omitempty"`  // gin/echo/chi/mux/fiber/…
	HTTPFrameworkRoute    bool `json:"httpFrameworkRoute,omitempty"`    // framework route registration call
	GRPCServerIdent       bool `json:"grpcServerIdent,omitempty"`
	// Worker / queue / CLI / ingest (language-neutral; finders fill these).
	JobHandlerImpl     bool `json:"jobHandlerImpl,omitempty"`     // Kind+Validate+Run on one concrete type
	JobRegisterExport  bool `json:"jobRegisterExport,omitempty"`  // exported Register
	JobRegisterCall    bool `json:"jobRegisterCall,omitempty"`    // Register(...) / registerHandler(...)
	JobEnqueueExport   bool `json:"jobEnqueueExport,omitempty"`   // Enqueue / Submit / Delay / apply_async
	JobRunLoop         bool `json:"jobRunLoop,omitempty"`         // Run / RunLoop with context
	JobTaskDecorator   bool `json:"jobTaskDecorator,omitempty"`   // @task / @app.task (Python)
	ImportsJobFramework bool `json:"importsJobFramework,omitempty"` // celery / rq / arq
	CLIDispatchExport  bool `json:"cliDispatchExport,omitempty"`  // RunCLI / Execute / Run(args, writers)
	CLIFlagParse       bool `json:"cliFlagParse,omitempty"`       // flag.Parse / cobra / argparse
	CLISubcommand      bool `json:"cliSubcommand,omitempty"`      // string-case subcommand switch
	ImportsCLIFramework bool `json:"importsCliFramework,omitempty"` // click / typer / argparse
	IngestSyncExport   bool `json:"ingestSyncExport,omitempty"`   // Sync(ctx, ...)
	IngestIndexOps     bool `json:"ingestIndexOps,omitempty"`     // upsert / delete / chunk index ops
	IngestWatch        bool `json:"ingestWatch,omitempty"`        // Watch / poll loop
	// Adapter / pipeline / view (language-neutral).
	ClientConstructor      bool `json:"clientConstructor,omitempty"`      // New returns *Client
	ImportsExternalDriver  bool `json:"importsExternalDriver,omitempty"`  // neo4j / meili / redis / …
	PipelineRegistryParam  bool `json:"pipelineRegistryParam,omitempty"`  // *ModuleRegistry param
	PipelineRegisterExport bool `json:"pipelineRegisterExport,omitempty"` // Register* modules
	PipelineModuleMap      bool `json:"pipelineModuleMap,omitempty"`      // map[string]Module result
	PipelineRunnerType     bool `json:"pipelineRunnerType,omitempty"`     // JobRunner / NewJobRunner
	ViewBuildExport        bool `json:"viewBuildExport,omitempty"`        // Build/Render -> local Page
	DeliveryHint           string `json:"deliveryHint,omitempty"`
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
	Incomplete      bool     `json:"Incomplete"`
	Error           *struct {
		Err string `json:"Err"`
	} `json:"Error"`
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
	// -e keeps harvest going when GOWORK=off surfaces missing go.sum noise;
	// incomplete packages are skipped below.
	cmd := exec.Command("go", "list", "-e", "-json", "./...")
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
		if strings.TrimSpace(pkg.Dir) == "" || pkg.Error != nil {
			continue
		}
		pkgs = append(pkgs, pkg)
	}
	if len(pkgs) == 0 {
		return nil, terrors.New(terrors.CodeFailedPrecondition, "sourceindex.listPackages",
			"go list returned no usable packages").With("dir", moduleRoot)
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
		Path:     normalizePath(pkg.Dir),
		Name:     pkg.Name,
		Language: LangGo,
		Files:    make([]string, 0, len(files)),
	}
	fset := token.NewFileSet()
	exportedDecls := map[string]struct{}{}
	exportedFuncs := map[string]struct{}{}
	exportedMethods := map[string]struct{}{}
	unexportedDecls := map[string]struct{}{}
	unexportedFuncs := map[string]struct{}{}
	unexportedMethods := map[string]struct{}{}
	errorTypes := map[string]struct{}{}
	privateBodies := map[string]SymbolBody{} // keyed by symbol name/method key
	var exportedBodies []SymbolBody
	httpSurfaceIdent := false
	grpcServerIdent := false
	handlerMethods := map[string]map[string]bool{} // receiver -> method names
	for _, file := range files {
		// go list GoFiles omits tests; skip explicitly so serving signals stay production-only.
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		abs := filepath.Join(absDir, file)
		rel, err := filepath.Rel(repoRoot, abs)
		if err != nil {
			return PackageEvidence{}, terrors.Wrap(err, terrors.CodeInternal, "sourceindex.scanPackage", "rel file").
				With("file", abs)
		}
		ev.Files = append(ev.Files, normalizePath(rel))
		src, err := os.ReadFile(abs)
		if err != nil {
			return PackageEvidence{}, terrors.Wrap(err, terrors.CodeUnavailable, "sourceindex.scanPackage", "read file").
				With("file", abs)
		}
		parsed, err := parser.ParseFile(fset, abs, src, parser.ParseComments)
		if err != nil {
			return PackageEvidence{}, terrors.Wrap(err, terrors.CodeInvalid, "sourceindex.scanPackage", "parse file").
				With("file", abs)
		}
		if ev.PackageDoc == "" && parsed.Doc != nil {
			ev.PackageDoc = compactPackageDoc(parsed.Doc.Text())
		}
		httpAlias := ""
		flagAlias := ""
		frameworkAliases := map[string]string{} // local name -> import path
		for _, imp := range parsed.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			alias := importAlias(imp)
			switch path {
			case "net/http":
				ev.ImportsNetHTTP = true
				httpAlias = alias
			case "flag":
				flagAlias = alias
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
			if isHTTPFrameworkImport(path) {
				ev.ImportsHTTPFramework = true
				frameworkAliases[alias] = path
			}
			if isCLIFrameworkImport(path) {
				ev.ImportsCLIFramework = true
			}
			if isJobFrameworkImport(path) {
				ev.ImportsJobFramework = true
			}
			if isExternalDriverImport(path) {
				ev.ImportsExternalDriver = true
			}
		}
		for _, decl := range parsed.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.Name == "main" && pkg.Name == "main" && d.Recv == nil {
					ev.HasMain = true
				}
				body := nodeSource(fset, src, d)
				exported := ast.IsExported(d.Name.Name)
				if d.Recv != nil {
					recvType := receiverTypeName(d.Recv)
					if recvType != "" {
						key := recvType + "." + d.Name.Name
						if exported {
							exportedMethods[key] = struct{}{}
							exportedBodies = append(exportedBodies, SymbolBody{Name: key, Kind: "method", Source: truncateBody(body)})
							if isHTTPSurfaceIdent(d.Name.Name) {
								httpSurfaceIdent = true
							}
							if isGRPCServerIdent(d.Name.Name) {
								grpcServerIdent = true
							}
							collectHTTPFuncSurface(d.Type, httpAlias, &ev)
							noteHandlerMethod(handlerMethods, recvType, d.Name.Name)
						} else {
							unexportedMethods[key] = struct{}{}
							privateBodies[key] = SymbolBody{Name: key, Kind: "method", Source: truncateBody(body)}
							noteHandlerMethod(handlerMethods, recvType, d.Name.Name)
						}
					}
				} else if exported {
					exportedFuncs[d.Name.Name] = struct{}{}
					exportedBodies = append(exportedBodies, SymbolBody{Name: d.Name.Name, Kind: "func", Source: truncateBody(body)})
					if isHTTPSurfaceIdent(d.Name.Name) {
						httpSurfaceIdent = true
					}
					if isGRPCServerIdent(d.Name.Name) {
						grpcServerIdent = true
					}
					collectHTTPFuncSurface(d.Type, httpAlias, &ev)
					collectRoleFuncExport(d, &ev)
				} else {
					unexportedFuncs[d.Name.Name] = struct{}{}
					privateBodies[d.Name.Name] = SymbolBody{Name: d.Name.Name, Kind: "func", Source: truncateBody(body)}
					// Unexported Register helpers still count as register surface when called.
					if d.Name.Name == "registerHandler" || d.Name.Name == "Register" {
						ev.JobRegisterExport = true
					}
				}
				if d.Body != nil {
					inspectHTTPServingCalls(d.Body, httpAlias, frameworkAliases, &ev)
					inspectRoleCalls(d.Body, flagAlias, &ev)
					if countStringSwitchCases(d.Body) >= 3 {
						ev.CLISubcommand = true
					}
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
						body := typeSpecSource(fset, src, d, s)
						if ast.IsExported(s.Name.Name) {
							exportedDecls[s.Name.Name] = struct{}{}
							kind := "type"
							if looksLikeErrorType(s.Name.Name, s.Type) {
								errorTypes[s.Name.Name] = struct{}{}
								kind = "error"
							}
							exportedBodies = append(exportedBodies, SymbolBody{Name: s.Name.Name, Kind: kind, Source: truncateBody(body)})
							if isHTTPSurfaceIdent(s.Name.Name) {
								httpSurfaceIdent = true
							}
							if isGRPCServerIdent(s.Name.Name) {
								grpcServerIdent = true
							}
						} else {
							unexportedDecls[s.Name.Name] = struct{}{}
							kind := "type"
							if looksLikeErrorType(s.Name.Name, s.Type) {
								errorTypes[s.Name.Name] = struct{}{}
								kind = "error"
							}
							privateBodies[s.Name.Name] = SymbolBody{Name: s.Name.Name, Kind: kind, Source: truncateBody(body)}
						}
						if structHasJSONTag(s.Type) {
							ev.JSONTags = true
						}
						if s.Name.Name == "JobRunner" {
							ev.PipelineRunnerType = true
						}
					case *ast.ValueSpec:
						if hasGoEmbed(s.Doc) || hasGoEmbed(d.Doc) {
							ev.GoEmbed = true
							if goEmbedLooksStatic(s.Doc) || goEmbedLooksStatic(d.Doc) {
								ev.EmbedsStatic = true
							}
						}
						for _, name := range s.Names {
							if strings.HasPrefix(name.Name, "Err") && ast.IsExported(name.Name) {
								errorTypes[name.Name] = struct{}{}
							}
							if ast.IsExported(name.Name) {
								exportedDecls[name.Name] = struct{}{}
								if isHTTPSurfaceIdent(name.Name) {
									httpSurfaceIdent = true
								}
								if isGRPCServerIdent(name.Name) {
									grpcServerIdent = true
								}
							} else {
								unexportedDecls[name.Name] = struct{}{}
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
	ev.UnexportedDecls = sortedKeys(unexportedDecls)
	ev.UnexportedFuncs = sortedKeys(unexportedFuncs)
	ev.UnexportedMethods = sortedKeys(unexportedMethods)
	ev.ErrorTypes = sortedKeys(errorTypes)
	ev.ExportedBodies = exportedBodies
	ev.PrivateOneHopBodies = oneHopPrivateBodies(exportedBodies, privateBodies)
	ev.HTTPSurfaceIdent = httpSurfaceIdent
	ev.GRPCServerIdent = grpcServerIdent
	if hasJobHandlerTrio(handlerMethods) {
		ev.JobHandlerImpl = true
	}
	ev.DeliveryHint = deliveryHint(ev, httpSurfaceIdent)
	return ev, nil
}

const maxSymbolBodyBytes = 4000

func truncateBody(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxSymbolBodyBytes {
		return s
	}
	return s[:maxSymbolBodyBytes] + "\n// ... truncated"
}

func nodeSource(fset *token.FileSet, src []byte, node ast.Node) string {
	if node == nil {
		return ""
	}
	start := fset.Position(node.Pos()).Offset
	end := fset.Position(node.End()).Offset
	if start < 0 || end > len(src) || start >= end {
		return ""
	}
	return string(src[start:end])
}

func typeSpecSource(fset *token.FileSet, src []byte, decl *ast.GenDecl, spec *ast.TypeSpec) string {
	if decl != nil && len(decl.Specs) == 1 {
		return nodeSource(fset, src, decl)
	}
	return nodeSource(fset, src, spec)
}

func looksLikeErrorType(name string, typ ast.Expr) bool {
	if strings.HasPrefix(name, "Err") || strings.HasSuffix(name, "Error") {
		return true
	}
	ident, ok := typ.(*ast.Ident)
	return ok && ident.Name == "error"
}

func oneHopPrivateBodies(exported []SymbolBody, private map[string]SymbolBody) []SymbolBody {
	needed := map[string]struct{}{}
	for _, b := range exported {
		for name := range private {
			base := name
			if i := strings.LastIndex(name, "."); i >= 0 {
				base = name[i+1:]
			}
			if strings.Contains(b.Source, base) {
				needed[name] = struct{}{}
			}
		}
	}
	out := make([]SymbolBody, 0, len(needed))
	for name := range needed {
		out = append(out, private[name])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func deliveryHint(ev PackageEvidence, httpSurfaceIdent bool) string {
	if ev.HasMain {
		return DeliveryCLI
	}
	if ev.GoEmbed && ev.EmbedsStatic {
		return DeliveryServerUI
	}
	if ev.GoEmbed || looksLikeHTTPServer(ev, httpSurfaceIdent) {
		return DeliveryServerHTTP
	}
	if ev.ImportsGRPC && ev.GRPCServerIdent {
		return DeliveryServerGRPC
	}
	if looksLikeShapePackage(ev) {
		return DeliveryDTO
	}
	return ""
}

// looksLikeHTTPServer reports AST-verified HTTP serving (stdlib or router frameworks).
// Folder/path names are never evidence. Test files must not contribute signals.
func looksLikeHTTPServer(ev PackageEvidence, httpSurfaceIdent bool) bool {
	if ev.ImportsHTTPFramework && ev.HTTPFrameworkRoute {
		return true
	}
	if !ev.ImportsNetHTTP {
		return false
	}
	if httpSurfaceIdent || ev.HTTPSurfaceIdent {
		return true
	}
	if ev.HTTPMuxParam || ev.HTTPRouteRegister || ev.HTTPListenServe {
		return true
	}
	if ev.HTTPHandlerResult || ev.HTTPFileServer {
		return true
	}
	// Tier 2: exported handler shape plus response-helper corroboration.
	if ev.HTTPHandlerSignature && ev.HTTPResponseHelper {
		return true
	}
	return false
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

func importAlias(imp *ast.ImportSpec) string {
	if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" && imp.Name.Name != "." {
		return imp.Name.Name
	}
	path := strings.Trim(imp.Path.Value, `"`)
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func isHTTPFrameworkImport(path string) bool {
	switch path {
	case "github.com/gin-gonic/gin",
		"github.com/labstack/echo",
		"github.com/labstack/echo/v4",
		"github.com/go-chi/chi",
		"github.com/go-chi/chi/v5",
		"github.com/gorilla/mux",
		"github.com/gofiber/fiber",
		"github.com/gofiber/fiber/v2",
		"github.com/julienschmidt/httprouter",
		"github.com/grpc-ecosystem/grpc-gateway/runtime",
		"github.com/grpc-ecosystem/grpc-gateway/v2/runtime":
		return true
	default:
		return false
	}
}

func collectHTTPFuncSurface(ft *ast.FuncType, httpAlias string, ev *PackageEvidence) {
	if ft == nil || httpAlias == "" {
		return
	}
	if funcHasHTTPMuxParam(ft, httpAlias) {
		ev.HTTPMuxParam = true
	}
	if funcHasHTTPHandlerSignature(ft, httpAlias) {
		ev.HTTPHandlerSignature = true
	}
	if funcReturnsHTTPHandler(ft, httpAlias) {
		ev.HTTPHandlerResult = true
	}
}

func funcHasHTTPMuxParam(ft *ast.FuncType, httpAlias string) bool {
	if ft == nil || ft.Params == nil {
		return false
	}
	for _, field := range ft.Params.List {
		if isHTTPNamedType(field.Type, httpAlias, "ServeMux") {
			return true
		}
	}
	return false
}

func funcHasHTTPHandlerSignature(ft *ast.FuncType, httpAlias string) bool {
	if ft == nil || ft.Params == nil || len(ft.Params.List) < 2 {
		return false
	}
	hasWriter, hasRequest := false, false
	for _, field := range ft.Params.List {
		if isHTTPNamedType(field.Type, httpAlias, "ResponseWriter") {
			hasWriter = true
		}
		if isHTTPNamedType(field.Type, httpAlias, "Request") {
			hasRequest = true
		}
	}
	return hasWriter && hasRequest
}

func funcReturnsHTTPHandler(ft *ast.FuncType, httpAlias string) bool {
	if ft == nil || ft.Results == nil {
		return false
	}
	for _, field := range ft.Results.List {
		if isHTTPNamedType(field.Type, httpAlias, "Handler") ||
			isHTTPNamedType(field.Type, httpAlias, "HandlerFunc") {
			return true
		}
	}
	return false
}

func isHTTPNamedType(expr ast.Expr, httpAlias, typeName string) bool {
	expr = unwrapExpr(expr)
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != httpAlias {
		return false
	}
	return sel.Sel != nil && sel.Sel.Name == typeName
}

func unwrapExpr(expr ast.Expr) ast.Expr {
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.ParenExpr:
			expr = e.X
		default:
			return expr
		}
	}
}

func inspectHTTPServingCalls(body *ast.BlockStmt, httpAlias string, frameworkAliases map[string]string, ev *PackageEvidence) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil {
			return true
		}
		name := sel.Sel.Name
		if pkg, ok := sel.X.(*ast.Ident); ok {
			if httpAlias != "" && pkg.Name == httpAlias {
				switch name {
				case "Handle", "HandleFunc":
					ev.HTTPRouteRegister = true
				case "ListenAndServe", "ListenAndServeTLS", "Serve", "ServeTLS":
					ev.HTTPListenServe = true
				case "FileServer":
					ev.HTTPFileServer = true
				case "Error":
					ev.HTTPResponseHelper = true
				}
			}
			if _, ok := frameworkAliases[pkg.Name]; ok {
				if isHTTPFrameworkRouteName(name) {
					ev.HTTPFrameworkRoute = true
				}
			}
		}
		// Method calls on locals (mux.HandleFunc, r.GET) when the package imports a framework or net/http.
		switch name {
		case "Handle", "HandleFunc":
			if httpAlias != "" {
				ev.HTTPRouteRegister = true
			}
			if len(frameworkAliases) > 0 {
				ev.HTTPFrameworkRoute = true
			}
		case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "CONNECT", "TRACE",
			"Any", "Use", "Group", "Route", "Static", "StaticFS", "RegisterHandler", "RegisterService":
			if len(frameworkAliases) > 0 {
				ev.HTTPFrameworkRoute = true
			}
		}
		return true
	})
}

func isHTTPFrameworkRouteName(name string) bool {
	switch name {
	case "Handle", "HandleFunc", "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS",
		"CONNECT", "TRACE", "Any", "Use", "Group", "Route", "Static", "StaticFS",
		"RegisterHandler", "RegisterService":
		return true
	default:
		return false
	}
}

func isCLIFrameworkImport(path string) bool {
	switch path {
	case "github.com/spf13/cobra",
		"github.com/urfave/cli",
		"github.com/urfave/cli/v2",
		"github.com/alecthomas/kong":
		return true
	default:
		return false
	}
}

func isJobFrameworkImport(path string) bool {
	switch path {
	case "github.com/hibiken/asynq",
		"github.com/riverqueue/river",
		"github.com/gocraft/work":
		return true
	default:
		return strings.Contains(path, "/celery") || strings.HasSuffix(path, "/rq")
	}
}

func isExternalDriverImport(path string) bool {
	switch {
	case strings.Contains(path, "neo4j-go-driver"):
		return true
	case strings.Contains(path, "meilisearch"):
		return true
	case path == "github.com/redis/go-redis" || strings.HasPrefix(path, "github.com/redis/go-redis/"):
		return true
	case path == "github.com/go-redis/redis" || strings.HasPrefix(path, "github.com/go-redis/redis/"):
		return true
	case path == "go.mongodb.org/mongo-driver" || strings.HasPrefix(path, "go.mongodb.org/mongo-driver/"):
		return true
	case path == "github.com/jackc/pgx" || strings.HasPrefix(path, "github.com/jackc/pgx/"):
		return true
	case path == "github.com/lib/pq":
		return true
	case path == "database/sql":
		return true
	case path == "google.golang.org/grpc":
		return true
	default:
		return false
	}
}

func noteHandlerMethod(byRecv map[string]map[string]bool, recv, method string) {
	switch method {
	case "Kind", "Validate", "Run":
	default:
		return
	}
	if byRecv[recv] == nil {
		byRecv[recv] = map[string]bool{}
	}
	byRecv[recv][method] = true
}

func hasJobHandlerTrio(byRecv map[string]map[string]bool) bool {
	for _, methods := range byRecv {
		if methods["Kind"] && methods["Validate"] && methods["Run"] {
			return true
		}
	}
	return false
}

func collectRoleFuncExport(d *ast.FuncDecl, ev *PackageEvidence) {
	if d == nil || d.Name == nil {
		return
	}
	name := d.Name.Name
	switch name {
	case "Enqueue", "Submit", "Delay":
		ev.JobEnqueueExport = true
	case "Register":
		ev.JobRegisterExport = true
	case "RunCLI", "Execute":
		ev.CLIDispatchExport = true
	case "Sync":
		if funcHasContextParam(d.Type) {
			ev.IngestSyncExport = true
		}
	case "Watch", "WatchPII":
		ev.IngestWatch = true
	case "Run", "RunLoop":
		if d.Recv == nil && funcHasContextParam(d.Type) {
			ev.JobRunLoop = true
		}
		if d.Recv == nil && looksLikeCLIDispatchSig(d.Type) {
			ev.CLIDispatchExport = true
		}
	case "NewJobRunner":
		ev.PipelineRunnerType = true
	}
	if name == "Run" && looksLikeCLIDispatchSig(d.Type) {
		ev.CLIDispatchExport = true
	}
	if name == "New" || strings.HasPrefix(name, "New") {
		if returnsClientType(d.Type) {
			ev.ClientConstructor = true
		}
	}
	if strings.HasPrefix(name, "Register") {
		if funcHasRegistryParam(d.Type) {
			ev.PipelineRegisterExport = true
			ev.PipelineRegistryParam = true
		}
	}
	if funcHasRegistryParam(d.Type) {
		ev.PipelineRegistryParam = true
	}
	if returnsModuleMap(d.Type) {
		ev.PipelineModuleMap = true
	}
	if name == "Build" || name == "Render" {
		if returnsLocalPageType(d.Type) {
			ev.ViewBuildExport = true
		}
	}
}

func returnsClientType(ft *ast.FuncType) bool {
	if ft == nil || ft.Results == nil {
		return false
	}
	for _, field := range ft.Results.List {
		name := typeBaseName(field.Type)
		if name == "Client" || strings.HasSuffix(name, "Client") {
			return true
		}
	}
	return false
}

func funcHasRegistryParam(ft *ast.FuncType) bool {
	if ft == nil || ft.Params == nil {
		return false
	}
	for _, field := range ft.Params.List {
		name := typeBaseName(field.Type)
		if name == "ModuleRegistry" || name == "Registry" {
			return true
		}
		if isNamedSelector(field.Type, "registry", "ModuleRegistry") {
			return true
		}
	}
	return false
}

func returnsModuleMap(ft *ast.FuncType) bool {
	if ft == nil || ft.Results == nil {
		return false
	}
	for _, field := range ft.Results.List {
		m, ok := unwrapExpr(field.Type).(*ast.MapType)
		if !ok {
			continue
		}
		key, ok := m.Key.(*ast.Ident)
		if !ok || key.Name != "string" {
			continue
		}
		valName := typeBaseName(m.Value)
		if valName == "Module" || strings.HasSuffix(valName, "Module") {
			return true
		}
	}
	return false
}

func returnsLocalPageType(ft *ast.FuncType) bool {
	if ft == nil || ft.Results == nil || len(ft.Results.List) == 0 {
		return false
	}
	// First result should be a same-package named type (Page, Document, View, Model).
	name := typeBaseName(ft.Results.List[0].Type)
	switch name {
	case "Page", "Document", "View", "Model", "Screen", "Panel":
		return true
	default:
		return false
	}
}

func typeBaseName(expr ast.Expr) string {
	expr = unwrapExpr(expr)
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if t.Sel != nil {
			return t.Sel.Name
		}
	}
	return ""
}

func funcHasContextParam(ft *ast.FuncType) bool {
	if ft == nil || ft.Params == nil {
		return false
	}
	for _, field := range ft.Params.List {
		if isNamedSelector(field.Type, "context", "Context") {
			return true
		}
	}
	return false
}

func looksLikeCLIDispatchSig(ft *ast.FuncType) bool {
	if ft == nil || ft.Params == nil || len(ft.Params.List) < 2 {
		return false
	}
	hasArgs, hasWriter := false, false
	for _, field := range ft.Params.List {
		if isStringSlice(field.Type) {
			hasArgs = true
		}
		if isNamedSelector(field.Type, "io", "Writer") ||
			isNamedSelector(field.Type, "io", "Reader") {
			hasWriter = true
		}
	}
	return hasArgs && hasWriter
}

func isStringSlice(expr ast.Expr) bool {
	arr, ok := unwrapExpr(expr).(*ast.ArrayType)
	if !ok || arr.Len != nil {
		return false
	}
	ident, ok := arr.Elt.(*ast.Ident)
	return ok && ident.Name == "string"
}

func isNamedSelector(expr ast.Expr, pkg, name string) bool {
	expr = unwrapExpr(expr)
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

func inspectRoleCalls(body *ast.BlockStmt, flagAlias string, ev *PackageEvidence) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			switch fun.Name {
			case "Register", "registerHandler":
				ev.JobRegisterCall = true
			case "chunkFiles":
				ev.IngestIndexOps = true
			}
		case *ast.SelectorExpr:
			if fun.Sel == nil {
				return true
			}
			name := fun.Sel.Name
			if pkg, ok := fun.X.(*ast.Ident); ok {
				if flagAlias != "" && pkg.Name == flagAlias && name == "Parse" {
					ev.CLIFlagParse = true
				}
			}
			switch name {
			case "Register", "registerHandler":
				ev.JobRegisterCall = true
			case "UpsertChunks", "DeleteAllDocuments", "DeleteDocuments", "DeleteDocumentsByDocIDs",
				"AddDocuments", "IndexDocuments", "chunkFiles":
				ev.IngestIndexOps = true
			case "Parse":
				// cobra/flag style without tracked alias
				if id, ok := fun.X.(*ast.Ident); ok && (id.Name == "flag" || id.Name == "flags") {
					ev.CLIFlagParse = true
				}
			case "AddCommand", "Execute":
				ev.CLIFlagParse = true
				ev.CLISubcommand = true
			case "delay", "apply_async", "Enqueue", "Submit":
				ev.JobEnqueueExport = true
			}
		}
		return true
	})
}

func countStringSwitchCases(body *ast.BlockStmt) int {
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		for _, stmt := range sw.Body.List {
			cc, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			for _, expr := range cc.List {
				if _, ok := expr.(*ast.BasicLit); ok {
					count++
				}
			}
		}
		return true
	})
	return count
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

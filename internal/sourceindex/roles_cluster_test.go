package sourceindex_test

import (
	"path/filepath"
	"testing"

	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestUnknownClusterRoles_steps(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		files    map[string]string
		wantRole string
	}{
		{
			name: "driver_client_adapter",
			files: map[string]string{
				"neo/client.go": `package neo

import "github.com/neo4j/neo4j-go-driver/v5/neo4j"

type Client struct {
	driver neo4j.DriverWithContext
}

func New() (*Client, error) {
	return &Client{}, nil
}
`,
				"go.mod": `module example.com/clusters

go 1.26.5

require github.com/neo4j/neo4j-go-driver/v5 v5.28.0
`,
			},
			wantRole: sourceindex.RoleAdapter,
		},
		{
			name: "http_client_still_adapter",
			files: map[string]string{
				"httpcli/client.go": `package httpcli

import "net/http"

type Client struct {
	c *http.Client
}

func NewClient() *Client {
	return &Client{c: http.DefaultClient}
}
`,
			},
			wantRole: sourceindex.RoleAdapter,
		},
		{
			name: "server_beats_client_field",
			files: map[string]string{
				"api/api.go": `package api

import "net/http"

type Client struct{}

func New() *Client { return &Client{} }

func Mount(mux *http.ServeMux) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
}
`,
			},
			wantRole: sourceindex.RoleHTTPSurface,
		},
		{
			name: "plain_new_without_driver_unknown",
			files: map[string]string{
				"plain/plain.go": `package plain

type Client struct{ N int }

func New() *Client { return &Client{} }
`,
			},
			wantRole: sourceindex.RoleUnknown,
		},
		{
			name: "pipeline_register_modules",
			files: map[string]string{
				"clients/modules.go": `package clients

import (
	"context"

	"example.com/clusters/registry"
	"example.com/clusters/core"
)

func RegisterChronology(ctx context.Context, reg *registry.ModuleRegistry) error {
	return nil
}

func NewChronologyModules() map[string]core.Module {
	return map[string]core.Module{}
}
`,
				"registry/reg.go": `package registry

type ModuleRegistry struct{}
`,
				"core/mod.go": `package core

type Module interface{}
`,
			},
			wantRole: sourceindex.RolePipeline,
			// pick clients via primaryNonStub - may pick wrong package; use path filter in runner
		},
		{
			name: "pipeline_job_runner",
			files: map[string]string{
				"runner/runner.go": `package runner

import "example.com/clusters/registry"

type JobRunner struct {
	reg *registry.ModuleRegistry
}

func NewJobRunner(reg *registry.ModuleRegistry) *JobRunner {
	return &JobRunner{reg: reg}
}
`,
				"registry/reg.go": `package registry

type ModuleRegistry struct{}
`,
			},
			wantRole: sourceindex.RolePipeline,
		},
		{
			name: "view_builder",
			files: map[string]string{
				"chronview/model.go": `package chronview

type Page struct {
	Title string ` + "`json:\"title\"`" + `
}

type Payload struct {
	Rows []string ` + "`json:\"rows\"`" + `
}
`,
				"chronview/build.go": `package chronview

func Build(payload Payload, base string) Page {
	return Page{Title: base}
}
`,
			},
			wantRole: sourceindex.RoleView,
		},
		{
			name: "json_only_still_dto",
			files: map[string]string{
				"shapes/shapes.go": `package shapes

type Row struct {
	ID string ` + "`json:\"id\"`" + `
}
`,
			},
			wantRole: sourceindex.RoleDTO,
		},
		{
			name: "locator_root_finders",
			files: map[string]string{
				"reporoot/reporoot.go": `package reporoot

import (
	"os"
	"path/filepath"
)

func Find(start string) (string, error) {
	return filepath.Abs(start)
}

func Resolve(start string) (string, error) {
	return os.Getwd()
}

func Split(repo string) (instance, product string) {
	return repo, filepath.Dir(repo)
}
`,
			},
			wantRole: sourceindex.RoleLocator,
		},
		{
			name: "locator_path_matchers",
			files: map[string]string{
				"pathmatch/pathmatch.go": `package pathmatch

import "strings"

func Excluded(rel string, excludes []string) bool {
	for _, ex := range excludes {
		if rel == ex {
			return true
		}
	}
	return false
}

func PrefixMatch(rel string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return false
}
`,
			},
			wantRole: sourceindex.RoleLocator,
		},
		{
			name: "locator_root_matchers",
			files: map[string]string{
				"rootpaths/paths.go": `package rootpaths

import "path/filepath"

func Root(repo string) string {
	return filepath.Join(repo, "data")
}

func ContentRoot(repo string) string {
	return filepath.Join(Root(repo), "content")
}
`,
			},
			wantRole: sourceindex.RoleLocator,
		},
		{
			name: "locator_single_signal_unknown",
			files: map[string]string{
				"finder/finder.go": `package finder

func Find(start string) (string, error) { return start, nil }
`,
			},
			wantRole: sourceindex.RoleUnknown,
		},
		{
			name: "crypto_encrypt_decrypt",
			files: map[string]string{
				"age/age.go": `package age

func KeyPath() string { return "" }

func Encrypt(src, dst, recipient string) error { return nil }

func Decrypt(enc, dst, keyPath string) error { return nil }
`,
			},
			wantRole: sourceindex.RoleCrypto,
		},
		{
			name: "crypto_encrypt_only_unknown",
			files: map[string]string{
				"enc/enc.go": `package enc

func Encrypt(src, dst string) error { return nil }
`,
			},
			wantRole: sourceindex.RoleUnknown,
		},
		{
			name: "prompt_instruction_builders",
			files: map[string]string{
				"prompts/prompts.go": `package prompts

func ChronologyBuildWhatInstruction() string { return "" }

func ChronologyBuildWhyInstruction() string { return "" }

func TimelineListTitleInstruction() string { return "" }
`,
			},
			wantRole: sourceindex.RolePrompt,
		},
		{
			name: "prompt_single_instruction_unknown",
			files: map[string]string{
				"ask/ask.go": `package ask

func AskInstruction() string { return "" }

func Summarize(text string) string { return text }
`,
			},
			wantRole: sourceindex.RoleUnknown,
		},
		{
			name: "container_runtime_hub",
			files: map[string]string{
				"svc1/svc1.go": `package svc1
type Service1 struct{}
`,
				"svc2/svc2.go": `package svc2
type Service2 struct{}
`,
				"container/runtime.go": `package container

import (
	"context"
	"example.com/clusters/svc1"
	"example.com/clusters/svc2"
)

type Runtime struct {
	S1 *svc1.Service1
	S2 *svc2.Service2
}

func Init(ctx context.Context, root string) (*Runtime, error) {
	return &Runtime{S1: &svc1.Service1{}, S2: &svc2.Service2{}}, nil
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	return nil
}
`,
			},
			wantRole: sourceindex.RoleContainer,
		},
		{
			name: "validate_results_and_issues",
			files: map[string]string{
				"cast/validate.go": `package cast

import "strings"

type Issue struct {
	Message string
}

type Result struct {
	Issues []Issue
}

func Validate(id string) (*Result, error) {
	_ = strings.TrimSpace(id)
	return &Result{}, nil
}
`,
			},
			wantRole: sourceindex.RoleValidation,
		},
		{
			name: "validate_report_shape",
			files: map[string]string{
				"story/validate.go": `package story

import "os"

type Issue struct {
	Message string
}

// ValidationResult holds validation output.
type ValidationResult struct {
	Issues []Issue
}

func Validate(root string) (*ValidationResult, error) {
	_, _ = os.Stat(root)
	return &ValidationResult{}, nil
}
`,
			},
			wantRole: sourceindex.RoleValidation,
		},
		{
			name: "validate_product_wrapper_still_unknown",
			files: map[string]string{
				"adapter/validate.go": `package adapter

import "example.com/clusters/catalog"

func ValidateProduct(root string) []catalog.Issue {
	return nil
}
`,
			},
			wantRole: sourceindex.RoleUnknown,
		},
		{
			name: "export_writes_files",
			files: map[string]string{
				"siteout/export.go": `package siteout

import "os"

type Bundle struct {
	Name string ` + "`json:\"name\"`" + `
}

func Export(bundle Bundle, out string) error {
	return os.WriteFile(out, []byte(bundle.Name), 0o644)
}

func ExportJSON(bundle Bundle, out string) error {
	return os.WriteFile(out, []byte("{}"), 0o644)
}
`,
			},
			wantRole: sourceindex.RoleExport,
		},
		{
			name: "export_surface_without_writes_unknown",
			files: map[string]string{
				"names/names.go": `package names

func ExportName(s string) string { return s }
`,
			},
			wantRole: sourceindex.RoleUnknown,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			gomod := tc.files["go.mod"]
			if gomod == "" {
				gomod = "module example.com/clusters\n\ngo 1.26.5\n"
			}
			mustWrite(t, filepath.Join(repo, "go.mod"), gomod)
			for rel, body := range tc.files {
				if rel == "go.mod" {
					continue
				}
				mustWrite(t, filepath.Join(repo, rel), body)
			}

			idx, err := sourceindex.Build(repo)
			if err != nil {
				// Offline neo4j module may fail go list; classify from evidence.
				if tc.name == "driver_client_adapter" {
					ev := sourceindex.PackageEvidence{
						Path:                  "neo",
						ExportedDecls:         []string{"Client"},
						ExportedFuncs:         []string{"New"},
						ClientConstructor:     true,
						ImportsExternalDriver: true,
					}
					topo := sourceindex.BuildRoleTopology(sourceindex.Index{
						Packages: map[string]sourceindex.PackageEvidence{"neo": ev},
					}, nil)
					if len(topo.Packages) != 1 || topo.Packages[0].Role != sourceindex.RoleAdapter {
						t.Fatalf("driver evidence classify got %+v", topo.Packages)
					}
					return
				}
				t.Fatal(err)
			}

			pkgPath := pickPackageForCase(idx, tc.name)
			pkg, ok := idx.Package(pkgPath)
			if !ok {
				t.Fatalf("missing %q in %+v", pkgPath, idx.Packages)
			}
			topo := sourceindex.BuildRoleTopology(sourceindex.Index{
				Packages: map[string]sourceindex.PackageEvidence{pkgPath: pkg},
			}, nil)
			if len(topo.Packages) != 1 {
				t.Fatalf("expected 1 node got %+v", topo.Packages)
			}
			got := topo.Packages[0]
			if got.Role != tc.wantRole {
				t.Fatalf("role=%q want %q evidence=%v pkg=%+v", got.Role, tc.wantRole, got.Evidence, pkg)
			}
		})
	}
}

func pickPackageForCase(idx sourceindex.Index, caseName string) string {
	prefer := map[string]string{
		"driver_client_adapter":        "neo",
		"http_client_still_adapter":    "httpcli",
		"server_beats_client_field":    "api",
		"plain_new_without_driver_unknown": "plain",
		"pipeline_register_modules":    "clients",
		"pipeline_job_runner":          "runner",
		"view_builder":                 "chronview",
		"json_only_still_dto":          "shapes",
		"locator_root_finders":         "reporoot",
		"locator_path_matchers":        "pathmatch",
	}
	if want, ok := prefer[caseName]; ok {
		for p := range idx.Packages {
			if p == want || filepath.Base(p) == want || p == want+"/" || len(p) >= len(want) && p[len(p)-len(want):] == want {
				return p
			}
			if filepath.Base(p) == want {
				return p
			}
		}
		for p := range idx.Packages {
			if filepath.Base(p) == want {
				return p
			}
		}
	}
	return primaryNonStubPackage(idx)
}

func TestPythonClusterRoles_adapterPipelineView(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		files    map[string]string
		wantRole string
	}{
		{
			name: "neo4j_client",
			files: map[string]string{
				"src/neoclient/__init__.py": "",
				"src/neoclient/client.py": `from neo4j import GraphDatabase

class Client:
    def __init__(self, uri: str) -> None:
        self.driver = GraphDatabase.driver(uri)
`,
			},
			wantRole: sourceindex.RoleAdapter,
		},
		{
			name: "pipeline_register",
			files: map[string]string{
				"src/pipepkg/__init__.py": "",
				"src/pipepkg/modules.py": `class ModuleRegistry:
    pass

def register_chronology(reg: ModuleRegistry) -> None:
    pass

def new_modules() -> dict[str, object]:
    return {}
`,
			},
			wantRole: sourceindex.RolePipeline,
		},
		{
			name: "view_build",
			files: map[string]string{
				"src/viewpkg/__init__.py": "",
				"src/viewpkg/page.py": `from dataclasses import dataclass

@dataclass
class Page:
    title: str

def build(title: str) -> Page:
    return Page(title=title)
`,
			},
			wantRole: sourceindex.RoleView,
		},
		{
			name: "validate_issue_report",
			files: map[string]string{
				"src/validpkg/__init__.py": "",
				"src/validpkg/validate.py": `from dataclasses import dataclass

@dataclass
class Issue:
    message: str

@dataclass
class Report:
    issues: list[Issue]

def validate_report(root: str) -> Report:
    return Report(issues=[])
`,
			},
			wantRole: sourceindex.RoleValidation,
		},
		{
			name: "locator_path_helpers",
			files: map[string]string{
				"src/locpkg/__init__.py": "",
				"src/locpkg/paths.py": `def find_root(start: str) -> str:
    return start

def pii_to_enc(path: str) -> str:
    return "enc/" + path
`,
			},
			wantRole: sourceindex.RoleLocator,
		},
		{
			name: "locator_root_helpers_python",
			files: map[string]string{
				"src/rootpkg/__init__.py": "",
				"src/rootpkg/paths.py": `def root(start: str) -> str:
    return start + "/data"

def content_root(start: str) -> str:
    return root(start) + "/content"
`,
			},
			wantRole: sourceindex.RoleLocator,
		},
		{
			name: "crypto_encrypt_decrypt",
			files: map[string]string{
				"src/cryptopkg/__init__.py": "",
				"src/cryptopkg/age.py": `def encrypt(src: str, dst: str) -> None:
    pass

def decrypt(src: str, dst: str) -> None:
    pass
`,
			},
			wantRole: sourceindex.RoleCrypto,
		},
		{
			name: "prompt_instruction_builders",
			files: map[string]string{
				"src/promptpkg/__init__.py": "",
				"src/promptpkg/prompts.py": `def chronology_build_what_instruction() -> str:
    return ""

def chronology_build_why_instruction() -> str:
    return ""

def timeline_list_title_instruction() -> str:
    return ""
`,
			},
			wantRole: sourceindex.RolePrompt,
		},
		{
			name: "container_runtime_hub_python",
			files: map[string]string{
				"src/svc1/__init__.py": "",
				"src/svc1/svc.py": `class Service1:
    pass
`,
				"src/svc2/__init__.py": "",
				"src/svc2/svc.py": `class Service2:
    pass
`,
				"src/container/__init__.py": "",
				"src/container/runtime.py": `from src.svc1.svc import Service1
from src.svc2.svc import Service2

class Runtime:
    def __init__(self):
        self.s1 = Service1()
        self.s2 = Service2()

def init(root: str) -> Runtime:
    return Runtime()
`,
			},
			wantRole: sourceindex.RoleContainer,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			mustWrite(t, filepath.Join(repo, "pyproject.toml"), "[project]\nname=\"c\"\nversion=\"0\"\n")
			for rel, body := range tc.files {
				mustWrite(t, filepath.Join(repo, rel), body)
			}
			idx, graph, err := sourceindex.BuildPython(repo)
			if err != nil {
				t.Fatal(err)
			}
			topo := sourceindex.BuildRoleTopology(idx, graph)
			found := false
			for _, n := range topo.Packages {
				if n.Role == tc.wantRole {
					found = true
					break
				}
			}
			if !found {
				roles := map[string]string{}
				for _, n := range topo.Packages {
					roles[n.Path] = n.Role
				}
				t.Fatalf("want role %q; got %v", tc.wantRole, roles)
			}
		})
	}
}

func TestConfig_fanOutGuard(t *testing.T) {
	t.Parallel()

	// Strong config: Load + Config type. Even with 3 internal imports, classifies as config.
	strongPkg := sourceindex.PackageEvidence{
		Path:          "myconfig",
		ExportedFuncs: []string{"Load"},
		ExportedDecls: []string{"Config"},
	}
	importGraphStrong := map[string][]string{
		"myconfig": {"dep1", "dep2", "dep3"},
	}
	topoStrong := sourceindex.BuildRoleTopology(sourceindex.Index{
		Packages: map[string]sourceindex.PackageEvidence{"myconfig": strongPkg},
	}, importGraphStrong)
	if len(topoStrong.Packages) != 1 || topoStrong.Packages[0].Role != sourceindex.RoleConfig {
		t.Fatalf("expected config for strong config with fanout, got %+v", topoStrong.Packages)
	}

	// Weak config: Load + Save only (no Config type, no json tags).
	// With fanout > 1, should stay unknown.
	weakPkg := sourceindex.PackageEvidence{
		Path:          "weakconfig",
		ExportedFuncs: []string{"Load", "Save"},
		ExportedDecls: []string{"File"},
	}
	importGraphWeak := map[string][]string{
		"weakconfig": {"dep1", "dep2", "dep3"},
	}
	topoWeak := sourceindex.BuildRoleTopology(sourceindex.Index{
		Packages: map[string]sourceindex.PackageEvidence{"weakconfig": weakPkg},
	}, importGraphWeak)
	if len(topoWeak.Packages) != 1 || topoWeak.Packages[0].Role != sourceindex.RoleUnknown {
		t.Fatalf("expected unknown for weak config with fanout > 1, got %+v", topoWeak.Packages)
	}
}

func TestRoleModifiers_goTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		files         map[string]string
		wantRole      string
		wantModifiers []string
	}{
		{
			name: "blended_validation_config",
			files: map[string]string{
				"flavour/flavour.go": `package flavour

type Issue struct {
	Message string
}

type Result struct {
	Issues []Issue
}

type Config struct {
	Name string
}

func Validate(id string) (*Result, error) {
	return &Result{}, nil
}

func Load(path string) (*Config, error) {
	return &Config{}, nil
}
`,
			},
			wantRole:      sourceindex.RoleValidation,
			wantModifiers: []string{sourceindex.RoleConfig},
		},
		{
			name: "blended_prompt_pipeline",
			files: map[string]string{
				"promptpipe/promptpipe.go": `package promptpipe

type Registry struct{}

func ChronologyBuildWhatInstruction() string { return "" }
func ChronologyBuildWhyInstruction() string { return "" }
func TimelineListTitleInstruction() string { return "" }

func Register(r Registry) {}
func Modules() map[string]any { return nil }
`,
			},
			wantRole:      sourceindex.RolePrompt,
			wantModifiers: []string{sourceindex.RolePipeline},
		},
		{
			name: "blended_locator_export",
			files: map[string]string{
				"locexport/locexport.go": `package locexport

import "os"

func FindProduct(root string) string { return root }
func ResolveFrom(root, rel string) string { return rel }

func ExportJSON(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
`,
			},
			wantRole:      sourceindex.RoleLocator,
			wantModifiers: []string{sourceindex.RoleExport},
		},
		{
			name: "single_signal_no_modifiers",
			files: map[string]string{
				"purevalid/valid.go": `package purevalid

type Issue struct {
	Message string
}

type Result struct {
	Issues []Issue
}

func Validate(id string) (*Result, error) {
	return &Result{}, nil
}
`,
			},
			wantRole:      sourceindex.RoleValidation,
			wantModifiers: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			gomod := "module example.com/modifiers\n\ngo 1.26.5\n"
			mustWrite(t, filepath.Join(repo, "go.mod"), gomod)
			for rel, body := range tc.files {
				mustWrite(t, filepath.Join(repo, rel), body)
			}

			idx, err := sourceindex.Build(repo)
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Packages) != 1 {
				t.Fatalf("expected 1 package, got %d", len(idx.Packages))
			}
			var pkgPath string
			for p := range idx.Packages {
				pkgPath = p
			}
			pkg := idx.Packages[pkgPath]
			topo := sourceindex.BuildRoleTopology(sourceindex.Index{
				Packages: map[string]sourceindex.PackageEvidence{pkgPath: pkg},
			}, nil)
			if len(topo.Packages) != 1 {
				t.Fatalf("expected 1 node, got %+v", topo.Packages)
			}
			got := topo.Packages[0]
			if got.Role != tc.wantRole {
				t.Fatalf("role=%q want %q (pkg=%+v)", got.Role, tc.wantRole, pkg)
			}
			if len(got.Modifiers) != len(tc.wantModifiers) {
				t.Fatalf("modifiers=%v want %v", got.Modifiers, tc.wantModifiers)
			}
			for i := range got.Modifiers {
				if got.Modifiers[i] != tc.wantModifiers[i] {
					t.Fatalf("modifiers=%v want %v", got.Modifiers, tc.wantModifiers)
				}
			}
		})
	}
}

func TestRoleModifiers_pythonTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		files         map[string]string
		wantRole      string
		wantModifiers []string
	}{
		{
			name: "blended_validation_config_python",
			files: map[string]string{
				"src/blended/__init__.py": "",
				"src/blended/mod.py": `from dataclasses import dataclass

@dataclass
class Issue:
    message: str

@dataclass
class Report:
    issues: list[Issue]

@dataclass
class Config:
    name: str

def validate_report(root: str) -> Report:
    return Report(issues=[])

def load_config(path: str) -> Config:
    return Config(name=path)
`,
			},
			wantRole:      sourceindex.RoleValidation,
			wantModifiers: []string{sourceindex.RoleConfig},
		},
		{
			name: "blended_prompt_pipeline_python",
			files: map[string]string{
				"src/promptpipe/__init__.py": "",
				"src/promptpipe/prompts.py": `def chronology_build_what_instruction() -> str:
    return ""

def chronology_build_why_instruction() -> str:
    return ""

def timeline_list_title_instruction() -> str:
    return ""

def register_pipeline():
    pass

def new_modules():
    pass
`,
			},
			wantRole:      sourceindex.RolePrompt,
			wantModifiers: []string{sourceindex.RolePipeline},
		},
		{
			name: "blended_locator_export_python",
			files: map[string]string{
				"src/locexp/__init__.py": "",
				"src/locexp/paths.py": `def find_root(start: str) -> str:
    return start

def pii_to_enc(path: str) -> str:
    return ""

def export_json(path: str) -> None:
    f = open(path, "w")
    f.write("{}")
`,
			},
			wantRole:      sourceindex.RoleLocator,
			wantModifiers: []string{sourceindex.RoleExport},
		},
		{
			name: "single_signal_no_modifiers_python",
			files: map[string]string{
				"src/purevalid/__init__.py": "",
				"src/purevalid/val.py": `from dataclasses import dataclass

@dataclass
class Issue:
    message: str

@dataclass
class Report:
    issues: list[Issue]

def validate_report(root: str) -> Report:
    return Report(issues=[])
`,
			},
			wantRole:      sourceindex.RoleValidation,
			wantModifiers: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			mustWrite(t, filepath.Join(repo, "pyproject.toml"), "[project]\nname=\"p\"\nversion=\"0\"\n")
			for rel, body := range tc.files {
				mustWrite(t, filepath.Join(repo, rel), body)
			}

			idx, graph, err := sourceindex.BuildPython(repo)
			if err != nil {
				t.Fatal(err)
			}
			topo := sourceindex.BuildRoleTopology(idx, graph)
			if len(topo.Packages) != 1 {
				t.Fatalf("expected 1 node, got %+v", topo.Packages)
			}
			got := topo.Packages[0]
			if got.Role != tc.wantRole {
				t.Fatalf("role=%q want %q", got.Role, tc.wantRole)
			}
			if len(got.Modifiers) != len(tc.wantModifiers) {
				t.Fatalf("modifiers=%v want %v", got.Modifiers, tc.wantModifiers)
			}
			for i := range got.Modifiers {
				if got.Modifiers[i] != tc.wantModifiers[i] {
					t.Fatalf("modifiers=%v want %v", got.Modifiers, tc.wantModifiers)
				}
			}
		})
	}
}



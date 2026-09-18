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
			name: "locator_stays_unknown",
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
		"locator_stays_unknown":        "reporoot",
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

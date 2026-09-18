package sourceindex_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestWorkerQueueCLIIngest_goTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		files    map[string]string
		wantRole string
	}{
		{
			name: "worker_handler_trio",
			files: map[string]string{
				"handlers/handlers.go": `package handlers

import "context"

type Echo struct{}

func (h *Echo) Kind() string { return "echo" }
func (h *Echo) Validate(payload map[string]any) error { return nil }
func (h *Echo) Run(ctx context.Context, job any, root string) (map[string]any, error) {
	return nil, nil
}

func Register() {
	registerHandler(&Echo{})
}

func registerHandler(h any) {}
`,
			},
			wantRole: sourceindex.RoleWorker,
		},
		{
			name: "queue_registry",
			files: map[string]string{
				"queue/queue.go": `package queue

import "context"

type Handler interface {
	Kind() string
}

func Register(h Handler) {}
func Enqueue(root string, kind string) (string, error) { return "", nil }
func Run(ctx context.Context, opts any) error { return nil }
func Load(path string) error { return nil }
func Save(path string) error { return nil }
`,
			},
			wantRole: sourceindex.RoleQueue,
		},
		{
			name: "cli_dispatch",
			files: map[string]string{
				"clipkg/cli.go": `package clipkg

import "io"

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return 2
	}
	return dispatch(args, stdout, stderr)
}

func dispatch(args []string, stdout, stderr io.Writer) int {
	switch args[0] {
	case "encrypt":
		return 0
	case "decrypt":
		return 0
	case "status":
		return 0
	case "scan":
		return 0
	default:
		return 2
	}
}
`,
			},
			wantRole: sourceindex.RoleCLI,
		},
		{
			name: "ingest_sync",
			files: map[string]string{
				"ingest/sync.go": `package ingest

import "context"

type Client interface {
	UpsertChunks(ctx context.Context, index string, chunks []any) error
	DeleteAllDocuments(ctx context.Context, index string) error
}

func Sync(ctx context.Context, cfg any, search Client, opts any) (int, error) {
	_ = search.DeleteAllDocuments(ctx, "idx")
	return 0, search.UpsertChunks(ctx, "idx", nil)
}
`,
			},
			wantRole: sourceindex.RoleIngest,
		},
		{
			name: "config_only_load_save",
			files: map[string]string{
				"cfg/cfg.go": `package cfg

type File struct {
	Version int ` + "`json:\"version\"`" + `
}

func Load(path string) (*File, error) { return &File{}, nil }
func Save(path string, f *File) error { return nil }
`,
			},
			wantRole: sourceindex.RoleConfig,
		},
		{
			name: "domain_stays_unknown",
			files: map[string]string{
				"domain/domain.go": `package domain

func Normalize(s string) string { return s }
func BuildLabel(a, b string) string { return a + b }
`,
			},
			wantRole: sourceindex.RoleUnknown,
		},
		{
			name: "test_only_worker_ignored",
			files: map[string]string{
				"plain/plain.go": `package plain

func Add(a, b int) int { return a + b }
`,
				"plain/plain_test.go": `package plain

import (
	"context"
	"testing"
)

type Echo struct{}

func (h *Echo) Kind() string { return "echo" }
func (h *Echo) Validate(payload map[string]any) error { return nil }
func (h *Echo) Run(ctx context.Context, job any, root string) (map[string]any, error) {
	return nil, nil
}

func TestAdd(t *testing.T) {
	_ = Add(1, 2)
	_ = (&Echo{}).Kind()
}
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
			mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/roles\n\ngo 1.26.5\n")
			for rel, body := range tc.files {
				mustWrite(t, filepath.Join(repo, rel), body)
			}
			idx, err := sourceindex.Build(repo)
			if err != nil {
				t.Fatal(err)
			}
			pkgPath := primaryNonStubPackage(idx)
			pkg, ok := idx.Package(pkgPath)
			if !ok {
				t.Fatalf("missing package in %+v", idx.Packages)
			}
			topo := sourceindex.BuildRoleTopology(sourceindex.Index{
				Packages: map[string]sourceindex.PackageEvidence{pkgPath: pkg},
			}, nil)
			if len(topo.Packages) != 1 {
				t.Fatalf("expected 1 node, got %+v", topo.Packages)
			}
			got := topo.Packages[0]
			if got.Role != tc.wantRole {
				t.Fatalf("role=%q want %q evidence=%v pkg=%+v", got.Role, tc.wantRole, got.Evidence, pkg)
			}
		})
	}
}

func TestWorkerQueueCLIIngest_pythonTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		files    map[string]string
		wantRole string
	}{
		{
			name: "celery_task",
			files: map[string]string{
				"src/workerpkg/__init__.py": "",
				"src/workerpkg/tasks.py": `from celery import shared_task

@shared_task
def paint(job_id: str) -> None:
    pass
`,
			},
			wantRole: sourceindex.RoleWorker,
		},
		{
			name: "enqueue_queue",
			files: map[string]string{
				"src/queuepkg/__init__.py": "",
				"src/queuepkg/queue.py": `import celery

def enqueue(name: str) -> None:
    celery.current_app.send_task(name)

def run() -> None:
    celery.worker_main()
`,
			},
			wantRole: sourceindex.RoleQueue,
		},
		{
			name: "click_cli",
			files: map[string]string{
				"src/clipkg/__init__.py": "",
				"src/clipkg/cli.py": `import click

@click.command()
@click.option("--verbose", is_flag=True)
def main(verbose: bool) -> None:
    pass
`,
			},
			wantRole: sourceindex.RoleCLI,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			mustWrite(t, filepath.Join(repo, "pyproject.toml"), "[project]\nname = \"rolespy\"\nversion = \"0.0.1\"\n")
			for rel, body := range tc.files {
				mustWrite(t, filepath.Join(repo, rel), body)
			}
			idx, graph, err := sourceindex.BuildPython(repo)
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Packages) == 0 {
				t.Fatal("expected python packages")
			}
			topo := sourceindex.BuildRoleTopology(idx, graph)
			found := false
			for _, n := range topo.Packages {
				if strings.Contains(n.Path, "pkg") || strings.HasSuffix(n.Path, tc.wantRole) {
					// pick the package under src/
					if !strings.HasPrefix(n.Path, "src/") {
						continue
					}
					found = true
					if n.Role != tc.wantRole {
						t.Fatalf("path=%s role=%q want %q evidence=%v packages=%v",
							n.Path, n.Role, tc.wantRole, n.Evidence, packageRoles(topo))
					}
					if n.Language != sourceindex.LangPython {
						t.Fatalf("language=%q want python", n.Language)
					}
					return
				}
			}
			// Fallback: any package matching want role
			for _, n := range topo.Packages {
				if n.Role == tc.wantRole {
					found = true
					return
				}
			}
			if !found {
				t.Fatalf("no package classified as %q; got %v", tc.wantRole, packageRoles(topo))
			}
		})
	}
}

func packageRoles(topo sourceindex.RoleTopology) map[string]string {
	out := map[string]string{}
	for _, n := range topo.Packages {
		out[n.Path] = n.Role
	}
	return out
}

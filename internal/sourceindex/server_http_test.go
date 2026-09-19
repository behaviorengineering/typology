package sourceindex_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/sourceindex"
)

func TestHTTPServerSignals_table(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		files      map[string]string
		wantRole   string
		wantEvidence []string // any of these must appear when wantRole is server
		wantNotRole string
	}{
		{
			name: "mux_mount",
			files: map[string]string{
				"mount/mount.go": `package mount

import "net/http"

func MountWithResolver(mux *http.ServeMux) {
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
`,
			},
			wantRole:     sourceindex.RoleHTTPSurface,
			wantEvidence: []string{"http_mux_param", "http_route_register"},
		},
		{
			name: "handler_struct_plus_mount",
			files: map[string]string{
				"api/handlers.go": `package api

import "net/http"

type Handlers struct{}

func (h Handlers) HandleList(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "nope", http.StatusBadRequest)
}

func Mount(mux *http.ServeMux, h Handlers) {
	mux.HandleFunc("/list", h.HandleList)
}
`,
			},
			wantRole:     sourceindex.RoleHTTPSurface,
			wantEvidence: []string{"http_mux_param"},
		},
		{
			name: "handler_factory",
			files: map[string]string{
				"files/files.go": `package files

import "net/http"

func FileHandler(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}
}
`,
			},
			wantRole:     sourceindex.RoleHTTPSurface,
			wantEvidence: []string{"http_handler_result"},
		},
		{
			name: "middleware",
			files: map[string]string{
				"mw/mw.go": `package mw

import "net/http"

func WrapHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
`,
			},
			wantRole:     sourceindex.RoleHTTPSurface,
			wantEvidence: []string{"http_handler_result"},
		},
		{
			name: "listen_non_main",
			files: map[string]string{
				"listen/listen.go": `package listen

import "net/http"

func ServeAddr(addr string, h http.Handler) error {
	return http.ListenAndServe(addr, h)
}
`,
			},
			wantRole:     sourceindex.RoleHTTPSurface,
			wantEvidence: []string{"http_listen_serve"},
		},
		{
			name: "tier2_handler_signature",
			files: map[string]string{
				"h/h.go": `package h

import "net/http"

func HandlePing(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "pong", http.StatusOK)
}
`,
			},
			wantRole:     sourceindex.RoleHTTPSurface,
			wantEvidence: []string{"http_handler_signature", "http_response_helper"},
		},
		{
			name: "pure_client",
			files: map[string]string{
				"client/client.go": `package client

import "net/http"

type Client struct {
	c *http.Client
}

func NewClient() *Client {
	return &Client{c: http.DefaultClient}
}

func (c *Client) Get(url string) (*http.Response, error) {
	return c.c.Get(url)
}
`,
			},
			wantRole: sourceindex.RoleAdapter,
		},
		{
			name: "httptest_only_in_test_file",
			files: map[string]string{
				"util/util.go": `package util

func Add(a, b int) int { return a + b }
`,
				"util/util_test.go": `package util

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	_ = Add(1, 2)
}
`,
			},
			wantRole:    sourceindex.RoleUnknown,
			wantNotRole: sourceindex.RoleHTTPSurface,
		},
		{
			name: "framework_mount",
			files: map[string]string{
				"go.mod": `module example.com/httpserver

go 1.26.5

require github.com/gin-gonic/gin v1.0.0

replace github.com/gin-gonic/gin => ./stub/gin
`,
				"stub/gin/gin.go": `package gin

type Engine struct{}
type Context struct{}

func New() *Engine { return &Engine{} }

func (e *Engine) GET(path string, handlers ...func(*Context)) *Engine { return e }
`,
				"web/web.go": `package web

import "github.com/gin-gonic/gin"

func Mount() *gin.Engine {
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {})
	return r
}
`,
			},
			wantRole:     sourceindex.RoleHTTPSurface,
			wantEvidence: []string{"http_framework_import", "http_framework_route"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			gomod := tc.files["go.mod"]
			if gomod == "" {
				gomod = "module example.com/httpserver\n\ngo 1.26.5\n"
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
				t.Fatal(err)
			}

			pkgPath := primaryNonStubPackage(idx)
			pkg, ok := idx.Package(pkgPath)
			if !ok {
				t.Fatalf("missing package %q in %+v", pkgPath, idx.Packages)
			}

			topo := sourceindex.BuildRoleTopology(sourceindex.Index{
				Packages: map[string]sourceindex.PackageEvidence{pkgPath: pkg},
			}, nil)
			if len(topo.Packages) != 1 {
				t.Fatalf("expected 1 role node, got %+v", topo.Packages)
			}
			got := topo.Packages[0]
			if tc.wantNotRole != "" && got.Role == tc.wantNotRole {
				t.Fatalf("role=%q evidence=%v must not be %q (pkg mux=%v route=%v listen=%v result=%v sig=%v helper=%v fw=%v/%v)",
					got.Role, got.Evidence, tc.wantNotRole,
					pkg.HTTPMuxParam, pkg.HTTPRouteRegister, pkg.HTTPListenServe,
					pkg.HTTPHandlerResult, pkg.HTTPHandlerSignature, pkg.HTTPResponseHelper,
					pkg.ImportsHTTPFramework, pkg.HTTPFrameworkRoute)
			}
			if tc.wantRole != "" && got.Role != tc.wantRole {
				t.Fatalf("role=%q want %q evidence=%v pkg=%+v", got.Role, tc.wantRole, got.Evidence, pkg)
			}
			if len(tc.wantEvidence) > 0 {
				joined := strings.Join(got.Evidence, ",")
				found := false
				for _, e := range tc.wantEvidence {
					if strings.Contains(joined, e) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("evidence %v missing any of %v", got.Evidence, tc.wantEvidence)
				}
			}
		})
	}
}

func primaryNonStubPackage(idx sourceindex.Index) string {
	var best string
	for p := range idx.Packages {
		if strings.Contains(p, "/stub/") || strings.HasPrefix(p, "stub/") {
			continue
		}
		if best == "" || len(p) < len(best) {
			best = p
		}
	}
	if best != "" {
		return best
	}
	for p := range idx.Packages {
		return p
	}
	return ""
}

func TestClassifyPackage_httpMuxParam(t *testing.T) {
	t.Parallel()
	ev := sourceindex.PackageEvidence{
		Path:           "internal/chronologyapi",
		ImportsNetHTTP: true,
		HTTPMuxParam:   true,
	}
	topo := sourceindex.BuildRoleTopology(sourceindex.Index{
		Packages: map[string]sourceindex.PackageEvidence{"internal/chronologyapi": ev},
	}, nil)
	if len(topo.Packages) != 1 || topo.Packages[0].Role != sourceindex.RoleHTTPSurface {
		t.Fatalf("got %+v; mux param must classify as server", topo.Packages)
	}
}

// Package boardsviewer serves the embedded cable-board SPA with an on-disk
// boards.json projection (typically the XDG viewer public/ tree).
package boardsviewer

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	terrors "github.com/behaviorengineering/typology/errors"
)

//go:embed all:dist
var distRoot embed.FS

// Options configures the boards HTTP server.
type Options struct {
	// Addr is the listen address (for example "127.0.0.1:5173").
	Addr string
	// PublicDir is the materialized Vite public/ tree (boards.json + boards/).
	PublicDir string
}

// Handler returns an HTTP handler that serves the embedded UI and overlays
// board JSON from PublicDir.
func Handler(publicDir string) (http.Handler, error) {
	public := filepath.Clean(strings.TrimSpace(publicDir))
	if public == "" || public == "." {
		return nil, terrors.New(terrors.CodeInvalid, "boardsviewer.Handler", "public dir empty")
	}
	manifest := filepath.Join(public, "boards.json")
	if _, err := os.Stat(manifest); err != nil {
		return nil, terrors.Wrap(err, terrors.CodeFailedPrecondition, "boardsviewer.Handler",
			"boards.json missing; run typology boards register or boards sync --viewer PUBLIC_DIR").
			With("path", manifest)
	}
	ui, err := fs.Sub(distRoot, "dist")
	if err != nil {
		return nil, terrors.Wrap(err, terrors.CodeInternal, "boardsviewer.Handler", "embed dist")
	}
	if _, err := ui.Open("index.html"); err != nil {
		return nil, terrors.Wrap(err, terrors.CodeFailedPrecondition, "boardsviewer.Handler",
			"embedded UI missing index.html; run make cable-board-dist").
			With("err", err.Error())
	}

	boardsDir := filepath.Join(public, "boards")
	mux := http.NewServeMux()
	mux.HandleFunc("/boards.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.ServeFile(w, r, manifest)
	})
	mux.Handle("/boards/", http.StripPrefix("/boards/", http.FileServer(http.Dir(boardsDir))))
	mux.Handle("/", spaFileServer(ui))
	return mux, nil
}

// ListenAndServe starts the boards viewer until the listener fails.
func ListenAndServe(opts Options) error {
	addr := strings.TrimSpace(opts.Addr)
	if addr == "" {
		addr = "127.0.0.1:5173"
	}
	h, err := Handler(opts.PublicDir)
	if err != nil {
		return err
	}
	return http.ListenAndServe(addr, h)
}

func spaFileServer(ui fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		if f, err := ui.Open(name); err == nil {
			_ = f.Close()
			http.ServeFileFS(w, r, ui, name)
			return
		}
		http.ServeFileFS(w, r, ui, "index.html")
	})
}

// EmbeddedOK reports whether a usable SPA index is embedded.
func EmbeddedOK() bool {
	ui, err := fs.Sub(distRoot, "dist")
	if err != nil {
		return false
	}
	f, err := ui.Open("index.html")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// DescribePublic summarizes the public dir for CLI messages.
func DescribePublic(publicDir string) string {
	return fmt.Sprintf("public=%s", filepath.Clean(publicDir))
}

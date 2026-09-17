package boardsviewer_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/boardsviewer"
)

func TestHandler_overlaysBoardsJSON(t *testing.T) {
	t.Parallel()
	public := t.TempDir()
	manifest := map[string]any{
		"defaultBoard": "demo",
		"boards": []map[string]string{
			{"id": "demo", "label": "Demo", "graph": "/boards/demo/assembly-graph.json"},
		},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(public, "boards.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	boardDir := filepath.Join(public, "boards", "demo")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	graph := []byte(`{"nodes":[],"edges":[],"slice":"demo"}` + "\n")
	if err := os.WriteFile(filepath.Join(boardDir, "assembly-graph.json"), graph, 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := boardsviewer.Handler(public)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boards.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("boards.json status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"demo"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boards/demo/assembly-graph.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("graph status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index status=%d", rec.Code)
	}
	if !boardsviewer.EmbeddedOK() {
		t.Fatal("expected EmbeddedOK")
	}
}

func TestHandler_missingManifest(t *testing.T) {
	t.Parallel()
	_, err := boardsviewer.Handler(t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "boards.json") {
		t.Fatalf("err=%v", err)
	}
}

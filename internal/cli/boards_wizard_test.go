package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoardsRegisterWizard_nonTTY(t *testing.T) {
	old := stdinIsTTY
	stdinIsTTY = func() bool { return false }
	t.Cleanup(func() { stdinIsTTY = old })

	var stdout, stderr bytes.Buffer
	code := Run([]string{"boards", "register"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit=%d want 2 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "TTY") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestDefaultViewerPublicHint_usesXDGDataDir(t *testing.T) {
	data := t.TempDir()
	t.Setenv("TYPOLOGY_DATA_DIR", data)
	t.Setenv("TYPOLOGY_CONFIG_DIR", t.TempDir())

	got := defaultViewerPublicHint()
	want := filepath.Join(data, "viewer", "public")
	if got != want {
		t.Fatalf("hint=%q want %q", got, want)
	}
}

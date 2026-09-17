package cli

import (
	"bytes"
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

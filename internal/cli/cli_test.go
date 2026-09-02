package cli_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/behaviorengineering/typology/internal/cli"
)

func TestCLI_validate_ok(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	var out, errOut bytes.Buffer
	code := cli.Run([]string{"validate", repo}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "validate: ok") {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestCLI_show_slices(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	var out, errOut bytes.Buffer
	code := cli.Run([]string{"show", "--catalog", filepath.Join(repo, "architecture", "typology.yaml")}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "billing") {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestCLI_nil_writers(t *testing.T) {
	t.Parallel()
	if code := cli.Run([]string{"version"}, nil, nil, &bytes.Buffer{}); code != 2 {
		t.Fatalf("nil stdout exit=%d", code)
	}
	if code := cli.Run([]string{"version"}, nil, &bytes.Buffer{}, nil); code != 2 {
		t.Fatalf("nil stderr exit=%d", code)
	}
}

func TestCLI_show_graph(t *testing.T) {
	t.Parallel()
	repo, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "tiny-module"))
	var out, errOut bytes.Buffer
	code := cli.Run([]string{"show", "graph", "--catalog", filepath.Join(repo, "architecture", "typology.yaml")}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Typology Import Graph") {
		t.Fatalf("stdout=%q", out.String())
	}
}

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		ldflag        string
		moduleVersion string
		want          string
	}{
		{name: "ldflag wins", ldflag: "0.0.28", moduleVersion: "v0.0.27", want: "0.0.28"},
		{name: "go install module version", ldflag: "dev", moduleVersion: "v0.0.28", want: "v0.0.28"},
		{name: "local devel stays dev", ldflag: "dev", moduleVersion: "(devel)", want: "dev"},
		{name: "empty module stays dev", ldflag: "dev", moduleVersion: "", want: "dev"},
		{name: "whitespace ldflag falls through", ldflag: "  ", moduleVersion: "v1.2.3", want: "v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveVersion(tt.ldflag, tt.moduleVersion); got != tt.want {
				t.Fatalf("resolveVersion(%q, %q) = %q, want %q", tt.ldflag, tt.moduleVersion, got, tt.want)
			}
		})
	}
}

func TestCLI_version_printsResolved(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	code := Run([]string{"version"}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	got := out.String()
	if !strings.HasPrefix(got, "typology ") {
		t.Fatalf("stdout=%q, want typology prefix", got)
	}
	ver := strings.TrimSpace(strings.TrimPrefix(got, "typology "))
	if ver == "" {
		t.Fatalf("stdout=%q, empty version", got)
	}
}
